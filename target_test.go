package main

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
