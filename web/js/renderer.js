// Renderer for first-person raycast view
class Renderer {
    constructor(canvas) {
        this.canvas = canvas;
        this.ctx = canvas.getContext('2d');
        this.ScreenWidth = 900;
        this.ScreenHeight = 600;
        this.FOV = 70.0;
        this.RayCount = 120;
        this.PlayerRadius = 24.0; // Doubled to match server
        
        canvas.width = this.ScreenWidth;
        canvas.height = this.ScreenHeight;
    }

    drawFPSView(playerX, playerY, playerYaw, walls, enemies, myPlayerID, scores) {
        // Clear screen (white background)
        this.ctx.fillStyle = 'rgb(255, 255, 255)';
        this.ctx.fillRect(0, 0, this.canvas.width, this.canvas.height);

        const rayAngleStep = this.FOV / this.RayCount;
        const startAngle = playerYaw - this.FOV / 2;
        const WorldSize = 800.0;

        // Cast rays
        for (let i = 0; i < this.RayCount; i++) {
            const rayAngle = startAngle + i * rayAngleStep;
            const rayAngleRad = rayAngle * Math.PI / 180;

            const rayDx = Math.cos(rayAngleRad);
            const rayDy = Math.sin(rayAngleRad);

            // Find nearest wall intersection
            let minDist = 999999;
            let hitWall = null;
            let isBoundary = false;

            // Always check boundary walls first (world edges) - thicker for better coverage
            const boundaries = [
                {x: -10, y: -10, w: WorldSize + 20, h: 10},      // Top
                {x: -10, y: WorldSize, w: WorldSize + 20, h: 10}, // Bottom
                {x: -10, y: -10, w: 10, h: WorldSize + 20},     // Left
                {x: WorldSize, y: -10, w: 10, h: WorldSize + 20}  // Right
            ];

            for (const boundary of boundaries) {
                const dist = this.rayWallDistance(playerX, playerY, rayDx, rayDy, boundary);
                if (dist > 0 && dist < minDist) {
                    minDist = dist;
                    hitWall = boundary;
                    isBoundary = true;
                }
            }

            // Check cover boxes (solid black)
            for (const wall of walls) {
                const dist = this.rayWallDistance(playerX, playerY, rayDx, rayDy, wall);
                if (dist > 0 && dist < minDist) {
                    minDist = dist;
                    hitWall = wall;
                    isBoundary = false;
                }
            }

            // Always draw something - if no wall hit, draw boundary as fallback
            if (!hitWall || minDist >= 999999) {
                // Default boundary wall at max distance
                minDist = 800;
                hitWall = {x: 0, y: 0, w: WorldSize, h: WorldSize};
                isBoundary = true;
            }

            if (hitWall) {
                // Calculate wall height (perspective projection)
                let wallHeight = this.ScreenHeight / (minDist * 0.01);
                if (wallHeight > this.ScreenHeight) {
                    wallHeight = this.ScreenHeight;
                }

                // Draw wall slice - use Math.ceil to ensure no gaps
                const x1 = Math.floor(i * (this.ScreenWidth / this.RayCount));
                const x2 = Math.ceil((i + 1) * (this.ScreenWidth / this.RayCount));
                const sliceWidth = Math.max(1, x2 - x1); // Ensure at least 1px width

                const wallTop = (this.ScreenHeight - wallHeight) / 2;
                const wallBottom = wallTop + wallHeight;
                
                if (isBoundary) {
                    // Boundary walls: solid gray
                    this.ctx.fillStyle = 'rgb(128, 128, 128)';
                    this.ctx.fillRect(x1, wallTop, sliceWidth, wallHeight);
                    
                    // Draw floor below the wall (same gray color)
                    this.ctx.fillStyle = 'rgb(128, 128, 128)';
                    this.ctx.fillRect(x1, wallBottom, sliceWidth, this.ScreenHeight - wallBottom);
                } else {
                    // Cover boxes: solid black
                    this.ctx.fillStyle = 'rgb(0, 0, 0)';
                    this.ctx.fillRect(x1, wallTop, sliceWidth, wallHeight);
                    
                    // Draw floor below cover boxes
                    this.ctx.fillStyle = 'rgb(128, 128, 128)';
                    this.ctx.fillRect(x1, wallBottom, sliceWidth, this.ScreenHeight - wallBottom);
                }
            } else {
                // If no wall hit (shouldn't happen but safety), draw floor and ceiling
                const x1 = Math.floor(i * (this.ScreenWidth / this.RayCount));
                const x2 = Math.ceil((i + 1) * (this.ScreenWidth / this.RayCount));
                const sliceWidth = Math.max(1, x2 - x1);
                
                // Draw floor
                this.ctx.fillStyle = 'rgb(128, 128, 128)';
                this.ctx.fillRect(x1, this.ScreenHeight / 2, sliceWidth, this.ScreenHeight / 2);
            }
        }
        
        // Draw ceiling (white/sky)
        this.ctx.fillStyle = 'rgb(200, 200, 200)'; // Light gray for ceiling
        this.ctx.fillRect(0, 0, this.ScreenWidth, this.ScreenHeight / 2);

        // Draw enemies (stickmen) - draw after walls but before HUD
        for (const enemy of enemies) {
            if (!enemy || enemy.id === myPlayerID || !enemy.alive) {
                continue;
            }
            // Pass enemy object so we can access ID for coloring
            this.drawEnemy(playerX, playerY, playerYaw, enemy.x, enemy.y, walls, enemy);
        }

        // Draw HUD elements last (always on top)
        // Draw score HUD first
        if (scores) {
            this.drawScoreHUD(scores);
        }

        // Draw crosshair last (always visible on top)
        const crosshairSize = 8;
        const crosshairThickness = 2;
        const crosshairX = this.ScreenWidth / 2;
        const crosshairY = this.ScreenHeight / 2;
        this.ctx.strokeStyle = 'rgb(0, 255, 0)'; // Green
        this.ctx.lineWidth = crosshairThickness;
        this.ctx.beginPath();
        // Horizontal line
        this.ctx.moveTo(crosshairX - crosshairSize, crosshairY);
        this.ctx.lineTo(crosshairX + crosshairSize, crosshairY);
        // Vertical line
        this.ctx.moveTo(crosshairX, crosshairY - crosshairSize);
        this.ctx.lineTo(crosshairX, crosshairY + crosshairSize);
        this.ctx.stroke();
    }

