// Package mcp stands for my custom protocol.
package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"os"
	"reflect"
	"slices"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// TODO: use the terms local and remote for the public api instead of laddr and raddr
// TODO: do not start goroutines with methods

var ErrClosed = errors.New("use of closed network connection")

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
	dial   bool
	laddr  net.Addr
	logger *slog.Logger

	sessions    map[string]*Session
	sessionCond sync.Cond
	acceptCh    chan *Session

	conn    net.PacketConn
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
		dial:        false, // TODO: make into private functional option
		laddr:       conn.LocalAddr(),
		logger:      slog.With("local", conn.LocalAddr()), // TODO: make into functional option
		sessions:    map[string]*Session{},
		sessionCond: sync.Cond{L: &sync.Mutex{}},
		acceptCh:    make(chan *Session),
		conn:        conn,
		die:         make(chan struct{}),
		dieOnce:     sync.Once{},
	}
	go ln.readLoop()
	go ln.writeLoop()
	return ln, nil
}

func Dial(ctx context.Context, raddr string) (*Session, error) {
	ln, err := Listen("127.0.0.1:")
	if err != nil {
		return nil, err
	}
	ln.dial = true

	remote, err := net.ResolveUDPAddr("udp", raddr)
	if err != nil {
		return nil, err
	}

	datagram := Datagram{
		Version: version,
		Flags:   flagJoin,
		Data:    nil,
	}
	b, err := datagram.MarshalBinary()
	if err != nil {
		return nil, err
	}

	_, err = writeToWithContext(ctx, ln.logger, ln.conn, b, remote)
	if err != nil {
		return nil, err
	}

	// TODO: re-try if not acknowledged

	sess := newSession(true, ln.laddr, remote, ln)
	sess.ln = ln

	ln.sessionCond.L.Lock()
	ln.sessions[remote.String()] = sess
	ln.sessionCond.Broadcast()
	ln.sessionCond.L.Unlock()

	return sess, nil
}

// please note that the context will affect all the writes happening at the
// time of this function running.
func writeToWithContext(
	ctx context.Context,
	logger *slog.Logger,
	conn net.PacketConn,
	b []byte,
	raddr net.Addr,
) (n int, err error) {
	if deadline, ok := ctx.Deadline(); ok {
		err := conn.SetWriteDeadline(deadline)
		if err != nil {
			return 0, err
		}
	}
	defer func() { // runs after local done channel is closed
		err = errors.Join(err, conn.SetWriteDeadline(time.Time{}))
	}()

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-done:
		case <-ctx.Done():
			err := conn.SetWriteDeadline(time.Now())
			if err != nil {
				logger.WarnContext(ctx, "failed to cancel write",
					"remote", raddr,
					"error", err)
			}
		}
	}()

	n, err = conn.WriteTo(b, raddr)
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return 0, ctx.Err()
	}
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (ln *Listener) Accept(ctx context.Context) (*Session, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case sess, open := <-ln.acceptCh:
		if !open {
			return nil, ErrClosed
		}
		return sess, nil
	}
}

func (ln *Listener) writeLoop() {
WRITER:
	for {
		// TODO: holding an exclusive lock is the bottle-neck to having
		// multiple writers
		ln.sessionCond.L.Lock()
		for len(ln.sessions) == 0 {
			select {
			case _, open := <-ln.die:
				if !open {
					break WRITER
				}
			default:
			}
			ln.sessionCond.Wait()
		}
		sessions := slices.Collect(maps.Values(ln.sessions))
		ln.sessionCond.L.Unlock()

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
			if chosenIdx == len(outboxCases)-1 {
				break
			}
			continue
		}

		datagram := Datagram{
			Version: version,
			Flags:   0,
			Data:    data.Bytes(),
		}
		marshaledDatagram, err := datagram.MarshalBinary()
		if err != nil {
			ln.logger.Warn("failed to marshal datagram", "error", err)
			continue
		}
		_, err = ln.conn.WriteTo(marshaledDatagram, sessions[chosenIdx].raddr)
		if errors.Is(err, net.ErrClosed) {
			break
		}
		if err != nil {
			ln.logger.Warn("failed to write to connection",
				"remote", sessions[chosenIdx].raddr,
				"error", err)
			continue
		}
	}
}

