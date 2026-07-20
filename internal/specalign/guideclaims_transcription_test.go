// guideclaims_transcription_test.go is spec/guide-claims-gate ac-1's
// TRANSCRIPTION-FIDELITY gate: it proves docs/guide-claims.yaml faithfully
// transcribes the Integration & Startup Guide's Appendix B table — the
// manifest drops no status sub-claim the guide asserts, and never claims
// MORE (a stronger status) than the guide grants. It parses the REAL guide
// (docs/design/concepts/2026-07-17-integration-startup-guide.md) found via
// the workspace walk-up (guideClaimsWorkspaceRoot), so in the worktree
// layout it RUNS against the live guide; a bare verdi checkout SKIPS loudly
// (surfaced by the spec-align target), never a silent pass.
//
// This is FIDELITY of the Appendix B TABLE, distinct from the disclosed
// ac-4 residual: guide-to-row completeness over the guide's freeform PROSE
// (every capability the prose claims has a row at all) still needs the
// harder Task-18 set-equality check and is NOT proven here. What IS proven:
// for every Appendix B section, the set of statuses the guide's table
// bolds equals the set of statuses the manifest's rows for that section
// carry (judged-ac1-dropped-secondary-status-subclaims), and the two
// EXISTS rows the guide qualifies keep their qualifier verbatim as caveat
// text (judged-ac1-exists-qualifier-dropped).
package specalign

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// guideRelPath is the Integration & Startup Guide's path under the
// workspace root (a sibling tree of docs/design/plans, the walk-up marker).
const guideRelPath = "docs/design/concepts/2026-07-17-integration-startup-guide.md"

// guideClaimStatusTokens are the three bold status tokens Appendix B uses.
// extractBoldStatuses matches ONLY the bold-delimited form (**EXISTS**), so
// a plain-text "EXISTS" inside an italic parenthetical (e.g. 5.2's
// "*(... verdi.model/v1 EXISTS ...)*") is correctly NOT counted as a
// second asserted status.
var guideClaimStatusTokens = []string{"EXISTS", "PARTIAL", "INVENTED"}

// adjudicatedSubRows is ac-1's EXPECTED SUB-ROW TABLE: for each of the four
// Appendix B sections whose single bundled-status prose row the manifest
// intentionally decomposes (the Task-0 design wave's adjudication, ledger
// L-N4: "Appendix B's prose rows become display groupings"), the EXACT set of
// atomic sub-row ids that decomposition produced. These sections carry
// DIFFERENT statuses than the guide's single bundled row by design — 8.4's
// bundled row becomes two EXISTS sub-rows + an INVENTED sub-row, 5.3's
// likewise — so per-section STATUS set-equality legitimately cannot hold and
// TestGuideClaimsTranscriptionFidelity_AppendixB still exempts them from THAT
// check.
//
// What the old code did NOT do — and this table now does — is pin the
// decomposition mechanically. The prior blanket exemption skipped these
// sections in BOTH directions, so the decomposition survived only as authored
// YAML plus a header comment: deleting 8.4-waive-verb, 5.3-form-field-
// generation, or every 7.2 row at once still greened the fidelity gate,
// silently re-opening the dropped-sub-claim gap for exactly the sections the
// adjudication decomposed (judged-ac1-adjudicated-sections-exempt-from-
// fidelity-both-directions). findAdjudicatedSubRowMismatches enforces id-set
// EQUALITY against this table, so a missing OR extra sub-row now reds naming
// the section and the id (TestGuideClaimsAdjudicatedSubRows_PinnedAgainst-
// Manifest, a decode-only check with teeth even in a bare clone).
//
// The pin is by ID, deliberately narrower than status set-equality: it proves
// the decomposition's SHAPE (which atomic rows exist), while each row's own
// status/caveat/cite/witness bindings are proven by the decode rules and the
// ac-2/ac-3 gates. To change a section's decomposition, an author must edit
// this table AND the manifest together — the adjudication is no longer the
// least-protected layer.
var adjudicatedSubRows = map[string][]string{
	"5.3": {
		"5.3-user-editable-templates",
		"5.3-template-contract",
		"5.3-custom-namespace",
		"5.3-form-field-generation",
	},
	"6.2": {
		"6.2-stubs",
		"6.2-board-stub-instantiate",
		"6.2-closure-reconciliation",
		"6.2-from-stub-cli",
	},
	"7.2": {
		"7.2-ci-evidence-bundles",
		"7.2-verdi-sync",
		"7.2-authoritative-vs-advisory",
		"7.2-fold",
		"7.2-matrix",
		"7.2-obligation-wall-and-receipts",
	},
	"8.4": {
		"8.4-waived-status-and-kind",
		"8.4-reaffirmations-kind",
		"8.4-waive-verb",
	},
}

