package jitter

import (
	"bytes"
	"encoding/binary"
	"multiplayer/internal/state"
)

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

func (buf Buffer) Encode(b *bytes.Buffer) {
	_ = binary.Write(b, binary.BigEndian, uint32(len(buf.inputs)))
	for _, input := range buf.inputs {
		_ = binary.Write(b, binary.BigEndian, input.index)
		input.Encode(b)
	}
}

func (buf *Buffer) Decode(r *bytes.Reader) error {
	var numInputs uint32
	err := binary.Read(r, binary.BigEndian, &numInputs)
	if err != nil {
		return err
	}

	buf.inputs = make([]indexedInput, numInputs)
	for i := range numInputs {
		err = binary.Read(r, binary.BigEndian, &buf.inputs[i].index)
		if err != nil {
			return err
		}
		err = buf.inputs[i].Decode(r)
		if err != nil {
			return err
		}
	}

	if numInputs > 0 {
		buf.next = buf.inputs[numInputs-1].index + 1
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
