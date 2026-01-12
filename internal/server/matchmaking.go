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
	lobby           []*LobbyPlayer
	rooms           map[string]*game.Room
	speedTypeRooms  map[string]*game.SpeedTypeRoom
	mathSprintRooms map[string]*game.MathSprintRoom
	clickSpeedRooms map[string]*game.ClickSpeedRoom
	connections     map[int]*Connection // Map player ID to active connection
	nextRoomID      int
	nextPlayerID    int
	selectedBy      *net.SelectedBy // Track who selected the game
	mu              sync.Mutex
}

type LobbyPlayer struct {
	PlayerID     int
	Name         string
	RoomCode     string
	Conn         *Connection
	Ready        bool
	SelectedGame string // "speedtype", "game2", "game3", or ""
}

func NewMatchmaking() *Matchmaking {
	return &Matchmaking{
		lobby:           make([]*LobbyPlayer, 0),
		rooms:           make(map[string]*game.Room),
		speedTypeRooms:  make(map[string]*game.SpeedTypeRoom),
		mathSprintRooms: make(map[string]*game.MathSprintRoom),
		clickSpeedRooms: make(map[string]*game.ClickSpeedRoom),
		connections:     make(map[int]*Connection),
		nextPlayerID:    1,
		nextRoomID:      1,
	}
}

// FindPlayerInGameRoom checks if a player with the given name is already in a speed type game room
// Returns the room and player ID if found, nil and 0 otherwise
func (m *Matchmaking) FindPlayerInGameRoom(name string) (*game.SpeedTypeRoom, int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, room := range m.speedTypeRooms {
		for _, player := range room.Players {
			if player != nil && player.Name == name {
				return room, player.ID
			}
		}
	}
	return nil, 0
}

func (m *Matchmaking) AddPlayer(name string, roomCode string, conn *Connection) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate room code is not empty
	if roomCode == "" {
		log.Printf("AddPlayer: Rejected player '%s' - empty room code", name)
		return 0
	}

	log.Printf("AddPlayer called for '%s' in room '%s'. speedTypeRooms=%d, mathSprintRooms=%d, clickSpeedRooms=%d, lobby=%d",
		name, roomCode, len(m.speedTypeRooms), len(m.mathSprintRooms), len(m.clickSpeedRooms), len(m.lobby))
	
	// Clean up empty game rooms first
	m.cleanupEmptyRoomsUnlocked()

	// Check if player is in an active Speed Type game room (reconnection after redirect)
	// Only reconnect if the room code matches - prevents cross-room contamination
	log.Printf("AddPlayer: Checking %d speed type rooms for player %s (room code: %s)", len(m.speedTypeRooms), name, roomCode)
	for roomID, room := range m.speedTypeRooms {
		log.Printf("AddPlayer: Checking speed type room %s: GameEnded=%v, RoomCode=%s, Players=[%v, %v]", 
			roomID, room.CheckGameEnd(), room.RoomCode,
			func() string { if room.Players[0] != nil { return room.Players[0].Name } else { return "nil" } }(),
			func() string { if room.Players[1] != nil { return room.Players[1].Name } else { return "nil" } }())
		if room.CheckGameEnd() {
			continue
		}
		// Only reconnect if room code matches
		if room.RoomCode != roomCode {
			log.Printf("AddPlayer: Skipping room %s - room code mismatch (%s != %s)", roomID, room.RoomCode, roomCode)
			continue
		}
		for _, player := range room.Players {
			if player != nil && player.Name == name {
				log.Printf("AddPlayer: Found player %s (ID %d) in speed type room %s - reconnecting", name, player.ID, room.ID)
				conn.playerID = player.ID
				conn.speedTypeRoom = room
				m.connections[player.ID] = conn
				player.Connected = true
				
				conn.SendWelcome(player.ID, room.ID, nil)
				conn.SendMessage(room.GetState())
				log.Printf("AddPlayer: Successfully reconnected player %s (ID %d) to speed type room %s", name, player.ID, room.ID)
				return player.ID
			}
		}
	}

	// Check if player is in an active Math Sprint game room (reconnection after redirect)
	// Only reconnect if the room code matches - prevents cross-room contamination
	for roomID, room := range m.mathSprintRooms {
		log.Printf("  Checking math sprint room %s: GameEnded=%v, RoomCode=%s, Players=[%v, %v]",
			roomID, room.GameEnded, room.RoomCode,
			func() string { if room.Players[0] != nil { return room.Players[0].Name } else { return "nil" } }(),
			func() string { if room.Players[1] != nil { return room.Players[1].Name } else { return "nil" } }())
		if room.CheckGameEnd() {
			continue
		}
		// Only reconnect if room code matches
		if room.RoomCode != roomCode {
			log.Printf("AddPlayer: Skipping math sprint room %s - room code mismatch (%s != %s)", roomID, room.RoomCode, roomCode)
			continue
		}
		for _, player := range room.Players {
			if player != nil && player.Name == name {
				log.Printf("Reconnecting player %s (ID %d) to math sprint room %s", name, player.ID, room.ID)
				conn.playerID = player.ID
				conn.mathSprintRoom = room
				m.connections[player.ID] = conn
				player.Connected = true
				
				conn.SendWelcome(player.ID, room.ID, nil)
				conn.SendMessage(room.GetState())
				return player.ID
			}
		}
	}

	// Check if player is in an active Click Speed game room (reconnection after redirect)
	// Only reconnect if the room code matches - prevents cross-room contamination
	for roomID, room := range m.clickSpeedRooms {
		log.Printf("  Checking click speed room %s: GameEnded=%v, RoomCode=%s, Players=[%v, %v]",
			roomID, room.GameEnded, room.RoomCode,
			func() string { if room.Players[0] != nil { return room.Players[0].Name } else { return "nil" } }(),
			func() string { if room.Players[1] != nil { return room.Players[1].Name } else { return "nil" } }())
		if room.CheckGameEnd() {
			continue
		}
		// Only reconnect if room code matches
		if room.RoomCode != roomCode {
			log.Printf("AddPlayer: Skipping click speed room %s - room code mismatch (%s != %s)", roomID, room.RoomCode, roomCode)
			continue
		}
		for _, player := range room.Players {
			if player != nil && player.Name == name {
				log.Printf("Reconnecting player %s (ID %d) to click speed room %s", name, player.ID, room.ID)
				conn.playerID = player.ID
				conn.clickSpeedRoom = room
				m.connections[player.ID] = conn
				player.Connected = true
				
				conn.SendWelcome(player.ID, room.ID, nil)
				conn.SendMessage(room.GetState())
				return player.ID
			}
		}
	}

	// Count players in this room code's lobby (not in game)
	playersInRoom := 0
	for _, lp := range m.lobby {
		if lp.RoomCode == roomCode {
			playersInRoom++
		}
	}

	// For lobby: Only allow 2 players max per room code
	if playersInRoom >= 2 {
		log.Printf("Room '%s' lobby is full (2 players), rejecting new player: %s", roomCode, name)
		return 0
	}

	// Create new player
	playerID := m.nextPlayerID
	m.nextPlayerID++

	lp := &LobbyPlayer{
		PlayerID: playerID,
		Name:     name,
		RoomCode: roomCode,
		Conn:     conn,
		Ready:    false,
	}

	m.lobby = append(m.lobby, lp)
	conn.lobbyPlayer = lp
	conn.playerID = playerID
	conn.speedTypeRoom = nil
	m.connections[playerID] = conn

	log.Printf("Added new player %d (%s) to room '%s' lobby (total: %d)", playerID, name, roomCode, len(m.lobby))

	// Send welcome message with current lobby state (filtered by room code)
	lobbyState := m.GetLobbyStateUnlocked(roomCode)
	conn.SendWelcome(playerID, "", lobbyState)
	m.broadcastLobbyUpdateUnlocked(roomCode)

	return playerID
}

