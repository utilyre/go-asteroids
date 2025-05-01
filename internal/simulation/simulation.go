package simulation

import (
	"context"
	"encoding/binary"
	"errors"
	"image"
	"log/slog"
	"multiplayer/internal/jitter"
	"multiplayer/internal/mcp"
	"multiplayer/internal/state"
	"os"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

type Simulation struct {
	houseImg       *ebiten.Image
	ln             *mcp.Listener
	clients        map[string]clientType
	clientLock     sync.Mutex
	state          state.State
	lastStateIndex uint32
}

func New(laddr string) (*Simulation, error) {
	houseImg, err := openImage("./assets/house.png")
	if err != nil {
		return nil, err
	}

	ln, err := mcp.Listen(laddr, mcp.WithLogger(slog.Default()))
	if err != nil {
		return nil, err
	}
	slog.Info("bound udp/mcp listener", "address", ln.LocalAddr())

	sim := &Simulation{
		houseImg:       ebiten.NewImageFromImage(houseImg),
		ln:             ln,
		clients:        map[string]clientType{},
		clientLock:     sync.Mutex{},
		state:          state.State{},
		lastStateIndex: 0,
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
		}()

		sim.clientLock.Lock()
		sim.clients[raddr] = client
		sim.clientLock.Unlock()
	}
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
	return sim.ln.Close(ctx)
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

	// collect inputs and then process them to reduce the duration at which the
	// lock is held.
	var inputs []state.Input
	sim.clientLock.Lock()
	for _, client := range sim.clients {
		select {
		case input := <-client.inputc:
			inputs = append(inputs, input)
		default:
		}
	}
	sim.clientLock.Unlock()

	for _, input := range inputs {
		sim.state.Update(dt, input)
	}

	data := make([]byte, 2+4+state.StateSize)
	binary.BigEndian.PutUint16(data, 1 /* type = state */)
	binary.BigEndian.PutUint32(data[2:], sim.lastStateIndex)
	stateData, err := sim.state.MarshalBinary()
	if err != nil {
		slog.Warn("failed to marshal state", "error", err)
		return nil
	}
	copy(data[6:], stateData)
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