// findAdjudicatedSubRowMismatches enforces ac-1's EXPECTED SUB-ROW TABLE: for
// each adjudicated section, the set of manifest row ids in that section must
// EQUAL the pinned set in expected. A pinned id MISSING from the manifest is a
// silently dropped decomposed sub-claim; a manifest id in an adjudicated
// section the table does NOT pin is an unadjudicated sub-row that appeared.
// Both red, naming the section and the offending id (judged-ac1-adjudicated-
// sections-exempt-from-fidelity-both-directions). Deterministic order.
func findAdjudicatedSubRowMismatches(expected map[string][]string, m *artifact.GuideClaimsManifest) []string {
	actual := map[string]map[string]bool{}
	for _, r := range m.Rows {
		if _, pinned := expected[r.Section]; !pinned {
			continue
		}
		if actual[r.Section] == nil {
			actual[r.Section] = map[string]bool{}
		}
		actual[r.Section][r.ID] = true
	}

	sections := make([]string, 0, len(expected))
	for s := range expected {
		sections = append(sections, s)
	}
	sort.Strings(sections)

	var findings []string
	for _, s := range sections {
		wantIDs := append([]string(nil), expected[s]...)
		sort.Strings(wantIDs)
		want := make(map[string]bool, len(wantIDs))
		for _, id := range wantIDs {
			want[id] = true
		}
		got := actual[s]
		for _, id := range wantIDs {
			if !got[id] {
				findings = append(findings, fmt.Sprintf("section %s: adjudicated sub-row %q is pinned by ac-1's EXPECTED SUB-ROW TABLE but is MISSING from the manifest — deleting a decomposed sub-row silently re-opens the dropped-sub-claim gap the blanket exemption once left open (judged-ac1-adjudicated-sections-exempt-from-fidelity-both-directions)", s, id))
			}
		}
		gotIDs := make([]string, 0, len(got))
		for id := range got {
			gotIDs = append(gotIDs, id)
		}
		sort.Strings(gotIDs)
		for _, id := range gotIDs {
			if !want[id] {
				findings = append(findings, fmt.Sprintf("section %s: manifest row %q sits in an adjudicated section but is NOT pinned by ac-1's EXPECTED SUB-ROW TABLE — an unadjudicated sub-row appeared; pin it in the table (with adjudication) or remove it (judged-ac1-adjudicated-sections-exempt-from-fidelity-both-directions)", s, id))
			}
		}
	}
	return findings
}

// isTableRuleCell reports whether a table cell is a markdown separator/rule
// cell (only '-' and ':'), so the header separator row `|---|---|---|` is
// skipped during parsing.
func isTableRuleCell(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, r := range s {
		if r != '-' && r != ':' {
			return false
		}
	}
	return true
}

// extractBoldStatuses returns the set of bold status tokens (**EXISTS** /
// **PARTIAL** / **INVENTED**) present in a guide status cell.
func extractBoldStatuses(cell string) map[string]bool {
	out := map[string]bool{}
	for _, s := range guideClaimStatusTokens {
		if strings.Contains(cell, "**"+s+"**") {
			out[s] = true
		}
	}
	return out
}

