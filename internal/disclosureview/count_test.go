package disclosureview

// TestCount_MatchesEnumeration (spec/index-coverage ac-3--static): the
// count Count returns for a checkout always equals len(Current(...)) for
// that same checkout — the disclosures page's own enumeration — never a
// second, separately decided tally. Exercised over the committed
// disclosure fixtures: a store with a disclosure (buildFixtureStore's
// bare-clone VL-017 case) and a store with none (the same fixture once
// its mutable zone exists, per TestCurrent_FreshPerCall).

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/disclosure"
)

func TestCount_MatchesEnumeration(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) string
	}{
		{"a store with a disclosure", buildFixtureStore},
		{"a store with none", buildDisclosureFreeFixtureStore},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := tt.setup(t)

			items, err := Current(context.Background(), root)
			if err != nil {
				t.Fatalf("Current: %v", err)
			}
			count, err := Count(context.Background(), root)
			if err != nil {
				t.Fatalf("Count: %v", err)
			}
			if count != len(items) {
				t.Fatalf("Count = %d, want %d (len(Current(...)), the disclosures page's own enumeration)", count, len(items))
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
