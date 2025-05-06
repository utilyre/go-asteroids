package state

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
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
	nextPlayerID uint16
	nextBulletID uint32
	idToAddr     map[uint16]string

	Players []Player
	Bullets []Bullet

	lastBullet time.Time
}

func (s *State) AddPlayer(addr string) {
	s.idToAddr[s.nextPlayerID] = addr
	s.Players = append(s.Players, Player{
		ID:       s.nextPlayerID,
		Trans:    Vec2{PlayerWidth, PlayerWidth},
		Vel:      Vec2{},
		Accel:    Vec2{},
		Rotation: 0,
	})
	s.nextPlayerID++
}

func (s *State) RemovePlayer(addr string) {
	var id uint16
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
		bulletSpeed    = 600
		bulletCooldown = time.Second / 4
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

		player.Rotation = wrapAngle(playerRotation*rotation + player.Rotation)
		player.Accel = HeadVec2(0.5*math.Pi + player.Rotation).Mul(playerAccel * forward)
		//                            π/2 - (-a) = π/2 + a
		player.Trans = player.Accel.Mul(0.5 * dt * dt).Add(player.Vel.Mul(dt)).Add(player.Trans)
		player.Trans.X = max(0, min(ScreenWidth, player.Trans.X))
		player.Trans.Y = max(0, min(ScreenHeight, player.Trans.Y))
		player.Vel = player.Accel.Mul(dt).Add(player.Vel)
		if player.Vel.Magnitude() > playerMaxSpeed {
			player.Vel = player.Vel.Normalize().Mul(playerMaxSpeed)
		}

		if input.Space && time.Since(s.lastBullet) > bulletCooldown {
			s.Bullets = append(s.Bullets, Bullet{
				ID:       s.nextBulletID,
				Trans:    player.Trans,
				Rotation: player.Rotation,
			})
			s.nextBulletID++
			s.lastBullet = time.Now()
		}
	}

	var bulletIndicesToRemove []int
	for i := range len(s.Bullets) {
		bullet := &s.Bullets[i]
		bullet.Trans = HeadVec2(1.5*math.Pi + bullet.Rotation).Mul(bulletSpeed * dt).Add(bullet.Trans)
		if bullet.Trans.X < 0 || bullet.Trans.X > ScreenWidth || bullet.Trans.Y < 0 || bullet.Trans.Y > ScreenHeight {
			bulletIndicesToRemove = append(bulletIndicesToRemove, i)
		}
	}
	for _, index := range slices.Backward(bulletIndicesToRemove) {
		s.Bullets = append(s.Bullets[:index], s.Bullets[index+1:]...)
	}
}

const BulletSize = 2 * Vec2Size

type Bullet struct {
	ID       uint32
	Trans    Vec2
	Rotation float64
}

func (b Bullet) Lerp(other Bullet, t float64) Bullet {
	b.Trans = b.Trans.Lerp(other.Trans, t)
	return b
}

const PlayerSize = 4 + 3*Vec2Size + 8

type Player struct {
	ID       uint16
	Trans    Vec2
	Vel      Vec2
	Accel    Vec2
	Rotation float64
}

func (p Player) Lerp(other Player, t float64) Player {
	p.Trans = p.Trans.Lerp(other.Trans, t)
	p.Rotation = rlerp(p.Rotation, other.Rotation, t)
	return p
}

func InitState() State {
	return State{
		nextPlayerID: 1,
		nextBulletID: 1,
		idToAddr:     map[uint16]string{},
	}
}

