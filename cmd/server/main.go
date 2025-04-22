package main

import (
	"errors"
	"image"
	_ "image/png"
	"multiplayer/internal/cli"
	_ "multiplayer/internal/config"
	"multiplayer/internal/simulation"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	ctx, cancel := cli.NewSignalContext()
	defer cancel()

	ebiten.SetWindowTitle("Multiplayer - Simulation")
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowClosingHandled(true)

	// listener

	houseImg, err := openImage("./assets/house.png")
	if err != nil {
		panic(err)
	}

	// simulation loop
	sim := simulation.New(ctx.Done(), houseImg)
	err = ebiten.RunGame(sim)
	if err != nil {
		panic(err)
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
