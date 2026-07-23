package main

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/httplimit"
	"github.com/sethvargo/go-limiter/memorystore"
	"github.com/sethvargo/go-limiter/noopstore"
)

// Rate limits by IP. Skips health-check (GET /).
func rateLimit(next http.Handler) http.Handler {
	var (
		store limiter.Store
		err   error
	)

	if cfg.Production {
		store, err = memorystore.New(&memorystore.Config{
			Tokens:   cfg.RateTokens,
			Interval: cfg.RateInterval,
		})
	} else {
		store, err = noopstore.New()
	}
	if err != nil {
		panic(err)
	}

	// wrap the middleware to skip rate limiting for health checks
	var keyFunc httplimit.KeyFunc
	if cfg.IPSourceHeader != "" {
		keyFunc = httplimit.IPKeyFunc(cfg.IPSourceHeader)
	} else {
		keyFunc = httplimit.IPKeyFunc()
	}

	middleware, err := httplimit.NewMiddleware(store, keyFunc)
	if err != nil {
		panic(err)
	}

	// wrap the middleware to skip rate limiting for health checks
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isHealthCheck(r) {
			next.ServeHTTP(w, r) // bypass rate limiter for health checks
			return
		}
		middleware.Handle(next).ServeHTTP(w, r) // apply rate-limiting
	})
}

// Adds CORS headers to allow any browser to use this endpoint.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // TODO: add real https://git-calendar.org or whatever
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Git-Protocol")

		// handle preflight OPTIONS request
		if r.Method == http.MethodOptions {
			// this prevents the 405 from e.g., GitHub
			// the browser only needs to get the CORS headers and OK for OPTIONS request,
			// so that it knows it's safe to send the real request
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r) // next handler
	})
}

// Logs access after execution.
func accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()                                                 // start timer
		writer := &responseWriter{ResponseWriter: w, status: http.StatusOK} // wrap the writer into our custom one

		next.ServeHTTP(writer, r) // next handler

		if isHealthCheck(r) {
			return // don't log healthchecks
		}
		slog.Info(
			"access",
			"method", r.Method,
			"status", writer.status,
			"duration", time.Since(start),
		)
	})
}

// A http.ResponseWriter wrapper, which catches the status code for logging.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