// checkAndResetLobbyAfterGameUnlocked checks if both players from an ended game room are in the lobby
// and resets their IDs to 1 and 2 if so. Returns the player ID if reset happened, 0 otherwise.
// This function assumes the lock is already held.
func (m *Matchmaking) checkAndResetLobbyAfterGameUnlocked(currentPlayerName string, endedGameRoom *game.SpeedTypeRoom) int {
	// Only check if we have an ended room and exactly 2 players in lobby
	if endedGameRoom == nil || len(m.lobby) != 2 {
		return 0
	}
	
	// Count how many players were in the ended room
	var roomPlayerCount int = 0
	for _, roomPlayer := range endedGameRoom.Players {
		if roomPlayer != nil {
			roomPlayerCount++
		}
	}
	
	// Only proceed if the ended room had exactly 2 players (a valid game room)
	if roomPlayerCount != 2 {
		return 0
	}
	
	// Find both players from the ended room in the lobby
	var p1FromRoom, p2FromRoom *LobbyPlayer
	var foundCount int = 0
	for _, roomPlayer := range endedGameRoom.Players {
		if roomPlayer != nil {
			// Find this player in the lobby by name
			for _, lobbyPlayer := range m.lobby {
				if lobbyPlayer.Name == roomPlayer.Name {
					if p1FromRoom == nil {
						p1FromRoom = lobbyPlayer
					} else {
						p2FromRoom = lobbyPlayer
					}
					foundCount++
					break
				}
			}
		}
	}
	
	// Only reset if BOTH players from the ended room are in the lobby
	if p1FromRoom == nil || p2FromRoom == nil || foundCount != 2 {
		return 0
	}
	
	log.Printf("RESET: Both players from ended room %s are in lobby - resetting IDs to 1 and 2", endedGameRoom.ID)
	
	// Remove old connections from map
	oldID1 := p1FromRoom.PlayerID
	oldID2 := p2FromRoom.PlayerID
	if oldID1 != 1 {
		delete(m.connections, oldID1)
	}
	if oldID2 != 2 {
		delete(m.connections, oldID2)
	}
	
	// CRITICAL: Verify connections exist before resetting
	if p1FromRoom.Conn == nil || p2FromRoom.Conn == nil {
		log.Printf("ERROR: Reset failed - connections are nil!")
		return 0
	}
	
	// Reassign IDs to 1 and 2
	p1FromRoom.PlayerID = 1
	p2FromRoom.PlayerID = 2
	
	// Update connection playerIDs
	p1FromRoom.Conn.playerID = 1
	p2FromRoom.Conn.playerID = 2
	
	// Force sync connections map (lobby is source of truth)
	m.connections[1] = p1FromRoom.Conn
	m.connections[2] = p2FromRoom.Conn
	
	// Update all connection references
	p1FromRoom.Conn.lobbyPlayer = p1FromRoom
	p2FromRoom.Conn.lobbyPlayer = p2FromRoom
	p1FromRoom.Conn.speedTypeRoom = nil // Clear game room reference
	p2FromRoom.Conn.speedTypeRoom = nil
	
	// Reset the player ID counter
	m.nextPlayerID = 3
	
	// Clear all state completely
	m.selectedBy = nil
	p1FromRoom.Ready = false
	p1FromRoom.SelectedGame = ""
	p2FromRoom.Ready = false
	p2FromRoom.SelectedGame = ""
	
	// Rebuild lobby with reset IDs
	m.lobby = []*LobbyPlayer{p1FromRoom, p2FromRoom}
	
	log.Printf("RESET COMPLETE: Player 1 (%s) has ID 1, Player 2 (%s) has ID 2", p1FromRoom.Name, p2FromRoom.Name)
	log.Printf("RESET VERIFIED: P1 Conn=%p (ID=%d), P2 Conn=%p (ID=%d)", 
		p1FromRoom.Conn, p1FromRoom.Conn.playerID, 
		p2FromRoom.Conn, p2FromRoom.Conn.playerID)
	
	// Delete the ended game room
	delete(m.speedTypeRooms, endedGameRoom.ID)
	
	// Send updated lobby state to both players (using first player's room code)
	roomCode := p1FromRoom.RoomCode
	lobbyState := m.GetLobbyStateUnlocked(roomCode)
	for _, lp := range m.lobby {
		if lp.Conn != nil && lp.RoomCode == roomCode {
			lp.Conn.SendWelcome(lp.PlayerID, "", lobbyState)
			lp.Conn.SendLobbyUpdate(lobbyState)
		}
	}
	
	// Return the ID assigned to the player that just connected (by name)
	if p1FromRoom.Name == currentPlayerName {
		return p1FromRoom.PlayerID
	} else {
		return p2FromRoom.PlayerID
	}
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
	roomCode := player.RoomCode
	log.Printf("Player %d (%s) in room '%s' ready status changed to: %v", playerID, player.Name, roomCode, ready)
	
	// Broadcast lobby update first
	m.broadcastLobbyUpdateUnlocked(roomCode)

	// Check if we can start the game - need 2 players in same room, selected game, and both ready
	// Count players in this room
	var playersInRoom []*LobbyPlayer
		for _, lp := range m.lobby {
		if lp.RoomCode == roomCode {
			playersInRoom = append(playersInRoom, lp)
		}
	}
	
	log.Printf("Checking if game can start: room '%s' has %d players", roomCode, len(playersInRoom))
	if len(playersInRoom) == 2 {
		selectedGame := ""
		for _, lp := range playersInRoom {
			log.Printf("  Player %d (%s): SelectedGame='%s', Ready=%v", lp.PlayerID, lp.Name, lp.SelectedGame, lp.Ready)
			if lp.SelectedGame != "" {
				selectedGame = lp.SelectedGame
			}
		}
		
		if selectedGame != "" {
		allReady := true
			for _, lp := range playersInRoom {
			if !lp.Ready {
				allReady = false
				break
			}
		}

		if allReady {
				log.Printf("All players in room '%s' ready! Starting game: %s", roomCode, selectedGame)
				m.startSelectedGameUnlocked(selectedGame, roomCode)
			return true
			} else {
				log.Printf("Not all players ready. Player 1 (%s) ready: %v, Player 2 (%s) ready: %v", 
					playersInRoom[0].Name, playersInRoom[0].Ready, playersInRoom[1].Name, playersInRoom[1].Ready)
		}
		} else {
			log.Printf("Game cannot start: No game selected (selectedBy: %v)", m.selectedBy)
		}
	} else {
		log.Printf("Game cannot start: Only %d players in room '%s' (need 2)", len(playersInRoom), roomCode)
	}

	return false
}

