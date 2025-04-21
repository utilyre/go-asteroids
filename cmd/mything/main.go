package main

import (
	"context"
	"errors"
	"log/slog"
	_ "multiplayer/internal/config"
	"multiplayer/internal/mcp"
	"os"
	"os/signal"
	"time"
)

func main() {
	run()
	select {}
}

func run() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	go startServer(ctx, ":3000")
	go startClient(ctx, ":3000")
	go startClient(ctx, ":3000")

	<-ctx.Done()
}

func startServer(ctx context.Context, laddr string) {
	logger := slog.With("type", "server", "laddr", laddr)

	ln, err := mcp.Listen(":3000", mcp.WithLogger(logger))
	if err != nil {
		logger.Error("failed to listen", "error", err)
		return
	}
	defer func() {
		err = ln.Close(ctx)
		if err != nil {
			logger.Error("failed to close listener", "error", err)
		}
		logger.Info("listener closed")
	}()
	logger.Info("listener started")

	go func() {
		time.Sleep(time.Second)
		for range 10 {
			err := ln.Broadcast(ctx, []byte("ping"))
			if errors.Is(err, mcp.ErrClosed) {
				break
			}
			if errors.Is(err, context.Canceled) {
				break
			}
			if err != nil {
				logger.Error("failed to broadcast data")
				continue
			}
		}
	}()

	for {
		sess, err := ln.Accept(ctx)
		if errors.Is(err, mcp.ErrClosed) {
			break
		}
		if errors.Is(err, context.Canceled) {
			break
		}
		if err != nil {
			logger.Warn("failed to accept session", "error", err)
			continue
		}

		logger.Info("session accepted", "raddr", sess.RemoteAddr())

		go func() {
			logger := logger.With("raddr", sess.RemoteAddr())
			defer func() {
				err := sess.Close(context.Background())
				if err != nil {
					logger.Error("failed to close session", "error", err)
					return
				}
				logger.Info("session closed")
			}()
			logger.Info("session started")

			<-ctx.Done()
		}()
	}
}

func startClient(ctx context.Context, raddr string) {
	logger := slog.With("type", "client", "raddr", raddr)

	sess, err := mcp.Dial(ctx, raddr, mcp.WithLogger(logger))
	if err != nil {
		logger.Error("failed to dial", "error", err)
		return
	}
	logger = logger.With("laddr", sess.LocalAddr())
	defer func() {
		err := sess.Close(ctx)
		if err != nil {
			logger.Error("failed to close session", "error", err)
		}
	}()
	logger.Info("session started")

	for {
		data, err := sess.Receive(ctx)
		if errors.Is(err, mcp.ErrClosed) {
			break
		}
		if errors.Is(err, context.Canceled) {
			break
		}
		if err != nil {
			logger.Warn("failed to receive data", "error", err)
			continue
		}
		slog.Info("received data", "data", string(data))
	}
	time.Sleep(time.Second) // TODO: make it that the session won't be closed until outbox is empty
}
