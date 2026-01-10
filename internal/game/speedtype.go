package game

import (
	"GoServerGames/internal/net"
	"math/rand"
	"time"
)

// Words and phrases for Speed Type game
var SpeedTypeWords = []string{
	// Single words - common but long
	"keyboard", "challenge", "practice", "computer", "accuracy", "improve",
	"important", "question", "business", "development", "information",
	"understand", "something", "everything", "different", "together",
	"beautiful", "wonderful", "fantastic", "amazing", "incredible",
	"adventure", "celebrate", "chocolate", "dangerous", "education",
	"friendship", "happiness", "knowledge", "lightning", "memorable",
	"newspaper", "operation", "perfectly", "questions", "recognize",
	"something", "telephone", "universe", "vegetable", "waterfall",
	"yesterday", "afternoon", "breakfast", "butterfly", "celebrate",
	"community", "dangerous", "education", "excellent", "furniture",
	"generation", "highlight", "important", "journalist", "kilometer",
	// Technical terms (easy spelling)
	"software", "hardware", "network", "computer", "internet", "database",
	"keyboard", "monitor", "download", "password", "username", "homepage",
	"bluetooth", "wireless", "streaming", "download", "upload", "storage",
	// Short phrases (3-5 words)
	"the quick brown fox", "jump over the fence", "hello world program",
	"good morning everyone", "nice to meet you", "have a great day",
	"thank you very much", "see you later", "what time is it",
	"i love coding", "this is amazing", "keep up the work",
	"never give up", "try your best", "you can do it",
	"practice makes perfect", "time to code", "lets get started",
	"ready set go", "on your marks", "time flies fast",
	"break a leg", "fingers crossed", "piece of cake",
	"easy as pie", "hit the road", "under the weather",
	"once upon time", "happily ever after", "the end is near",
	"back to basics", "now or never", "better late than never",
	"actions speak louder", "all in all", "at the moment",
	"by the way", "come what may", "day by day",
	"every now and then", "for the record", "get the ball rolling",
	"in a nutshell", "just in case", "keep in mind",
	"little by little", "make up mind", "no pain no gain",
	"out of sight", "point of view", "quite a few",
	"sooner or later", "take it easy", "up and running",
	"work in progress", "zero to hero", "best of luck",
	// More single words
	"absolutely", "background", "calculation", "decoration", "electronic",
	"foundation", "government", "helicopter", "illustration", "journalism",
	"kindergarten", "laboratory", "mathematics", "neighborhood", "observation",
	"participate", "quarantine", "restaurant", "satisfaction", "technology",
	"underneath", "vocabulary", "watermelon", "xylophone", "yellowstone",
}

type SpeedTypePlayer struct {
	ID           int
	Name         string
	Score        int
	LastTimeMs   float64
	Connected    bool
	ReadyForNext bool // Ready for next round
	ReadyForNewGame bool // Ready to play a new game
}

type RoundHistory struct {
	RoundNumber    int
	Player1TimeMs  float64
	Player2TimeMs  float64
	WinnerID       int
	Word           string
}

type SpeedTypeRoom struct {
	ID          string
	Players     [2]*SpeedTypePlayer
	CurrentWord string
	State       string // "waiting", "ready", "playing", "results", "finished"
	RoundStartTime time.Time
	Player1SubmitTime float64
	Player2SubmitTime float64
	RoundWinner int
	RoundNumber int
	RoundHistory []RoundHistory
	GameEnded   bool
}

func NewSpeedTypeRoom(id string) *SpeedTypeRoom {
	return &SpeedTypeRoom{
		ID:          id,
		State:       "waiting",
		RoundNumber: 0,
		RoundHistory: make([]RoundHistory, 0),
	}
}

func (r *SpeedTypeRoom) AddPlayer(id int, name string) {
	if r.Players[0] == nil {
		r.		Players[0] = &SpeedTypePlayer{
			ID:             id,
			Name:           name,
			Score:          0,
			Connected:      true,
			ReadyForNext:   false,
			ReadyForNewGame: false,
		}
	} else if r.Players[1] == nil {
		r.		Players[1] = &SpeedTypePlayer{
			ID:             id,
			Name:           name,
			Score:          0,
			Connected:      true,
			ReadyForNext:   false,
			ReadyForNewGame: false,
		}
		r.State = "ready"
	}
}