// parseAppendixBStatuses parses the guide's "## Appendix B" markdown table
// and returns, per section id (the first column), the set of bold statuses
// the "Status today" column (the third column) asserts. Multiple guide rows
// sharing a section id (e.g. 6.1's two rows) are unioned. Returns nil if no
// "## Appendix B" heading is found at all (a signal the guide moved or its
// format changed, which the caller treats as a hard error, not a skip).
func parseAppendixBStatuses(guide string) map[string]map[string]bool {
	lines := strings.Split(guide, "\n")
	start := -1
	for i, ln := range lines {
		if strings.HasPrefix(ln, "## Appendix B") {
			start = i
			break
		}
	}
	if start < 0 {
		return nil
	}
	out := map[string]map[string]bool{}
	inTable := false
	for i := start + 1; i < len(lines); i++ {
		ln := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(ln, "|") {
			if inTable {
				break // the Appendix B table ended
			}
			continue // still scanning down to the table
		}
		inTable = true
		fields := strings.Split(ln, "|")
		// "| a | b | c |" splits to ["", " a ", " b ", " c ", ""].
		if len(fields) < 5 {
			continue
		}
		section := strings.TrimSpace(fields[1])
		if section == "" || section == "§" || isTableRuleCell(section) {
			continue // header / separator row
		}
		statuses := extractBoldStatuses(fields[3])
		if len(statuses) == 0 {
			continue
		}
		if out[section] == nil {
			out[section] = map[string]bool{}
		}
		for s := range statuses {
			out[section][s] = true
		}
	}
	return out
}

// TestParseAppendixBStatuses proves the parser hermetically: a synthetic
// table with a mixed-status row, a plain-text status word inside a
// parenthetical (which must NOT count), a header, and a separator.
func TestParseAppendixBStatuses(t *testing.T) {
	guide := strings.Join([]string{
		"# Guide",
		"",
		"## Appendix B — Honesty ledger",
		"",
		"| § | Capability | Status today |",
		"|---|---|---|",
		"| 8.2 | Stub reconciliation | **EXISTS**; as a *declarable* obligation **INVENTED** |",
		"| 5.2 | model.yaml | **PARTIAL** *(Phase 1: verdi.model/v1 EXISTS; obligations remain INVENTED)* |",
		"| 15 | Presets | **INVENTED** |",
		"",
		"Prose after the table with a stray **EXISTS** that must be ignored.",
		"",
		"## Appendix C",
		"| Key | Values | Source |",
		"|---|---|---|",
		"| forge | github | kernel |",
	}, "\n")

	got := parseAppendixBStatuses(guide)
	if got == nil {
		t.Fatal("parser returned nil — Appendix B heading not found")
	}
	// 8.2 asserts two statuses.
	if !got["8.2"]["EXISTS"] || !got["8.2"]["INVENTED"] || len(got["8.2"]) != 2 {
		t.Errorf("section 8.2 statuses = %v, want {EXISTS, INVENTED}", got["8.2"])
	}
	// 5.2's parenthetical EXISTS/INVENTED are plain text, not bold: only PARTIAL counts.
	if !got["5.2"]["PARTIAL"] || len(got["5.2"]) != 1 {
		t.Errorf("section 5.2 statuses = %v, want {PARTIAL} (plain-text status words must not count)", got["5.2"])
	}
	if !got["15"]["INVENTED"] || len(got["15"]) != 1 {
		t.Errorf("section 15 statuses = %v, want {INVENTED}", got["15"])
	}
	// The Appendix C table must not bleed in.
	if _, ok := got["forge"]; ok {
		t.Error("parser bled past Appendix B into a later table")
	}
}

