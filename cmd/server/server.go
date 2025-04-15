package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"multiplayer/internal/types"
	"multiplayer/internal/udp"
	"sync/atomic"
	"time"
)

const (
	inputTopicBufSize = 100
	inputAckRate      = time.Second / 15
)

type GameServer struct {
	ln         *udp.Listener
	mux        *udp.Mux
	inputQueue *InputQueue
}

func NewGameServer(addr string, inputQueue *InputQueue) (*GameServer, error) {
	ln, err := udp.Listen(addr)
	if err != nil {
		return nil, fmt.Errorf("binding to udp %s: %w", addr, err)
	}
	slog.Info("bound to udp", "address", ln.LocalAddr())
	mux := udp.NewMux(ln)
	inputTopic := mux.Subscribe(types.ScopeInput, inputTopicBufSize)
	go mux.Run()

	srv := &GameServer{
		ln:         ln,
		mux:        mux,
		inputQueue: inputQueue,
	}
	go srv.inputLoop(inputTopic)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for ; ; <-ticker.C {
			n := numAckedInput.Load()
			slog.Debug("input ack rate", "rate", n)
			numAckedInput.Store(0)
		}
	}()
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

var numAckedInput atomic.Uint32

func (srv *GameServer) inputLoop(inputTopic <-chan udp.Envelope) {
	lastAck := time.Now()

	for envel := range inputTopic {
		inputs, err := parseInputMessageBody(envel.Message.Body)
		if err != nil {
			slog.Warn("failed to read input message",
				"sender", envel.Sender, "error", err)
			continue
		}

		if len(inputs) > 0 && time.Since(lastAck) > inputAckRate {
			lastInput := inputs[len(inputs)-1]
			body := make([]byte, 4)
			must(binary.Encode(body, binary.BigEndian, lastInput.Index))

			msg := udp.NewMessageWithLabel(body, types.ScopeInputAck)

			err = srv.ln.TrySend(context.TODO(), envel.Sender, msg)
			if err != nil {
				slog.Warn("failed to acknowledge last input",
					"sender", envel.Sender, "error", err)
			}
			numAckedInput.Add(1)
			lastAck = time.Now()
		}

		srv.inputQueue.ProcessInputs(envel.Sender, inputs)
	}
}

var ErrCorruptedMessage = errors.New("message corrupted")

func parseInputMessageBody(body []byte) ([]types.Input, error) {
	if len(body) < 2 {
		return nil, fmt.Errorf("short body: %w", ErrCorruptedMessage)
	}

	var size uint16
	must(binary.Decode(body, binary.BigEndian, &size))
	if expected := 2 + int(size)*types.InputSize; expected > len(body) {
		return nil, fmt.Errorf(
			"expected body size > %d; actual body size = %d: %w",
			expected,
			len(body),
			ErrCorruptedMessage,
		)
	}

	inputs := make([]types.Input, size)
	for i := range len(inputs) {
		err := inputs[i].UnmarshalBinary(body[2+i*types.InputSize : 2+(i+1)*types.InputSize])
		if err != nil {
			return nil, fmt.Errorf("unmarshaling input #%d: %w", i, err)
		}
	}

	return inputs, nil
}
