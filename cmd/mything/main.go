package main

import (
	"errors"
	"log/slog"
	_ "multiplayer/internal/config"
	"multiplayer/internal/mcp"
	"time"
)

func main() {
	ln, err := mcp.Listen(":3000")
	if err != nil {
		panic(err)
	}
	defer func() {
		err = ln.Close()
		if err != nil {
			slog.Error("failed to close listener", "error", err)
		}
	}()

	slog.Info("listening on", "address", ln.LocalAddr())

	go client()

	for {
		sess, err := ln.Accept()
		if errors.Is(err, mcp.ErrClosed) {
			slog.Info("connection closed")
			break
		}
		if err != nil {
			slog.Error("failed to accept session", "error", err)
			continue
		}

		slog.Info("accepted session", "sess", sess)

		for {
			data := sess.Receive()
			slog.Info("received data", "data", data)
		}
	}
}

func client() {
	time.Sleep(time.Second)

	sess, err := mcp.Dial(":3000")
	if err != nil {
		slog.Error("failed to dial server", "error", err)
		return
	}
	defer func() {
		err := sess.Close()
		if err != nil {
			slog.Error("failed to close client", "error", err)
		}
	}()

	slog.Info("established session", "laddr", sess.LocalAddr(), "raddr", sess.RemoteAddr())

	time.Sleep(time.Second)
	for range 10 {
		sess.Send([]byte("ping"))
	}
}
