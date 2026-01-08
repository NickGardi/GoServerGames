# GoServerGames

A website that uses Go concurrency to play minigames.

## Setup

### Environment Variables
- `GAME_PASSWORD` - **Required** - Password for player login
- `PORT` - Optional - Server port (default: 8080)

### Running

```bash
GAME_PASSWORD=your-secret-password go run ./cmd/server
```

The server will start at `http://localhost:8080`

## How to Play

1. Open your browser to `http://localhost:8080`
2. Login with a username and the password you set in `GAME_PASSWORD`
3. Wait for a second player to join
4. Select a game
5. Both players ready up
6. Game starts automatically when both are ready
