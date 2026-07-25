package proxy

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestHandlerSanitizesHeaders(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		for _, name := range []string{"Cookie", "Cookie2", "Proxy", "Fly-Client-IP"} {
			if values := request.Header.Values(name); len(values) != 0 {
				t.Fatalf("upstream %s headers = %q, want none", name, values)
			}
		}
		for _, name := range forwardingHeaderNames() {
			if name == "X-Forwarded-For" {
				continue
			}
			if values := request.Header.Values(name); len(values) != 0 {
				t.Fatalf("upstream spoofed %s headers = %q, want none", name, values)
			}
		}
		if got := request.Header.Get("X-Forwarded-For"); got != "198.51.100.42" {
			t.Fatalf("upstream X-Forwarded-For = %q, want verified client IP", got)
		}
		if got := request.Header.Get("User-Agent"); got != "GitCalendarCorsProxy/1.0 (+https://proxy.example/report-abuse)" {
			t.Fatalf("upstream User-Agent = %q, want proxy identity", got)
		}
		if got := request.Header.Get("Authorization"); got != "Basic private-repository-credentials" {
			t.Fatalf("upstream Authorization = %q, want private Git credentials", got)
		}
		if _, ok := request.Context().Deadline(); !ok {
			t.Fatal("upstream request context has no timeout deadline")
		}

		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header: http.Header{
				"Set-Cookie":       {"session=secret", "tracking=secret"},
				"Set-Cookie2":      {"legacy=secret"},
				"Clear-Site-Data":  {"\"cookies\""},
				"Alt-Svc":          {"h3=\":443\""},
				"WWW-Authenticate": {"Basic realm=private-repository"},
				"X-Test":           {"forwarded"},
			},
			Body: io.NopCloser(strings.NewReader("authentication required")),
		}, nil
	})

	handler := New(Options{
		AllowedHosts:    []string{"example.com"},
		UpstreamTimeout: time.Second,
		MaxResponseSize: 1024,
		IPSourceHeader:  "Fly-Client-IP",
		AbuseContact:    "https://proxy.example/report-abuse",
		Transport:       transport,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	request := httptest.NewRequest(http.MethodGet, "/https://example.com/repo.git/info/refs", nil)
	request.Header.Add("Cookie", "session=secret")
	request.Header.Add("Cookie", "tracking=secret")
	request.Header.Set("Cookie2", "legacy=secret")
	request.Header.Set("Proxy", "http://attacker.example")
	request.Header.Set("Authorization", "Basic private-repository-credentials")
	request.Header.Set("User-Agent", "AttackerProxy/1.0")
	request.Header.Set("Fly-Client-IP", "198.51.100.42")
	for _, name := range forwardingHeaderNames() {
		request.Header.Set(name, "198.51.100.99")
	}
	request.RemoteAddr = "203.0.113.10:54321"
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("response status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	for _, name := range []string{"Set-Cookie", "Set-Cookie2", "Clear-Site-Data", "Alt-Svc"} {
		if values := response.Header().Values(name); len(values) != 0 {
			t.Fatalf("client %s headers = %q, want none", name, values)
		}
	}
	if got := response.Header().Get("WWW-Authenticate"); got != "Basic realm=private-repository" {
		t.Fatalf("WWW-Authenticate = %q, want private Git challenge", got)
	}
	if got := response.Header().Get("X-Test"); got != "forwarded" {
		t.Fatalf("X-Test header = %q, want forwarded", got)
	}
}

func TestHandlerDoesNotServeAncillaryRoutes(t *testing.T) {
	t.Parallel()

	handler := New(Options{})
	for _, path := range []string{"/", "/.well-known/security.txt"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusBadRequest {
			t.Errorf("GET %s status = %d, want %d", path, response.Code, http.StatusBadRequest)
		}
	}
}
