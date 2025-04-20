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

	<-ctx.Done()
}

func startServer(ctx context.Context, laddr string) {
	logger := slog.With("type", "server", "laddr", laddr)

	ln, err := mcp.Listen(":3000")
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

	for ctx.Err() == nil {
		sess, err := ln.Accept(ctx)
		if err != nil {
			logger.Warn("failed to accept session", "error", err)
			continue
		}

		go func() {
			logger = logger.With("raddr", sess.RemoteAddr())
			defer func() {
				err := sess.Close(context.Background())
				if err != nil {
					logger.Error("failed to close session", "error", err)
					return
				}
				logger.Info("session closed")
			}()
			logger.Info("session started")

			for ctx.Err() == nil {
				data, err := sess.Receive(ctx)
				if errors.Is(err, mcp.ErrClosed) {
					break
				}
				if err != nil {
					logger.Warn("failed to receive data", "error", err)
					continue
				}
				logger.Info("data received", "data", string(data))

				var idx int
				_, err = fmt.Sscanf(string(data), "ping %d", &idx)
				if err != nil {
					logger.Warn("failed to scan for ping index", "error", err)
					continue
				}
				data = []byte(fmt.Sprintf("pong %d", idx))
				err = sess.Send(ctx, data)
				if err != nil {
					logger.Warn("failed to send back data", "data", string(data), "error", err)
					continue
				}
				logger.Info("sent back data", "data", string(data))
			}
		}()
	}
}

func startClient(ctx context.Context, raddr string) {
	logger := slog.With("type", "client", "raddr", raddr)

	sess, err := mcp.Dial(ctx, raddr)
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

	for i := range 10 {
		data := fmt.Sprintf("ping %d", i)
		err := sess.Send(ctx, []byte(data))
		if err != nil {
			logger.Warn("failed to send data", "data", data, "error", err)
			continue
		}
		logger.Info("sent data", "data", data)
	}
	time.Sleep(time.Second) // TODO: make it that the session won't be closed until outbox is empty
}
