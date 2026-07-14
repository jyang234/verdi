package decisionsweep

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

func threeExemptionCount(adrRef string) map[string]*ExemptionCount {
	return map[string]*ExemptionCount{
		adrRef: {
			ADRRef: adrRef,
			Owners: []string{"platform-team"},
			Sources: []ExemptSource{
				{SpecRef: "spec/spec-a", DecisionID: "dc-1", Reason: "reason A"},
				{SpecRef: "spec/spec-b", DecisionID: "dc-1", Reason: "reason B"},
				{SpecRef: "spec/spec-c", DecisionID: "dc-1", Reason: "reason C"},
			},
		},
	}
}

// TestPlanAutoFilings_ThresholdCrossed is the exit criterion's exact case:
// seeding audit.exempts_conflict_threshold: 3 and three exempts edges
// against one ADR auto-files a conflict record naming that ADR via
// challenges:.
func TestPlanAutoFilings_ThresholdCrossed(t *testing.T) {
	root := t.TempDir()
	counts := threeExemptionCount("adr/retry-policy")

	filings, err := PlanAutoFilings(root, counts, 3)
	if err != nil {
		t.Fatalf("PlanAutoFilings: %v", err)
	}
	if len(filings) != 1 {
		t.Fatalf("filings = %+v, want exactly 1", filings)
	}
	f := filings[0]
	if f.ADRRef != "adr/retry-policy" {
		t.Fatalf("ADRRef = %q", f.ADRRef)
	}

	fm, body, err := artifact.SplitFrontmatter(f.Content)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	decoded, err := artifact.DecodeConflict(fm)
	if err != nil {
		t.Fatalf("the auto-filed record must decode as a valid conflict artifact: %v\ncontent:\n%s", err, f.Content)
	}
	if decoded.Status != "open" {
		t.Fatalf("Status = %q, want open", decoded.Status)
	}
	found := false
	for _, l := range decoded.Links {
		if l.Type == artifact.LinkChallenges && l.Ref == "adr/retry-policy" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Links = %+v, want a challenges link naming adr/retry-policy", decoded.Links)
	}
	if len(body) == 0 {
		t.Fatal("empty body")
	}
}

// TestPlanAutoFilings_BelowThresholdNotFiled proves 2 exemptions against a
// threshold of 3 never files.
func TestPlanAutoFilings_BelowThresholdNotFiled(t *testing.T) {
	root := t.TempDir()
	counts := threeExemptionCount("adr/retry-policy")
	counts["adr/retry-policy"].Sources = counts["adr/retry-policy"].Sources[:2]

	filings, err := PlanAutoFilings(root, counts, 3)
	if err != nil {
		t.Fatalf("PlanAutoFilings: %v", err)
	}
	if len(filings) != 0 {
		t.Fatalf("filings = %+v, want none (below threshold)", filings)
	}
}

// TestPlanAutoFilings_DefaultThreshold proves a zero/absent threshold falls
// back to DefaultExemptsConflictThreshold (3).
func TestPlanAutoFilings_DefaultThreshold(t *testing.T) {
	root := t.TempDir()
	counts := threeExemptionCount("adr/retry-policy")

	filings, err := PlanAutoFilings(root, counts, 0)
	if err != nil {
		t.Fatalf("PlanAutoFilings: %v", err)
	}
	if len(filings) != 1 {
		t.Fatalf("filings = %+v, want 1 (default threshold 3, exactly 3 sources)", filings)
	}
}

// TestPlanAutoFilings_Idempotent proves re-running PlanAutoFilings after a
// prior filing already landed on disk (at the deterministic, ADR-ref-keyed
// path) never re-files — the phase's binding idempotency requirement.
func TestPlanAutoFilings_Idempotent(t *testing.T) {
	root := t.TempDir()
	counts := threeExemptionCount("adr/retry-policy")

	first, err := PlanAutoFilings(root, counts, 3)
	if err != nil || len(first) != 1 {
		t.Fatalf("PlanAutoFilings (first): filings=%+v err=%v", first, err)
	}
	if _, err := WriteFilings(root, first); err != nil {
		t.Fatalf("WriteFilings: %v", err)
	}

	// Re-run with a LARGER count against the same ADR — still must not
	// re-file, since the key is the ADR ref alone, never the count.
	counts["adr/retry-policy"].Sources = append(counts["adr/retry-policy"].Sources,
		ExemptSource{SpecRef: "spec/spec-d", DecisionID: "dc-1", Reason: "reason D"})
	second, err := PlanAutoFilings(root, counts, 3)
	if err != nil {
		t.Fatalf("PlanAutoFilings (second): %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("filings (second run) = %+v, want none (idempotent — already filed)", second)
	}

	// Exactly one file must exist on disk.
	entries, err := os.ReadDir(filepath.Join(root, ".verdi", "conflicts"))
	if err != nil {
		t.Fatalf("reading conflicts dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("conflicts dir has %d entries, want exactly 1", len(entries))
	}
}

func TestPlanAutoFilings_MultipleADRsIndependent(t *testing.T) {
	root := t.TempDir()
	counts := threeExemptionCount("adr/retry-policy")
	counts["adr/other-policy"] = &ExemptionCount{
		ADRRef: "adr/other-policy",
		Owners: []string{"platform-team"},
		Sources: []ExemptSource{
			{SpecRef: "spec/spec-x", DecisionID: "dc-1", Reason: "r"},
		},
	}
	filings, err := PlanAutoFilings(root, counts, 3)
	if err != nil {
		t.Fatalf("PlanAutoFilings: %v", err)
	}
	if len(filings) != 1 || filings[0].ADRRef != "adr/retry-policy" {
		t.Fatalf("filings = %+v, want exactly the adr/retry-policy filing (adr/other-policy below threshold)", filings)
	}
}

func TestPlanAutoFilings_Negative_OwnersMissing(t *testing.T) {
	root := t.TempDir()
	counts := threeExemptionCount("adr/retry-policy")
	counts["adr/retry-policy"].Owners = nil
	if _, err := PlanAutoFilings(root, counts, 3); err == nil {
		t.Fatal("PlanAutoFilings with no owners: want error, got nil")
	}
}

func TestWriteFilings_WritesExactContent(t *testing.T) {
	root := t.TempDir()
	filings := []Filing{{ADRRef: "adr/x", RelPath: filepath.Join(".verdi", "conflicts", "exempts-threshold-x.md"), Content: []byte("---\nid: conflict/exempts-threshold-x\n---\nbody\n")}}
	written, err := WriteFilings(root, filings)
	if err != nil {
		t.Fatalf("WriteFilings: %v", err)
	}
	if len(written) != 1 {
		t.Fatalf("written = %v, want 1 path", written)
	}
	data, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(data) != string(filings[0].Content) {
		t.Fatalf("written content mismatch")
	}
}
