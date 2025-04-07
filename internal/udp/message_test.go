package udp_test

import (
	"multiplayer/internal/udp"
	"testing"
)

func FuzzMessage_Binary(f *testing.F) {
	tests := [...][]byte{
		nil,
		{},
		{1},
		[]byte("Hello, world"),
		[]byte("👋"),
	}

	for _, test := range tests {
		f.Add(test)
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		orig := udp.NewMessage(body)
		data, err := orig.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		var parsed udp.Message
		err = parsed.UnmarshalBinary(data)
		if err != nil {
			t.Fatal(err)
		}

		if !orig.Equal(parsed) {
			t.Errorf("expected message %q; actual message %q", orig, parsed)
		}
	})
}
