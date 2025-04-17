// Package mcp stands for my custom protocol.
package mcp

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
)

var ErrClosed = errors.New("connection closed")

const version byte = 1

const (
	flagJoin uint16 = 1 << iota
	flagLeave
)

// how is this any different from net.PacketConn?
//
// 1. broadcast (channels)
// 2. ack via checksum
// 3. sessions (multiplex incoming messages)

type Listener struct {
	laddr net.Addr

	conn net.PacketConn

	sessions    map[string]*Session
	sessionLock sync.Mutex
	acceptCh    chan *Session
}

func (ln *Listener) LocalAddr() net.Addr {
	return ln.laddr
}

func Listen(laddr string) (*Listener, error) {
	conn, err := net.ListenPacket("udp", laddr)
	if err != nil {
		return nil, err
	}

	// NOTE: keep fields exhaustive
	ln := &Listener{
		laddr:       conn.LocalAddr(),
		conn:        conn,
		sessions:    map[string]*Session{},
		sessionLock: sync.Mutex{},
		acceptCh:    make(chan *Session),
	}
	go ln.readLoop()
	return ln, nil
}

func Dial(raddr string) (*Session, error) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:")
	if err != nil {
		return nil, err
	}

	remote, err := net.ResolveUDPAddr("udp", raddr)
	if err != nil {
		return nil, err
	}

	sess := newSession(conn.LocalAddr(), remote, conn)

	datagram := Datagram{
		Version: version,
		Flags:   flagJoin,
		Data:    nil,
	}
	data, err := datagram.MarshalBinary()
	if err != nil {
		return nil, err
	}

	_, err = conn.WriteTo(data, remote)
	if err != nil {
		return nil, err
	}

	return sess, nil
}

func (ln *Listener) Accept() (*Session, error) {
	sess, open := <-ln.acceptCh
	if !open {
		return nil, ErrClosed
	}
	return sess, nil
}

func (ln *Listener) readLoop() {
	const bufSize = 1024

	buf := make([]byte, bufSize)
	for {
		n, raddr, readErr := ln.conn.ReadFrom(buf)
		if errors.Is(readErr, net.ErrClosed) {
			slog.Info("connection closed", "local", ln.laddr)
			return
		}

		var datagram Datagram
		err := datagram.UnmarshalBinary(buf[:n])
		if err != nil {
			slog.Debug("failed to unmarshal datagram", "error", err)
			continue
		}

		err = ln.handleDatagram(raddr, datagram)
		if err != nil {
			slog.Warn("failed to handle datagram", "error", err)
			continue
		}
	}
}

func assertDatagram(datagram Datagram) {
	if datagram.Version != version {
		panic("mcp error: unsupported protocol version")
	}
	if datagram.Flags&flagJoin != 0 && datagram.Flags&flagLeave != 0 {
		panic("mcp error: unknown datagram flags state")
	}
}

func (ln *Listener) handleDatagram(raddr net.Addr, datagram Datagram) error {
	assertDatagram(datagram)

	switch {
	case datagram.Flags&flagJoin != 0:
		sess := newSession(ln.laddr, raddr, ln.conn)
		ln.sessionLock.Lock()
		if _, exists := ln.sessions[raddr.String()]; exists {
			ln.sessionLock.Unlock()
			return fmt.Errorf("session %q: already exists", raddr)
		}
		ln.sessions[raddr.String()] = sess
		ln.sessionLock.Unlock()

		// TODO: acknowledge join
		ln.acceptCh <- sess

	case datagram.Flags&flagLeave != 0:
		ln.sessionLock.Lock()
		if _, exists := ln.sessions[raddr.String()]; !exists {
			ln.sessionLock.Unlock()
			return fmt.Errorf("session %q: not found", raddr)
		}
		delete(ln.sessions, raddr.String())
		ln.sessionLock.Unlock()

	default:
		ln.sessionLock.Lock()
		sess, exists := ln.sessions[raddr.String()]
		ln.sessionLock.Unlock()
		if !exists {
			return fmt.Errorf("deliver datagram %q: session %q: not found",
				datagram, raddr)
		}

		// try to deliver the data
		select {
		case sess.inbox <- datagram.Data:
		default:
		}
	}

	return nil
}

func (ln *Listener) Close() error {
	close(ln.acceptCh)
	return ln.conn.Close()
}

type writerTo interface {
	WriteTo(p []byte, addr net.Addr) (n int, err error)
}

type Session struct {
	laddr net.Addr
	raddr net.Addr

	outbox  writerTo
	inbox   chan []byte
	die     chan struct{}
	dieOnce sync.Once
}

func newSession(laddr, raddr net.Addr, outbox writerTo) *Session {
	// NOTE: keep fields exhaustive
	return &Session{
		laddr:   laddr,
		raddr:   raddr,
		outbox:  outbox,
		inbox:   make(chan []byte, 1),
		die:     make(chan struct{}),
		dieOnce: sync.Once{},
	}
}

func (sess *Session) Receive() []byte {
	return <-sess.inbox
}

func (sess *Session) Send(data []byte) error {
	datagram := Datagram{
		Version: version,
		Flags:   0,
		Data:    data,
	}
	b, err := datagram.MarshalBinary()
	if err != nil {
		return err
	}
	_, err = sess.outbox.WriteTo(b, sess.raddr)
	if err != nil {
		return err
	}
	return nil
}

func (sess *Session) Close() error {
	once := false
	sess.dieOnce.Do(func() {
		close(sess.die)
		close(sess.inbox)
		once = true
	})
	if !once {
		return io.ErrClosedPipe
	}
	return nil
}

func (sess *Session) LocalAddr() net.Addr {
	return sess.laddr
}

func (sess *Session) RemoteAddr() net.Addr {
	return sess.raddr
}
