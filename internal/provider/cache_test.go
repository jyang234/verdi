package provider_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/verdi/internal/provider"
)

// manualClock is an injectable provider.Clock for deterministic TTL
// tests (CLAUDE.md: no wall-clock in tests).
type manualClock struct {
	mu  sync.Mutex
	now time.Time
}

func newManualClock(start time.Time) *manualClock {
	return &manualClock{now: start}
}

func (c *manualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *manualClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// countingProvider is a minimal StoryProvider stub that counts Resolve
// calls and can be told to fail, used to observe CachingProvider's
// caching decisions directly (whether it called through or not).
type countingProvider struct {
	mu       sync.Mutex
	calls    int
	story    provider.Story
	failWith error
}

func (p *countingProvider) Resolve(ctx context.Context, ref provider.StoryRef) (provider.Story, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if p.failWith != nil {
		return provider.Story{}, p.failWith
	}
	return p.story, nil
}

func (p *countingProvider) PublishRollup(ctx context.Context, r provider.Rollup) error {
	return nil
}

func (p *countingProvider) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

func TestNewCachingProvider_DefaultTTL(t *testing.T) {
	inner := &countingProvider{story: provider.Story{Ref: "jira:X-1"}}
	clock := newManualClock(time.Unix(0, 0))

	// ttl <= 0 should fall back to DefaultResolveTTL: a value just
	// under 15m stays fresh; at/after 15m it re-fetches.
	c := provider.NewCachingProvider(inner, 0, clock)
	ref := provider.StoryRef("jira:X-1")

	if _, err := c.Resolve(context.Background(), ref); err != nil {
		t.Fatalf("Resolve error = %v, want nil", err)
	}
	clock.Advance(provider.DefaultResolveTTL - time.Second)
	if _, err := c.Resolve(context.Background(), ref); err != nil {
		t.Fatalf("Resolve error = %v, want nil", err)
	}
	if got := inner.callCount(); got != 1 {
		t.Fatalf("inner call count = %d just under default TTL, want 1 (cache hit)", got)
	}

	clock.Advance(2 * time.Second) // now past 15m since the first fetch
	if _, err := c.Resolve(context.Background(), ref); err != nil {
		t.Fatalf("Resolve error = %v, want nil", err)
	}
	if got := inner.callCount(); got != 2 {
		t.Fatalf("inner call count = %d after default TTL elapsed, want 2 (cache miss)", got)
	}
}

func TestCachingProvider_Resolve(t *testing.T) {
	ref := provider.StoryRef("jira:X-1")
	story := provider.Story{Ref: ref, Title: "X", Status: "open", URL: "https://example.invalid/X-1"}

	t.Run("fresh cache hit avoids the inner call", func(t *testing.T) {
		inner := &countingProvider{story: story}
		clock := newManualClock(time.Unix(0, 0))
		c := provider.NewCachingProvider(inner, 15*time.Minute, clock)

		if _, err := c.Resolve(context.Background(), ref); err != nil {
			t.Fatalf("first Resolve error = %v, want nil", err)
		}
		clock.Advance(time.Minute)
		got, err := c.Resolve(context.Background(), ref)
		if err != nil {
			t.Fatalf("second Resolve error = %v, want nil", err)
		}
		if got != story {
			t.Fatalf("second Resolve = %+v, want %+v", got, story)
		}
		if inner.callCount() != 1 {
			t.Fatalf("inner call count = %d, want 1 (served from cache)", inner.callCount())
		}
	})

	t.Run("TTL expiry triggers a re-fetch", func(t *testing.T) {
		inner := &countingProvider{story: story}
		clock := newManualClock(time.Unix(0, 0))
		c := provider.NewCachingProvider(inner, 15*time.Minute, clock)

		if _, err := c.Resolve(context.Background(), ref); err != nil {
			t.Fatalf("first Resolve error = %v, want nil", err)
		}
		clock.Advance(16 * time.Minute)
		if _, err := c.Resolve(context.Background(), ref); err != nil {
			t.Fatalf("second Resolve error = %v, want nil", err)
		}
		if inner.callCount() != 2 {
			t.Fatalf("inner call count = %d, want 2 (TTL expired, re-fetched)", inner.callCount())
		}
	})

	t.Run("underlying failure with a cached entry serves stale, no error", func(t *testing.T) {
		inner := &countingProvider{story: story}
		clock := newManualClock(time.Unix(0, 0))
		c := provider.NewCachingProvider(inner, 15*time.Minute, clock)

		if _, err := c.Resolve(context.Background(), ref); err != nil {
			t.Fatalf("first Resolve error = %v, want nil", err)
		}
		clock.Advance(16 * time.Minute)
		inner.failWith = fmt.Errorf("adapter down: %w", provider.ErrUnavailable)

		got, err := c.Resolve(context.Background(), ref)
		if err != nil {
			t.Fatalf("Resolve error = %v, want nil (degrade to stale)", err)
		}
		if got != story {
			t.Fatalf("Resolve = %+v, want stale %+v", got, story)
		}
	})

	t.Run("underlying failure with no cached entry surfaces the typed error", func(t *testing.T) {
		wantErr := fmt.Errorf("adapter down: %w", provider.ErrUnavailable)
		inner := &countingProvider{failWith: wantErr}
		clock := newManualClock(time.Unix(0, 0))
		c := provider.NewCachingProvider(inner, 15*time.Minute, clock)

		_, err := c.Resolve(context.Background(), ref)
		if err == nil {
			t.Fatalf("Resolve error = nil, want %v", wantErr)
		}
		if !errors.Is(err, provider.ErrUnavailable) {
			t.Fatalf("Resolve error = %v, want errors.Is(err, provider.ErrUnavailable)", err)
		}
	})

	t.Run("not-found with no cached entry surfaces ErrNotFound", func(t *testing.T) {
		inner := &countingProvider{failWith: fmt.Errorf("no such issue: %w", provider.ErrNotFound)}
		clock := newManualClock(time.Unix(0, 0))
		c := provider.NewCachingProvider(inner, 15*time.Minute, clock)

		_, err := c.Resolve(context.Background(), ref)
		if !errors.Is(err, provider.ErrNotFound) {
			t.Fatalf("Resolve error = %v, want errors.Is(err, provider.ErrNotFound)", err)
		}
	})
}

func TestCachingProvider_PublishRollupPassesThroughUncached(t *testing.T) {
	inner := &countingProvider{}
	clock := newManualClock(time.Unix(0, 0))
	c := provider.NewCachingProvider(inner, 15*time.Minute, clock)

	roll := provider.Rollup{Story: "jira:X-1", Ref: "spec/x", Commit: "c1", Eligible: false}
	if err := c.PublishRollup(context.Background(), roll); err != nil {
		t.Fatalf("PublishRollup error = %v, want nil", err)
	}
}
