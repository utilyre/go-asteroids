package udp_test

import (
	"multiplayer/internal/udp"
	"testing"
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
	t.Run("server:1 client:1", func(t *testing.T) {
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
}
