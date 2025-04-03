package main

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"multiplayer/internal/gameconn"
	"multiplayer/internal/types"
	"net"
	"time"
)

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
	srv.conn.Handle(types.ScopeInput, srv.inputHandler())

	return srv, nil
}

func (srv *GameServer) Close() error {
	err := srv.conn.Close()
	if err != nil {
		return fmt.Errorf("closing udp conn %q: %w", srv.conn.LocalAddr(), err)
	}
	return nil
}

func (srv *GameServer) inputHandler() gameconn.Handler {
	const inputRate = 15

	lastMessage := time.Now()
	return func(sender net.Addr, msg *gameconn.Message) {
		inputs, err := parseInputMessageBody(msg.Body)
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
			body := make([]byte, 4)
			_, _ = binary.Encode(body, binary.BigEndian, lastInput.Index)

			err = srv.conn.Send(sender, &gameconn.Message{
				Scope: types.ScopeInputAck,
				Body:  body,
			})
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

func parseInputMessageBody(body []byte) ([]types.Input, error) {
	if len(body) < 2 {
		return nil, gameconn.ErrCorruptedMessage
	}

	var size uint16
	_, err := binary.Decode(body, binary.BigEndian, &size)
	if err != nil {
		panic("message should have been large enough")
	}
	if 2+int(size)*types.InputSize > len(body) {
		return nil, gameconn.ErrCorruptedMessage
	}

	inputs := make([]types.Input, size)
	for i := range len(inputs) {
		err = inputs[i].UnmarshalBinary(body[2+i*types.InputSize : 2+(i+1)*types.InputSize])
		if err != nil {
			return nil, fmt.Errorf("unmarshaling input #%d: %w", i, err)
		}
	}

	return inputs, nil
}
