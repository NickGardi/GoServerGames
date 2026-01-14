package server

import (
	"GoServerGames/internal/game"
	"GoServerGames/internal/net"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for local dev
	},
}

type Connection struct {
	conn            *websocket.Conn
	send            chan []byte
	mm              *Matchmaking
	room            *game.Room
	speedTypeRoom   *game.SpeedTypeRoom
	mathSprintRoom  *game.MathSprintRoom
	clickSpeedRoom  *game.ClickSpeedRoom
	playerIdx       int
	playerID        int
	lobbyPlayer     *LobbyPlayer
	session         *Session
	lastBufferFullLog time.Time
}

func NewConnection(conn *websocket.Conn, mm *Matchmaking, session *Session) *Connection {
	return &Connection{
		conn:    conn,
		send:    make(chan []byte, 1024), // Large buffer to handle bursts
		mm:      mm,
		session: session,
	}
}

func (c *Connection) SendWelcome(playerID int, roomID string, lobby *net.LobbyState) {
	c.playerID = playerID
	msg := net.WelcomeMessage{
		Type:     "welcome",
		PlayerID: playerID,
		RoomID:   roomID,
		RoomCode: c.session.RoomCode,
		Lobby:    lobby,
	}
	log.Printf("Sending welcome to player %d with lobby: %v", playerID, lobby != nil)
	c.SendMessage(msg)
}

func (c *Connection) SendLobbyUpdate(lobby *net.LobbyState) {
	msg := net.SnapMessage{
		Type:  "lobby",
		Lobby: lobby,
	}
	log.Printf("Sending lobby update to player %d: %d players", c.playerID, len(lobby.Players))
	c.SendMessage(msg)
}

func (c *Connection) SendMessage(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}
	select {
	case c.send <- data:
		// Message queued successfully
	default:
		// Buffer full - log occasionally to avoid spam (once per second max)
		if c.lastBufferFullLog.IsZero() || time.Since(c.lastBufferFullLog) > time.Second {
			log.Printf("Send buffer full for player %d - messages may be dropped", c.playerID)
			c.lastBufferFullLog = time.Now()
		}
	}
}

func (c *Connection) readPump() {
	defer func() {
		c.conn.Close()
		if c.playerID > 0 {
			c.mm.RemovePlayer(c.playerID, c)
		}
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var baseMsg map[string]interface{}
		if err := json.Unmarshal(message, &baseMsg); err != nil {
			continue
		}

		msgType, ok := baseMsg["type"].(string)
		if !ok {
			continue
		}

		switch msgType {
		case "hello":
			// Hello is now just for compatibility - player is already added on connect
			log.Printf("Hello received from player %d (%s)", c.playerID, c.session.PlayerName)

		case "ready":
			var ready net.ReadyMessage
			if err := json.Unmarshal(message, &ready); err == nil {
				c.mm.SetReady(c.playerID, ready.Ready)
			}

		case "input":
			var input net.InputMessage
			if err := json.Unmarshal(message, &input); err == nil {
				if c.room != nil && c.playerIdx >= 0 && c.playerIdx < 2 {
					c.room.QueueInput(c.playerIdx, input)
				}
			}

		case "selectGame":
			var selectMsg net.SelectGameMessage
			if err := json.Unmarshal(message, &selectMsg); err == nil {
				c.mm.SelectGame(c.playerID, selectMsg.GameType)
			}

		// readyForNextRound message handler removed - rounds auto-advance after 5 seconds

		case "speedTypeSubmit":
			var submitMsg net.SpeedTypeSubmitMessage
			if err := json.Unmarshal(message, &submitMsg); err == nil {
				if c.speedTypeRoom != nil {
					c.speedTypeRoom.SubmitWord(c.playerID, submitMsg.Word, submitMsg.TimeMs)
					c.mm.broadcastSpeedTypeState(c.speedTypeRoom)
				}
			}

		case "mathSprintSubmit":
			var submitMsg net.MathSprintSubmitMessage
			if err := json.Unmarshal(message, &submitMsg); err == nil {
				if c.mathSprintRoom != nil {
					c.mathSprintRoom.SubmitAnswer(c.playerID, submitMsg.Answer, submitMsg.TimeMs)
					c.mm.broadcastMathSprintState(c.mathSprintRoom)
				}
			}

		case "clickSpeedSubmit":
			var submitMsg net.ClickSpeedSubmitMessage
			if err := json.Unmarshal(message, &submitMsg); err == nil {
				if c.clickSpeedRoom != nil {
					c.clickSpeedRoom.SubmitClick(c.playerID, submitMsg.TimeMs)
					c.mm.broadcastClickSpeedState(c.clickSpeedRoom)
				}
			}
		}
	}
}

func (c *Connection) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Send queued messages
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Connection) StartSnapshotLoop() {
	ticker := time.NewTicker(time.Second / 60) // 60Hz
	defer ticker.Stop()

	for range ticker.C {
		// Only send snapshots for the old FPS game room, not speed type
		if c.room != nil && c.speedTypeRoom == nil {
			snap := c.room.GetSnap()
			c.SendMessage(snap)
		}
	}
}

func HandleWebSocketWithAuth(mm *Matchmaking, sessionStore *SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session from cookie
		cookie, err := r.Cookie("session")
		if err != nil {
			log.Printf("WebSocket upgrade failed: no session cookie - %v", err)
			http.Error(w, "Not authenticated", http.StatusUnauthorized)
			return
		}

		session, ok := sessionStore.GetSession(cookie.Value)
		if !ok {
			log.Printf("WebSocket upgrade failed: invalid session cookie %s", cookie.Value)
			http.Error(w, "Invalid session", http.StatusUnauthorized)
			return
		}
		
		log.Printf("WebSocket upgrade: session validated for %s (room: %s)", session.PlayerName, session.RoomCode)

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}

		c := NewConnection(conn, mm, session)
		
		// Validate room code from session
		if session.RoomCode == "" {
			log.Printf("Client rejected: %s - session has empty room code", session.PlayerName)
			conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "Invalid session: room code missing"))
			conn.Close()
			return
		}
		
		log.Printf("Client connected: %s (room: %s)", session.PlayerName, session.RoomCode)
		
		// Add player immediately when they connect (handles both lobby and game page connections)
		log.Printf("About to call AddPlayer for %s in room %s", session.PlayerName, session.RoomCode)
		playerID := mm.AddPlayer(session.PlayerName, session.RoomCode, c)
		log.Printf("AddPlayer returned playerID=%d for %s in room %s", playerID, session.PlayerName, session.RoomCode)
		if playerID == 0 {
			log.Printf("Failed to add player: %s in room %s (room may be full or validation error)", session.PlayerName, session.RoomCode)
			conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "Room full or invalid"))
			conn.Close()
			return
		}
		log.Printf("Player added with ID %d, name: %s, room: %s", playerID, session.PlayerName, session.RoomCode)
		
		go c.writePump()
		go c.readPump()
		go c.StartSnapshotLoop()
	}
}
