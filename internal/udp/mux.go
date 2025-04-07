package udp

import (
	"log/slog"
	"sync/atomic"
)

func NewMessageWithLabel(body []byte, label byte) Message {
	newBody := make([]byte, 1, 1+len(body))
	newBody[0] = label
	newBody = append(newBody, body...)
	return NewMessage(newBody)
}

type Mux struct {
	ln       *Listener
	channels map[byte]chan Envelope // maps a label to its channel
	running  atomic.Bool
}

func NewMux(ln *Listener) *Mux {
	return &Mux{
		ln:       ln,
		channels: map[byte]chan Envelope{},
	}
}

// NOTE: does not close mux.ln
func (mux *Mux) Close() error {
	for _, ch := range mux.channels {
		close(ch)
	}
	return nil
}

func (mux *Mux) Subscribe(label byte, queueSize int) <-chan Envelope {
	if mux.running.Load() {
		panic("mux error: cannot subscribe to labels while running")
	}

	ch := make(chan Envelope, queueSize)
	mux.channels[label] = ch
	return ch
}

func (mux *Mux) Run() {
	mux.running.Store(true)
	for envel := range mux.ln.Inbox() {
		if len(envel.Message.Body) < 1 {
			slog.Warn("message too short to have a label",
				"sender", envel.Sender, "message", envel.Message)
			continue
		}

		label := envel.Message.Body[0]
		ch, exists := mux.channels[label]
		if !exists {
			slog.Warn(
				"failed to find a subscriber for the label, dropping the message",
				"sender", envel.Sender,
				"message", envel.Message,
				"label", label,
			)
			continue
		}

		ch <- envel
	}
	mux.running.Store(false)
}
