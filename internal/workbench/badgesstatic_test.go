package workbench

import (
	"os"
	"strings"
	"testing"
)

// TestBadgeAttachment_StaticEvidence is badge-computes ac-1's STATIC
// obligation: badges attach in loadBoard's I/O tier, and buildProjection
// itself is untouched by any badge input (no lint/decisionsweep/evidence
// import in the pure projector). There is no pre-existing source-
// inspection test pattern in this codebase to extend (every other
// "static" claim here is a doc-comment assertion plus a behavioral test —
// see internal/wallbadge's own TestLadderStaticCallSites doc comment for
// the same note); this is the same deliberately-minimal source-text
// witness, applied to this package's own attachment point.
func TestBadgeAttachment_StaticEvidence(t *testing.T) {
	boardspec, err := os.ReadFile("boardspec.go")
	if err != nil {
		t.Fatalf("reading boardspec.go: %v", err)
	}
	src := string(boardspec)

	// attachBadges is called exactly once inside loadBoard, AFTER
	// buildProjection and attachObligations — the same I/O-enrichment
	// posture attachObligations already established.
	if n := strings.Count(src, "attachBadges("); n != 1 {
		t.Errorf("boardspec.go calls attachBadges %d times, want exactly 1 (the one attachment point, dc-1)", n)
	}
	buildIdx := strings.Index(src, "buildProjection(name, fm, bodyBytes, stored, annotations, comments, mode)")
	obligIdx := strings.Index(src, "attachObligations(proj, s.root, name, fm)")
	badgeIdx := strings.Index(src, "attachBadges(ctx, proj, s.root, name, raw, fm, s.supersession)")
	if buildIdx < 0 || obligIdx < 0 || badgeIdx < 0 {
		t.Fatalf("could not locate all three call sites in boardspec.go (buildProjection=%d, attachObligations=%d, attachBadges=%d)", buildIdx, obligIdx, badgeIdx)
	}
	if buildIdx >= obligIdx || obligIdx >= badgeIdx {
		t.Errorf("call order in loadBoard is buildProjection@%d, attachObligations@%d, attachBadges@%d — want badges attached strictly after both the pure projector and the obligation enrichment", buildIdx, obligIdx, badgeIdx)
	}

	// LoadProjection (get_board's entrypoint) reaches the same attachment
	// by calling loadBoard itself — never a parallel, second computation.
	if !strings.Contains(src, "func LoadProjection(") {
		t.Fatal("boardspec.go no longer declares LoadProjection")
	}
	loadProjIdx := strings.Index(src, "func LoadProjection(")
	loadBoardCallIdx := strings.Index(src[loadProjIdx:], "s.loadBoard(ctx, name)")
	if loadBoardCallIdx < 0 {
		t.Error("LoadProjection does not call s.loadBoard — get_board would reimplement the projection instead of sharing it")
	}

	// The pure projector's own file must never import a badge-computing
	// package: buildProjection stays a pure function of its four
	// in-memory inputs (projection.go's own doc comment).
	projection, err := os.ReadFile("projection.go")
	if err != nil {
		t.Fatalf("reading projection.go: %v", err)
	}
	for _, forbidden := range []string{
		`"github.com/jyang234/verdi/internal/lint"`,
		`"github.com/jyang234/verdi/internal/decisionsweep"`,
		`"github.com/jyang234/verdi/internal/evidence"`,
		`"github.com/jyang234/verdi/internal/wallbadge"`,
	} {
		if strings.Contains(string(projection), forbidden) {
			t.Errorf("projection.go imports %s — the pure projector must stay untouched by any badge input", forbidden)
		}
	}
}

// TestBadgePartition_NoRuleIDSwitch is badge-computes ac-2's STATIC
// obligation: the card/case-file/off-wall routing reads ONLY each
// finding's own Locus declaration plus its Path — never a switch or map
// over VL rule ids inside internal/workbench. badges.go is this
// package's entire badge-attachment surface; it must name no VL-xxx rule
// id anywhere.
func TestBadgePartition_NoRuleIDSwitch(t *testing.T) {
	data, err := os.ReadFile("badges.go")
	if err != nil {
		t.Fatalf("reading badges.go: %v", err)
	}
	src := string(data)
	for n := 1; n <= 20; n++ {
		id := ruleID(n)
		if strings.Contains(src, id) {
			t.Errorf("badges.go names %s — the wall's routing must read only Locus/Path, never an enumerated VL rule id", id)
		}
	}
}

func ruleID(n int) string {
	digits := "0123456789"
	tens, ones := n/10, n%10
	return "VL-0" + string(digits[tens]) + string(digits[ones])
}
