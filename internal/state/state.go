package state

import (
	"math"
	"time"
)

type Input struct {
	Left, Down, Up, Right bool
}

// zero value input does not manipulate the state.
func (input Input) Manipulate(who int, state State, delta time.Duration) State {
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

	switch who {
	case 1:
		state.House1.Accel = v.Normalize().Mul(houseAccel)
		state.House1.Trans = state.House1.Accel.Mul(0.5 * dt * dt).Add(state.House1.Vel.Mul(dt)).Add(state.House1.Trans)
		state.House1.Vel = state.House1.Accel.Mul(dt).Add(state.House1.Vel)
	case 2:
		state.House2.Accel = v.Normalize().Mul(houseAccel)
		state.House2.Trans = state.House2.Accel.Mul(0.5 * dt * dt).Add(state.House2.Vel.Mul(dt)).Add(state.House2.Trans)
		state.House2.Vel = state.House2.Accel.Mul(dt).Add(state.House2.Vel)
	}

	return state
}

type State struct {
	House1 House
	House2 House
}

type House struct {
	Trans Vec2
	Vel   Vec2
	Accel Vec2
}

type Vec2 struct{ X, Y float64 }

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
