package provider

import "errors"

// Sentinel errors adapters return (wrapped with %w and context, per
// CLAUDE.md's error-wrapping rule) to report 04's failure table. Callers
// use errors.Is to tell them apart and apply the matching behavior:
//
//	NotFound            lint warning on the spec's story: link; ref shown raw
//	Unauthorized         fail the publish job loudly (credential drift is a real error)
//	Unavailable/timeout   Resolve: degrade + cache stale; Publish: job retry
var (
	// ErrNotFound means the tracker has no story at the given ref.
	ErrNotFound = errors.New("provider: story not found")

	// ErrUnauthorized means the adapter's credentials were rejected by
	// the tracker.
	ErrUnauthorized = errors.New("provider: unauthorized")

	// ErrUnavailable means the tracker could not be reached, the call
	// timed out, or the tracker rate-limited the call (HTTP 429 classifies
	// here too, spec/forge-transport ac-3: an uncached rate-limited call
	// must route to the same degrade/retry path a 5xx does, not hard-fail).
	ErrUnavailable = errors.New("provider: unavailable")
)
