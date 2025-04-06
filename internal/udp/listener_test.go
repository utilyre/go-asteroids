package udp_test

import (
	"context"
	"errors"
	"multiplayer/internal/udp"
	"sync"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
)

func makeListener(tb testing.TB) *udp.Listener {
	tb.Helper()

	ln, err := udp.Listen("127.0.0.1:")
	if err != nil {
		tb.Fatal(err)
	}

	tb.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := ln.Close(ctx)
		if err != nil {
			tb.Fatal(err)
		}
	})

	return ln
}

func TestListener(t *testing.T) {
	t.Run("simple message passing one to one", func(t *testing.T) {
		var mu sync.Mutex
		tests := map[string]struct{}{
			"ping 1": {},
			"ping 2": {},
			"ping 3": {},
			"ping 4": {},
			"ping 5": {},
			"ping 6": {},
			"ping 7": {},
			"ping 8": {},
			"ping 9": {},
		}
		n := len(tests)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		server := makeListener(t)
		t.Logf("server bound to udp %q", server.LocalAddr())
		client := makeListener(t)
		t.Logf("client bound to udp %q", client.LocalAddr())

		err := client.Greet(ctx, server.LocalAddr())
		if err != nil {
			t.Fatal(err)
		}

		g, ctx := errgroup.WithContext(ctx)
		g.Go(func() error {
			mu.Lock()
			for test := range tests {
				mu.Unlock()
				msg := udp.NewMessage([]byte(test))
				err = client.TrySend(ctx, server.LocalAddr(), msg)
				if err != nil {
					return err
				}
				t.Logf("sent message %q to server", msg)
				mu.Lock()
			}
			mu.Unlock()
			return nil
		})

		g.Go(func() error {
			for range n {
				select {
				case <-ctx.Done():
					break
				case envel := <-server.Chan():
					t.Logf("received message %q from client", envel.Message)
					if addr := client.LocalAddr(); addr.String() != envel.Sender.String() {
						t.Errorf("expected client %q; actual client %q", addr, envel.Sender)
					}
					body := string(envel.Message.Body)
					mu.Lock()
					if _, exists := tests[body]; !exists {
						t.Errorf("unexpected message %q", envel.Message)
					}
					delete(tests, body)
					mu.Unlock()
				}
			}

			if len(tests) > 0 {
				t.Errorf("missed messages %#v", tests)
			}

			return nil
		})

		err = g.Wait()
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("race condition in greet and farewell", func(t *testing.T) {
		const n = 10 // number of goroutines

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		server := makeListener(t)
		t.Logf("server bound to udp %q", server.LocalAddr())
		client := makeListener(t)
		t.Logf("client bound to udp %q", client.LocalAddr())

		g, ctx := errgroup.WithContext(ctx)

		for range n {
			g.Go(func() error {
				err := client.Greet(ctx, server.LocalAddr())
				if err != nil {
					return err
				}
				err = client.Farewell(ctx, server.LocalAddr())
				if err != nil {
					return err
				}
				return nil
			})
		}

		err := g.Wait()
		if err != nil && !errors.Is(err, udp.ErrAlreadyGreeted) {
			t.Fatal(err)
		}
	})
}
