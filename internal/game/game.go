package game

import (
	"context"
	"encoding/binary"
	"errors"
	_ "image/png"
	"log/slog"
	"multiplayer/internal/jitter"
	"multiplayer/internal/mcp"
	"multiplayer/internal/state"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

type snapshot struct {
	s state.State
	t time.Time
}

type Game struct {
	imgPlayer *ebiten.Image
	imgBullet *ebiten.Image
	imgRock   *ebiten.Image

	sess *mcp.Session

	inputBuffer     jitter.Buffer
	inputBufferLock sync.Mutex

	state          state.State
	prevSnapshot   snapshot
	nextSnapshot   snapshot
	lastStateIndex uint32
	snapshotLock   sync.Mutex
}

func New(ctx context.Context, raddr string, imgPlayer, imgBullet, imgRock *ebiten.Image) (*Game, error) {
	sess, err := mcp.Dial(ctx, raddr, mcp.WithLogger(slog.Default()))
	if err != nil {
		return nil, err
	}

	g := &Game{
		imgPlayer:       imgPlayer,
		imgBullet:       imgBullet,
		imgRock:         imgRock,
		sess:            sess,
		inputBuffer:     jitter.Buffer{},
		inputBufferLock: sync.Mutex{},
		state:           state.State{},
		lastStateIndex:  0,
		prevSnapshot:    snapshot{},
		nextSnapshot:    snapshot{},
		snapshotLock:    sync.Mutex{},
	}
	go g.receiveLoop(context.Background())
	return g, nil
}

func (g *Game) receiveLoop(ctx context.Context) {
	for {
		data, err := g.sess.Receive(ctx)
		if errors.Is(err, mcp.ErrClosed) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			break
		}
		if err != nil {
			slog.Warn("failed to receive state", "error", err)
			continue
		}
		if len(data) < 2 {
			slog.Warn("received data does not contain type")
			continue
		}

		// TODO: this is stupid, come up with an actual protocol
		typ := binary.BigEndian.Uint16(data)
		data = data[2:]
		switch typ {
		case 0: // input ack
			if l := len(data); l < 4 {
				slog.Warn("input acknowledgement is smaller than uint32", "length", l)
				continue
			}
			index := binary.BigEndian.Uint32(data)
			g.inputBufferLock.Lock()
			g.inputBuffer.DiscardUntil(index)
			g.inputBufferLock.Unlock()

		case 1: // state
			if len(data) < 4 {
				slog.Warn("state data is smaller than uint32", "length", len(data))
				continue
			}
			index := binary.BigEndian.Uint32(data)
			if index <= g.lastStateIndex {
				continue
			}

			var s state.State
			err = s.UnmarshalBinary(data[4:])
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
			g.lastStateIndex = index
		}
	}
}

func (g *Game) Close(ctx context.Context) error {
	return g.sess.Close(ctx)
}

func (g *Game) Layout(int, int) (int, int) {
	return state.ScreenWidth, state.ScreenHeight
}

func (g *Game) Draw(screen *ebiten.Image) {
	var m ebiten.GeoM
	bounds := g.imgPlayer.Bounds()
	m.Translate(-float64(bounds.Dx()/2), -float64(bounds.Dy()/2))
	m.Rotate(g.state.Player.Rotation)
	m.Scale(
		state.PlayerSize/float64(bounds.Dx()),
		state.PlayerSize/float64(bounds.Dy()),
	)
	m.Translate(g.state.Player.Trans.X, g.state.Player.Trans.Y)
	screen.DrawImage(g.imgPlayer, &ebiten.DrawImageOptions{
		GeoM: m,
	})
}

func (g *Game) Update() error {
	input := state.Input{
		Left:  ebiten.IsKeyPressed(ebiten.KeyA),
		Down:  ebiten.IsKeyPressed(ebiten.KeyS),
		Up:    ebiten.IsKeyPressed(ebiten.KeyW),
		Right: ebiten.IsKeyPressed(ebiten.KeyD),
	}

	g.inputBufferLock.Lock()
	g.inputBuffer.Append(input)
	data, err := g.inputBuffer.MarshalBinary()
	g.inputBufferLock.Unlock()
	if err != nil {
		slog.Warn("failed to marshal input buffer", "error", err)
		return nil
	}
	if g.sess.Closed() {
		return ebiten.Termination
	}
	_ = g.sess.TrySend(data)

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

		g.state = g.prevSnapshot.s.Lerp(g.nextSnapshot.s, t)
	}
	g.snapshotLock.Unlock()

	return nil
}
