package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/store"
)

// TestReportClosureGateConditions_FeatureUsesStructuredOutcome pins the one
// reporting loop shared by story and feature closure conditions. Rendered
// disclosure detail is counted, feature-only informational Extra lines are
// not, and an ordinary failure retains the existing not-ready semantics.
func TestReportClosureGateConditions_FeatureUsesStructuredOutcome(t *testing.T) {
	tests := []struct {
		name            string
		conditions      []gateCondition
		wantReady       bool
		wantDisclosures int
	}{
		{
			name: "condition and per-record disclosures preserve ready",
			conditions: []gateCondition{
				{
					Name: "1. every feature AC evidenced", OK: true,
					Extra: []string{
						disclosure.Render(disclosure.New("gate:evidence-quarantine", "ac-1", "record excluded")),
						"       [union tally is informational, not disclosed-unproven]",
					},
				},
				{Name: "4. no unresolved spec-stale flag", Disclosed: true, Source: "gate:spec-stale-feature-union", Reason: "archive unavailable"},
			},
			wantReady:       true,
			wantDisclosures: 2,
		},
		{
			name:            "failure remains not ready",
			conditions:      []gateCondition{{Name: "1. every feature AC evidenced", Reason: "ac-1 pending"}},
			wantReady:       false,
			wantDisclosures: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			outcome := reportClosureGateConditions(&stdout, "closure(feature): ", tc.conditions)
			if outcome.Ready != tc.wantReady || outcome.Disclosures != tc.wantDisclosures {
				t.Fatalf("outcome = %+v, want Ready=%v Disclosures=%d; stdout=%s", outcome, tc.wantReady, tc.wantDisclosures, stdout.String())
			}
		})
	}
}

// nDistinctADFindings renders findingsYAML for n judged accepted-deviation
// findings with globally-unique id+text (so each is a distinct budget
// identity, align.Identity) — the L-N14 report-scaled-threshold tests need to
// place a controlled number of accepted deviations across the union.
func nDistinctADFindings(prefix string, n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "  - { id: judged-%s-%d, kind: judged, text: \"deviation %s %d\", disposition: accepted-deviation, note: n }\n", prefix, i, prefix, i)
	}
	return b.String()
}

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
	// Threshold 1 per report: the union basis here is 2 reports (the feature's own
	// + one story archive), so the L-N14 report-scaled effective threshold is 2,
	// which the archive's 4 accepted-deviations must still exceed — the never-zero
	// property is that they COUNT (4), not that any fixed flat threshold fires.
	manifest := &store.Manifest{Audit: &store.AuditConfig{DeviationsStaleThreshold: 1}}

	cond, err := checkFeatureSpecStaleCondition(root, feature, manifest, stories, nil, nil)
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

	cond, err := checkFeatureSpecStaleCondition(root, feature, manifest, stories, nil, nil)
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

	cond, err := checkFeatureSpecStaleCondition(root, feature, manifest, stories, nil, nil)
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

	cond, err := checkFeatureSpecStaleCondition(root, feature, manifest, stories, nil, nil)
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

	cond, err := checkFeatureSpecStaleCondition(root, feature, manifest, stories, nil, nil)
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
	// Threshold 1 per report: the AVAILABLE union spans 2 reports (the feature's
	// own + the one present story archive; the missing archive contributes no
	// report to the basis), so the L-N14 scaled effective threshold is 2 and the
	// present archive's 4 accepted-deviations already exceed it — a strict lower
	// bound that a restored archive can only raise.
	manifest := &store.Manifest{Audit: &store.AuditConfig{DeviationsStaleThreshold: 1}}

	cond, err := checkFeatureSpecStaleCondition(root, feature, manifest, stories, nil, nil)
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

	cond, err := checkFeatureSpecStaleCondition(root, feature, manifest, stories, nil, nil)
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

	cond, err := checkFeatureSpecStaleCondition(root, feature, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("checkFeatureSpecStaleCondition: %v", err)
	}
	if !cond.OK {
		t.Fatalf("cond = %+v, want PASS", cond)
	}
}

