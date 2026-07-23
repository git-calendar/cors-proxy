package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/lmittmann/tint"
	"github.com/sethvargo/go-envconfig"
)

var cfg *config

type config struct {
	Host            string        `env:"HOST,default=0.0.0.0"`
	Port            string        `env:"PORT,default=8080"`
	Production      bool          `env:"PRODUCTION,default=false"`
	UpstreamTimeout time.Duration `env:"UPSTREAM_TIMEOUT,default=15s"`
	MaxResponseSize int64         `env:"MAX_RESPONSE_SIZE,default=1048576"` // 1MiB
	AllowedHosts    []string      `env:"ALLOWED_HOSTS,default=github.com,raw.githubusercontent.com,gitlab.com,codeberg.org"`
	RateTokens      uint64        `env:"RATE_TOKENS,default=60"` // 60 req/min should be ok for legit usage
	RateInterval    time.Duration `env:"RATE_INTERVAL,default=1m"`
	IPSourceHeader  string        `env:"RATE_IP_SOURCE_HEADER"` // for reverse proxy
}

func loadConfig() {
	cfg = envconfig.MustProcess(context.Background(), &config{})

	// trim spaces for hosts
	for i := range cfg.AllowedHosts {
		cfg.AllowedHosts[i] = strings.TrimSpace(cfg.AllowedHosts[i])
	}
}

func setupLogger() {
	options := &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	}

	var handler slog.Handler = tint.NewHandler(os.Stdout, &tint.Options{
		Level:      options.Level,
		AddSource:  options.AddSource,
		TimeFormat: time.DateTime,
	})
	if cfg.Production {
		handler = slog.NewJSONHandler(os.Stdout, options)
	}

	slog.SetDefault(slog.New(handler))
}