func (m *Matchmaking) SelectGame(playerID int, gameType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("SelectGame called: playerID=%d, gameType=%s, lobby has %d players", playerID, gameType, len(m.lobby))

	// Find player
	var player *LobbyPlayer
	for _, lp := range m.lobby {
		if lp.PlayerID == playerID {
			player = lp
			break
		}
	}

	if player == nil {
		log.Printf("SelectGame: Player %d not found in lobby!", playerID)
		return
	}

	roomCode := player.RoomCode
	log.Printf("SelectGame: Found player %d (%s) in room '%s', setting game to %s", playerID, player.Name, roomCode, gameType)

	// Set selected game for all players in the same room
	for _, lp := range m.lobby {
		if lp.RoomCode == roomCode {
			lp.SelectedGame = gameType
			lp.Ready = false // Reset ready status when game changes
			log.Printf("  Set player %d (%s): SelectedGame='%s', Ready=false", lp.PlayerID, lp.Name, gameType)
		}
	}

	// Store who selected the game
	m.selectedBy = &net.SelectedBy{
		PlayerID: playerID,
		Name:     player.Name,
	}

	// Broadcast game selection to players in the same room
	for _, lp := range m.lobby {
		if lp.Conn != nil && lp.RoomCode == roomCode {
			lp.Conn.SendMessage(net.GameSelectedMessage{
				Type:     "gameSelected",
				GameType: gameType,
				PlayerID: playerID,
			})
		}
	}

	// Broadcast lobby update with selected game
	m.broadcastLobbyUpdateUnlocked(roomCode)
	log.Printf("SelectGame: Game selection complete and broadcasted to room '%s'", roomCode)
}

func (m *Matchmaking) startSelectedGame(gameType string, roomCode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startSelectedGameUnlocked(gameType, roomCode)
}

