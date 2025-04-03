package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"multiplayer/internal/gameconn"
	"multiplayer/internal/types"
	"net"
)

var errCorruptedMessage = errors.New("message was corrupted")

func readInputsPacket(body []byte) ([]types.Input, error) {
	if len(body) < 2 {
		return nil, errCorruptedMessage
	}

	var size uint16
	_, err := binary.Decode(body, binary.BigEndian, &size)
	if err != nil {
		panic("message should have been large enough")
	}
	if size == 0 {
		return []types.Input{}, nil
	}
	if 2+int(size)*types.InputSize > len(body) {
		return nil, fmt.Errorf("reading from udp %w", errCorruptedMessage)
	}

	inputs := make([]types.Input, size)
	for i := range len(inputs) {
		err = inputs[i].UnmarshalBinary(body[2+i*types.InputSize : 2+(i+1)*types.InputSize])
		if err != nil {
			return nil, fmt.Errorf("unmarshaling input #%d: %w", i, err)
		}
	}

	return inputs, nil
}

func ackInput(conn *gameconn.Conn, addr net.Addr, input types.Input) error {
	lastIndexData := make([]byte, 4)
	_, err := binary.Encode(lastIndexData, binary.BigEndian, input.Index)
	if err != nil {
		panic("data should have been large enough")
	}

	err = conn.Send(addr, &gameconn.Message{
		Scope: types.ScopeInputAck,
		Body:  lastIndexData,
	})
	if err != nil {
		return fmt.Errorf("writing to udp: %w", err)
	}

	return nil
}