// TestCheckFeatureSpecStaleCondition_SupersededStory_DisclosedAndExcluded is
// spec/finding-identity judged-feature-union-superseded-story-archive's fix
// proof (ledger L-N12): a SUPERSEDED implementing story's archived
// accepted-deviations are EXCLUDED from the feature-close budget — supersession
// is the spec-stale budget's own prescribed remedy — but NEVER silently: the
// condition discloses a named line naming the story and the excluded count. Here
// the superseded story's archive carries 4 accepted-deviations (over threshold
// 3); excluded, the feature PASSES, and the exclusion is disclosed verbatim.
//
// Red-first (before the fix): discoverImplementingStories' flat view excludes
// superseded stories (D-16), so condition 4 never saw the archive — it silently
// contributed zero, with no disclosure and no ledger entry (ac-4's forbidden
// silent-zero shape, for the superseded case).
func TestCheckFeatureSpecStaleCondition_SupersededStory_DisclosedAndExcluded(t *testing.T) {
	root := t.TempDir()
	feature := featureStaleTestSpec("spec/my-feature", "ac-1")

	// The superseded story's ARCHIVED report carries 4 accepted-deviations.
	supersededFindings := "  - { id: judged-a, kind: judged, text: t1, disposition: accepted-deviation, note: n1 }\n" +
		"  - { id: judged-b, kind: judged, text: t2, disposition: accepted-deviation, note: n2 }\n" +
		"  - { id: judged-c, kind: judged, text: t3, disposition: accepted-deviation, note: n3 }\n" +
		"  - { id: judged-d, kind: judged, text: t4, disposition: accepted-deviation, note: n4 }\n"
	writeFeatureStaleDeviationReport(t, root, store.ZoneArchive, "superseded-story", supersededFindings, "")
	// The feature's own report reproduces none of them.
	writeFeatureStaleDeviationReport(t, root, store.ZoneActive, "my-feature", "  - { id: computed-x, kind: computed, text: unrelated, disposition: fixed }\n", "")

	manifest := &store.Manifest{Audit: &store.AuditConfig{DeviationsStaleThreshold: 3}}
	// The story is superseded, so discoverImplementingStories excludes it from the
	// flat `stories` view (nil here) and hands it back via the superseded set.
	cond, err := checkFeatureSpecStaleCondition(root, feature, manifest, nil, []string{"spec/superseded-story"}, nil)
	if err != nil {
		t.Fatalf("checkFeatureSpecStaleCondition: %v", err)
	}
	// Excluded from the budget -> the 4 accepted-deviations do NOT flag; PASS.
	if !cond.OK {
		t.Fatalf("cond = %+v, want PASS — a superseded story's archived deviations are excluded (supersession is the budget's own remedy)", cond)
	}
	// But NEVER silently: the named exclusion line rides the condition (Extra),
	// verbatim per L-N12.
	joined := strings.Join(cond.Extra, "\n")
	want := "superseded story spec/superseded-story's archived report (4 accepted-deviation(s)) excluded — supersession is the spec-stale budget's own prescribed remedy, taken"
	if !strings.Contains(joined, want) {
		t.Fatalf("cond.Extra = %q, want the verbatim L-N12 exclusion line %q", cond.Extra, want)
	}
}

// TestCheckFeatureSpecStaleCondition_SupersededStory_NoArchive_NoDisclosureNoError
// pins the neighbor: a superseded story with NO archived report has nothing to
// exclude and nothing to disclose (never a spurious "(0 accepted-deviation(s))"
// line), and never an operational error.
func TestCheckFeatureSpecStaleCondition_SupersededStory_NoArchive_NoDisclosureNoError(t *testing.T) {
	root := t.TempDir()
	feature := featureStaleTestSpec("spec/my-feature", "ac-1")
	writeFeatureStaleDeviationReport(t, root, store.ZoneActive, "my-feature", "  - { id: computed-x, kind: computed, text: unrelated, disposition: fixed }\n", "")

	manifest := &store.Manifest{Audit: &store.AuditConfig{DeviationsStaleThreshold: 3}}
	cond, err := checkFeatureSpecStaleCondition(root, feature, manifest, nil, []string{"spec/superseded-no-archive"}, nil)
	if err != nil {
		t.Fatalf("checkFeatureSpecStaleCondition: %v", err)
	}
	if !cond.OK {
		t.Fatalf("cond = %+v, want PASS", cond)
	}
	if strings.Contains(strings.Join(cond.Extra, "\n"), "superseded story") {
		t.Fatalf("cond.Extra = %q, want NO superseded-exclusion line (nothing was excluded)", cond.Extra)
	}
}

