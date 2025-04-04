package udp_test

import (
	"fmt"
	"multiplayer/internal/udp"
	"testing"
)

func TestMessage_MarshalBinary_UnmarshalBinary(t *testing.T) {
	tests := []udp.Message{
		udp.NewMessage(nil),
		udp.NewMessage([]byte{}),
		udp.NewMessage([]byte{1}),
		udp.NewMessage([]byte("Hello, world")),
		udp.NewMessage([]byte("👋")),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test case %02d", i), func(t *testing.T) {
			data, err := test.MarshalBinary()
			if err != nil {
				t.Fatal(t)
			}

			var msg udp.Message
			err = msg.UnmarshalBinary(data)
			if err != nil {
				t.Fatal(t)
			}

			if !test.Equal(msg) {
				t.Errorf("expected message %q; actual message %q", test, msg)
			}
		})
	}
}
