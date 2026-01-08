package game

import (
	"GoServerGames/internal/net"
	"math/rand"
	"time"
)

// Common words for Speed Type game
var SpeedTypeWords = []string{
	"hello", "world", "quick", "brown", "fox", "jumps", "lazy", "dog",
	"speed", "type", "challenge", "keyboard", "typing", "skill", "test",
	"computer", "mouse", "screen", "keyboard", "button", "click", "enter",
	"practice", "improve", "accuracy", "words", "random", "select", "game",
}

type SpeedTypePlayer struct {
	ID           int
	Name         string
	Score        int
	LastTimeMs   float64
	Connected    bool
	ReadyForNext bool // Ready for next round
}

type SpeedTypeRoom struct {
	ID          string
	Players     [2]*SpeedTypePlayer
	CurrentWord string
	State       string // "waiting", "ready", "playing", "results"
	RoundStartTime time.Time
	Player1SubmitTime float64
	Player2SubmitTime float64
	RoundWinner int
}

func NewSpeedTypeRoom(id string) *SpeedTypeRoom {
	return &SpeedTypeRoom{
		ID:    id,
		State: "waiting",
	}
}

func (r *SpeedTypeRoom) AddPlayer(id int, name string) {
	if r.Players[0] == nil {
		r.Players[0] = &SpeedTypePlayer{
			ID:           id,
			Name:         name,
			Score:        0,
			Connected:    true,
			ReadyForNext: false,
		}
	} else if r.Players[1] == nil {
		r.Players[1] = &SpeedTypePlayer{
			ID:           id,
			Name:         name,
			Score:        0,
			Connected:    true,
			ReadyForNext: false,
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

func (r *SpeedTypeRoom) StartRound() {
	if r.State != "ready" && r.State != "results" {
		return
	}

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

	// Store submission time
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

func (r *SpeedTypeRoom) CheckGameEnd() bool {
	if r.Players[0] != nil && r.Players[0].Score >= 10 {
		return true
	}
	if r.Players[1] != nil && r.Players[1].Score >= 10 {
		return true
	}
	return false
}

