package main

import (
	"context"
	"errors"
	"image"
	_ "image/png"
	"multiplayer/internal/state"
	"os"
	"os/signal"
	"syscall"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	screenWidth  = 640
	screenHeight = 480
)

func main() {
	ctx, cancel := newSignalContext()
	defer cancel()

	ebiten.SetWindowTitle("Multiplayer - Simulation")
	ebiten.SetWindowSize(screenWidth, screenHeight)

	// listener

	houseImg, err := openImage("./assets/house.png")
	if err != nil {
		panic(err)
	}

	// simulation loop
	sim := &Simulation{
		done:     ctx.Done(),
		houseImg: ebiten.NewImageFromImage(houseImg),
	}
	err = ebiten.RunGame(sim)
	if err != nil {
		panic(err)
	}
}

func newSignalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	quitCh := make(chan os.Signal, 1)
	signal.Notify(
		quitCh,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGHUP,
		syscall.SIGQUIT,
		syscall.SIGPIPE,
	)

	go func() {
		wasSIGINT := false

		for sig := range quitCh {
			if wasSIGINT && sig == syscall.SIGINT {
				os.Exit(1)
			}

			wasSIGINT = sig == syscall.SIGINT
			cancel()
		}
	}()

	return ctx, cancel
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

type Simulation struct {
	done     <-chan struct{}
	houseImg *ebiten.Image

	state.State
}

func (sim *Simulation) Layout(int, int) (int, int) {
	return screenWidth, screenHeight
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

	const (
		dt         = 1.0 / 60
		houseAccel = 80
	)

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
