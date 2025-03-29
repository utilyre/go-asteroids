package main

import (
	"encoding/binary"
	"log"
	"log/slog"
	"multiplayer/internal/inputbuffer"
	"multiplayer/internal/types"
	"net"
	"time"

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

func (g *Game) Start() error {
	go g.inputBufferSender()
	go g.inputBufferFlusher()
	return nil
}

func (g *Game) inputBufferFlusher() {
	for {
		data := make([]byte, 4)
		n, err := g.conn.Read(data)
		if err != nil {
			slog.Error("failed to read ack", "error", err)
			continue
		}
		if n != len(data) {
			panic("heeee")
		}

		var index uint32
		n, err = binary.Decode(data, binary.BigEndian, &index)
		if err != nil {
			slog.Error("failed to decode ack index", "error", err)
			continue
		}
		if n != len(data) {
			panic("stop it")
		}

		err = g.inputBuffer.FlushUntil(index)
		if err != nil {
			slog.Error("failed to flush until", "error", err)
			continue
		}
	}
}

func (g *Game) inputBufferSender() {
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()
	for {
		g.sendInputBuffer()
		<-ticker.C
	}
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

	// g.sendInputBuffer()

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

	err = g.Start()
	if err != nil {
		slog.Error("failed to start game", "error", err)
		return
	}

	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
