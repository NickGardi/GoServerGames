package game

import (
	"GoServerGames/internal/net"
	"fmt"
	"math/rand"
	"time"
)

type MathOperation int

const (
	OpAdd MathOperation = iota
	OpSubtract
	OpMultiply
	OpDivide
)

type MathQuestion struct {
	Num1      int
	Num2      int
	Operation MathOperation
	Answer    int
	Display   string
}

func GenerateMathQuestion() MathQuestion {
	// Randomly choose between add/subtract or multiply/divide
	if rand.Intn(2) == 0 {
		// Addition or Subtraction (1 to 999)
		if rand.Intn(2) == 0 {
			// Addition
			num1 := rand.Intn(999) + 1
			num2 := rand.Intn(999) + 1
			return MathQuestion{
				Num1:      num1,
				Num2:      num2,
				Operation: OpAdd,
				Answer:    num1 + num2,
				Display:   fmt.Sprintf("%d + %d", num1, num2),
			}
		} else {
			// Subtraction - ensure positive result
			num1 := rand.Intn(999) + 1
			num2 := rand.Intn(num1) + 1 // num2 <= num1
			return MathQuestion{
				Num1:      num1,
				Num2:      num2,
				Operation: OpSubtract,
				Answer:    num1 - num2,
				Display:   fmt.Sprintf("%d - %d", num1, num2),
			}
		}
	} else {
		// Multiplication or Division (up to 12x12)
		if rand.Intn(2) == 0 {
			// Multiplication
			num1 := rand.Intn(12) + 1
			num2 := rand.Intn(12) + 1
			return MathQuestion{
				Num1:      num1,
				Num2:      num2,
				Operation: OpMultiply,
				Answer:    num1 * num2,
				Display:   fmt.Sprintf("%d × %d", num1, num2),
			}
		} else {
			// Division - ensure clean division
			num2 := rand.Intn(12) + 1
			answer := rand.Intn(12) + 1
			num1 := num2 * answer
			return MathQuestion{
				Num1:      num1,
				Num2:      num2,
				Operation: OpDivide,
				Answer:    answer,
				Display:   fmt.Sprintf("%d ÷ %d", num1, num2),
			}
		}
	}
}

type MathSprintPlayer struct {
	ID             int
	Name           string
	Score          int
	LastTimeMs     float64
	Connected      bool
	ReadyForNext   bool
	ReadyForNewGame bool
}

type MathRoundHistory struct {
	RoundNumber   int
	Player1TimeMs float64
	Player2TimeMs float64
	WinnerID      int
	Question      string
	Answer        int
}

type MathSprintRoom struct {
	ID                string
	RoomCode          string // Room code this game belongs to (for isolation)
	Players           [2]*MathSprintPlayer
	CurrentQuestion   MathQuestion
	State             string // "waiting", "ready", "playing", "results", "finished"
	RoundStartTime    time.Time
	Player1SubmitTime float64
	Player2SubmitTime float64
	RoundWinner       int
	RoundNumber       int
	RoundHistory      []MathRoundHistory
	GameEnded         bool
}

func NewMathSprintRoom(id string, roomCode string) *MathSprintRoom {
	return &MathSprintRoom{
		ID:           id,
		RoomCode:     roomCode,
		State:        "waiting",
		RoundNumber:  0,
		RoundHistory: make([]MathRoundHistory, 0),
	}
}

func (r *MathSprintRoom) AddPlayer(id int, name string) {
	if r.Players[0] == nil {
		r.Players[0] = &MathSprintPlayer{
			ID:        id,
			Name:      name,
			Score:     0,
			Connected: true,
		}
	} else if r.Players[1] == nil {
		r.Players[1] = &MathSprintPlayer{
			ID:        id,
			Name:      name,
			Score:     0,
			Connected: true,
		}
		r.State = "ready"
	}
}

func (r *MathSprintRoom) ResetReadyForNext() {
	for _, player := range r.Players {
		if player != nil {
			player.ReadyForNext = false
		}
	}
}

