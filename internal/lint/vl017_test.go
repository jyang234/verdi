package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// vl017OpenQuestionStorySpec is a new-class story spec (always new,
// isNewClassSpec) carrying no declared open_questions: block of its own —
// the mutable-zone annotation below is the only record of the question.
const vl017OpenQuestionStorySpec = `---
id: spec/open-question-story
kind: spec
class: story
title: "VL-017: open question story"
status: draft
owners: [platform-team]
problem: { text: "retry behavior under tenant load is unclear", anchor: "#problem" }
outcome: { text: "retry behavior is documented and configurable if needed", anchor: "#outcome" }
story: jira:LOAN-1499
links:
  - { type: implements, ref: "spec/stale-decline#ac-1" }
---
# VL-017: open question story

## Problem

Retry behavior under tenant load is unclear.

## Outcome

Retry behavior is documented and configurable if needed.
`

// vl017OpenQuestionStoryCarriedSpec is the same story, but this revision
// has since formalized the question as a declared open_questions: object
// carrying the exact same text the annotation's body carries.
const vl017OpenQuestionStoryCarriedSpec = `---
id: spec/open-question-story
kind: spec
class: story
title: "VL-017: open question story, carried"
status: draft
owners: [platform-team]
problem: { text: "retry behavior under tenant load is unclear", anchor: "#problem" }
outcome: { text: "retry behavior is documented and configurable if needed", anchor: "#outcome" }
story: jira:LOAN-1499
links:
  - { type: implements, ref: "spec/stale-decline#ac-1" }
open_questions:
  - { id: oq-1, text: "should the retry window be configurable per tenant?", anchor: "#oq-1" }
---
# VL-017: open question story, carried

## Problem

Retry behavior under tenant load is unclear.

## Outcome

Retry behavior is documented and configurable if needed.

## OQ-1

Should the retry window be configurable per tenant?
`

const openQuestionAnnotationJSONL = `{"id":"a-01J8Z0K9DDDDDDDDDDDDDDDDDD","ts":"2026-07-11T18:00:00Z","author":"jyang","target":{"ref":"spec/open-question-story@16219044c9d6d41de9a0de9464ed24d49283b40c","selector":{"heading":"open-questions","quote":"should the retry window be configurable per tenant?","line":null}},"type":"question","body":"should the retry window be configurable per tenant?","status":"open"}
`

const resolvedOpenQuestionAnnotationJSONL = `{"id":"a-01J8Z0K9DDDDDDDDDDDDDDDDDD","ts":"2026-07-11T18:00:00Z","author":"jyang","target":{"ref":"spec/open-question-story@16219044c9d6d41de9a0de9464ed24d49283b40c","selector":{"heading":"open-questions","quote":"should the retry window be configurable per tenant?","line":null}},"type":"question","body":"should the retry window be configurable per tenant?","status":"resolved"}
`

// writeMutableAnnotation writes content into root's untracked
// data/mutable/annotations/<name> — the same location vl017.go's
// readMutableAnnotations reads directly off the working tree (never
// through fixturegit/git at all, matching VL-013: the mutable zone is
// never git-tracked).
func writeMutableAnnotation(t *testing.T, root, name, content string) {
	t.Helper()
	writeTestFile(t, filepath.Join(root, ".verdi", "data", "mutable", "annotations", name), content)
}

// removeMutableZone deletes root's data/mutable/ entirely — modeling a
// bare CI clone, where the (gitignored, per-checkout) mutable zone was
// never created at all (01 §Zones).
func removeMutableZone(t *testing.T, root string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(root, ".verdi", "data", "mutable")); err != nil {
		t.Fatalf("removing mutable zone: %v", err)
	}
}

