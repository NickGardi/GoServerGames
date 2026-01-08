package server

import (
	"GoServerGames/internal/game"
	"GoServerGames/internal/net"
	"fmt"
	"log"
	"sync"
	"time"
)

type Matchmaking struct {
	lobby          []*LobbyPlayer
	rooms          map[string]*game.Room
	speedTypeRooms map[string]*game.SpeedTypeRoom
	nextRoomID     int
	nextPlayerID   int
	selectedBy     *net.SelectedBy // Track who selected the game
	mu             sync.Mutex
}

type LobbyPlayer struct {
	PlayerID     int
	Name         string
	Conn         *Connection
	Ready        bool
	SelectedGame string // "speedtype", "game2", "game3", or ""
}

func NewMatchmaking() *Matchmaking {
	return &Matchmaking{
		lobby:          make([]*LobbyPlayer, 0),
		rooms:          make(map[string]*game.Room),
		speedTypeRooms: make(map[string]*game.SpeedTypeRoom),
		nextPlayerID:   1,
		nextRoomID:     1,
	}
}

func (m *Matchmaking) AddPlayer(name string, conn *Connection) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if a player with this name already exists in the lobby (reconnection)
	// If so, remove the old entry first, then we'll create a new one with same ID if needed
	var existingPlayer *LobbyPlayer
	var existingIndex int = -1
	for i, lp := range m.lobby {
		if lp.Name == name {
			existingPlayer = lp
			existingIndex = i
			break
		}
	}
	
	if existingPlayer != nil {
		log.Printf("Player %s reconnecting (was player %d), updating connection", name, existingPlayer.PlayerID)
		
		// Remove old entry
		if existingIndex >= 0 {
			m.lobby = append(m.lobby[:existingIndex], m.lobby[existingIndex+1:]...)
		}
		
		// Create new entry with same ID but new connection
		existingPlayer.Conn = conn
		existingPlayer.Ready = false // Reset ready on reconnect
		existingPlayer.SelectedGame = "" // Clear selected game on reconnect
		
		m.lobby = append(m.lobby, existingPlayer)
		conn.lobbyPlayer = existingPlayer
		conn.playerID = existingPlayer.PlayerID
		
		// Send welcome message to the reconnecting player
		lobbyState := m.GetLobbyStateUnlocked()
		conn.SendWelcome(existingPlayer.PlayerID, "", lobbyState)
		
		// Broadcast lobby update
		m.broadcastLobbyUpdateUnlocked()
		
		// Clear selected game tracking if this player had selected it
		if m.selectedBy != nil && m.selectedBy.PlayerID == existingPlayer.PlayerID {
			m.selectedBy = nil
			// Clear from other players too
			for _, lp := range m.lobby {
				lp.SelectedGame = ""
				lp.Ready = false
			}
		}
		
		return existingPlayer.PlayerID
	}

	// Only allow 2 players max in the lobby
	if len(m.lobby) >= 2 {
		log.Printf("Lobby is full (2 players: %s and %s), rejecting new player: %s", 
			m.lobby[0].Name, m.lobby[1].Name, name)
		// Send error message (though the connection will just not receive lobby updates)
		return 0
	}

	// Create new player
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
	conn.playerID = playerID

	log.Printf("Added new player %d (%s) to lobby (total: %d)", playerID, name, len(m.lobby))

	// Send welcome message to the new player with current lobby state
	lobbyState := m.GetLobbyStateUnlocked()
	conn.SendWelcome(playerID, "", lobbyState)

	// Broadcast lobby update to all players (including the new one)
	m.broadcastLobbyUpdateUnlocked()

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
	log.Printf("Player %d (%s) ready status changed to: %v", playerID, player.Name, ready)
	
	// Broadcast lobby update first
	m.broadcastLobbyUpdate()

	// Check if we can start the game - need 2 players, selected game, and both ready
	if len(m.lobby) == 2 {
		selectedGame := ""
		for _, lp := range m.lobby {
			if lp.SelectedGame != "" {
				selectedGame = lp.SelectedGame
				break
			}
		}
		
		if selectedGame != "" {
			allReady := true
			for _, lp := range m.lobby {
				if !lp.Ready {
					allReady = false
					break
				}
			}

			if allReady {
				log.Printf("All players ready! Starting game: %s", selectedGame)
				m.startSelectedGameUnlocked(selectedGame)
				return true
			} else {
				log.Printf("Not all players ready. Player 1 ready: %v, Player 2 ready: %v", 
					m.lobby[0].Ready, m.lobby[1].Ready)
			}
		} else {
			log.Printf("Game cannot start: No game selected")
		}
	} else {
		log.Printf("Game cannot start: Only %d players in lobby (need 2)", len(m.lobby))
	}

	return false
}

