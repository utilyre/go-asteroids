package state

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"time"
)

var ErrShortData = errors.New("short data")

const (
	ScreenWidth  = 1920
	ScreenHeight = 1080

	PlayerWidth  = 80
	PlayerHeight = 80

	AsteroidWidth  = 60
	AsteroidHeight = 60
)

const InputSize = 1

// A zero valued input does not manipulate the state.
type Input struct {
	Left, Down, Up, Right bool
	Space                 bool
}

type State struct {
	nextPlayerID   uint16
	nextBulletID   uint32
	nextAsteroidID uint32

	idToAddr map[uint16]string

	TotalScore uint32
	Players    []Player
	Bullets    []Bullet
	Asteroids  []Asteroid

	lastAsteroid time.Time
}

func (s *State) AddPlayer(addr string) {
	s.idToAddr[s.nextPlayerID] = addr
	s.Players = append(s.Players, Player{
		ID: s.nextPlayerID,
		Trans: Vec2{
			ScreenWidth * rand.Float64(),
			ScreenHeight * rand.Float64(),
		},
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
		playerAngVel    = 2.5
		playerAccel     = 500
		playerMaxSpeed  = 400
		playerScoreLoss = 10

		bulletSpeed    = 1200
		bulletCooldown = 200 * time.Millisecond

		asteroidTimeout  = 2 * time.Second
		asteroidDirRange = 0.75 * math.Pi
		asteroidScore    = 1
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

		player.Rotation = wrapAngle(playerAngVel*rotation*dt + player.Rotation)
		player.Accel = HeadVec2(0.5*math.Pi + player.Rotation).Mul(playerAccel * forward)
		//                            π/2 - (-a) = π/2 + a
		player.Trans = player.Accel.Mul(0.5 * dt * dt).Add(player.Vel.Mul(dt)).Add(player.Trans)
		if player.Trans.X < 0 {
			player.Trans.X = 0
			player.Vel.X = 0
		} else if player.Trans.X > ScreenWidth {
			player.Trans.X = ScreenWidth
			player.Vel.X = 0
		}
		if player.Trans.Y < 0 {
			player.Trans.Y = 0
			player.Vel.Y = 0
		} else if player.Trans.Y > ScreenHeight {
			player.Trans.Y = ScreenHeight
			player.Vel.Y = 0
		}
		player.Vel = player.Accel.Mul(dt).Add(player.Vel)
		if player.Vel.Magnitude() > playerMaxSpeed {
			player.Vel = player.Vel.Normalize().Mul(playerMaxSpeed)
		}

		if input.Space && time.Since(player.lastBullet) > bulletCooldown {
			s.Bullets = append(s.Bullets, Bullet{
				ID:       s.nextBulletID,
				Trans:    player.Trans,
				Rotation: player.Rotation,
			})
			s.nextBulletID++
			player.lastBullet = time.Now()
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

	if time.Since(s.lastAsteroid) > asteroidTimeout {
		const (
			top    = iota // up = -π/2
			bottom        // down = π/2
			left          // left = -π
			right         // right = 0
		)
		var trans Vec2
		var dir float64
		switch rand.N(4) {
		case top:
			trans.X = ScreenWidth * rand.Float64()
			trans.Y = 10
			dir = asteroidDirRange*(rand.Float64()-0.5) + 0.5*math.Pi
		case bottom:
			trans.X = ScreenWidth * rand.Float64()
			trans.Y = ScreenHeight - 10
			dir = asteroidDirRange*(rand.Float64()-0.5) - 0.5*math.Pi
		case left:
			trans.X = 10
			trans.Y = ScreenHeight * rand.Float64()
			dir = asteroidDirRange * (rand.Float64() - 0.5)
		case right:
			trans.X = ScreenWidth - 10
			trans.Y = ScreenHeight * rand.Float64()
			dir = asteroidDirRange*(rand.Float64()-0.5) - math.Pi
		}

		s.Asteroids = append(s.Asteroids, Asteroid{
			ID:       s.nextAsteroidID,
			Trans:    trans,
			Vel:      HeadVec2(dir).Mul(100),
			AngVel:   math.Pi * (rand.Float64() - 0.5),
			Rotation: 2 * math.Pi * (rand.Float64() - 0.5),
		})
		s.nextAsteroidID++
		s.lastAsteroid = time.Now()
	}

	var asteroidIndicesToRemove []int
	for i := range len(s.Asteroids) {
		asteroid := &s.Asteroids[i]
		asteroid.Trans = asteroid.Vel.Mul(dt).Add(asteroid.Trans)
		asteroid.Rotation = wrapAngle(asteroid.AngVel*dt + asteroid.Rotation)
		if asteroid.Trans.X < 0 || asteroid.Trans.X > ScreenWidth || asteroid.Trans.Y < 0 || asteroid.Trans.Y > ScreenHeight {
			asteroidIndicesToRemove = append(asteroidIndicesToRemove, i)
		}
	}
	for _, index := range slices.Backward(asteroidIndicesToRemove) {
		s.Asteroids = append(s.Asteroids[:index], s.Asteroids[index+1:]...)
	}

	// bullet-asteroid collision check
	bulletIndicesToRemove = nil
	asteroidIndicesToRemove = nil
	for ibullet, bullet := range s.Bullets {
		for iasteroid, asteroid := range s.Asteroids {
			if bullet.Trans.Sub(asteroid.Trans).Magnitude() <= AsteroidWidth {
				bulletIndicesToRemove = append(bulletIndicesToRemove, ibullet)
				asteroidIndicesToRemove = append(asteroidIndicesToRemove, iasteroid)
				s.TotalScore += asteroidScore
			}
		}
	}
	slices.Sort(bulletIndicesToRemove)
	for _, index := range slices.Backward(bulletIndicesToRemove) {
		s.Bullets = append(s.Bullets[:index], s.Bullets[index+1:]...)
	}
	slices.Sort(asteroidIndicesToRemove)
	for _, index := range slices.Backward(asteroidIndicesToRemove) {
		s.Asteroids = append(s.Asteroids[:index], s.Asteroids[index+1:]...)
	}

	// player-asteroid collision check
	var playerIndicesToRemove []int
	asteroidIndicesToRemove = nil
	for iplayer, player := range s.Players {
		for iasteroid, asteroid := range s.Asteroids {
			if player.Trans.Sub(asteroid.Trans).Magnitude() <= AsteroidWidth+PlayerWidth {
				playerIndicesToRemove = append(playerIndicesToRemove, iplayer)
				asteroidIndicesToRemove = append(asteroidIndicesToRemove, iasteroid)
				if s.TotalScore < playerScoreLoss {
					s.TotalScore = 0
				} else {
					s.TotalScore -= playerScoreLoss
				}
			}
		}
	}
	slices.Sort(playerIndicesToRemove)
	for _, index := range slices.Backward(playerIndicesToRemove) {
		s.Players = append(s.Players[:index], s.Players[index+1:]...)
	}
	slices.Sort(asteroidIndicesToRemove)
	for _, index := range slices.Backward(asteroidIndicesToRemove) {
		s.Asteroids = append(s.Asteroids[:index], s.Asteroids[index+1:]...)
	}
}

const BulletSize = 4 + 2*Vec2Size

type Bullet struct {
	ID       uint32
	Trans    Vec2
	Rotation float64
}

func (b Bullet) Lerp(other Bullet, t float64) Bullet {
	b.Trans = b.Trans.Lerp(other.Trans, t)
	return b
}

const AsteroidSize = 4 + 2*Vec2Size

type Asteroid struct {
	ID       uint32
	Trans    Vec2
	Vel      Vec2
	AngVel   float64
	Rotation float64
}

func (a Asteroid) Lerp(other Asteroid, t float64) Asteroid {
	a.Trans = a.Trans.Lerp(other.Trans, t)
	a.Rotation = rlerp(a.Rotation, other.Rotation, t)
	return a
}

const PlayerSize = 4 + 3*Vec2Size + 8

type Player struct {
	ID       uint16
	Trans    Vec2
	Vel      Vec2
	Accel    Vec2
	Rotation float64

	lastBullet time.Time
}

func (p Player) Lerp(other Player, t float64) Player {
	p.Trans = p.Trans.Lerp(other.Trans, t)
	p.Rotation = rlerp(p.Rotation, other.Rotation, t)
	return p
}

func InitState() State {
	return State{
		nextPlayerID:   1,
		nextBulletID:   1,
		nextAsteroidID: 1,
		idToAddr:       map[uint16]string{},
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

	{
		i, j := 0, 0
		for i < len(s.Asteroids) && j < len(other.Asteroids) {
			l := &s.Asteroids[i]
			r := &other.Asteroids[j]
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

func (v Vec2) Sub(other Vec2) Vec2 {
	v.X -= other.X
	v.Y -= other.Y
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

	err := binary.Write(&buf, binary.BigEndian, s.TotalScore)
	if err != nil {
		return nil, err
	}

	err = binary.Write(&buf, binary.BigEndian, uint16(len(s.Players)))
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

	err = binary.Write(&buf, binary.BigEndian, uint16(len(s.Asteroids)))
	if err != nil {
		return nil, err
	}
	for _, asteroid := range s.Asteroids {
		err = binary.Write(&buf, binary.BigEndian, asteroid.ID)
		if err != nil {
			return nil, err
		}
		err = binary.Write(&buf, binary.BigEndian, uint16(asteroid.Trans.X))
		if err != nil {
			return nil, err
		}
		err = binary.Write(&buf, binary.BigEndian, uint16(asteroid.Trans.Y))
		if err != nil {
			return nil, err
		}
		err = binary.Write(&buf, binary.BigEndian, float32(asteroid.Rotation))
		if err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func (s *State) UnmarshalBinary(data []byte) error {
	r := bytes.NewReader(data)

	err := binary.Read(r, binary.BigEndian, &s.TotalScore)
	if err != nil {
		return err
	}

	var playersLen uint16
	err = binary.Read(r, binary.BigEndian, &playersLen)
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

	var asteroidsLen uint16
	err = binary.Read(r, binary.BigEndian, &asteroidsLen)
	if err != nil {
		return err
	}
	s.Asteroids = make([]Asteroid, asteroidsLen)
	for i := range asteroidsLen {
		err = binary.Read(r, binary.BigEndian, &s.Asteroids[i].ID)
		if err != nil {
			return err
		}
		var tx uint16
		err = binary.Read(r, binary.BigEndian, &tx)
		if err != nil {
			return err
		}
		s.Asteroids[i].Trans.X = float64(tx)
		var ty uint16
		err = binary.Read(r, binary.BigEndian, &ty)
		if err != nil {
			return err
		}
		s.Asteroids[i].Trans.Y = float64(ty)
		var rotation float32
		err = binary.Read(r, binary.BigEndian, &rotation)
		if err != nil {
			return err
		}
		s.Asteroids[i].Rotation = float64(rotation)
	}

	return nil
}
