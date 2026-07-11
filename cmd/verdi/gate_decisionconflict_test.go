package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/fixturegit"
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

// buildDesignGateRepo builds a fixturegit repo whose spec (a draft feature
// spec — a legitimate design-branch spec, ResolveDesignSpec accepts feature
// and story class alike) lives at specs/active/stale-decline, then checks
// out the design/stale-decline branch `verdi design start` would have cut.
func buildDesignGateRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                         "schema: verdi.layout/v1\nforge: gitlab\n",
				".verdi/specs/active/stale-decline/spec.md": gateSpecMD("draft"),
			},
			Message: "scaffold + draft design spec",
		},
	})
	checkoutBranch(t, repo.Dir, "design/stale-decline")
	return repo
}

// TestSpecMRGate_DanglingExemptsFails proves the spec-MR path fails the gate
// (exit 1, naming the declared-decision-conflict condition) when the
// decision-conflict report carries a dangling declared edge — here an
// undispositioned computed finding standing for an unresolved `exempts` edge.
func TestSpecMRGate_DanglingExemptsFails(t *testing.T) {
	repo := buildDesignGateRepo(t)
	writeDecisionConflictReport(t, repo.Dir, repo.Head,
		"  - { id: f-1, kind: computed, text: \"exempts edge to adr/decline-policy is unresolved\" }\n")

	var stdout, stderr bytes.Buffer
	got := runSpecMRGate(context.Background(), repo.Dir, "design/stale-decline", nil, "main", &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runSpecMRGate = %d, want 1; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "declared decision conflicts") {
		t.Fatalf("stdout = %q, want it to name the declared-decision-conflict condition", stdout.String())
	}
	if !strings.Contains(stdout.String(), "gate: FAIL") {
		t.Fatalf("stdout = %q, want a final gate: FAIL line", stdout.String())
	}
}

// TestSpecMRGate_ResolvedPasses proves the same path passes (exit 0) once
// every declared edge is resolved (its finding dispositioned) — with a
// nil forge, the review-thread condition (gate_threads.go) discloses
// unproven (a printed [NOTICE], never a silent pass — constitution 2/10)
// rather than either failing the gate or being silently skipped.
func TestSpecMRGate_ResolvedPasses(t *testing.T) {
	repo := buildDesignGateRepo(t)
	writeDecisionConflictReport(t, repo.Dir, repo.Head,
		"  - { id: f-1, kind: computed, text: \"exempts edge to adr/decline-policy\", disposition: exempt, note: \"excused, see witness\" }\n")

	var stdout, stderr bytes.Buffer
	got := runSpecMRGate(context.Background(), repo.Dir, "design/stale-decline", nil, "main", &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runSpecMRGate = %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "gate: PASS") {
		t.Fatalf("stdout = %q, want a final gate: PASS line", stdout.String())
	}
	if !strings.Contains(stdout.String(), "[NOTICE]") || !strings.Contains(stdout.String(), "review threads resolved") {
		t.Fatalf("stdout = %q, want a [NOTICE] disclosing the review-thread condition unproven (nil forge)", stdout.String())
	}
}

// TestCmdGate_SpecMR_EntryPoint drives the real `verdi gate` entry point on a
// design branch, proving cmdGate's branch-prefix dispatch into the spec-MR
// path works end to end (not just runSpecMRGate's core): a resolved report
// exits 0.
func TestCmdGate_SpecMR_EntryPoint(t *testing.T) {
	repo := buildDesignGateRepo(t)
	writeDecisionConflictReport(t, repo.Dir, repo.Head,
		"  - { id: f-1, kind: computed, text: t, disposition: exempt, note: n }\n")
	t.Chdir(repo.Dir)

	var stderr bytes.Buffer
	got := run([]string{"gate"}, &stderr)
	if got != 0 {
		t.Fatalf("run([gate]) on a design branch = %d, want 0; stderr=%s", got, stderr.String())
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
