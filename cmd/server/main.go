package main

import (
	"context"
	"log/slog"
	_ "multiplayer/internal/config"
	"multiplayer/internal/types"
	"multiplayer/internal/udp"
)

func main() {
	inputQueue := NewInputQueue()
	defer inputQueue.Close()

	srv, err := NewGameServer(":3000", inputQueue)
	if err != nil {
		slog.Error("failed to instantiate game server", "error", err)
		return
	}
	defer srv.Close(context.TODO())

	simulation := NewSimulation(inputQueue)

	go func() {
		for snapshot := range simulation.SnapshotQueue() {
			data, err := snapshot.MarshalBinary()
			if err != nil {
				slog.Error("failed to marshal snapshot", "error", err)
				continue
			}

			msg := udp.NewMessageWithLabel(data, types.ScopeSnapshot)
			err = srv.ln.TrySendAll(context.TODO(), msg)
			if err != nil {
				slog.Error("failed to send snapshot", "error", err)
				continue
			}
		}
	}()

	simulation.Run()
}
