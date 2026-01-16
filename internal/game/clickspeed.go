package game

import (
	"GoServerGames/internal/net"
	"math/rand"
	"time"
)

type ClickTarget struct {
	X      float64 // 0-100 percentage
	Y      float64 // 0-100 percentage
	Radius float64 // pixels
}

func GenerateClickTarget() ClickTarget {
	// Generate random position (10-90% to keep target fully visible)
	return ClickTarget{
		X:      float64(rand.Intn(80) + 10),
		Y:      float64(rand.Intn(80) + 10),
		Radius: 30, // Fixed radius for now
	}
}

type ClickSpeedPlayer struct {
	ID             int
	Name           string
	Score          int
	LastTimeMs     float64
	Connected      bool
	ReadyForNext   bool
	ReadyForNewGame bool
}

type ClickRoundHistory struct {
	RoundNumber   int
	Player1TimeMs float64
	Player2TimeMs float64
	WinnerID      int
	TargetX       float64
	TargetY       float64
}

type ClickSpeedRoom struct {
	ID                string
	RoomCode          string // Room code this game belongs to (for isolation)
	Players           [2]*ClickSpeedPlayer
	CurrentTarget     ClickTarget
	State             string // "waiting", "ready", "playing", "results"
	RoundStartTime    time.Time
	TargetAppearDelayMs int64 // Delay in milliseconds before target appears (server-controlled)
	Player1SubmitTime float64
	Player2SubmitTime float64
	RoundWinner       int
	RoundNumber       int
	RoundHistory      []ClickRoundHistory
	GameEnded         bool
}

func NewClickSpeedRoom(id string, roomCode string) *ClickSpeedRoom {
	return &ClickSpeedRoom{
		ID:           id,
		RoomCode:     roomCode,
		State:        "waiting",
		RoundNumber:  0,
		RoundHistory: make([]ClickRoundHistory, 0),
	}
}

func (r *ClickSpeedRoom) AddPlayer(id int, name string) {
	if r.Players[0] == nil {
		r.Players[0] = &ClickSpeedPlayer{
			ID:        id,
			Name:      name,
			Score:     0,
			Connected: true,
		}
	} else if r.Players[1] == nil {
		r.Players[1] = &ClickSpeedPlayer{
			ID:        id,
			Name:      name,
			Score:     0,
			Connected: true,
		}
		r.State = "ready"
	}
}

func (r *ClickSpeedRoom) ResetReadyForNext() {
	for _, player := range r.Players {
		if player != nil {
			player.ReadyForNext = false
		}
	}
}

func (r *ClickSpeedRoom) StartRound() {
	if r.State != "ready" && r.State != "results" {
		return
	}

	r.RoundNumber++
	r.CurrentTarget = GenerateClickTarget()
	r.State = "playing"
	r.RoundStartTime = time.Now()
	// Random delay between 2000-4000ms (2-4 seconds) for target to appear
	r.TargetAppearDelayMs = int64(2000 + rand.Intn(2000))
	r.Player1SubmitTime = 0
	r.Player2SubmitTime = 0
	r.RoundWinner = 0
}

