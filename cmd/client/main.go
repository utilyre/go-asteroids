package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image/color"
	"io"
	"log/slog"
	"multiplayer/internal/inputbuffer"
	"multiplayer/internal/types"
	"net"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
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
	types.State
	img         *ebiten.Image
	conn        net.Conn
	inputBuffer inputbuffer.InputBuffer
}

func NewGame() (*Game, error) {
	conn, err := net.Dial("udp", ":3000")
	if err != nil {
		return nil, err
	}

	img := ebiten.NewImage(10, 10)
	img.Fill(color.White)

	return &Game{img: img, conn: conn}, nil
}

func (g *Game) Start() error {
	go g.inputBufferSender()
	go g.inputBufferFlusher()
	return nil
}

var errShortMessage = errors.New("message not long enough")

func readAckIndex(r io.Reader) (index uint32, err error) {
	data := make([]byte, 4)
	n, err := r.Read(data)
	if err != nil {
		return 0, fmt.Errorf("reading ack: %w", err)
	}
	if l := len(data); n < l {
		return 0, fmt.Errorf("reading ack: %w", errShortMessage)
	}

	n, err = binary.Decode(data, binary.BigEndian, &index)
	if err != nil {
		panic("data should have been big enough")
	}
	if l := len(data); n < l {
		panic("message should have been big enough")
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
	err := g.conn.Close()
	if err != nil {
		return fmt.Errorf("closing udp %s: %w", g.conn.LocalAddr(), err)
	}
	return nil
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
	_, err = w.Write(bufData)
	if err != nil {
		return fmt.Errorf("writing input buffer: %w", err)
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	/* ebitenutil.DebugPrint(screen, fmt.Sprintf("W: %v\nA: %v\nS: %v\nD: %v",
		ebiten.IsKeyPressed(ebiten.KeyW),
		ebiten.IsKeyPressed(ebiten.KeyA),
		ebiten.IsKeyPressed(ebiten.KeyS),
		ebiten.IsKeyPressed(ebiten.KeyD),
	)) */

	var m ebiten.GeoM
	m.Translate(g.Position.X, g.Position.Y)
	screen.DrawImage(g.img, &ebiten.DrawImageOptions{GeoM: m})
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 320, 240
}