// TestVL017_MutableZoneAbsent_DisclosedUnproven is the "mutable-zone-absent
// case reports disclosed-unproven, never a silent pass" exit criterion
// (E1): a bare clone with no data/mutable/ present never gets a vacuous
// green for a new-class spec. Adjudicated at W2 wave close: the report is a
// SeverityDisclosure notice — printed (never silent) but NOT a verdict
// failure, so a run whose only finding is this disclosure exits 0 (CI stays
// green once a new-class spec exists).
func TestVL017_MutableZoneAbsent_DisclosedUnproven(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/active/open-question-story/spec.md", vl017OpenQuestionStorySpec)
	repo := buildLintRepo(t, dir) // provisions the mutable zone by default
	removeMutableZone(t, repo.Dir)

	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-017")
	// Every new-class spec in the merged examples/showcase corpus (Task
	// 1.2 folded testdata/dexoverlay's escrow-notify(-v2)/rate-lock(-v2)/
	// refi-rate-check-2024 into layers.txt) trips this SAME disclosure on
	// a bare clone, alongside this test's own added open-question-story
	// fixture — so the count is no longer exactly 1; find THIS test's own
	// finding by path instead of assuming it is the only one.
	var got *Finding
	for i := range findings {
		if findings[i].Path == ".verdi/specs/active/open-question-story/spec.md" {
			got = &findings[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("no VL-017 disclosure for .verdi/specs/active/open-question-story/spec.md; findings:\n%s", findingsString(findings))
	}
	if !containsAll(got.Message, "disclosed-unproven", "data/mutable") {
		t.Fatalf("message = %q, want it to name disclosed-unproven and data/mutable", got.Message)
	}
	// spec/uat-round-1 ac-3 (closing UAT-002): the message names the
	// checkout CONDITION (mutable zone absent, never committed, 01
	// §Zones), never a diagnosis of its cause — it must not assert the
	// checkout is a bare clone, which was false on the UAT checkout that
	// surfaced this defect.
	if !containsAll(got.Message, "mutable zone (.verdi/data/mutable/) is absent from this checkout", "never committed", "01 §Zones") {
		t.Fatalf("message = %q, want it to describe the mutable zone as absent from this checkout, never committed (01 §Zones)", got.Message)
	}
	if strings.Contains(got.Message, "bare clone") {
		t.Fatalf("message = %q, must not assert a bare clone (ac-3)", got.Message)
	}
	if got.Severity != SeverityDisclosure {
		t.Fatalf("severity = %v, want SeverityDisclosure (a printed notice, not a verdict failure)", got.Severity)
	}
	// The disclosure is printed through the shared internal/disclosure seam
	// (spec/disclosure-seam-v2, ac-1) — never silent.
	if s := got.String(); !strings.HasPrefix(s, "disclosed-unproven [lint:VL-017] ") {
		t.Fatalf("String() = %q, want a printed \"disclosed-unproven [lint:VL-017] ...\" disclosure line", s)
	}
}

// TestVL017_MutableZonePresent_UnresolvedAndUncarried_Fails is the
// mutable-zone-present twin: an open-question annotation that is neither
// resolved nor carried as a declared object fails VL-017.
func TestVL017_MutableZonePresent_UnresolvedAndUncarried_Fails(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/active/open-question-story/spec.md", vl017OpenQuestionStorySpec)
	repo := buildLintRepo(t, dir)
	writeMutableAnnotation(t, repo.Dir, "spec--open-question-story.jsonl", openQuestionAnnotationJSONL)

	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-017")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

// TestVL017_MutableZonePresent_Resolved_Clean is the "status: resolved"
// half of "resolved-or-carried": a resolved open-question annotation never
// fires VL-017, mutable zone present.
func TestVL017_MutableZonePresent_Resolved_Clean(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/active/open-question-story/spec.md", vl017OpenQuestionStorySpec)
	repo := buildLintRepo(t, dir)
	writeMutableAnnotation(t, repo.Dir, "spec--open-question-story.jsonl", resolvedOpenQuestionAnnotationJSONL)

	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-017" {
			t.Fatalf("VL-017 fired on a resolved open-question annotation: %s", f.String())
		}
	}
}

// TestVL017_MutableZonePresent_Carried_Clean is the "carried" half: an
// unresolved open-question annotation whose text is formalized as a
// declared open_questions object on the spec never fires VL-017.
func TestVL017_MutableZonePresent_Carried_Clean(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/active/open-question-story/spec.md", vl017OpenQuestionStoryCarriedSpec)
	repo := buildLintRepo(t, dir)
	writeMutableAnnotation(t, repo.Dir, "spec--open-question-story.jsonl", openQuestionAnnotationJSONL)

	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-017" {
			t.Fatalf("VL-017 fired on an annotation carried as a declared open_questions object: %s", f.String())
		}
	}
}

