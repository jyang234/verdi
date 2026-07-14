package diagramverify

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/upstream"
)

// TestStaleBase_MatchingDigest_NoStaleness is obligation ac-4--behavioral
// case (1): derived_from.digest computed from the same canned truth graph
// the test regenerates reports no staleness.
func TestStaleBase_MatchingDigest_NoStaleness(t *testing.T) {
	f := upstream.NewFakeRunner()
	f.Enqueue("flowmap", "graph", upstream.Result{Stdout: readCanned(t, "graph.json"), ExitCode: 0})

	g, err := upstream.DecodeGraph(readCanned(t, "graph.json"))
	if err != nil {
		t.Fatalf("DecodeGraph: %v", err)
	}
	matchingDigest, err := canonjson.Digest(g)
	if err != nil {
		t.Fatalf("canonjson.Digest: %v", err)
	}

	stale, current, err := StaleBase(context.Background(), f, "testdata/svcfix", "deadbeef", "", matchingDigest)
	if err != nil {
		t.Fatalf("StaleBase: %v", err)
	}
	if stale {
		t.Errorf("stale = true, want false (digests match)")
	}
	if current != matchingDigest {
		t.Errorf("currentDigest = %q, want %q", current, matchingDigest)
	}
}

// TestStaleBase_MismatchedDigest_Stale is obligation ac-4--behavioral case
// (2): a deliberately different (fixed wrong sha256) digest reports
// stale-base.
func TestStaleBase_MismatchedDigest_Stale(t *testing.T) {
	f := upstream.NewFakeRunner()
	f.Enqueue("flowmap", "graph", upstream.Result{Stdout: readCanned(t, "graph.json"), ExitCode: 0})

	const wrongDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"

	stale, _, err := StaleBase(context.Background(), f, "testdata/svcfix", "deadbeef", "", wrongDigest)
	if err != nil {
		t.Fatalf("StaleBase: %v", err)
	}
	if !stale {
		t.Errorf("stale = false, want true (digests differ)")
	}
}

// TestStaleBase_IndependentOfThreeWayComparison is obligation
// ac-4--behavioral case (3): stale-base's outcome does not depend on, and
// is not conflated with, Compare's own three-way result — demonstrated
// here by pairing each StaleBase outcome with both an empty-residual
// (all-exists) and a non-empty-residual (kept-but-gone present) Compare
// call and asserting neither influences the other.
func TestStaleBase_IndependentOfThreeWayComparison(t *testing.T) {
	g, err := upstream.DecodeGraph(readCanned(t, "graph.json"))
	if err != nil {
		t.Fatalf("DecodeGraph: %v", err)
	}
	matchingDigest, err := canonjson.Digest(g)
	if err != nil {
		t.Fatalf("canonjson.Digest: %v", err)
	}
	const wrongDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"

	truth := TruthShortNames(g)
	var anyTruthName string
	for name := range truth {
		anyTruthName = name
		break
	}
	if anyTruthName == "" {
		t.Fatal("canned graph has no unambiguous truth names to test with")
	}

	cases := []struct {
		name           string
		baseDigest     string
		wantStale      bool
		proposal, base []string
		wantResidual   bool
	}{
		{
			name:         "matching digest, empty residual (all exists)",
			baseDigest:   matchingDigest,
			wantStale:    false,
			proposal:     []string{anyTruthName},
			base:         []string{anyTruthName},
			wantResidual: false,
		},
		{
			name:         "matching digest, non-empty residual (kept-but-gone present)",
			baseDigest:   matchingDigest,
			wantStale:    false,
			proposal:     []string{"LegacyStep"},
			base:         []string{"LegacyStep"},
			wantResidual: true,
		},
		{
			name:         "mismatched digest, empty residual (all exists)",
			baseDigest:   wrongDigest,
			wantStale:    true,
			proposal:     []string{anyTruthName},
			base:         []string{anyTruthName},
			wantResidual: false,
		},
		{
			name:         "mismatched digest, non-empty residual (kept-but-gone present)",
			baseDigest:   wrongDigest,
			wantStale:    true,
			proposal:     []string{"LegacyStep"},
			base:         []string{"LegacyStep"},
			wantResidual: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := upstream.NewFakeRunner()
			f.Enqueue("flowmap", "graph", upstream.Result{Stdout: readCanned(t, "graph.json"), ExitCode: 0})

			// StaleBase never looks at Compare's results (and vice versa,
			// Compare below never looks at the digest) — the two checks
			// are wired independently, which is exactly the claim under
			// test: every (stale, residual) quadrant in this table is
			// reachable.
			stale, _, err := StaleBase(context.Background(), f, "testdata/svcfix", "deadbeef", "", tc.baseDigest)
			if err != nil {
				t.Fatalf("StaleBase: %v", err)
			}
			if stale != tc.wantStale {
				t.Errorf("stale = %v, want %v", stale, tc.wantStale)
			}

			results := Compare(tc.proposal, tc.base, truth)
			hasResidual := false
			for _, r := range results {
				if r.Classification != Exists {
					hasResidual = true
				}
			}
			if hasResidual != tc.wantResidual {
				t.Errorf("hasResidual = %v, want %v (results: %+v)", hasResidual, tc.wantResidual, results)
			}
		})
	}
}

// TestStaleBase_Negative_OperationalError proves a flowmap exec failure
// surfaces as an error rather than a silently-wrong "not stale".
func TestStaleBase_Negative_OperationalError(t *testing.T) {
	f := upstream.NewFakeRunner()
	f.Enqueue("flowmap", "graph", upstream.Result{Stderr: []byte("bad flag"), ExitCode: 2})

	if _, _, err := StaleBase(context.Background(), f, "testdata/svcfix", "deadbeef", "", "sha256:deadbeef"); err == nil {
		t.Fatal("StaleBase with exit 2: want error, got nil")
	}
}
