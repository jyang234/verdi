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

// adjudicatedSubRow is one pinned (id, STATUS) pair in ac-1's EXPECTED SUB-ROW
// TABLE: the atomic sub-row's id AND the exact status the adjudication + the
// guide's Appendix B grant it.
type adjudicatedSubRow struct {
	ID     string
	Status artifact.GuideClaimStatus
}

// adjudicatedSubRows is ac-1's EXPECTED SUB-ROW TABLE: for each of the four
// Appendix B sections whose single bundled-status prose row the manifest
// intentionally decomposes (the Task-0 design wave's adjudication, ledger
// L-N4: "Appendix B's prose rows become display groupings"), the EXACT set of
// atomic sub-rows that decomposition produced — each pinned as an (id, STATUS)
// PAIR. These sections carry DIFFERENT statuses than the guide's single
// bundled row by design — 8.4's bundled row becomes two EXISTS sub-rows + an
// INVENTED sub-row, 5.3's likewise — so per-section STATUS set-equality
// legitimately cannot hold and TestGuideClaimsTranscriptionFidelity_AppendixB
// still exempts them from THAT check. The pinned status here is the exact
// per-sub-row status the adjudicated decomposition assigns, transcribed from
// docs/guide-claims.yaml as it stands.
//
// What an id-ONLY pin did NOT do — and this (id, status) pin now does — is
// close the upgrade hole in the adjudicated sections. Inside 5.3/6.2/7.2/8.4
// the fidelity gate's status set-equality is exempt, findAdjudicatedSubRow-
// Mismatches once checked ids alone, the downgrade detector ignores upgrades
// by design, and decode is guide-blind — so flipping 8.4-waive-verb from
// INVENTED to EXISTS (or any adjudicated sub-row's status) greened every gate,
// the manifest claiming MORE than the guide grants in precisely the four
// sections the adjudication decomposed (judged-ac1-adjudicated-section-
// statuses-unpinned-upgrade-invisible). Pinning the status here reds that flip
// at no extra machinery. It also still pins the decomposition's SHAPE: a
// missing OR extra sub-row reds naming the section and the id
// (judged-ac1-adjudicated-sections-exempt-from-fidelity-both-directions).
//
// A legitimate future status flip (e.g. Phase 2 Task 15 lands `verdi waive`
// and 8.4-waive-verb becomes EXISTS) updates THIS pin, the manifest row, AND
// the guide's Appendix B (the workspace-side honesty ledger) in the SAME
// reviewed edit — that co-visibility is the point: the adjudicated status can
// never move in one layer while the other two silently disagree.
var adjudicatedSubRows = map[string][]adjudicatedSubRow{
	"5.3": {
		{"5.3-user-editable-templates", artifact.GuideClaimPartial},
		{"5.3-template-contract", artifact.GuideClaimExists},
		{"5.3-custom-namespace", artifact.GuideClaimExists},
		{"5.3-form-field-generation", artifact.GuideClaimInvented},
	},
	"6.2": {
		{"6.2-stubs", artifact.GuideClaimExists},
		{"6.2-board-stub-instantiate", artifact.GuideClaimExists},
		{"6.2-closure-reconciliation", artifact.GuideClaimExists},
		{"6.2-from-stub-cli", artifact.GuideClaimInvented},
	},
	"7.2": {
		{"7.2-ci-evidence-bundles", artifact.GuideClaimExists},
		{"7.2-verdi-sync", artifact.GuideClaimExists},
		{"7.2-authoritative-vs-advisory", artifact.GuideClaimExists},
		{"7.2-fold", artifact.GuideClaimExists},
		{"7.2-matrix", artifact.GuideClaimExists},
		{"7.2-obligation-wall-and-receipts", artifact.GuideClaimExists},
	},
	"8.4": {
		{"8.4-waived-status-and-kind", artifact.GuideClaimExists},
		{"8.4-reaffirmations-kind", artifact.GuideClaimExists},
		{"8.4-waive-verb", artifact.GuideClaimInvented},
	},
}