func (m *Matchmaking) startSelectedGameUnlocked(gameType string, roomCode string) {
	// This function assumes the lock is already held by the caller
	// Find the 2 players in this room
	var playersInRoom []*LobbyPlayer
	for _, lp := range m.lobby {
		if lp.RoomCode == roomCode {
			playersInRoom = append(playersInRoom, lp)
		}
	}
	
	if len(playersInRoom) != 2 {
		log.Printf("Cannot start game in room '%s': expected 2 players, got %d", roomCode, len(playersInRoom))
		return
	}

	p1 := playersInRoom[0]
	p2 := playersInRoom[1]
	
	roomID := m.generateRoomID()

	log.Printf("Starting game %s in room %s with players: %s (%d) vs %s (%d)", 
		gameType, roomID, p1.Name, p1.PlayerID, p2.Name, p2.PlayerID)

	// Use connections directly from lobby players - these are guaranteed to be current
	conn1 := p1.Conn
	conn2 := p2.Conn
	
	if conn1 == nil {
		log.Printf("ERROR: Player 1 (%d, %s) connection is nil!", p1.PlayerID, p1.Name)
		return
	}
	if conn2 == nil {
		log.Printf("ERROR: Player 2 (%d, %s) connection is nil!", p2.PlayerID, p2.Name)
		return
	}

	// Update connections map to match (in case of any mismatch)
	m.connections[p1.PlayerID] = conn1
	m.connections[p2.PlayerID] = conn2

	log.Printf("Using connections: P1 (%d, %s) conn=%p, P2 (%d, %s) conn=%p", 
		p1.PlayerID, p1.Name, conn1, p2.PlayerID, p2.Name, conn2)

	switch gameType {
	case "speedtype":
		room := game.NewSpeedTypeRoom(roomID, roomCode)
		room.AddPlayer(p1.PlayerID, p1.Name)
		room.AddPlayer(p2.PlayerID, p2.Name)

		m.speedTypeRooms[roomID] = room
		conn1.speedTypeRoom = room
		conn2.speedTypeRoom = room
		
		for _, player := range room.Players {
			if player != nil {
				player.Connected = true
			}
		}

		gameStartMsg := net.GameStartMessage{
			Type:     "gameStart",
			GameType: gameType,
			RoomID:   roomID,
		}
		
		log.Printf("Sending gameStart to P1 (%d, %s) and P2 (%d, %s)", p1.PlayerID, p1.Name, p2.PlayerID, p2.Name)
		conn1.SendMessage(gameStartMsg)
		conn2.SendMessage(gameStartMsg)
		log.Printf("gameStart messages sent to both players")
		
		time.Sleep(200 * time.Millisecond)
		m.selectedBy = nil
		m.removePlayersFromLobby(roomCode)

		log.Printf("Starting speed type game for room %s", roomID)
		go m.startSpeedTypeGame(room, p1, p2)

	case "mathsprint":
		room := game.NewMathSprintRoom(roomID, roomCode)
		room.AddPlayer(p1.PlayerID, p1.Name)
		room.AddPlayer(p2.PlayerID, p2.Name)

		m.mathSprintRooms[roomID] = room
		conn1.mathSprintRoom = room
		conn2.mathSprintRoom = room
		
		for _, player := range room.Players {
			if player != nil {
				player.Connected = true
			}
		}

		gameStartMsg := net.GameStartMessage{
			Type:     "gameStart",
			GameType: gameType,
			RoomID:   roomID,
		}
		
		log.Printf("Sending gameStart to P1 (%d, %s) and P2 (%d, %s)", p1.PlayerID, p1.Name, p2.PlayerID, p2.Name)
		conn1.SendMessage(gameStartMsg)
		conn2.SendMessage(gameStartMsg)
		log.Printf("gameStart messages sent to both players")
		
		// Broadcast initial ready state so reconnecting players see it
		m.broadcastMathSprintState(room)
		
		time.Sleep(200 * time.Millisecond)
		m.selectedBy = nil
		m.removePlayersFromLobby(roomCode)

		log.Printf("Starting math sprint game for room %s", roomID)
		go m.startMathSprintGame(room, p1, p2)

	case "clickspeed":
		room := game.NewClickSpeedRoom(roomID, roomCode)
		room.AddPlayer(p1.PlayerID, p1.Name)
		room.AddPlayer(p2.PlayerID, p2.Name)

		m.clickSpeedRooms[roomID] = room
		conn1.clickSpeedRoom = room
		conn2.clickSpeedRoom = room
		
		for _, player := range room.Players {
			if player != nil {
				player.Connected = true
			}
		}

		gameStartMsg := net.GameStartMessage{
			Type:     "gameStart",
			GameType: gameType,
			RoomID:   roomID,
		}
		
		log.Printf("Sending gameStart to P1 (%d, %s) and P2 (%d, %s)", p1.PlayerID, p1.Name, p2.PlayerID, p2.Name)
		conn1.SendMessage(gameStartMsg)
		conn2.SendMessage(gameStartMsg)
		log.Printf("gameStart messages sent to both players")
		
		time.Sleep(200 * time.Millisecond)
		m.selectedBy = nil
		m.removePlayersFromLobby(roomCode)

		log.Printf("Starting click speed game for room %s", roomID)
		go m.startClickSpeedGame(room, p1, p2)

	default:
		log.Printf("Unknown game type: %s", gameType)
	}
}

// removePlayersFromLobby removes all players with the given room code from the lobby
func (m *Matchmaking) removePlayersFromLobby(roomCode string) {
	var newLobby []*LobbyPlayer
	for _, lp := range m.lobby {
		if lp.RoomCode != roomCode {
			newLobby = append(newLobby, lp)
		}
	}
	m.lobby = newLobby
	log.Printf("Removed players from room '%s', lobby now has %d players", roomCode, len(m.lobby))
}

func (m *Matchmaking) startSpeedTypeGame(room *game.SpeedTypeRoom, p1, p2 *LobbyPlayer) {
	// Wait for both players to reconnect after redirecting
	time.Sleep(2 * time.Second)

	log.Printf("Game loop starting for room %s", room.ID)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	maxRounds := 5

	// Start rounds
	for round := 1; round <= maxRounds; round++ {
		// Check if room still exists and game hasn't ended
		m.mu.Lock()
		if _, exists := m.speedTypeRooms[room.ID]; !exists || room.GameEnded {
			m.mu.Unlock()
			log.Printf("Game loop exiting: room %s no longer exists or game ended", room.ID)
			return
		}
		
		// Check if there are any active connections before starting round
		activeConns := m.getRoomConnectionsUnlocked(room)
		if len(activeConns) == 0 {
			m.mu.Unlock()
			log.Printf("Game loop exiting: no active connections for room %s", room.ID)
			return
		}
		m.mu.Unlock()
		
		room.StartRound()
		log.Printf("Started round %d/%d for room %s", round, maxRounds, room.ID)

		// Send round state to all connected players
		m.broadcastSpeedTypeState(room)

		// Wait for both players to submit (state changes to "results")
		for room.State != "results" {
			// Check before each broadcast if we should continue
			m.mu.Lock()
			if _, exists := m.speedTypeRooms[room.ID]; !exists || room.GameEnded {
				m.mu.Unlock()
				log.Printf("Game loop exiting: room %s no longer exists or game ended", room.ID)
				return
			}
			
			activeConns := m.getRoomConnectionsUnlocked(room)
			if len(activeConns) == 0 {
				m.mu.Unlock()
				log.Printf("Game loop exiting: no active connections for room %s", room.ID)
				return
			}
			m.mu.Unlock()
			
			<-ticker.C
			m.broadcastSpeedTypeState(room)
		}

		log.Printf("Round %d complete", round)
		
		// Broadcast final results state
		m.broadcastSpeedTypeState(room)

		// If this was the last round, send summary and exit
		if round >= maxRounds {
			log.Printf("Game complete after %d rounds", maxRounds)
			m.sendGameSummary(room)
			return
		}

		// Wait before next round
		time.Sleep(3 * time.Second)
		room.ResetReadyForNext()
		room.State = "ready"
	}
	
	log.Printf("Game loop ended for room %s", room.ID)
}

