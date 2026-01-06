package game

import "1v1/internal/net"

const (
	WorldSize = 800.0
)

// GetWalls returns the hardcoded map walls
func GetWalls() []net.Wall {
	return []net.Wall{
		// Central cover pieces
		{X: 300, Y: 300, W: 60, H: 20},
		{X: 440, Y: 300, W: 60, H: 20},
		{X: 370, Y: 500, W: 60, H: 20},
		{X: 370, Y: 100, W: 60, H: 20},
		// Side covers
		{X: 150, Y: 200, W: 40, H: 100},
		{X: 610, Y: 500, W: 40, H: 100},
	}
}

// SpawnPoints for players (opposite corners)
var SpawnPoints = []struct {
	X, Y float32
}{
	{X: 100, Y: 100},   // Player 1 spawn
	{X: 700, Y: 700},   // Player 2 spawn
}

