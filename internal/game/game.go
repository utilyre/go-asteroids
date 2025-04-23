package game

import (
	"context"
	"errors"
	"image"
	_ "image/png"
	"multiplayer/internal/mcp"
	"multiplayer/internal/state"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	ticksPerSecond = 60 // ebiten's default
	deltaTickTime  = time.Second / ticksPerSecond
)

type Game struct {
	houseImg *ebiten.Image
	sess     *mcp.Session
	state    state.State
	quit     bool
}

func New(ctx context.Context, raddr string) (*Game, error) {
	houseImg, err := openImage("./assets/house.png")
	if err != nil {
		return nil, err
	}

	sess, err := mcp.Dial(ctx, raddr)
	if err != nil {
		return nil, err
	}

	return &Game{
		houseImg: ebiten.NewImageFromImage(houseImg),
		sess:     sess,
		state:    state.State{},
		quit:     false,
	}, nil
}

func (g *Game) Close(ctx context.Context) error {
	g.Stop()
	return g.sess.Close(ctx)
}

func (g *Game) Stop() {
	g.quit = true
}

func (g *Game) Layout(int, int) (int, int) {
	return 640, 480
}

func (g *Game) Draw(screen *ebiten.Image) {
	var m ebiten.GeoM
	m.Scale(0.2, 0.2)
	m.Translate(g.state.House.Trans.X, g.state.House.Trans.Y)
	screen.DrawImage(g.houseImg, &ebiten.DrawImageOptions{
		GeoM: m,
	})
}

func (g *Game) Update() error {
	if g.quit {
		return ebiten.Termination
	}

	input := state.Input{
		Left:  ebiten.IsKeyPressed(ebiten.KeyH),
		Down:  ebiten.IsKeyPressed(ebiten.KeyJ),
		Up:    ebiten.IsKeyPressed(ebiten.KeyK),
		Right: ebiten.IsKeyPressed(ebiten.KeyL),
	}

	g.state.Update(deltaTickTime, input)
	return nil
}

func openImage(name string) (img image.Image, err error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()

	img, _, err = image.Decode(f)
	if err != nil {
		return nil, err
	}

	return img, nil
}
