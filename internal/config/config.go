package config

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

func init() {
	logger := slog.New(tint.NewHandler(os.Stderr, &tint.Options{
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
	}))
	slog.SetDefault(logger)
}
