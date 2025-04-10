package main

import (
	"log/slog"
	"multiplayer/internal/types"
	"sync/atomic"
)

type InputQueue struct {
	ch        chan types.Input
	lastIndex atomic.Uint32
}

func NewInputQueue() *InputQueue {
	return &InputQueue{ch: make(chan types.Input, 1)}
}

func (q *InputQueue) Close() {
	close(q.ch)
}

func (q *InputQueue) ProcessInputs(inputs []types.Input) {
	lastIdx := q.lastIndex.Load()
	for _, input := range inputs {
		if input.Index <= lastIdx {
			continue
		}

		slog.Debug("wanna break from the ads?")
		q.ch <- input
		slog.Debug("enqueued input", "index", input.Index)
		q.lastIndex.Store(input.Index)
	}
}

func (q *InputQueue) Dequeue() (input types.Input, open bool) {
	input, open = <-q.ch
	return input, open
}
