package httpserver

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/git-calendar/cors-proxy/internal/proxy"
	"github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/httplimit"
	"github.com/sethvargo/go-limiter/memorystore"
	"github.com/sethvargo/go-limiter/noopstore"
)

type rateLimitOptions struct {
	production     bool
	tokens         uint64
	interval       time.Duration
	ipSourceHeader string
}

// rateLimit limits proxy requests by their validated client IP.
func rateLimit(next http.Handler, options rateLimitOptions) (http.Handler, error) {
	var (
		store limiter.Store
		err   error
	)
	if options.production {
		store, err = memorystore.New(&memorystore.Config{
			Tokens:   options.tokens,
			Interval: options.interval,
		})
	} else {
		store, err = noopstore.New()
	}
	if err != nil {
		return nil, err
	}

	middleware, err := httplimit.NewMiddleware(store, clientIPKey(options.ipSourceHeader))
	if err != nil {
		return nil, err
	}
	return middleware.Handle(next), nil
}

func clientIPKey(sourceHeader string) httplimit.KeyFunc {
	return func(request *http.Request) (string, error) {
		if sourceHeader != "" {
			value, _, _ := strings.Cut(request.Header.Get(sourceHeader), ",")
			if ip := net.ParseIP(strings.TrimSpace(value)); ip != nil {
				return ip.String(), nil
			}
		}

		host, _, err := net.SplitHostPort(request.RemoteAddr)
		if err != nil {
			return "", fmt.Errorf("invalid client address %q: %w", request.RemoteAddr, err)
		}
		ip := net.ParseIP(host)
		if ip == nil {
			return "", fmt.Errorf("invalid client IP %q", host)
		}
		return ip.String(), nil
	}
}

// corsMiddleware adds CORS headers so browsers can use the proxy.
func corsMiddleware(next http.Handler, origins string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origins)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Git-Protocol")

		// Handle preflight requests locally so upstream Git servers cannot reject them.
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// accessLog logs each proxy request after execution.
func accessLog(next http.Handler, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		writer := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(writer, r)

		attributes := []any{
			"method", r.Method,
			"status", writer.status,
			"duration", time.Since(start),
		}
		if host, targetType, ok := proxy.TargetMetadata(r.URL); ok {
			attributes = append(attributes, "target_host", host, "target_type", targetType)
		} else {
			attributes = append(attributes, "target_type", "other", "path", r.URL.Path)
		}
		logger.Info("access", attributes...)
	})
}

// responseWriter captures the response status code for access logging.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
