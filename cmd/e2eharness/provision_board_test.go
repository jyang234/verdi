package main

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/wallbadge"
)

// TestBadgeSpecDecodes proves every wall-badge fixture instance the
// harness provisions is a VALID spec document (spec/badge-computes ac-5's
// fixtures must reach a live, renderable board — a decode failure would
// 500 the board before any badge could compute): the draft instances and
// the frozen sealed instance all strict-decode and validate, and the
// badge-triggering state (the dangling stub ref, the dangling decision
// link, the dangling top-level link) is present in each.
func TestBadgeSpecDecodes(t *testing.T) {
	cases := []struct {
		name       string
		spec       string
		status     string
		frozenLine string
	}{
		{"authoring draft", badgeWallSpecName, "draft", ""},
		{"review draft", badgeReviewSpecName, "draft", ""},
		{"sealed", badgeSealedSpecName, "accepted-pending-build", "frozen: { at: 2024-01-01, commit: 0123456789abcdef0123456789abcdef01234567 }\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := badgeSpec(tc.spec, tc.status, tc.frozenLine)
			fmBytes, _, err := artifact.SplitFrontmatter([]byte(doc))
			if err != nil {
				t.Fatalf("SplitFrontmatter: %v", err)
			}
			fm, err := artifact.DecodeSpec(fmBytes)
			if err != nil {
				t.Fatalf("DecodeSpec: %v", err)
			}
			if fm.ID != "spec/"+tc.spec || string(fm.Status) != tc.status {
				t.Errorf("decoded id/status = %q/%q, want spec/%s / %s", fm.ID, fm.Status, tc.spec, tc.status)
			}
			if len(fm.Stubs) != 1 || fm.Stubs[0].Slug != "badge-orphan" || fm.Stubs[0].AcceptanceCriteria[0] != "ac-99" {
				t.Errorf("stubs = %+v, want the dangling badge-orphan → ac-99 fixture", fm.Stubs)
			}
			if len(fm.Links) != 1 || fm.Links[0].Ref != "spec/no-such-parent" {
				t.Errorf("links = %+v, want the dangling spec/no-such-parent depends-on", fm.Links)
			}
			if len(fm.Decisions) != 1 || len(fm.Decisions[0].Links) != 1 || fm.Decisions[0].Links[0].Ref != "adr/0099-no-such-adr" {
				t.Errorf("decisions = %+v, want dc-1 carrying the dangling exempts link", fm.Decisions)
			}
		})
	}
}

// TestACCountSpecDecodesAndStraddlesTheThreshold proves the size-smell
// fixture pair (spec/case-file-flags ac-2/ac-3) is valid and HONEST:
// both instances strict-decode with exactly their declared AC counts,
// and the counts genuinely straddle dc-1's threshold as computed from
// the SAME declared constants the compute reads — so a future amendment
// of the layout geometry or the reference constant fails here, loudly,
// instead of silently hollowing out the Playwright proof.
func TestACCountSpecDecodesAndStraddlesTheThreshold(t *testing.T) {
	cases := []struct {
		spec  string
		count int
		over  bool
	}{
		{sizeSmellWallSpecName, sizeSmellACCount, true},
		{sizeFitWallSpecName, sizeFitACCount, false},
	}
	for _, tc := range cases {
		t.Run(tc.spec, func(t *testing.T) {
			doc := acCountSpec(tc.spec, tc.count)
			fmBytes, _, err := artifact.SplitFrontmatter([]byte(doc))
			if err != nil {
				t.Fatalf("SplitFrontmatter: %v", err)
			}
			fm, err := artifact.DecodeSpec(fmBytes)
			if err != nil {
				t.Fatalf("DecodeSpec: %v", err)
			}
			if fm.ID != "spec/"+tc.spec || len(fm.AcceptanceCriteria) != tc.count {
				t.Errorf("decoded id/AC count = %q/%d, want spec/%s with %d ACs", fm.ID, len(fm.AcceptanceCriteria), tc.spec, tc.count)
			}
			estimate := boardlayout.ZoneOriginY + tc.count*boardlayout.RowPitch
			if got := estimate > wallbadge.ReferenceViewportHeight; got != tc.over {
				t.Errorf("dc-1 estimate for %d ACs = %d vs reference %d: over=%v, want %v — the fixture no longer straddles the threshold", tc.count, estimate, wallbadge.ReferenceViewportHeight, got, tc.over)
			}
		})
	}
}

// TestSweepFixturesDecode proves the judged-sweep fixtures the harness
// provisions (spec/derivation-drawer ac-3) are VALID artifacts: both spec
// revisions strict-decode as feature drafts declaring dc-1/dc-2, and the
// report — fresh-shaped and partial-shaped alike — strict-decodes through
// the same artifact.DecodeDecisionConflict the wall itself uses, carrying
// one dispositioned and one undispositioned judged finding plus the
// sweep_provenance block.
func TestSweepFixturesDecode(t *testing.T) {
	for _, outcome := range []string{sweepOutcomeV1, sweepOutcomeStaleV2} {
		fmBytes, _, err := artifact.SplitFrontmatter([]byte(sweepSpec(sweepFreshSpecName, outcome)))
		if err != nil {
			t.Fatalf("SplitFrontmatter(%q): %v", outcome, err)
		}
		fm, err := artifact.DecodeSpec(fmBytes)
		if err != nil {
			t.Fatalf("DecodeSpec(%q): %v", outcome, err)
		}
		if len(fm.Decisions) != 2 || fm.Decisions[0].ID != "dc-1" || fm.Decisions[1].ID != "dc-2" {
			t.Errorf("decisions = %+v, want declared dc-1 and dc-2 (the comparison operand)", fm.Decisions)
		}
	}

	const sha = "0123456789abcdef0123456789abcdef01234567"
	for name, scanned := range map[string]string{
		"full":    "spec/x#dc-1, spec/x#dc-2",
		"partial": "spec/x#dc-1",
	} {
		t.Run(name, func(t *testing.T) {
			fmBytes, _, err := artifact.SplitFrontmatter([]byte(sweepReport(sha, scanned)))
			if err != nil {
				t.Fatalf("SplitFrontmatter: %v", err)
			}
			report, err := artifact.DecodeDecisionConflict(fmBytes)
			if err != nil {
				t.Fatalf("DecodeDecisionConflict: %v", err)
			}
			if report.Covers != sha {
				t.Errorf("covers = %q, want the pinned sha", report.Covers)
			}
			if len(report.Findings) != 2 || !report.Findings[0].Dispositioned() || report.Findings[1].Dispositioned() {
				t.Errorf("findings = %+v, want one dispositioned + one undispositioned", report.Findings)
			}
			if report.SweepProvenance == nil || len(report.SweepProvenance.DecisionsScanned) == 0 {
				t.Error("sweep_provenance block missing from the fixture report")
			}
		})
	}
}

// TestBadgeSpecSealedRequiresFrozen is the negative path: the sealed
// status without its frozen stamp must FAIL validation (the frozenLine
// parameter is load-bearing, not decoration).
func TestBadgeSpecSealedRequiresFrozen(t *testing.T) {
	doc := badgeSpec(badgeSealedSpecName, "accepted-pending-build", "")
	fmBytes, _, err := artifact.SplitFrontmatter([]byte(doc))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	_, err = artifact.DecodeSpec(fmBytes)
	if err == nil || !strings.Contains(err.Error(), "frozen") {
		t.Fatalf("DecodeSpec of an unfrozen sealed fixture: err = %v, want a frozen-stamp refusal", err)
	}
}
