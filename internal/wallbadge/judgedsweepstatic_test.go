package wallbadge

import (
	"os"
	"strings"
	"testing"
)

// TestJudgedSweep_StaticEvidence is derivation-drawer ac-3's STATIC
// obligation (and ac-4's, for this compute): the judged-findings surface
// reads the report through artifact.DecodeDecisionConflict — the one
// strict decoder, never a local YAML parse — surfaces Covers,
// ADRCorpusDigest, and DecisionsScanned into the drawer content, computes
// only equality/set comparisons (dc-3: no new staleness verdict type, no
// blocking rule), and reads no clock. The same deliberately-minimal
// source-text witness this package's TestLadderStaticCallSites pattern
// established.
func TestJudgedSweep_StaticEvidence(t *testing.T) {
	data, err := os.ReadFile("judgedsweep.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)

	// The one strict decoder — and no YAML library in sight.
	if !strings.Contains(src, "artifact.DecodeDecisionConflict(") {
		t.Error("judgedsweep.go does not decode through artifact.DecodeDecisionConflict")
	}
	if strings.Contains(src, "yaml") {
		t.Error("judgedsweep.go references yaml — the report must go through the one internal/artifact decoder")
	}

	// The pinned provenance fields all surface into the record's drawer
	// content (Covers, ADRCorpusDigest, DecisionsScanned).
	for _, field := range []string{".Covers", ".ADRCorpusDigest", ".DecisionsScanned"} {
		if !strings.Contains(src, field) {
			t.Errorf("judgedsweep.go never reads %s — the drawer must surface the report's own sweep provenance", field)
		}
	}

	// Comparison, never verdict (dc-3): no staleness type or field is
	// declared anywhere in this package's compute layer, and nothing here
	// can block (no exit, no error verdict on a mismatch).
	for _, forbidden := range []string{"Stale bool", "type Stale", "IsStale", "os.Exit"} {
		if strings.Contains(src, forbidden) {
			t.Errorf("judgedsweep.go contains %q — staleness is a disclosed comparison, never a computed verdict", forbidden)
		}
	}

	// No clock on the path (ac-4): every revision cited traces to a
	// derivation-record or decoded-report field.
	for _, forbidden := range []string{"time.Now", `"time"`, ".Format("} {
		if strings.Contains(src, forbidden) {
			t.Errorf("judgedsweep.go contains %q — no drawer content path may read or format a clock", forbidden)
		}
	}
}
