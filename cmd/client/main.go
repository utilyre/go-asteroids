package main

import (
	"context"
	"log/slog"

	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Hello, World!")

	g, err := NewGame()
	if err != nil {
		slog.Error("failed to initialize game", "error", err)
		return
	}
	defer g.Close(context.TODO())

	if err := ebiten.RunGame(g); err != nil {
		slog.Error("failed to run game", "error", err)
	}
}
