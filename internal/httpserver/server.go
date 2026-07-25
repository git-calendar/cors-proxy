package httpserver

import (
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/git-calendar/cors-proxy/internal/config"
)

// New constructs the HTTP server and its complete handler stack.
func New(cfg config.Config, proxyHandler http.Handler, logger *slog.Logger) (*http.Server, error) {
	handler, err := NewHandler(cfg, proxyHandler, logger)
	if err != nil {
		return nil, err
	}

	return &http.Server{
		Addr:           net.JoinHostPort(cfg.Host, cfg.Port),
		Handler:        handler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   15 * time.Second,
		MaxHeaderBytes: 64 << 10,
	}, nil
}

// NewHandler constructs local routes and applies middleware only to proxy traffic.
func NewHandler(cfg config.Config, proxyHandler http.Handler, logger *slog.Logger) (http.Handler, error) {
	limitedProxy, err := rateLimit(proxyHandler, rateLimitOptions{
		production:     cfg.Production,
		tokens:         cfg.RateTokens,
		interval:       cfg.RateInterval,
		ipSourceHeader: cfg.IPSourceHeader,
	})
	if err != nil {
		return nil, err
	}

	return &router{
		health:   http.HandlerFunc(serveHealth),
		security: newSecurityHandler(cfg.AbuseContact, time.Now),
		proxy:    accessLog(corsMiddleware(limitedProxy, cfg.CORSAllowOrigin), logger),
	}, nil
}