func (m *Matchmaking) sendGameSummary(room *game.SpeedTypeRoom) {
	summary := room.GetGameSummary()
	if summary == nil {
		log.Printf("ERROR: GetGameSummary returned nil!")
		return
	}
	
	summaryMsg := &net.GameSummaryMessage{
		Type:           "gameSummary",
		Player1ID:      summary.Player1ID,
		Player1Name:    summary.Player1Name,
		Player1Score:   summary.Player1Score,
		Player1AvgTime: summary.Player1AvgTime,
		Player2ID:      summary.Player2ID,
		Player2Name:    summary.Player2Name,
		Player2Score:   summary.Player2Score,
		Player2AvgTime: summary.Player2AvgTime,
		WinnerID:       summary.WinnerID,
		RoundHistory:   make([]net.RoundHistoryData, len(summary.RoundHistory)),
	}
	for i, rh := range summary.RoundHistory {
		summaryMsg.RoundHistory[i] = net.RoundHistoryData{
			RoundNumber:   rh.RoundNumber,
			Player1TimeMs: rh.Player1TimeMs,
			Player2TimeMs: rh.Player2TimeMs,
			WinnerID:      rh.WinnerID,
			Word:          rh.Word,
		}
	}
	
	m.mu.Lock()
	conns := m.getRoomConnectionsUnlocked(room)
	m.mu.Unlock()
	
	log.Printf("Sending game summary to %d connections", len(conns))
	for _, conn := range conns {
		conn.SendMessage(summaryMsg)
	}
	
	// Mark the room as ended - players will click button to leave
	room.GameEnded = true
	log.Printf("Speed type room %s marked as ended", room.ID)
}

// Math Sprint game functions

func (m *Matchmaking) startMathSprintGame(room *game.MathSprintRoom, p1, p2 *LobbyPlayer) {
	// Broadcast initial state periodically while waiting for players to reconnect
	// This ensures reconnecting players get the correct state
	broadcastTicker := time.NewTicker(200 * time.Millisecond)
	stopBroadcasting := make(chan bool)
	go func() {
		for {
				select {
			case <-broadcastTicker.C:
				m.mu.Lock()
				if _, exists := m.mathSprintRooms[room.ID]; !exists || room.GameEnded {
					m.mu.Unlock()
					stopBroadcasting <- true
					return
				}
				activeConns := m.getMathRoomConnectionsUnlocked(room)
				if len(activeConns) > 0 {
					m.broadcastMathSprintStateUnlocked(room)
				}
				m.mu.Unlock()
			case <-stopBroadcasting:
				return
			}
		}
	}()
	
	time.Sleep(2 * time.Second)
	broadcastTicker.Stop()
	stopBroadcasting <- true
	
	log.Printf("Math sprint game loop starting for room %s", room.ID)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	maxRounds := 5
	
	for round := 1; round <= maxRounds; round++ {
		// Check if room still exists and game hasn't ended
		m.mu.Lock()
		if _, exists := m.mathSprintRooms[room.ID]; !exists || room.GameEnded {
			m.mu.Unlock()
			log.Printf("Math sprint game loop exiting: room %s no longer exists or game ended", room.ID)
			return
		}
		
		// Check if there are any active connections before starting round
		activeConns := m.getMathRoomConnectionsUnlocked(room)
		if len(activeConns) == 0 {
			m.mu.Unlock()
			log.Printf("Math sprint game loop exiting: no active connections for room %s", room.ID)
			return
		}
		m.mu.Unlock()
		
		room.StartRound()
		log.Printf("Math sprint round %d/%d for room %s", round, maxRounds, room.ID)

		m.broadcastMathSprintState(room)

		for room.State != "results" {
			// Check before each broadcast if we should continue
			m.mu.Lock()
			if _, exists := m.mathSprintRooms[room.ID]; !exists || room.GameEnded {
				m.mu.Unlock()
				log.Printf("Math sprint game loop exiting: room %s no longer exists or game ended", room.ID)
				return
			}
			
			activeConns := m.getMathRoomConnectionsUnlocked(room)
			if len(activeConns) == 0 {
				m.mu.Unlock()
				log.Printf("Math sprint game loop exiting: no active connections for room %s", room.ID)
				return
			}
			m.mu.Unlock()
			
			<-ticker.C
			m.broadcastMathSprintState(room)
		}

		log.Printf("Math sprint round %d complete", round)
		m.broadcastMathSprintState(room)

		if round >= maxRounds {
			log.Printf("Math sprint game complete after %d rounds", maxRounds)
			m.sendMathGameSummary(room)
			return
		}

		time.Sleep(3 * time.Second)
					room.ResetReadyForNext()
					room.State = "ready"
	}
	
	log.Printf("Math sprint game loop ended for room %s", room.ID)
}

func (m *Matchmaking) sendMathGameSummary(room *game.MathSprintRoom) {
	summary := room.GetGameSummary()
	if summary == nil {
		log.Printf("ERROR: GetGameSummary returned nil for math sprint!")
		return
	}
	
	summaryMsg := &net.MathGameSummaryMessage{
		Type:           "mathGameSummary",
		Player1ID:      summary.Player1ID,
		Player1Name:    summary.Player1Name,
		Player1Score:   summary.Player1Score,
		Player1AvgTime: summary.Player1AvgTime,
		Player2ID:      summary.Player2ID,
		Player2Name:    summary.Player2Name,
		Player2Score:   summary.Player2Score,
		Player2AvgTime: summary.Player2AvgTime,
		WinnerID:       summary.WinnerID,
		RoundHistory:   make([]net.MathRoundHistoryData, len(summary.RoundHistory)),
	}
	for i, rh := range summary.RoundHistory {
		summaryMsg.RoundHistory[i] = net.MathRoundHistoryData{
			RoundNumber:   rh.RoundNumber,
			Player1TimeMs: rh.Player1TimeMs,
			Player2TimeMs: rh.Player2TimeMs,
			WinnerID:      rh.WinnerID,
			Question:      rh.Question,
			Answer:        rh.Answer,
		}
	}
	
	m.mu.Lock()
	conns := m.getMathRoomConnectionsUnlocked(room)
	m.mu.Unlock()
	
	log.Printf("Sending math game summary to %d connections", len(conns))
	for _, conn := range conns {
		conn.SendMessage(summaryMsg)
	}
	
	room.GameEnded = true
	log.Printf("Math sprint room %s marked as ended", room.ID)
}

