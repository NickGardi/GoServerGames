package server

import (
	"1v1/internal/game"
	"1v1/internal/net"
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
	conn      *websocket.Conn
	send      chan []byte
	mm        *Matchmaking
	room      *game.Room
	playerIdx int
	playerID  int
}

func NewConnection(conn *websocket.Conn, mm *Matchmaking) *Connection {
	return &Connection{
		conn: conn,
		send: make(chan []byte, 256),
		mm:   mm,
	}
}

func (c *Connection) SendWelcome(playerID int, roomID string) {
	c.playerID = playerID
	msg := net.WelcomeMessage{
		Type:     "welcome",
		PlayerID: playerID,
		RoomID:   roomID,
	}
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
	default:
		log.Printf("Send buffer full for player %d", c.playerID)
	}
}

func (c *Connection) readPump() {
	defer func() {
		c.conn.Close()
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
				c.playerID = c.mm.AddPlayer(hello.Name, c)
			}

		case "input":
			var input net.InputMessage
			if err := json.Unmarshal(message, &input); err == nil {
				if c.room != nil && c.playerIdx >= 0 && c.playerIdx < 2 {
					c.room.QueueInput(c.playerIdx, input)
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
		if c.room != nil {
			snap := c.room.GetSnap()
			c.SendMessage(snap)
		}
	}
}

func HandleWebSocket(mm *Matchmaking) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}

		c := NewConnection(conn, mm)
		go c.writePump()
		go c.readPump()
		go c.StartSnapshotLoop()

		log.Printf("Client connected")
	}
}

