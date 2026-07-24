package main

import (
	"net"
	"net/http"
	"strings"
	"time"
)

var upstreamDialer = newSSRFSafeDialer(net.DefaultResolver, &net.Dialer{
	Timeout:   5 * time.Second,
	KeepAlive: 30 * time.Second,
})

// a transport used for upstream requests
var roundTripper http.RoundTripper = &http.Transport{
	DialContext:           upstreamDialer.DialContext,
	MaxIdleConnsPerHost:   1000,
	IdleConnTimeout:       60 * time.Second,
	TLSHandshakeTimeout:   5 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
	ResponseHeaderTimeout: 10 * time.Second,
	ForceAttemptHTTP2:     true,
	Proxy:                 nil, // intentionally nil: an HTTP proxy would resolve the target again and bypass the SSRF-safe dialer
}

// Hop-by-hop headers must not be forwarded by proxies.
// https://www.rfc-editor.org/rfc/rfc2616?ref=journal.hexmos.com#section-13.5.1
var hopByHopHeaders = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"TE":                  true,
	"Trailers":            true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

var forwardingHeaders = []string{
	"Forwarded",
	"X-Forwarded-For",
	"X-Forwarded-Host",
	"X-Forwarded-Proto",
	"X-Forwarded-Port",
	"X-Real-IP",
	"CF-Connecting-IP",
	"True-Client-IP",
}

func copyHeaders(source, destination http.Header) {
	for name, values := range source {
		for _, value := range values {
			destination.Add(name, value)
		}
	}
}

func sanitizeRequestHeaders(headers http.Header) {
	removeHopByHopHeaders(headers)
	headers.Del("Cookie")
	headers.Del("Cookie2")
	headers.Del("Proxy")

	for _, name := range forwardingHeaders {
		headers.Del(name)
	}
}

func setTraceHeaders(headers http.Header, request *http.Request) {
	if ip := originalClientIP(request, cfg.IPSourceHeader); ip != "" {
		headers.Set("X-Forwarded-For", ip)
	}
	headers.Set("User-Agent", "GitCalendarCorsProxy/1.0 (+"+cfg.AbuseURL+")")
}

func originalClientIP(request *http.Request, sourceHeader string) string {
	if sourceHeader != "" {
		value, _, _ := strings.Cut(request.Header.Get(sourceHeader), ",")
		if ip := net.ParseIP(strings.TrimSpace(value)); ip != nil {
			return ip.String()
		}
	}

	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		return ""
	}
	return host
}

func sanitizeResponseHeaders(headers http.Header) {
	removeHopByHopHeaders(headers)
	headers.Del("Set-Cookie")
	headers.Del("Set-Cookie2")
	headers.Del("Clear-Site-Data")
	headers.Del("Alt-Svc")
}

func removeHopByHopHeaders(headers http.Header) {
	for header := range strings.SplitSeq(headers.Get("Connection"), ",") {
		headers.Del(strings.TrimSpace(header))
	}

	for name := range hopByHopHeaders {
		headers.Del(name)
	}
}
