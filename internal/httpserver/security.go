package httpserver

import (
	"fmt"
	"net/http"
	"time"
)

// securityHandler serves the deployment's RFC 9116 security contact.
type securityHandler struct {
	contact string
	now     func() time.Time
}

func newSecurityHandler(contact string, now func() time.Time) *securityHandler {
	return &securityHandler{contact: contact, now: now}
}

func (h *securityHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	expires := h.now().UTC().AddDate(1, 0, 0).Format(time.RFC3339)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}

	_, _ = fmt.Fprintf(w, "Contact: %s\nExpires: %s\nPreferred-Languages: en\n", h.contact, expires)
}
