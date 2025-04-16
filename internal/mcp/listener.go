package mcp

import (
	"io"
	"net"
	"sync"
	"time"
)

type Listener struct {
	conn net.PacketConn

	sessions    map[string]*Session
	sessionLock sync.RWMutex

	die     chan struct{}
	dieOnce sync.Once
}

func (ln *Listener) Close() error {
	once := false
	ln.dieOnce.Do(func() {
		close(ln.die)
		once = true
	})

	if !once {
		return io.ErrClosedPipe
	}
}

// TODO: broadcast?????????????????

type Session struct {
	conn net.PacketConn
	ln   *Listener

	remote net.Addr

	die     chan struct{}
	dieOnce sync.Once
}

func (sess *Session) Read(b []byte) (int, error) {
	panic("TODO")
}

func (sess *Session) Write(b []byte) (int, error) {
	panic("TODO")
}

func (sess *Session) Close() error {
	once := false
	sess.dieOnce.Do(func() {
		close(sess.die)
		once = true
	})
	if !once {
		return io.ErrClosedPipe
	}
	return sess.conn.Close()
}

func (sess *Session) LocalAddr() net.Addr {
	return sess.conn.LocalAddr()
}

func (sess *Session) RemoteAddr() net.Addr {
	return sess.remote
}

func (sess *Session) SetDeadline(t time.Time) error {
	return sess.conn.SetDeadline(t)
}

func (sess *Session) SetReadDeadline(t time.Time) error {
	return sess.conn.SetReadDeadline(t)
}

func (sess *Session) SetWriteDeadline(t time.Time) error {
	return sess.conn.SetWriteDeadline(t)
}
