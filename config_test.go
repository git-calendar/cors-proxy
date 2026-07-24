package main

import (
	"reflect"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	originalConfig := cfg
	t.Cleanup(func() { cfg = originalConfig })

	t.Setenv("HOST", "127.0.0.1")
	t.Setenv("PORT", "9090")
	t.Setenv("PRODUCTION", "true")
	t.Setenv("UPSTREAM_TIMEOUT", "3s")
	t.Setenv("MAX_RESPONSE_SIZE", "2048")
	t.Setenv("ALLOWED_HOSTS", " github.com , example.com ")
	t.Setenv("RATE_TOKENS", "7")
	t.Setenv("RATE_INTERVAL", "30s")
	t.Setenv("RATE_IP_SOURCE_HEADER", " X-Real-IP ")
	t.Setenv("ABUSE_URL", " mailto:security@example.com ")

	loadConfig()

	want := &config{
		Host:            "127.0.0.1",
		Port:            "9090",
		Production:      true,
		UpstreamTimeout: 3 * time.Second,
		MaxResponseSize: 2048,
		AllowedHosts:    []string{"github.com", "example.com"},
		RateTokens:      7,
		RateInterval:    30 * time.Second,
		IPSourceHeader:  "X-Real-IP",
		AbuseURL:        "mailto:security@example.com",
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("loadConfig() = %+v, want %+v", cfg, want)
	}
}
