package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/store"
)

// featureSpecStaleTestHex is a stand-in sha256 hex payload (64 hex chars)
// for fixture digest/covers fields this file's deviation-report.md fixtures
// need — mirrors closuregate_test.go's own convention.
var featureSpecStaleTestHex = strings.Repeat("ab", 32)

// writeFeatureStaleDeviationReport writes a minimal, schema-valid
// deviation-report.md at zone/name (store.ZoneActive or store.ZoneArchive)
// under root — this file's own fixture writer, since closuregate_test.go's
// writeGateReport is hardcoded to the "stale-decline" story name and the
// active zone only.
func writeFeatureStaleDeviationReport(t *testing.T, root, zone, name, findingsYAML, notResurfacedYAML string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", zone, name)
	nr := ""
	if notResurfacedYAML != "" {
		nr = "not-resurfaced:\n" + notResurfacedYAML
	}
	content := "---\nschema: verdi.deviation/v1\ncovers: " + strings.Repeat("a", 40) + "\nfindings:\n" + findingsYAML + nr +
		"digest: sha256:" + featureSpecStaleTestHex + "\n---\n# Alignment report\n"
	writeTestFile(t, filepath.Join(dir, "deviation-report.md"), []byte(content))
}

func featureStaleTestSpec(id string, acIDs ...string) *artifact.SpecFrontmatter {
	acs := make([]artifact.AcceptanceCriterion, len(acIDs))
	for i, id := range acIDs {
		acs[i] = artifact.AcceptanceCriterion{ID: id, Text: "t", Evidence: []artifact.EvidenceKind{artifact.EvidenceBehavioral}}
	}
	return &artifact.SpecFrontmatter{
		Base:               artifact.Base{ID: id, Kind: artifact.KindSpec, Title: "t", Owners: []string{"platform-team"}},
		Class:              artifact.ClassFeature,
		AcceptanceCriteria: acs,
	}
}

// TestCheckFeatureSpecStaleCondition_UnionsStoryArchive_NeverZero is
// spec/finding-identity ac-4's "never zero" half, the true X-18 shape: an
// accepted-deviation recorded ONLY in a closed implementing story's
// ARCHIVED report (the feature's own report never reproduced it) must
// still count toward the feature-close budget — not silently dropped.
func TestCheckFeatureSpecStaleCondition_UnionsStoryArchive_NeverZero(t *testing.T) {
	root := t.TempDir()
	feature := featureStaleTestSpec("spec/my-feature", "ac-1")

	storyFindings := "  - { id: judged-a, kind: judged, text: t1, disposition: accepted-deviation, note: n1 }\n" +
		"  - { id: judged-b, kind: judged, text: t2, disposition: accepted-deviation, note: n2 }\n" +
		"  - { id: judged-c, kind: judged, text: t3, disposition: accepted-deviation, note: n3 }\n" +
		"  - { id: judged-d, kind: judged, text: t4, disposition: accepted-deviation, note: n4 }\n"
	writeFeatureStaleDeviationReport(t, root, store.ZoneArchive, "my-story", storyFindings, "")
	// The feature's own report reproduces NONE of them.
	writeFeatureStaleDeviationReport(t, root, store.ZoneActive, "my-feature", "  - { id: computed-x, kind: computed, text: unrelated, disposition: fixed }\n", "")

	stories := []implementingStoryEdges{{SpecRef: "spec/my-story", Closed: true}}
	manifest := &store.Manifest{Audit: &store.AuditConfig{DeviationsStaleThreshold: 3}}

	cond, err := checkFeatureSpecStaleCondition(root, feature, manifest, stories, nil)
	if err != nil {
		t.Fatalf("checkFeatureSpecStaleCondition: %v", err)
	}
	if cond.OK {
		t.Fatalf("cond = %+v, want FAIL — 4 accepted-deviations living ONLY in the story's archive must still count (never zero)", cond)
	}
	if !strings.Contains(cond.Reason, "count 4") {
		t.Fatalf("Reason = %q, want it to name accepted-deviation count 4", cond.Reason)
	}
}

