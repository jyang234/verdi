package workbench

// Render tests for the wall-badge visual grammar (spec/badge-computes
// ac-5, dc-4): card badges as compact chips in the card's receipt-row
// vocabulary, case-file badges as stamps on the case-file lockup beside
// the class tag, every badge element a BUTTON carrying data-badge-source
// and its serialized derivation record (the derivation-drawer story's
// opener contract). Deterministic markup — server-rendered, no client
// templating — over the projection seam the backend phase delivered
// (cardView.Badges / StubView.Badges / BoardProjection.CaseFileBadges).

import (
	"encoding/json"
	stdhtml "html"
	"regexp"
	"strings"
	"testing"
)

// badgeRenderProjection is a hand-built projection carrying one badge of
// each anchor kind: an object-anchored chip on a decision card, an
// object-anchored chip on a stub card, and a spec-level case-file stamp.
func badgeRenderProjection(mode boardModeKind) *BoardProjection {
	specInput := badgeInputView{Name: "spec", Path: ".verdi/specs/active/badge-fixture/spec.md", Revision: "sha256:aabb"}
	return &BoardProjection{
		Spec:    "badge-fixture",
		Title:   "Badge fixture",
		Mode:    mode,
		Class:   "feature",
		Problem: "p",
		Outcome: "o",
		Cards: []cardView{
			{ID: "dc-1", Kind: "decision", Text: "a decision", Badges: []badgeView{{
				Source:  "lint:VL-003",
				Label:   "decisions[dc-1].links[].ref \"adr/none\" does not resolve",
				Target:  "dc-1",
				Inputs:  []badgeInputView{specInput},
				Records: []string{"decisions[dc-1].links[].ref \"adr/none\" does not resolve"},
			}}},
			{ID: "ac-1", Kind: "acceptance-criterion", Text: "bare card"},
		},
		StubViews: []StubView{{Slug: "orphan-stub", Badges: []badgeView{{
			Source:  "lint:VL-006",
			Label:   "stub \"orphan-stub\" names acceptance_criteria ac-99…",
			Target:  "stub:orphan-stub",
			Inputs:  []badgeInputView{specInput},
			Records: []string{"stub \"orphan-stub\" names acceptance_criteria ac-99, which is not a declared acceptance criterion of this spec"},
		}}}},
		CaseFileBadges: []badgeView{{
			Source:  "lint:VL-003",
			Label:   "links[].ref \"spec/none\" does not resolve",
			Inputs:  []badgeInputView{specInput},
			Records: []string{"links[].ref \"spec/none\" does not resolve"},
		}},
	}
}

// buttonRe matches one rendered badge button and captures its attributes.
var buttonRe = regexp.MustCompile(`<button type="button" class="(badge-chip|case-stamp)" data-badge-source="([^"]*)" data-badge-record="([^"]*)"`)

