package main

import (
	"bytes"
	"errors"
	"fmt"
	"image/color"
	"log/slog"
	"multiplayer/internal/gameconn"
	"multiplayer/internal/types"
	"net"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

type Game struct {
	types.State
	img         *ebiten.Image
	conn        *gameconn.Conn
	serverAddr  net.Addr
	inputBuffer *InputBuffer
}

func NewGame() (*Game, error) {
	conn, err := gameconn.Listen(":")
	if err != nil {
		return nil, err
	}

	serverAddr, err := net.ResolveUDPAddr("udp", ":3000")
	if err != nil {
		return nil, err
	}

	img := ebiten.NewImage(10, 10)
	img.Fill(color.White)

	g := &Game{
		img:         img,
		conn:        conn,
		serverAddr:  serverAddr,
		inputBuffer: &InputBuffer{},
	}

	go g.inputBufferSender()
	g.conn.Handle(types.ScopeInputAck, g.inputAckHandler)

	return g, nil
}

func (g *Game) inputAckHandler(sender net.Addr, msg *gameconn.Message) {
	index, err := readAckIndex(bytes.NewReader(msg.Body))
	if err != nil {
		slog.Warn("failed to read ack index",
			"sender", sender, "error", err)
		return
	}

	err = g.inputBuffer.FlushUntil(index)
	if err != nil {
		slog.Error("failed to flush input buffer",
			"until_index", index, "error", err)
		return
	}
}

func (g *Game) inputBufferSender() {
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()
	for ; ; <-ticker.C {
		var body bytes.Buffer

		err := writeInputBuffer(&body, g.inputBuffer)
		if err != nil {
			slog.Warn("failed to write input buffer", "error", err)
			continue
		}

		err = g.conn.Send(g.serverAddr, &gameconn.Message{
			Scope: types.ScopeInput,
			Body:  body.Bytes(),
		})
		if errors.Is(err, net.ErrClosed) {
			slog.Info("connection closed", "server_address", g.serverAddr)
			return
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
