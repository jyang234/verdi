package align

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
)

// writeADR writes .verdi/adr/<name>.md with the given status (proposed,
// accepted, or superseded), a minimal but Validate-legal ADR document.
func writeADR(t *testing.T, root, name, status string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "adr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var extra string
	switch status {
	case "accepted", "superseded":
		extra = "decided: 2026-01-01\nfrozen: { at: 2026-01-01, commit: 3e91ab2 }\n"
	}
	content := "---\nid: adr/" + name + "\nkind: adr\ntitle: \"" + name + "\"\nstatus: " + status + "\nowners: [platform-team]\n" + extra + "---\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeActiveSpec writes .verdi/specs/active/<name>/spec.md, a minimal but
// Validate-legal component-class spec (the simplest class that legally
// carries no acceptance criteria/problem/outcome requirements) with one
// decision object of the given id — enough to be a legal supersedes/exempts
// target (a decision fragment) for another spec's declared edges.
func writeActiveSpec(t *testing.T, root, name, decisionID string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nid: spec/" + name + "\nkind: spec\ntitle: \"" + name + "\"\nclass: component\nstatus: draft\nowners: [platform-team]\n" +
		"decisions:\n  - { id: " + decisionID + ", text: \"some decision\", anchor: \"#" + decisionID + "\" }\n" +
		"---\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func decisionSpecWithLinks(links []artifact.Link) *artifact.SpecFrontmatter {
	return &artifact.SpecFrontmatter{
		Decisions: []artifact.Decision{
			{ID: "dc-1", Text: "some decision", Anchor: "#dc-1", Links: links},
		},
	}
}

func TestComputeDecisionEdges_ExemptsResolvedWithReason(t *testing.T) {
	root := t.TempDir()
	writeADR(t, root, "retry-policy", "accepted")

	spec := decisionSpecWithLinks([]artifact.Link{
		{Type: artifact.LinkExempts, Ref: "adr/retry-policy", Note: "documented exception"},
	})
	findings, err := ComputeDecisionEdges(root, spec)
	if err != nil {
		t.Fatalf("ComputeDecisionEdges: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want exactly 1", findings)
	}
	f := findings[0]
	if f.Kind != artifact.FindingComputed {
		t.Fatalf("Kind = %q, want computed", f.Kind)
	}
	if f.Disposition != artifact.ConflictExempt {
		t.Fatalf("Disposition = %q, want exempt (a reasoned exempts edge against an existing ADR resolves)", f.Disposition)
	}
	if f.TargetRef != "adr/retry-policy" {
		t.Fatalf("TargetRef = %q, want adr/retry-policy", f.TargetRef)
	}
	if err := f.Validate(); err != nil {
		t.Fatalf("finding failed its own Validate: %v", err)
	}
}

func TestComputeDecisionEdges_ExemptsWithoutReasonUnresolved(t *testing.T) {
	root := t.TempDir()
	writeADR(t, root, "retry-policy", "accepted")

	spec := decisionSpecWithLinks([]artifact.Link{
		{Type: artifact.LinkExempts, Ref: "adr/retry-policy"}, // no Note
	})
	findings, err := ComputeDecisionEdges(root, spec)
	if err != nil {
		t.Fatalf("ComputeDecisionEdges: %v", err)
	}
	if len(findings) != 1 || findings[0].Dispositioned() {
		t.Fatalf("findings = %+v, want one undispositioned finding (exempts requires a reason)", findings)
	}
}

func TestComputeDecisionEdges_SupersedesResolvedWhenTargetADRSuperseded(t *testing.T) {
	root := t.TempDir()
	writeADR(t, root, "old-policy", "superseded")

	spec := decisionSpecWithLinks([]artifact.Link{
		{Type: artifact.LinkSupersedes, Ref: "adr/old-policy"},
	})
	findings, err := ComputeDecisionEdges(root, spec)
	if err != nil {
		t.Fatalf("ComputeDecisionEdges: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want exactly 1", findings)
	}
	if findings[0].Disposition != artifact.ConflictSuperseded {
		t.Fatalf("Disposition = %q, want superseded", findings[0].Disposition)
	}
}

func TestComputeDecisionEdges_SupersedesUnresolvedWhenTargetADRNotYetSuperseded(t *testing.T) {
	root := t.TempDir()
	writeADR(t, root, "current-policy", "accepted")

	spec := decisionSpecWithLinks([]artifact.Link{
		{Type: artifact.LinkSupersedes, Ref: "adr/current-policy"},
	})
	findings, err := ComputeDecisionEdges(root, spec)
	if err != nil {
		t.Fatalf("ComputeDecisionEdges: %v", err)
	}
	if len(findings) != 1 || findings[0].Dispositioned() {
		t.Fatalf("findings = %+v, want one undispositioned finding (target ADR not yet superseded)", findings)
	}
}

// TestComputeDecisionEdges_DanglingFailsClosed is the exit criterion's
// dangling-edge case: a declared edge naming a ref that resolves to no
// document in the corpus at all must fail closed (unresolved), never be
// silently skipped or treated as resolved.
func TestComputeDecisionEdges_DanglingFailsClosed(t *testing.T) {
	root := t.TempDir() // no ADRs or specs written at all

	spec := decisionSpecWithLinks([]artifact.Link{
		{Type: artifact.LinkSupersedes, Ref: "adr/does-not-exist"},
		{Type: artifact.LinkExempts, Ref: "adr/also-missing", Note: "reason present but target dangling"},
	})
	findings, err := ComputeDecisionEdges(root, spec)
	if err != nil {
		t.Fatalf("ComputeDecisionEdges: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("findings = %+v, want exactly 2", findings)
	}
	for _, f := range findings {
		if f.Dispositioned() {
			t.Fatalf("finding %+v: dangling edge must never resolve, even with a reason present", f)
		}
	}
}

// TestComputeDecisionEdges_SupersedesTargetingDecisionFragmentUnresolved
// proves the disclosed scope limitation: a supersedes edge targeting a
// spec-scoped decision (not an ADR) never computed-resolves, since Decision
// carries no independent status field.
func TestComputeDecisionEdges_SupersedesTargetingDecisionFragmentUnresolved(t *testing.T) {
	root := t.TempDir()
	writeActiveSpec(t, root, "other-feature", "dc-9")

	spec := decisionSpecWithLinks([]artifact.Link{
		{Type: artifact.LinkSupersedes, Ref: "spec/other-feature#dc-9"},
	})
	findings, err := ComputeDecisionEdges(root, spec)
	if err != nil {
		t.Fatalf("ComputeDecisionEdges: %v", err)
	}
	if len(findings) != 1 || findings[0].Dispositioned() {
		t.Fatalf("findings = %+v, want one undispositioned finding (non-ADR supersedes target)", findings)
	}
}

// TestComputeDecisionEdges_NoDeclaredEdges proves a spec with no
// supersedes/exempts links produces no findings at all.
func TestComputeDecisionEdges_NoDeclaredEdges(t *testing.T) {
	root := t.TempDir()
	spec := &artifact.SpecFrontmatter{
		Decisions: []artifact.Decision{{ID: "dc-1", Text: "t", Anchor: "#dc-1"}},
	}
	findings, err := ComputeDecisionEdges(root, spec)
	if err != nil {
		t.Fatalf("ComputeDecisionEdges: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("findings = %+v, want none", findings)
	}
}

func TestComputeDecisionEdges_Negative_NilSpec(t *testing.T) {
	if _, err := ComputeDecisionEdges(t.TempDir(), nil); err == nil {
		t.Fatal("ComputeDecisionEdges(nil spec): want error, got nil")
	}
}

func TestComputeDecisionEdges_Negative_EmptyRoot(t *testing.T) {
	if _, err := ComputeDecisionEdges("", decisionSpecWithLinks(nil)); err == nil {
		t.Fatal("ComputeDecisionEdges(empty root): want error, got nil")
	}
}