func (s State) Lerp(other State, t float64) State {
	{
		// Double pointer problem
		//
		// Example:
		// 1, 2, 5, 6, 7, 10
		// 2, 4, 5, 6, 8, 9

		i, j := 0, 0
		for i < len(s.Players) && j < len(other.Players) {
			l := &s.Players[i]
			r := &other.Players[j]
			if l.ID < r.ID {
				i++
			} else if l.ID > r.ID {
				j++
			} else {
				*l = l.Lerp(*r, t)
				i++
				j++
			}
		}
	}

	{
		i, j := 0, 0
		for i < len(s.Bullets) && j < len(other.Bullets) {
			l := &s.Bullets[i]
			r := &other.Bullets[j]
			if l.ID < r.ID {
				i++
			} else if l.ID > r.ID {
				j++
			} else {
				*l = l.Lerp(*r, t)
				i++
				j++
			}
		}
	}

	return s
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

func wrapAngle(angle float64) float64 {
	// Wrap angle to [-π, π]
	wrapped := math.Mod(angle+math.Pi, 2*math.Pi)
	if wrapped < 0 {
		wrapped += 2 * math.Pi
	}
	return wrapped - math.Pi
}

func rlerp(a, b, t float64) float64 {
	x := lerp(math.Cos(a), math.Cos(b), t)
	y := lerp(math.Sin(a), math.Sin(b), t)
	return math.Atan2(y, x)
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
	var buf bytes.Buffer

	err := binary.Write(&buf, binary.BigEndian, uint16(len(s.Players)))
	if err != nil {
		return nil, err
	}

	for _, player := range s.Players {
		err = binary.Write(&buf, binary.BigEndian, player.ID)
		if err != nil {
			return nil, err
		}
		err = binary.Write(&buf, binary.BigEndian, uint16(player.Trans.X))
		if err != nil {
			return nil, err
		}
		err = binary.Write(&buf, binary.BigEndian, uint16(player.Trans.Y))
		if err != nil {
			return nil, err
		}
		err = binary.Write(&buf, binary.BigEndian, float32(player.Rotation))
		if err != nil {
			return nil, err
		}
	}

	err = binary.Write(&buf, binary.BigEndian, uint16(len(s.Bullets)))
	if err != nil {
		return nil, err
	}
	for _, bullet := range s.Bullets {
		err = binary.Write(&buf, binary.BigEndian, bullet.ID)
		if err != nil {
			return nil, err
		}
		err = binary.Write(&buf, binary.BigEndian, uint16(bullet.Trans.X))
		if err != nil {
			return nil, err
		}
		err = binary.Write(&buf, binary.BigEndian, uint16(bullet.Trans.Y))
		if err != nil {
			return nil, err
		}
		err = binary.Write(&buf, binary.BigEndian, float32(bullet.Rotation))
		if err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func (s *State) UnmarshalBinary(data []byte) error {
	r := bytes.NewReader(data)

	var playersLen uint16
	err := binary.Read(r, binary.BigEndian, &playersLen)
	if err != nil {
		return err
	}
	s.Players = make([]Player, playersLen)
	for i := range playersLen {
		err = binary.Read(r, binary.BigEndian, &s.Players[i].ID)
		if err != nil {
			return err
		}
		var tx uint16
		err = binary.Read(r, binary.BigEndian, &tx)
		if err != nil {
			return err
		}
		s.Players[i].Trans.X = float64(tx)
		var ty uint16
		err = binary.Read(r, binary.BigEndian, &ty)
		if err != nil {
			return err
		}
		s.Players[i].Trans.Y = float64(ty)
		var rotation float32
		err = binary.Read(r, binary.BigEndian, &rotation)
		if err != nil {
			return err
		}
		s.Players[i].Rotation = float64(rotation)
	}

	var bulletsLen uint16
	err = binary.Read(r, binary.BigEndian, &bulletsLen)
	if err != nil {
		return err
	}
	s.Bullets = make([]Bullet, bulletsLen)
	for i := range bulletsLen {
		err = binary.Read(r, binary.BigEndian, &s.Bullets[i].ID)
		if err != nil {
			return err
		}
		var tx uint16
		err = binary.Read(r, binary.BigEndian, &tx)
		if err != nil {
			return err
		}
		s.Bullets[i].Trans.X = float64(tx)
		var ty uint16
		err = binary.Read(r, binary.BigEndian, &ty)
		if err != nil {
			return err
		}
		s.Bullets[i].Trans.Y = float64(ty)
		var rotation float32
		err = binary.Read(r, binary.BigEndian, &rotation)
		if err != nil {
			return err
		}
		s.Bullets[i].Rotation = float64(rotation)
	}

	return nil
}
