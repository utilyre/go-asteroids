package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image/color"
	"log/slog"
	"multiplayer/internal/types"
	"multiplayer/internal/udp"
	"net"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

type Game struct {
	types.State
	img                *ebiten.Image
	ln                 *udp.Listener
	mux                *udp.Mux
	muxInputAckChannel <-chan udp.Envelope
	muxSnapshotChannel <-chan udp.Envelope
	serverAddr         net.Addr
	inputBuffer        *InputBuffer
}

func NewGame() (*Game, error) {
	ln, err := udp.Listen(":")
	if err != nil {
		return nil, err
	}
	slog.Info("bound to udp", "address", ln.LocalAddr())
	mux := udp.NewMux(ln)
	muxInputAckChannel := mux.Subscribe(types.ScopeInputAck, 1)
	muxSnapshotChannel := mux.Subscribe(types.ScopeSnapshot, 1)
	go mux.Run()

	serverAddr, err := net.ResolveUDPAddr("udp", ":3000")
	if err != nil {
		return nil, err
	}

	err = ln.Greet(context.TODO(), serverAddr)
	if err != nil {
		return nil, fmt.Errorf("saying hi to server: %w", err)
	}

	img := ebiten.NewImage(10, 10)
	img.Fill(color.White)

	g := &Game{
		img:                img,
		ln:                 ln,
		mux:                mux,
		muxInputAckChannel: muxInputAckChannel,
		muxSnapshotChannel: muxSnapshotChannel,
		serverAddr:         serverAddr,
		inputBuffer:        &InputBuffer{},
	}

	go g.inputBufferSender()
	go g.inputAckLoop()
	go g.snapshotLoop()

	return g, nil
}

func (g *Game) snapshotLoop() {
	for envel := range g.muxSnapshotChannel {
		err := g.State.UnmarshalBinary(envel.Message.Body)
		if err != nil {
			slog.Warn("failed to unmarshal snapshot", "error", err)
			continue
		}
	}
}

func (g *Game) Close(ctx context.Context) error {
	err := g.mux.Close()
	if err != nil {
		return fmt.Errorf("close mux: %w", err)
	}
	err = g.ln.Close(ctx)
	if err != nil {
		return fmt.Errorf("close ln: %w", err)
	}
	return nil
}

func (g *Game) inputAckLoop() {
	for envel := range g.muxInputAckChannel {
		var index uint32
		_, err := binary.Decode(envel.Message.Body, binary.BigEndian, &index)
		if err != nil {
			slog.Warn("failed to decode ack input index", "error", err)
		}

		err = g.inputBuffer.FlushUntil(index)
		if err != nil {
			slog.Warn("failed to flush input buffer",
				"until_index", index, "error", err)
			return
		}

	}
}

func (g *Game) inputBufferSender() {
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()
	for ; ; <-ticker.C {
		body, err := g.inputBuffer.MarshalBinary()
		if err != nil {
			slog.Warn("failed to marshal input buffer", "error", err)
			continue
		}

		msg := udp.NewMessageWithLabel(body, types.ScopeInput)
		err = g.ln.TrySend(context.TODO(), g.serverAddr, msg)
		if errors.Is(err, net.ErrClosed) {
			slog.Info("connection closed", "server_address", g.serverAddr)
			return
		}
		if err != nil {
			slog.Warn("failed to send inputs", "error", err)
			continue
		}
	}
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
