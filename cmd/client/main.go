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
	conn        net.Conn
	inputBuffer *inputbuffer.InputBuffer
}

func NewGame() (*Game, error) {
	conn, err := net.Dial("udp", ":3000")
	if err != nil {
		return nil, err
	}

	return &Game{
		conn:        conn,
		inputBuffer: &inputbuffer.InputBuffer{},
	}, nil
}

/* func (g *Game) Start(ctx context.Context) error {
	go g.inputBufferSynchronizer(ctx)
	return nil
}

func (g *Game) inputBufferSynchronizer(ctx context.Context) {
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()
	for {
		data, err := g.inputBuffer.MarshalBinary()
		if err != nil {
			slog.Warn("failed to marshal input buffer", "error", err)
			continue
		}

		n, err := g.conn.Write(data)
		if n != len(data) {
			panic("could not write the entire input buffer")
		}

		<-ticker.C
	}
} */

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
	/* ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel() */

	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Hello, World!")

	g, err := NewGame()
	if err != nil {
		slog.Error("failed to initialize game", "error", err)
		return
	}
	defer g.Close()

	/* err = g.Start(ctx)
	if err != nil {
		slog.Error("failed to start game", "error", err)
		return
	} */

	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
