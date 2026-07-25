package config

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

// NewLogger creates the configured development or production logger.
func NewLogger(production bool) *slog.Logger {
	options := &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	}

	var handler slog.Handler = tint.NewHandler(os.Stdout, &tint.Options{
		Level:      options.Level,
		AddSource:  options.AddSource,
		TimeFormat: time.DateTime,
	})
	if production {
		handler = slog.NewJSONHandler(os.Stdout, options)
	}

	return slog.New(handler)
}
