package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"multiplayer/internal/types"
	"multiplayer/internal/udp"
	"time"
)

type GameServer struct {
	ln              *udp.Listener
	mux             *udp.Mux
	muxInputChannel <-chan udp.Envelope
	inputQueue      *InputQueue
}

func NewGameServer(addr string, inputQueue *InputQueue) (*GameServer, error) {
	ln, err := udp.Listen(addr)
	if err != nil {
		return nil, fmt.Errorf("binding to udp %s: %w", addr, err)
	}
	mux := udp.NewMux(ln)
	muxInputChannel := mux.Subscribe(types.ScopeInput, 1)
	go mux.Run()

	srv := &GameServer{
		ln:              ln,
		mux:             mux,
		muxInputChannel: muxInputChannel,
		inputQueue:      inputQueue,
	}
	go srv.inputLoop()
	return srv, nil
}

func (srv *GameServer) Close(ctx context.Context) error {
	err := srv.mux.Close()
	if err != nil {
		return fmt.Errorf("close mux: %w", err)
	}
	err = srv.ln.Close(ctx)
	if err != nil {
		return fmt.Errorf("close ln: %w", err)
	}
	return nil
}

func (srv *GameServer) inputLoop() {
	const inputRate = 15
	lastMessage := time.Now()

	for envel := range srv.muxInputChannel {
		inputs, err := parseInputMessageBody(envel.Message.Body)
		if err != nil {
			slog.Warn("failed to read input message",
				"sender", envel.Sender, "error", err)
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

			msg := udp.NewMessageWithLabel(body, types.ScopeInputAck)

			err = srv.ln.TrySend(context.TODO(), envel.Sender, msg)
			if err != nil {
				slog.Warn("failed to acknowledge last input",
					"sender", envel.Sender, "error", err)
				return
			}
			slog.Info("acknowledged last input", "index", lastInput.Index)
		}

		srv.inputQueue.ProcessInputs(inputs)
	}
}

/* func (srv *GameServer) inputHandler() gameconn.Handler {
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
} */

var ErrCorruptedMessage = errors.New("message corrupted")

func parseInputMessageBody(body []byte) ([]types.Input, error) {
	if len(body) < 2 {
		return nil, ErrCorruptedMessage
	}

	var size uint16
	_, err := binary.Decode(body, binary.BigEndian, &size)
	if err != nil {
		panic("message should have been large enough")
	}
	if 2+int(size)*types.InputSize > len(body) {
		return nil, ErrCorruptedMessage
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
