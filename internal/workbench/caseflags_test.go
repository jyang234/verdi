package workbench

// Tests for spec/case-file-flags — the case file's SURFACE CONTRACT
// (dc-3: which walls wear which stamps, their placement and register, and
// how disclosed-unproven renders), over the compute substrate
// spec/badge-computes shipped. The ladder stamps (spec-stale,
// pending-supersession) render on STORY walls, through loadBoard's one
// attachment point; the size-smell observation stamps ANY spec wall whose
// declared AC count drives dc-1's estimate over the reference constant;
// and a disclosed-unproven ladder outcome renders as a case-file
// DISCLOSURE LINE in the board's notice vocabulary — never a stamp,
// never silence (dc-4).

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/wallbadge"
)

// sizeSmellCounts mirrors the dc-1 threshold from the SAME declared
// constants the compute reads: the largest AC count at or under the
// reference viewport and the smallest count over it.
func sizeSmellCounts() (maxUnder, minOver int) {
	maxUnder = (wallbadge.ReferenceViewportHeight - boardlayout.ZoneOriginY) / boardlayout.RowPitch
	return maxUnder, maxUnder + 1
}

const caseFlagsStoryName = "widget-caseflag-story"

// caseFlagsStorySpec is a story wall with one implements edge — the
// fixture every ladder-outcome test drives (the same shape internal/
// wallbadge's own ladder tests use, rendered here through the real
// loadBoard + renderBoardRegion path).
const caseFlagsStorySpec = `---
id: spec/widget-caseflag-story
kind: spec
class: story
title: "Widget caseflag story"
status: draft
owners: [platform-team]
story: jira:WCF-1
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "does a thing", evidence: [attestation], anchor: "#ac-1" }
links:
  - { type: implements, ref: "spec/parent-feature#ac-1" }
---
# Widget caseflag story

## Problem

p

## Outcome

o

## ac-1

Prose.
`

// caseFlagsDeviationReport carries one accepted-deviation finding whose
// id equals the story's own ac-1 — ScanSpecStale's own-text trigger.
const caseFlagsDeviationReport = `---
schema: verdi.deviation/v1
covers: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3
findings:
  - { id: ac-1, kind: computed, text: "own-text drift", disposition: accepted-deviation, note: "known, deferred" }
---
# Deviation report
`

func newCaseFlagsStoryFixture(t *testing.T, extraFiles map[string]string) string {
	t.Helper()
	files := map[string]string{
		".verdi/specs/active/" + caseFlagsStoryName + "/spec.md": caseFlagsStorySpec,
		".verdi/.gitignore": "data/\n",
	}
	for k, v := range extraFiles {
		files[k] = v
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "seed caseflags story fixture"}})
	return repo.Dir
}

// fakeCandidateLoader is a hermetic wallbadge.SupersessionCandidateLoader.
type fakeCandidateLoader struct {
	candidates []evidence.OpenSupersessionCandidate
	ok         bool
}

func (f fakeCandidateLoader) LoadCandidates(ctx context.Context, featureRef, specPath string) ([]evidence.OpenSupersessionCandidate, bool, error) {
	return f.candidates, f.ok, nil
}

// renderCaseFlagsBoard loads the fixture board through the REAL load path
// (loadBoard, the one attachment point) and renders the full region — the
// same markup the page and the post-mutation fragment share.
func renderCaseFlagsBoard(t *testing.T, root, name string, loader wallbadge.SupersessionCandidateLoader) (*BoardProjection, string) {
	t.Helper()
	s := &boardSpecServer{root: root, supersession: loader}
	proj, git, _, err := s.loadBoard(context.Background(), name)
	if err != nil {
		t.Fatalf("loadBoard: %v", err)
	}
	return proj, renderBoardRegion(proj, git)
}

