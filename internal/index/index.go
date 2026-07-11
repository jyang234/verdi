package index

import (
	"fmt"
	"sort"

	"github.com/OWNER/verdi/internal/store"
)

// Index is the in-memory index built by Build: every committed-zone
// artifact plus every index-minted external ref, with backlinks and a
// full-text search index built alongside. There is no persistence — a
// fresh Index is always a fresh walk (01 §Scale envelope; the store
// package's TreeHash/CacheKey are what a caller uses to decide whether a
// rebuild is worth skipping, which this package has no opinion on).
type Index struct {
	root      string
	entries   map[string]*Entry
	backlinks map[string][]Backlink
	tokens    map[string]map[string]int
}

// Build walks root's committed zone (.verdi/, minus data/) decoding every
// artifact via internal/artifact, discovers services via
// internal/store.DiscoverServices and mints their external refs, then
// builds the backlink and search indexes over the combined set.
func Build(root string) (*Index, error) {
	committed, err := walkCommittedZone(root)
	if err != nil {
		return nil, fmt.Errorf("index: Build: %w", err)
	}

	services, err := store.DiscoverServices(root)
	if err != nil {
		return nil, fmt.Errorf("index: Build: %w", err)
	}
	external, err := externalEntries(services)
	if err != nil {
		return nil, fmt.Errorf("index: Build: %w", err)
	}

	ix := &Index{
		root:      root,
		entries:   make(map[string]*Entry, len(committed)+len(external)),
		backlinks: nil,
		tokens:    make(map[string]map[string]int),
	}

	all := make([]*Entry, 0, len(committed)+len(external))
	all = append(all, committed...)
	all = append(all, external...)

	for _, e := range all {
		if prior, dup := ix.entries[e.Ref]; dup {
			return nil, fmt.Errorf("index: Build: duplicate ref %q (%s and %s)", e.Ref, prior.Path, e.Path)
		}
		ix.entries[e.Ref] = e
		ix.indexTokens(e)
	}

	ix.backlinks = buildBacklinks(all)
	return ix, nil
}

// Get returns the entry for an unpinned ref, exactly as it currently
// stands in the working tree.
func (ix *Index) Get(ref string) (*Entry, bool) {
	e, ok := ix.entries[ref]
	return e, ok
}

// Backlinks returns ref's inverted backlinks (02 §Link taxonomy), sorted
// by type then by source ref. ref need not resolve to an indexed Entry —
// a backlink can be recorded against a ref that is not itself indexed
// (e.g. a dangling or not-yet-discovered target; lint owns resolution).
func (ix *Index) Backlinks(ref string) []Backlink {
	return ix.backlinks[ref]
}

// Len returns the number of indexed entries (committed-zone + external).
func (ix *Index) Len() int {
	return len(ix.entries)
}

// All returns every indexed entry, sorted by Ref for deterministic
// iteration.
func (ix *Index) All() []*Entry {
	all := make([]*Entry, 0, len(ix.entries))
	for _, e := range ix.entries {
		all = append(all, e)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Ref < all[j].Ref })
	return all
}