func (r *MathSprintRoom) StartRound() {
	if r.State != "ready" && r.State != "results" {
		return
	}

	r.RoundNumber++
	r.CurrentQuestion = GenerateMathQuestion()
	r.State = "playing"
	r.RoundStartTime = time.Now()
	r.Player1SubmitTime = 0
	r.Player2SubmitTime = 0
	r.RoundWinner = 0
}

func (r *MathSprintRoom) SubmitAnswer(playerID int, answer int, timeMs float64) bool {
	if r.State != "playing" {
		return false
	}

	if answer != r.CurrentQuestion.Answer {
		return false // Wrong answer
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

	// Validate client time
	elapsedMs := float64(time.Since(r.RoundStartTime).Milliseconds())
	if timeMs > elapsedMs+1000 {
		timeMs = elapsedMs
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
		} else if r.Player2SubmitTime < r.Player1SubmitTime {
			r.RoundWinner = r.Players[1].ID
			r.Players[1].Score++
		}

		// Store times for display
		r.Players[0].LastTimeMs = r.Player1SubmitTime
		r.Players[1].LastTimeMs = r.Player2SubmitTime

		// Add to history
		r.RoundHistory = append(r.RoundHistory, MathRoundHistory{
			RoundNumber:   r.RoundNumber,
			Player1TimeMs: r.Player1SubmitTime,
			Player2TimeMs: r.Player2SubmitTime,
			WinnerID:      r.RoundWinner,
			Question:      r.CurrentQuestion.Display,
			Answer:        r.CurrentQuestion.Answer,
		})
	}

	return true
}

func (r *MathSprintRoom) GetState() *net.MathSprintStateMessage {
	scores := []net.MathSprintScore{}

	if r.Players[0] != nil {
		scores = append(scores, net.MathSprintScore{
			PlayerID: r.Players[0].ID,
			Name:     r.Players[0].Name,
			Score:    r.Players[0].Score,
			TimeMs:   r.Players[0].LastTimeMs,
		})
	}
	if r.Players[1] != nil {
		scores = append(scores, net.MathSprintScore{
			PlayerID: r.Players[1].ID,
			Name:     r.Players[1].Name,
			Score:    r.Players[1].Score,
			TimeMs:   r.Players[1].LastTimeMs,
		})
	}

	msg := &net.MathSprintStateMessage{
		Type:     "mathSprintState",
		Question: r.CurrentQuestion.Display,
		Answer:   r.CurrentQuestion.Answer,
		State:    r.State,
		Scores:   scores,
	}

	if r.State == "results" {
		msg.RoundResult = &net.MathSprintResult{
			WinnerID:      r.RoundWinner,
			Player1TimeMs: r.Player1SubmitTime,
			Player2TimeMs: r.Player2SubmitTime,
			CorrectAnswer: r.CurrentQuestion.Answer,
		}
	}

	return msg
}

func (r *MathSprintRoom) CheckGameEnd() bool {
	return r.RoundNumber >= 10 || r.GameEnded
}

func (r *MathSprintRoom) GetGameSummary() *MathGameSummary {
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

	summary := &MathGameSummary{
		WinnerID:     winnerID,
		RoundHistory: make([]MathRoundHistoryData, len(r.RoundHistory)),
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
		summary.RoundHistory[i] = MathRoundHistoryData{
			RoundNumber:   rh.RoundNumber,
			Player1TimeMs: rh.Player1TimeMs,
			Player2TimeMs: rh.Player2TimeMs,
			WinnerID:      rh.WinnerID,
			Question:      rh.Question,
			Answer:        rh.Answer,
		}
	}

	return summary
}

type MathGameSummary struct {
	Player1ID      int
	Player1Name    string
	Player1Score   int
	Player1AvgTime float64
	Player2ID      int
	Player2Name    string
	Player2Score   int
	Player2AvgTime float64
	WinnerID       int
	RoundHistory   []MathRoundHistoryData
}

type MathRoundHistoryData struct {
	RoundNumber   int
	Player1TimeMs float64
	Player2TimeMs float64
	WinnerID      int
	Question      string
	Answer        int
}

