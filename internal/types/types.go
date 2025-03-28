package types

import (
	"encoding/binary"
	"errors"
)

const InputSize int = 5

type Input struct { // ~5B
	Index                 uint32 // 4B
	Up, Left, Down, Right bool   // 0.5B
}

func (i Input) MarshalBinary() ([]byte, error) {
	data := make([]byte, InputSize)
	_, err := binary.Encode(data, binary.BigEndian, i.Index)
	if err != nil {
		panic("data is large enough")
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

func (i *Input) UnmarshalBinary(data []byte) error {
	if len(data) < InputSize {
		return errors.New("data too small")
	}

	_, err := binary.Decode(data, binary.BigEndian, &i.Index)
	if err != nil {
		panic("data is large enough")
	}

	i.Up = data[4]&(1<<0) != 0
	i.Left = data[4]&(1<<1) != 0
	i.Down = data[4]&(1<<2) != 0
	i.Right = data[4]&(1<<3) != 0

	return nil
}
