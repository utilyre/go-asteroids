package main

import (
	"flag"
	"fmt"
	"log/slog"
	"multiplayer/internal/cli"
	_ "multiplayer/internal/config"
	"multiplayer/internal/game"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
)

var remoteAddr string

func init() {
	flag.StringVar(&remoteAddr, "remote", "", "specify remote server address")
	flag.Parse()

	if remoteAddr == "" {
		fmt.Fprintln(os.Stderr, "flag -remote is required")
		flag.PrintDefaults()
		os.Exit(1)
	}
}

func main() {
	ctx, cancel := cli.NewSignalContext()
	defer cancel()

	g, err := game.New(ctx, remoteAddr)
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
		slog.Error("failed to run game as an ebiten game", "error", err)
		return
	}
}