func (r *ClickSpeedRoom) SubmitClick(playerID int, timeMs float64) bool {
	if r.State != "playing" {
		return false
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

	// Client sends time from when target appeared - use it directly
	// The delay is only for synchronization, NOT part of reaction time
	actualTimeMs := timeMs
	
	// Validate: reasonable reaction time (1-10 seconds)
	if actualTimeMs < 0 || actualTimeMs > 10000 {
		// Invalid time, use a default
		actualTimeMs = 10000
	}

	// Store submission time (actual time from round start)
	if playerIdx == 0 {
		if r.Player1SubmitTime > 0 {
			return false // Already submitted
		}
		r.Player1SubmitTime = actualTimeMs
	} else {
		if r.Player2SubmitTime > 0 {
			return false // Already submitted
		}
		r.Player2SubmitTime = actualTimeMs
	}

	// Check if both players clicked
	if r.Player1SubmitTime > 0 && r.Player2SubmitTime > 0 {
		r.State = "results"
		// Determine winner (faster time wins)
		// RoundWinner = 0 means tie
		if r.Player1SubmitTime < r.Player2SubmitTime {
			r.RoundWinner = r.Players[0].ID
			r.Players[0].Score++
		} else if r.Player2SubmitTime < r.Player1SubmitTime {
			r.RoundWinner = r.Players[1].ID
			r.Players[1].Score++
		} else {
			// Tie - RoundWinner stays 0
			r.RoundWinner = 0
		}

		// Store times for display
		r.Players[0].LastTimeMs = r.Player1SubmitTime
		r.Players[1].LastTimeMs = r.Player2SubmitTime

		// Add to history
		r.RoundHistory = append(r.RoundHistory, ClickRoundHistory{
			RoundNumber:   r.RoundNumber,
			Player1TimeMs: r.Player1SubmitTime,
			Player2TimeMs: r.Player2SubmitTime,
			WinnerID:      r.RoundWinner,
			TargetX:       r.CurrentTarget.X,
			TargetY:       r.CurrentTarget.Y,
		})
	}

	return true
}

func (r *ClickSpeedRoom) GetState() *net.ClickSpeedStateMessage {
	scores := []net.ClickSpeedScore{}

	if r.Players[0] != nil {
		scores = append(scores, net.ClickSpeedScore{
			PlayerID: r.Players[0].ID,
			Name:     r.Players[0].Name,
			Score:    r.Players[0].Score,
			TimeMs:   r.Players[0].LastTimeMs,
		})
	}
	if r.Players[1] != nil {
		scores = append(scores, net.ClickSpeedScore{
			PlayerID: r.Players[1].ID,
			Name:     r.Players[1].Name,
			Score:    r.Players[1].Score,
			TimeMs:   r.Players[1].LastTimeMs,
		})
	}

	msg := &net.ClickSpeedStateMessage{
		Type:              "clickSpeedState",
		TargetX:           r.CurrentTarget.X,
		TargetY:           r.CurrentTarget.Y,
		Radius:            r.CurrentTarget.Radius,
		State:             r.State,
		Scores:            scores,
		TargetAppearDelayMs: int(r.TargetAppearDelayMs),
	}

	if r.State == "results" {
		msg.RoundResult = &net.ClickSpeedResult{
			WinnerID:      r.RoundWinner,
			Player1TimeMs: r.Player1SubmitTime,
			Player2TimeMs: r.Player2SubmitTime,
		}
	}

	return msg
}

func (r *ClickSpeedRoom) CheckGameEnd() bool {
	return r.RoundNumber >= 10 || r.GameEnded
}

func (r *ClickSpeedRoom) GetGameSummary() *ClickGameSummary {
	if len(r.RoundHistory) == 0 {
		return nil
	}

	var p1TotalTime, p2TotalTime float64
	for _, rh := range r.RoundHistory {
		p1TotalTime += rh.Player1TimeMs
		p2TotalTime += rh.Player2TimeMs
	}

	numRounds := float64(len(r.RoundHistory))
	p1AvgTime := p1TotalTime / numRounds
	p2AvgTime := p2TotalTime / numRounds

	var winnerID int
	if r.Players[0] != nil && r.Players[1] != nil {
		if r.Players[0].Score > r.Players[1].Score {
			winnerID = r.Players[0].ID
		} else if r.Players[1].Score > r.Players[0].Score {
			winnerID = r.Players[1].ID
		}
	}

	summary := &ClickGameSummary{
		WinnerID:     winnerID,
		RoundHistory: make([]ClickRoundHistoryData, len(r.RoundHistory)),
	}

	if r.Players[0] != nil {
		summary.Player1ID = r.Players[0].ID
		summary.Player1Name = r.Players[0].Name
		summary.Player1Score = r.Players[0].Score
		summary.Player1AvgTime = p1AvgTime
	}
	if r.Players[1] != nil {
		summary.Player2ID = r.Players[1].ID
		summary.Player2Name = r.Players[1].Name
		summary.Player2Score = r.Players[1].Score
		summary.Player2AvgTime = p2AvgTime
	}

	for i, rh := range r.RoundHistory {
		summary.RoundHistory[i] = ClickRoundHistoryData{
			RoundNumber:   rh.RoundNumber,
			Player1TimeMs: rh.Player1TimeMs,
			Player2TimeMs: rh.Player2TimeMs,
			WinnerID:      rh.WinnerID,
		}
	}

	return summary
}

type ClickGameSummary struct {
	Player1ID      int
	Player1Name    string
	Player1Score   int
	Player1AvgTime float64
	Player2ID      int
	Player2Name    string
	Player2Score   int
	Player2AvgTime float64
	WinnerID       int
	RoundHistory   []ClickRoundHistoryData
}

type ClickRoundHistoryData struct {
	RoundNumber   int
	Player1TimeMs float64
	Player2TimeMs float64
	WinnerID      int
}

