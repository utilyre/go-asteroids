package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"multiplayer/internal/types"
)

type InputBuffer struct {
	next   uint32
	inputs []types.Input
}

func (buf *InputBuffer) Add(input types.Input) {
	input.Index = buf.next
	buf.inputs = append(buf.inputs, input)
	buf.next++
}

func (buf *InputBuffer) FlushUntil(index uint32) error {
	idx := -1
	for i, input := range buf.inputs {
		if input.Index == index {
			idx = i
			break
		}
	}
	if idx == -1 {
		return errors.New("index not found")
	}

	buf.inputs = append([]types.Input(nil), buf.inputs[idx:]...)
	return nil
}

func (buf InputBuffer) MarshalBinary() ([]byte, error) {
	finalSize := 2 + types.InputSize*len(buf.inputs)
	data := make([]byte, 2, finalSize)

	must(binary.Encode(data, binary.BigEndian, uint16(len(buf.inputs))))

	for _, input := range buf.inputs {
		inputData, err := input.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("marshaling input: %w", err)
		}
		data = append(data, inputData...)
	}

	if l := len(data); l != finalSize {
		panic(fmt.Sprintf("ended up with data of size %d instead of %d",
			l, finalSize)) // HIT
	}

	return data, nil
}
