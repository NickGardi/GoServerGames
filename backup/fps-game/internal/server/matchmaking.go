package server

import (
	"1v1/internal/game"
	"1v1/internal/net"
	"fmt"
	"log"
	"sync"
	"time"
)

type Matchmaking struct {
	lobby       []*LobbyPlayer
	rooms       map[string]*game.Room
	nextRoomID  int
	nextPlayerID int
	mu          sync.Mutex
}

type LobbyPlayer struct {
	PlayerID int
	Name     string
	Conn     *Connection
	Ready    bool
}

func NewMatchmaking() *Matchmaking {
	return &Matchmaking{
		lobby:       make([]*LobbyPlayer, 0),
		rooms:       make(map[string]*game.Room),
		nextPlayerID: 1,
		nextRoomID: 1,
	}
}

func (m *Matchmaking) AddPlayer(name string, conn *Connection) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	playerID := m.nextPlayerID
	m.nextPlayerID++

	lp := &LobbyPlayer{
		PlayerID: playerID,
		Name:     name,
		Conn:     conn,
		Ready:    false,
	}

	m.lobby = append(m.lobby, lp)
	conn.lobbyPlayer = lp
	
	// Send welcome message to the new player with current lobby state
	lobbyState := m.GetLobbyStateUnlocked()
	conn.SendWelcome(playerID, "", lobbyState)

	// Broadcast lobby update to all players (including the new one)
	m.broadcastLobbyUpdate()

	return playerID
}

func (m *Matchmaking) SetReady(playerID int, ready bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find player in lobby
	var player *LobbyPlayer
	for _, lp := range m.lobby {
		if lp.PlayerID == playerID {
			player = lp
			break
		}
	}

	if player == nil {
		return false
	}

	player.Ready = ready
	m.broadcastLobbyUpdate()

	// Check if we can start the game
	if len(m.lobby) == 2 {
		allReady := true
		for _, lp := range m.lobby {
			if !lp.Ready {
				allReady = false
				break
			}
		}

		if allReady {
			m.startGame()
			return true
		}
	}

	return false
}

func (m *Matchmaking) startGame() {
	if len(m.lobby) != 2 {
		return
	}

	p1 := m.lobby[0]
	p2 := m.lobby[1]

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

	// Clear lobby
	m.lobby = make([]*LobbyPlayer, 0)

	// Send welcome messages
	p1.Conn.SendWelcome(p1.PlayerID, roomID, nil)
	p2.Conn.SendWelcome(p2.PlayerID, roomID, nil)
}

func (m *Matchmaking) GetLobbyState() *net.LobbyState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.GetLobbyStateUnlocked()
}

func (m *Matchmaking) GetLobbyStateUnlocked() *net.LobbyState {
	log.Printf("GetLobbyState: lobby has %d players", len(m.lobby))
	players := make([]net.LobbyPlayer, len(m.lobby))
	for i, lp := range m.lobby {
		players[i] = net.LobbyPlayer{
			ID:    lp.PlayerID,
			Name:  lp.Name,
			Ready: lp.Ready,
		}
		log.Printf("  Player %d: %s (ready: %v)", lp.PlayerID, lp.Name, lp.Ready)
	}

	state := "waiting"
	if len(players) == 2 {
		allReady := true
		for _, p := range players {
			if !p.Ready {
				allReady = false
				break
			}
		}
		if allReady {
			state = "starting"
		} else {
			state = "ready"
		}
	}

	return &net.LobbyState{
		Players: players,
		State:   state,
	}
}

func (m *Matchmaking) broadcastLobbyUpdate() {
	// This function assumes the lock is already held by the caller
	lobbyState := m.GetLobbyStateUnlocked()
	log.Printf("Broadcasting lobby update: %d players", len(lobbyState.Players))
	for _, lp := range m.lobby {
		if lp.Conn != nil {
			log.Printf("Sending lobby update to player %d (%s)", lp.PlayerID, lp.Name)
			lp.Conn.SendLobbyUpdate(lobbyState)
		}
	}
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

	// Remove from lobby
	for i, lp := range m.lobby {
		if lp.PlayerID == playerID {
			m.lobby = append(m.lobby[:i], m.lobby[i+1:]...)
			m.broadcastLobbyUpdate()
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
