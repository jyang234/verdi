package specstate

import (
	"context"
	"sync"
)

// corpusCacheLimit is how many successor corpora one corpusCache keeps. A
// store has one default-branch commit at a time, so one entry per root is
// the steady state; the headroom covers a default branch that moves while
// a process runs and a process that works over a few roots in turn. The
// limit bounds the whole cache, not each root, so a long-lived `verdi
// serve` holds at most this many corpora however many roots or commits it
// resolves over its lifetime.
const corpusCacheLimit = 4

// corpusKey names one successor corpus: the store root it was read from
// and the default-branch commit it was read at. commit is always a
// resolved commit id, never a ref name, so a default branch that moves
// resolves to a new key and is scanned again.
type corpusKey struct {
	root   string
	commit string
}

// corpusEntry is one cached corpus under its key.
type corpusEntry struct {
	key    corpusKey
	corpus *successorCorpus
}

// corpusCache memoizes successfully scanned successor corpora by
// corpusKey, so the one-spec Resolve calls a whole-store operation makes
// (dex build, lint) read a default-branch commit's corpus once instead of
// once per spec. A corpus depends only on the Git objects at its commit —
// scanSuccessors reads the tree and blobs at that commit and never the
// working tree, and its decode is a pure function of those bytes — so a
// cached corpus is what a fresh scan under the same key would build.
//
// A failed scan is never stored: it leaves no entry, and the next get for
// that key scans again. Concurrent gets for a missing key share one scan:
// the first caller runs it and the others wait, then read the stored entry
// or, when that scan failed, run their own. Construct with newCorpusCache;
// the zero value has no flight map and no limit.
type corpusCache struct {
	limit int

	mu      sync.Mutex
	entries []corpusEntry               // most recently used first, at most limit long
	flights map[corpusKey]chan struct{} // scans in progress; each channel closes when its scan ends
}

// newCorpusCache returns an empty cache that keeps at most limit corpora.
func newCorpusCache(limit int) *corpusCache {
	return &corpusCache{limit: limit, flights: map[corpusKey]chan struct{}{}}
}

// get returns key's corpus. It calls scan only when key has no stored
// entry and no other caller is scanning it, and stores what scan returns
// only when scan succeeds. A caller whose ctx ends while it waits on
// another caller's scan stops waiting and runs scan itself, uncached, so
// it gets the result its own context produces.
func (c *corpusCache) get(ctx context.Context, key corpusKey, scan func() (*successorCorpus, error)) (*successorCorpus, error) {
	for {
		c.mu.Lock()
		if corpus, ok := c.lookupLocked(key); ok {
			c.mu.Unlock()
			return corpus, nil
		}
		flight, scanning := c.flights[key]
		if !scanning {
			flight = make(chan struct{})
			c.flights[key] = flight
			c.mu.Unlock()
			return c.lead(key, flight, scan)
		}
		c.mu.Unlock()

		select {
		case <-flight:
			// That scan ended. Loop: it either stored an entry, or it
			// failed and stored nothing, in which case this caller leads.
		case <-ctx.Done():
			return scan()
		}
	}
}

// lead runs scan as key's one in-flight scan, stores the corpus when scan
// succeeds, and releases the waiters. The deferred release also runs when
// scan panics: the flight is cleared, nothing is stored, and the waiters
// go on to scan for themselves.
func (c *corpusCache) lead(key corpusKey, flight chan struct{}, scan func() (*successorCorpus, error)) (corpus *successorCorpus, err error) {
	defer func() {
		c.mu.Lock()
		delete(c.flights, key)
		if err == nil && corpus != nil {
			c.storeLocked(key, corpus)
		}
		c.mu.Unlock()
		close(flight)
	}()
	return scan()
}

// lookupLocked returns key's stored corpus and marks it most recently
// used. The caller holds c.mu.
func (c *corpusCache) lookupLocked(key corpusKey) (*successorCorpus, bool) {
	for i, e := range c.entries {
		if e.key == key {
			copy(c.entries[1:i+1], c.entries[:i])
			c.entries[0] = e
			return e.corpus, true
		}
	}
	return nil, false
}

// storeLocked records corpus under key as the most recently used entry and
// drops the least recently used entries beyond the limit. It builds a new
// slice, so a dropped corpus is not kept alive by the old backing array.
// The caller holds c.mu.
func (c *corpusCache) storeLocked(key corpusKey, corpus *successorCorpus) {
	next := make([]corpusEntry, 0, c.limit)
	next = append(next, corpusEntry{key: key, corpus: corpus})
	for _, e := range c.entries {
		if len(next) >= c.limit {
			break
		}
		if e.key != key {
			next = append(next, e)
		}
	}
	c.entries = next
}
