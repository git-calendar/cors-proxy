package httpserver

import (
	"io"
	"net/http"
)

const securityTxtPath = "/.well-known/security.txt"

type router struct {
	health   http.Handler
	security http.Handler
	proxy    http.Handler
}

// ServeHTTP routes exact local endpoints without cleaning embedded target URLs.
func (h *router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/":
		h.health.ServeHTTP(w, r)
	case r.URL.Path == securityTxtPath:
		h.security.ServeHTTP(w, r)
	default:
		h.proxy.ServeHTTP(w, r)
	}
}

// serveHealth reports that the proxy process is ready to accept requests.
func serveHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "git calendar cors proxy")
}
