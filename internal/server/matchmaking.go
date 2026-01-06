package server

import (
	"1v1/internal/game"
	"fmt"
	"sync"
	"time"
)

type Matchmaking struct {
	queue      []*WaitingPlayer
	rooms      map[string]*game.Room
	nextRoomID int
	nextPlayerID int
	mu         sync.Mutex
}

type WaitingPlayer struct {
	PlayerID int
	Name     string
	Conn     *Connection
}

func NewMatchmaking() *Matchmaking {
	return &Matchmaking{
		queue:      make([]*WaitingPlayer, 0),
		rooms:      make(map[string]*game.Room),
		nextPlayerID: 1,
		nextRoomID: 1,
	}
}

func (m *Matchmaking) AddPlayer(name string, conn *Connection) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	playerID := m.nextPlayerID
	m.nextPlayerID++

	wp := &WaitingPlayer{
		PlayerID: playerID,
		Name:     name,
		Conn:     conn,
	}

	m.queue = append(m.queue, wp)

	// Try to match
	if len(m.queue) >= 2 {
		p1 := m.queue[0]
		p2 := m.queue[1]
		m.queue = m.queue[2:]

		// Create room
		roomID := m.generateRoomID()
		room := game.NewRoom(roomID)
		room.AddPlayer(p1.PlayerID, p1.Name)
		room.AddPlayer(p2.PlayerID, p2.Name)
		m.rooms[roomID] = room

		// Assign players to room
		p1.Conn.room = room
		p1.Conn.playerIdx = 0
		p2.Conn.room = room
		p2.Conn.playerIdx = 1

		// Send welcome messages
		p1.Conn.SendWelcome(p1.PlayerID, roomID)
		p2.Conn.SendWelcome(p2.PlayerID, roomID)
	}

	return playerID
}

func (m *Matchmaking) generateRoomID() string {
	id := m.nextRoomID
	m.nextRoomID++
	return fmt.Sprintf("room%d", id)
}

func (m *Matchmaking) GetRoom(roomID string) *game.Room {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rooms[roomID]
}

func (m *Matchmaking) RemovePlayer(playerID int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove from queue
	for i, wp := range m.queue {
		if wp.PlayerID == playerID {
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			break
		}
	}

	// Handle disconnection from room
	for _, room := range m.rooms {
		for i, player := range room.Players {
			if player != nil && player.ID == playerID {
				player.Connected = false
				// Opponent wins by default
				opponentIdx := 1 - i
				if room.Players[opponentIdx] != nil {
					room.Players[opponentIdx].Score++
				}
				break
			}
		}
	}
}

func (m *Matchmaking) StartRoomTicks() {
	ticker := time.NewTicker(game.TickDuration)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		for _, room := range m.rooms {
			room.ProcessTick()
		}
		m.mu.Unlock()
	}
}