func (r *SpeedTypeRoom) SetReadyForNext(playerID int, ready bool) bool {
	// Find player and set ready status
	for _, player := range r.Players {
		if player != nil && player.ID == playerID {
			player.ReadyForNext = ready
			return true
		}
	}
	return false
}

func (r *SpeedTypeRoom) AllReadyForNext() bool {
	if r.State != "results" {
		return false
	}
	for _, player := range r.Players {
		if player != nil && !player.ReadyForNext {
			return false
		}
	}
	return true
}

func (r *SpeedTypeRoom) ResetReadyForNext() {
	for _, player := range r.Players {
		if player != nil {
			player.ReadyForNext = false
		}
	}
}

func (r *SpeedTypeRoom) SetReadyForNewGame(playerID int, ready bool) bool {
	// Find player and set ready status
	for _, player := range r.Players {
		if player != nil && player.ID == playerID {
			player.ReadyForNewGame = ready
			return true
		}
	}
	return false
}

func (r *SpeedTypeRoom) AllReadyForNewGame() bool {
	for _, player := range r.Players {
		if player != nil && !player.ReadyForNewGame {
			return false
		}
	}
	return true
}

func (r *SpeedTypeRoom) ResetGame() {
	// Reset all game state for a new game
	r.RoundNumber = 0
	r.RoundHistory = make([]RoundHistory, 0)
	r.State = "waiting"
	r.CurrentWord = ""
	r.Player1SubmitTime = 0
	r.Player2SubmitTime = 0
	r.RoundWinner = 0
	
	// Reset player scores and ready status
	for _, player := range r.Players {
		if player != nil {
			player.Score = 0
			player.LastTimeMs = 0
			player.ReadyForNext = false
			player.ReadyForNewGame = false
		}
	}
}

func (r *SpeedTypeRoom) StartRound() {
	if r.State != "ready" && r.State != "results" {
		return
	}

	r.RoundNumber++
	r.CurrentWord = SpeedTypeWords[rand.Intn(len(SpeedTypeWords))]
	r.State = "playing"
	r.RoundStartTime = time.Now()
	r.Player1SubmitTime = 0
	r.Player2SubmitTime = 0
	r.RoundWinner = 0
}

func (r *SpeedTypeRoom) SubmitWord(playerID int, word string, timeMs float64) bool {
	if r.State != "playing" {
		return false
	}

	if word != r.CurrentWord {
		return false // Wrong word
	}

	playerIdx := -1
	if r.Players[0] != nil && r.Players[0].ID == playerID {
		playerIdx = 0
	} else if r.Players[1] != nil && r.Players[1].ID == playerID {
		playerIdx = 1
	}

	if playerIdx == -1 {
		return false
	}

	// Validate client time isn't too far off (allow 1 second network delay tolerance)
	elapsedMs := float64(time.Since(r.RoundStartTime).Milliseconds())
	if timeMs > elapsedMs+1000 {
		// Client time exceeds server elapsed time by more than 1 second - use server time
		timeMs = elapsedMs
	}
	
	// Store submission time (no capping - use actual time)
	if playerIdx == 0 {
		r.Player1SubmitTime = timeMs
	} else {
		r.Player2SubmitTime = timeMs
	}

	// Check if both players submitted
	if r.Player1SubmitTime > 0 && r.Player2SubmitTime > 0 {
		r.State = "results"
		// Determine winner (faster time wins)
		if r.Player1SubmitTime < r.Player2SubmitTime {
			r.RoundWinner = r.Players[0].ID
			r.Players[0].Score++
			r.Players[0].LastTimeMs = r.Player1SubmitTime
		} else {
			r.RoundWinner = r.Players[1].ID
			r.Players[1].Score++
			r.Players[1].LastTimeMs = r.Player2SubmitTime
		}
		// Record round history
		r.recordRoundHistory()
	}

	return true
}


