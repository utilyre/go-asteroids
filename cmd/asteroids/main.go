package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"image"
	"io/fs"
	"log/slog"
	"multiplayer/internal/cli"
	_ "multiplayer/internal/config"
	"multiplayer/internal/game"
	"multiplayer/internal/simulation"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed assets
var assetFSys embed.FS

func main() {
	var (
		serverAddr string
		remoteAddr string
	)
	flag.StringVar(&serverAddr, "listen", "", "specify address to listen on")
	flag.StringVar(&remoteAddr, "connect", "", "specify remote address for connecting to a server")
	flag.Parse()

	var errs []error
	imgPlayer, err := openImage(assetFSys, "assets/PLAYER.png")
	errs = append(errs, err)
	imgBullet, err := openImage(assetFSys, "assets/BULLET.png")
	errs = append(errs, err)
	imgRock, err := openImage(assetFSys, "assets/ROCK.png")
	errs = append(errs, err)
	if err := errors.Join(errs...); err != nil {
		slog.Error("failed to open image", "error", err)
		return
	}

	ctx, cancel := cli.NewSignalContext()
	defer cancel()

	if len(serverAddr) > 0 {
		listenAndSimulate(
			ctx,
			serverAddr,
			ebiten.NewImageFromImage(imgPlayer),
			ebiten.NewImageFromImage(imgBullet),
			ebiten.NewImageFromImage(imgRock),
		)
	} else if len(remoteAddr) > 0 {
		connectAndRun(
			ctx,
			remoteAddr,
			ebiten.NewImageFromImage(imgPlayer),
			ebiten.NewImageFromImage(imgBullet),
			ebiten.NewImageFromImage(imgRock),
		)
	} else {
		slog.Error("please specify either a -listen flag or a -connect flag")
		os.Exit(1)
	}
}

func listenAndSimulate(ctx context.Context, addr string, imgPlayer, imgBullet, imgRock *ebiten.Image) {
	sim, err := simulation.New(addr, imgPlayer, imgBullet, imgRock)
	if err != nil {
		slog.Error("failed to instantiate simulation", "error", err)
		return
	}
	defer func() {
		err = sim.Close(ctx)
		if err != nil {
			slog.Error("failed to close simulation", "error", err)
		}
	}()

	ebiten.SetWindowTitle("Asteroids [SERVER]")
	ebiten.SetWindowSize(640, 360)
	ebiten.SetTPS(10)
	err = ebiten.RunGame(sim)
	if err != nil {
		slog.Error("failed to run simulation as an ebiten game", "error", err)
		return
	}
}

func connectAndRun(ctx context.Context, raddr string, imgPlayer, imgBullet, imgRock *ebiten.Image) {
	g, err := game.New(ctx, raddr, imgPlayer, imgBullet, imgRock)
	if err != nil {
		slog.Error("failed to initialize game", "error", err)
		return
	}
	defer func() {
		err = g.Close(ctx)
		if err != nil {
			slog.Error("failed to close game", "error", err)
		}
	}()

	ebiten.SetWindowTitle("Asteroids")
	ebiten.SetWindowSize(640, 360)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	err = ebiten.RunGame(g)
	if err != nil {
		slog.Error("failed to run game as an ebiten game", "error", err)
		return
	}
}

func openImage(fsys fs.FS, name string) (img image.Image, err error) {
	f, err := fsys.Open(name)
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
