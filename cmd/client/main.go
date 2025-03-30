package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"multiplayer/internal/inputbuffer"
	"multiplayer/internal/types"
	"net"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

func main() {
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
		slog.Error("failed to run game", "error", err)
	}
}

type Game struct {
	conn        net.Conn
	inputBuffer inputbuffer.InputBuffer
}

func NewGame() (*Game, error) {
	conn, err := net.Dial("udp", ":3000")
	if err != nil {
		return nil, err
	}

	return &Game{conn: conn}, nil
}

func (g *Game) Start() error {
	go g.inputBufferSender()
	go g.inputBufferFlusher()
	return nil
}

func readAckIndex(r io.Reader) (index uint32, err error) {
	data := make([]byte, 4)
	n, err := r.Read(data)
	if err != nil {
		return 0, fmt.Errorf("reading ack: %w", err)
	}
	if l := len(data); n < l {
		panic("TODO")
	}

	n, err = binary.Decode(data, binary.BigEndian, &index)
	if err != nil {
		return 0, fmt.Errorf("decoding ack index: %w", err)
	}
	if l := len(data); n < l {
		panic("TODO")
	}

	return index, nil
}

func (g *Game) inputBufferFlusher() {
	for {
		index, err := readAckIndex(g.conn)
		if errors.Is(err, net.ErrClosed) {
			slog.Info("connection closed", "remote", g.conn.RemoteAddr())
			return
		}
		if err != nil {
			slog.Warn("failed to read ack index",
				"remote", g.conn.RemoteAddr(), "error", err)
			continue
		}

		err = g.inputBuffer.FlushUntil(index)
		if err != nil {
			slog.Error("failed to flush input buffer",
				"until_index", index, "error", err)
			continue
		}
	}
}

func (g *Game) inputBufferSender() {
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()
	for ; ; <-ticker.C {
		err := writeInputBuffer(g.conn, g.inputBuffer)
		if errors.Is(err, net.ErrClosed) {
			slog.Info("connection closed", "remote", g.conn.RemoteAddr())
			return
		}
		if err != nil {
			slog.Warn("failed to write input buffer",
				"remote", g.conn.RemoteAddr(), "error", err)
			continue
		}
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

	return nil
}

func writeInputBuffer(w io.Writer, buf inputbuffer.InputBuffer) error {
	bufData, err := buf.MarshalBinary()
	if err != nil {
		return fmt.Errorf("marshaling input buffer: %w", err)
	}

	n, err := w.Write(bufData)
	if err != nil {
		return fmt.Errorf("writing input buffer: %w", err)
	}
	if l := len(bufData); n < l {
		panic("TODO")
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	ebitenutil.DebugPrint(screen, "Hello, World!")
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 320, 240
}
