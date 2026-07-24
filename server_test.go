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

func TestSecurityTxt(t *testing.T) {
	originalConfig := cfg
	t.Cleanup(func() { cfg = originalConfig })
	cfg = &config{AbuseURL: "mailto:abuse@proxy.example"}

	before := time.Now().UTC()
	response := httptest.NewRecorder()
	proxyHandler(response, httptest.NewRequest(http.MethodGet, securityTxtPath, nil))

	if response.Code != http.StatusOK {
		t.Fatalf("response status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/plain; charset=utf-8", got)
	}

	fields := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(response.Body.String()), "\n") {
		name, value, ok := strings.Cut(line, ": ")
		if ok {
			fields[name] = value
		}
	}
	if got := fields["Contact"]; got != "mailto:abuse@proxy.example" {
		t.Fatalf("Contact = %q, want configured abuse contact", got)
	}
	if got := fields["Preferred-Languages"]; got != "en" {
		t.Fatalf("Preferred-Languages = %q, want en", got)
	}

	expires, err := time.Parse(time.RFC3339, fields["Expires"])
	if err != nil {
		t.Fatalf("Expires = %q, want RFC3339 timestamp: %v", fields["Expires"], err)
	}
	if expires.Before(before.AddDate(0, 11, 0)) || expires.After(before.AddDate(1, 0, 1)) {
		t.Fatalf("Expires = %s, want approximately one year from now", expires)
	}
}

func TestSecurityTxtHeadAndMethodNotAllowed(t *testing.T) {
	originalConfig := cfg
	t.Cleanup(func() { cfg = originalConfig })
	cfg = &config{AbuseURL: "mailto:abuse@proxy.example"}

	headResponse := httptest.NewRecorder()
	proxyHandler(headResponse, httptest.NewRequest(http.MethodHead, securityTxtPath, nil))
	if headResponse.Code != http.StatusOK || headResponse.Body.Len() != 0 {
		t.Fatalf("HEAD response = status %d, body %q; want 200 with empty body", headResponse.Code, headResponse.Body.String())
	}

	postResponse := httptest.NewRecorder()
	proxyHandler(postResponse, httptest.NewRequest(http.MethodPost, securityTxtPath, nil))
	if postResponse.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST response status = %d, want %d", postResponse.Code, http.StatusMethodNotAllowed)
	}
	if got := postResponse.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("Allow = %q, want GET, HEAD", got)
	}
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
		AbuseURL:        "https://proxy.example/report-abuse",
	}

	roundTripper = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		for _, name := range []string{"Cookie", "Cookie2", "Proxy"} {
			if values := request.Header.Values(name); len(values) != 0 {
				t.Fatalf("upstream %s headers = %q, want none", name, values)
			}
		}
		for _, name := range forwardingHeaders {
			if name == "X-Forwarded-For" {
				continue
			}
			if values := request.Header.Values(name); len(values) != 0 {
				t.Fatalf("upstream spoofed %s headers = %q, want none", name, values)
			}
		}
		if got := request.Header.Get("X-Forwarded-For"); got != "203.0.113.10" {
			t.Fatalf("upstream X-Forwarded-For = %q, want verified client IP", got)
		}
		if got := request.Header.Get("User-Agent"); got != "GitCalendarCorsProxy/1.0 (+https://proxy.example/report-abuse)" {
			t.Fatalf("upstream User-Agent = %q, want proxy identity", got)
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
	request.Header.Set("User-Agent", "AttackerProxy/1.0")
	for _, name := range forwardingHeaders {
		request.Header.Set(name, "198.51.100.99")
	}
	request.RemoteAddr = "203.0.113.10:54321"
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
