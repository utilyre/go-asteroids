package main

import (
	"fmt"
	"log/slog"
	"multiplayer/internal/gameconn"
	"multiplayer/internal/types"
	"net"
	"time"
)

const inputRate = 15

type GameServer struct {
	conn       *gameconn.Conn
	inputQueue *InputQueue
}

func NewGameServer(addr string, inputQueue *InputQueue) (*GameServer, error) {
	conn, err := gameconn.Listen(addr)
	if err != nil {
		return nil, fmt.Errorf("binding to udp %s: %w", addr, err)
	}

	srv := &GameServer{conn: conn, inputQueue: inputQueue}
	srv.conn.Handle(types.ScopeInput, srv.handleInputs())

	return srv, nil
}

func (srv *GameServer) Close() error {
	err := srv.conn.Close()
	if err != nil {
		return fmt.Errorf("closing udp conn %q: %w", srv.conn.LocalAddr(), err)
	}
	return nil
}

func (srv *GameServer) handleInputs() gameconn.Handler {
	lastMessage := time.Now()
	return func(sender net.Addr, msg *gameconn.Message) {
		inputs, err := readInputsPacket(msg.Body)
		if err != nil {
			slog.Warn("failed to read input message",
				"sender_address", sender, "error", err)
			return
		}

		// drop messages that are received faster than inputRate
		if dt := time.Since(lastMessage); dt < time.Second/inputRate {
			return
		}
		lastMessage = time.Now()

		if len(inputs) > 0 {
			lastInput := inputs[len(inputs)-1]
			err = ackInput(srv.conn, sender, lastInput)
			if err != nil {
				slog.Warn("failed to acknowledge last input",
					"sender_address", sender, "error", err)
				return
			}
			slog.Info("acknowledged last input", "index", lastInput.Index)
		}

		srv.inputQueue.ProcessInputs(inputs)
	}
}
