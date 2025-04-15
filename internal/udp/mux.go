package udp

import (
	"log/slog"
	"sync/atomic"
	"time"
)

func NewMessageWithLabel(body []byte, label byte) Message {
	newBody := make([]byte, 1, 1+len(body))
	newBody[0] = label
	newBody = append(newBody, body...)
	return NewMessage(newBody)
}

type Mux struct {
	ln      *Listener
	topics  map[byte]chan Envelope // maps labels to topics
	running atomic.Bool
}

func NewMux(ln *Listener) *Mux {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for ; ; <-ticker.C {
			n := muxRate.Load()
			slog.Debug("mux rate", "rate", n)
			muxRate.Store(0)
		}
	}()

	return &Mux{
		ln:     ln,
		topics: map[byte]chan Envelope{},
	}
}

// NOTE: does not close mux.ln
func (mux *Mux) Close() error {
	for _, ch := range mux.topics {
		close(ch)
	}
	return nil
}

func (mux *Mux) Subscribe(label byte, queueSize int) <-chan Envelope {
	if mux.running.Load() {
		panic("mux error: cannot subscribe to labels while running")
	}

	topic := make(chan Envelope, queueSize)
	mux.topics[label] = topic
	return topic
}

var muxRate atomic.Uint32

func (mux *Mux) Run() {
	mux.running.Store(true)
	defer mux.running.Store(false)

	for envel := range mux.ln.Inbox() {
		if len(envel.Message.Body) < 1 {
			slog.Warn("message too short to have a label",
				"sender", envel.Sender, "message", envel.Message)
			continue
		}

		label := envel.Message.Body[0]
		envel.Message.Body = envel.Message.Body[1:] // omit the label

		topic, exists := mux.topics[label]
		if !exists {
			slog.Warn(
				"dropping udp message as there are no topics for its label",
				"sender", envel.Sender,
				"message", envel.Message,
				"label", label,
			)
			continue
		}

		topic <- envel
		muxRate.Add(1)
	}
}
