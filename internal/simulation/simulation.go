package simulation

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"multiplayer/assets"
	"multiplayer/internal/jitter"
	"multiplayer/internal/mcp"
	"multiplayer/internal/state"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

type Simulation struct {
	ln             *mcp.Listener
	clients        map[string]clientType
	clientLock     sync.Mutex
	state          state.State
	lastStateIndex uint32

	newAddrCh chan string
	rmAddrCh  chan string
}

func New(laddr string) (*Simulation, error) {
	ln, err := mcp.Listen(laddr, mcp.WithLogger(slog.Default()))
	if err != nil {
		return nil, err
	}
	slog.Info("bound udp/mcp listener", "address", ln.LocalAddr())

	sim := &Simulation{
		ln:             ln,
		clients:        map[string]clientType{},
		clientLock:     sync.Mutex{},
		state:          state.InitState(),
		lastStateIndex: 0,
		newAddrCh:      make(chan string, 10),
		rmAddrCh:       make(chan string, 10),
	}
	go sim.acceptLoop(context.Background())
	return sim, nil
}

type clientType struct {
	sess   *mcp.Session
	inputc chan state.Input
}

func (c clientType) start(ctx context.Context) {
	logger := slog.With("remote", c.sess.RemoteAddr())

	for {
		data, err := c.sess.Receive(ctx)
		if errors.Is(err, mcp.ErrClosed) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			break
		}
		if err != nil {
			logger.Warn("failed to receive inputs from session", "error", err)
			continue
		}

		var buf jitter.Buffer
		err = buf.UnmarshalBinary(data)
		if err != nil {
			logger.Warn("failed to unmarshal inputs", "error", err)
			continue
		}

		if indices := buf.Indices(); len(indices) > 0 {
			b := make([]byte, 2+4)
			binary.BigEndian.PutUint16(b, 0 /* type = input ack */)
			binary.BigEndian.PutUint32(b[2:], indices[len(indices)-1])
			// i refuse to spawn a new goroutine just to do this
			_ = c.sess.TrySend(b)
		}

		for _, input := range buf.Inputs() {
			select {
			case c.inputc <- input:
			default:
			}
		}
	}
}

func (sim *Simulation) acceptLoop(ctx context.Context) {
	for {
		sess, err := sim.ln.Accept(ctx)
		if errors.Is(err, mcp.ErrClosed) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			break
		}
		if err != nil {
			slog.Warn("failed to accept session", "error", err)
			continue
		}

		client := clientType{
			sess:   sess,
			inputc: make(chan state.Input, 1),
		}
		raddr := sess.RemoteAddr().String()
		go func() {
			client.start(context.Background())

			// should not sess.Close() since the only reason client.start()
			// returns is because sess has closed.

			sim.clientLock.Lock()
			delete(sim.clients, raddr)
			sim.clientLock.Unlock()
			sim.rmAddrCh <- raddr
		}()

		sim.clientLock.Lock()
		sim.clients[raddr] = client
		sim.clientLock.Unlock()
		sim.newAddrCh <- raddr
	}
}

func (sim *Simulation) Close(ctx context.Context) error {
	close(sim.rmAddrCh)
	close(sim.newAddrCh)
	return sim.ln.Close(ctx)
}

func (sim *Simulation) Layout(int, int) (int, int) {
	return state.ScreenWidth, state.ScreenHeight
}

func (sim *Simulation) Draw(screen *ebiten.Image) {
	for _, bullet := range sim.state.Bullets {
		var m ebiten.GeoM
		m.Scale(2, 2)
		m.Translate(bullet.Trans.X, bullet.Trans.Y)
		screen.DrawImage(assets.Bullet, &ebiten.DrawImageOptions{GeoM: m})
	}

	for _, asteroid := range sim.state.Asteroids {
		var m ebiten.GeoM
		bounds := assets.Rock.Bounds()
		m.Translate(-float64(bounds.Dx()/2), -float64(bounds.Dy()/2))
		m.Rotate(asteroid.Rotation)
		m.Scale(
			state.AsteroidWidth/float64(bounds.Dx()),
			state.AsteroidHeight/float64(bounds.Dy()),
		)
		m.Translate(asteroid.Trans.X, asteroid.Trans.Y)
		screen.DrawImage(assets.Rock, &ebiten.DrawImageOptions{GeoM: m})
	}

	for _, player := range sim.state.Players {
		var m ebiten.GeoM
		bounds := assets.Player.Bounds()
		m.Translate(-float64(bounds.Dx()/2), -float64(bounds.Dy()/2))
		m.Rotate(player.Rotation)
		m.Scale(
			state.PlayerWidth/float64(bounds.Dx()),
			state.PlayerHeight/float64(bounds.Dy()),
		)
		m.Translate(player.Trans.X, player.Trans.Y)
		screen.DrawImage(assets.Player, &ebiten.DrawImageOptions{
			GeoM: m,
		})

		op := &text.DrawOptions{}
		op.GeoM.Translate(player.Trans.X-state.PlayerWidth, player.Trans.Y-state.PlayerHeight)
		text.Draw(screen, fmt.Sprintf("%d", player.ID), &text.GoTextFace{
			Source: assets.MPlus1pRegular,
			Size:   50,
		}, op)
	}

	text.Draw(
		screen,
		fmt.Sprintf("Total Score: %d", sim.state.TotalScore),
		&text.GoTextFace{Source: assets.MPlus1pRegular, Size: 60},
		&text.DrawOptions{},
	)
}

func (sim *Simulation) Update() error {
	dt := time.Second / time.Duration(ebiten.TPS())

	ctx, cancel := context.WithTimeout(context.Background(), dt)
	defer cancel()

OUTER:
	for {
		select {
		case addr := <-sim.newAddrCh:
			sim.state.AddPlayer(addr)
		default:
			break OUTER
		}
	}
OUTER2:
	for {
		select {
		case addr := <-sim.rmAddrCh:
			sim.state.RemovePlayer(addr)
		default:
			break OUTER2
		}
	}

	inputs := map[string]state.Input{}
	sim.clientLock.Lock()
	for addr, client := range sim.clients {
		select {
		case input := <-client.inputc:
			inputs[addr] = input
		default:
		}
	}
	sim.clientLock.Unlock()

	sim.state.Update(dt, inputs)

	data := make([]byte, 2+4)
	binary.BigEndian.PutUint16(data, 1 /* type = state */)
	binary.BigEndian.PutUint32(data[2:], sim.lastStateIndex)
	stateData, err := sim.state.MarshalBinary()
	if err != nil {
		slog.Warn("failed to marshal state", "error", err)
		return nil
	}
	data = append(data, stateData...)
	err = sim.ln.Broadcast(ctx, data)
	if errors.Is(err, mcp.ErrClosed) {
		return ebiten.Termination
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return nil
	}
	if err != nil {
		slog.Warn("failed to send state", "error", err)
		return nil
	}
	sim.lastStateIndex++

	return nil
}
