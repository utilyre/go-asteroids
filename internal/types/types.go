package types

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const StateSize = 128

type State struct {
	Position struct{ X, Y float64 }
}

func (s *State) MarshalBinary() ([]byte, error) {
	data := make([]byte, StateSize)
	_, err := binary.Encode(data, binary.BigEndian, s.Position.X)
	if err != nil {
		panic("data should have been large enough")
	}
	_, err = binary.Encode(data[64:], binary.BigEndian, s.Position.Y)
	if err != nil {
		panic("data should have been large enough")
	}
	return data, nil
}

func (s *State) UnmarshalBinary(data []byte) error {
	if l := len(data); l < InputSize {
		return fmt.Errorf("data with len %d: %w", l, ErrTooSmall)
	}

	_, err := binary.Decode(data, binary.BigEndian, &s.Position.X)
	if err != nil {
		panic("data should have been large enough")
	}
	_, err = binary.Decode(data, binary.BigEndian, &s.Position.Y)
	if err != nil {
		panic("data should have been large enough")
	}

	return nil
}

const InputSize int = 5

type Input struct { // ~5B
	Index                 uint32 // 4B
	Up, Left, Down, Right bool   // 0.5B
}

func (i Input) MarshalBinary() ([]byte, error) {
	data := make([]byte, InputSize)
	_, err := binary.Encode(data, binary.BigEndian, i.Index)
	if err != nil {
		panic("data should have been large enough")
	}

	if i.Up {
		data[4] |= 1 << 0
	}
	if i.Left {
		data[4] |= 1 << 1
	}
	if i.Down {
		data[4] |= 1 << 2
	}
	if i.Right {
		data[4] |= 1 << 3
	}

	return data, nil
}

var ErrTooSmall = errors.New("too small")

func (i *Input) UnmarshalBinary(data []byte) error {
	if l := len(data); l < InputSize {
		return fmt.Errorf("data with len %d: %w", l, ErrTooSmall)
	}

	_, err := binary.Decode(data, binary.BigEndian, &i.Index)
	if err != nil {
		panic("data should have been large enough")
	}

	i.Up = data[4]&(1<<0) != 0
	i.Left = data[4]&(1<<1) != 0
	i.Down = data[4]&(1<<2) != 0
	i.Right = data[4]&(1<<3) != 0

	return nil
}
