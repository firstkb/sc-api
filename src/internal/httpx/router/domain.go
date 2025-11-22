package router

import (
	"net/http"
	"net/url"
	"strings"
)

// ExtractDomain normalizes the Origin header (if present) to a host value.
// Returns "undefined" if the Origin header is missing or invalid.
func ExtractDomain(r *http.Request) string {
	if r == nil {
		return "undefined"
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		return "undefined"
	}

	if parsed, err := url.Parse(origin); err == nil && parsed.Host != "" {
		return parsed.Host
	}

	domain := strings.TrimPrefix(origin, "http://")
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimSuffix(domain, "/")
	if domain == "" {
		return "undefined"
	}
	return domain
}