// containsAll reports whether s contains every one of subs.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// TestCollapseVL017Disclosures_MultipleSpecsCollapseToOne is ac-3's core
// exerciser at the function level (closing UAT-002): given several
// per-spec VL-017 disclosure findings (Check's own output shape on a
// mutable-zone-absent checkout), the collapse replaces them all with
// exactly one Finding naming every affected spec — never the identical
// paragraph repeated once per spec.
func TestCollapseVL017Disclosures_MultipleSpecsCollapseToOne(t *testing.T) {
	in := []Finding{
		{Rule: "VL-001", Path: ".verdi/adr/0001-x.md", Message: "unrelated violation"},
		{Rule: "VL-017", Path: ".verdi/specs/active/zeta/spec.md", Severity: SeverityDisclosure, Message: vl017DisclosureCore},
		{Rule: "VL-017", Path: ".verdi/specs/active/alpha/spec.md", Severity: SeverityDisclosure, Message: vl017DisclosureCore},
	}
	got := CollapseVL017Disclosures(in)

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (the untouched VL-001 finding plus one combined VL-017 disclosure):\n%s", len(got), findingsString(got))
	}
	if got[0] != in[0] {
		t.Fatalf("got[0] = %+v, want the unrelated VL-001 finding untouched: %+v", got[0], in[0])
	}
	combined := got[1]
	if combined.Rule != "VL-017" || combined.Severity != SeverityDisclosure {
		t.Fatalf("combined finding = %+v, want Rule VL-017 / Severity SeverityDisclosure", combined)
	}
	if combined.Path != "" {
		t.Fatalf("combined.Path = %q, want empty (a checkout-wide disclosure, no single artifact locus)", combined.Path)
	}
	// Sorted regardless of input order (alpha was given second).
	wantOrder := ".verdi/specs/active/alpha/spec.md, .verdi/specs/active/zeta/spec.md"
	if !strings.Contains(combined.Message, wantOrder) {
		t.Fatalf("combined.Message = %q, want the affected paths listed sorted: %q", combined.Message, wantOrder)
	}
	if !strings.Contains(combined.Message, "(2)") {
		t.Fatalf("combined.Message = %q, want it to name the affected-spec count (2)", combined.Message)
	}
	if !containsAll(combined.Message, "mutable zone (.verdi/data/mutable/) is absent from this checkout", "never committed", "01 §Zones") {
		t.Fatalf("combined.Message = %q, want the ac-3 condition wording", combined.Message)
	}
	if strings.Contains(combined.Message, "bare clone") {
		t.Fatalf("combined.Message = %q, must not assert a bare clone (ac-3)", combined.Message)
	}
	// Printed through the shared seam, never silent.
	if s := combined.String(); !strings.HasPrefix(s, "disclosed-unproven [lint:VL-017]: ") {
		t.Fatalf("combined.String() = %q, want the unscoped shared-seam rendering", s)
	}
}

