package disclosureview

import (
	"context"
	"sync"
	"time"

	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/gitx"
)

// maxCachedRoots bounds a Cache: it holds at most this many roots, one
// result each (the result for the root's last key).
const maxCachedRoots = 8

// racyWindow is how recently an input may have been written for a result
// to still be stored. A write in the same timestamp granule as the stamp
// readInputs took would leave the stamp unchanged; two seconds covers the
// coarsest common granule (FAT's two-second modification time) and a
// coarse kernel clock.
const racyWindow = 2 * time.Second

// Test seams: tests wrap enumerateLint to count enumerations and shift now
// to step past the racy window. Tests that set them do not run in
// parallel and restore them with t.Cleanup.
var (
	enumerateLint = lintDisclosures
	now           = time.Now
)

// Cache memoizes Current's lint half per root under a key that covers
// every input the lint run reads (SI-295; see cachekey.go for the input
// inventory). Process extras are never cached: every call appends its own
// after the cached lint half. The zero value is ready to use and safe for
// concurrent use. Nothing is persisted and nothing expires by time; an
// entry is replaced when its root's key changes, and by every Refresh.
//
// Current serves the cached lint half (the index's count). Refresh never
// does: it enumerates on every call (the disclosures page, which the
// closed spec/disclosures-panel ac-1 requires to compute fresh per render)
// and then refreshes the cache with its result.
//
// A result is served only under a key proven complete for this call:
//   - when the key cannot be computed (not a git repository, a symbolic
//     link lint would read through, a second object store, an
//     unsupported platform, any read error), the call enumerates afresh
//     and stores nothing;
//   - a result is stored only if a second reading after the enumeration
//     matches the first (same contents, nothing written in between) and
//     no input was written within racyWindow before the first reading;
//   - an enumeration error is returned and never stored.
//
// Concurrent calls for the same root and key share one enumeration.
type Cache struct {
	mu      sync.Mutex
	entries map[string]*cacheEntry
}

// cacheEntry is one root's result for one key. items and ok are written
// before done is closed and read only after.
type cacheEntry struct {
	key   string
	done  chan struct{}
	items []disclosure.Disclosure
	ok    bool
}

// shared is the process-wide cache: the workbench's index reads it
// (Cached, through Count) and its /disclosures page refreshes it
// (Refresh).
var shared Cache

// Cached is Current served through the process-wide cache: the same
// enumeration, the same order, the same extras, recomputed whenever any
// input changes.
func Cached(ctx context.Context, root string, extras ...disclosure.Disclosure) ([]disclosure.Disclosure, error) {
	return shared.Current(ctx, root, extras...)
}

// Refresh is the process-wide cache's Refresh: Current computed fresh,
// its lint half then stored for Cached's readers.
func Refresh(ctx context.Context, root string, extras ...disclosure.Disclosure) ([]disclosure.Disclosure, error) {
	return shared.Refresh(ctx, root, extras...)
}

// Current returns what the package-level Current returns for root and
// extras, serving the lint half from the cache when its key is unchanged.
func (c *Cache) Current(ctx context.Context, root string, extras ...disclosure.Disclosure) ([]disclosure.Disclosure, error) {
	lintItems, err := c.lint(ctx, root)
	if err != nil {
		return nil, err
	}
	return withExtras(lintItems, extras), nil
}

// Refresh returns what the package-level Current returns for root and
// extras, always from a fresh enumeration, never a cached one. It then
// refreshes root's entry: the result replaces whatever the cache held and
// is stored under the same guard as a miss (a second reading with the
// same key and stamps, outside the racy window). A result the guard
// refuses, or a key that cannot be computed, leaves root with no entry,
// so a value the fresh enumeration may contradict is never served again.
func (c *Cache) Refresh(ctx context.Context, root string, extras ...disclosure.Disclosure) ([]disclosure.Disclosure, error) {
	lintItems, err := c.refresh(ctx, root)
	if err != nil {
		return nil, err
	}
	return withExtras(lintItems, extras), nil
}