// TestCaseFileStamps_SpecStaleFlagged is the ac-1 obligation's outcome
// (a): a story whose deviation report crosses a spec-stale trigger wears
// the spec-stale stamp on the case-file lockup, its record naming the
// firing finding id.
func TestCaseFileStamps_SpecStaleFlagged(t *testing.T) {
	root := newCaseFlagsStoryFixture(t, map[string]string{
		".verdi/specs/active/" + caseFlagsStoryName + "/deviation-report.md": caseFlagsDeviationReport,
	})
	proj, html := renderCaseFlagsBoard(t, root, caseFlagsStoryName, nil)

	row := extractElement(t, html, `class="case-stamp-row"`)
	if !strings.Contains(row, `class="case-stamp" data-badge-source="ladder:spec-stale"`) {
		t.Errorf("case-stamp-row carries no spec-stale stamp:\n%s", row)
	}
	if !strings.Contains(row, ">spec-stale</button>") {
		t.Errorf("the stamp does not wear the dex lens's flag name spec-stale (dc-4):\n%s", row)
	}
	found := false
	for _, b := range proj.CaseFileBadges {
		if b.Source == "ladder:spec-stale" {
			found = true
			if len(b.Records) == 0 || b.Records[0] != "ac-1" {
				t.Errorf("spec-stale records = %+v, want the firing finding id ac-1 first", b.Records)
			}
		}
	}
	if !found {
		t.Fatalf("CaseFileBadges = %+v, want ladder:spec-stale", proj.CaseFileBadges)
	}
}

// TestCaseFileStamps_PendingSupersessionFlagged is outcome (b): a story
// whose implemented objects an open supersession MR touches (hermetic
// fake loader) wears the pending-supersession stamp, naming MR and
// touched object ids — and no disclosure line (proven, not unproven).
func TestCaseFileStamps_PendingSupersessionFlagged(t *testing.T) {
	root := newCaseFlagsStoryFixture(t, nil)
	loader := fakeCandidateLoader{
		ok: true,
		candidates: []evidence.OpenSupersessionCandidate{{
			MRID:   "7",
			Digest: "sha256:cccc",
			Spec:   &artifact.SpecFrontmatter{Supersession: &artifact.Supersession{Amended: []artifact.SupersessionNote{{ID: "ac-1", Note: "tightened"}}}},
		}},
	}
	proj, html := renderCaseFlagsBoard(t, root, caseFlagsStoryName, loader)

	row := extractElement(t, html, `class="case-stamp-row"`)
	if !strings.Contains(row, `class="case-stamp" data-badge-source="ladder:pending-supersession"`) {
		t.Errorf("case-stamp-row carries no pending-supersession stamp:\n%s", row)
	}
	if !strings.Contains(row, ">pending-supersession</button>") {
		t.Errorf("the stamp does not wear the dex lens's flag name pending-supersession (dc-4):\n%s", row)
	}
	if strings.Contains(html, `data-testid="case-file-disclosure"`) {
		t.Errorf("a PROVEN outcome must not also render a disclosure line:\n%s", html)
	}
	if len(proj.CaseFileDisclosures) != 0 {
		t.Errorf("CaseFileDisclosures = %+v, want none on a proven outcome", proj.CaseFileDisclosures)
	}
}

// TestCaseFileDisclosure_UnprovenIsALineNeverAStamp is outcome (c) and
// dc-4's second half: with NO forge available, pending-supersession is
// disclosed-unproven and renders as a case-file disclosure LINE in the
// board's notice vocabulary — on the case-file lockup itself, never a
// stamp (unproven never dressed as a verdict) and never silence.
func TestCaseFileDisclosure_UnprovenIsALineNeverAStamp(t *testing.T) {
	root := newCaseFlagsStoryFixture(t, nil)
	proj, html := renderCaseFlagsBoard(t, root, caseFlagsStoryName, nil)

	header := extractElement(t, html, `class="board-placards case-file"`)
	if !strings.Contains(header, `data-testid="case-file-disclosure"`) {
		t.Fatalf("case-file lockup carries no disclosure line:\n%s", header)
	}
	if !strings.Contains(header, "pending-supersession is disclosed-unproven") {
		t.Errorf("the disclosure line does not name the unproven state:\n%s", header)
	}
	// The line speaks the board's notice vocabulary (dc-4).
	if !strings.Contains(header, `class="board-notice case-disclosure"`) {
		t.Errorf("the disclosure line does not wear the board-notice vocabulary:\n%s", header)
	}
	// Never a stamp — in either direction.
	if strings.Contains(html, `data-badge-source="ladder:pending-supersession"`) {
		t.Errorf("disclosed-unproven rendered as a stamp:\n%s", html)
	}
	// The disclosure lives on the case file, not in the generic top-of-
	// board notice chrome.
	if i := strings.Index(html, "pending-supersession is disclosed-unproven"); i >= 0 {
		if j := strings.Index(html, `class="board-placards case-file"`); j < 0 || i < j {
			t.Errorf("the disclosure renders before the case-file lockup (in the generic chrome), want it on the case file itself")
		}
	}
	for _, n := range proj.Notices {
		if strings.Contains(n, "disclosed-unproven") {
			t.Errorf("Notices = %+v still carries the ladder disclosure — it belongs to CaseFileDisclosures now", proj.Notices)
		}
	}
	if len(proj.CaseFileDisclosures) != 1 {
		t.Fatalf("CaseFileDisclosures = %+v, want exactly the one ladder disclosure", proj.CaseFileDisclosures)
	}
}

