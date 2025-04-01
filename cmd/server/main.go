package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"multiplayer/internal/types"
	"net"
	"os"
	"sync/atomic"
	"time"
)

const inputRate = 15

func main() {
	srv, err := NewGameServer(":3000")
	if err != nil {
		slog.Error("failed to instantiate game server", "error", err)
		return
	}

	inputQueue := NewInputQueue()
	defer inputQueue.Close()

	simulation := NewSimulation(inputQueue)
	go simulation.Run()

	lastMessage := time.Now()
	for {
		inputs, addr, err := readInputsPacket(srv.conn, 1024)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			continue
		}
		if err != nil {
			slog.Warn("failed to read udp message",
				"address", addr, "error", err)
			continue
		}

		// drop messages that are received faster than inputRate
		if dt := time.Since(lastMessage); dt < time.Second/inputRate {
			continue
		}
		lastMessage = time.Now()

		if len(inputs) > 0 {
			lastInput := inputs[len(inputs)-1]
			err = ackInput(srv.conn, addr, lastInput)
			if err != nil {
				slog.Warn("failed to acknowledge last input",
					"address", addr, "error", err)
				continue
			}
			slog.Info("acknowledged last input", "index", lastInput.Index)
		}

		inputQueue.ProcessInputs(inputs)
	}
}

type GameServer struct {
	conn net.PacketConn
}

func NewGameServer(addr string) (*GameServer, error) {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("binding to udp %s: %w", addr, err)
	}

	return &GameServer{conn: conn}, nil
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

func readInputsPacket(conn net.PacketConn, bufSize int) ([]types.Input, net.Addr, error) {
	buf := make([]byte, bufSize)
	n, addr, readErr := conn.ReadFrom(buf)
	if errors.Is(readErr, os.ErrDeadlineExceeded) {
		return nil, nil, fmt.Errorf("reading from udp: %w", readErr)
	}
	if n < 3 {
		return nil, nil, fmt.Errorf("reading from udp: %w", errShortMessage)
	}

	var typ byte
	_, err := binary.Decode(buf[:n], binary.BigEndian, &typ)
	if err != nil {
		panic("message should have been large enough")
	}
	if typ != 1 {
		panic("unsupported message type")
	}

	var size uint16
	_, err = binary.Decode(buf[1:n], binary.BigEndian, &size)
	if err != nil {
		panic("message should have been large enough")
	}
	if size == 0 {
		return []types.Input{}, addr, nil
	}
	if 3+int(size)*types.InputSize > n {
		return nil, nil, fmt.Errorf("reading from udp %w", errCorruptedMessage)
	}
	if 3+int(size)*types.InputSize > bufSize {
		// TODO: ensure this does not happen by timing out slow connections
		panic("message size should not have exceeded the anticipated buffer size")
	}

	// readErr must be handled after processing udp message
	if readErr != nil {
		return nil, nil, fmt.Errorf("reading from udp: %w", err)
	}

	inputs := make([]types.Input, size)
	for i := range len(inputs) {
		err = inputs[i].UnmarshalBinary(buf[3+i*types.InputSize : 3+(i+1)*types.InputSize])
		if err != nil {
			return nil, nil, fmt.Errorf("unmarshaling input #%d: %w", i, err)
		}
	}

	return inputs, addr, nil
}

func ackInput(conn net.PacketConn, addr net.Addr, input types.Input) error {
	lastIndexData := make([]byte, 4)
	_, err := binary.Encode(lastIndexData, binary.BigEndian, input.Index)
	if err != nil {
		panic("data should have been large enough")
	}

	_, err = conn.WriteTo(lastIndexData, addr)
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
