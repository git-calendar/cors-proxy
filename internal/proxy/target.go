package proxy

import (
	"net/url"
	"strings"
)

// targetURL extracts the destination URL from the incoming request path.
func targetURL(requestURL *url.URL) *url.URL {
	target := strings.TrimPrefix(requestURL.Path, "/")

	destinationURL, err := url.Parse(target)
	if err != nil || destinationURL.Scheme == "" || destinationURL.Host == "" {
		return nil
	}

	destinationURL.Scheme = strings.ToLower(destinationURL.Scheme)
	if destinationURL.Scheme != "http" && destinationURL.Scheme != "https" {
		return nil
	}

	if requestURL.RawQuery != "" {
		destinationURL.RawQuery = requestURL.RawQuery
	}

	return destinationURL
}

// isAllowedPath reports whether the destination is an iCalendar file or a Git
// smart HTTP endpoint.
func isAllowedPath(destinationURL *url.URL) bool {
	if destinationURL == nil {
		return false
	}

	return strings.HasSuffix(strings.ToLower(destinationURL.Path), ".ics") || isGitRequest(destinationURL)
}

func isGitRequest(destinationURL *url.URL) bool {
	if destinationURL == nil {
		return false
	}

	path := strings.TrimSuffix(destinationURL.Path, "/")
	return strings.HasSuffix(path, "/info/refs") ||
		strings.HasSuffix(path, "/git-upload-pack") ||
		strings.HasSuffix(path, "/git-receive-pack")
}

// isAllowedHost checks the destination host against the configured allowlist.
func (h *Handler) isAllowedHost(destinationURL *url.URL) bool {
	if destinationURL == nil {
		return false
	}

	_, allowed := h.allowedHosts[strings.ToLower(destinationURL.Hostname())]
	return allowed
}
