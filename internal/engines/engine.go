package engines

import "context"

// Engine is the search engine abstraction, mirroring core/engines/base.py.
type Engine interface {
	// Name returns the engine display name.
	Name() string

	// CheckIndexed reports whether a URL is indexed.
	// Returns &true / &false / nil(error nil) ; nil result with non-nil error
	// means the check failed (maps to Python's None).
	CheckIndexed(ctx context.Context, url string) (*bool, error)

	// SubmitURLs submits URLs via IndexNow, returning a {host: ok} map.
	SubmitURLs(ctx context.Context, urls []string, apiKey string) (map[string]bool, error)

	// Close releases browser resources held by the engine.
	Close() error
}

// BatchResult describes the outcome of a single IndexNow POST (≤10k URLs).
type BatchResult struct {
	Ok bool
	// StatusCode is the HTTP status (0 = network/context error).
	StatusCode int
	// Reason is a short machine-readable code: ok, bad_key, mismatch,
	// bad_request, rate_limited, server_error, network_error, cancelled.
	Reason string
	// Detail carries a truncated response body or error text for display.
	Detail string
}

// BatchSubmitter is implemented by engines that can POST one batch and
// report a per-batch reason (used by the submitter for slice-accurate status).
type BatchSubmitter interface {
	SubmitBatch(ctx context.Context, host, apiKey string, batch []string) BatchResult
}
