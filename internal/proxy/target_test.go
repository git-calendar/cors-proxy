package proxy

import (
	"net/url"
	"testing"
)

func TestIsAllowedPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path    string
		allowed bool
	}{
		{path: "/calendar.ics", allowed: true},
		{path: "/calendars/work.ICS", allowed: true},
		{path: "/repository.git/info/refs", allowed: true},
		{path: "/repository.git/git-upload-pack", allowed: true},
		{path: "/repository.git/git-receive-pack/", allowed: true},
		{path: "/calendar.ics/json", allowed: false},
		{path: "/calendar.ics.json", allowed: false},
		{path: "/calendarics", allowed: false},
		{path: "/repository.git/objects/abc", allowed: false},
		{path: "/", allowed: false},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			t.Parallel()

			destination := &url.URL{Path: test.path}
			if got := isAllowedPath(destination); got != test.allowed {
				t.Fatalf("isAllowedPath(%q) = %t, want %t", test.path, got, test.allowed)
			}
		})
	}

	if isAllowedPath(nil) {
		t.Fatal("isAllowedPath(nil) = true, want false")
	}
}

func TestTargetURLAllowsOnlyHTTP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		target  string
		allowed bool
	}{
		{target: "/https://example.com/calendar.ics", allowed: true},
		{target: "/HTTP://example.com/calendar.ics", allowed: true},
		{target: "/ftp://example.com/calendar.ics", allowed: false},
		{target: "/file:///etc/passwd", allowed: false},
	}

	for _, test := range tests {
		t.Run(test.target, func(t *testing.T) {
			t.Parallel()

			requestURL := &url.URL{Path: test.target}
			if got := targetURL(requestURL) != nil; got != test.allowed {
				t.Fatalf("targetURL(%q) allowed = %t, want %t", test.target, got, test.allowed)
			}
		})
	}
}

func TestTargetMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		requestPath string
		host        string
		targetType  string
		ok          bool
	}{
		{
			requestPath: "/https://calendar.example.com/private/feed-token.ics",
			host:        "calendar.example.com",
			targetType:  "ical",
			ok:          true,
		},
		{
			requestPath: "/https://token@github.com/org/private.git/info/refs",
			host:        "github.com",
			targetType:  "git",
			ok:          true,
		},
		{
			requestPath: "/https://example.com/unsupported",
			host:        "example.com",
			targetType:  "other",
			ok:          true,
		},
		{requestPath: "/favicon.ico"},
	}

	for _, test := range tests {
		t.Run(test.requestPath, func(t *testing.T) {
			t.Parallel()

			host, targetType, ok := TargetMetadata(&url.URL{Path: test.requestPath})
			if host != test.host || targetType != test.targetType || ok != test.ok {
				t.Fatalf(
					"TargetMetadata(%q) = (%q, %q, %t), want (%q, %q, %t)",
					test.requestPath,
					host,
					targetType,
					ok,
					test.host,
					test.targetType,
					test.ok,
				)
			}
		})
	}
}

func TestNewNormalizesAllowedHosts(t *testing.T) {
	t.Parallel()

	handler := New(Options{AllowedHosts: []string{" Example.COM ", "EXAMPLE.com"}})
	if len(handler.allowedHosts) != 1 {
		t.Fatalf("allowed host count = %d, want 1", len(handler.allowedHosts))
	}
	if !handler.isAllowedHost(&url.URL{Host: "EXAMPLE.COM:443"}) {
		t.Fatal("normalized allowed host did not match mixed-case destination")
	}
}