// TestCheckFeatureSpecStaleCondition_UnionsStoryArchive_NeverTwice is ac-4's
// "never twice" half, the true X-18 shape's other side: the IDENTICAL
// accepted-deviation finding present in BOTH a closed implementing story's
// archived report AND the feature's own report must count exactly once.
func TestCheckFeatureSpecStaleCondition_UnionsStoryArchive_NeverTwice(t *testing.T) {
	root := t.TempDir()
	feature := featureStaleTestSpec("spec/my-feature", "ac-1")

	shared := "  - { id: judged-shared, kind: judged, text: \"identical text\", disposition: accepted-deviation, note: n }\n"
	writeFeatureStaleDeviationReport(t, root, store.ZoneArchive, "my-story", shared, "")
	writeFeatureStaleDeviationReport(t, root, store.ZoneActive, "my-feature", shared, "")

	stories := []implementingStoryEdges{{SpecRef: "spec/my-story", Closed: true}}
	manifest := &store.Manifest{Audit: &store.AuditConfig{DeviationsStaleThreshold: 3}}

	cond, err := checkFeatureSpecStaleCondition(root, feature, manifest, stories, nil)
	if err != nil {
		t.Fatalf("checkFeatureSpecStaleCondition: %v", err)
	}
	// Threshold is 3; a correct union count of 1 (not 2) must pass.
	if !cond.OK {
		t.Fatalf("cond = %+v, want PASS — the identical finding recorded in both the story archive and the feature's own report must count exactly once (1, not 2)", cond)
	}
}

// TestCheckFeatureSpecStaleCondition_UnclosedStory_NoArchiveYet_NoOperationalError
// proves a not-yet-closed implementing story (no archived report exists
// yet) is simply skipped from the union — never an operational error —
// since printFeatureMatrix-style callers compute every condition
// unconditionally regardless of condition 3's own verdict.
func TestCheckFeatureSpecStaleCondition_UnclosedStory_NoArchiveYet_NoOperationalError(t *testing.T) {
	root := t.TempDir()
	feature := featureStaleTestSpec("spec/my-feature", "ac-1")
	writeFeatureStaleDeviationReport(t, root, store.ZoneActive, "my-feature", "  - { id: computed-x, kind: computed, text: unrelated, disposition: fixed }\n", "")

	stories := []implementingStoryEdges{{SpecRef: "spec/still-open", Closed: false}}
	manifest := &store.Manifest{Audit: &store.AuditConfig{DeviationsStaleThreshold: 3}}

	cond, err := checkFeatureSpecStaleCondition(root, feature, manifest, stories, nil)
	if err != nil {
		t.Fatalf("checkFeatureSpecStaleCondition: %v, want no operational error for an unclosed story", err)
	}
	if !cond.OK {
		t.Fatalf("cond = %+v, want PASS (nothing accepted-deviation anywhere)", cond)
	}
}

// TestCheckFeatureSpecStaleCondition_ClosedStoryMissingArchive_Disclosed is
// spec/finding-identity judged-feature-union-missing-archive-silent-zero's fix
// proof, now scoped (judged-feature-union-missing-archive-flag-shortcircuit) to
// the ONLY case a missing archive is still disclosed: the AVAILABLE data does
// NOT independently flag (here the feature's own report carries only a fixed
// finding). A CLOSED implementing story whose archived deviation report is
// absent is a store-integrity anomaly; its recorded accepted deviations would
// count exactly zero toward the feature-close union — the un-disclosed
// undercount three-valued honesty forbids. Because nothing in the available
// union flags, the partial union proves nothing either way, so the condition
// must DISCLOSE it (naming the story and the anomaly), never silently pass.
//
// Red-first (before the original silent-zero fix): the loop did
// `archived == nil -> continue` with no disclosure, so the missing archive
// silently contributed zero and the condition passed.
func TestCheckFeatureSpecStaleCondition_ClosedStoryMissingArchive_Disclosed(t *testing.T) {
	root := t.TempDir()
	feature := featureStaleTestSpec("spec/my-feature", "ac-1")
	// The feature's own report is present; the CLOSED implementing story has NO
	// archived report on disk.
	writeFeatureStaleDeviationReport(t, root, store.ZoneActive, "my-feature", "  - { id: computed-x, kind: computed, text: unrelated, disposition: fixed }\n", "")

	stories := []implementingStoryEdges{{SpecRef: "spec/my-story", Closed: true}}
	manifest := &store.Manifest{Audit: &store.AuditConfig{DeviationsStaleThreshold: 3}}

	cond, err := checkFeatureSpecStaleCondition(root, feature, manifest, stories, nil)
	if err != nil {
		t.Fatalf("checkFeatureSpecStaleCondition: %v", err)
	}
	if !cond.Disclosed {
		t.Fatalf("cond = %+v, want Disclosed — a CLOSED story missing its archive is a store-integrity anomaly, never a silent zero", cond)
	}
	if cond.OK {
		t.Fatalf("cond = %+v, want NOT OK (disclosed-unproven is never a silent pass)", cond)
	}
	if cond.Source == "" {
		t.Fatalf("cond = %+v, want a disclosure Source id", cond)
	}
	if !strings.Contains(cond.Reason, "spec/my-story") || !strings.Contains(cond.Reason, "anomaly") {
		t.Fatalf("cond.Reason = %q, want it to name the story and the store-integrity anomaly", cond.Reason)
	}
}

