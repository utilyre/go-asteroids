package state

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"
)

var ErrShortData = errors.New("short data")

const (
	ScreenWidth  = 1920
	ScreenHeight = 1080
	PlayerSize   = 80
)

const InputSize = 1

// A zero valued input does not manipulate the state.
type Input struct {
	Left, Down, Up, Right bool
	Space                 bool
}

func (s *State) Update(delta time.Duration, inputs []Input) {
	const (
		playerRotation = 0.3
		playerAccel    = 500
	)

	dt := delta.Seconds()

	var forward float64
	var rotation float64
	for _, input := range inputs {
		if input.Down {
			forward += 1
		}
		if input.Up {
			forward -= 1
		}
		if input.Left {
			rotation -= 1
		}
		if input.Right {
			rotation += 1
		}
	}
	forward = signum(forward)

	s.Player.Rotation += playerRotation * rotation
	s.Player.Accel = HeadVec2(0.5*math.Pi + s.Player.Rotation).Mul(playerAccel * forward)
	//                            π/2 - (-a) = π/2 + a
	s.Player.Trans = s.Player.Accel.Mul(0.5 * dt * dt).Add(s.Player.Vel.Mul(dt)).Add(s.Player.Trans)
	s.Player.Vel = s.Player.Accel.Mul(dt).Add(s.Player.Vel)
}

const StateSize = MovableSize

type State struct {
	Player Movable
}

func InitState() State {
	return State{
		Player: Movable{
			Trans:    Vec2{PlayerSize, PlayerSize},
			Vel:      Vec2{},
			Accel:    Vec2{},
			Rotation: 0,
		},
	}
}

func (s State) Lerp(other State, t float64) State {
	s.Player = s.Player.Lerp(other.Player, t)
	return s
}

const MovableSize = 3*Vec2Size + 16

type Movable struct {
	Trans    Vec2
	Vel      Vec2
	Accel    Vec2
	Rotation float64
}

func (m Movable) Lerp(other Movable, t float64) Movable {
	m.Trans = m.Trans.Lerp(other.Trans, t)
	m.Vel = m.Vel.Lerp(other.Vel, t)
	m.Accel = m.Accel.Lerp(other.Accel, t)
	m.Rotation = lerp(m.Rotation, other.Rotation, t)
	return m
}

const Vec2Size = 16

type Vec2 struct{ X, Y float64 }

func HeadVec2(angle float64) Vec2 {
	return Vec2{
		X: math.Cos(angle),
		Y: math.Sin(angle),
	}
}

func signum(x float64) float64 {
	if x > 0 {
		return 1
	}
	if x < 0 {
		return -1
	}
	return 0
}

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

func (v Vec2) Rotate(rotation float64) Vec2 {
	cos := math.Cos(rotation)
	sin := math.Sin(rotation)
	v.X = cos*v.X - sin*v.Y
	v.Y = sin*v.X + cos*v.Y
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
	if i.Space {
		b |= 1 << 4
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
	i.Space = b&(1<<4) != 0
	return nil
}

func (s State) MarshalBinary() ([]byte, error) {
	return s.Player.MarshalBinary()
}

func (s *State) UnmarshalBinary(data []byte) error {
	return s.Player.UnmarshalBinary(data)
}

func (h Movable) MarshalBinary() ([]byte, error) {
	data := make([]byte, MovableSize)

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

	_, err = binary.Encode(data[3*Vec2Size:], binary.BigEndian, h.Rotation)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (h *Movable) UnmarshalBinary(data []byte) error {
	if l := len(data); l < MovableSize {
		return fmt.Errorf("data length %d less than %d: %w", l, MovableSize, ErrShortData)
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

	_, err = binary.Decode(data[3*Vec2Size:], binary.BigEndian, &h.Rotation)
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
