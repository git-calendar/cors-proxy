package httpserver

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
	t.Parallel()

	server, err := New(Options{Host: "127.0.0.1", Port: "9090"}, http.NotFoundHandler(), discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	if server.Addr != "127.0.0.1:9090" {
		t.Fatalf("server address = %q, want 127.0.0.1:9090", server.Addr)
	}
	if server.ReadTimeout != 10*time.Second || server.WriteTimeout != 15*time.Second {
		t.Fatalf("server timeouts = %s/%s, want 10s/15s", server.ReadTimeout, server.WriteTimeout)
	}
	if server.MaxHeaderBytes != 64<<10 {
		t.Fatalf("MaxHeaderBytes = %d, want %d", server.MaxHeaderBytes, 64<<10)
	}
}

func TestHandlerRoutesLocalAndProxyRequests(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	proxyCalls := 0
	proxyHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		proxyCalls++
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, "proxied")
	})
	handler, err := NewHandler(Options{
		AbuseContact: "mailto:security@example.com",
	}, proxyHandler, logger)
	if err != nil {
		t.Fatal(err)
	}

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/", nil))
	if health.Code != http.StatusOK || health.Body.String() != "git calendar cors proxy" {
		t.Fatalf("health response = status %d, body %q", health.Code, health.Body.String())
	}
	if health.Header().Get("Access-Control-Allow-Origin") != "" || logs.Len() != 0 {
		t.Fatal("health route unexpectedly used proxy middleware")
	}

	security := httptest.NewRecorder()
	handler.ServeHTTP(security, httptest.NewRequest(http.MethodGet, securityTxtPath, nil))
	if security.Code != http.StatusOK || !strings.Contains(security.Body.String(), "Contact: mailto:security@example.com") {
		t.Fatalf("security response = status %d, body %q", security.Code, security.Body.String())
	}
	if security.Header().Get("Access-Control-Allow-Origin") != "" || logs.Len() != 0 {
		t.Fatal("security route unexpectedly used proxy middleware")
	}

	proxied := httptest.NewRecorder()
	handler.ServeHTTP(proxied, httptest.NewRequest(http.MethodGet, "/https://example.com/calendar.ics", nil))
	if proxied.Code != http.StatusAccepted || proxied.Body.String() != "proxied" {
		t.Fatalf("proxy response = status %d, body %q", proxied.Code, proxied.Body.String())
	}
	if proxied.Header().Get("Access-Control-Allow-Origin") != "*" || proxyCalls != 1 {
		t.Fatalf("proxy CORS/calls = %q/%d, want */1", proxied.Header().Get("Access-Control-Allow-Origin"), proxyCalls)
	}
	if !strings.Contains(logs.String(), "msg=access") {
		t.Fatalf("proxy access log = %q, want access message", logs.String())
	}

	preflight := httptest.NewRecorder()
	handler.ServeHTTP(preflight, httptest.NewRequest(http.MethodOptions, "/https://example.com/calendar.ics", nil))
	if preflight.Code != http.StatusNoContent || proxyCalls != 1 {
		t.Fatalf("preflight = status %d, proxy calls %d; want 204, 1", preflight.Code, proxyCalls)
	}
}

func TestSecurityHandler(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 25, 12, 0, 0, 0, time.UTC)
	handler := newSecurityHandler("mailto:security@firu.dev", func() time.Time { return now })

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, securityTxtPath, nil))
	wantBody := "Contact: mailto:security@firu.dev\nExpires: 2027-07-25T12:00:00Z\nPreferred-Languages: en\n"
	if response.Code != http.StatusOK || response.Body.String() != wantBody {
		t.Fatalf("security response = status %d, body %q; want 200, %q", response.Code, response.Body.String(), wantBody)
	}
	if got := response.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/plain; charset=utf-8", got)
	}

	head := httptest.NewRecorder()
	handler.ServeHTTP(head, httptest.NewRequest(http.MethodHead, securityTxtPath, nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("HEAD response = status %d, body %q; want 200, empty", head.Code, head.Body.String())
	}

	post := httptest.NewRecorder()
	handler.ServeHTTP(post, httptest.NewRequest(http.MethodPost, securityTxtPath, nil))
	if post.Code != http.StatusMethodNotAllowed || post.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST response = status %d, Allow %q", post.Code, post.Header().Get("Allow"))
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