func (m *Matchmaking) broadcastMathSprintState(room *game.MathSprintRoom) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.broadcastMathSprintStateUnlocked(room)
}

func (m *Matchmaking) broadcastMathSprintStateUnlocked(room *game.MathSprintRoom) {
	state := room.GetState()
	for _, player := range room.Players {
		if player != nil {
			if conn, ok := m.connections[player.ID]; ok && conn != nil {
				if conn.mathSprintRoom == room {
					conn.SendMessage(state)
				}
			}
		}
	}
}

func (m *Matchmaking) getMathRoomConnectionsUnlocked(room *game.MathSprintRoom) []*Connection {
	var conns []*Connection
	for _, player := range room.Players {
		if player != nil {
			if conn, ok := m.connections[player.ID]; ok && conn != nil && conn.mathSprintRoom == room {
				conns = append(conns, conn)
			}
		}
	}
	return conns
}

// Click Speed game functions

func (m *Matchmaking) startClickSpeedGame(room *game.ClickSpeedRoom, p1, p2 *LobbyPlayer) {
	time.Sleep(2 * time.Second)
	
	log.Printf("Click speed game loop starting for room %s", room.ID)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	maxRounds := 5
	
	for round := 1; round <= maxRounds; round++ {
		// Check if room still exists and game hasn't ended
		m.mu.Lock()
		if _, exists := m.clickSpeedRooms[room.ID]; !exists || room.GameEnded {
			m.mu.Unlock()
			log.Printf("Click speed game loop exiting: room %s no longer exists or game ended", room.ID)
			return
		}
		
		// Check if there are any active connections before starting round
		activeConns := m.getClickRoomConnectionsUnlocked(room)
		if len(activeConns) == 0 {
			m.mu.Unlock()
			log.Printf("Click speed game loop exiting: no active connections for room %s", room.ID)
			return
		}
		m.mu.Unlock()
		
		room.StartRound()
		log.Printf("Click speed round %d/%d for room %s", round, maxRounds, room.ID)

		m.broadcastClickSpeedState(room)

		for room.State != "results" {
			// Check before each broadcast if we should continue
			m.mu.Lock()
			if _, exists := m.clickSpeedRooms[room.ID]; !exists || room.GameEnded {
				m.mu.Unlock()
				log.Printf("Click speed game loop exiting: room %s no longer exists or game ended", room.ID)
				return
			}
			
			activeConns := m.getClickRoomConnectionsUnlocked(room)
			if len(activeConns) == 0 {
				m.mu.Unlock()
				log.Printf("Click speed game loop exiting: no active connections for room %s", room.ID)
				return
			}
			m.mu.Unlock()
			
			<-ticker.C
			m.broadcastClickSpeedState(room)
		}

		log.Printf("Click speed round %d complete", round)
		m.broadcastClickSpeedState(room)

		if round >= maxRounds {
			log.Printf("Click speed game complete after %d rounds", maxRounds)
			m.sendClickGameSummary(room)
			return
		}

		time.Sleep(3 * time.Second)
			room.ResetReadyForNext()
			room.State = "ready"
	}
	
	log.Printf("Click speed game loop ended for room %s", room.ID)
}

func (m *Matchmaking) sendClickGameSummary(room *game.ClickSpeedRoom) {
	summary := room.GetGameSummary()
	if summary == nil {
		log.Printf("ERROR: GetGameSummary returned nil for click speed!")
		return
	}
	
	summaryMsg := &net.ClickGameSummaryMessage{
		Type:           "clickGameSummary",
		Player1ID:      summary.Player1ID,
		Player1Name:    summary.Player1Name,
		Player1Score:   summary.Player1Score,
		Player1AvgTime: summary.Player1AvgTime,
		Player2ID:      summary.Player2ID,
		Player2Name:    summary.Player2Name,
		Player2Score:   summary.Player2Score,
		Player2AvgTime: summary.Player2AvgTime,
		WinnerID:       summary.WinnerID,
		RoundHistory:   make([]net.ClickRoundHistoryData, len(summary.RoundHistory)),
	}
	for i, rh := range summary.RoundHistory {
		summaryMsg.RoundHistory[i] = net.ClickRoundHistoryData{
			RoundNumber:   rh.RoundNumber,
			Player1TimeMs: rh.Player1TimeMs,
			Player2TimeMs: rh.Player2TimeMs,
			WinnerID:      rh.WinnerID,
		}
	}
	
	m.mu.Lock()
	conns := m.getClickRoomConnectionsUnlocked(room)
	m.mu.Unlock()
	
	log.Printf("Sending click game summary to %d connections", len(conns))
	for _, conn := range conns {
		conn.SendMessage(summaryMsg)
	}
	
	room.GameEnded = true
	log.Printf("Click speed room %s marked as ended", room.ID)
}

func (m *Matchmaking) broadcastClickSpeedState(room *game.ClickSpeedRoom) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := room.GetState()
	for _, player := range room.Players {
		if player != nil {
			if conn, ok := m.connections[player.ID]; ok && conn != nil {
				if conn.clickSpeedRoom == room {
					conn.SendMessage(state)
				}
			}
		}
	}
}

func (m *Matchmaking) getClickRoomConnectionsUnlocked(room *game.ClickSpeedRoom) []*Connection {
	var conns []*Connection
	for _, player := range room.Players {
		if player != nil {
			if conn, ok := m.connections[player.ID]; ok && conn != nil && conn.clickSpeedRoom == room {
				conns = append(conns, conn)
			}
		}
	}
	return conns
}

