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
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

type snapshot struct {
	s state.State
	t time.Time
}

type Game struct {
	houseImg     *ebiten.Image
	sess         *mcp.Session
	state        state.State
	prevSnapshot snapshot
	nextSnapshot snapshot
	snapshotLock sync.Mutex
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

	g := &Game{
		houseImg:     ebiten.NewImageFromImage(houseImg),
		sess:         sess,
		state:        state.State{},
		prevSnapshot: snapshot{},
		nextSnapshot: snapshot{},
		snapshotLock: sync.Mutex{},
	}
	go g.snapshotLoop()
	return g, nil
}

func (g *Game) snapshotLoop() {
	ctx := context.Background()
	for {
		data, err := g.sess.Receive(ctx)
		if errors.Is(err, mcp.ErrClosed) {
			break
		}
		if err != nil {
			slog.Warn("failed to receive state", "error", err)
			continue
		}
		var s state.State
		err = s.UnmarshalBinary(data)
		if err != nil {
			slog.Warn("failed to unmarshal state", "error", err)
			continue
		}
		g.snapshotLock.Lock()
		g.prevSnapshot = g.nextSnapshot
		g.nextSnapshot = snapshot{
			s: s,
			t: time.Now(),
		}
		g.snapshotLock.Unlock()
	}
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

	g.snapshotLock.Lock()
	if !g.nextSnapshot.t.IsZero() {
		now := time.Now()

		// We'd ideally like to interpolate from the last frame to the next
		// frame, that is in the future. However, what actually happens is that
		// the two frames have already happened and our interpolation is forced
		// to delay from the reality in order to work properly. This is why t
		// is calculated by working out what fraction does $now - next$ have
		// over the duration between the two frames ($next - prev$), instead of
		// working out $now - prev$ over $next - prev$.
		//
		// Ideal:
		//  |-------|______________|
		// prev    now           next
		//
		// Reality:
		//  |______________________|-------|
		// prev                  next     now
		t := now.Sub(g.nextSnapshot.t).Seconds() / g.nextSnapshot.t.Sub(g.prevSnapshot.t).Seconds()

		slog.Debug("times", "t", t)
		g.state = g.prevSnapshot.s.Lerp(g.nextSnapshot.s, t)
	}
	g.snapshotLock.Unlock()

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
