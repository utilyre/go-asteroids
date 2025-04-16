package simulation

import (
	"image"
	"multiplayer/internal/state"

	"github.com/hajimehoshi/ebiten/v2"
)

type Simulation struct {
	done     <-chan struct{}
	houseImg *ebiten.Image

	state.State
}

func New(done <-chan struct{}, houseImg image.Image) *Simulation {
	return &Simulation{
		done:     done,
		houseImg: ebiten.NewImageFromImage(houseImg),
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
	const (
		dt         = 1.0 / 60
		houseAccel = 80
	)

	select {
	case <-sim.done:
		return ebiten.Termination
	default:
	}

	var v state.Vec2
	if ebiten.IsKeyPressed(ebiten.KeyH) {
		v.X -= 1
	}
	if ebiten.IsKeyPressed(ebiten.KeyJ) {
		v.Y += 1
	}
	if ebiten.IsKeyPressed(ebiten.KeyK) {
		v.Y -= 1
	}
	if ebiten.IsKeyPressed(ebiten.KeyL) {
		v.X += 1
	}
	sim.House.Accel = v.Normalize().Mul(houseAccel)
	sim.House.Trans = sim.House.Accel.Mul(0.5 * dt * dt).Add(sim.House.Vel.Mul(dt)).Add(sim.House.Trans)
	sim.House.Vel = sim.House.Accel.Mul(dt).Add(sim.House.Vel)

	return nil
}
