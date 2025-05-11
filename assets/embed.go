package assets

import (
	"bytes"
	_ "embed"
	"image"
	_ "image/png"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/examples/resources/fonts"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

var (
	MPlus1pRegular *text.GoTextFaceSource

	Player *ebiten.Image
	Bullet *ebiten.Image
	Rock   *ebiten.Image
)

var (
	//go:embed PLAYER.png
	imgPlayer []byte

	//go:embed BULLET.png
	imgBullet []byte

	//go:embed ROCK.png
	imgRock []byte
)

func init() {
	MPlus1pRegular = must(text.NewGoTextFaceSource(bytes.NewReader(fonts.MPlus1pRegular_ttf)))

	pngPlayer, _ := must2(image.Decode(bytes.NewReader(imgPlayer)))
	Player = ebiten.NewImageFromImage(pngPlayer)
	pngBullet, _ := must2(image.Decode(bytes.NewReader(imgBullet)))
	Bullet = ebiten.NewImageFromImage(pngBullet)
	pngRock, _ := must2(image.Decode(bytes.NewReader(imgRock)))
	Rock = ebiten.NewImageFromImage(pngRock)
}

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

func must2[T, U any](value1 T, value2 U, err error) (T, U) {
	if err != nil {
		panic(err)
	}
	return value1, value2
}