func (c *Cache) refresh(ctx context.Context, root string) ([]disclosure.Disclosure, error) {
	start := now()
	before, err := readInputs(ctx, root)
	if err != nil {
		c.forget(root)
		return enumerateFresh(ctx, root)
	}
	return c.lead(ctx, root, c.replace(root, before.key), before, start)
}

// lint returns root's lint half: cached when the key is unchanged,
// otherwise enumerated afresh (and stored when the result is provably the
// key's).
func (c *Cache) lint(ctx context.Context, root string) ([]disclosure.Disclosure, error) {
	start := now()
	before, err := readInputs(ctx, root)
	if err != nil {
		return enumerateFresh(ctx, root)
	}

	e, leader := c.claim(root, before.key)
	if !leader {
		items, ok, err := e.await(ctx)
		if err != nil || ok {
			return items, err
		}
		return enumerateFresh(ctx, root)
	}

	return c.lead(ctx, root, e, before, start)
}

// lead runs e's enumeration and stores its result only when a second
// reading proves it is the key's (see Cache). However the enumeration
// ends — a result, an error or a panic — e is finished and, unless
// stored, released, so no later call waits on it.
func (c *Cache) lead(ctx context.Context, root string, e *cacheEntry, before inputs, start time.Time) (items []disclosure.Disclosure, err error) {
	defer func() {
		close(e.done)
		if !e.ok {
			c.release(root, e)
		}
	}()
	items, err = enumerateFresh(ctx, root)
	if err == nil && before.newest.Before(start.Add(-racyWindow)) {
		if after, aerr := readInputs(ctx, root); aerr == nil && after.same(before) {
			e.items, e.ok = items, true
		}
	}
	return items, err
}

// claim returns root's entry for key and whether the caller leads its
// enumeration: an existing entry with the same key (done or in flight) is
// shared; otherwise a new entry replaces root's old one.
func (c *Cache) claim(root, key string) (*cacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[root]; ok && e.key == key {
		return e, false
	}
	return c.putLocked(root, key), true
}

// replace gives root a new entry for key, which its caller leads,
// whatever root held before.
func (c *Cache) replace(root, key string) *cacheEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.putLocked(root, key)
}

// forget drops root's entry.
func (c *Cache) forget(root string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, root)
}

// putLocked installs a new in-flight entry for root, evicting another
// root's entry if the cache is full. c.mu must be held.
func (c *Cache) putLocked(root, key string) *cacheEntry {
	if c.entries == nil {
		c.entries = make(map[string]*cacheEntry)
	}
	if _, ok := c.entries[root]; !ok && len(c.entries) >= maxCachedRoots {
		for other := range c.entries {
			delete(c.entries, other)
			break
		}
	}
	e := &cacheEntry{key: key, done: make(chan struct{})}
	c.entries[root] = e
	return e
}

// await waits for e's enumeration and reports its stored result, or
// ok=false when the leader stored none. It returns ctx's error if ctx
// ends first.
func (e *cacheEntry) await(ctx context.Context) (items []disclosure.Disclosure, ok bool, err error) {
	select {
	case <-e.done:
		return e.items, e.ok, nil
	case <-ctx.Done():
		return nil, false, ctx.Err()
	}
}

// release drops e if it is still root's entry, so a result that could not
// be stored is never served.
func (c *Cache) release(root string, e *cacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries[root] == e {
		delete(c.entries, root)
	}
}

// enumerateFresh runs the lint half afresh. It first clears gitx's
// process-wide shallow memo so the run reads the repository's current
// shallow state, which the key also reads (gitx documents the reset as
// the long-lived consumer's duty).
func enumerateFresh(ctx context.Context, root string) ([]disclosure.Disclosure, error) {
	gitx.ResetShallowCache()
	return enumerateLint(ctx, root)
}
