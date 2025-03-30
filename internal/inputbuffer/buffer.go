package inputbuffer

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

	_, err := binary.Encode(data, binary.BigEndian, uint16(len(buf.inputs)))
	if err != nil {
		panic("buffer is large enough")
	}

	for _, input := range buf.inputs {
		inputData, err := input.MarshalBinary()
		if err != nil {
			return nil, err
		}
		data = append(data, inputData...)
	}

	if len(data) != finalSize {
		panic(fmt.Sprintf("expected size %d; actual size %d", finalSize, len(data))) // HIT
	}

	return data, nil
}
