package engines

import (
	"net/url"
	"strings"
)

// NormalizeHost returns the lowercase hostname without port, trailing dot
// or surrounding whitespace. Returns "" when the URL has no usable host.
func NormalizeHost(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	h := strings.ToLower(strings.TrimSpace(u.Hostname()))
	h = strings.TrimSuffix(h, ".")
	return h
}

// ValidSubmitURL reports whether raw is an http(s) URL with a usable host.
func ValidSubmitURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return NormalizeHost(raw) != ""
}

// hostWithoutWWW returns the netloc without port and without a leading "www.".
func hostWithoutWWW(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	host = strings.TrimPrefix(host, "www.")
	return host
}

// pathOf returns the URL path (no query/fragment).
func pathOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Path
}

func splitNonEmpty(s, sep string) []string {
	var out []string
	for _, p := range strings.Split(s, sep) {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// lastPathSegment returns the last non-empty segment of a URL path.
func lastPathSegment(path string) string {
	parts := splitNonEmpty(path, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
