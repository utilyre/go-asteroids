package main

import (
	"log/slog"
	"multiplayer/internal/cli"
	_ "multiplayer/internal/config"
	"multiplayer/internal/game"

	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	ctx, cancel := cli.NewSignalContext()
	defer cancel()

	g, err := game.New(ctx, "127.0.0.1:3000")
	if err != nil {
		slog.Error("failed to initialize game", "error", err)
		return
	}
	defer func() {
		err = g.Close(ctx)
		if err != nil {
			slog.Error("failed to close game", "error", err)
		}
	}()

	ebiten.SetWindowTitle("Multiplayer")
	err = ebiten.RunGame(g)
	if err != nil {
		slog.Error("failed to run game", "error", err)
		return
	}
}
