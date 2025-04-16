package simulation

import (
	"image"
	"multiplayer/internal/state"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	ticksPerSecond = 60 // ebiten's default
	deltaTickTime  = time.Second / ticksPerSecond
)

type Simulation struct {
	done     <-chan struct{}
	houseImg *ebiten.Image

	// TODO: make multiplayer
	singlePlayerInputCh <-chan state.Input

	state.State
}

func New(done <-chan struct{}, houseImg image.Image) *Simulation {
	// bufferred channel for testing
	// TODO: should be a battle tested jitter buffer
	ch := make(chan state.Input, 1)
	// this "mocks" inputs coming in from a single client
	go func() {
		defer close(ch)

		t := time.NewTicker(deltaTickTime)
		defer t.Stop()

		for {
			select {
			case <-done:
				return
			case <-t.C:
			}

			input := state.Input{
				Left:  ebiten.IsKeyPressed(ebiten.KeyH),
				Down:  ebiten.IsKeyPressed(ebiten.KeyJ),
				Up:    ebiten.IsKeyPressed(ebiten.KeyK),
				Right: ebiten.IsKeyPressed(ebiten.KeyL),
			}

			select {
			case ch <- input:
			default:
			}
		}
	}()

	return &Simulation{
		done:                done,
		houseImg:            ebiten.NewImageFromImage(houseImg),
		singlePlayerInputCh: ch,
	}
}

func (sim *Simulation) Layout(int, int) (int, int) {
	return 640, 480
}

func (sim *Simulation) Draw(screen *ebiten.Image) {
	var m ebiten.GeoM
	m.Scale(0.2, 0.2)
	m.Translate(sim.House.Trans.X, sim.House.Trans.Y)
	screen.DrawImage(sim.houseImg, &ebiten.DrawImageOptions{
		GeoM: m,
	})
}

func (sim *Simulation) Update() error {
	select {
	case <-sim.done:
		return ebiten.Termination
	default:
	}

	// try to read input of each player
	// if no input for any player then they dont get to play on this frame
	// PERF: use reflect.Select to process the earliest, earlier

	var input state.Input
	select {
	case input = <-sim.singlePlayerInputCh:
	default:
	}

	sim.State = input.Manipulate(deltaTickTime, sim.State)

	return nil
}