func (ln *Listener) readLoop() {
	const bufSize = 1024

	buf := make([]byte, bufSize)
	for {
		n, raddr, readErr := ln.conn.ReadFrom(buf)
		if errors.Is(readErr, net.ErrClosed) {
			return
		}

		var datagram Datagram
		err := datagram.UnmarshalBinary(buf[:n])
		if err != nil {
			ln.logger.Warn("failed to unmarshal datagram", "error", err)
			continue
		}

		err = ln.handleDatagram(raddr, datagram)
		if err != nil {
			ln.logger.Warn("failed to handle datagram", "error", err)
			continue
		}

		if readErr != nil {
			ln.logger.Warn("failed to read from connection", "error", err)
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
		sess := newSession(false, ln.laddr, raddr, ln)
		ln.sessionCond.L.Lock()
		if _, exists := ln.sessions[raddr.String()]; exists {
			ln.sessionCond.L.Unlock()
			return fmt.Errorf("session %q: already exists", raddr)
		}
		ln.sessions[raddr.String()] = sess
		ln.sessionCond.Broadcast()
		ln.sessionCond.L.Unlock()

		// TODO: acknowledge join
		ln.acceptCh <- sess

	case datagram.Flags&flagLeave != 0:
		ln.sessionCond.L.Lock()
		if _, exists := ln.sessions[raddr.String()]; !exists {
			ln.sessionCond.L.Unlock()
			return fmt.Errorf("session %q: not found", raddr)
		}
		delete(ln.sessions, raddr.String())
		ln.sessionCond.L.Unlock()

	default:
		ln.sessionCond.L.Lock()
		sess, exists := ln.sessions[raddr.String()]
		ln.sessionCond.L.Unlock()
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

func (ln *Listener) Close(ctx context.Context) error {
	ln.logger.Debug("close listener called", "dial", ln.dial)

	ran := false
	ln.dieOnce.Do(func() {
		close(ln.die)
		close(ln.acceptCh)
		// notify ln.writeLoop to continue and realize ln is closed
		ln.sessionCond.Broadcast()
		ran = true
	})
	if !ran {
		return ErrClosed
	}

	if !ln.dial {
		g, ctx := errgroup.WithContext(ctx)
		ln.sessionCond.L.Lock()
		for _, sess := range ln.sessions {
			ln.sessionCond.L.Unlock()
			g.Go(func() error { return sess.Close(ctx) })
			ln.sessionCond.L.Lock()
		}
		ln.sessionCond.L.Unlock()
		err := g.Wait()
		if err != nil {
			return fmt.Errorf("close session: %w", err)
		}
	}

	return ln.conn.Close()
}

type Session struct {
	dial  bool
	laddr net.Addr
	raddr net.Addr

	inbox  chan []byte
	outbox chan []byte

	ln      *Listener
	die     chan struct{}
	dieOnce sync.Once
}

func newSession(dial bool, laddr, raddr net.Addr, ln *Listener) *Session {
	// NOTE: keep fields exhaustive
	return &Session{
		dial:    dial,
		laddr:   laddr,
		raddr:   raddr,
		inbox:   make(chan []byte, 1),
		outbox:  make(chan []byte, 1),
		ln:      ln,
		die:     make(chan struct{}),
		dieOnce: sync.Once{},
	}
}

func (sess *Session) Receive(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case data := <-sess.inbox:
		return data, nil
	}
}

func (sess *Session) Send(ctx context.Context, data []byte) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case sess.outbox <- data:
		return nil
	}
}

func (sess *Session) Close(ctx context.Context) error {
	slog.Debug("close session called", "local", sess.laddr, "dial", sess.dial)

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
	if sess.dial {
		err := sess.ln.Close(ctx)
		if err != nil {
			return fmt.Errorf("close listener: %w", err)
		}
	}
	// TODO: send datagram w/ flagLeave
	sess.ln.sessionCond.L.Lock()
	delete(sess.ln.sessions, sess.raddr.String())
	sess.ln.sessionCond.L.Unlock()
	return nil
}

func (sess *Session) LocalAddr() net.Addr {
	return sess.laddr
}

func (sess *Session) RemoteAddr() net.Addr {
	return sess.raddr
}
