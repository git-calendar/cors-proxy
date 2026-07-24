package main

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCORSMiddleware(t *testing.T) {
	calls := 0
	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusCreated)
	}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/target", nil))

	if response.Code != http.StatusCreated || calls != 1 {
		t.Fatalf("GET response = status %d, calls %d; want 201, 1", response.Code, calls)
	}
	for name, want := range map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
		"Access-Control-Allow-Headers": "Authorization, Content-Type, Git-Protocol",
	} {
		if got := response.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	preflightResponse := httptest.NewRecorder()
	handler.ServeHTTP(preflightResponse, httptest.NewRequest(http.MethodOptions, "/target", nil))
	if preflightResponse.Code != http.StatusNoContent || calls != 1 {
		t.Fatalf("OPTIONS response = status %d, calls %d; want 204, 1", preflightResponse.Code, calls)
	}
}

func TestRateLimitByRemoteAddressAndBypassesHealthCheck(t *testing.T) {
	originalConfig := cfg
	t.Cleanup(func() { cfg = originalConfig })
	cfg = &config{Production: true, RateTokens: 1, RateInterval: time.Hour}

	calls := 0
	handler := rateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))

	request := func(path string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.RemoteAddr = "203.0.113.10:54321"
		return r
	}

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, request("/target"))
	if first.Code != http.StatusNoContent {
		t.Fatalf("first response status = %d, want %d", first.Code, http.StatusNoContent)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, request("/target"))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second response status = %d, want %d", second.Code, http.StatusTooManyRequests)
	}

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, request("/"))
	if health.Code != http.StatusNoContent || calls != 2 {
		t.Fatalf("health response = status %d, calls %d; want 204, 2", health.Code, calls)
	}
}

func TestRateLimitUsesTrustedIPHeader(t *testing.T) {
	originalConfig := cfg
	t.Cleanup(func() { cfg = originalConfig })
	cfg = &config{
		Production:     true,
		RateTokens:     1,
		RateInterval:   time.Hour,
		IPSourceHeader: "X-Real-IP",
	}

	handler := rateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := func(clientIP string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/target", nil)
		r.RemoteAddr = "192.0.2.10:54321"
		r.Header.Set("X-Real-IP", clientIP)
		return r
	}

	for _, clientIP := range []string{"203.0.113.10", "203.0.113.11"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request(clientIP))
		if response.Code != http.StatusNoContent {
			t.Fatalf("first response for %s = %d, want %d", clientIP, response.Code, http.StatusNoContent)
		}
	}

	limited := httptest.NewRecorder()
	handler.ServeHTTP(limited, request("203.0.113.10"))
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("repeated client response = %d, want %d", limited.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimitIsDisabledOutsideProduction(t *testing.T) {
	originalConfig := cfg
	t.Cleanup(func() { cfg = originalConfig })
	cfg = &config{Production: false}

	calls := 0
	handler := rateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))

	for range 2 {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/target", nil)
		request.RemoteAddr = "203.0.113.10:54321"
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("response status = %d, want %d", response.Code, http.StatusNoContent)
		}
	}
	if calls != 2 {
		t.Fatalf("handler calls = %d, want 2", calls)
	}
}

func TestAccessLog(t *testing.T) {
	originalLogger := slog.Default()
	t.Cleanup(func() { slog.SetDefault(originalLogger) })

	var output bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
	handler := accessLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/target", nil))
	if response.Code != http.StatusTeapot {
		t.Fatalf("response status = %d, want %d", response.Code, http.StatusTeapot)
	}
	logLine := output.String()
	for _, field := range []string{"msg=access", "method=POST", "status=418"} {
		if !strings.Contains(logLine, field) {
			t.Errorf("access log %q does not contain %q", logLine, field)
		}
	}

	output.Reset()
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if output.Len() != 0 {
		t.Fatalf("health check log = %q, want none", output.String())
	}
}
