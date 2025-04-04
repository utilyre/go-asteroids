package udp

import (
	"bytes"
	"errors"
	"fmt"
)

type Message struct {
	version byte
	flags   byte
	Body    []byte
}

func NewMessage(body []byte) Message {
	return Message{
		version: msgVersion,
		flags:   0,
		Body:    body,
	}
}

func newMessage(body []byte, flags ...byte) Message {
	var f byte
	for _, flag := range flags {
		f |= flag
	}

	return Message{
		version: msgVersion,
		flags:   f,
		Body:    body,
	}
}

func (msg Message) Equal(other Message) bool {
	return msg.flags == other.flags && bytes.Equal(msg.Body, other.Body)
}

func (msg Message) String() string {
	return fmt.Sprintf("Message:v%d(%x)", msg.version, msg.Body)
}

const (
	msgVersion    byte = 1
	msgHeaderSize      = /* version: */ 1 + /* flags: */ 1
)

const (
	flagHi byte = 1 << iota
	flagBye
)

func (msg Message) MarshalBinary() ([]byte, error) {
	data := make([]byte, msgHeaderSize+len(msg.Body))
	data[0] = msg.version
	data[1] = msg.flags
	copy(data[2:], msg.Body)
	return data, nil
}

var ErrMessageCorrupt = errors.New("message corrupt")

func (msg *Message) UnmarshalBinary(data []byte) error {
	if len(data) < msgHeaderSize {
		return ErrMessageCorrupt
	}

	msg.version = data[0]
	msg.flags = data[1]
	msg.Body = make([]byte, len(data[2:]))
	copy(msg.Body, data[2:])
	return nil
}