func (r *SpeedTypeRoom) GetState() *net.SpeedTypeStateMessage {
	scores := []net.SpeedTypeScore{}
	
	if r.Players[0] != nil {
		scores = append(scores, net.SpeedTypeScore{
			PlayerID: r.Players[0].ID,
			Name:     r.Players[0].Name,
			Score:    r.Players[0].Score,
			TimeMs:   r.Players[0].LastTimeMs,
		})
	}
	if r.Players[1] != nil {
		scores = append(scores, net.SpeedTypeScore{
			PlayerID: r.Players[1].ID,
			Name:     r.Players[1].Name,
			Score:    r.Players[1].Score,
			TimeMs:   r.Players[1].LastTimeMs,
		})
	}

	msg := &net.SpeedTypeStateMessage{
		Type:   "speedTypeState",
		Word:   r.CurrentWord,
		State:  r.State,
		Scores: scores,
	}

	if r.State == "results" {
		msg.RoundResult = &net.SpeedTypeResult{
			WinnerID:      r.RoundWinner,
			Player1TimeMs: r.Player1SubmitTime,
			Player2TimeMs: r.Player2SubmitTime,
		}
		
		// Include ready status for next round
		readyStatus := []net.ReadyStatus{}
		if r.Players[0] != nil {
			readyStatus = append(readyStatus, net.ReadyStatus{
				PlayerID: r.Players[0].ID,
				Ready:    r.Players[0].ReadyForNext,
			})
		}
		if r.Players[1] != nil {
			readyStatus = append(readyStatus, net.ReadyStatus{
				PlayerID: r.Players[1].ID,
				Ready:    r.Players[1].ReadyForNext,
			})
		}
		msg.ReadyStatus = readyStatus
	}

	return msg
}

func (r *SpeedTypeRoom) recordRoundHistory() {
	history := RoundHistory{
		RoundNumber:   r.RoundNumber,
		Player1TimeMs: r.Player1SubmitTime,
		Player2TimeMs: r.Player2SubmitTime,
		WinnerID:      r.RoundWinner,
		Word:          r.CurrentWord,
	}
	r.RoundHistory = append(r.RoundHistory, history)
}

func (r *SpeedTypeRoom) CheckGameEnd() bool {
	// Game ends after 10 rounds or if explicitly marked as ended
	return r.RoundNumber >= 10 || r.GameEnded
}

func (r *SpeedTypeRoom) GetGameSummary() *GameSummary {
	if len(r.RoundHistory) == 0 {
		return nil
	}

	// Calculate average times
	var player1TotalTime, player2TotalTime float64
	var player1Rounds, player2Rounds int

	for _, round := range r.RoundHistory {
		if round.Player1TimeMs > 0 {
			player1TotalTime += round.Player1TimeMs
			player1Rounds++
		}
		if round.Player2TimeMs > 0 {
			player2TotalTime += round.Player2TimeMs
			player2Rounds++
		}
	}

	player1AvgTime := 0.0
	if player1Rounds > 0 {
		player1AvgTime = player1TotalTime / float64(player1Rounds)
	}

	player2AvgTime := 0.0
	if player2Rounds > 0 {
		player2AvgTime = player2TotalTime / float64(player2Rounds)
	}

	// Determine winner
	winnerID := 0
	if r.Players[0] != nil && r.Players[1] != nil {
		if r.Players[0].Score > r.Players[1].Score {
			winnerID = r.Players[0].ID
		} else if r.Players[1].Score > r.Players[0].Score {
			winnerID = r.Players[1].ID
		}
	}

	return &GameSummary{
		Player1ID:      r.Players[0].ID,
		Player1Name:    r.Players[0].Name,
		Player1Score:   r.Players[0].Score,
		Player1AvgTime: player1AvgTime,
		Player2ID:      r.Players[1].ID,
		Player2Name:    r.Players[1].Name,
		Player2Score:   r.Players[1].Score,
		Player2AvgTime: player2AvgTime,
		WinnerID:       winnerID,
		RoundHistory:   r.RoundHistory,
	}
}

type GameSummary struct {
	Player1ID      int
	Player1Name    string
	Player1Score   int
	Player1AvgTime float64
	Player2ID      int
	Player2Name    string
	Player2Score   int
	Player2AvgTime float64
	WinnerID       int
	RoundHistory   []RoundHistory
}

