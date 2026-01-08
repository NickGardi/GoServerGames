package game

import (
	"GoServerGames/internal/net"
	"math"
	"time"
)

const (
	MoveSpeed     = 450.0 // units per second - much faster
	TickRate      = 60
	TickDuration  = time.Second / TickRate
	RespawnDelayMs = 1000 // 1 second respawn delay
	FireRateLimit = 3.0 // shots per second
	MinFireDelay  = time.Second / time.Duration(FireRateLimit)
	ShootRange    = 1000.0
)

type Player struct {
	ID          int
	Name        string
	X, Y        float32
	Yaw         float32
	Alive       bool
	Score       int
	LastShot    time.Time
	Connected   bool
}

type RoundStateEnum string

const (
	RoundWaiting RoundStateEnum = "waiting"
	RoundPlaying RoundStateEnum = "playing"
	RoundEnded   RoundStateEnum = "ended"
)

type Room struct {
	ID            string
	Players       [2]*Player
	Walls         []net.Wall
	Tick          uint32
	RoundState    RoundStateEnum
	WinnerID      int
	ResetTimer    time.Time
	InputQueues   [2][]net.InputMessage
	LastTickTime  time.Time
	RespawnTimers [2]time.Time // Individual respawn timers per player
}

func NewRoom(id string) *Room {
	walls := GetWalls()
	return &Room{
		ID:          id,
		Walls:       walls,
		RoundState:  RoundWaiting,
		InputQueues: [2][]net.InputMessage{},
		LastTickTime: time.Now(),
	}
}

func (r *Room) AddPlayer(id int, name string) {
	if r.Players[0] == nil {
		r.Players[0] = &Player{
			ID:        id,
			Name:      name,
			X:         SpawnPoints[0].X,
			Y:         SpawnPoints[0].Y,
			Yaw:       45,
			Alive:     true,
			Connected: true,
		}
	} else if r.Players[1] == nil {
		r.Players[1] = &Player{
			ID:        id,
			Name:      name,
			X:         SpawnPoints[1].X,
			Y:         SpawnPoints[1].Y,
			Yaw:       225,
			Alive:     true,
			Connected: true,
		}
		r.RoundState = RoundPlaying
		r.ResetTimer = time.Time{} // No reset needed
	}
}

func (r *Room) QueueInput(playerIdx int, input net.InputMessage) {
	if playerIdx >= 0 && playerIdx < 2 && r.Players[playerIdx] != nil {
		r.InputQueues[playerIdx] = append(r.InputQueues[playerIdx], input)
	}
}

func (r *Room) ProcessTick() {
	now := time.Now()
	if now.Sub(r.LastTickTime) < TickDuration {
		return
	}
	r.LastTickTime = now

	// Process respawn timers for dead players
	for i := 0; i < 2; i++ {
		if r.Players[i] != nil && !r.Players[i].Alive && !r.RespawnTimers[i].IsZero() {
			elapsed := time.Since(r.RespawnTimers[i])
			if elapsed >= RespawnDelayMs*time.Millisecond {
				r.RespawnPlayer(i)
			}
		}
	}

	if r.RoundState != RoundPlaying {
		return
	}

	// Process inputs and movement
	for i := 0; i < 2; i++ {
		if r.Players[i] == nil || !r.Players[i].Alive {
			continue
		}

		// Get latest input
		var input net.InputMessage
		if len(r.InputQueues[i]) > 0 {
			input = r.InputQueues[i][len(r.InputQueues[i])-1]
			// Clear queue (could use proper input reconciliation later)
			r.InputQueues[i] = r.InputQueues[i][:0]
		}

		// Apply yaw
		r.Players[i].Yaw += input.YawDelta
		for r.Players[i].Yaw < 0 {
			r.Players[i].Yaw += 360
		}
		for r.Players[i].Yaw >= 360 {
			r.Players[i].Yaw -= 360
		}

		// Calculate movement
		dx := float32(0)
		dy := float32(0)

		if input.Up {
			yawRad := r.Players[i].Yaw * math.Pi / 180
			dx += float32(math.Cos(float64(yawRad))) * MoveSpeed * float32(TickDuration.Seconds())
			dy += float32(math.Sin(float64(yawRad))) * MoveSpeed * float32(TickDuration.Seconds())
		}
		if input.Down {
			yawRad := r.Players[i].Yaw * math.Pi / 180
			dx -= float32(math.Cos(float64(yawRad))) * MoveSpeed * float32(TickDuration.Seconds())
			dy -= float32(math.Sin(float64(yawRad))) * MoveSpeed * float32(TickDuration.Seconds())
		}
		if input.Left {
			yawRad := (r.Players[i].Yaw - 90) * math.Pi / 180
			dx += float32(math.Cos(float64(yawRad))) * MoveSpeed * float32(TickDuration.Seconds())
			dy += float32(math.Sin(float64(yawRad))) * MoveSpeed * float32(TickDuration.Seconds())
		}
		if input.Right {
			yawRad := (r.Players[i].Yaw + 90) * math.Pi / 180
			dx += float32(math.Cos(float64(yawRad))) * MoveSpeed * float32(TickDuration.Seconds())
			dy += float32(math.Sin(float64(yawRad))) * MoveSpeed * float32(TickDuration.Seconds())
		}

		// Apply movement
		newX := r.Players[i].X + dx
		newY := r.Players[i].Y + dy

		// Boundary check
		if newX < PlayerRadius {
			newX = PlayerRadius
		}
		if newX > WorldSize-PlayerRadius {
			newX = WorldSize - PlayerRadius
		}
		if newY < PlayerRadius {
			newY = PlayerRadius
		}
		if newY > WorldSize-PlayerRadius {
			newY = WorldSize - PlayerRadius
		}

		// Collision check
		if !CheckWallCollision(newX, newY, PlayerRadius, r.Walls) {
			r.Players[i].X = newX
			r.Players[i].Y = newY
		} else {
			// Try to resolve
			resolvedX, resolvedY := ResolveCollision(newX, newY, PlayerRadius, r.Walls)
			r.Players[i].X = resolvedX
			r.Players[i].Y = resolvedY
		}

		// Handle shooting
		if input.Shoot {
			timeSinceLastShot := time.Since(r.Players[i].LastShot)
			if timeSinceLastShot >= MinFireDelay {
				r.Players[i].LastShot = now
				r.ProcessShoot(i)
			}
		}
	}

	r.Tick++
}

