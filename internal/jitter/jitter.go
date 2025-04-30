package jitter

import (
	"encoding/binary"
	"fmt"
	"multiplayer/internal/state"
)

const indexedInputSize = state.InputSize + 4

type indexedInput struct {
	state.Input
	index uint32
}

type Buffer struct {
	inputs []indexedInput
	next   uint32
}

func NewBufferFrom(inputs []state.Input) Buffer {
	indexedInputs := make([]indexedInput, len(inputs))
	var next uint32 = 0
	for ; int(next) < len(inputs); next++ {
		indexedInputs[next] = indexedInput{
			Input: inputs[next],
			index: uint32(next),
		}
	}
	return Buffer{
		inputs: indexedInputs,
		next:   next,
	}
}

func (buf Buffer) Size() int { return len(buf.inputs) }

func (buf Buffer) Indices() []uint32 {
	indices := make([]uint32, len(buf.inputs))
	for i, input := range buf.inputs {
		indices[i] = input.index
	}
	return indices
}

func (buf Buffer) Inputs() []state.Input {
	inputs := make([]state.Input, len(buf.inputs))
	for i, input := range buf.inputs {
		inputs[i] = input.Input
	}
	return inputs
}

func (buf Buffer) MarshalBinary() ([]byte, error) {
	data := make([]byte, 4+indexedInputSize*len(buf.inputs))

	binary.BigEndian.PutUint32(data, uint32(len(buf.inputs)))

	for i, input := range buf.inputs {
		binary.BigEndian.PutUint32(data[4+indexedInputSize*i:], input.index)
		inputData, err := input.MarshalBinary()
		if err != nil {
			return nil, err
		}
		copy(data[4+indexedInputSize*i+4:], inputData)
	}

	return data, nil
}

func (buf *Buffer) UnmarshalBinary(data []byte) error {
	if l, expected := len(data), 4; l < expected {
		return fmt.Errorf("data length %d less than %d: %w", l, expected, state.ErrShortData)
	}
	numInputs := binary.BigEndian.Uint32(data)
	if l, expected := len(data), int(4+indexedInputSize*numInputs); l < expected {
		return fmt.Errorf("data length %d less than %d: %w", l, expected, state.ErrShortData)
	}

	inputs := make([]indexedInput, numInputs)
	for i := range numInputs {
		index := binary.BigEndian.Uint32(data[4+indexedInputSize*i:])
		var input state.Input
		err := input.UnmarshalBinary(data[4+indexedInputSize*i+4:])
		if err != nil {
			return err
		}
		inputs[i] = indexedInput{
			Input: input,
			index: index,
		}
	}

	buf.inputs = inputs
	if numInputs > 0 {
		buf.next = inputs[numInputs-1].index + 1
	}
	return nil
}

func (buf *Buffer) DiscardUntil(index uint32) {
	idx := 0
	for idx < len(buf.inputs) && buf.inputs[idx].index <= index {
		idx++
	}
	copy(buf.inputs, buf.inputs[idx:])
	buf.inputs = buf.inputs[:len(buf.inputs)-idx]
}

func (buf *Buffer) Append(input state.Input) {
	buf.inputs = append(buf.inputs, indexedInput{
		Input: input,
		index: buf.next,
	})
	buf.next++
}
