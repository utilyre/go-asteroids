package main

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"multiplayer/internal/types"
	"net"
)

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

func main() {
	srv, err := NewGameServer(":3000")
	if err != nil {
		slog.Error("failed to instantiate game server", "error", err)
		return
	}

	for {
		buf := make([]byte, 1024)

		n, addr, err := srv.conn.ReadFrom(buf)
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
			panic("we are in trouble") // HIT
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

		lastInput := inputs[len(inputs)-1]
		slog.Info("last input", "idx", lastInput.Index, "left", lastInput.Left)

		lastIndexData := make([]byte, 4)
		n, err = binary.Encode(lastIndexData, binary.BigEndian, lastInput.Index)
		if err != nil {
			panic("should have enough space")
		}
		if n != len(lastIndexData) {
			panic("no way")
		}

		n, err = srv.conn.WriteTo(lastIndexData, addr)
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

type Input struct {
	Index                 uint32
	Up, Left, Down, Right bool
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
