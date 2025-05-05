package state

import (
	"encoding/binary"
	"errors"
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

type State struct {
	Players []Player
}

func (s *State) AddPlayer(addr string) {
	s.Players = append(s.Players, Player{
		Movable: Movable{
			Trans:    Vec2{PlayerSize, PlayerSize},
			Vel:      Vec2{},
			Accel:    Vec2{},
			Rotation: 0,
		},
		addr: addr,
	})
}

func (s *State) Update(delta time.Duration, inputs map[string]Input) {
	const (
		playerRotation = 0.3
		playerAccel    = 500
	)

	dt := delta.Seconds()

	for i := range len(s.Players) {
		player := &s.Players[i]
		input := inputs[player.addr]

		forward := 0.0
		rotation := 0.0
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

		player.Rotation += playerRotation * rotation
		player.Accel = HeadVec2(0.5*math.Pi + player.Rotation).Mul(playerAccel * forward)
		//                            π/2 - (-a) = π/2 + a
		player.Trans = player.Accel.Mul(0.5 * dt * dt).Add(player.Vel.Mul(dt)).Add(player.Trans)
		player.Vel = player.Accel.Mul(dt).Add(player.Vel)
	}
}

const StateSize = MovableSize

type Player struct {
	addr string
	Movable
}

func (p Player) Lerp(other Player, t float64) Player {
	p.Movable = p.Movable.Lerp(other.Movable, t)
	return p
}

func InitState() State {
	return State{}
}

func (s State) Lerp(other State, t float64) State {
	if len(s.Players) != len(other.Players) {
		panic("current and other state do not have the same number of players")
	}

	for i, rplayer := range other.Players {
		player := &s.Players[i]
		*player = player.Lerp(rplayer, t)
	}
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

func (i Input) MarshalBinary() ([]byte, error) {
	data := make([]byte, 1)
	_, err := binary.Encode(data, binary.BigEndian, i)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (i *Input) UnmarshalBinary(data []byte) error {
	_, err := binary.Decode(data, binary.BigEndian, i)
	return err
}

func (s State) MarshalBinary() ([]byte, error) {
	panic("TODO")
}

func (s *State) UnmarshalBinary(data []byte) error {
	panic("TODO")
}
