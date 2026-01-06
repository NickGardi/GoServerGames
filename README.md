# 1v1 FPS - Minimal Multiplayer First-Person Shooter

A minimal 1v1 first-person shooter built with Go, featuring an authoritative server and Ebiten-based client with raycast rendering.

## Features

- **1v1 multiplayer** over WebSocket (local dev: ws://, production: wss:// via reverse proxy)
- **Authoritative server** at 60Hz tick rate
- **First-person raycast rendering** (2.5D style)
- **One-shot kill** hitscan combat
- **Round-based gameplay** with 2-second reset after kills
- **Simple visuals**: stick-figure enemies, flat white map, black rectangular cover

## Architecture

### Server (`cmd/server`)
- WebSocket server on `:8080` endpoint `/ws`
- Matchmaking queue (2 players per room)
- Authoritative game simulation
- Hit detection and collision handling

### Client (`cmd/client`)
- Ebiten-based rendering (900x600 window)
- First-person camera with raycast wall rendering
- Billboarded stick-figure enemy rendering
- WebSocket client for server communication

## Building

```bash
# Build server
go build ./cmd/server

# Build client
go build ./cmd/client
```

## Running

### Start the Server

```bash
go run ./cmd/server
# Server starts on :8080
```

### Start Clients

```bash
# Terminal 1
go run ./cmd/client

# Terminal 2 (or different machine)
go run ./cmd/client
```

For remote play, update `ServerAddr` in `cmd/client/main.go` to point to your server's IP/domain.

## Controls

- **WASD**: Move forward/back/strafe
- **Mouse X**: Horizontal look (yaw)
- **Left Click**: Shoot
- **ESC**: Quit

## Game Rules

- 1v1 only (matchmaking pairs players)
- One-shot kill (hitscan)
- Round resets 2 seconds after a kill
- Score increments per round win
- Spawn points at opposite corners of the 800x800 map
- No jumping, no crouching, no reloading

## Technical Details

### Networking
- Transport: WebSocket (JSON messages)
- Server tick: 60Hz
- Client sends input only
- Server broadcasts snapshots

### Rendering
- ~120 rays cast across 70° FOV
- Wall rendering via distance-based vertical slices
- Enemy stickmen rendered as billboarded sprites
- Simple distance shading

### Map
- World size: 800x800 units
- Hardcoded walls (4-6 black rectangles)
- Spawn points at (100, 100) and (700, 700)

## Project Structure

```
/cmd
  /server/main.go        # Server entry point
  /client/main.go        # Client entry point
/internal
  /net/protocol.go       # JSON message definitions
  /server/
    matchmaking.go       # Matchmaking queue and room management
    room.go              # (not used, logic in game/sim.go)
    ws.go                # WebSocket connection handling
  /game/
    sim.go               # Game simulation (ticks, movement, shooting)
    collision.go         # Collision detection and raycasting
    raycast.go           # (not used, logic in collision.go)
    map.go               # Map definition (walls, spawns)
  /client/
    netclient.go         # WebSocket client
    renderer.go          # FPS rendering (raycast, enemies)
```

## Limitations & Future Improvements

This is an MVP with intentionally minimal features:

- Mouse look uses simple delta tracking (no cursor locking)
- No client-side prediction/interpolation
- Simple collision resolution
- Basic raycast rendering (no textures, lighting)
- No reconnection handling
- Single room type

Potential improvements:
- Client-side prediction
- Interpolation for smooth movement
- Better mouse look (cursor locking)
- Reconnection handling
- Multiple maps
- Sound effects
- Better UI/HUD

