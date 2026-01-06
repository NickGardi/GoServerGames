package game

import "1v1/internal/net"

const PlayerRadius = 12.0

// CheckWallCollision checks if a circle at (x,y) with radius r collides with any wall
func CheckWallCollision(x, y, r float32, walls []net.Wall) bool {
	for _, wall := range walls {
		// AABB vs Circle collision
		closestX := x
		if x < wall.X {
			closestX = wall.X
		} else if x > wall.X+wall.W {
			closestX = wall.X + wall.W
		}

		closestY := y
		if y < wall.Y {
			closestY = wall.Y
		} else if y > wall.Y+wall.H {
			closestY = wall.Y + wall.H
		}

		dx := x - closestX
		dy := y - closestY
		distSq := dx*dx + dy*dy

		if distSq < r*r {
			return true
		}
	}
	return false
}

// ResolveCollision moves the player out of collision
func ResolveCollision(x, y, r float32, walls []net.Wall) (float32, float32) {
	newX, newY := x, y
	
	// Try moving back in X first
	if CheckWallCollision(newX, newY, r, walls) {
		// Try X-1, X+1
		if !CheckWallCollision(x-1, y, r, walls) {
			newX = x - 1
		} else if !CheckWallCollision(x+1, y, r, walls) {
			newX = x + 1
		} else {
			newX = x // Can't resolve X
		}
	}
	
	// Try moving back in Y
	if CheckWallCollision(newX, newY, r, walls) {
		if !CheckWallCollision(newX, y-1, r, walls) {
			newY = y - 1
		} else if !CheckWallCollision(newX, y+1, r, walls) {
			newY = y + 1
		} else {
			newY = y // Can't resolve Y
		}
	}
	
	return newX, newY
}

// RayIntersectsWall checks if a ray from (x,y) in direction (dx,dy) hits a wall
// Returns (hit bool, hitX, hitY, distance)
func RayIntersectsWall(rayX, rayY, rayDx, rayDy float32, walls []net.Wall) (bool, float32, float32, float32) {
	minDist := float32(999999)
	hitX, hitY := float32(0), float32(0)
	hit := false

	for _, wall := range walls {
		// Ray-AABB intersection
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

			// Check if hit point is within wall bounds
			if hx >= wall.X && hx <= wall.X+wall.W && hy >= wall.Y && hy <= wall.Y+wall.H {
				dist := tMin
				if dist < minDist {
					minDist = dist
					hitX = hx
					hitY = hy
					hit = true
				}
			}
		}
	}

	return hit, hitX, hitY, minDist
}

// RayIntersectsCircle checks if a ray hits a circle
// Returns (hit bool, distance)
func RayIntersectsCircle(rayX, rayY, rayDx, rayDy, circleX, circleY, radius float32) (bool, float32) {
	dx := circleX - rayX
	dy := circleY - rayY

	// Project circle center onto ray
	t := (dx*rayDx + dy*rayDy) / (rayDx*rayDx + rayDy*rayDy)

	if t < 0 {
		return false, 0
	}

	// Closest point on ray to circle center
	px := rayX + rayDx*t
	py := rayY + rayDy*t

	distToCenter := (px-circleX)*(px-circleX) + (py-circleY)*(py-circleY)
	radSq := radius * radius

	if distToCenter <= radSq {
		// Ray intersects circle
		// Find intersection point (entry point)
		intersectDist := t
		return true, intersectDist
	}

	return false, 0
}