// TestCollapseVL017Disclosures_NoDisclosures_PassesThroughUnchanged is the
// negative path: a findings slice with no VL-017 disclosure at all
// (whether empty, or carrying only unrelated/violation findings) comes
// back byte-for-byte unchanged — no spurious combined Finding ever
// appears when there is nothing to disclose.
func TestCollapseVL017Disclosures_NoDisclosures_PassesThroughUnchanged(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		got := CollapseVL017Disclosures(nil)
		if len(got) != 0 {
			t.Fatalf("got %d findings, want 0: %s", len(got), findingsString(got))
		}
	})

	t.Run("unrelated findings only", func(t *testing.T) {
		in := []Finding{
			{Rule: "VL-001", Path: "a", Message: "m1"},
			{Rule: "VL-009", Path: "b", Message: "m2", Severity: SeverityDisclosure},
		}
		got := CollapseVL017Disclosures(in)
		if len(got) != len(in) {
			t.Fatalf("len(got) = %d, want %d (unchanged): %s", len(got), len(in), findingsString(got))
		}
		for i := range in {
			if got[i] != in[i] {
				t.Fatalf("got[%d] = %+v, want unchanged %+v", i, got[i], in[i])
			}
		}
	})
}

// TestCollapseVL017Disclosures_ViolationSeverityPassesThroughUncollapsed
// proves the collapse keys on Severity as well as Rule: a VL-017 finding
// from the mutable-zone-PRESENT path (Check's violation branch above,
// Severity the zero value SeverityViolation) is a genuinely different
// notice — an unresolved, uncarried open-question annotation — and must
// never be swept into the disclosure collapse or silently dropped.
func TestCollapseVL017Disclosures_ViolationSeverityPassesThroughUncollapsed(t *testing.T) {
	violation := Finding{Rule: "VL-017", Path: ".verdi/specs/active/x/spec.md", Message: "open-question annotation a-1 is neither resolved nor carried"}
	got := CollapseVL017Disclosures([]Finding{violation})
	if len(got) != 1 || got[0] != violation {
		t.Fatalf("got = %+v, want the VL-017 violation finding untouched: %+v", got, violation)
	}
}

// TestCollapseVL017Disclosures_SingleSpec_StillCollapsesToOne proves the
// function is a no-op in EFFECT (still exactly one Finding) when only one
// spec is affected — the collapse is unconditional, not merely triggered
// once a second spec appears.
func TestCollapseVL017Disclosures_SingleSpec_StillCollapsesToOne(t *testing.T) {
	in := []Finding{
		{Rule: "VL-017", Path: ".verdi/specs/active/solo/spec.md", Severity: SeverityDisclosure, Message: vl017DisclosureCore},
	}
	got := CollapseVL017Disclosures(in)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1: %s", len(got), findingsString(got))
	}
	if !strings.Contains(got[0].Message, ".verdi/specs/active/solo/spec.md") {
		t.Fatalf("combined.Message = %q, want the sole affected spec named", got[0].Message)
	}
	if !strings.Contains(got[0].Message, "(1)") {
		t.Fatalf("combined.Message = %q, want the affected-spec count (1)", got[0].Message)
	}
}

// vl017RestatedCorpusDisclosure is exactly what candidate.go's
// unrelatedCorpusDisclosures builds for a VL-017 finding about a document
// other than the candidate: the ORIGINAL rule id and path, severity forced
// to SeverityDisclosure, and the original fact wrapped in the "unrelated
// existing corpus finding" preamble. The wrapped fact here is the real
// mutable-zone-PRESENT violation — an unresolved, uncarried open-question
// annotation — i.e. the most damaging thing a rule-plus-severity-only
// collapse could swallow.
func vl017RestatedCorpusDisclosure() Finding {
	violation := Finding{
		Rule:    "VL-017",
		Path:    ".verdi/specs/active/other-story/spec.md",
		Message: `open-question annotation a-01J8Z0K9DDDDDDDDDDDDDDDDDD ("should the retry window be configurable per tenant?") is neither status:resolved nor carried as a declared open_questions object on spec/other-story`,
	}
	return Finding{
		Rule:     violation.Rule,
		Path:     violation.Path,
		Message:  fmt.Sprintf("unrelated existing corpus finding: %s %s: %s", violation.Rule, violation.Path, violation.Message),
		Severity: SeverityDisclosure,
	}
}

