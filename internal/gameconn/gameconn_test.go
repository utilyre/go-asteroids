package gameconn_test

import (
	"multiplayer/internal/gameconn"
	"net"
	"slices"
	"testing"
)

func TestConn(t *testing.T) {
	server, err := gameconn.Listen("127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := server.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()
	t.Logf("server started on %q", server.LocalAddr())

	client, err := gameconn.Listen("127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := client.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()
	t.Logf("client started on %q", client.LocalAddr())

	msg := &gameconn.Message{
		Scope: 69,
		Body:  []byte("hello world"),
	}

	server.Handle(msg.Scope, func(sender net.Addr, rmsg *gameconn.Message) {
		if sender.String() != client.LocalAddr().String() {
			t.Errorf("expected sender %q; actual sender %q",
				client.LocalAddr(), sender)
		}
		if rmsg.Scope != msg.Scope {
			t.Errorf("expected scope %d; actual scope %d",
				msg.Scope, rmsg.Scope)
		}
		if !slices.Equal(rmsg.Body, msg.Body) {
			t.Errorf("expected body %q; actual body %q", msg.Body, rmsg.Body)
		}
	})

	err = client.Send(server.LocalAddr(), msg)
	if err != nil {
		t.Fatal(err)
	}
}