// findAdjudicatedSubRowMismatches enforces ac-1's EXPECTED SUB-ROW TABLE: for
// each adjudicated section, the set of manifest (id, STATUS) pairs must EQUAL
// the pinned set in expected. Three ways to red, all naming the section and the
// offending id, deterministic order:
//   - a pinned id MISSING from the manifest — a silently dropped decomposed
//     sub-claim (judged-ac1-adjudicated-sections-exempt-from-fidelity-both-
//     directions);
//   - a pinned id present but carrying a status DIFFERENT from the pinned one —
//     a status flip (e.g. an INVENTED->EXISTS upgrade) that the fidelity gate's
//     set-equality exemption and the upgrade-blind downgrade detector cannot
//     see (judged-ac1-adjudicated-section-statuses-unpinned-upgrade-invisible);
//   - a manifest id in an adjudicated section the table does NOT pin — an
//     unadjudicated sub-row that appeared.
func findAdjudicatedSubRowMismatches(expected map[string][]adjudicatedSubRow, m *artifact.GuideClaimsManifest) []string {
	actual := map[string]map[string]artifact.GuideClaimStatus{}
	for _, r := range m.Rows {
		if _, pinned := expected[r.Section]; !pinned {
			continue
		}
		if actual[r.Section] == nil {
			actual[r.Section] = map[string]artifact.GuideClaimStatus{}
		}
		actual[r.Section][r.ID] = r.Status
	}

	sections := make([]string, 0, len(expected))
	for s := range expected {
		sections = append(sections, s)
	}
	sort.Strings(sections)

	var findings []string
	for _, s := range sections {
		want := make(map[string]artifact.GuideClaimStatus, len(expected[s]))
		wantIDs := make([]string, 0, len(expected[s]))
		for _, sub := range expected[s] {
			want[sub.ID] = sub.Status
			wantIDs = append(wantIDs, sub.ID)
		}
		sort.Strings(wantIDs)
		got := actual[s]
		for _, id := range wantIDs {
			gotStatus, present := got[id]
			if !present {
				findings = append(findings, fmt.Sprintf("section %s: adjudicated sub-row %q is pinned by ac-1's EXPECTED SUB-ROW TABLE but is MISSING from the manifest — deleting a decomposed sub-row silently re-opens the dropped-sub-claim gap the blanket exemption once left open (judged-ac1-adjudicated-sections-exempt-from-fidelity-both-directions)", s, id))
				continue
			}
			if gotStatus != want[id] {
				findings = append(findings, fmt.Sprintf("section %s: adjudicated sub-row %q carries status %s but ac-1's EXPECTED SUB-ROW TABLE pins it at %s — a status flip in an adjudicated section is invisible to the fidelity set-equality exemption and to the upgrade-blind downgrade detector, so it is pinned HERE; a legitimate flip updates this pin, the manifest row, AND the guide's Appendix B in one reviewed edit (judged-ac1-adjudicated-section-statuses-unpinned-upgrade-invisible)", s, id, gotStatus, want[id]))
			}
		}
		gotIDs := make([]string, 0, len(got))
		for id := range got {
			gotIDs = append(gotIDs, id)
		}
		sort.Strings(gotIDs)
		for _, id := range gotIDs {
			if _, ok := want[id]; !ok {
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

// checkAppendixBFidelity reads the guide at workspaceRoot/guideRelPath, parses
// its Appendix B, and returns one finding per per-section status set-equality
// violation against m (adjudicated sections exempt — their (id, status) pairs
// are pinned instead by findAdjudicatedSubRowMismatches). A guide status with
// no matching manifest row is a DROPPED sub-claim; a manifest status the guide
// does not assert is the manifest claiming MORE than the guide.
//
// It returns a NON-NIL ERROR — a drift signal the caller reds on — when the
// guide is UNREADABLE or parses to ZERO Appendix B rows. The caller invokes
// this only after guideClaimsWorkspaceRoot found the workspace marker, so an
// unreadable/unparseable guide here means the guide was RELOCATED, removed, or
// reformatted WITHIN a live workspace — exactly the drift this gate exists to
// catch, symmetric with the long-standing zero-rows hard error. The bare-clone
// case (no workspace marker at all) is the caller's own loud SKIP, upstream of
// this helper, and is unchanged (judged-ac1-fidelity-gate-skips-when-guide-
// file-missing-under-found-workspace).
func checkAppendixBFidelity(workspaceRoot string, m *artifact.GuideClaimsManifest) ([]string, error) {
	guidePath := filepath.Join(workspaceRoot, filepath.FromSlash(guideRelPath))
	data, err := os.ReadFile(guidePath)
	if err != nil {
		return nil, fmt.Errorf("guide %s unreadable under a FOUND workspace (%w) — the guide was relocated or removed; the transcription-fidelity gate reds on this drift rather than skipping (judged-ac1-fidelity-gate-skips-when-guide-file-missing-under-found-workspace)", guidePath, err)
	}
	guideBySection := parseAppendixBStatuses(string(data))
	if len(guideBySection) == 0 {
		return nil, fmt.Errorf("parsed zero Appendix B rows from %s — the guide moved or its Appendix B format changed; the fidelity gate cannot silently pass on an unparseable guide", guidePath)
	}

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
	sorted := make([]string, 0, len(sections))
	for s := range sections {
		sorted = append(sorted, s)
	}
	sort.Strings(sorted)

	var findings []string
	for _, s := range sorted {
		if _, exempt := adjudicatedSubRows[s]; exempt {
			// STATUS set-equality legitimately cannot hold for the adjudicated
			// decompositions (they carry different statuses than the guide's
			// single bundled row by design). Their (id, status) pairs are pinned
			// instead by findAdjudicatedSubRowMismatches against
			// adjudicatedSubRows — no longer a both-directions blank exemption.
			continue
		}
		guideSet := guideBySection[s]
		manifestSet := manifestBySection[s]
		for _, st := range guideClaimStatusTokens {
			if guideSet[st] && !manifestSet[st] {
				findings = append(findings, fmt.Sprintf("section %s: guide's Appendix B asserts status %s, but no manifest row for %s carries it — a silently DROPPED Appendix B sub-claim now lives nowhere the gate can see (judged-ac1-dropped-secondary-status-subclaims)", s, st, s))
			}
			if manifestSet[st] && !guideSet[st] {
				findings = append(findings, fmt.Sprintf("section %s: manifest carries status %s, but the guide's Appendix B asserts no such status for %s — the manifest must never claim MORE than the guide", s, st, s))
			}
		}
	}
	return findings, nil
}

// TestGuideClaimsTranscriptionFidelity_AppendixB is ac-1's transcription
// fidelity gate (judged-ac1-dropped-secondary-status-subclaims): for every
// Appendix B section outside the four adjudicated decompositions, the set
// of statuses the guide's table asserts equals the set of statuses the
// manifest carries for that section. A guide status with no matching
// manifest row is a DROPPED sub-claim; a manifest status the guide does not
// assert is the manifest claiming MORE than the guide.
//
// It SKIPS only for a true bare clone (no workspace marker at all). Once the
// marker IS found, an unreadable or unparseable guide REDS as drift, never
// skips (judged-ac1-fidelity-gate-skips-when-guide-file-missing-under-found-
// workspace) — checkAppendixBFidelity returns the error and this fails.
func TestGuideClaimsTranscriptionFidelity_AppendixB(t *testing.T) {
	root, ok := guideClaimsWorkspaceRoot(verdiRepoRoot)
	if !ok {
		t.Skipf("DISCLOSURE: no workspace marker docs/design/plans found within %d levels above %s — the Integration & Startup Guide is out-of-repo and cannot be read in this layout. This is a SKIP, not a pass: a green run here is NOT proof the manifest transcribes Appendix B faithfully.", guideClaimsWorkspaceWalkLimit, verdiRepoRoot)
	}
	m := decodeRealGuideClaims(t, verdiRepoRoot)
	findings, err := checkAppendixBFidelity(root, m)
	if err != nil {
		t.Fatalf("DRIFT (workspace marker present, guide unreadable/unparseable): %v — this REDS, it is not a skip: a guide relocated or reformatted within a live workspace is the exact drift signal this gate exists to catch (judged-ac1-fidelity-gate-skips-when-guide-file-missing-under-found-workspace)", err)
	}
	for _, f := range findings {
		t.Error(f)
	}
}

// TestCheckAppendixBFidelity proves the helper hermetically over synthetic
// workspaces: the red-first case (judged-ac1-fidelity-gate-skips-when-guide-
// file-missing-under-found-workspace) is a FOUND workspace whose guide is
// absent — it must red (return an error), not skip. Plus the happy path, a
// manifest-claims-more finding, and the unparseable-guide drift.
func TestCheckAppendixBFidelity(t *testing.T) {
	writeGuide := func(t *testing.T, ws, body string) {
		t.Helper()
		p := filepath.Join(ws, filepath.FromSlash(guideRelPath))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// marker plants the docs/design/plans directory guideClaimsWorkspaceRoot
	// keys on, so each fixture is a FOUND workspace (not a bare clone) — the
	// precondition the real caller establishes before invoking the helper.
	marker := func(t *testing.T, ws string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(ws, "docs", "design", "plans"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	oneRow := func(status artifact.GuideClaimStatus) *artifact.GuideClaimsManifest {
		return &artifact.GuideClaimsManifest{Rows: []artifact.GuideClaimRow{
			{ID: "1-thing", Section: "1", Capability: "Thing", Status: status},
		}}
	}
	guideBody := func(status string) string {
		return strings.Join([]string{
			"## Appendix B — Honesty ledger",
			"",
			"| § | Capability | Status today |",
			"|---|---|---|",
			"| 1 | Thing | **" + status + "** |",
		}, "\n")
	}

	t.Run("guide absent under a FOUND workspace reds (relocation drift, not a skip)", func(t *testing.T) {
		ws := t.TempDir()
		marker(t, ws) // workspace present ...
		// ... but the guide file is deliberately NOT written (relocated/removed).
		if _, err := checkAppendixBFidelity(ws, oneRow(artifact.GuideClaimExists)); err == nil {
			t.Fatal("want an error (RED) when the workspace marker is present but the guide is absent — a relocated guide is the drift this gate exists to catch; it must not skip (judged-ac1-fidelity-gate-skips-when-guide-file-missing-under-found-workspace)")
		}
	})

	t.Run("guide present and faithfully transcribed is clean", func(t *testing.T) {
		ws := t.TempDir()
		marker(t, ws)
		writeGuide(t, ws, guideBody("EXISTS"))
		f, err := checkAppendixBFidelity(ws, oneRow(artifact.GuideClaimExists))
		if err != nil {
			t.Fatalf("unexpected error for a present, faithful guide: %v", err)
		}
		if len(f) != 0 {
			t.Fatalf("want no findings for a faithful transcription, got %v", f)
		}
	})

	t.Run("manifest claiming MORE than the guide reds as a finding", func(t *testing.T) {
		ws := t.TempDir()
		marker(t, ws)
		writeGuide(t, ws, guideBody("INVENTED"))                                // guide asserts INVENTED ...
		f, err := checkAppendixBFidelity(ws, oneRow(artifact.GuideClaimExists)) // ... manifest claims EXISTS
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(f) == 0 {
			t.Fatal("want a finding when the manifest claims MORE than the guide, got none")
		}
	})

	t.Run("guide present but Appendix B unparseable (zero rows) reds", func(t *testing.T) {
		ws := t.TempDir()
		marker(t, ws)
		writeGuide(t, ws, "# Guide with no Appendix B table at all\n")
		if _, err := checkAppendixBFidelity(ws, oneRow(artifact.GuideClaimExists)); err == nil {
			t.Fatal("want an error (RED) when the guide parses to zero Appendix B rows — the guide moved or its format changed")
		}
	})
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
	table := map[string][]adjudicatedSubRow{"8.4": {
		{"8.4-waived-status-and-kind", artifact.GuideClaimExists},
		{"8.4-reaffirmations-kind", artifact.GuideClaimExists},
		{"8.4-waive-verb", artifact.GuideClaimInvented},
	}}

	t.Run("exact pinned set is clean", func(t *testing.T) {
		if f := findAdjudicatedSubRowMismatches(table, full); len(f) != 0 {
			t.Fatalf("want no findings for the exact pinned set, got %v", f)
		}
	})

	t.Run("flipping 8.4-waive-verb INVENTED->EXISTS reds naming id and both statuses", func(t *testing.T) {
		// The dispatch's red-first case: the id-set is unchanged, so an id-only
		// pin sees nothing and every gate greens — the manifest claiming MORE
		// than the guide grants in an adjudicated section
		// (judged-ac1-adjudicated-section-statuses-unpinned-upgrade-invisible).
		flipped := &artifact.GuideClaimsManifest{Rows: []artifact.GuideClaimRow{
			full.Rows[0], full.Rows[1],
			{ID: "8.4-waive-verb", Section: "8.4", Status: artifact.GuideClaimExists},
		}}
		f := findAdjudicatedSubRowMismatches(table, flipped)
		if len(f) != 1 {
			t.Fatalf("want exactly 1 status-mismatch finding for a flipped adjudicated sub-row, got %v", f)
		}
		if !strings.Contains(f[0], "8.4-waive-verb") || !strings.Contains(f[0], "EXISTS") || !strings.Contains(f[0], "INVENTED") {
			t.Errorf("finding = %q, want it to name 8.4-waive-verb, the found EXISTS, and the pinned INVENTED", f[0])
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
