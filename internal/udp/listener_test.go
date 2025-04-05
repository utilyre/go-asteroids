package udp_test

import (
	"context"
	"multiplayer/internal/udp"
	"testing"

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
		client1 := makeListener(t)
		t.Logf("client1 bound to udp %q", client1.LocalAddr())
		client2 := makeListener(t)
		t.Logf("client2 bound to udp %q", client2.LocalAddr())

		g, _ := errgroup.WithContext(context.TODO())
		g.Go(func() error { return client1.Greet(server.LocalAddr()) })
		g.Go(func() error { return client2.Greet(server.LocalAddr()) })
		err := g.Wait()
		if err != nil {
			t.Fatal(err)
		}

		g, _ = errgroup.WithContext(context.TODO())
		g.Go(func() error { return client2.Farewell(server.LocalAddr()) })
		g.Go(func() error { return client1.Farewell(server.LocalAddr()) })
		err = g.Wait()
		if err != nil {
			t.Fatal(err)
		}
	})
}
