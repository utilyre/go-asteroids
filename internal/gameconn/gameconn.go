package gameconn

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
)

const (
	protocolVersion byte = 1
	headerSize      int  = 2
	bufSize         int  = 1024
)

type Message struct {
	Scope byte
	Body  []byte
}

type Handler func(sender net.Addr, msg *Message)

type Conn struct {
	inner    net.PacketConn
	addr     net.Addr
	handlers map[byte]Handler
	addrs    map[string]struct{}
}

func Listen(address string) (*Conn, error) {
	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		return nil, err
	}

	c := &Conn{
		inner:    conn,
		addr:     conn.LocalAddr(),
		handlers: make(map[byte]Handler),
	}

	go c.listener()
	return c, nil
}

func (conn *Conn) Close() error {
	// TODO: make sure all the handlers are finished
	return conn.inner.Close()
}

func (conn *Conn) Handle(scope byte, handler Handler) {
	conn.handlers[scope] = handler
}

func (conn *Conn) LocalAddr() net.Addr {
	return conn.addr
}

func (conn *Conn) Send(addr net.Addr, msg *Message) error {
	buf := make([]byte, headerSize, headerSize+len(msg.Body))

	buf[0] = protocolVersion
	buf[1] = msg.Scope
	buf = append(buf, msg.Body...)

	_, err := conn.inner.WriteTo(buf, addr)
	if err != nil {
		return fmt.Errorf("writing to udp %q: %w", addr, err)
	}
	return nil
}

func (conn *Conn) SendAll(msg *Message) error {
	var errs []error

	for addr := range conn.addrs {
		actualAddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		err = conn.Send(actualAddr, msg)
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}

	return errors.Join(errs...)
}

const (
	ScopeHi  byte = 0
	ScopeBye byte = 1
)

func (conn *Conn) listener() {
	for {
		msg, addr, err := conn.readMessage()
		if errors.Is(err, net.ErrClosed) {
			slog.Info("connection closed", "address", conn.addr)
			break
		}
		if err != nil {
			slog.Error("failed to read message",
				"address", conn.addr, "error", err)
			continue
		}

		if msg.Scope == ScopeHi {
			if conn.addrs == nil {
				conn.addrs = map[string]struct{}{}
			}

			conn.addrs[addr.String()] = struct{}{}
			continue
		}
		if msg.Scope == ScopeBye {
			delete(conn.addrs, addr.String())
			continue
		}

		handle, exists := conn.handlers[msg.Scope]
		if !exists {
			slog.Warn("handler not found", "scope", msg.Scope)
			continue
		}

		handle(addr, msg)
	}
}

var (
	ErrCorruptedMessage   = errors.New("message corrupted")
	ErrUnsupportedVersion = errors.New("version not supported")
)

func (conn *Conn) readMessage() (*Message, net.Addr, error) {
	buf := make([]byte, bufSize)
	n, addr, err := conn.inner.ReadFrom(buf)
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return nil, nil, fmt.Errorf("reading from udp %q: %w", addr, err)
	}
	if n < headerSize {
		return nil, nil, fmt.Errorf("reading from udp %q: %w",
			addr, ErrCorruptedMessage)
	}

	if v := buf[0]; v != protocolVersion {
		return nil, nil, fmt.Errorf("protocol version %d: %w",
			v, ErrUnsupportedVersion)
	}

	msg := &Message{
		Scope: buf[1],
		Body:  buf[2:n],
	}

	if err != nil {
		return nil, nil, fmt.Errorf("reading from udp %q: %w", addr, err)
	}

	return msg, addr, nil
}
