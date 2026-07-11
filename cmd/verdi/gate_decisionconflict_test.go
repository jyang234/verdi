package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeDecisionConflictReport writes decision-conflict-report.md directly
// to the working tree, mirroring gate_test.go's writeGateReport for the
// build-branch deviation report.
func writeDecisionConflictReport(t *testing.T, root, covers, findingsYAML string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", "stale-decline")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := "---\nschema: verdi.decisionconflict/v1\ncovers: " + covers + "\nfindings:\n" + findingsYAML + "digest: sha256:" + repeatZero(64) + "\n---\n# Decision-conflict report\n"
	if err := os.WriteFile(filepath.Join(dir, "decision-conflict-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing decision-conflict-report.md: %v", err)
	}
}

func repeatZero(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = '0'
	}
	return string(b)
}

const gdcHeadCommit = "0000000000000000000000000000000000000c"

func TestCheckDeclaredDecisionConflicts_NoReport(t *testing.T) {
	root := t.TempDir()
	cond, err := checkDeclaredDecisionConflicts(root, "stale-decline", gdcHeadCommit)
	if err != nil {
		t.Fatalf("checkDeclaredDecisionConflicts: %v", err)
	}
	if cond.OK {
		t.Fatal("OK = true, want false (no report at all)")
	}
}

func TestCheckDeclaredDecisionConflicts_StaleCovers(t *testing.T) {
	root := t.TempDir()
	writeDecisionConflictReport(t, root, "0000000000000000000000000000000000000b",
		"  - { id: f-1, kind: computed, text: t, disposition: exempt, note: n }\n")
	cond, err := checkDeclaredDecisionConflicts(root, "stale-decline", gdcHeadCommit)
	if err != nil {
		t.Fatalf("checkDeclaredDecisionConflicts: %v", err)
	}
	if cond.OK {
		t.Fatal("OK = true, want false (stale covers)")
	}
}

func TestCheckDeclaredDecisionConflicts_UndispositionedFails(t *testing.T) {
	root := t.TempDir()
	writeDecisionConflictReport(t, root, gdcHeadCommit,
		"  - { id: f-1, kind: computed, text: t }\n")
	cond, err := checkDeclaredDecisionConflicts(root, "stale-decline", gdcHeadCommit)
	if err != nil {
		t.Fatalf("checkDeclaredDecisionConflicts: %v", err)
	}
	if cond.OK {
		t.Fatal("OK = true, want false (undispositioned finding — unresolved declared edge)")
	}
}

func TestCheckDeclaredDecisionConflicts_AllResolvedPasses(t *testing.T) {
	root := t.TempDir()
	writeDecisionConflictReport(t, root, gdcHeadCommit,
		"  - { id: f-1, kind: computed, text: t, disposition: exempt, note: n }\n  - { id: f-2, kind: judged, text: t2, disposition: no-conflict, note: n2 }\n")
	cond, err := checkDeclaredDecisionConflicts(root, "stale-decline", gdcHeadCommit)
	if err != nil {
		t.Fatalf("checkDeclaredDecisionConflicts: %v", err)
	}
	if !cond.OK {
		t.Fatalf("OK = false (%s), want true (every declared edge resolved, every judged finding dispositioned)", cond.Reason)
	}
}
