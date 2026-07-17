package provider

import (
	"context"
	"sync"
	"time"
)

// DefaultResolveTTL is 04 §Semantics' default resolve-cache TTL: "local
// surfaces cache results with a short TTL (default 15m) so lenses never
// hammer the tracker."
const DefaultResolveTTL = 15 * time.Minute

// Clock abstracts wall-clock reads so TTL expiry is deterministic in
// tests (CLAUDE.md: "No wall-clock or randomness ... except declared
// stamps" and the general "no network/no wall-clock in tests" testing
// discipline). Production callers pass SystemClock{}.
type Clock interface {
	Now() time.Time
}

// SystemClock is the real wall clock.
type SystemClock struct{}

// Now returns time.Now().
func (SystemClock) Now() time.Time { return time.Now() }

// CachingProvider decorates a StoryProvider, caching Resolve results for
// ttl (04 §Semantics). On an underlying Resolve failure it degrades: a
// stale cached entry is served instead of erroring; with no cached entry
// the underlying error is surfaced unchanged. CachingProvider never
// blocks rendering itself — per 04 §Semantics ("On failure, degrade to
// displaying the raw ref; never block rendering"), a caller that
// receives an error from Resolve is expected to render the raw StoryRef
// rather than block, since CachingProvider has no rendered view to fall
// back to.
//
// PublishRollup is not cached: 04 §Semantics scopes caching to Resolve
// only (publish is a CI-only, one-shot write), so it passes straight
// through to the inner provider.
//
// CachingProvider is safe for concurrent use.
//
// Disclosure (spec/code-health dc-5): CachingProvider is constructed
// only by its own tests. It is deliberately unwired — no production
// caller exists — with single-flight (collapsing concurrent Resolve
// calls for the same ref into one inner fetch) and eviction (bounding
// cache growth) recorded here as prerequisites for whenever a caller
// (e.g. serve) wires it in. An exported capability nothing calls is a
// disclosure problem: without this note, the reader would believe a
// live defense exists.
type CachingProvider struct {
	inner StoryProvider
	ttl   time.Duration
	clock Clock

	mu    sync.Mutex
	cache map[StoryRef]cacheEntry
}

type cacheEntry struct {
	story   Story
	fetched time.Time
}

// NewCachingProvider wraps inner with a resolve cache. ttl <= 0 uses
// DefaultResolveTTL. clock must be non-nil; pass SystemClock{} in
// production, an injectable fake in tests.
func NewCachingProvider(inner StoryProvider, ttl time.Duration, clock Clock) *CachingProvider {
	if ttl <= 0 {
		ttl = DefaultResolveTTL
	}
	return &CachingProvider{
		inner: inner,
		ttl:   ttl,
		clock: clock,
		cache: make(map[StoryRef]cacheEntry),
	}
}

// Resolve returns the cached Story for ref if it was fetched within ttl;
// otherwise it calls the inner provider. On an inner failure it serves a
// stale cached entry if one exists, otherwise it returns the inner
// error unchanged.
func (c *CachingProvider) Resolve(ctx context.Context, ref StoryRef) (Story, error) {
	c.mu.Lock()
	entry, cached := c.cache[ref]
	fresh := cached && c.clock.Now().Sub(entry.fetched) < c.ttl
	c.mu.Unlock()

	if fresh {
		return entry.story, nil
	}

	story, err := c.inner.Resolve(ctx, ref)
	if err != nil {
		if cached {
			// Degrade: serve the stale value rather than block
			// rendering (04 §Semantics).
			return entry.story, nil
		}
		return Story{}, err
	}

	c.mu.Lock()
	c.cache[ref] = cacheEntry{story: story, fetched: c.clock.Now()}
	c.mu.Unlock()
	return story, nil
}

// PublishRollup delegates to the inner provider unchanged; publishes are
// never cached.
func (c *CachingProvider) PublishRollup(ctx context.Context, r Rollup) error {
	return c.inner.PublishRollup(ctx, r)
}

var _ StoryProvider = (*CachingProvider)(nil)
