package httpserver

import (
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Options configures the HTTP listener, local routes, and rate limiter.
type Options struct {
	Host           string
	Port           string
	Production     bool
	RateTokens     uint64
	RateInterval   time.Duration
	IPSourceHeader string
	AbuseContact   string
}

// New constructs the HTTP server and its complete handler stack.
func New(options Options, proxyHandler http.Handler, logger *slog.Logger) (*http.Server, error) {
	handler, err := NewHandler(options, proxyHandler, logger)
	if err != nil {
		return nil, err
	}

	return &http.Server{
		Addr:           net.JoinHostPort(options.Host, options.Port),
		Handler:        handler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   15 * time.Second,
		MaxHeaderBytes: 64 << 10,
	}, nil
}

// NewHandler constructs local routes and applies middleware only to proxy traffic.
func NewHandler(options Options, proxyHandler http.Handler, logger *slog.Logger) (http.Handler, error) {
	limitedProxy, err := rateLimit(proxyHandler, rateLimitOptions{
		production:     options.Production,
		tokens:         options.RateTokens,
		interval:       options.RateInterval,
		ipSourceHeader: options.IPSourceHeader,
	})
	if err != nil {
		return nil, err
	}

	return &router{
		health:   http.HandlerFunc(serveHealth),
		security: newSecurityHandler(options.AbuseContact, time.Now),
		proxy:    accessLog(corsMiddleware(limitedProxy), logger),
	}, nil
}
