package disclosureview

// TestCount_MatchesEnumeration (spec/index-coverage ac-3--static): for
// every fixture store the count equals the number of entries the
// disclosures page enumerates for the same inputs — the page computes
// fresh with Current, the count reads the index's cache with the same
// extras, and both run the one enumeration (lintDisclosures plus
// withExtras), never a separately decided tally. On a git-backed store the
// count enumerates once and a second count reads the cache; without git
// the key is uncomputable and every count enumerates afresh.
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
		name      string
		setup     func(t *testing.T) string
		extras    []disclosure.Disclosure
		wantCount int
		// wantEnumerations is how many times two successive counts run
		// the enumeration.
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

			page, err := Current(context.Background(), root, tt.extras...)
			if err != nil {
				t.Fatalf("Current (the page's enumeration): %v", err)
			}
			n := countEnumerations(t)
			for i := range 2 {
				count, err := Count(context.Background(), root, tt.extras...)
				if err != nil {
					t.Fatalf("Count %d: %v", i+1, err)
				}
				if count != len(page) || count != tt.wantCount {
					t.Fatalf("Count %d = %d, the page enumerates %d, want both %d", i+1, count, len(page), tt.wantCount)
				}
			}
			if got := n.Load(); got != tt.wantEnumerations {
				t.Fatalf("two counts enumerated %d times, want %d", got, tt.wantEnumerations)
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
