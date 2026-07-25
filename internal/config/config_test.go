package config

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	t.Setenv("UPSTREAM_TIMEOUT", "not-a-duration")

	if _, err := Load(context.Background()); err == nil {
		t.Fatal("Load() succeeded, want invalid duration error")
	}
}

func TestLoad(t *testing.T) {
	t.Setenv("HOST", "127.0.0.1")
	t.Setenv("PORT", "9090")
	t.Setenv("PRODUCTION", "true")
	t.Setenv("UPSTREAM_TIMEOUT", "3s")
	t.Setenv("MAX_RESPONSE_SIZE", "2048")
	t.Setenv("ALLOWED_HOSTS", " GitHub.COM , Example.COM ")
	t.Setenv("RATE_TOKENS", "7")
	t.Setenv("RATE_INTERVAL", "30s")
	t.Setenv("RATE_IP_SOURCE_HEADER", " X-Real-IP ")
	t.Setenv("ABUSE_CONTACT", " mailto:security@example.com ")

	got, err := Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := Config{
		Host:            "127.0.0.1",
		Port:            "9090",
		Production:      true,
		UpstreamTimeout: 3 * time.Second,
		MaxResponseSize: 2048,
		AllowedHosts:    []string{"github.com", "example.com"},
		CORSAllowOrigin: "*",
		RateTokens:      7,
		RateInterval:    30 * time.Second,
		IPSourceHeader:  "X-Real-IP",
		AbuseContact:    "mailto:security@example.com",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}
}