// TestCheckFeatureSpecStaleCondition_MissingArchive_OwnTextFlag_Fails is
// spec/finding-identity judged-feature-union-missing-archive-flag-shortcircuit's
// first fix proof: a feature whose OWN report already carries an own-text
// accepted-deviation (trigger a needs only the feature's own report — no
// archives at all) must FAIL the condition even when a closed implementing
// story's archive is missing. The missing archive can only ever UNDERCOUNT the
// budget, so the flag the AVAILABLE data already proves stands regardless — a
// proven violated-with-witness ranks ABOVE a disclosure (three-valued honesty).
//
// Red-first (before the fix): the missing-archive branch returned a
// non-blocking Disclosed condition BEFORE evidence.SpecStale was ever
// evaluated, so this provable own-text violation was demoted to
// disclosed-as-unproven and the feature closed anyway.
func TestCheckFeatureSpecStaleCondition_MissingArchive_OwnTextFlag_Fails(t *testing.T) {
	root := t.TempDir()
	feature := featureStaleTestSpec("spec/my-feature", "ac-1")
	// The feature's OWN report carries an own-text accepted-deviation (its id
	// equals the feature's own ac-1) — trigger (a), provable from the own
	// report alone.
	writeFeatureStaleDeviationReport(t, root, store.ZoneActive, "my-feature",
		"  - { id: ac-1, kind: computed, text: \"the feature's own ac-1 text was wrong\", disposition: accepted-deviation, note: owner-ratified }\n", "")
	// A CLOSED implementing story whose archived report is absent.
	stories := []implementingStoryEdges{{SpecRef: "spec/my-story", Closed: true}}
	manifest := &store.Manifest{Audit: &store.AuditConfig{DeviationsStaleThreshold: 3}}

	cond, err := checkFeatureSpecStaleCondition(root, feature, manifest, stories, nil)
	if err != nil {
		t.Fatalf("checkFeatureSpecStaleCondition: %v", err)
	}
	if cond.Disclosed {
		t.Fatalf("cond = %+v, want NOT Disclosed — a provable own-text violation must FAIL, never be demoted to a non-blocking disclosure by a missing archive", cond)
	}
	if cond.OK {
		t.Fatalf("cond = %+v, want FAIL — the own-text flag stands regardless of the missing input", cond)
	}
	if !strings.Contains(cond.Reason, "ac-1") {
		t.Fatalf("cond.Reason = %q, want it to name the own-text finding ac-1", cond.Reason)
	}
	if !strings.Contains(cond.Reason, "spec/my-story") || !strings.Contains(cond.Reason, "anomaly") {
		t.Fatalf("cond.Reason = %q, want the missing-archive anomaly noted in the failure reason (spec/my-story)", cond.Reason)
	}
}

