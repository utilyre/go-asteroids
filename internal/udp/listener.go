package udp

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"slices"
)

func init() {
	// slog.SetLogLoggerLevel(slog.LevelDebug)
}

type Envelope struct {
	Sender net.Addr
	Message
}

type Listener struct {
	C chan Envelope

	conn    net.PacketConn
	clients map[string]struct{} // set of active addrs
	//          ^ TODO: consider using uuid.UUID
	servers map[string]struct{}
	//          ^ TODO: consider using uuid.UUID
	// FIX: use sync.Mutex for maps
}

func Listen(addr string) (*Listener, error) {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("binding to udp %q: %w", addr, err)
	}

	ln := &Listener{
		C:       make(chan Envelope, 1),
		conn:    conn,
		clients: map[string]struct{}{},
		servers: map[string]struct{}{},
	}
	go ln.readLoop()

	return ln, nil
}

func (ln *Listener) Close() error {
	for addr := range ln.servers {
		udpAddr, _ := net.ResolveUDPAddr("udp", addr)
		err := ln.Farewell(udpAddr)
		if err != nil {
			return fmt.Errorf("farewelling servers: %w", err)
		}
	}

	err := ln.conn.Close()
	if err != nil {
		return fmt.Errorf("closing udp %q: %w", ln.LocalAddr(), err)
	}

	close(ln.C)

	return nil
}

func (ln *Listener) LocalAddr() net.Addr { return ln.conn.LocalAddr() }

func (ln *Listener) RemoteAddrs() []string {
	//                            ^ TODO: convert to net.Addr, or even better uuid.UUID
	return slices.Collect(maps.Keys(ln.clients))
}

var (
	ErrAlreadyGreeted = errors.New("already greeted")
	ErrServerNotFound = errors.New("server not found")
)

func (ln *Listener) Greet(dest net.Addr) error {
	if _, exists := ln.servers[dest.String()]; exists {
		return ErrAlreadyGreeted
	}

	msg := newMessage(nil, flagHi)
	err := ln.Send(dest, msg)
	if err != nil {
		return err
	}
	ln.servers[dest.String()] = struct{}{}
	return nil
}

func (ln *Listener) Farewell(dest net.Addr) error {
	if _, exists := ln.servers[dest.String()]; !exists {
		return ErrServerNotFound
	}

	msg := newMessage(nil, flagBye)
	err := ln.Send(dest, msg)
	if err != nil {
		return err
	}
	delete(ln.servers, dest.String())
	return nil
}

func (ln *Listener) Send(dest net.Addr, msg Message) error {
	data, err := msg.MarshalBinary()
	if err != nil {
		return fmt.Errorf("marshaling message: %w", err)
	}

	_, err = ln.conn.WriteTo(data, dest)
	if err != nil {
		return fmt.Errorf("writing message to udp %q: %w", dest, err)
	}

	return nil
}

const bufSize = 1024

func (ln *Listener) readLoop() {
	buf := make([]byte, bufSize)
	for {
		n, addr, readErr := ln.conn.ReadFrom(buf)
		if errors.Is(readErr, net.ErrClosed) {
			// TODO: remove from ln.clients if not already
			slog.Info("connection closed", "address", addr)
			break
		}

		var msg Message
		err := msg.UnmarshalBinary(buf[:n])
		if err != nil {
			slog.Debug("failed to unmarshal message", "error", err)
			continue
		}

		slog.Debug("got message", "sender", addr, "msg", msg)

		if msg.flags&flagHi != 0 {
			if _, exists := ln.clients[addr.String()]; !exists {
				ln.clients[addr.String()] = struct{}{}
				slog.Debug("somebody just connected", "address", addr)
			} else {
				slog.Debug("somebody tried to connect more than once",
					"address", addr)
			}
			continue
		}
		if msg.flags&flagBye != 0 {
			delete(ln.clients, addr.String())
			slog.Debug("somebody just disconnected", "address", addr)
			continue
		}

		ln.C <- Envelope{
			Sender:  addr,
			Message: msg,
		}

		if readErr != nil {
			slog.Warn("failed to read from udp", "error", err)
			continue
		}
	}
}
