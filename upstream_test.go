package main

import (
	"net/http/httptest"
	"testing"
)

func TestOriginalClientIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		remoteAddr   string
		sourceHeader string
		headerValue  string
		want         string
	}{
		{
			name:       "direct IPv4 client",
			remoteAddr: "203.0.113.10:54321",
			want:       "203.0.113.10",
		},
		{
			name:       "direct IPv6 client",
			remoteAddr: "[2001:db8::10]:54321",
			want:       "2001:db8::10",
		},
		{
			name:         "trusted proxy header",
			remoteAddr:   "192.0.2.20:54321",
			sourceHeader: "X-Forwarded-For",
			headerValue:  "203.0.113.10, 192.0.2.10",
			want:         "203.0.113.10",
		},
		{
			name:         "invalid trusted header falls back to peer",
			remoteAddr:   "192.0.2.20:54321",
			sourceHeader: "X-Real-IP",
			headerValue:  "not-an-ip",
			want:         "192.0.2.20",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest("GET", "/", nil)
			request.RemoteAddr = test.remoteAddr
			if test.sourceHeader != "" {
				request.Header.Set(test.sourceHeader, test.headerValue)
			}

			if got := originalClientIP(request, test.sourceHeader); got != test.want {
				t.Fatalf("originalClientIP() = %q, want %q", got, test.want)
			}
		})
	}
}