// TestCaseFileStamps_UnflaggedWearsNoStamp is the obligation's fourth
// outcome: an unflagged story (no deviation report; a loader that proves
// no open MR touches it) wears NO ladder stamp and no disclosure —
// proven-unflagged is silence on the stamp row, honestly.
func TestCaseFileStamps_UnflaggedWearsNoStamp(t *testing.T) {
	root := newCaseFlagsStoryFixture(t, nil)
	loader := fakeCandidateLoader{ok: true} // enumerable, no candidates
	proj, html := renderCaseFlagsBoard(t, root, caseFlagsStoryName, loader)

	for _, source := range []string{"ladder:spec-stale", "ladder:pending-supersession"} {
		if strings.Contains(html, `data-badge-source="`+source+`"`) {
			t.Errorf("unflagged wall wears a %s stamp:\n%s", source, html)
		}
	}
	if strings.Contains(html, `data-testid="case-file-disclosure"`) {
		t.Errorf("proven-unflagged wall renders a disclosure line:\n%s", html)
	}
	if len(proj.CaseFileDisclosures) != 0 {
		t.Errorf("CaseFileDisclosures = %+v, want none", proj.CaseFileDisclosures)
	}
}

// TestCaseFileStamps_LadderNeverOnAFeatureWall is dc-3's wall-class
// routing: the ladder stamps are story-scoped computes and never render
// on a feature wall — even one carrying a deviation report and an
// implements edge (the badge-computes class gate, witnessed here at the
// rendered surface).
func TestCaseFileStamps_LadderNeverOnAFeatureWall(t *testing.T) {
	const name = "widget-caseflag-feature"
	spec := `---
id: spec/widget-caseflag-feature
kind: spec
class: feature
title: "Widget caseflag feature"
status: draft
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "does a thing", evidence: [runtime], anchor: "#ac-1" }
links:
  - { type: implements, ref: "spec/parent-feature#ac-1" }
---
# Widget caseflag feature

## Problem

p

## Outcome

o

## ac-1

Prose.
`
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{
		".verdi/specs/active/" + name + "/spec.md":             spec,
		".verdi/specs/active/" + name + "/deviation-report.md": caseFlagsDeviationReport,
		".verdi/.gitignore": "data/\n",
	}, Message: "seed caseflags feature fixture"}})

	proj, html := renderCaseFlagsBoard(t, repo.Dir, name, nil)
	for _, source := range []string{"ladder:spec-stale", "ladder:pending-supersession"} {
		if strings.Contains(html, `data-badge-source="`+source+`"`) {
			t.Errorf("feature wall wears the story-scoped %s stamp (dc-3):\n%s", source, html)
		}
	}
	if strings.Contains(html, `data-testid="case-file-disclosure"`) {
		t.Errorf("feature wall renders a ladder disclosure line (the ladder never runs there):\n%s", html)
	}
	if len(proj.CaseFileDisclosures) != 0 {
		t.Errorf("CaseFileDisclosures = %+v, want none on a feature wall", proj.CaseFileDisclosures)
	}
}