// TestCheckFeatureSpecStaleCondition_MissingArchive_PartialUnionOverThreshold_Fails
// is the same finding's second fix proof: when the AVAILABLE union (the
// feature's own report plus the archives that ARE present) already exceeds the
// threshold, the condition must FAIL even though another closed story's archive
// is missing — the partial union is a strict lower bound (a missing archive can
// only add more), so restoring it can never clear a flag the partial data
// already raised.
//
// Red-first (before the fix): the missing archive short-circuited to a
// non-blocking Disclosed condition and the over-threshold budget never blocked
// closure.
func TestCheckFeatureSpecStaleCondition_MissingArchive_PartialUnionOverThreshold_Fails(t *testing.T) {
	root := t.TempDir()
	feature := featureStaleTestSpec("spec/my-feature", "ac-1")
	// One PRESENT closed-story archive already carrying 4 accepted-deviations
	// (> threshold 3) — the partial union alone is over budget.
	present := "  - { id: judged-a, kind: judged, text: t1, disposition: accepted-deviation, note: n1 }\n" +
		"  - { id: judged-b, kind: judged, text: t2, disposition: accepted-deviation, note: n2 }\n" +
		"  - { id: judged-c, kind: judged, text: t3, disposition: accepted-deviation, note: n3 }\n" +
		"  - { id: judged-d, kind: judged, text: t4, disposition: accepted-deviation, note: n4 }\n"
	writeFeatureStaleDeviationReport(t, root, store.ZoneArchive, "story-present", present, "")
	writeFeatureStaleDeviationReport(t, root, store.ZoneActive, "my-feature",
		"  - { id: computed-x, kind: computed, text: unrelated, disposition: fixed }\n", "")

	stories := []implementingStoryEdges{
		{SpecRef: "spec/story-present", Closed: true},
		{SpecRef: "spec/story-missing", Closed: true}, // archive absent
	}
	manifest := &store.Manifest{Audit: &store.AuditConfig{DeviationsStaleThreshold: 3}}

	cond, err := checkFeatureSpecStaleCondition(root, feature, manifest, stories, nil)
	if err != nil {
		t.Fatalf("checkFeatureSpecStaleCondition: %v", err)
	}
	if cond.Disclosed {
		t.Fatalf("cond = %+v, want NOT Disclosed — a partial union already over threshold must FAIL, not be demoted by a missing archive", cond)
	}
	if cond.OK {
		t.Fatalf("cond = %+v, want FAIL — the partial union (a lower bound) already exceeds the threshold", cond)
	}
	if !strings.Contains(cond.Reason, "count 4") {
		t.Fatalf("cond.Reason = %q, want it to name accepted-deviation count 4", cond.Reason)
	}
	if !strings.Contains(cond.Reason, "spec/story-missing") || !strings.Contains(cond.Reason, "anomaly") {
		t.Fatalf("cond.Reason = %q, want the missing-archive anomaly noted in the failure reason (spec/story-missing)", cond.Reason)
	}
}

// TestCheckFeatureSpecStaleCondition_TallyPrintsOnPass pins the second half of
// the fix: the storiesUnioned tally rides the condition on the PASS path too
// (previously it appeared only inside the FAIL reason), so a passing
// feature-close gate shows how many archives actually fed the union.
func TestCheckFeatureSpecStaleCondition_TallyPrintsOnPass(t *testing.T) {
	root := t.TempDir()
	feature := featureStaleTestSpec("spec/my-feature", "ac-1")
	// Two closed stories WITH archives (one accepted-deviation total, under
	// threshold) plus the feature's own report — a clean PASS.
	writeFeatureStaleDeviationReport(t, root, store.ZoneArchive, "story-a", "  - { id: judged-a, kind: judged, text: t1, disposition: accepted-deviation, note: n1 }\n", "")
	writeFeatureStaleDeviationReport(t, root, store.ZoneArchive, "story-b", "  - { id: computed-y, kind: computed, text: y, disposition: fixed }\n", "")
	writeFeatureStaleDeviationReport(t, root, store.ZoneActive, "my-feature", "  - { id: computed-x, kind: computed, text: unrelated, disposition: fixed }\n", "")

	stories := []implementingStoryEdges{
		{SpecRef: "spec/story-a", Closed: true},
		{SpecRef: "spec/story-b", Closed: true},
	}
	manifest := &store.Manifest{Audit: &store.AuditConfig{DeviationsStaleThreshold: 3}}

	cond, err := checkFeatureSpecStaleCondition(root, feature, manifest, stories, nil)
	if err != nil {
		t.Fatalf("checkFeatureSpecStaleCondition: %v", err)
	}
	if !cond.OK {
		t.Fatalf("cond = %+v, want PASS (1 accepted-deviation, threshold 3)", cond)
	}
	joined := strings.Join(cond.Extra, "\n")
	if !strings.Contains(joined, "union over the feature's own report + 2 closed implementing story archive(s)") {
		t.Fatalf("cond.Extra = %q, want the union tally printed on the PASS path", cond.Extra)
	}
}

// TestCheckFeatureSpecStaleCondition_NoReportsAnywhere_TriviallyUnflagged
// proves the absent-report base case (mirroring checkSpecStaleCondition's
// own "a story with no build activity yet cannot be spec-stale"): no
// feature report, no story archives at all — trivially unflagged, no error.
func TestCheckFeatureSpecStaleCondition_NoReportsAnywhere_TriviallyUnflagged(t *testing.T) {
	root := t.TempDir()
	feature := featureStaleTestSpec("spec/my-feature", "ac-1")

	cond, err := checkFeatureSpecStaleCondition(root, feature, nil, nil, nil)
	if err != nil {
		t.Fatalf("checkFeatureSpecStaleCondition: %v", err)
	}
	if !cond.OK {
		t.Fatalf("cond = %+v, want PASS", cond)
	}
}
