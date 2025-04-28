package config

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
)

func init() {
	w := os.Stderr
	var handler slog.Handler
	if isatty.IsTerminal(w.Fd()) {
		handler = tint.NewHandler(w, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.TimeOnly,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if err, ok := a.Value.Any().(error); ok {
					aErr := tint.Err(err)
					aErr.Key = a.Key
					return aErr
				}
				return a
			},
		})
	} else {
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}
