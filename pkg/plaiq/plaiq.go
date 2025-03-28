package plaiq

import (
	"sync/atomic"
	"time"
)

// plays the buffer nicely delayed
// when buffer becomes empty waits until it's full
type PlayQueue[T any] struct {
	config
	queue    chan T
	overflow chan struct{}
	playing  atomic.Bool
}

type Option func(cfg *config)

func WithCapacity(capacity int) Option {
	return func(cfg *config) {
		cfg.capacity = capacity
	}
}

type config struct {
	interval time.Duration
	capacity int
}

func New[T any](interval time.Duration, opts ...Option) *PlayQueue[T] {
	cfg := config{
		interval: interval,
		capacity: 1,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	return &PlayQueue[T]{
		config:   cfg,
		queue:    make(chan T, cfg.capacity),
		overflow: make(chan struct{}),
		playing:  atomic.Bool{},
	}
}

func (q *PlayQueue[T]) Close() error {
	close(q.overflow)
	close(q.queue)
	return nil
}

func (q *PlayQueue[T]) Enqueue(value T) {
	// TODO: add context param
	select {
	case q.queue <- value:
	default:
		select {
		case q.overflow <- struct{}{}:
		default:
		}
		q.queue <- value
	}
}

func (q *PlayQueue[T]) Dequeue() T {
	// TODO: add context param
	if !q.playing.Load() {
		<-q.overflow
		q.playing.Store(true)
	}

	t := time.After(q.interval)
	value := <-q.queue
	<-t

	if len(q.queue) == 0 {
		q.playing.Store(false)
	}

	return value
}
