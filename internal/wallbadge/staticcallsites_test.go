package wallbadge

import (
	"os"
	"strings"
	"testing"
)

// TestLadderStaticCallSites is badge-computes ac-3's STATIC evidence
// (co-3's "ac-4 trap", named): the spec-stale and pending-supersession
// ladder badges must be computed by the SAME exported entry points
// internal/dex/lens.go's computeLensData calls, and this package must
// contain no local reimplementation of either fold. There is no existing
// source-inspection test pattern elsewhere in this codebase to extend
// (every other "static" claim here is backed by a behavioral/differential
// test plus a doc-comment assertion) — this is deliberately the smallest
// literal witness: it reads ladder.go's own source text and asserts the
// three exact call expressions appear, and that none of the private
// fold-logic shapes those functions own (an accepted-deviation counter, an
// amended/removed-bucket membership fold) are duplicated here.
//
// This is a coarse, source-text check, not a full call-graph proof — it
// cannot catch a call routed through an intermediate wrapper function
// that itself calls the right entry point under a different local name.
// Given this package's actual shape (ladder.go calls the three functions
// directly, with no such wrapper), it is a faithful witness of what co-3
// asks for: the reader can see the exact call sites without needing runtime
// tracing.
func TestLadderStaticCallSites(t *testing.T) {
	src, err := os.ReadFile("ladder.go")
	if err != nil {
		t.Fatalf("reading ladder.go: %v", err)
	}
	text := string(src)

	for _, want := range []string{
		"decisionsweep.ScanSpecStale(",
		"evidence.PendingSupersession(",
		"evidence.ImplementsByFeature(",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("ladder.go does not call %s — badge-computes co-3 requires the exact dex-lens entry point, not a lookalike", want)
		}
	}

	// Negative: no local reimplementation of ScanSpecStale's own fold (an
	// accepted-deviation disposition counter) or PendingSupersession's own
	// fold (an amended/removed bucket membership test) anywhere under this
	// package. These are the two shapes co-3 explicitly names as the
	// defect ("a second accepted-deviation counter, ... a second open-MR
	// supersession fold").
	forbidden := []string{
		"FindingAcceptedDeviation", // the accepted-deviation disposition enum — only evidence.SpecStale's own fold should ever switch on this
		".Amended",                 // a supersession bucket field — only evidence.PendingSupersession's own fold should ever range over this
		".Removed",
	}
	for _, name := range []string{"ladder.go", "compute.go", "vlfindings.go", "record.go", "port.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		body := string(data)
		for _, bad := range forbidden {
			if strings.Contains(body, bad) {
				t.Errorf("%s contains %q — a local reimplementation marker of a fold internal/evidence already owns (co-3)", name, bad)
			}
		}
	}
}
