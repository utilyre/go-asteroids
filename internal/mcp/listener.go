// Package mcp stands for my custom protocol.
package mcp

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"reflect"
	"slices"
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
	sessionLock sync.RWMutex
	acceptCh    chan *Session

	die     chan struct{}
	dieOnce sync.Once
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
		sessionLock: sync.RWMutex{},
		acceptCh:    make(chan *Session),
		die:         make(chan struct{}),
		dieOnce:     sync.Once{},
	}
	go ln.readLoop()
	go ln.writeLoop()
	return ln, nil
}

func Dial(raddr string) (*Session, error) {
	ln, err := Listen("127.0.0.1:")
	if err != nil {
		return nil, err
	}

	remote, err := net.ResolveUDPAddr("udp", raddr)
	if err != nil {
		return nil, err
	}
	sess, err := ln.join(remote)
	if err != nil {
		return nil, err
	}

	return sess, nil
}

func (ln *Listener) join(raddr net.Addr) (*Session, error) {
	datagram := Datagram{
		Version: version,
		Flags:   flagJoin,
		Data:    nil,
	}
	b, err := datagram.MarshalBinary()
	if err != nil {
		return nil, err
	}
	_, err = ln.conn.WriteTo(b, raddr)
	if err != nil {
		return nil, err
	}

	// TODO: re-try if not acknowledged

	sess := newSession(ln.laddr, raddr)
	ln.sessionLock.Lock()
	ln.sessions[raddr.String()] = sess
	ln.sessionLock.Unlock()
	return sess, nil
}

func (ln *Listener) Accept() (*Session, error) {
	sess, open := <-ln.acceptCh
	if !open {
		return nil, ErrClosed
	}
	return sess, nil
}

func (ln *Listener) writeLoop() {
	do := func() bool {
		ln.sessionLock.RLock()
		defer ln.sessionLock.RUnlock()

		sessions := slices.Collect(maps.Values(ln.sessions))

		// TODO: use a notification mechanism instead of skipping iteration and
		// ending up with a busy loop
		if len(sessions) == 0 {
			return true
		}

		outboxCases := make([]reflect.SelectCase, len(sessions)+1)
		for i, sess := range sessions {
			outboxCases[i] = reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(sess.outbox),
			}
		}
		outboxCases[len(outboxCases)-1] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ln.die),
		}

		chosenIdx, data, open := reflect.Select(outboxCases)
		if !open {
			// do not continue if ln.die is closed
			return chosenIdx != len(outboxCases)-1
		}

		datagram := Datagram{
			Version: version,
			Flags:   0,
			Data:    data.Bytes(),
		}
		marshaledDatagram, err := datagram.MarshalBinary()
		if err != nil {
			slog.Warn("failed to marshal datagram", "error", err)
			return true
		}
		_, err = ln.conn.WriteTo(marshaledDatagram, sessions[chosenIdx].raddr)
		if err != nil {
			slog.Warn("failed to write datagram to session remote",
				"remote", sessions[chosenIdx].raddr, "error", err)
			return true
		}

		return true
	}

	for {
		if !do() {
			return
		}
	}
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
			slog.Warn("failed to unmarshal datagram", "error", err)
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
		sess := newSession(ln.laddr, raddr)
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
	ran := false
	ln.dieOnce.Do(func() {
		close(ln.die)
		close(ln.acceptCh)
		ran = true
	})
	if !ran {
		return ErrClosed
	}
	// TODO: close all associated sessions
	return ln.conn.Close()
}

type Session struct {
	laddr net.Addr
	raddr net.Addr

	inbox   chan []byte
	outbox  chan []byte
	die     chan struct{}
	dieOnce sync.Once
}

func newSession(laddr, raddr net.Addr) *Session {
	// NOTE: keep fields exhaustive
	return &Session{
		laddr:   laddr,
		raddr:   raddr,
		outbox:  make(chan []byte, 1),
		inbox:   make(chan []byte, 1),
		die:     make(chan struct{}),
		dieOnce: sync.Once{},
	}
}

func (sess *Session) Receive() []byte {
	return <-sess.inbox
}

func (sess *Session) Send(data []byte) {
	sess.outbox <- data
}

func (sess *Session) Close() error {
	once := false
	sess.dieOnce.Do(func() {
		close(sess.die)
		close(sess.outbox)
		close(sess.inbox)
		once = true
	})
	if !once {
		return ErrClosed
	}
	return nil
}

func (sess *Session) LocalAddr() net.Addr {
	return sess.laddr
}

func (sess *Session) RemoteAddr() net.Addr {
	return sess.raddr
}
