package state_test

import (
	"multiplayer/internal/state"
	"testing"
)

func FuzzInput(f *testing.F) {
	f.Add(false, false, false, false)
	f.Fuzz(func(t *testing.T, left, down, up, right bool) {
		expected := state.Input{
			Left:  left,
			Down:  down,
			Up:    up,
			Right: right,
		}
		data, err := expected.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		var actual state.Input
		err = actual.UnmarshalBinary(data)
		if err != nil {
			t.Fatal(err)
		}

		if expected != actual {
			t.Errorf("expected input %#v; actual %#v", expected, actual)
		}
	})
}

func FuzzState(f *testing.F) {
	f.Add(0.0, 0.0, 0.0, 0.0, 0.0, 0.0)
	f.Fuzz(func(t *testing.T, transX, transY, velX, velY, accelX, accelY float64) {
		expected := state.State{
			Player: state.Movable{
				Trans: state.Vec2{transX, transY},
				Vel:   state.Vec2{velX, velY},
				Accel: state.Vec2{accelX, accelY},
			},
		}
		data, err := expected.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		var actual state.State
		err = actual.UnmarshalBinary(data)
		if err != nil {
			t.Fatal(err)
		}

		if expected != actual {
			t.Errorf("expected state %#v; actual %#v", expected, actual)
		}
	})
}