func (m *Matchmaking) restartSpeedTypeGame(room *game.SpeedTypeRoom) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	log.Printf("Game ended for room %s - redirecting players to login", room.ID)
	
	// Get both players' connections
	var conns []*Connection
	for _, player := range room.Players {
		if player != nil {
			if conn, ok := m.connections[player.ID]; ok && conn != nil {
				conns = append(conns, conn)
			}
		}
	}
	
	// Delete the game room
	delete(m.speedTypeRooms, room.ID)
	
	// Send redirect message to all players
	redirectMsg := map[string]interface{}{
		"type": "redirect",
		"url":  "/",
	}
	
	for _, conn := range conns {
		conn.SendMessage(redirectMsg)
		// Remove player from connections and lobby
		if conn.playerID > 0 {
			delete(m.connections, conn.playerID)
		}
		// Remove from lobby if present
		for i, lp := range m.lobby {
			if lp.PlayerID == conn.playerID {
				m.lobby = append(m.lobby[:i], m.lobby[i+1:]...)
				break
			}
		}
	}
	
	// Reset everything for next game
	m.lobby = make([]*LobbyPlayer, 0)
	m.nextPlayerID = 1
	m.selectedBy = nil
	
	log.Printf("All players redirected to login. Server reset for next game.")
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

func (m *Matchmaking) GetLobbyState(roomCode string) *net.LobbyState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.GetLobbyStateUnlocked(roomCode)
}

func (m *Matchmaking) GetLobbyStateUnlocked(roomCode string) *net.LobbyState {
	// Filter players by room code
	var filteredPlayers []*LobbyPlayer
	for _, lp := range m.lobby {
		if lp.RoomCode == roomCode {
			filteredPlayers = append(filteredPlayers, lp)
		}
	}
	
	log.Printf("GetLobbyState for room '%s': %d players (total lobby: %d)", roomCode, len(filteredPlayers), len(m.lobby))
	players := make([]net.LobbyPlayer, len(filteredPlayers))
	selectedGame := ""
	
	for i, lp := range filteredPlayers {
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
		Players:      players,
		State:        state,
		SelectedGame: selectedGame,
		SelectedBy:   m.selectedBy,
	}
}

func (m *Matchmaking) broadcastSpeedTypeState(room *game.SpeedTypeRoom) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Send state to all players in the room
	// Only send if connection is still associated with this room
	state := room.GetState()
	for _, player := range room.Players {
		if player != nil {
			if conn, ok := m.connections[player.ID]; ok && conn != nil {
				// Only send if connection is still in this room (not returned to lobby)
				if conn.speedTypeRoom == room {
					conn.SendMessage(state)
				}
			}
		}
	}
	
	// Check if game ended and send summary if needed
	// This handles the case where both players submit quickly and the round ends
	// before the ticker loop can check for game end
	if room.State == "results" && room.CheckGameEnd() {
		log.Printf("Game ended after %d rounds (from broadcastSpeedTypeState). Sending summary...", room.RoundNumber)
		summary := room.GetGameSummary()
		if summary != nil {
			summaryMsg := &net.GameSummaryMessage{
				Type:          "gameSummary",
				Player1ID:      summary.Player1ID,
				Player1Name:   summary.Player1Name,
				Player1Score:  summary.Player1Score,
				Player1AvgTime: summary.Player1AvgTime,
				Player2ID:      summary.Player2ID,
				Player2Name:    summary.Player2Name,
				Player2Score:   summary.Player2Score,
				Player2AvgTime: summary.Player2AvgTime,
				WinnerID:       summary.WinnerID,
				RoundHistory:   make([]net.RoundHistoryData, len(summary.RoundHistory)),
			}
			for i, rh := range summary.RoundHistory {
				summaryMsg.RoundHistory[i] = net.RoundHistoryData{
					RoundNumber:   rh.RoundNumber,
					Player1TimeMs: rh.Player1TimeMs,
					Player2TimeMs: rh.Player2TimeMs,
					WinnerID:      rh.WinnerID,
					Word:          rh.Word,
				}
			}
			conns := m.getRoomConnectionsUnlocked(room)
			log.Printf("Sending game summary to %d connections (from broadcastSpeedTypeState)", len(conns))
			for _, conn := range conns {
				conn.SendMessage(summaryMsg)
			}
		} else {
			log.Printf("ERROR: GetGameSummary returned nil in broadcastSpeedTypeState!")
		}
	}
}

// getRoomConnectionsUnlocked returns active connections for players in a speed type room
// Must be called with lock held
func (m *Matchmaking) getRoomConnectionsUnlocked(room *game.SpeedTypeRoom) []*Connection {
	var conns []*Connection
	for _, player := range room.Players {
		if player != nil {
			if conn, ok := m.connections[player.ID]; ok && conn != nil && conn.speedTypeRoom == room {
				conns = append(conns, conn)
			}
		}
	}
	return conns
}


func (m *Matchmaking) broadcastLobbyUpdateUnlocked(roomCode string) {
	// This function assumes the lock is already held by the caller
	// Only broadcast to players in the same room
	lobbyState := m.GetLobbyStateUnlocked(roomCode)
	log.Printf("Broadcasting lobby update to room '%s': %d players", roomCode, len(lobbyState.Players))
	for _, lp := range m.lobby {
		if lp.Conn != nil && lp.RoomCode == roomCode {
			log.Printf("Sending lobby update to player %d (%s)", lp.PlayerID, lp.Name)
			lp.Conn.SendLobbyUpdate(lobbyState)
		}
	}
}

