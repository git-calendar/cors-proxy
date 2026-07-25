package config

import (
	"context"
	"strings"
	"time"

	"github.com/sethvargo/go-envconfig"
)

// Config contains the proxy's environment-based configuration.
type Config struct {
	Host       string `env:"HOST,default=0.0.0.0"`
	Port       string `env:"PORT,default=8080"`
	Production bool   `env:"PRODUCTION,default=false"`

	AllowedHosts    []string `env:"ALLOWED_HOSTS,default=github.com,raw.githubusercontent.com,gitlab.com,codeberg.org"`
	CORSAllowOrigin string   `env:"CORS_ALLOW_ORIGIN,default=*"` // Access-Control-Allow-Origin
	AbuseContact    string   `env:"ABUSE_CONTACT"`
	IPSourceHeader  string   `env:"RATE_IP_SOURCE_HEADER"` // trusted reverse proxy header

	UpstreamTimeout time.Duration `env:"UPSTREAM_TIMEOUT,default=15s"`
	MaxResponseSize int64         `env:"MAX_RESPONSE_SIZE,default=1048576"` // 1 MiB
	RateTokens      uint64        `env:"RATE_TOKENS,default=60"`            // 60 requests per minute should be enough for legitimate usage
	RateInterval    time.Duration `env:"RATE_INTERVAL,default=1m"`
}

// Load reads configuration from the environment and normalizes string values.
func Load(ctx context.Context) (Config, error) {
	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		return Config{}, err
	}

	for i := range cfg.AllowedHosts {
		cfg.AllowedHosts[i] = strings.ToLower(strings.TrimSpace(cfg.AllowedHosts[i]))
	}
	cfg.IPSourceHeader = strings.TrimSpace(cfg.IPSourceHeader)
	cfg.AbuseContact = strings.TrimSpace(cfg.AbuseContact)

	return cfg, nil
}
