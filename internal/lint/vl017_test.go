package lint

import (
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

const openQuestionAnnotationJSONL = `{"id":"a-01J8Z0K9DDDDDDDDDDDDDDDDDD","ts":"2026-07-11T18:00:00Z","author":"jyang","target":{"ref":"spec/open-question-story@93ddc5bbbb398cf747151e1c466afb83114398df","selector":{"heading":"open-questions","quote":"should the retry window be configurable per tenant?","line":null}},"type":"question","body":"should the retry window be configurable per tenant?","status":"open"}
`

const resolvedOpenQuestionAnnotationJSONL = `{"id":"a-01J8Z0K9DDDDDDDDDDDDDDDDDD","ts":"2026-07-11T18:00:00Z","author":"jyang","target":{"ref":"spec/open-question-story@93ddc5bbbb398cf747151e1c466afb83114398df","selector":{"heading":"open-questions","quote":"should the retry window be configurable per tenant?","line":null}},"type":"question","body":"should the retry window be configurable per tenant?","status":"resolved"}
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
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1 disclosed-unproven finding:\n%s", len(findings), findingsString(findings))
	}
	if got := findings[0].Message; !containsAll(got, "disclosed-unproven", "data/mutable") {
		t.Fatalf("message = %q, want it to name disclosed-unproven and data/mutable", got)
	}
	if findings[0].Severity != SeverityDisclosure {
		t.Fatalf("severity = %v, want SeverityDisclosure (a printed notice, not a verdict failure)", findings[0].Severity)
	}
	// The disclosure is printed through the shared internal/disclosure seam
	// (spec/disclosure-seam-v2, ac-1) — never silent.
	if got := findings[0].String(); !strings.HasPrefix(got, "disclosed-unproven [lint:VL-017] ") {
		t.Fatalf("String() = %q, want a printed \"disclosed-unproven [lint:VL-017] ...\" disclosure line", got)
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
