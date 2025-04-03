package main

import "log/slog"

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
	simulation.Run()
}
