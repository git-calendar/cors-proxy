package proxy

import (
	"net"
	"net/http"
	"time"
)

// newDefaultTransport creates the transport used for upstream requests.
func newDefaultTransport() *http.Transport {
	dialer := newSSRFSafeDialer(net.DefaultResolver, &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	})

	return &http.Transport{
		DialContext:           dialer.DialContext,
		MaxIdleConnsPerHost:   1000,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ForceAttemptHTTP2:     true,
		// An HTTP proxy would resolve the target again and bypass the SSRF-safe dialer.
		Proxy: nil,
	}
}
