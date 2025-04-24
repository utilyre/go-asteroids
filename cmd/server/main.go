package main

import (
	"flag"
	_ "image/png"
	"log/slog"
	"multiplayer/internal/cli"
	_ "multiplayer/internal/config"
	"multiplayer/internal/simulation"

	"github.com/hajimehoshi/ebiten/v2"
)

var localAddr string

func init() {
	flag.StringVar(&localAddr, "addr", "127.0.0.1:", "specify udp/mcp listener address")
	flag.Parse()
}

func main() {
	ctx, cancel := cli.NewSignalContext()
	defer cancel()

	sim, err := simulation.New(ctx, localAddr)
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
	ebiten.SetTPS(15)
	err = ebiten.RunGame(sim)
	if err != nil {
		slog.Error("failed to run simulation as an ebiten game", "error", err)
		return
	}
}
