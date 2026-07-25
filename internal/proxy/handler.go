package proxy

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Options configures a Handler.
type Options struct {
	AllowedHosts    []string
	UpstreamTimeout time.Duration
	MaxResponseSize int64
	IPSourceHeader  string
	AbuseContact    string
	Transport       http.RoundTripper
	Logger          *slog.Logger
}

// Handler forwards allowed calendar and Git smart HTTP requests to upstreams.
type Handler struct {
	allowedHosts    map[string]struct{}
	upstreamTimeout time.Duration
	maxResponseSize int64
	ipSourceHeader  string
	abuseContact    string
	transport       http.RoundTripper
	logger          *slog.Logger
}

// New constructs a proxy handler. When no transport is supplied, it creates a
// transport that validates and pins public upstream IP addresses before dialing.
func New(options Options) *Handler {
	allowedHosts := make(map[string]struct{}, len(options.AllowedHosts))
	for _, host := range options.AllowedHosts {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" {
			allowedHosts[host] = struct{}{}
		}
	}

	transport := options.Transport
	if transport == nil {
		transport = newDefaultTransport()
	}

	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		allowedHosts:    allowedHosts,
		upstreamTimeout: options.UpstreamTimeout,
		maxResponseSize: options.MaxResponseSize,
		ipSourceHeader:  strings.TrimSpace(options.IPSourceHeader),
		abuseContact:    strings.TrimSpace(options.AbuseContact),
		transport:       transport,
		logger:          logger,
	}
}

// ServeHTTP handles outbound proxy requests only.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// get the destination URL from the path
	destinationURL := targetURL(r.URL)
	if destinationURL == nil {
		http.Error(w, "invalid target URL", http.StatusBadRequest)
		return
	}

	// only allow calendar files and Git smart HTTP endpoints
	if !isAllowedPath(destinationURL) {
		http.Error(w, "target must be an .ics file or Git smart HTTP endpoint", http.StatusBadRequest)
		return
	}

	// reject unknown hosts
	if !h.isAllowedHost(destinationURL) {
		http.Error(w, "forbidden upstream host", http.StatusForbidden)
		return
	}

	// prepare the request to the destination
	request, err := http.NewRequest(r.Method, destinationURL.String(), r.Body)
	if err != nil {
		http.Error(w, "failed to create outbound request", http.StatusInternalServerError)
		h.logger.Error("failed to create outbound request", "error", err)
		return
	}
	copyHeaders(r.Header, request.Header)
	sanitizeRequestHeaders(request.Header, h.ipSourceHeader)
	setTraceHeaders(request.Header, r, h.ipSourceHeader, h.abuseContact)

	// add context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), h.upstreamTimeout)
	defer cancel()
	request = request.WithContext(ctx)

	// send the actual request
	response, err := h.transport.RoundTrip(request)
	if err != nil {
		http.Error(w, fmt.Sprintf("upstream request failed with: %v", err), http.StatusBadGateway)
		h.logger.Error("upstream request failed", "error", err)
		return
	}
	defer response.Body.Close()

	// forward the headers back to the client
	sanitizeResponseHeaders(response.Header)
	copyHeaders(response.Header, w.Header())
	w.WriteHeader(response.StatusCode)

	// limit the response body
	limitedReader := &io.LimitedReader{
		R: response.Body,
		N: h.maxResponseSize + 1, // +1 to detect overflow
	}
	// forward the response body back to the client
	n, err := io.Copy(w, limitedReader)
	if err != nil {
		h.logger.Error("failed to forward upstream response", "error", err)
		return
	}
	if n > h.maxResponseSize {
		h.logger.Info("response too large")
	}
}
