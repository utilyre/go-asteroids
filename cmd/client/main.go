package main

import (
	"log"
	"log/slog"
	"multiplayer/internal/inputbuffer"
	"multiplayer/internal/types"
	"net"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type Game struct {
	conn        *net.UDPConn
	inputBuffer *inputbuffer.InputBuffer
}

func NewGame() (*Game, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", ":3000")
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}

	return &Game{
		conn:        conn,
		inputBuffer: &inputbuffer.InputBuffer{},
	}, nil
}

func (g *Game) Close() error {
	return g.conn.Close()
}

func (g *Game) Update() error {
	input := types.Input{
		Up:    ebiten.IsKeyPressed(ebiten.KeyW),
		Left:  ebiten.IsKeyPressed(ebiten.KeyA),
		Down:  ebiten.IsKeyPressed(ebiten.KeyS),
		Right: ebiten.IsKeyPressed(ebiten.KeyD),
	}
	g.inputBuffer.Add(input)

	g.sendInputBuffer()

	return nil
}

func (g *Game) sendInputBuffer() {
	inputData, err := g.inputBuffer.MarshalBinary()
	if err != nil {
		slog.Error("failed to marshal input buffer", "error", err)
		return
	}

	_, err = g.conn.Write(inputData)
	if err != nil {
		slog.Error("failed to send input buffer over udp", "error", err)
		return
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	ebitenutil.DebugPrint(screen, "Hello, World!")
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 320, 240
}

func main() {
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Hello, World!")

	g, err := NewGame()
	if err != nil {
		slog.Error("failed to initialize game", "error", err)
		return
	}
	defer g.Close()

	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
