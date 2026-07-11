package decisionsweep

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/verdi/internal/lint"
)

func writeFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func adrMD(name, status string) string {
	extra := ""
	if status == "accepted" {
		extra = "decided: 2026-01-01\nfrozen: { at: 2026-01-01, commit: 3e91ab2 }\n"
	}
	return "---\nid: adr/" + name + "\nkind: adr\ntitle: \"" + name + "\"\nstatus: " + status + "\nowners: [platform-team]\n" + extra + "---\nbody\n"
}

// componentSpecWithExempts writes a minimal component-class spec carrying
// one decision object with an `exempts` link against adrRef.
func componentSpecWithExempts(name, decisionID, adrRef, reason string) string {
	noteField := ""
	if reason != "" {
		noteField = ", note: \"" + reason + "\""
	}
	return "---\nid: spec/" + name + "\nkind: spec\ntitle: \"" + name + "\"\nclass: component\nstatus: draft\nowners: [platform-team]\n" +
		"decisions:\n  - { id: " + decisionID + ", text: \"some decision\", anchor: \"#" + decisionID + "\", links: [ { type: exempts, ref: " + adrRef + noteField + " } ] }\n" +
		"---\nbody\n"
}

func buildSnapshot(t *testing.T, root string) *lint.Snapshot {
	t.Helper()
	writeFile(t, root, ".verdi/verdi.yaml", "schema: verdi.layout/v1\n")
	snap, err := lint.BuildSnapshot(root, lint.Options{})
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	return snap
}

func TestScanExemptions_CountsLiveExemptsEdges(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".verdi/adr/retry-policy.md", adrMD("retry-policy", "accepted"))
	writeFile(t, root, ".verdi/specs/active/spec-a/spec.md", componentSpecWithExempts("spec-a", "dc-1", "adr/retry-policy", "reason A"))
	writeFile(t, root, ".verdi/specs/active/spec-b/spec.md", componentSpecWithExempts("spec-b", "dc-1", "adr/retry-policy", "reason B"))

	snap := buildSnapshot(t, root)
	counts := ScanExemptions(snap)

	c, ok := counts["adr/retry-policy"]
	if !ok {
		t.Fatal(`counts["adr/retry-policy"] missing`)
	}
	if c.Count() != 2 {
		t.Fatalf("Count() = %d, want 2", c.Count())
	}
	if len(c.Owners) != 1 || c.Owners[0] != "platform-team" {
		t.Fatalf("Owners = %v, want [platform-team]", c.Owners)
	}
	if c.Sources[0].SpecRef != "spec/spec-a" || c.Sources[1].SpecRef != "spec/spec-b" {
		t.Fatalf("Sources = %+v, want sorted by SpecRef", c.Sources)
	}
}

// TestScanExemptions_DanglingTargetExcluded proves an exempts edge naming
// an ADR that does not actually exist in the corpus is not counted — VL-003
// already flags it as dangling elsewhere; this audit counts live,
// resolvable exemptions only.
func TestScanExemptions_DanglingTargetExcluded(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".verdi/specs/active/spec-a/spec.md", componentSpecWithExempts("spec-a", "dc-1", "adr/does-not-exist", "reason"))

	snap := buildSnapshot(t, root)
	counts := ScanExemptions(snap)
	if len(counts) != 0 {
		t.Fatalf("counts = %+v, want empty (dangling target excluded)", counts)
	}
}

func TestScanExemptions_NoExemptsEdgesAtAll(t *testing.T) {
	root := t.TempDir()
	snap := buildSnapshot(t, root)
	counts := ScanExemptions(snap)
	if len(counts) != 0 {
		t.Fatalf("counts = %+v, want empty", counts)
	}
}