    drawScoreHUD(scores) {
        // Draw semi-transparent background
        this.ctx.fillStyle = 'rgba(0, 0, 0, 0.5)';
        this.ctx.fillRect(10, 10, 200, 60);

        // Draw scores
        this.ctx.fillStyle = 'rgb(255, 255, 255)';
        this.ctx.font = 'bold 20px Arial';
        
        let yPos = 35;
        for (const score of scores) {
            this.ctx.fillText(`Player ${score.id}: ${score.score}`, 20, yPos);
            yPos += 25;
        }
    }

    rayWallDistance(rayX, rayY, rayDx, rayDy, wall) {
        let tMin = 0;
        let tMax = 999999;

        if (rayDx !== 0) {
            let t1 = (wall.x - rayX) / rayDx;
            let t2 = ((wall.x + wall.w) - rayX) / rayDx;
            if (t1 > t2) [t1, t2] = [t2, t1];
            if (t1 > tMin) tMin = t1;
            if (t2 < tMax) tMax = t2;
        }

        if (rayDy !== 0) {
            let t1 = (wall.y - rayY) / rayDy;
            let t2 = ((wall.y + wall.h) - rayY) / rayDy;
            if (t1 > t2) [t1, t2] = [t2, t1];
            if (t1 > tMin) tMin = t1;
            if (t2 < tMax) tMax = t2;
        }

        if (tMin < tMax && tMin > 0) {
            const hx = rayX + rayDx * tMin;
            const hy = rayY + rayDy * tMin;

            if (hx >= wall.x && hx <= wall.x + wall.w && hy >= wall.y && hy <= wall.y + wall.h) {
                return tMin;
            }
        }

        return -1;
    }

