package main

import (
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

var gitPathPattern = regexp.MustCompile(`/(info/refs|git-upload-pack|git-receive-pack)/?$`)

// Extracts the target/destination URL from the path of the original request.
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

// Checks if the destination path is an iCalendar file or Git smart HTTP endpoint.
func isAllowedPath(destinationURL *url.URL) bool {
	if destinationURL == nil {
		return false
	}

	return strings.HasSuffix(strings.ToLower(destinationURL.Path), ".ics") || isGitRequest(destinationURL)
}

func isGitRequest(destinationURL *url.URL) bool {
	return destinationURL != nil && gitPathPattern.MatchString(destinationURL.Path)
}

// Checks target/destination host with the allowlist.
func isAllowedHost(destinationURL *url.URL) bool {
	if destinationURL == nil {
		return false
	}

	host := strings.ToLower(destinationURL.Hostname())
	return slices.Contains(cfg.AllowedHosts, host)
}

func isHealthCheck(request *http.Request) bool {
	return request.Method == http.MethodGet && request.URL.Path == "/"
}
