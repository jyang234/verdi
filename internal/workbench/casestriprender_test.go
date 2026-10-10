package workbench

import (
	"encoding/json"
	stdhtml "html"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/fixturegit"
)

// The case-file strip's markup (spec/wall-strip-and-drawer-v2 ac-1, ac-2,
// dc-1; ledger SI-368 (13), (19)).

// stripChipRe matches one badge chip button in the strip and captures its
// source and its serialized derivation record.
var stripChipRe = regexp.MustCompile(`<button type="button" class="case-stamp" data-badge-source="([^"]*)" data-badge-record="([^"]*)" title="[^"]*">([^<]*)</button>`)

// stripOf cuts the rendered strip out of a region render: from its header
// to the header's close.
func stripOf(t *testing.T, html string) string {
	t.Helper()
	strip := extractElement(t, html, `class="case-strip case-file"`)
	end := strings.Index(strip, "</header>")
	if end < 0 {
		t.Fatalf("the strip never closes:\n%s", strip)
	}
	return strip[:end+len("</header>")]
}

// TestCaseFileStrip_BadgeAndFlagChips is ac-2's static producer (obligation
// ac-2--static): over fixture walls in each flag state — flagged spec-stale,
// flagged pending-supersession (proven), proven-unflagged,
// disclosed-unproven, and size-smell on a wall declaring acceptance
// criteria — the server-rendered strip gives every badge chip a button
// carrying data-badge-source and its serialized derivation record, inside
// the strip's chips under the case-file-badges hook (dc-1); the flags carry
// the badge compute layer's own sources and the dex story-lens names
// (internal/wallbadge's ladder and observe computes, through loadBoard's
// one attachment point — the compute path caseflagsstatic_test guards),
// three-valued: a flag with its witness, proven unflagged as no chip, or
// disclosed unproven as a chip in the disclosure style, never the flag
// class, with no ladder badge beside it.
func TestCaseFileStrip_BadgeAndFlagChips(t *testing.T) {
	_, minOver := sizeSmellCounts()
	type flagged struct {
		source, label string
	}
	for _, tc := range []struct {
		name          string
		render        func(t *testing.T) (*BoardProjection, string)
		wantChips     []flagged
		wantDisclosed bool
	}{
		{
			name: "spec-stale flagged, pending-supersession disclosed",
			render: func(t *testing.T) (*BoardProjection, string) {
				root := newCaseFlagsStoryFixture(t, map[string]string{
					".verdi/specs/active/" + caseFlagsStoryName + "/deviation-report.md": caseFlagsDeviationReport,
				})
				return renderCaseFlagsBoard(t, root, caseFlagsStoryName, nil)
			},
			wantChips:     []flagged{{"ladder:spec-stale", "spec-stale"}},
			wantDisclosed: true,
		},
		{
			name: "pending-supersession flagged, proven",
			render: func(t *testing.T) (*BoardProjection, string) {
				root := newCaseFlagsStoryFixture(t, nil)
				loader := fakeCandidateLoader{ok: true, candidates: []evidence.OpenSupersessionCandidate{{
					MRID: "7", Digest: "sha256:cccc",
					Spec: &artifact.SpecFrontmatter{Supersession: &artifact.Supersession{Amended: []artifact.SupersessionNote{{ID: "ac-1", Note: "tightened"}}}},
				}}}
				return renderCaseFlagsBoard(t, root, caseFlagsStoryName, loader)
			},
			wantChips: []flagged{{"ladder:pending-supersession", "pending-supersession"}},
		},
		{
			name: "proven unflagged",
			render: func(t *testing.T) (*BoardProjection, string) {
				root := newCaseFlagsStoryFixture(t, nil)
				return renderCaseFlagsBoard(t, root, caseFlagsStoryName, fakeCandidateLoader{ok: true})
			},
		},
		{
			name: "disclosed unproven",
			render: func(t *testing.T) (*BoardProjection, string) {
				root := newCaseFlagsStoryFixture(t, nil)
				return renderCaseFlagsBoard(t, root, caseFlagsStoryName, nil)
			},
			wantDisclosed: true,
		},
		{
			name: "size-smell on a wall declaring acceptance criteria",
			render: func(t *testing.T) (*BoardProjection, string) {
				repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{
					".verdi/specs/active/sprawl-strip/spec.md": manyACWallSpec("sprawl-strip", "feature", minOver),
					".verdi/.gitignore":                        "data/\n",
				}, Message: "seed size-smell strip fixture"}})
				return renderCaseFlagsBoard(t, repo.Dir, "sprawl-strip", nil)
			},
			wantChips: []flagged{{"observe:size-smell", "size-smell"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proj, html := tc.render(t)
			strip := stripOf(t, html)

			// Every badge chip on the page is inside the strip's chips, under
			// the case-file-badges hook, beside the class tag.
			all := stripChipRe.FindAllStringSubmatch(html, -1)
			inStrip := stripChipRe.FindAllStringSubmatch(strip, -1)
			if len(all) != len(inStrip) {
				t.Errorf("%d badge chips on the page, %d in the strip:\n%s", len(all), len(inStrip), html)
			}
			if len(tc.wantChips) > 0 {
				chips := extractElement(t, strip, `class="case-strip-chips"`)
				if !strings.Contains(chips, `data-testid="case-file-badges"`) || !strings.Contains(chips, `data-testid="case-class-tag"`) {
					t.Errorf("the strip's chips lack the case-file-badges hook or the class tag beside it:\n%s", chips)
				}
			}
			got := map[string]string{}
			for _, m := range inStrip {
				source, raw, label := m[1], m[2], stdhtml.UnescapeString(m[3])
				got[source] = label
				var record badgeView
				if err := json.Unmarshal([]byte(stdhtml.UnescapeString(raw)), &record); err != nil {
					t.Errorf("%s: data-badge-record does not round-trip as JSON: %v", source, err)
					continue
				}
				if record.Source != source || record.Label != label {
					t.Errorf("%s: record = {%q %q}, want the chip's own source and label %q", source, record.Source, record.Label, label)
				}
				if len(record.Inputs) == 0 || len(record.Records) == 0 {
					t.Errorf("%s: record lacks its pinned inputs or firing records: %+v", source, record)
				}
			}
			want := map[string]string{}
			for _, f := range tc.wantChips {
				want[f.source] = f.label
			}
			for source, label := range want {
				if got[source] != label {
					t.Errorf("chip %s = %q, want the compute layer's flag name %q (chips: %v)", source, got[source], label, got)
				}
			}
			// A flag the wall must not wear (a ladder or observe source outside
			// the state under test); the fixtures' own lint findings may sit
			// beside the flags as chips of their own.
			for source := range got {
				if _, ok := want[source]; !ok && (strings.HasPrefix(source, "ladder:") || strings.HasPrefix(source, "observe:")) {
					t.Errorf("unexpected flag %s on this wall (chips: %v)", source, got)
				}
			}
			// The flags are the compute layer's own badges, attached through
			// loadBoard: each chip's source is one of the projection's
			// case-file badges, and every case-file badge is a chip.
			if len(proj.CaseFileBadges) != len(inStrip) {
				t.Errorf("CaseFileBadges = %d, chips = %d", len(proj.CaseFileBadges), len(inStrip))
			}
			for _, bd := range proj.CaseFileBadges {
				if _, ok := got[bd.Source]; !ok {
					t.Errorf("case-file badge %s was computed but no chip wears it", bd.Source)
				}
			}

			// Disclosed-unproven: a chip in the disclosure style, never the
			// flag class, and never a ladder badge beside it.
			disclosure := `<span class="case-chip case-chip--disclosed case-disclosure" data-testid="case-file-disclosure" role="status"`
			if tc.wantDisclosed {
				if !strings.Contains(strip, disclosure) {
					t.Errorf("the strip carries no disclosure chip:\n%s", strip)
				}
				if !strings.Contains(strip, "disclosed-unproven [gate:pending-supersession]") {
					t.Errorf("the disclosure chip does not name the unproven state:\n%s", strip)
				}
				if strings.Contains(html, `data-badge-source="ladder:pending-supersession"`) {
					t.Errorf("disclosed-unproven also rendered as a flag:\n%s", html)
				}
				if strings.Contains(strip, `class="case-stamp" data-badge-source="gate:`) || strings.Contains(strip, `case-stamp case-chip--disclosed`) {
					t.Errorf("the disclosure wears the flag class:\n%s", strip)
				}
			} else if strings.Contains(html, `data-testid="case-file-disclosure"`) {
				t.Errorf("a proven outcome renders a disclosure chip:\n%s", html)
			}
		})
	}

	// Every board mode (ac-2: badges render in authoring, review and
	// read-only alike): the badged projection's spec-level chip is a button
	// in the strip's chips in each.
	for _, mode := range []boardModeKind{modeAuthoring, modeReview, modeReadOnly} {
		t.Run(string(mode), func(t *testing.T) {
			html := renderBoardRegion(badgeRenderProjection(mode), &boardGitState{Branch: "design/x", DefaultBranch: "main"}, testASDView())
			chips := extractElement(t, stripOf(t, html), `class="case-strip-chips"`)
			if !strings.Contains(chips, `<button type="button" class="case-stamp" data-badge-source="lint:VL-003" data-badge-record="`) {
				t.Errorf("%s: the strip's chips carry no VL-003 chip button:\n%s", mode, chips)
			}
		})
	}
}
