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
