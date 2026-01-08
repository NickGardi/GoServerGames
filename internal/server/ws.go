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
			c.mm.RemovePlayer(c.playerID)
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
			var hello net.HelloMessage
			if err := json.Unmarshal(message, &hello); err == nil {
				log.Printf("Hello received from session %s (username: %s), adding player", c.session.PlayerName, hello.Name)
				// Use session player name, not hello.Name
				// AddPlayer will send welcome and broadcast lobby update
				c.playerID = c.mm.AddPlayer(c.session.PlayerName, c)
				log.Printf("Player added with ID %d, name: %s", c.playerID, c.session.PlayerName)
			} else {
				log.Printf("Error parsing hello message: %v", err)
			}

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

		case "readyForNextRound":
			var readyMsg net.ReadyForNextRoundMessage
			if err := json.Unmarshal(message, &readyMsg); err == nil {
				log.Printf("Received readyForNextRound from player %d: ready=%v", c.playerID, readyMsg.Ready)
				if c.speedTypeRoom != nil {
					c.speedTypeRoom.SetReadyForNext(c.playerID, readyMsg.Ready)
					// Broadcast updated state with ready status
					c.mm.broadcastSpeedTypeState(c.speedTypeRoom)
					
					// Check if both ready - the matchmaking loop will handle starting next round
					if c.speedTypeRoom.AllReadyForNext() {
						log.Printf("Both players ready for next round in room %s", c.speedTypeRoom.ID)
					}
				}
			}

		case "speedTypeSubmit":
			var submitMsg net.SpeedTypeSubmitMessage
			if err := json.Unmarshal(message, &submitMsg); err == nil {
				if c.speedTypeRoom != nil {
					c.speedTypeRoom.SubmitWord(c.playerID, submitMsg.Word, submitMsg.TimeMs)
					// Broadcast updated state to both players
					c.mm.broadcastSpeedTypeState(c.speedTypeRoom)
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
			http.Error(w, "Not authenticated", http.StatusUnauthorized)
			return
		}

		session, ok := sessionStore.GetSession(cookie.Value)
		if !ok {
			http.Error(w, "Invalid session", http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}

		c := NewConnection(conn, mm, session)
		go c.writePump()
		go c.readPump()
		go c.StartSnapshotLoop()

		log.Printf("Client connected: %s", session.PlayerName)
	}
}