func (m *Matchmaking) SelectGame(playerID int, gameType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find player
	var player *LobbyPlayer
	for _, lp := range m.lobby {
		if lp.PlayerID == playerID {
			player = lp
			break
		}
	}

	if player == nil {
		return
	}

	// Set selected game for all players in lobby
	for _, lp := range m.lobby {
		lp.SelectedGame = gameType
		lp.Ready = false // Reset ready status when game changes
	}

	// Store who selected the game
	m.selectedBy = &net.SelectedBy{
		PlayerID: playerID,
		Name:     player.Name,
	}

	// Broadcast game selection to all players
	for _, lp := range m.lobby {
		if lp.Conn != nil {
			lp.Conn.SendMessage(net.GameSelectedMessage{
				Type:     "gameSelected",
				GameType: gameType,
				PlayerID: playerID,
			})
		}
	}

	// Broadcast lobby update with selected game
	m.broadcastLobbyUpdate()
}

func (m *Matchmaking) startSelectedGame(gameType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startSelectedGameUnlocked(gameType)
}

func (m *Matchmaking) startSelectedGameUnlocked(gameType string) {
	// This function assumes the lock is already held by the caller
	if len(m.lobby) != 2 {
		log.Printf("Cannot start game: expected 2 players, got %d", len(m.lobby))
		return
	}

	p1 := m.lobby[0]
	p2 := m.lobby[1]
	roomID := m.generateRoomID()

	log.Printf("Starting game %s in room %s with players: %s (%d) vs %s (%d)", 
		gameType, roomID, p1.Name, p1.PlayerID, p2.Name, p2.PlayerID)

	switch gameType {
	case "speedtype":
		room := game.NewSpeedTypeRoom(roomID)
		room.AddPlayer(p1.PlayerID, p1.Name)
		room.AddPlayer(p2.PlayerID, p2.Name)

		// Store room and assign to connections BEFORE sending messages
		m.speedTypeRooms[roomID] = room
		p1.Conn.speedTypeRoom = room
		p2.Conn.speedTypeRoom = room

		// Send game start messages BEFORE clearing lobby
		// Send immediately and separately to ensure they get through
		gameStartMsg1 := net.GameStartMessage{
			Type:     "gameStart",
			GameType: gameType,
			RoomID:   roomID,
		}
		gameStartMsg2 := net.GameStartMessage{
			Type:     "gameStart",
			GameType: gameType,
			RoomID:   roomID,
		}
		
		log.Printf("Sending gameStart message to player %d (%s)", p1.PlayerID, p1.Name)
		if p1.Conn != nil {
			p1.Conn.SendMessage(gameStartMsg1)
		}
		
		log.Printf("Sending gameStart message to player %d (%s)", p2.PlayerID, p2.Name)
		if p2.Conn != nil {
			p2.Conn.SendMessage(gameStartMsg2)
		}
		
		// Give messages a moment to be sent before clearing lobby
		time.Sleep(50 * time.Millisecond)

		// Clear selected game tracking and lobby AFTER sending messages
		m.selectedBy = nil
		m.lobby = make([]*LobbyPlayer, 0)

		// Start game loop for Speed Type
		log.Printf("Starting speed type game loop for room %s", roomID)
		go m.startSpeedTypeGame(room, p1, p2)
	default:
		log.Printf("Unknown game type: %s", gameType)
	}
}