func (r *Room) ProcessShoot(shooterIdx int) {
	shooter := r.Players[shooterIdx]
	if !shooter.Alive {
		return
	}

	// Calculate ray direction
	yawRad := shooter.Yaw * math.Pi / 180
	rayDx := float32(math.Cos(float64(yawRad)))
	rayDy := float32(math.Sin(float64(yawRad)))

	// Find target (other player)
	targetIdx := 1 - shooterIdx
	target := r.Players[targetIdx]
	if target == nil || !target.Alive {
		return
	}

	// Check ray intersection with wall first
	wallHit, _, _, wallDist := RayIntersectsWall(shooter.X, shooter.Y, rayDx, rayDy, r.Walls)
	
	// Check ray intersection with target
	playerHit, playerDist := RayIntersectsCircle(shooter.X, shooter.Y, rayDx, rayDy, target.X, target.Y, PlayerRadius)

	if playerHit && (!wallHit || playerDist < wallDist) && playerDist <= ShootRange {
		// Hit!
		target.Alive = false
		shooter.Score++
		// Set respawn timer for dead player
		targetIdx := 1 - shooterIdx
		r.RespawnTimers[targetIdx] = time.Now()
	}
}

func (r *Room) RespawnPlayer(playerIdx int) {
	if playerIdx < 0 || playerIdx >= 2 || r.Players[playerIdx] == nil {
		return
	}
	
	player := r.Players[playerIdx]
	player.X = SpawnPoints[playerIdx].X
	player.Y = SpawnPoints[playerIdx].Y
	if playerIdx == 0 {
		player.Yaw = 45
	} else {
		player.Yaw = 225
	}
	player.Alive = true
	player.LastShot = time.Time{}
	r.RespawnTimers[playerIdx] = time.Time{} // Clear timer
}

func (r *Room) ResetRound() {
	for i := 0; i < 2; i++ {
		if r.Players[i] != nil && r.Players[i].Connected {
			r.Players[i].X = SpawnPoints[i].X
			r.Players[i].Y = SpawnPoints[i].Y
			if i == 0 {
				r.Players[i].Yaw = 45
			} else {
				r.Players[i].Yaw = 225
			}
			r.Players[i].Alive = true
			r.Players[i].LastShot = time.Time{}
			r.RespawnTimers[i] = time.Time{}
		}
	}
	r.RoundState = RoundPlaying
	r.WinnerID = 0
	r.ResetTimer = time.Time{}
}

func (r *Room) GetSnap() net.SnapMessage {
	players := make([]net.PlayerState, 0, 2)
	for i := 0; i < 2; i++ {
		if r.Players[i] != nil {
			players = append(players, net.PlayerState{
				ID:    r.Players[i].ID,
				X:     r.Players[i].X,
				Y:     r.Players[i].Y,
				Yaw:   r.Players[i].Yaw,
				Alive: r.Players[i].Alive,
				Score: r.Players[i].Score,
			})
		}
	}

	resetInMs := 0
	// No longer using round state for respawn, but keeping for compatibility

	return net.SnapMessage{
		Type:  "snap",
		Tick:  r.Tick,
		Players: players,
		Round: net.RoundState{
			State:     string(r.RoundState),
			WinnerID:  r.WinnerID,
			ResetInMs: resetInMs,
		},
		Walls: r.Walls,
	}
}

