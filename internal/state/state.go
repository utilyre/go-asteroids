package state

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"time"
)

var ErrShortData = errors.New("short data")

const (
	ScreenWidth  = 1920
	ScreenHeight = 1080
	PlayerWidth  = 80
	PlayerHeight = 80
)

const InputSize = 1

// A zero valued input does not manipulate the state.
type Input struct {
	Left, Down, Up, Right bool
	Space                 bool
}

type State struct {
	nextID   uint32
	idToAddr map[uint32]string
	Players  []Player
	Bullets  []Bullet
}

func (s *State) AddPlayer(addr string) {
	s.idToAddr[s.nextID] = addr
	s.Players = append(s.Players, Player{
		ID: s.nextID,
		Movable: Movable{
			Trans:    Vec2{PlayerWidth, PlayerWidth},
			Vel:      Vec2{},
			Accel:    Vec2{},
			Rotation: 0,
		},
	})
	s.nextID++
}

func (s *State) RemovePlayer(addr string) {
	var id uint32
	for i := range len(s.Players) {
		if currentID := s.Players[i].ID; s.idToAddr[currentID] == addr {
			s.Players = append(s.Players[:i], s.Players[i+1:]...)
			id = currentID
			break
		}
	}
	if id == 0 {
		return
	}

	delete(s.idToAddr, id)
}

func (s *State) Update(delta time.Duration, inputs map[string]Input) {
	const (
		playerRotation = 0.3
		playerAccel    = 500
		playerMaxSpeed = 400
	)

	dt := delta.Seconds()

	for i := range len(s.Players) {
		player := &s.Players[i]
		input := inputs[s.idToAddr[player.ID]]

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
		if player.Vel.Magnitude() > playerMaxSpeed {
			player.Vel = player.Vel.Normalize().Mul(playerMaxSpeed)
		}

		if input.Space {
			s.Bullets = append(s.Bullets, Bullet{
				Movable: Movable{
					Trans:    player.Trans,
					Vel:      HeadVec2(0.5*math.Pi + player.Rotation).Mul(-600),
					Rotation: rotation,
				},
			})
		}
	}

	var bulletIndicesToRemove []int
	for i := range len(s.Bullets) {
		bullet := &s.Bullets[i]
		bullet.Trans = bullet.Vel.Mul(dt).Add(bullet.Trans)
		if bullet.Trans.X < 0 || bullet.Trans.X > ScreenWidth || bullet.Trans.Y < 0 || bullet.Trans.Y > ScreenHeight {
			bulletIndicesToRemove = append(bulletIndicesToRemove, i)
		}
	}
	for _, index := range slices.Backward(bulletIndicesToRemove) {
		s.Bullets = append(s.Bullets[:index], s.Bullets[index+1:]...)
	}
}

const BulletSize = MovableSize

type Bullet struct {
	Movable
}

func (b Bullet) Lerp(other Bullet, t float64) Bullet {
	b.Movable = b.Movable.Lerp(other.Movable, t)
	return b
}

const PlayerSize = 4 + MovableSize

type Player struct {
	ID uint32
	Movable
}

func (p Player) Lerp(other Player, t float64) Player {
	p.Movable = p.Movable.Lerp(other.Movable, t)
	return p
}

func InitState() State {
	return State{
		nextID:   1,
		idToAddr: map[uint32]string{},
	}
}

func (s State) Lerp(other State, t float64) State {
	if len(s.Players) != len(other.Players) || len(s.Bullets) != len(other.Bullets) {
		// TODO: fault tolerate this
		// panic("current and other state do not have the same number of players")
		return s
	}

	for i, rplayer := range other.Players {
		player := &s.Players[i]
		*player = player.Lerp(rplayer, t)
	}
	for i, rbullet := range other.Bullets {
		bullet := &s.Bullets[i]
		*bullet = bullet.Lerp(rbullet, t)
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

func (v Vec2) Magnitude() float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y)
}

func (v Vec2) Normalize() Vec2 {
	if v.X == 0 && v.Y == 0 {
		return v
	}
	l := v.Magnitude()
	v.X /= l
	v.Y /= l
	return v
}

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
	data := make([]byte, 0, 8+PlayerSize*len(s.Players)+BulletSize*len(s.Bullets))
	buf := bytes.NewBuffer(data)

	err := binary.Write(buf, binary.BigEndian, uint64(len(s.Players)))
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, s.Players)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, uint64(len(s.Bullets)))
	if err != nil {
		return nil, err
	}
	err = binary.Write(buf, binary.BigEndian, s.Bullets)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *State) UnmarshalBinary(data []byte) error {
	slog.Debug("length of data passed to unmarshal binary", "len", len(data))
	r := bytes.NewReader(data)

	var playersLen uint64
	err := binary.Read(r, binary.BigEndian, &playersLen)
	if err != nil {
		return err
	}
	s.Players = make([]Player, playersLen)
	err = binary.Read(r, binary.BigEndian, s.Players)
	if err != nil {
		return err
	}

	var bulletsLen uint64
	err = binary.Read(r, binary.BigEndian, &bulletsLen)
	if err != nil {
		return err
	}
	s.Bullets = make([]Bullet, bulletsLen)
	err = binary.Read(r, binary.BigEndian, s.Bullets)
	if err != nil {
		return err
	}

	return nil
}
