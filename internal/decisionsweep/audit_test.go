package decisionsweep

import (
	"os"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
)

// testAuditNow is the fixed reference "now" every pre-existing Audit test
// above passes — none of them exercise waiver-expiry lapsing, so a fixed,
// deterministic instant (never time.Now()) keeps them reproducible exactly
// like every other test in this module. TestAudit_WaiverStale* below pass
// their own explicit, varied `now` values to exercise lapsing precisely.
var testAuditNow = time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)

func storySpecMD(name string, acFragments ...string) string {
	links := ""
	for _, f := range acFragments {
		links += "  - { type: implements, ref: spec/my-feature#" + f + " }\n"
	}
	linksBlock := ""
	if links != "" {
		linksBlock = "links:\n" + links
	}
	return "---\nid: spec/" + name + "\nkind: spec\ntitle: \"" + name + "\"\nclass: story\nstatus: draft\nowners: [platform-team]\n" +
		"story: jira:LOAN-1\nproblem: { text: \"p\", anchor: \"#p\" }\noutcome: { text: \"o\", anchor: \"#o\" }\n" +
		linksBlock + "---\nbody\n"
}

// storySpecMDWithOwnAC builds a story spec that declares its OWN
// acceptance_criteria block (ownACID) while implementing a DIFFERENT feature
// AC fragment (implementsFrag) — the general case where a story's own AC ids
// and the feature AC ids it implements do not coincide. Trigger (a) must key
// off the own AC id, never the implemented feature fragment.
func storySpecMDWithOwnAC(name, ownACID, implementsFrag string) string {
	return "---\nid: spec/" + name + "\nkind: spec\ntitle: \"" + name + "\"\nclass: story\nstatus: draft\nowners: [platform-team]\n" +
		"story: jira:LOAN-1\nproblem: { text: \"p\", anchor: \"#p\" }\noutcome: { text: \"o\", anchor: \"#o\" }\n" +
		"acceptance_criteria:\n  - { id: " + ownACID + ", text: \"own text\", evidence: [behavioral] }\n" +
		"links:\n  - { type: implements, ref: spec/my-feature#" + implementsFrag + " }\n" +
		"---\nbody\n"
}

func deviationReportMD(covers string, findings string) string {
	return "---\nschema: verdi.deviation/v1\ncovers: " + covers + "\nfindings:\n" + findings + "digest: sha256:" + decisionConflictTestHex + "\n---\nbody\n"
}

const decisionConflictTestHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// TestAudit_ExemptsConflictThreshold_AutoFilesConflictRecord is the exit
// criterion end to end: seeding audit.exempts_conflict_threshold: 3 and three
// exempts edges against one ADR, `Audit` auto-files a .verdi/conflicts/ record
// naming that ADR via challenges:.
//
// Named for the behavior it proves (not the bare "TestAudit_Exemption-
// ThresholdEndToEnd" it once shared with cmd/verdi's audit e2e), so the
// guide-claims-gate's bare-name witness identity for row 8.1 resolves
// unambiguously to the anchored cmd/verdi declaration
// (judged-ac2-witness-identity-is-bare-name-not-package-qualified).
func TestAudit_ExemptsConflictThreshold_AutoFilesConflictRecord(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".verdi/verdi.yaml", "schema: verdi.layout/v1\naudit:\n  exempts_conflict_threshold: 3\n  deviations_stale_threshold: 3\n")
	writeFile(t, root, ".verdi/adr/retry-policy.md", adrMD("retry-policy", "accepted"))
	writeFile(t, root, ".verdi/specs/active/spec-a/spec.md", componentSpecWithExempts("spec-a", "dc-1", "adr/retry-policy", "reason A"))
	writeFile(t, root, ".verdi/specs/active/spec-b/spec.md", componentSpecWithExempts("spec-b", "dc-1", "adr/retry-policy", "reason B"))
	writeFile(t, root, ".verdi/specs/active/spec-c/spec.md", componentSpecWithExempts("spec-c", "dc-1", "adr/retry-policy", "reason C"))

	result, err := Audit(root, 3, 3, 3, testAuditNow)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if len(result.Exemptions) != 1 || result.Exemptions[0].Count() != 3 {
		t.Fatalf("Exemptions = %+v, want one ADR with count 3", result.Exemptions)
	}
	if len(result.Filed) != 1 {
		t.Fatalf("Filed = %v, want exactly one auto-filed conflict", result.Filed)
	}

	data, err := os.ReadFile(result.Filed[0])
	if err != nil {
		t.Fatalf("reading filed conflict: %v", err)
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	decoded, err := artifact.DecodeConflict(fm)
	if err != nil {
		t.Fatalf("filed record does not decode as a valid conflict: %v", err)
	}
	hasChallenges := false
	for _, l := range decoded.Links {
		if l.Type == artifact.LinkChallenges && l.Ref == "adr/retry-policy" {
			hasChallenges = true
		}
	}
	if !hasChallenges {
		t.Fatalf("Links = %+v, want a challenges link naming adr/retry-policy", decoded.Links)
	}

	// Re-running must not duplicate the filing (idempotent).
	result2, err := Audit(root, 3, 3, 3, testAuditNow)
	if err != nil {
		t.Fatalf("Audit (second run): %v", err)
	}
	if len(result2.Filed) != 0 {
		t.Fatalf("Filed (second run) = %v, want none (idempotent)", result2.Filed)
	}
}

