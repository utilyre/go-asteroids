package simulation

import (
	"context"
	"errors"
	"image"
	"log/slog"
	"multiplayer/internal/mcp"
	"multiplayer/internal/state"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

type Simulation struct {
	houseImg *ebiten.Image
	sess     *mcp.Session
	state    state.State
}

func New(ctx context.Context, laddr string) (*Simulation, error) {
	houseImg, err := openImage("./assets/house.png")
	if err != nil {
		return nil, err
	}

	ln, err := mcp.Listen(laddr, mcp.WithLogger(slog.Default()))
	if err != nil {
		return nil, err
	}
	slog.Info("bound udp/mcp listener", "address", ln.LocalAddr())

	sess, err := ln.Accept(ctx)
	if err != nil {
		return nil, err
	}

	return &Simulation{
		houseImg: ebiten.NewImageFromImage(houseImg),
		sess:     sess,
		state:    state.State{},
	}, nil
}

func openImage(name string) (img image.Image, err error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()

	img, _, err = image.Decode(f)
	if err != nil {
		return nil, err
	}

	return img, nil
}
func (sim *Simulation) Close(ctx context.Context) error {
	return sim.sess.Close(ctx)
}

func (sim *Simulation) Layout(int, int) (int, int) {
	return 640, 480
}

func (sim *Simulation) Draw(screen *ebiten.Image) {
	var m ebiten.GeoM
	m.Scale(0.2, 0.2)
	m.Translate(sim.state.House.Trans.X, sim.state.House.Trans.Y)
	screen.DrawImage(sim.houseImg, &ebiten.DrawImageOptions{
		GeoM: m,
	})
}

func (sim *Simulation) Update() error {
	dt := time.Second / time.Duration(ebiten.TPS())

	ctx, cancel := context.WithTimeout(context.Background(), dt)
	defer cancel()

	// try to read input of each player
	// if no input for any player then they dont get to play on this frame
	// PERF: use reflect.Select to process the earliest, earlier

	data, err := sim.sess.Receive(ctx)
	if errors.Is(err, mcp.ErrClosed) {
		return ebiten.Termination
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return nil
	}
	if err != nil {
		slog.Warn("failed to receive input", "error", err)
		return nil
	}
	var input state.Input
	err = input.UnmarshalBinary(data)
	if err != nil {
		slog.Warn("failed to unmarshal input", "error", err)
		return nil
	}

	sim.state.Update(dt, input)
	return nil
}
