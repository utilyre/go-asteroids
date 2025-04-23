package main

import (
	_ "image/png"
	"log/slog"
	"multiplayer/internal/cli"
	_ "multiplayer/internal/config"
	"multiplayer/internal/simulation"

	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	ctx, cancel := cli.NewSignalContext()
	defer cancel()

	sim, err := simulation.New(ctx, "127.0.0.1:3000")
	if err != nil {
		slog.Error("failed to instantiate simulation", "error", err)
		return
	}
	defer func() {
		err = sim.Close(ctx)
		if err != nil {
			slog.Error("failed to close simulation", "error", err)
		}
	}()

	ebiten.SetWindowTitle("Multiplayer - Simulation")
	err = ebiten.RunGame(sim)
	if err != nil {
		slog.Error("failed to run simulation as an ebiten game", "error", err)
		return
	}
}
