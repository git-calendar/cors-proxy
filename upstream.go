package main

import (
	"net"
	"net/http"
	"strings"
	"time"
)

// a transport used for upstream requests
var roundTripper http.RoundTripper = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	MaxIdleConnsPerHost:   1000,
	IdleConnTimeout:       60 * time.Second,
	TLSHandshakeTimeout:   5 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
	ResponseHeaderTimeout: 10 * time.Second,
	ForceAttemptHTTP2:     true,
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

func copyHeaders(source, destination http.Header) {
	for name, values := range source {
		for _, value := range values {
			destination.Add(name, value)
		}
	}
}

func removeHopByHopHeaders(headers http.Header) {
	for header := range strings.SplitSeq(headers.Get("Connection"), ",") {
		headers.Del(strings.TrimSpace(header))
	}

	for name := range hopByHopHeaders {
		headers.Del(name)
	}
}
