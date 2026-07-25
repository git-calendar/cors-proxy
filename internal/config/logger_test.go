package config

import (
	"context"
	"log/slog"
	"testing"
)

func TestNewLogger(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		production bool
	}{
		{name: "development"},
		{name: "production", production: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			logger := NewLogger(test.production)
			if logger == nil {
				t.Fatal("NewLogger() returned nil")
			}
			if !logger.Enabled(context.Background(), slog.LevelDebug) {
				t.Fatal("NewLogger() did not enable debug logging")
			}
		})
	}
}