func (m *Matchmaking) startSpeedTypeGame(room *game.SpeedTypeRoom, p1, p2 *LobbyPlayer) {
	// Wait a moment for clients to connect
	time.Sleep(1 * time.Second)

	ticker := time.NewTicker(500 * time.Millisecond) // 2Hz update rate - reduced to prevent buffer overflow
	defer ticker.Stop()

	roundTimeout := time.NewTimer(30 * time.Second)
	defer roundTimeout.Stop()

	// Start rounds
	for !room.CheckGameEnd() {
		// Start new round
		room.StartRound()

		// Send initial state
		state := room.GetState()
		if p1.Conn != nil {
			p1.Conn.SendMessage(state)
		}
		if p2.Conn != nil {
			p2.Conn.SendMessage(state)
		}

		roundTimeout.Reset(30 * time.Second)

		// Monitor round progress
		roundComplete := false
		for !roundComplete && !room.CheckGameEnd() {
			select {
			case <-ticker.C:
				// Send periodic state updates only if state has changed
				currentState := room.GetState()
				if p1.Conn != nil {
					p1.Conn.SendMessage(currentState)
				}
				if p2.Conn != nil {
					p2.Conn.SendMessage(currentState)
				}

				// Check if round is complete
				if room.State == "results" {
					roundComplete = true
				}

			case <-roundTimeout.C:
				// Round timeout
				if room.State == "playing" {
					room.State = "ready"
					roundComplete = true
				}
			}
		}

		// Wait for both players to be ready for next round
		if !room.CheckGameEnd() {
			// Wait until both players are ready (they'll click the button)
			log.Printf("Waiting for both players to be ready for next round...")
			readyTicker := time.NewTicker(500 * time.Millisecond)
			readyTimeout := time.NewTimer(120 * time.Second) // Max wait 2 minutes

			for !room.AllReadyForNext() {
				select {
				case <-readyTicker.C:
					// Send current state with ready status
					currentState := room.GetState()
					if p1.Conn != nil {
						p1.Conn.SendMessage(currentState)
					}
					if p2.Conn != nil {
						p2.Conn.SendMessage(currentState)
					}

					// Check if both ready - exit loop
					if room.AllReadyForNext() {
						goto readyComplete
					}
				case <-readyTimeout.C:
					log.Printf("Timeout waiting for players to be ready, starting next round anyway")
					room.ResetReadyForNext()
					goto readyComplete
				}
			}
		readyComplete:
			readyTicker.Stop()
			readyTimeout.Stop()

			// Reset ready status and move to ready state
			room.ResetReadyForNext()
			room.State = "ready"
			log.Printf("Both players ready, starting next round")
		}
	}
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
	selectedGame := ""
	
	for i, lp := range m.lobby {
		players[i] = net.LobbyPlayer{
			ID:    lp.PlayerID,
			Name:  lp.Name,
			Ready: lp.Ready,
		}
		if lp.SelectedGame != "" {
			selectedGame = lp.SelectedGame
		}
		log.Printf("  Player %d: %s (ready: %v)", lp.PlayerID, lp.Name, lp.Ready)
	}

	state := "waiting"
	if len(players) == 2 && selectedGame != "" {
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
		Players:     players,
		State:       state,
		SelectedGame: selectedGame,
		SelectedBy:   m.selectedBy,
	}
}

func (m *Matchmaking) broadcastSpeedTypeState(room *game.SpeedTypeRoom) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// State is sent from the game loop in startSpeedTypeGame
	// This method exists for potential future use
}

func (m *Matchmaking) broadcastLobbyUpdate() {
	// This function assumes the lock is already held by the caller
	m.broadcastLobbyUpdateUnlocked()
}

func (m *Matchmaking) broadcastLobbyUpdateUnlocked() {
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

func (m *Matchmaking) BroadcastLobbyUpdate() {
	// Public method that acquires the lock
	m.mu.Lock()
	defer m.mu.Unlock()
	m.broadcastLobbyUpdateUnlocked()
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
	m.RemovePlayerUnlocked(playerID)
}

func (m *Matchmaking) RemovePlayerUnlocked(playerID int) {
	// Find and remove player
	var playerName string
	found := false
	for i, lp := range m.lobby {
		if lp.PlayerID == playerID {
			playerName = lp.Name
			m.lobby = append(m.lobby[:i], m.lobby[i+1:]...)
			found = true
			log.Printf("Player %d (%s) removed from lobby, %d players remaining", playerID, playerName, len(m.lobby))
			
			// Clear selected game if the player who selected it left
			if m.selectedBy != nil && m.selectedBy.PlayerID == playerID {
				m.selectedBy = nil
				// Clear selected game from all remaining players
				for _, lp := range m.lobby {
					lp.SelectedGame = ""
					lp.Ready = false
				}
			}
			break
		}
	}
	
	if !found {
		log.Printf("Player %d not found in lobby for removal (might have already been removed)", playerID)
		return
	}
	
	// Broadcast update if player was actually removed
	m.broadcastLobbyUpdateUnlocked()

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
