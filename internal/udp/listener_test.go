package udp_test

import (
	"context"
	"math/rand/v2"
	"multiplayer/internal/udp"
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
		err := ln.Close()
		if err != nil {
			tb.Fatal(err)
		}
	})

	return ln
}

func randSleep() {
	dt := rand.N(300) + 100
	time.Sleep(time.Duration(dt) * time.Millisecond)
}

func TestListener(t *testing.T) {
	t.Run("simple message passing one to one", func(t *testing.T) {
		server := makeListener(t)
		t.Logf("server bound to udp %q", server.LocalAddr())
		client := makeListener(t)
		t.Logf("client bound to udp %q", client.LocalAddr())

		err := client.Greet(server.LocalAddr())
		if err != nil {
			t.Fatal(err)
		}

		msg := udp.NewMessage([]byte("ping"))

		done := make(chan struct{})
		go func() {
			envel := <-server.C
			if addr := client.LocalAddr(); addr.String() != envel.Sender.String() {
				t.Errorf("expected client %q; actual client %q", addr, envel.Sender)
			}
			if !msg.Equal(envel.Message) {
				t.Errorf("expected message %q; actual message %q", msg, envel.Message)
			}
			close(done)
		}()

		t.Logf("sending message %q to server", msg)
		err = client.Send(server.LocalAddr(), msg)
		if err != nil {
			t.Fatal(err)
		}

		t.Log("waiting for server to receive message")
		<-done
	})

	t.Run("race condition in greet and farewell", func(t *testing.T) {
		server := makeListener(t)
		t.Logf("server bound to udp %q", server.LocalAddr())
		client := makeListener(t)
		t.Logf("client bound to udp %q", client.LocalAddr())

		g, _ := errgroup.WithContext(context.TODO())
		g.Go(func() error {
			err := client.Greet(server.LocalAddr())
			if err != nil {
				return err
			}
			err = client.Farewell(server.LocalAddr())
			if err != nil {
				return err
			}
			return nil
		})
		g.Go(func() error {
			err := client.Greet(server.LocalAddr())
			if err != nil {
				return err
			}
			err = client.Farewell(server.LocalAddr())
			if err != nil {
				return err
			}
			return nil
		})

		err := g.Wait()
		if err != nil {
			t.Fatal(err)
		}
	})
}
