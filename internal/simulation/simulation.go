package simulation

import (
	"context"
	"encoding/binary"
	"errors"
	"image"
	"log/slog"
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
	sessions       map[string]*mcp.Session
	sessionLock    sync.Mutex
	state          state.State
	lastStateIndex uint32
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

	sim := &Simulation{
		houseImg:       ebiten.NewImageFromImage(houseImg),
		ln:             ln,
		sessions:       map[string]*mcp.Session{},
		sessionLock:    sync.Mutex{},
		state:          state.State{},
		lastStateIndex: 0,
	}
	go sim.acceptLoop()
	return sim, nil
}

func (sim *Simulation) acceptLoop() {
	ctx := context.Background()
	for {
		sess, err := sim.ln.Accept(ctx)
		if errors.Is(err, mcp.ErrClosed) {
			break
		}
		if err != nil {
			slog.Warn("failed to accept session", "error", err)
			continue
		}

		// TODO: remove sess from sessions whenever closed

		sim.sessionLock.Lock()
		sim.sessions[sess.RemoteAddr().String()] = sess
		sim.sessionLock.Unlock()
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

	// collect data and then process it to reduce the duration at which the
	// lock is held.
	var inputDatas [][]byte
	sim.sessionLock.Lock()
	for _, sess := range sim.sessions {
		data, succeeded := sess.TryReceive()
		if !succeeded {
			continue
		}
		inputDatas = append(inputDatas, data)
	}
	sim.sessionLock.Unlock()

	for _, data := range inputDatas {
		var input state.Input
		err := input.UnmarshalBinary(data)
		if err != nil {
			slog.Warn("failed to unmarshal input", "error", err)
			return nil
		}
		sim.state.Update(dt, input)
	}

	data := make([]byte, 4+state.StateSize)
	binary.BigEndian.PutUint32(data, sim.lastStateIndex)
	stateData, err := sim.state.MarshalBinary()
	if err != nil {
		slog.Warn("failed to marshal state", "error", err)
		return nil
	}
	copy(data[4:], stateData)
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
