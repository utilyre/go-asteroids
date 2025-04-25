package state

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"
)

var ErrShortData = errors.New("short data")

const InputSize = 1

type Input struct {
	Left, Down, Up, Right bool
}

// A zero valued input does not manipulate the state.
func (s *State) Update(delta time.Duration, input Input) {
	const houseAccel = 300
	dt := delta.Seconds()

	var v Vec2
	if input.Left {
		v.X -= 1
	}
	if input.Down {
		v.Y += 1
	}
	if input.Up {
		v.Y -= 1
	}
	if input.Right {
		v.X += 1
	}

	s.House.Accel = v.Normalize().Mul(houseAccel)
	s.House.Trans = s.House.Accel.Mul(0.5 * dt * dt).Add(s.House.Vel.Mul(dt)).Add(s.House.Trans)
	s.House.Vel = s.House.Accel.Mul(dt).Add(s.House.Vel)
}

const StateSize = HouseSize

type State struct {
	House House
}

func (s State) Lerp(other State, t float64) State {
	s.House.Trans = s.House.Trans.Lerp(other.House.Trans, t)
	s.House.Vel = s.House.Vel.Lerp(other.House.Vel, t)
	s.House.Accel = s.House.Accel.Lerp(other.House.Accel, t)
	return s
}

const HouseSize = 3 * Vec2Size

type House struct {
	Trans Vec2
	Vel   Vec2
	Accel Vec2
}

const Vec2Size = 16

type Vec2 struct{ X, Y float64 }

func lerp(a, b, t float64) float64 {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}

	return a + (b-a)*t
}

func (v Vec2) Lerp(other Vec2, t float64) Vec2 {
	v.X = lerp(v.X, other.X, t)
	v.Y = lerp(v.Y, other.Y, t)
	return v
}

func (v Vec2) Add(other Vec2) Vec2 {
	v.X += other.X
	v.Y += other.Y
	return v
}

func (v Vec2) Mul(other float64) Vec2 {
	v.X *= other
	v.Y *= other
	return v
}

func (v Vec2) Normalize() Vec2 {
	if v.X == 0 && v.Y == 0 {
		return v
	}
	l := math.Sqrt(v.X*v.X + v.Y*v.Y)
	v.X /= l
	v.Y /= l
	return v
}

// MarshalBinary encodes Input into a single byte where each bit represents a boolean field
func (i Input) MarshalBinary() ([]byte, error) {
	var b byte
	if i.Left {
		b |= 1 << 0
	}
	if i.Down {
		b |= 1 << 1
	}
	if i.Up {
		b |= 1 << 2
	}
	if i.Right {
		b |= 1 << 3
	}
	return []byte{b}, nil
}

// UnmarshalBinary decodes a byte into the Input struct
func (i *Input) UnmarshalBinary(data []byte) error {
	if l := len(data); l < InputSize {
		return fmt.Errorf("data length %d less than %d: %w", l, InputSize, ErrShortData)
	}
	b := data[0]
	i.Left = b&(1<<0) != 0
	i.Down = b&(1<<1) != 0
	i.Up = b&(1<<2) != 0
	i.Right = b&(1<<3) != 0
	return nil
}

func (s State) MarshalBinary() ([]byte, error) {
	return s.House.MarshalBinary()
}

func (s *State) UnmarshalBinary(data []byte) error {
	return s.House.UnmarshalBinary(data)
}

func (h House) MarshalBinary() ([]byte, error) {
	data := make([]byte, HouseSize)

	b, err := h.Trans.MarshalBinary()
	if err != nil {
		return nil, err
	}
	copy(data, b)

	b, err = h.Vel.MarshalBinary()
	if err != nil {
		return nil, err
	}
	copy(data[Vec2Size:], b)

	b, err = h.Accel.MarshalBinary()
	if err != nil {
		return nil, err
	}
	copy(data[2*Vec2Size:], b)

	return data, nil
}

func (h *House) UnmarshalBinary(data []byte) error {
	if l := len(data); l < HouseSize {
		return fmt.Errorf("data length %d less than %d: %w", l, HouseSize, ErrShortData)
	}

	err := h.Trans.UnmarshalBinary(data)
	if err != nil {
		return err
	}
	err = h.Vel.UnmarshalBinary(data[Vec2Size:])
	if err != nil {
		return err
	}
	err = h.Accel.UnmarshalBinary(data[2*Vec2Size:])
	if err != nil {
		return err
	}

	return nil
}

// MarshalBinary encodes Vec2 as two float64s in network order
func (v Vec2) MarshalBinary() ([]byte, error) {
	data := make([]byte, Vec2Size)
	_, err := binary.Encode(data, binary.BigEndian, v.X)
	if err != nil {
		return nil, err
	}
	_, err = binary.Encode(data[8:], binary.BigEndian, v.Y)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// UnmarshalBinary decodes two float64s from binary data
func (v *Vec2) UnmarshalBinary(data []byte) error {
	if l := len(data); l < Vec2Size {
		return fmt.Errorf("data length %d less than %d: %w", l, Vec2Size, ErrShortData)
	}
	_, err := binary.Decode(data, binary.BigEndian, &v.X)
	if err != nil {
		return err
	}
	_, err = binary.Decode(data[8:], binary.BigEndian, &v.Y)
	if err != nil {
		return err
	}
	return nil
}
