package main

import (
	"log/slog"
	"multiplayer/internal/types"
	"net"
	"sync/atomic"
)

type InputQueue struct {
	ch          chan types.Input
	lastIndices map[string]*atomic.Uint32
}

func NewInputQueue() *InputQueue {
	return &InputQueue{
		ch:          make(chan types.Input, 1),
		lastIndices: map[string]*atomic.Uint32{},
	}
}

func (q *InputQueue) Close() {
	close(q.ch)
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

		slog.Debug("wanna break from the ads?")
		q.ch <- input
		slog.Debug("enqueued input", "index", input.Index)
		q.lastIndices[senderStr].Store(input.Index)
	}
}

func (q *InputQueue) Dequeue() (input types.Input, open bool) {
	input, open = <-q.ch
	return input, open
}
