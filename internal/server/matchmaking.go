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

func (m *Matchmaking) AddPlayer(name string, conn *Connection) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("AddPlayer called for '%s'. speedTypeRooms=%d, mathSprintRooms=%d, clickSpeedRooms=%d, lobby=%d",
		name, len(m.speedTypeRooms), len(m.mathSprintRooms), len(m.clickSpeedRooms), len(m.lobby))

	// Check if player is in an active Speed Type game room (reconnection after redirect)
	for _, room := range m.speedTypeRooms {
		if room.CheckGameEnd() {
			continue
		}
		for _, player := range room.Players {
			if player != nil && player.Name == name {
				log.Printf("Reconnecting player %s (ID %d) to speed type room %s", name, player.ID, room.ID)
				conn.playerID = player.ID
				conn.speedTypeRoom = room
				m.connections[player.ID] = conn
				player.Connected = true
				
				conn.SendWelcome(player.ID, room.ID, nil)
				conn.SendMessage(room.GetState())
				return player.ID
			}
		}
	}

	// Check if player is in an active Math Sprint game room (reconnection after redirect)
	for roomID, room := range m.mathSprintRooms {
		log.Printf("  Checking math sprint room %s: GameEnded=%v, Players=[%v, %v]",
			roomID, room.GameEnded,
			func() string { if room.Players[0] != nil { return room.Players[0].Name } else { return "nil" } }(),
			func() string { if room.Players[1] != nil { return room.Players[1].Name } else { return "nil" } }())
		if room.CheckGameEnd() {
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
	for roomID, room := range m.clickSpeedRooms {
		log.Printf("  Checking click speed room %s: GameEnded=%v, Players=[%v, %v]",
			roomID, room.GameEnded,
			func() string { if room.Players[0] != nil { return room.Players[0].Name } else { return "nil" } }(),
			func() string { if room.Players[1] != nil { return room.Players[1].Name } else { return "nil" } }())
		if room.CheckGameEnd() {
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

	// For lobby: Only allow 2 players max
	if len(m.lobby) >= 2 {
		log.Printf("Lobby is full (2 players), rejecting new player: %s", name)
		return 0
	}

	// Create new player - simple, no reconnection logic
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
	conn.speedTypeRoom = nil
	m.connections[playerID] = conn

	log.Printf("Added new player %d (%s) to lobby (total: %d)", playerID, name, len(m.lobby))

	// Send welcome message with current lobby state
	lobbyState := m.GetLobbyStateUnlocked()
	conn.SendWelcome(playerID, "", lobbyState)
	m.broadcastLobbyUpdateUnlocked()

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
	
	// Send updated lobby state to both players
	lobbyState := m.GetLobbyStateUnlocked()
	for _, lp := range m.lobby {
		if lp.Conn != nil {
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
	log.Printf("Player %d (%s) ready status changed to: %v", playerID, player.Name, ready)
	
	// Broadcast lobby update first
	m.broadcastLobbyUpdate()

	// Check if we can start the game - need 2 players, selected game, and both ready
	log.Printf("Checking if game can start: lobby has %d players", len(m.lobby))
	if len(m.lobby) == 2 {
		selectedGame := ""
		for _, lp := range m.lobby {
			log.Printf("  Player %d (%s): SelectedGame='%s', Ready=%v", lp.PlayerID, lp.Name, lp.SelectedGame, lp.Ready)
			if lp.SelectedGame != "" {
				selectedGame = lp.SelectedGame
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
				log.Printf("Not all players ready. Player 1 (%s) ready: %v, Player 2 (%s) ready: %v", 
					m.lobby[0].Name, m.lobby[0].Ready, m.lobby[1].Name, m.lobby[1].Ready)
			}
		} else {
			log.Printf("Game cannot start: No game selected (selectedBy: %v)", m.selectedBy)
		}
	} else {
		log.Printf("Game cannot start: Only %d players in lobby (need 2)", len(m.lobby))
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

	log.Printf("SelectGame: Found player %d (%s), setting game to %s for all players", playerID, player.Name, gameType)

	// Set selected game for all players in lobby
	for _, lp := range m.lobby {
		lp.SelectedGame = gameType
		lp.Ready = false // Reset ready status when game changes
		log.Printf("  Set player %d (%s): SelectedGame='%s', Ready=false", lp.PlayerID, lp.Name, gameType)
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
	log.Printf("SelectGame: Game selection complete and broadcasted")
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

	// Get connections from the connections map - this is the source of truth
	conn1, ok1 := m.connections[p1.PlayerID]
	conn2, ok2 := m.connections[p2.PlayerID]
	
	if !ok1 || conn1 == nil {
		log.Printf("ERROR: Player 1 (%d) connection not found in map!", p1.PlayerID)
		return
	}
	if !ok2 || conn2 == nil {
		log.Printf("ERROR: Player 2 (%d) connection not found in map!", p2.PlayerID)
		return
	}

	switch gameType {
	case "speedtype":
		room := game.NewSpeedTypeRoom(roomID)
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
		
		conn1.SendMessage(gameStartMsg)
		conn2.SendMessage(gameStartMsg)
		
		time.Sleep(200 * time.Millisecond)
		m.selectedBy = nil
		m.lobby = make([]*LobbyPlayer, 0)

		log.Printf("Starting speed type game for room %s", roomID)
		go m.startSpeedTypeGame(room, p1, p2)

	case "mathsprint":
		room := game.NewMathSprintRoom(roomID)
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
		
		conn1.SendMessage(gameStartMsg)
		conn2.SendMessage(gameStartMsg)
		
		time.Sleep(200 * time.Millisecond)
		m.selectedBy = nil
		m.lobby = make([]*LobbyPlayer, 0)

		log.Printf("Starting math sprint game for room %s", roomID)
		go m.startMathSprintGame(room, p1, p2)

	case "clickspeed":
		room := game.NewClickSpeedRoom(roomID)
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
		
		conn1.SendMessage(gameStartMsg)
		conn2.SendMessage(gameStartMsg)
		
		time.Sleep(200 * time.Millisecond)
		m.selectedBy = nil
		m.lobby = make([]*LobbyPlayer, 0)

		log.Printf("Starting click speed game for room %s", roomID)
		go m.startClickSpeedGame(room, p1, p2)

	default:
		log.Printf("Unknown game type: %s", gameType)
	}
}

func (m *Matchmaking) startSpeedTypeGame(room *game.SpeedTypeRoom, p1, p2 *LobbyPlayer) {
	// Wait for both players to reconnect after redirecting
	time.Sleep(2 * time.Second)

	log.Printf("Game loop starting for room %s", room.ID)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	maxRounds := 2

	// Start rounds
	for round := 1; round <= maxRounds; round++ {
		room.StartRound()
		log.Printf("Started round %d/%d for room %s", round, maxRounds, room.ID)

		// Send round state to all connected players
		m.broadcastSpeedTypeState(room)

		// Wait for both players to submit (state changes to "results")
		for room.State != "results" {
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
	time.Sleep(2 * time.Second)
	
	log.Printf("Math sprint game loop starting for room %s", room.ID)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	maxRounds := 2
	
	for round := 1; round <= maxRounds; round++ {
		room.StartRound()
		log.Printf("Math sprint round %d/%d for room %s", round, maxRounds, room.ID)

		m.broadcastMathSprintState(room)

		for room.State != "results" {
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

	maxRounds := 2
	
	for round := 1; round <= maxRounds; round++ {
		room.StartRound()
		log.Printf("Click speed round %d/%d for room %s", round, maxRounds, room.ID)

		m.broadcastClickSpeedState(room)

		for room.State != "results" {
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

func (m *Matchmaking) RemovePlayer(playerID int, conn *Connection) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// CRITICAL: Only remove from connections if this is the CURRENT connection
	// This prevents old connections from removing new ones after redirect
	if existingConn, ok := m.connections[playerID]; ok && existingConn == conn {
		delete(m.connections, playerID)
		log.Printf("Removed connection for player %d", playerID)
	} else {
		log.Printf("Skipping connection removal for player %d - connection already replaced", playerID)
	}
	
	// Find and remove player from lobby
	for i, lp := range m.lobby {
		if lp.PlayerID == playerID && lp.Conn == conn {
			m.lobby = append(m.lobby[:i], m.lobby[i+1:]...)
			log.Printf("Player %d removed from lobby, %d players remaining", playerID, len(m.lobby))
			
			// Clear selected game if the player who selected it left
			if m.selectedBy != nil && m.selectedBy.PlayerID == playerID {
				m.selectedBy = nil
				for _, lp := range m.lobby {
					lp.SelectedGame = ""
					lp.Ready = false
				}
			}
			
			m.broadcastLobbyUpdateUnlocked()
			break
		}
	}

	// Clean up ended speed type rooms where both players have disconnected
	for roomID, room := range m.speedTypeRooms {
		if room.GameEnded {
			anyConnected := false
			for _, player := range room.Players {
				if player != nil {
					if _, ok := m.connections[player.ID]; ok {
						anyConnected = true
						break
					}
				}
			}
			if !anyConnected {
				delete(m.speedTypeRooms, roomID)
				log.Printf("Cleaned up ended speed type room %s", roomID)
			}
		}
	}
	
	// Clean up ended math sprint rooms where both players have disconnected
	for roomID, room := range m.mathSprintRooms {
		if room.GameEnded {
			anyConnected := false
			for _, player := range room.Players {
				if player != nil {
					if _, ok := m.connections[player.ID]; ok {
						anyConnected = true
				break
					}
				}
			}
			if !anyConnected {
				delete(m.mathSprintRooms, roomID)
				log.Printf("Cleaned up ended math sprint room %s", roomID)
			}
		}
	}
	
	// Clean up ended click speed rooms where both players have disconnected
	for roomID, room := range m.clickSpeedRooms {
		if room.GameEnded {
			anyConnected := false
			for _, player := range room.Players {
				if player != nil {
					if _, ok := m.connections[player.ID]; ok {
						anyConnected = true
						break
					}
				}
			}
			if !anyConnected {
				delete(m.clickSpeedRooms, roomID)
				log.Printf("Cleaned up ended click speed room %s", roomID)
			}
		}
	}
	
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