// manyACWallSpec renders a decodable spec whose declared AC count is
// exactly n — the size-smell fixtures' one variable.
func manyACWallSpec(name, class string, n int) string {
	var sb strings.Builder
	sb.WriteString("---\nid: spec/" + name + "\nkind: spec\nclass: " + class + "\ntitle: \"Sprawl\"\nstatus: draft\nowners: [platform-team]\n")
	if class == "story" {
		sb.WriteString("story: jira:SPR-9\n")
	}
	sb.WriteString("problem: { text: \"p\", anchor: \"#problem\" }\noutcome: { text: \"o\", anchor: \"#outcome\" }\nacceptance_criteria:\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&sb, "  - { id: ac-%d, text: \"does thing %d\", evidence: [runtime], anchor: \"#ac-%d\" }\n", i, i, i)
	}
	if class == "story" {
		sb.WriteString("links:\n  - { type: implements, ref: \"spec/parent-feature#ac-1\" }\n")
	}
	sb.WriteString("---\n# Sprawl\n\n## Problem\n\np\n\n## Outcome\n\no\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&sb, "\n## ac-%d\n\nProse.\n", i)
	}
	return sb.String()
}

// TestCaseFileSizeSmell_StampsAnyACDeclaringWallOverTheEstimate is ac-2
// at the rendered surface, both wall classes and both sides of the dc-1
// boundary: the smallest exceeding count raises the stamp on the case
// file; the largest fitting count raises nothing.
func TestCaseFileSizeSmell_StampsAnyACDeclaringWallOverTheEstimate(t *testing.T) {
	maxUnder, minOver := sizeSmellCounts()
	tests := map[string]struct {
		class string
		count int
		want  bool
	}{
		"feature over":  {class: "feature", count: minOver, want: true},
		"story over":    {class: "story", count: minOver, want: true},
		"feature under": {class: "feature", count: maxUnder, want: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			specName := "sprawl-" + tc.class
			repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{
				".verdi/specs/active/" + specName + "/spec.md": manyACWallSpec(specName, tc.class, tc.count),
				".verdi/.gitignore":                            "data/\n",
			}, Message: "seed size-smell fixture"}})
			_, html := renderCaseFlagsBoard(t, repo.Dir, specName, nil)

			hasStamp := strings.Contains(html, `class="case-stamp" data-badge-source="observe:size-smell"`)
			if hasStamp != tc.want {
				t.Fatalf("size-smell stamp present=%v, want %v (%d ACs on a %s wall):\n%s", hasStamp, tc.want, tc.count, tc.class, html)
			}
			if tc.want && !strings.Contains(html, ">size-smell</button>") {
				t.Errorf("the stamp does not wear its flag name size-smell:\n%s", html)
			}
		})
	}
}

// TestRenderCaseDisclosures_NoHeaderFallsBackToNotices is the surface's
// own negative path: a projection with a case-file disclosure but NO
// case-file header at all (a grandfathered wall with neither problem nor
// outcome) still renders the line — in the top notice chrome — never
// silently dropped (unproven is never silence).
func TestRenderCaseDisclosures_NoHeaderFallsBackToNotices(t *testing.T) {
	p := &BoardProjection{
		Spec:                "bare-wall",
		Mode:                modeReadOnly,
		Class:               "story",
		CaseFileDisclosures: []string{"pending-supersession is disclosed-unproven: no forge is configured to enumerate open MRs"},
	}
	html := renderBoardRegion(p, &boardGitState{})
	if strings.Contains(html, "board-placards") {
		t.Fatalf("fixture grew a case-file header; the fallback path is untested:\n%s", html)
	}
	if !strings.Contains(html, `data-testid="case-file-disclosure"`) {
		t.Fatalf("headerless wall dropped the disclosure line (silence is never a pass):\n%s", html)
	}
	if !strings.Contains(html, "disclosed-unproven") {
		t.Errorf("fallback disclosure lost its text:\n%s", html)
	}
}

// TestRenderCaseDisclosures_EscapesHostileText: the disclosure is
// document-derived text and must never inject markup.
func TestRenderCaseDisclosures_EscapesHostileText(t *testing.T) {
	p := &BoardProjection{
		Spec:                "hostile",
		Mode:                modeAuthoring,
		Class:               "story",
		Problem:             "p",
		Outcome:             "o",
		CaseFileDisclosures: []string{`"><script>alert(1)</script>`},
	}
	html := renderBoardRegion(p, &boardGitState{Branch: "design/x", DefaultBranch: "main"})
	if strings.Contains(html, "<script>") {
		t.Fatalf("hostile disclosure text reached the markup unescaped:\n%s", html)
	}
}
