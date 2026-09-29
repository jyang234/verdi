package disclosureview

// TestCount_MatchesEnumeration (spec/index-coverage ac-3--static): for
// every fixture store the count equals the number of entries the
// disclosures page enumerates — the page reads Cached, the count reads the
// same cache with the same extras — and equals a fresh Current. Where the
// cache key is computable (git-backed stores) the count adds no second
// enumeration: page and count together enumerate once. Where it is not
// (no git), every call enumerates afresh, as the cache never serves a
// value under an unprovable key.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/disclosure"
)

func TestCount_MatchesEnumeration(t *testing.T) {
	extra := disclosure.New("mcp:review-feed", "", "forge configured but unreachable")
	tests := []struct {
		name             string
		setup            func(t *testing.T) string
		extras           []disclosure.Disclosure
		wantCount        int
		wantEnumerations int64
	}{
		{"a store with a disclosure, no git", buildFixtureStore, nil, 1, 2},
		{"a store with none, no git", buildDisclosureFreeFixtureStore, nil, 0, 2},
		{"a git-backed store with two disclosures (VL-004, VL-017)", func(t *testing.T) string { return newCacheFixture(t).root }, nil, 2, 1},
		{"a git-backed store with one (mutable zone present)", func(t *testing.T) string {
			root := newCacheFixture(t).root
			if err := os.MkdirAll(filepath.Join(root, ".verdi", "data", "mutable"), 0o755); err != nil {
				t.Fatal(err)
			}
			return root
		}, nil, 1, 1},
		{"a git-backed store with a process extra", func(t *testing.T) string { return newCacheFixture(t).root }, []disclosure.Disclosure{extra}, 3, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := tt.setup(t)
			pastRacyWindow(t)
			n := countEnumerations(t)

			page, err := Cached(context.Background(), root, tt.extras...)
			if err != nil {
				t.Fatalf("Cached (the page's enumeration): %v", err)
			}
			count, err := Count(context.Background(), root, tt.extras...)
			if err != nil {
				t.Fatalf("Count: %v", err)
			}
			if got := n.Load(); got != tt.wantEnumerations {
				t.Fatalf("page then count enumerated %d times, want %d", got, tt.wantEnumerations)
			}
			items, err := Current(context.Background(), root, tt.extras...)
			if err != nil {
				t.Fatalf("Current: %v", err)
			}
			if count != len(page) || count != len(items) || count != tt.wantCount {
				t.Fatalf("Count = %d, page enumerates %d, fresh Current %d, want all %d", count, len(page), len(items), tt.wantCount)
			}
		})
	}
}

// TestCount_AppendsExtrasLikeCurrent proves Count reads the identical
// extras argument Current does — the same "calling process's own
// disclosed context" — rather than a narrower or separately-decided set.
func TestCount_AppendsExtrasLikeCurrent(t *testing.T) {
	root := buildDisclosureFreeFixtureStore(t)
	extra := disclosure.New("mcp:review-feed", "", "forge configured but unreachable")

	count, err := Count(context.Background(), root, extra)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	items, err := Current(context.Background(), root, extra)
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if count != len(items) {
		t.Fatalf("Count(with extra) = %d, want %d", count, len(items))
	}
	if count == 0 {
		t.Fatal("Count did not reflect the extra disclosure")
	}
}

// TestCount_MissingRootErrors mirrors TestCurrent_MissingRootErrors: Count
// surfaces the same operational failure Current does, never a silent 0.
func TestCount_MissingRootErrors(t *testing.T) {
	_, err := Count(context.Background(), filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("Count on a nonexistent root: want an operational error, got nil")
	}
}

// buildDisclosureFreeFixtureStore is buildFixtureStore's store once its
// mutable zone exists (TestCurrent_FreshPerCall's own construction):
// VL-017's bare-clone disclosure no longer fires, and no other disclosure
// this package's fixture triggers, so Current returns zero items.
func buildDisclosureFreeFixtureStore(t *testing.T) string {
	t.Helper()
	root := buildFixtureStore(t)
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "data", "mutable"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}