// TestCollapseVL017Disclosures_OnlyTheAbsentZoneNoticeCollapses pins the
// collapse's third gate: the MESSAGE must be the mutable-zone-absent
// notice Check itself raises (vl017DisclosureCore), not merely "some
// VL-017 finding at SeverityDisclosure".
//
// Rule+severity alone is not a safe key, because SeverityDisclosure is not
// owned by this rule: candidate.go's unrelatedCorpusDisclosures restates
// ARBITRARY findings — a real mutable-zone-PRESENT VL-017 violation
// included — at SeverityDisclosure under their original rule id. Keyed on
// rule and severity alone, such a restatement would be deleted and
// "replaced" by a zone-absent notice asserting the exact opposite of what
// it said: the strongest form of the vacuous pass constitution 2 and
// spec/uat-round-1 co-5 forbid ("a quieter disclosure is still a
// disclosure, never a vacuous pass"). No caller feeds candidate output to
// this collapse today; this test is what keeps that from becoming a silent
// corruption if one ever does.
//
// The already-collapsed row additionally pins EXACT equality over a prefix
// test: the combined finding's own message starts with the core sentence,
// so a prefix gate would re-collapse it and lose the affected-spec list
// behind the combined finding's empty Path. Equality makes the function
// idempotent.
func TestCollapseVL017Disclosures_OnlyTheAbsentZoneNoticeCollapses(t *testing.T) {
	absentZoneNotice := Finding{
		Rule:     "VL-017",
		Path:     ".verdi/specs/active/borrower-update/spec.md",
		Severity: SeverityDisclosure,
		Message:  vl017DisclosureCore,
	}
	alreadyCollapsed := CollapseVL017Disclosures([]Finding{absentZoneNotice})[0]

	tests := []struct {
		name          string
		in            Finding
		wantCollapsed bool
	}{
		{
			name:          "absent-zone notice as Check raises it",
			in:            absentZoneNotice,
			wantCollapsed: true,
		},
		{
			name:          "restated corpus finding at disclosure severity",
			in:            vl017RestatedCorpusDisclosure(),
			wantCollapsed: false,
		},
		{
			name:          "already-collapsed combined disclosure",
			in:            alreadyCollapsed,
			wantCollapsed: false,
		},
		{
			name:          "mutable-zone-present violation",
			in:            Finding{Rule: "VL-017", Path: ".verdi/specs/active/x/spec.md", Message: "open-question annotation a-1 is neither status:resolved nor carried"},
			wantCollapsed: false,
		},
		{
			name:          "another rule's disclosure carrying the same sentence",
			in:            Finding{Rule: "VL-009", Path: ".verdi/specs/active/y/spec.md", Severity: SeverityDisclosure, Message: vl017DisclosureCore},
			wantCollapsed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CollapseVL017Disclosures([]Finding{tt.in})
			if len(got) != 1 {
				t.Fatalf("len(got) = %d, want 1:\n%s", len(got), findingsString(got))
			}
			if !tt.wantCollapsed {
				if got[0] != tt.in {
					t.Fatalf("got[0] = %+v, want the finding passed through byte-for-byte: %+v", got[0], tt.in)
				}
				return
			}
			if got[0] == tt.in {
				t.Fatalf("got[0] = %+v, want a COMBINED finding, not the input unchanged", got[0])
			}
			if got[0].Path != "" {
				t.Fatalf("combined.Path = %q, want empty (a checkout-wide disclosure)", got[0].Path)
			}
			if !strings.Contains(got[0].Message, tt.in.Path) {
				t.Fatalf("combined.Message = %q, want the affected spec %q named", got[0].Message, tt.in.Path)
			}
		})
	}
}