// TestGuideClaimsTranscriptionFidelity_AppendixB is ac-1's transcription
// fidelity gate (judged-ac1-dropped-secondary-status-subclaims): for every
// Appendix B section outside the four adjudicated decompositions, the set
// of statuses the guide's table asserts equals the set of statuses the
// manifest carries for that section. A guide status with no matching
// manifest row is a DROPPED sub-claim; a manifest status the guide does not
// assert is the manifest claiming MORE than the guide.
func TestGuideClaimsTranscriptionFidelity_AppendixB(t *testing.T) {
	root, ok := guideClaimsWorkspaceRoot(verdiRepoRoot)
	if !ok {
		t.Skipf("DISCLOSURE: no workspace marker docs/design/plans found within %d levels above %s — the Integration & Startup Guide is out-of-repo and cannot be read in this layout. This is a SKIP, not a pass: a green run here is NOT proof the manifest transcribes Appendix B faithfully.", guideClaimsWorkspaceWalkLimit, verdiRepoRoot)
	}
	guidePath := filepath.Join(root, filepath.FromSlash(guideRelPath))
	data, err := os.ReadFile(guidePath)
	if err != nil {
		t.Skipf("DISCLOSURE: guide %s unreadable (%v) — cannot verify Appendix B transcription fidelity. SKIP, not a pass.", guidePath, err)
	}

	guideBySection := parseAppendixBStatuses(string(data))
	if len(guideBySection) == 0 {
		t.Fatalf("parsed zero Appendix B rows from %s — the guide moved or its Appendix B format changed; the fidelity gate cannot silently pass on an unparseable guide", guidePath)
	}

	m := decodeRealGuideClaims(t, verdiRepoRoot)
	manifestBySection := map[string]map[string]bool{}
	for _, r := range m.Rows {
		if manifestBySection[r.Section] == nil {
			manifestBySection[r.Section] = map[string]bool{}
		}
		manifestBySection[r.Section][string(r.Status)] = true
	}

	sections := map[string]bool{}
	for s := range guideBySection {
		sections[s] = true
	}
	for s := range manifestBySection {
		sections[s] = true
	}

	for s := range sections {
		if _, exempt := adjudicatedSubRows[s]; exempt {
			// STATUS set-equality legitimately cannot hold for the adjudicated
			// decompositions (they carry different statuses than the guide's
			// single bundled row by design). Their id-set is pinned instead by
			// findAdjudicatedSubRowMismatches against adjudicatedSubRows — no
			// longer a both-directions blank exemption.
			continue
		}
		guideSet := guideBySection[s]
		manifestSet := manifestBySection[s]
		for st := range guideSet {
			if !manifestSet[st] {
				t.Errorf("section %s: guide's Appendix B asserts status %s, but no manifest row for %s carries it — a silently DROPPED Appendix B sub-claim now lives nowhere the gate can see (judged-ac1-dropped-secondary-status-subclaims)", s, st, s)
			}
		}
		for st := range manifestSet {
			if !guideSet[st] {
				t.Errorf("section %s: manifest carries status %s, but the guide's Appendix B asserts no such status for %s — the manifest must never claim MORE than the guide", s, st, s)
			}
		}
	}
}

// TestGuideClaimsExistsQualifiersNotDropped is ac-1's EXISTS-qualifier
// fidelity check (judged-ac1-exists-qualifier-dropped): Appendix B qualifies
// two EXISTS rows, and dropping the qualifier makes the manifest's claim
// strictly STRONGER than the guide's. The qualifiers are pinned here as
// test data transcribed from Appendix B (guide lines 7.4 and 14.2) — a
// decode-only check that runs even in a bare clone (no workspace needed).
func TestGuideClaimsExistsQualifiersNotDropped(t *testing.T) {
	// section -> the guide qualifier that must survive as caveat substring.
	want := map[string]string{
		"7.4":  "over the canonical model only", // Appendix B: "EXISTS — over the canonical model only"
		"14.2": "(VL-013)",                      // Appendix B: "EXISTS (VL-013)"
	}
	m := decodeRealGuideClaims(t, verdiRepoRoot)
	for section, qualifier := range want {
		found := false
		for _, r := range m.Rows {
			if r.Section == section && strings.Contains(r.Caveat, qualifier) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("section %s: no manifest row carries the guide's EXISTS qualifier %q in its caveat — a dropped honesty qualifier that makes the manifest's claim stronger than the guide's (judged-ac1-exists-qualifier-dropped)", section, qualifier)
		}
	}
}

