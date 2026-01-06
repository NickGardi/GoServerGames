package client

import (
	"1v1/internal/net"
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type NetClient struct {
	conn     *websocket.Conn
	send     chan []byte
	snapshot chan net.SnapMessage
	welcome  chan net.WelcomeMessage
	mu       sync.Mutex
	PlayerID int
	RoomID   string
}

func NewNetClient(addr string) (*NetClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial(addr, nil)
	if err != nil {
		return nil, err
	}

	nc := &NetClient{
		conn:     conn,
		send:     make(chan []byte, 256),
		snapshot: make(chan net.SnapMessage, 10),
		welcome:  make(chan net.WelcomeMessage, 1),
	}

	go nc.readPump()
	go nc.writePump()

	// Send hello
	hello := net.HelloMessage{
		Type:    "hello",
		Name:    "Player",
		Version: 1,
	}
	nc.SendMessage(hello)

	return nc, nil
}

func (nc *NetClient) SendMessage(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case nc.send <- data:
	default:
	}
}

func (nc *NetClient) SendInput(input net.InputMessage) {
	nc.SendMessage(input)
}

func (nc *NetClient) readPump() {
	defer nc.conn.Close()

	for {
		_, message, err := nc.conn.ReadMessage()
		if err != nil {
			log.Printf("Read error: %v", err)
			return
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
		case "welcome":
			var welcome net.WelcomeMessage
			if err := json.Unmarshal(message, &welcome); err == nil {
				nc.mu.Lock()
				nc.PlayerID = welcome.PlayerID
				nc.RoomID = welcome.RoomID
				nc.mu.Unlock()
				select {
				case nc.welcome <- welcome:
				default:
				}
			}

		case "snap":
			var snap net.SnapMessage
			if err := json.Unmarshal(message, &snap); err == nil {
				select {
				case nc.snapshot <- snap:
				default:
					// Drop if buffer full
				}
			}
		}
	}
}

func (nc *NetClient) writePump() {
	defer nc.conn.Close()

	for {
		select {
		case message, ok := <-nc.send:
			if !ok {
				nc.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := nc.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		}
	}
}

func (nc *NetClient) GetSnapshot() *net.SnapMessage {
	select {
	case snap := <-nc.snapshot:
		return &snap
	default:
		return nil
	}
}

func (nc *NetClient) GetWelcome() *net.WelcomeMessage {
	select {
	case welcome := <-nc.welcome:
		return &welcome
	default:
		return nil
	}
}

func (nc *NetClient) Close() {
	nc.conn.Close()
	close(nc.send)
}

