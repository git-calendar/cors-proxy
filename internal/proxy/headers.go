package proxy

import (
	"net"
	"net/http"
	"strings"
)

func copyHeaders(source, destination http.Header) {
	for name, values := range source {
		for _, value := range values {
			destination.Add(name, value)
		}
	}
}

func sanitizeRequestHeaders(headers http.Header, ipSourceHeader string) {
	removeHopByHopHeaders(headers)
	headers.Del("Cookie")
	headers.Del("Cookie2")
	headers.Del("Proxy")
	headers.Del(ipSourceHeader)

	for _, name := range forwardingHeaderNames() {
		headers.Del(name)
	}
}

func setTraceHeaders(headers http.Header, request *http.Request, ipSourceHeader, abuseContact string) {
	if ip := originalClientIP(request, ipSourceHeader); ip != "" {
		headers.Set("X-Forwarded-For", ip)
	}
	headers.Set("User-Agent", "GitCalendarCorsProxy/1.0 (+"+abuseContact+")")
}

func originalClientIP(request *http.Request, sourceHeader string) string {
	if sourceHeader != "" {
		value, _, _ := strings.Cut(request.Header.Get(sourceHeader), ",")
		if ip := net.ParseIP(strings.TrimSpace(value)); ip != nil {
			return ip.String()
		}
	}

	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		return ""
	}
	return host
}

func sanitizeResponseHeaders(headers http.Header) {
	removeHopByHopHeaders(headers)
	headers.Del("Set-Cookie")
	headers.Del("Set-Cookie2")
	headers.Del("Clear-Site-Data")
	headers.Del("Alt-Svc")
}

// Hop-by-hop headers must not be forwarded by proxies.
// https://www.rfc-editor.org/rfc/rfc2616#section-13.5.1
func removeHopByHopHeaders(headers http.Header) {
	for header := range strings.SplitSeq(headers.Get("Connection"), ",") {
		headers.Del(strings.TrimSpace(header))
	}

	for _, name := range []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"TE",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	} {
		headers.Del(name)
	}
}

func forwardingHeaderNames() []string {
	return []string{
		"Forwarded",
		"X-Forwarded-For",
		"X-Forwarded-Host",
		"X-Forwarded-Proto",
		"X-Forwarded-Port",
		"X-Real-IP",
		"CF-Connecting-IP",
		"True-Client-IP",
	}
}
