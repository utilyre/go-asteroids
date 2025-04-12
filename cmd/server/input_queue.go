package main

import (
	"log/slog"
	"multiplayer/internal/types"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	inputChanCap  = 10
	cancelChanCap = 5
	inputTimeout  = time.Second / 60
)

type InputQueue struct {
	inputc         chan types.Input
	cancelTimeoutc chan struct{}
	lastIndices    map[string]*atomic.Uint32
}

var (
	statisticsMu sync.Mutex
	maxInputLen  int
	maxCancelLen int
)

func NewInputQueue() *InputQueue {
	q := &InputQueue{
		inputc:         make(chan types.Input, inputChanCap),
		cancelTimeoutc: make(chan struct{}, cancelChanCap),
		lastIndices:    map[string]*atomic.Uint32{},
	}

	go func() {
		ticker := time.NewTicker(time.Second / 60)
		defer ticker.Stop()

		for ; ; <-ticker.C {
			statisticsMu.Lock()
			maxInputLen = max(maxInputLen, len(q.inputc))
			maxCancelLen = max(maxCancelLen, len(q.cancelTimeoutc))
			statisticsMu.Lock()
		}
	}()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for ; ; <-ticker.C {
			statisticsMu.Lock()
			slog.Debug("input queue load",
				"max_input_len", maxInputLen,
				"max_cancel_len", maxCancelLen,
			)
			statisticsMu.Unlock()
		}
	}()

	return q
}

func (q *InputQueue) Close() {
	close(q.inputc)
}

func (q *InputQueue) ProcessInputs(sender net.Addr, inputs []types.Input) {
	senderStr := sender.String()
	if _, exists := q.lastIndices[senderStr]; !exists {
		q.lastIndices[senderStr] = &atomic.Uint32{}
	}
	lastIdx := q.lastIndices[senderStr].Load()

	for _, input := range inputs {
		if input.Index <= lastIdx {
			continue
		}

		q.inputc <- input
		q.lastIndices[senderStr].Store(input.Index)

		go func() {
			select {
			case <-time.After(inputTimeout):
				<-q.inputc
			case <-q.cancelTimeoutc:
			}
		}()
	}
}

func (q *InputQueue) Dequeue() (input types.Input, open bool) {
	input, open = <-q.inputc
	q.cancelTimeoutc <- struct{}{}
	return input, open
}
