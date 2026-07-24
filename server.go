package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

func newServer() *http.Server {
	handler := http.HandlerFunc(proxyHandler)

	return &http.Server{
		Addr:           fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Handler:        accessLog(corsMiddleware(rateLimit(handler))),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   15 * time.Second,
		MaxHeaderBytes: 64 << 10,
	}
}

const securityTxtPath = "/.well-known/security.txt"

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == securityTxtPath {
		serveSecurityTxt(w, r)
		return
	}

	if isHealthCheck(r) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("git calendar cors proxy"))
		return
	}

	// get the destination url from path
	destUrl := targetURL(r.URL)
	if destUrl == nil {
		http.Error(w, "invalid target URL", http.StatusBadRequest)
		return
	}

	// only allow calendar files and Git smart HTTP endpoints
	if !isAllowedPath(destUrl) {
		http.Error(w, "target must be an .ics file or Git smart HTTP endpoint", http.StatusBadRequest)
		return
	}

	// reject unknown hosts
	if !isAllowedHost(destUrl) {
		http.Error(w, "forbidden upstream host", http.StatusForbidden)
		return
	}

	// prepare the request to destination
	req, err := http.NewRequest(r.Method, destUrl.String(), r.Body)
	if err != nil {
		http.Error(w, "failed to create outbound request", http.StatusInternalServerError)
		slog.Error(err.Error())
		return
	}
	copyHeaders(r.Header, req.Header)
	sanitizeRequestHeaders(req.Header)
	setTraceHeaders(req.Header, r)

	// add context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), cfg.UpstreamTimeout)
	defer cancel()
	req = req.WithContext(ctx)

	// send the actual request
	resp, err := roundTripper.RoundTrip(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("upstream request failed with: %v", err), http.StatusBadGateway)
		slog.Error(err.Error())
		return
	}
	defer resp.Body.Close()

	// forward the headers back to client
	sanitizeResponseHeaders(resp.Header)
	copyHeaders(resp.Header, w.Header())
	w.WriteHeader(resp.StatusCode)

	// limit response body
	limitedReader := &io.LimitedReader{
		R: resp.Body,
		N: cfg.MaxResponseSize + 1, // to detect overflow
	}

	// forward response body back to client
	n, err := io.Copy(w, limitedReader)
	if err != nil {
		slog.Error(err.Error())
		return
	}

	if n > cfg.MaxResponseSize {
		slog.Info("response too large")
		return
	}
}

func serveSecurityTxt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}

	expires := time.Now().UTC().AddDate(1, 0, 0).Format(time.RFC3339)
	fmt.Fprintf(w, "Contact: %s\nExpires: %s\nPreferred-Languages: en\n", cfg.AbuseURL, expires)
}
