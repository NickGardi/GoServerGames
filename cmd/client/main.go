package main

import (
	"1v1/internal/client"
	"1v1/internal/net"
	"fmt"
	"image/color"
	"log"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	ServerAddr = "ws://localhost:8080/ws"
)

type Game struct {
	netClient   *client.NetClient
	renderer    *client.Renderer
	playerID    int
	currentSnap *net.SnapMessage
	inputSeq    uint32
	lastInput   net.InputMessage
	keys        map[ebiten.Key]bool
	lastMouseX  int
	lastYaw     float32
	yaw         float32
}

func NewGame() (*Game, error) {
	netClient, err := client.NewNetClient(ServerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return &Game{
		netClient: netClient,
		renderer:  client.NewRenderer(),
		keys:      make(map[ebiten.Key]bool),
		yaw:       0,
	}, nil
}

func (g *Game) Update() error {
	// Handle welcome
	if welcome := g.netClient.GetWelcome(); welcome != nil {
		g.playerID = welcome.PlayerID
		log.Printf("Connected! PlayerID: %d, RoomID: %s", welcome.PlayerID, welcome.RoomID)
	}

	// Get latest snapshot
	if snap := g.netClient.GetSnapshot(); snap != nil {
		g.currentSnap = snap
		// Sync yaw from server for my player
		for _, player := range snap.Players {
			if player.ID == g.playerID {
				// Only sync if we don't have a local yaw set yet
				if g.lastYaw == 0 && g.yaw == 0 {
					g.yaw = player.Yaw
					g.lastYaw = player.Yaw
				}
				break
			}
		}
	}

	// Handle input
	g.handleInput()

	// ESC to quit
	if ebiten.IsKeyPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}

	return nil
}

func (g *Game) handleInput() {
	// Mouse look - track mouse movement delta
	mx, _ := ebiten.CursorPosition()
	if g.lastMouseX != 0 {
		delta := float32(mx - g.lastMouseX) * 0.15 // Sensitivity
		g.yaw += delta
		for g.yaw < 0 {
			g.yaw += 360
		}
		for g.yaw >= 360 {
			g.yaw -= 360
		}
	}
	g.lastMouseX = mx

	// Keyboard
	input := net.InputMessage{
		Type:         "input",
		Seq:          g.inputSeq,
		Up:           ebiten.IsKeyPressed(ebiten.KeyW),
		Down:         ebiten.IsKeyPressed(ebiten.KeyS),
		Left:         ebiten.IsKeyPressed(ebiten.KeyA),
		Right:        ebiten.IsKeyPressed(ebiten.KeyD),
		YawDelta:     g.yaw - g.lastYaw,
		Shoot:        ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft),
		ClientTimeMs: time.Now().UnixMilli(),
	}

	g.lastYaw = g.yaw
	g.inputSeq++

	if input.Up || input.Down || input.Left || input.Right || input.YawDelta != 0 || input.Shoot {
		g.netClient.SendInput(input)
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.currentSnap == nil {
		// Waiting screen
		screen.Fill(color.RGBA{200, 200, 200, 255})
		return
	}

	// Find my player
	var myPlayer *net.PlayerState
	for i := range g.currentSnap.Players {
		if g.currentSnap.Players[i].ID == g.playerID {
			myPlayer = &g.currentSnap.Players[i]
			break
		}
	}

	if myPlayer == nil {
		screen.Fill(color.RGBA{200, 200, 200, 255})
		return
	}

	// Render FPS view
	g.renderer.DrawFPSView(
		screen,
		myPlayer.X,
		myPlayer.Y,
		myPlayer.Yaw,
		g.currentSnap.Walls,
		g.currentSnap.Players,
		g.playerID,
	)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return client.ScreenWidth, client.ScreenHeight
}

func main() {
	ebiten.SetWindowSize(client.ScreenWidth, client.ScreenHeight)
	ebiten.SetWindowTitle("1v1 FPS")

	game, err := NewGame()
	if err != nil {
		log.Fatal(err)
	}

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}

