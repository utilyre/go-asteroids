package state

import (
	"math"
	"time"
)

type Input struct {
	Left, Down, Up, Right bool
}

func (input Input) Manipulate(delta time.Duration, state State) State {
	const houseAccel = 80
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
	state.House.Accel = v.Normalize().Mul(houseAccel)
	state.House.Trans = state.House.Accel.Mul(0.5 * dt * dt).Add(state.House.Vel.Mul(dt)).Add(state.House.Trans)
	state.House.Vel = state.House.Accel.Mul(dt).Add(state.House.Vel)

	return state
}

type State struct {
	House House
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