// TestRenderBadges_ChipsAndStamps is dc-4's happy path across every mode
// (ac-5: badges render in authoring, review, and read-only alike): chips
// inside their own card's markup, stamps on the case-file lockup beside
// the class tag, each a button carrying data-badge-source and the
// serialized derivation record.
func TestRenderBadges_ChipsAndStamps(t *testing.T) {
	for _, mode := range []boardModeKind{modeAuthoring, modeReview, modeReadOnly} {
		t.Run(string(mode), func(t *testing.T) {
			p := badgeRenderProjection(mode)
			html := renderBoardRegion(p, &boardGitState{Branch: "design/x", DefaultBranch: "main"})

			// The decision card's chip lives INSIDE that card's element.
			card := extractElement(t, html, `data-testid="card-dc-1"`)
			if !strings.Contains(card, `class="badge-chip" data-badge-source="lint:VL-003"`) {
				t.Errorf("card dc-1 carries no VL-003 badge chip:\n%s", card)
			}
			// The stub card's chip lives inside the stub card.
			stub := extractElement(t, html, `data-testid="stub-card-orphan-stub"`)
			if !strings.Contains(stub, `class="badge-chip" data-badge-source="lint:VL-006"`) {
				t.Errorf("stub card carries no VL-006 badge chip:\n%s", stub)
			}
			// A card with no badge stays bare.
			bare := extractElement(t, html, `data-testid="card-ac-1"`)
			if strings.Contains(bare, "badge-chip") {
				t.Errorf("bare card ac-1 grew a badge chip:\n%s", bare)
			}

			// The case-file stamp rides the placards lockup beside the class
			// tag: one stamp row containing BOTH the stamp button and the
			// class tag (dc-4: "stamps on the case-file lockup beside the
			// class tag").
			row := extractElement(t, html, `class="case-stamp-row"`)
			if !strings.Contains(row, `class="case-stamp" data-badge-source="lint:VL-003"`) {
				t.Errorf("case-stamp-row carries no VL-003 stamp:\n%s", row)
			}
			if !strings.Contains(row, "case-class-tag") {
				t.Errorf("case-stamp-row does not keep the class tag beside the stamps:\n%s", row)
			}

			// Every badge element is a BUTTON carrying its full serialized
			// derivation record — the drawer's opener contract, verbatim
			// (unescaping the attribute yields exactly json.Marshal of the
			// badgeView).
			matches := buttonRe.FindAllStringSubmatch(html, -1)
			if len(matches) != 3 {
				t.Fatalf("found %d badge buttons, want 3:\n%s", len(matches), html)
			}
			for _, m := range matches {
				var got badgeView
				if err := json.Unmarshal([]byte(stdhtml.UnescapeString(m[3])), &got); err != nil {
					t.Errorf("data-badge-record of %s does not round-trip as JSON: %v", m[2], err)
					continue
				}
				if got.Source != m[2] {
					t.Errorf("data-badge-record.source = %q, want the data-badge-source value %q", got.Source, m[2])
				}
				if len(got.Inputs) != 1 || got.Inputs[0].Revision != "sha256:aabb" {
					t.Errorf("data-badge-record inputs = %+v, want the pinned spec input with its revision", got.Inputs)
				}
			}
		})
	}
}

// TestRenderBadges_NoBadgesLeavesMarkupUntouched is the negative path: a
// badge-free projection renders NO chip, NO stamp, and the case-class tag
// exactly as before (never an empty receipts row, never an empty stamp
// row) — existing walls are byte-stable through this feature.
func TestRenderBadges_NoBadgesLeavesMarkupUntouched(t *testing.T) {
	p := badgeRenderProjection(modeReadOnly)
	p.Cards[0].Badges = nil
	p.StubViews[0].Badges = nil
	p.CaseFileBadges = nil
	html := renderBoardRegion(p, &boardGitState{})
	for _, forbidden := range []string{"badge-chip", "case-stamp", "card-badges"} {
		if strings.Contains(html, forbidden) {
			t.Errorf("badge-free wall still renders %q", forbidden)
		}
	}
	if !strings.Contains(html, `<span class="case-class-tag case-class-tag--feature" data-testid="case-class-tag">`) {
		t.Error("badge-free wall lost the unwrapped case-class-tag markup")
	}
}

// TestRenderBadges_EscapesHostileLabel proves the chip's label, tooltip,
// and serialized record are HTML-escaped: a finding message is untrusted
// document-derived text and must never inject markup.
func TestRenderBadges_EscapesHostileLabel(t *testing.T) {
	p := badgeRenderProjection(modeReadOnly)
	p.Cards[0].Badges[0].Label = `"><script>alert(1)</script>`
	p.Cards[0].Badges[0].Records = []string{`"><script>alert(1)</script>`}
	html := renderBoardRegion(p, &boardGitState{})
	if strings.Contains(html, "<script>") {
		t.Fatalf("hostile badge label reached the markup unescaped:\n%s", html)
	}
}

// extractElement returns the window of html from the element carrying
// marker up to the next sibling wall element (the following card, stub,
// ref card, or the canvas that follows the placards header). Cards and
// stubs are flat sibling divs, so a boundary-bounded window is a faithful
// "inside this element" scope for these assertions without an HTML parser.
func extractElement(t *testing.T, html, marker string) string {
	t.Helper()
	i := strings.Index(html, marker)
	if i < 0 {
		t.Fatalf("marker %q not found in rendered board:\n%s", marker, html)
	}
	window := html[i:]
	end := len(window)
	for _, boundary := range []string{
		`data-testid="card-`,
		`data-testid="stub-card-`,
		`class="refcard`,
		`id="board-canvas"`,
	} {
		// Search past the marker itself (it may BE one of the boundaries).
		if j := strings.Index(window[1:], boundary); j >= 0 && j+1 < end {
			end = j + 1
		}
	}
	return window[:end]
}
