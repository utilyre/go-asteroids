package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	_ "multiplayer/internal/config"
	"multiplayer/internal/mcp"
	"os"
	"os/signal"
)

// TODO: lifetime is not handled properly
//       1. don't know when to stop receiving
//       2. closing client session twice gives no error

func main() {
	server, err := mcp.Listen(":3000")
	if err != nil {
		slog.Error("failed to start server", "error", err)
		return
	}
	defer func() {
		err = server.Close()
		if err != nil {
			slog.Error("failed to close server", "error", err)
		}
	}()
	slog.Info("server started", "address", server.LocalAddr())

	client, err := mcp.Dial(context.TODO(), ":3000")
	if err != nil {
		slog.Error("failed to start client", "error", err)
		return
	}
	slog.Info("client dialed server", "address", client.LocalAddr())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig

		err := client.Close()
		if err != nil {
			slog.Error("failed to close client", "error", err)
		}

		err = server.Close()
		if err != nil {
			slog.Error("failed to close server", "error", err)
		}
	}()

	go provider(client)

	for {
		sess, err := server.Accept(context.TODO())
		if errors.Is(err, mcp.ErrClosed) {
			slog.Info("connection closed")
			break
		}
		if err != nil {
			slog.Error("failed to accept session", "error", err)
			continue
		}
		slog.Info("session accepted", "address", sess.RemoteAddr())

		go consumer(sess)
	}
}

func consumer(sess *mcp.Session) {
	defer func() {
		err := sess.Close()
		if err != nil {
			slog.Error("failed to close consumer (server) session", "error", err)
		}
	}()
	for {
		data, err := sess.Receive(context.TODO())
		if err != nil {
			slog.Error("failed to receive data", "error", err)
			continue
		}

		slog.Info("received data",
			"remote", sess.RemoteAddr(), "data", string(data))
	}
}

func provider(sess *mcp.Session) {
	for i := range 10 {
		data := []byte(fmt.Sprintf("ping %d", i))
		err := sess.Send(context.TODO(), data)
		if err != nil {
			slog.Error("failed to send data", "data", string(data), "error", err)
		}
		slog.Info("sent data", "remote", sess.RemoteAddr(), "data", string(data))
	}
}