// TestFindAdjudicatedSubRowMismatches is ac-1's EXPECTED SUB-ROW TABLE proven
// hermetically over synthetic manifests: the exact pinned set is clean, a
// DELETED adjudicated sub-row reds naming the section and the id (the
// dispatch's red-first case — 8.4's INVENTED 8.4-waive-verb dropped), and an
// EXTRA unadjudicated sub-row in an exempt section reds. This is the mechanical
// pin the old blanket both-directions exemption lacked
// (judged-ac1-adjudicated-sections-exempt-from-fidelity-both-directions).
func TestFindAdjudicatedSubRowMismatches(t *testing.T) {
	full := &artifact.GuideClaimsManifest{Rows: []artifact.GuideClaimRow{
		{ID: "8.4-waived-status-and-kind", Section: "8.4", Status: artifact.GuideClaimExists},
		{ID: "8.4-reaffirmations-kind", Section: "8.4", Status: artifact.GuideClaimExists},
		{ID: "8.4-waive-verb", Section: "8.4", Status: artifact.GuideClaimInvented},
	}}
	table := map[string][]string{"8.4": {
		"8.4-waived-status-and-kind", "8.4-reaffirmations-kind", "8.4-waive-verb",
	}}

	t.Run("exact pinned set is clean", func(t *testing.T) {
		if f := findAdjudicatedSubRowMismatches(table, full); len(f) != 0 {
			t.Fatalf("want no findings for the exact pinned set, got %v", f)
		}
	})

	t.Run("deleting 8.4's INVENTED sub-row reds naming section and id", func(t *testing.T) {
		// Drop 8.4-waive-verb (the INVENTED sub-row) — with the old blanket
		// exemption this deletion still greened the fidelity gate.
		missing := &artifact.GuideClaimsManifest{Rows: full.Rows[:2]}
		f := findAdjudicatedSubRowMismatches(table, missing)
		if len(f) != 1 {
			t.Fatalf("want exactly 1 finding for a deleted adjudicated sub-row, got %v", f)
		}
		if !strings.Contains(f[0], "8.4") || !strings.Contains(f[0], "8.4-waive-verb") {
			t.Errorf("finding = %q, want it to name both section 8.4 and 8.4-waive-verb", f[0])
		}
	})

	t.Run("an extra unadjudicated sub-row in an exempt section reds", func(t *testing.T) {
		extra := &artifact.GuideClaimsManifest{Rows: append(append([]artifact.GuideClaimRow(nil), full.Rows...),
			artifact.GuideClaimRow{ID: "8.4-conjured-from-nowhere", Section: "8.4", Status: artifact.GuideClaimExists})}
		f := findAdjudicatedSubRowMismatches(table, extra)
		if len(f) != 1 {
			t.Fatalf("want exactly 1 finding for an extra sub-row, got %v", f)
		}
		if !strings.Contains(f[0], "8.4") || !strings.Contains(f[0], "8.4-conjured-from-nowhere") {
			t.Errorf("finding = %q, want it to name section 8.4 and the extra id", f[0])
		}
	})
}

// TestGuideClaimsAdjudicatedSubRows_PinnedAgainstManifest runs ac-1's EXPECTED
// SUB-ROW TABLE against the REAL verdi/docs/guide-claims.yaml. It is
// decode-only (no workspace/guide file needed), so unlike the guide-parsing
// fidelity gate it has teeth even in a bare clone: the four adjudicated
// sections must carry EXACTLY their pinned sub-row ids, or the decomposition
// silently drifted (judged-ac1-adjudicated-sections-exempt-from-fidelity-both-
// directions).
func TestGuideClaimsAdjudicatedSubRows_PinnedAgainstManifest(t *testing.T) {
	m := decodeRealGuideClaims(t, verdiRepoRoot)
	if f := findAdjudicatedSubRowMismatches(adjudicatedSubRows, m); len(f) > 0 {
		t.Errorf("guide-claims.yaml adjudicated sections drifted from ac-1's EXPECTED SUB-ROW TABLE (%d):\n  %s", len(f), strings.Join(f, "\n  "))
	}
}
