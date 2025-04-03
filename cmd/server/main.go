package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"multiplayer/internal/gameconn"
	"multiplayer/internal/types"
	"net"
	"sync/atomic"
	"time"
)

const inputRate = 15

func main() {
	inputQueue := NewInputQueue()
	defer inputQueue.Close()

	srv, err := NewGameServer(":3000", inputQueue)
	if err != nil {
		slog.Error("failed to instantiate game server", "error", err)
		return
	}
	defer srv.Close()
	srv.Start()

	simulation := NewSimulation(inputQueue)
	go simulation.Run()

	select {}
}

type GameServer struct {
	conn       *gameconn.Conn
	inputQueue *InputQueue
}

func NewGameServer(addr string, inputQueue *InputQueue) (*GameServer, error) {
	conn, err := gameconn.Listen(addr)
	if err != nil {
		return nil, fmt.Errorf("binding to udp %s: %w", addr, err)
	}

	return &GameServer{conn: conn, inputQueue: inputQueue}, nil
}

func (srv *GameServer) Start() {
	lastMessage := time.Now()
	srv.conn.Handle(types.ScopeInput, func(sender net.Addr, msg *gameconn.Message) {
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
	})
}

func (srv *GameServer) Close() error {
	err := srv.conn.Close()
	if err != nil {
		return fmt.Errorf("closing udp conn %q: %w", srv.conn.LocalAddr(), err)
	}
	return nil
}

var (
	errShortMessage     = errors.New("message not long enough")
	errCorruptedMessage = errors.New("message was corrupted")
)

func readInputsPacket(body []byte) ([]types.Input, error) {
	if len(body) < 2 {
		return nil, errShortMessage
	}

	var size uint16
	_, err := binary.Decode(body, binary.BigEndian, &size)
	if err != nil {
		panic("message should have been large enough")
	}
	if size == 0 {
		return []types.Input{}, nil
	}
	if 2+int(size)*types.InputSize > len(body) {
		return nil, fmt.Errorf("reading from udp %w", errCorruptedMessage)
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

func ackInput(conn *gameconn.Conn, addr net.Addr, input types.Input) error {
	lastIndexData := make([]byte, 4)
	_, err := binary.Encode(lastIndexData, binary.BigEndian, input.Index)
	if err != nil {
		panic("data should have been large enough")
	}

	err = conn.Send(addr, &gameconn.Message{
		Scope: types.ScopeInputAck,
		Body:  lastIndexData,
	})
	if err != nil {
		return fmt.Errorf("writing to udp: %w", err)
	}

	return nil
}

type InputQueue struct {
	ch        chan types.Input
	lastIndex atomic.Uint32
}

func NewInputQueue() *InputQueue {
	return &InputQueue{ch: make(chan types.Input, 1)}
}

func (q *InputQueue) Close() {
	close(q.ch)
}

func (q *InputQueue) ProcessInputs(inputs []types.Input) {
	lastIdx := q.lastIndex.Load()
	for _, input := range inputs {
		if input.Index <= lastIdx {
			continue
		}

		q.ch <- input
		q.lastIndex.Store(input.Index)
	}
}

func (q *InputQueue) Dequeue() (input types.Input, open bool) {
	input, open = <-q.ch
	return input, open
}

type Simulation struct {
	types.State
	inputQueue *InputQueue
}

func NewSimulation(inputQueue *InputQueue) *Simulation {
	return &Simulation{
		inputQueue: inputQueue,
	}
}

func (g *Simulation) Run() {
	for {
		input, open := g.inputQueue.Dequeue()
		if !open {
			break
		}

		g.Update(input)
		slog.Info("game state changed", "x", g.Position.X, "y", g.Position.Y)
	}
}

func (g *Simulation) Update(input types.Input) {
	dx := 0.0
	dy := 0.0
	if input.Up {
		dy += 1
	}
	if input.Left {
		dx -= 1
	}
	if input.Down {
		dy -= 1
	}
	if input.Right {
		dx += 1
	}

	magnitude := math.Sqrt(dx*dx + dy*dy)
	if magnitude > 0 {
		dx /= magnitude
		dy /= magnitude
	}

	g.Position.X += dx
	g.Position.Y += dy
}