// TestAudit_SpecStaleSurfaced proves Audit also surfaces V1-P3's spec-stale
// count against deviations_stale_threshold for a story with an
// accepted-deviation-heavy deviation report.
func TestAudit_SpecStaleSurfaced(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".verdi/verdi.yaml", "schema: verdi.layout/v1\n")
	writeFile(t, root, ".verdi/specs/active/my-story/spec.md", storySpecMD("my-story", "ac-1"))
	findings := "  - { id: f-1, kind: judged, text: t1, disposition: accepted-deviation, note: n1 }\n" +
		"  - { id: f-2, kind: judged, text: t2, disposition: accepted-deviation, note: n2 }\n" +
		"  - { id: f-3, kind: judged, text: t3, disposition: accepted-deviation, note: n3 }\n" +
		"  - { id: f-4, kind: judged, text: t4, disposition: accepted-deviation, note: n4 }\n"
	writeFile(t, root, ".verdi/specs/active/my-story/deviation-report.md", deviationReportMD("7f3c2a1", findings))

	result, err := Audit(root, 3, 3, 3, testAuditNow)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if len(result.SpecStale) != 1 {
		t.Fatalf("SpecStale = %+v, want exactly 1 entry", result.SpecStale)
	}
	entry := result.SpecStale[0]
	if entry.StoryRef != "spec/my-story" {
		t.Fatalf("StoryRef = %q", entry.StoryRef)
	}
	if !entry.Result.Flagged || !entry.Result.TriggeredByThreshold {
		t.Fatalf("Result = %+v, want flagged via threshold (4 accepted-deviations > 3)", entry.Result)
	}
}

// TestAudit_SpecStaleOwnTextTrigger proves trigger (a): an
// accepted-deviation finding whose id equals one of the story's OWN declared
// AC ids flags the story spec-stale, keyed off the story's own
// acceptance_criteria block (matching the closure gate) — NOT off the feature
// AC fragment the story implements. The report also carries an
// accepted-deviation finding whose id equals the implemented FEATURE AC
// fragment; that one must NOT trigger (a), proving audit no longer joins on
// implements-fragment ids.
func TestAudit_SpecStaleOwnTextTrigger(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".verdi/verdi.yaml", "schema: verdi.layout/v1\n")
	writeFile(t, root, ".verdi/specs/active/own-story/spec.md",
		storySpecMDWithOwnAC("own-story", "ac-own", "ac-feat"))
	// Two accepted-deviations (count 2 < threshold 3, so trigger (b) is
	// silent): one keyed on the story's own AC id (must fire trigger (a)),
	// one keyed on the implemented feature AC fragment (must NOT).
	findings := "  - { id: ac-own, kind: judged, text: t1, disposition: accepted-deviation, note: n1 }\n" +
		"  - { id: ac-feat, kind: judged, text: t2, disposition: accepted-deviation, note: n2 }\n"
	writeFile(t, root, ".verdi/specs/active/own-story/deviation-report.md", deviationReportMD("7f3c2a1", findings))

	result, err := Audit(root, 3, 3, 3, testAuditNow)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if len(result.SpecStale) != 1 {
		t.Fatalf("SpecStale = %+v, want exactly 1 entry", result.SpecStale)
	}
	r := result.SpecStale[0].Result
	if !r.Flagged || r.TriggeredByThreshold {
		t.Fatalf("Result = %+v, want flagged via own-text trigger (a), not threshold", r)
	}
	if len(r.OwnTextFindingIDs) != 1 || r.OwnTextFindingIDs[0] != "ac-own" {
		t.Fatalf("OwnTextFindingIDs = %v, want exactly [ac-own] (own AC id, not the implemented feature fragment ac-feat)", r.OwnTextFindingIDs)
	}
}

