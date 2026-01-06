package client

import (
	"1v1/internal/net"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	ScreenWidth  = 900
	ScreenHeight = 600
	FOV          = 70.0 // degrees
	RayCount     = 120
	PlayerRadius = 12.0
)

type Renderer struct {
	wallCache map[float32]float32 // distance -> wall height
}

func NewRenderer() *Renderer {
	return &Renderer{
		wallCache: make(map[float32]float32),
	}
}

func (r *Renderer) DrawFPSView(screen *ebiten.Image, playerX, playerY, playerYaw float32, walls []net.Wall, enemies []net.PlayerState, myPlayerID int) {
	// Clear screen (white background for floor/ceiling)
	screen.Fill(color.RGBA{255, 255, 255, 255})

	rayAngleStep := FOV / float32(RayCount)
	startAngle := playerYaw - FOV/2

	// Cast rays
	for i := 0; i < RayCount; i++ {
		rayAngle := startAngle + float32(i)*rayAngleStep
		rayAngleRad := rayAngle * math.Pi / 180

		rayDx := float32(math.Cos(float64(rayAngleRad)))
		rayDy := float32(math.Sin(float64(rayAngleRad)))

		// Find nearest wall intersection
		minDist := float32(999999)
		hitWall := false

		for _, wall := range walls {
			dist := r.rayWallDistance(playerX, playerY, rayDx, rayDy, wall)
			if dist > 0 && dist < minDist {
				minDist = dist
				hitWall = true
			}
		}

		if hitWall {
			// Calculate wall height (perspective projection)
			wallHeight := float32(ScreenHeight) / (minDist * 0.01)
			if wallHeight > ScreenHeight {
				wallHeight = ScreenHeight
			}

			// Draw wall slice (black)
			x1 := float32(i) * (ScreenWidth / RayCount)
			x2 := float32(i+1) * (ScreenWidth / RayCount)

			// Distance shading (darker = closer)
			shade := uint8(255 - minDist*0.3)
			if shade < 50 {
				shade = 50
			}
			c := color.RGBA{shade, shade, shade, 255}

			vector.DrawFilledRect(screen, x1, (ScreenHeight-wallHeight)/2, x2-x1, wallHeight, c, false)
		}
	}

	// Draw enemies (stickmen)
	for _, enemy := range enemies {
		if enemy.ID == myPlayerID || !enemy.Alive {
			continue
		}

		r.drawEnemy(screen, playerX, playerY, playerYaw, enemy.X, enemy.Y, walls)
	}

	// Draw crosshair (center dot)
	crosshairSize := float32(3)
	crosshairX := ScreenWidth / 2
	crosshairY := ScreenHeight / 2
	vector.DrawFilledCircle(screen, float32(crosshairX), float32(crosshairY), crosshairSize, color.RGBA{0, 0, 0, 255}, false)
}

func (r *Renderer) rayWallDistance(rayX, rayY, rayDx, rayDy float32, wall net.Wall) float32 {
	tMin := float32(0)
	tMax := float32(999999)

	if rayDx != 0 {
		t1 := (wall.X - rayX) / rayDx
		t2 := ((wall.X + wall.W) - rayX) / rayDx
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		if t1 > tMin {
			tMin = t1
		}
		if t2 < tMax {
			tMax = t2
		}
	}

	if rayDy != 0 {
		t1 := (wall.Y - rayY) / rayDy
		t2 := ((wall.Y + wall.H) - rayY) / rayDy
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		if t1 > tMin {
			tMin = t1
		}
		if t2 < tMax {
			tMax = t2
		}
	}

	if tMin < tMax && tMin > 0 {
		hx := rayX + rayDx*tMin
		hy := rayY + rayDy*tMin

		if hx >= wall.X && hx <= wall.X+wall.W && hy >= wall.Y && hy <= wall.Y+wall.H {
			return tMin
		}
	}

	return -1
}

func (r *Renderer) drawEnemy(screen *ebiten.Image, camX, camY, camYaw, enemyX, enemyY float32, walls []net.Wall) {
	// Vector from camera to enemy
	dx := enemyX - camX
	dy := enemyY - camY
	dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	if dist <= 0 || dist > 1000 {
		return
	}

	// Angle to enemy
	angleToEnemy := float32(math.Atan2(float64(dy), float64(dx))) * 180 / math.Pi
	for angleToEnemy < 0 {
		angleToEnemy += 360
	}
	for angleToEnemy >= 360 {
		angleToEnemy -= 360
	}

	// Check if enemy is in FOV
	angleDiff := angleToEnemy - camYaw
	for angleDiff > 180 {
		angleDiff -= 360
	}
	for angleDiff < -180 {
		angleDiff += 360
	}

	if angleDiff < -FOV/2 || angleDiff > FOV/2 {
		return // Not in FOV
	}

	// Use angle to enemy for ray
	enemyAngleRad := angleToEnemy * math.Pi / 180
	enemyRayDx := float32(math.Cos(float64(enemyAngleRad)))
	enemyRayDy := float32(math.Sin(float64(enemyAngleRad)))

	wallHit := false
	for _, wall := range walls {
		wDist := r.rayWallDistance(camX, camY, enemyRayDx, enemyRayDy, wall)
		if wDist > 0 && wDist < dist-5 { // Small margin
			wallHit = true
			break
		}
	}

	if wallHit {
		return // Occluded
	}

	// Project enemy to screen
	screenX := ScreenWidth/2 + angleDiff*(ScreenWidth/FOV)
	screenY := ScreenHeight / 2

	// Scale by distance
	scale := 100.0 / dist
	if scale > 2 {
		scale = 2
	}
	if scale < 0.3 {
		scale = 0.3
	}

	// Draw stickman (vertical line + circle for head)
	bodyHeight := 30 * scale
	headRadius := 5 * scale

	// Body (vertical line)
	bodyY1 := float32(screenY) - bodyHeight/2
	bodyY2 := float32(screenY) + bodyHeight/2
	vector.StrokeLine(screen, screenX, bodyY1, screenX, bodyY2, 2*scale, color.RGBA{0, 0, 0, 255}, false)

	// Head (circle)
	headY := bodyY1 - headRadius
	vector.DrawFilledCircle(screen, screenX, headY, headRadius, color.RGBA{0, 0, 0, 255}, false)
}

func (r *Renderer) DrawHUD(screen *ebiten.Image, score int, roundState string, resetInMs int) {
	// Simple text rendering would go here
	// For MVP, we'll skip text and just show visual feedback
}