// writeSixReportFeature lays out the P2-11 union basis: the feature's own report
// plus five closed implementing story archives (six reports total), with adCount
// distinct accepted-deviations concentrated in the first story's archive and a
// single non-accepted finding in every other report so each archive exists and
// is counted as one report toward the union basis. Returns the stories slice for
// the feature gate. The number of reports (6) and the per-report threshold (6,
// set by the caller's manifest) are the L-N14 recalibration's own worked example.
func writeSixReportFeature(t *testing.T, root string, adCount int) []implementingStoryEdges {
	t.Helper()
	writeFeatureStaleDeviationReport(t, root, store.ZoneActive, "my-feature",
		"  - { id: computed-own, kind: computed, text: own-unrelated, disposition: fixed }\n", "")
	writeFeatureStaleDeviationReport(t, root, store.ZoneArchive, "story-1", nDistinctADFindings("s1", adCount), "")
	stories := []implementingStoryEdges{{SpecRef: "spec/story-1", Closed: true}}
	for i := 2; i <= 5; i++ {
		name := fmt.Sprintf("story-%d", i)
		writeFeatureStaleDeviationReport(t, root, store.ZoneArchive, name,
			fmt.Sprintf("  - { id: computed-%d, kind: computed, text: unrelated-%d, disposition: fixed }\n", i, i), "")
		stories = append(stories, implementingStoryEdges{SpecRef: "spec/" + name, Closed: true})
	}
	return stories
}

// TestCheckFeatureSpecStaleCondition_ReportScaledThreshold_Passes is P2-11's own
// worked example (ledger L-N14, owner-ratified 2026-07-21): 23 accepted-deviations
// unioned across a SIX-report basis (the feature's own report + five closed
// implementing story archives) with a per-report threshold of 6. Under the
// pre-union flat threshold this FAILED (23 > 6); the ratified recalibration
// preserves PER-REPORT density across the enlarged basis — effective threshold
// = 6 × 6 = 36 — so 23 PASSES, and the scaled arithmetic is printed on the PASS
// path (the tally rides every verdict).
//
// Red-first: against the pre-fix flat threshold this reds — 23 > 6 flags and the
// condition FAILs, so cond.OK is false where this test requires true.
func TestCheckFeatureSpecStaleCondition_ReportScaledThreshold_Passes(t *testing.T) {
	root := t.TempDir()
	feature := featureStaleTestSpec("spec/my-feature", "ac-1")
	stories := writeSixReportFeature(t, root, 23)
	manifest := &store.Manifest{Audit: &store.AuditConfig{DeviationsStaleThreshold: 6}}

	cond, err := checkFeatureSpecStaleCondition(root, feature, manifest, stories, nil, nil)
	if err != nil {
		t.Fatalf("checkFeatureSpecStaleCondition: %v", err)
	}
	if !cond.OK {
		t.Fatalf("cond = %+v, want PASS — 23 accepted-deviations over 6 reports is under the report-scaled threshold 36 (6 × 6)", cond)
	}
	joined := strings.Join(cond.Extra, "\n")
	if !strings.Contains(joined, "count 23") {
		t.Fatalf("cond.Extra = %q, want the accepted-deviation count 23 printed on the PASS path", cond.Extra)
	}
	if !strings.Contains(joined, "threshold 36 = 6 × 6 reports") {
		t.Fatalf("cond.Extra = %q, want the scaled arithmetic 'threshold 36 = 6 × 6 reports' printed on the PASS path", cond.Extra)
	}
}

// TestCheckFeatureSpecStaleCondition_ReportScaledThreshold_DenseStillFails is the
// recalibration's honesty guard (L-N14: "NOT a raising of the bar"): a genuinely
// dense feature — 40 accepted-deviations over the same six-report basis — still
// FAILs, because 40 exceeds the scaled threshold 36. The scaling preserves the
// per-report density that fires the counterweight; it never lets an over-dense
// feature through.
func TestCheckFeatureSpecStaleCondition_ReportScaledThreshold_DenseStillFails(t *testing.T) {
	root := t.TempDir()
	feature := featureStaleTestSpec("spec/my-feature", "ac-1")
	stories := writeSixReportFeature(t, root, 40)
	manifest := &store.Manifest{Audit: &store.AuditConfig{DeviationsStaleThreshold: 6}}

	cond, err := checkFeatureSpecStaleCondition(root, feature, manifest, stories, nil, nil)
	if err != nil {
		t.Fatalf("checkFeatureSpecStaleCondition: %v", err)
	}
	if cond.OK {
		t.Fatalf("cond = %+v, want FAIL — 40 accepted-deviations over 6 reports exceeds the scaled threshold 36", cond)
	}
	if !strings.Contains(cond.Reason, "count 40") {
		t.Fatalf("cond.Reason = %q, want it to name accepted-deviation count 40", cond.Reason)
	}
	if !strings.Contains(cond.Reason, "threshold 36 = 6 × 6 reports") {
		t.Fatalf("cond.Reason = %q, want the scaled arithmetic in the failure reason", cond.Reason)
	}
}