// TestAudit_SpecStale_NotResurfacedCountsToo is spec/finding-identity ac-3's
// counterweight-hardening proof at the `verdi audit` seam (05 §Lenses' anti-
// hairball law: dex's spec-stale badge and `verdi audit` must compute the
// SAME way, ScanSpecStale's own doc comment) — mirroring
// TestRunClosureGate_SpecStaleCondition's closure-gate proof of the same
// property: an accepted-deviation finding sitting ONLY in not-resurfaced:
// (never in findings:) still counts toward the threshold trigger, exactly
// as if it were still live — the X-18 laundering drain this story closes.
func TestAudit_SpecStale_NotResurfacedCountsToo(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".verdi/verdi.yaml", "schema: verdi.layout/v1\n")
	writeFile(t, root, ".verdi/specs/active/drained-story/spec.md", storySpecMD("drained-story", "ac-1"))
	report := "---\nschema: verdi.deviation/v1\ncovers: 7f3c2a1\nfindings: []\n" +
		"not-resurfaced:\n" +
		"  - { id: judged-f1, kind: judged, text: t1, disposition: accepted-deviation, note: n1 }\n" +
		"  - { id: judged-f2, kind: judged, text: t2, disposition: accepted-deviation, note: n2 }\n" +
		"  - { id: judged-f3, kind: judged, text: t3, disposition: accepted-deviation, note: n3 }\n" +
		"  - { id: judged-f4, kind: judged, text: t4, disposition: accepted-deviation, note: n4 }\n" +
		"digest: sha256:" + decisionConflictTestHex + "\n---\nbody\n"
	writeFile(t, root, ".verdi/specs/active/drained-story/deviation-report.md", report)

	result, err := Audit(root, 3, 3, 3, testAuditNow)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if len(result.SpecStale) != 1 {
		t.Fatalf("SpecStale = %+v, want exactly 1 entry", result.SpecStale)
	}
	entry := result.SpecStale[0]
	if !entry.Result.Flagged || !entry.Result.TriggeredByThreshold {
		t.Fatalf("Result = %+v, want flagged via threshold — 4 accepted-deviations living in not-resurfaced: must still count", entry.Result)
	}
	if entry.Result.AcceptedDeviationCount != 4 {
		t.Fatalf("AcceptedDeviationCount = %d, want 4", entry.Result.AcceptedDeviationCount)
	}
}

// TestAudit_StoryWithNoDeviationReportSkipped proves a story that was
// never `align`-ed (no deviation-report.md yet) is skipped, not flagged.
func TestAudit_StoryWithNoDeviationReportSkipped(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".verdi/verdi.yaml", "schema: verdi.layout/v1\n")
	writeFile(t, root, ".verdi/specs/active/my-story/spec.md", storySpecMD("my-story"))

	result, err := Audit(root, 3, 3, 3, testAuditNow)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if len(result.SpecStale) != 0 {
		t.Fatalf("SpecStale = %+v, want none (no deviation report yet)", result.SpecStale)
	}
}

func TestAudit_Negative_NoVerdiDir(t *testing.T) {
	if _, err := Audit(t.TempDir(), 3, 3, 3, testAuditNow); err == nil {
		t.Fatal("Audit(no .verdi dir): want error, got nil")
	}
}

func TestAudit_EmptyCorpusNoOp(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".verdi/verdi.yaml", "schema: verdi.layout/v1\n")
	result, err := Audit(root, 3, 3, 3, testAuditNow)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if len(result.Exemptions) != 0 || len(result.Filed) != 0 || len(result.SpecStale) != 0 {
		t.Fatalf("result = %+v, want all empty", result)
	}
}
