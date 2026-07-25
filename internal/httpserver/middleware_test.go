package httpserver

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
	t.Parallel()

	calls := 0
	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusCreated)
	}), "*")

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

	preflight := httptest.NewRecorder()
	handler.ServeHTTP(preflight, httptest.NewRequest(http.MethodOptions, "/target", nil))
	if preflight.Code != http.StatusNoContent || calls != 1 {
		t.Fatalf("OPTIONS response = status %d, calls %d; want 204, 1", preflight.Code, calls)
	}
}

func TestRateLimitByRemoteAddress(t *testing.T) {
	t.Parallel()

	handler, err := rateLimit(noContentHandler(), rateLimitOptions{
		production: true,
		tokens:     1,
		interval:   time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	request := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/target", nil)
		r.RemoteAddr = "203.0.113.10:54321"
		return r
	}

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, request())
	if first.Code != http.StatusNoContent {
		t.Fatalf("first response = %d, want %d", first.Code, http.StatusNoContent)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, request())
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second response = %d, want %d", second.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimitUsesTrustedIPHeader(t *testing.T) {
	t.Parallel()

	handler, err := rateLimit(noContentHandler(), rateLimitOptions{
		production:     true,
		tokens:         1,
		interval:       time.Hour,
		ipSourceHeader: "X-Real-IP",
	})
	if err != nil {
		t.Fatal(err)
	}

	request := func(clientIP string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/target", nil)
		r.RemoteAddr = "192.0.2.10:54321"
		r.Header.Set("X-Real-IP", clientIP)
		return r
	}

	for _, clientIP := range []string{"203.0.113.10, 192.0.2.1", "203.0.113.11, 192.0.2.1"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request(clientIP))
		if response.Code != http.StatusNoContent {
			t.Fatalf("first response for %s = %d, want %d", clientIP, response.Code, http.StatusNoContent)
		}
	}

	limited := httptest.NewRecorder()
	handler.ServeHTTP(limited, request("203.0.113.10, 198.51.100.99"))
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("repeated client response = %d, want %d", limited.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimitInvalidTrustedHeaderFallsBackToPeer(t *testing.T) {
	t.Parallel()

	handler, err := rateLimit(noContentHandler(), rateLimitOptions{
		production:     true,
		tokens:         1,
		interval:       time.Hour,
		ipSourceHeader: "X-Real-IP",
	})
	if err != nil {
		t.Fatal(err)
	}

	for attempt := range 2 {
		request := httptest.NewRequest(http.MethodGet, "/target", nil)
		request.RemoteAddr = "192.0.2.10:54321"
		request.Header.Set("X-Real-IP", "not-an-ip")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		want := http.StatusNoContent
		if attempt == 1 {
			want = http.StatusTooManyRequests
		}
		if response.Code != want {
			t.Fatalf("attempt %d response = %d, want %d", attempt+1, response.Code, want)
		}
	}
}

func TestRateLimitIsDisabledOutsideProduction(t *testing.T) {
	t.Parallel()

	handler, err := rateLimit(noContentHandler(), rateLimitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/target", nil))
		if response.Code != http.StatusNoContent {
			t.Fatalf("response = %d, want %d", response.Code, http.StatusNoContent)
		}
	}
}

func TestAccessLog(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	handler := accessLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}), slog.New(slog.NewTextHandler(&output, nil)))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/favicon.ico?cache=1", nil))
	if response.Code != http.StatusTeapot {
		t.Fatalf("response = %d, want %d", response.Code, http.StatusTeapot)
	}
	for _, field := range []string{
		"msg=access",
		"method=POST",
		"target_type=other",
		"path=/favicon.ico",
		"status=418",
		"duration=",
	} {
		if !strings.Contains(output.String(), field) {
			t.Errorf("access log %q does not contain %q", output.String(), field)
		}
	}
	if strings.Contains(output.String(), "cache=1") {
		t.Errorf("access log %q contains the query string", output.String())
	}
}

func TestAccessLogRedactsUpstreamPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		requestURI string
		host       string
		targetType string
		sensitive  []string
	}{
		{
			name:       "calendar",
			requestURI: "/https://calendar.example.com/private/feed-token.ics?key=secret",
			host:       "calendar.example.com",
			targetType: "ical",
			sensitive:  []string{"private", "feed-token", "key=secret"},
		},
		{
			name:       "git",
			requestURI: "/https://token@github.com/firu11/test-cal-enc.git/info/refs?service=git-upload-pack",
			host:       "github.com",
			targetType: "git",
			sensitive:  []string{"token", "firu11", "test-cal-enc", "git-upload-pack"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer
			handler := accessLog(noContentHandler(), slog.New(slog.NewTextHandler(&output, nil)))
			handler.ServeHTTP(
				httptest.NewRecorder(),
				httptest.NewRequest(http.MethodGet, test.requestURI, nil),
			)

			logEntry := output.String()
			for _, field := range []string{
				"target_host=" + test.host,
				"target_type=" + test.targetType,
			} {
				if !strings.Contains(logEntry, field) {
					t.Errorf("access log %q does not contain %q", logEntry, field)
				}
			}
			for _, value := range test.sensitive {
				if strings.Contains(logEntry, value) {
					t.Errorf("access log %q contains sensitive value %q", logEntry, value)
				}
			}
		})
	}
}

func noContentHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}
