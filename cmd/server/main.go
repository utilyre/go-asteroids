package main

import (
	"encoding/binary"
	"log/slog"
	"math"
	"multiplayer/internal/types"
	"multiplayer/pkg/plaiq"
	"net"
	"time"
)

func main() {
	conn, err := net.ListenPacket("udp", ":3000")
	if err != nil {
		slog.Error("failed to listen on udp", "error", err)
		return
	}
	defer conn.Close()

	for {
		buf := make([]byte, 1024)

		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			slog.Error("failed to read from udp", "error", err)
			continue
		}

		var size uint16
		n, err = binary.Decode(buf[:n], binary.BigEndian, &size)
		if err != nil {
			slog.Error("failed to decode size from payload", "error", err)
			continue
		}
		if n != 2 {
			panic("why?")
		}
		if 2+int(size)*types.InputSize > len(buf) {
			panic("we are in trouble")
		}

		inputs := make([]types.Input, size)
		for i := range len(inputs) {
			err = inputs[i].UnmarshalBinary(buf[2+i*types.InputSize : 2+(i+1)*types.InputSize])
			if err != nil {
				panic("wtf")
			}
		}
		if len(inputs) == 0 {
			slog.Info("skipping this one")
			continue
		}

		lastIndex := inputs[len(inputs)-1].Index
		slog.Info("last index", "idx", lastIndex)

		lastIndexData := make([]byte, 4)
		n, err = binary.Encode(lastIndexData, binary.BigEndian, lastIndex)
		if err != nil {
			panic("should have enough space")
		}
		if n != len(lastIndexData) {
			panic("no way")
		}

		n, err = conn.WriteTo(lastIndexData, addr)
		if err != nil {
			slog.Error("failed to ack last input", "error", err)
			continue
		}
		if n != len(lastIndexData) {
			panic("why not")
		}
		slog.Info("sent ack")
	}
}

func (conn *Conn) Do() {
	sizeData := make([]byte, 2)
	n, firstAddr, err := conn.ReadFromUDP(sizeData)
	if err != nil {
		slog.Error("failed to read size from udp", "error", err)
		return
	}
	if n != len(sizeData) {
		panic("unexpected bytes read from udp")
	}

	var size uint16
	n, err = binary.Decode(sizeData, binary.BigEndian, &size)
	if err != nil {
		slog.Error("failed to decode big endian data size", "error", err)
		return
	}
	if n != len(sizeData) {
		panic("unexpected number of bytes consumed for size")
	}

	payloadSize := 2
	if size%2 == 0 {
		payloadSize += int(size) / 2
	} else {
		payloadSize += int(size)/2 + 1
	}
	data := make([]byte, payloadSize)
	n, secondAddr, err := conn.ReadFromUDP(data)
	if err != nil {
		slog.Error("failed to read data from udp", "error", err)
		return
	}
	if n != len(data) {
		panic("unexpected number of bytes for data")
	}
	if firstAddr.String() != secondAddr.String() {
		panic("received data from different client")
	}

	inputs := make([]Input, size)
	for i := range size {
		inputs[i] = Input{
			From:  secondAddr,
			Index: 0,
			Up:    false,
			Left:  false,
			Down:  false,
			Right: false,
		}
	}
}

// array -> ack -> enqueue

type Input struct {
	From                  *net.UDPAddr
	Index                 uint32
	Up, Left, Down, Right bool
}

type Conn struct {
	*net.UDPConn
	inputQueue *plaiq.PlayQueue[Input]
}

func Listen(addr string) (*Conn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}

	inputQueue := plaiq.New[Input](time.Second / 60)

	conn := &Conn{
		UDPConn:    udpConn,
		inputQueue: inputQueue,
	}

	go conn.listenInput()

	return conn, nil
}

func (conn *Conn) listenInput() {
	buf := make([]byte, 1)
	n, remote, err := conn.ReadFromUDP(buf)
	if n != len(buf) {
		panic("read less bytes than needed")
	}
	if err != nil {
		slog.Error("failed to read from udp", "remote", remote, "error", err)
	}

	input := Input{
		From:  remote,
		Up:    buf[0]&(1<<0) != 0,
		Left:  buf[0]&(1<<1) != 0,
		Down:  buf[0]&(1<<2) != 0,
		Right: buf[0]&(1<<4) != 0,
	}
	conn.inputQueue.Enqueue(input)
}

func (conn *Conn) Close() error {
	err := conn.inputQueue.Close()
	if err != nil {
		return err
	}

	err = conn.UDPConn.Close()
	if err != nil {
		return err
	}

	return nil
}

func (conn *Conn) ReceiveInput() Input {
	input := conn.inputQueue.Dequeue()
	// TODO: ack received input
	return input
}

type Game struct {
	Position struct{ X, Y float64 }
}

func (g *Game) Update(input Input) {
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
