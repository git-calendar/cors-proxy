package main

import (
	"io"
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

func TestProxyHandlerSanitizesHeaders(t *testing.T) {
	originalConfig := cfg
	originalRoundTripper := roundTripper
	t.Cleanup(func() {
		cfg = originalConfig
		roundTripper = originalRoundTripper
	})

	cfg = &config{
		AllowedHosts:    []string{"example.com"},
		UpstreamTimeout: time.Second,
		MaxResponseSize: 1024,
	}

	roundTripper = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		for _, name := range []string{"Cookie", "Cookie2", "Proxy"} {
			if values := request.Header.Values(name); len(values) != 0 {
				t.Fatalf("upstream %s headers = %q, want none", name, values)
			}
		}
		if got := request.Header.Get("Authorization"); got != "Basic private-repository-credentials" {
			t.Fatalf("upstream Authorization = %q, want private Git credentials", got)
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

	request := httptest.NewRequest(http.MethodGet, "/https://example.com/repo.git/info/refs", nil)
	request.Header.Add("Cookie", "session=secret")
	request.Header.Add("Cookie", "tracking=secret")
	request.Header.Set("Cookie2", "legacy=secret")
	request.Header.Set("Proxy", "http://attacker.example")
	request.Header.Set("Authorization", "Basic private-repository-credentials")
	response := httptest.NewRecorder()

	proxyHandler(response, request)

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
