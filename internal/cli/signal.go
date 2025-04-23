package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func NewSignalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	quitCh := make(chan os.Signal, 1)
	signal.Notify(
		quitCh,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGHUP,
		syscall.SIGQUIT,
		syscall.SIGPIPE,
	)

	go func() {
		wasSIGINT := false

		for sig := range quitCh {
			if wasSIGINT && sig == syscall.SIGINT {
				os.Exit(1)
			}

			wasSIGINT = sig == syscall.SIGINT
			cancel()
		}
	}()

	return ctx, cancel
}