func (m *Matchmaking) BroadcastLobbyUpdate(roomCode string) {
	// Public method that acquires the lock
	m.mu.Lock()
	defer m.mu.Unlock()
	m.broadcastLobbyUpdateUnlocked(roomCode)
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

// cleanupEmptyRoomsUnlocked removes game rooms that have no active connections
// This function assumes the lock is already held
func (m *Matchmaking) cleanupEmptyRoomsUnlocked() {
	// Clean up speed type rooms that have ended AND have no active players
	// Don't clean up active games even if connections temporarily drop (e.g., during redirect)
	for roomID, room := range m.speedTypeRooms {
		if !room.GameEnded {
			continue // Don't clean up rooms that are still active
		}
		hasActivePlayer := false
		for _, player := range room.Players {
			if player != nil {
				if _, ok := m.connections[player.ID]; ok {
					hasActivePlayer = true
					break
				}
			}
		}
		if !hasActivePlayer {
			delete(m.speedTypeRooms, roomID)
			log.Printf("Cleaned up ended speed type room %s (no active players)", roomID)
		}
	}
	
	// Clean up math sprint rooms that have ended AND have no active players
	// Don't clean up active games even if connections temporarily drop (e.g., during redirect)
	for roomID, room := range m.mathSprintRooms {
		if !room.GameEnded {
			continue // Don't clean up rooms that are still active
		}
		hasActivePlayer := false
		for _, player := range room.Players {
			if player != nil {
				if _, ok := m.connections[player.ID]; ok {
					hasActivePlayer = true
					break
				}
			}
		}
		if !hasActivePlayer {
			delete(m.mathSprintRooms, roomID)
			log.Printf("Cleaned up ended math sprint room %s (no active players)", roomID)
		}
	}
	
	// Clean up click speed rooms that have ended AND have no active players
	// Don't clean up active games even if connections temporarily drop (e.g., during redirect)
	for roomID, room := range m.clickSpeedRooms {
		if !room.GameEnded {
			continue // Don't clean up rooms that are still active
		}
		hasActivePlayer := false
		for _, player := range room.Players {
			if player != nil {
				if _, ok := m.connections[player.ID]; ok {
					hasActivePlayer = true
					break
				}
			}
		}
		if !hasActivePlayer {
			delete(m.clickSpeedRooms, roomID)
			log.Printf("Cleaned up ended click speed room %s (no active players)", roomID)
		}
	}
}

func (m *Matchmaking) RemovePlayer(playerID int, conn *Connection) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clean up empty rooms when a player leaves
	defer m.cleanupEmptyRoomsUnlocked()

	// CRITICAL: Only remove from connections if this is the CURRENT connection
	// This prevents old connections from removing new ones after redirect
	if existingConn, ok := m.connections[playerID]; ok && existingConn == conn {
		delete(m.connections, playerID)
		log.Printf("Removed connection for player %d", playerID)
		
		// Check if this player was in a game room, and mark room as ended if no active connections remain
		// BUT only if the game has actually started (not in "waiting" or "ready" state)
		// This prevents marking rooms as ended during the redirect phase when players temporarily disconnect
		if conn.speedTypeRoom != nil && !conn.speedTypeRoom.GameEnded {
			room := conn.speedTypeRoom
			hasActiveConnections := false
			for _, p := range room.Players {
				if p != nil {
					if _, ok := m.connections[p.ID]; ok {
						hasActiveConnections = true
			break
		}
				}
			}
			// Only mark as ended if no active connections AND game has started (past "waiting"/"ready")
			// This allows players to reconnect after redirect without the room being prematurely ended
			if !hasActiveConnections && room.State != "waiting" && room.State != "ready" {
				room.GameEnded = true
				log.Printf("Marked speed type room %s as ended - all players disconnected (state: %s)", room.ID, room.State)
			} else if !hasActiveConnections {
				log.Printf("Not marking speed type room %s as ended - waiting for reconnection (state: %s)", room.ID, room.State)
			}
		}
		if conn.mathSprintRoom != nil && !conn.mathSprintRoom.GameEnded {
			room := conn.mathSprintRoom
			hasActiveConnections := false
			for _, p := range room.Players {
				if p != nil {
					if _, ok := m.connections[p.ID]; ok {
						hasActiveConnections = true
						break
					}
				}
			}
			// Only mark as ended if no active connections AND game has started (past "waiting"/"ready")
			// This allows players to reconnect after redirect without the room being prematurely ended
			if !hasActiveConnections && room.State != "waiting" && room.State != "ready" {
				room.GameEnded = true
				log.Printf("Marked math sprint room %s as ended - all players disconnected (state: %s)", room.ID, room.State)
			} else if !hasActiveConnections {
				log.Printf("Not marking math sprint room %s as ended - waiting for reconnection (state: %s)", room.ID, room.State)
			}
		}
		if conn.clickSpeedRoom != nil && !conn.clickSpeedRoom.GameEnded {
			room := conn.clickSpeedRoom
			hasActiveConnections := false
			for _, p := range room.Players {
				if p != nil {
					if _, ok := m.connections[p.ID]; ok {
						hasActiveConnections = true
						break
					}
				}
			}
			// Only mark as ended if no active connections AND game has started (past "waiting"/"ready")
			// This allows players to reconnect after redirect without the room being prematurely ended
			if !hasActiveConnections && room.State != "waiting" && room.State != "ready" {
				room.GameEnded = true
				log.Printf("Marked click speed room %s as ended - all players disconnected (state: %s)", room.ID, room.State)
			} else if !hasActiveConnections {
				log.Printf("Not marking click speed room %s as ended - waiting for reconnection (state: %s)", room.ID, room.State)
			}
		}
	} else {
		log.Printf("Skipping connection removal for player %d - connection already replaced", playerID)
	}
	
	// Find and remove player from lobby
	for i, lp := range m.lobby {
		if lp.PlayerID == playerID && lp.Conn == conn {
			roomCode := lp.RoomCode
			m.lobby = append(m.lobby[:i], m.lobby[i+1:]...)
			log.Printf("Player %d removed from lobby (room '%s'), %d players remaining", playerID, roomCode, len(m.lobby))
			
			// Clear selected game if the player who selected it left
			if m.selectedBy != nil && m.selectedBy.PlayerID == playerID {
				m.selectedBy = nil
				for _, remaining := range m.lobby {
					if remaining.RoomCode == roomCode {
						remaining.SelectedGame = ""
						remaining.Ready = false
					}
				}
			}
			
			m.broadcastLobbyUpdateUnlocked(roomCode)
				break
			}
		}

	// Cleanup is now handled by cleanupEmptyRoomsUnlocked() which is deferred
	
	// Reset player IDs only when lobby is empty AND no active game rooms exist
	// This prevents resetting during the game start transition when connections temporarily drop
	activeGames := len(m.speedTypeRooms) + len(m.mathSprintRooms) + len(m.clickSpeedRooms)
	if len(m.lobby) == 0 && len(m.connections) == 0 && activeGames == 0 {
		m.nextPlayerID = 1
		log.Printf("Reset player ID counter to 1")
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
