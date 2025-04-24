package game

import (
	"context"
	"errors"
	"image"
	_ "image/png"
	"log/slog"
	"multiplayer/internal/mcp"
	"multiplayer/internal/state"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

type Game struct {
	houseImg *ebiten.Image
	sess     *mcp.Session
	state    state.State
}

func New(ctx context.Context, raddr string) (*Game, error) {
	houseImg, err := openImage("./assets/house.png")
	if err != nil {
		return nil, err
	}

	sess, err := mcp.Dial(ctx, raddr, mcp.WithLogger(slog.Default()))
	if err != nil {
		return nil, err
	}

	return &Game{
		houseImg: ebiten.NewImageFromImage(houseImg),
		sess:     sess,
		state:    state.State{},
	}, nil
}

func (g *Game) Close(ctx context.Context) error {
	return g.sess.Close(ctx)
}

func (g *Game) Layout(int, int) (int, int) {
	return 640, 480
}

func (g *Game) Draw(screen *ebiten.Image) {
	var m ebiten.GeoM
	m.Scale(0.2, 0.2)
	m.Translate(g.state.House.Trans.X, g.state.House.Trans.Y)
	screen.DrawImage(g.houseImg, &ebiten.DrawImageOptions{
		GeoM: m,
	})
}

func (g *Game) Update() error {
	dt := time.Second / time.Duration(ebiten.TPS())

	ctx, cancel := context.WithTimeout(context.Background(), dt)
	defer cancel()

	input := state.Input{
		Left:  ebiten.IsKeyPressed(ebiten.KeyH),
		Down:  ebiten.IsKeyPressed(ebiten.KeyJ),
		Up:    ebiten.IsKeyPressed(ebiten.KeyK),
		Right: ebiten.IsKeyPressed(ebiten.KeyL),
	}

	// TODO: use a buffer for sending inputs to ensure order and reliability
	data, err := input.MarshalBinary()
	if err != nil {
		slog.Warn("failed to marshal input", "error", err)
		return nil
	}
	err = g.sess.Send(ctx, data)
	if errors.Is(err, mcp.ErrClosed) {
		return ebiten.Termination
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return nil
	}
	if err != nil {
		slog.Warn("failed to send input", "error", err)
		return nil
	}

	// TODO: use an interpolation buffer to prevent jitter
	//
	// From https://gafferongames.com/post/snapshot_interpolation
	// > Now for the trick with snapshots. What we do is instead of
	// > immediately rendering snapshot data received is that we buffer
	// > snapshots for a short amount of time in an interpolation buffer. This
	// > interpolation buffer holds on to snapshots for a period of time such
	// > that you have not only the snapshot you want to render but also,
	// > statistically speaking, you are very likely to have the next snapshot
	// > as well. Then as the right side moves forward in time we interpolate
	// > between the position and orientation for the two slightly delayed
	// > snapshots providing the illusion of smooth movement. In effect, we’ve
	// > traded a small amount of added latency for smoothness.
	data, err = g.sess.Receive(ctx)
	if errors.Is(err, mcp.ErrClosed) {
		return ebiten.Termination
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return nil
	}
	if err != nil {
		slog.Warn("failed to receive state", "error", err)
		return nil
	}
	err = g.state.UnmarshalBinary(data)
	if err != nil {
		slog.Warn("failed to unmarshal state", "error", err)
		return nil
	}

	return nil
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
