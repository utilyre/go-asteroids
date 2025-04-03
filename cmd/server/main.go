package main

import (
	"log/slog"
	"multiplayer/internal/gameconn"
	"multiplayer/internal/types"
)

func main() {
	inputQueue := NewInputQueue()
	defer inputQueue.Close()

	srv, err := NewGameServer(":3000", inputQueue)
	if err != nil {
		slog.Error("failed to instantiate game server", "error", err)
		return
	}
	defer srv.Close()

	simulation := NewSimulation(inputQueue)

	go func() {
		for snapshot := range simulation.SnapshotQueue() {
			data, err := snapshot.MarshalBinary()
			if err != nil {
				slog.Error("failed to marshal snapshot", "error", err)
				continue
			}

			srv.conn.SendAll(&gameconn.Message{
				Scope: types.ScopeSnapshot,
				Body:  data,
			})
		}
	}()

	simulation.Run()
}