    drawEnemy(camX, camY, camYaw, enemyX, enemyY, walls, enemyObj) {
        // Vector from camera to enemy
        const dx = enemyX - camX;
        const dy = enemyY - camY;
        const dist = Math.sqrt(dx * dx + dy * dy);

        if (dist <= 0 || dist > 1000) {
            return;
        }

        // Angle to enemy
        let angleToEnemy = Math.atan2(dy, dx) * 180 / Math.PI;
        while (angleToEnemy < 0) angleToEnemy += 360;
        while (angleToEnemy >= 360) angleToEnemy -= 360;

        // Check if enemy is in FOV
        let angleDiff = angleToEnemy - camYaw;
        while (angleDiff > 180) angleDiff -= 360;
        while (angleDiff < -180) angleDiff += 360;

        if (angleDiff < -this.FOV / 2 || angleDiff > this.FOV / 2) {
            return; // Not in FOV
        }

        // Check if occluded by wall
        const enemyAngleRad = angleToEnemy * Math.PI / 180;
        const enemyRayDx = Math.cos(enemyAngleRad);
        const enemyRayDy = Math.sin(enemyAngleRad);

        let wallHit = false;
        for (const wall of walls) {
            const wDist = this.rayWallDistance(camX, camY, enemyRayDx, enemyRayDy, wall);
            if (wDist > 0 && wDist < dist - 5) {
                wallHit = true;
                break;
            }
        }

        if (wallHit) {
            return; // Occluded
        }

        // Project enemy to screen
        const screenX = this.ScreenWidth / 2 + angleDiff * (this.ScreenWidth / this.FOV);
        const screenY = this.ScreenHeight / 2;

        // Scale by distance
        let scale = 100.0 / dist;
        if (scale > 3) scale = 3; // Increased max scale
        if (scale < 0.5) scale = 0.5; // Increased min scale

        // Draw stickman with arms and legs - much bigger
        const bodyHeight = 100 * scale;
        const headRadius = 18 * scale;
        const armLength = 35 * scale;
        const legLength = 40 * scale;
        const shoulderWidth = 20 * scale;
        const hipWidth = 15 * scale;

        // Position player standing on the floor (horizon is at ScreenHeight/2)
        const horizonY = this.ScreenHeight / 2;
        const floorDistance = dist;
        const floorY = horizonY + (100.0 / floorDistance) * 300;
        const groundLevel = Math.min(floorY, this.ScreenHeight - 20);
        
        const bodyY2 = groundLevel; // Feet at floor level
        const bodyY1 = bodyY2 - bodyHeight; // Top of body
        const headY = bodyY1 - headRadius; // Head position
        const neckY = bodyY1; // Where neck meets body
        const shoulderY = neckY + 15 * scale; // Shoulder level
        const waistY = bodyY1 + bodyHeight * 0.6; // Waist level
        const hipY = waistY + 10 * scale; // Hip level

        // Color based on player ID: odd = red, even = blue
        const enemyID = enemyObj ? enemyObj.id : 1;
        const playerColor = (enemyID % 2 === 1) ? 'rgb(255, 0, 0)' : 'rgb(0, 0, 255)';

        this.ctx.strokeStyle = playerColor;
        this.ctx.fillStyle = playerColor;
        this.ctx.lineWidth = 5 * scale;

        // Head (circle)
        this.ctx.beginPath();
        this.ctx.arc(screenX, headY, headRadius, 0, Math.PI * 2);
        this.ctx.fill();

        // Body (vertical line from neck to waist)
        this.ctx.beginPath();
        this.ctx.moveTo(screenX, neckY);
        this.ctx.lineTo(screenX, waistY);
        this.ctx.stroke();

        // Arms (diagonal lines from shoulders)
        const armAngle = 30; // Angle of arms downward
        const armDx = Math.cos(armAngle * Math.PI / 180) * armLength;
        const armDy = Math.sin(armAngle * Math.PI / 180) * armLength;
        
        // Left arm
        this.ctx.beginPath();
        this.ctx.moveTo(screenX - shoulderWidth / 2, shoulderY);
        this.ctx.lineTo(screenX - shoulderWidth / 2 - armDx, shoulderY + armDy);
        this.ctx.stroke();
        
        // Right arm
        this.ctx.beginPath();
        this.ctx.moveTo(screenX + shoulderWidth / 2, shoulderY);
        this.ctx.lineTo(screenX + shoulderWidth / 2 + armDx, shoulderY + armDy);
        this.ctx.stroke();

        // Legs (from hips to feet)
        // Left leg
        this.ctx.beginPath();
        this.ctx.moveTo(screenX - hipWidth / 2, hipY);
        this.ctx.lineTo(screenX - hipWidth / 2, groundLevel);
        this.ctx.stroke();
        
        // Right leg
        this.ctx.beginPath();
        this.ctx.moveTo(screenX + hipWidth / 2, hipY);
        this.ctx.lineTo(screenX + hipWidth / 2, groundLevel);
        this.ctx.stroke();
    }
}

