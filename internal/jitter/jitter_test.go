package jitter_test

import (
	"multiplayer/internal/jitter"
	"multiplayer/internal/state"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJitter_MarshalBinary(t *testing.T) {
	buf := jitter.NewBufferFrom([]state.Input{
		{Left: false, Down: false, Up: true, Right: false},
		{Left: true, Down: false, Up: true, Right: true},
		{Left: true, Down: true, Up: true, Right: false},
		{Left: false, Down: false, Up: false, Right: false},
		{Left: true, Down: true, Up: false, Right: false},
		{Left: false, Down: false, Up: false, Right: true},
	})
	data, err := buf.MarshalBinary()
	assert.NoError(t, err)

	assert.Equal(t, []byte{
		0, 0, 0, 6, // numInputs
		0, 0, 0, 0, // index
		0b00000100, // input
		0, 0, 0, 1, // index
		0b00001101, // input
		0, 0, 0, 2, // index
		0b00000111, // input
		0, 0, 0, 3, // index
		0b00000000, // input
		0, 0, 0, 4, // index
		0b00000011, // input
		0, 0, 0, 5, // index
		0b00001000, // input
	}, data)
}

func TestJitter_UnmarshalBinary(t *testing.T) {
	data := []byte{
		0, 0, 0, 4, // numInputs
		0, 0, 0, 8, // index
		0b00000101, // input
		0, 0, 0, 9, // index
		0b00001101,  // input
		0, 0, 0, 10, // index
		0b00000010,  // input
		0, 0, 0, 11, // index
		0b00000110, // input
	}

	var buf jitter.Buffer
	err := buf.UnmarshalBinary(data)
	assert.NoError(t, err)

	assert.Equal(t, []state.Input{
		{Left: true, Down: false, Up: true, Right: false},
		{Left: true, Down: false, Up: true, Right: true},
		{Left: false, Down: true, Up: false, Right: false},
		{Left: false, Down: true, Up: true, Right: false},
	}, buf.Inputs())

	assert.Equal(t, []uint32{8, 9, 10, 11}, buf.Indices())
}

func TestJitter_DiscardUntil(t *testing.T) {
	var buf jitter.Buffer
	buf.Append(state.Input{}) // index = 0
	buf.Append(state.Input{}) // index = 1
	buf.Append(state.Input{}) // index = 2
	buf.Append(state.Input{}) // index = 3
	buf.Append(state.Input{}) // index = 4
	buf.Append(state.Input{}) // index = 5

	assert.Equal(t, []uint32{0, 1, 2, 3, 4, 5}, buf.Indices())
	buf.DiscardUntil(2)
	assert.Equal(t, []uint32{3, 4, 5}, buf.Indices())
	buf.DiscardUntil(1)
	assert.Equal(t, []uint32{3, 4, 5}, buf.Indices())
	buf.DiscardUntil(0)
	assert.Equal(t, []uint32{3, 4, 5}, buf.Indices())
	buf.DiscardUntil(6)
	assert.Equal(t, []uint32{}, buf.Indices())
}
