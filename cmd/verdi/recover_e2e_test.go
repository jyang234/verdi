// Built-binary end-to-end tests for `verdi recover` (spec/readiness-
// recovery-v2 ac-8..ac-10, Task 3's read path): the empty-branch-cut
// fixture drives the REAL compiled verdi binary as a real OS process
// (buildVerdiBinary/runVerdiBinary, mirroring document_parity_e2e_test.go
// and obligationseam_e2e_test.go's own convention) — never a package-
// internal call standing in for it — proving argument parsing, store
// resolution, the recovery.CommandLog observer wiring, and
// VERDI_RECOVERY_GITLOG's test-only observability end to end. The
// --apply stub case proves Task 3's exit-2 mapping for Task 4's own
// signature; per the plan's own Step 1 note, Task 4 deletes this case
// and replaces it with the real apply-protocol proof.
package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/recovery"
)

// recoverE2ERepo builds the same minimal spec/checkout fixture
// recover_test.go's own recoverFixtureStore does, for the real-binary
// tests below.
func recoverE2ERepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "")
	const checkoutSpecMD = `---
id: spec/checkout
kind: spec
class: feature
title: "Checkout"
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
---
# body
`
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                    "schema: verdi.layout/v1\nforge: gitlab\n",
				".verdi/specs/active/checkout/spec.md": checkoutSpecMD,
			},
			Message: "scaffold",
		},
	})
}

func TestRecoverE2E_EmptyBranchCut(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := recoverE2ERepo(t)
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "close/checkout"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "gitlog.txt")
	env := []string{recoveryGitLogEnv + "=" + logPath}
	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "recover", "--json", "spec/checkout")
	if code != 1 {
		t.Fatalf("verdi recover: exit %d, want 1\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	line := strings.TrimRight(stdout, "\n")
	if strings.Contains(line, "\n") {
		t.Fatalf("stdout = %q, want exactly one canonical JSON line", stdout)
	}
	proj, err := recovery.Decode([]byte(line))
	if err != nil {
		t.Fatalf("recovery.Decode(stdout): %v\nstdout: %s", err, stdout)
	}
	if !hasState(proj, recovery.StateEmptyBranchCut) {
		t.Fatalf("no empty-branch-cut state in %+v", proj.States)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading gitlog %s: %v", logPath, err)
	}
	if len(data) == 0 {
		t.Fatal("gitlog file is empty; want at least one recorded command")
	}
	forbidden := map[string]bool{}
	for _, tok := range recovery.ForbiddenTokens {
		forbidden[tok] = true
	}
	for _, ln := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		parts := strings.SplitN(ln, "\t", 2)
		if len(parts) != 2 {
			t.Fatalf("gitlog line %q is not <root>\\t<argv...>", ln)
		}
		for _, word := range strings.Fields(parts[1]) {
			if forbidden[word] {
				t.Fatalf("gitlog line %q carries forbidden token %q", ln, word)
			}
		}
	}
}

// TestRecoverE2E_ApplyStub proves the Task 3 --apply stub's exit-2
// mapping end to end through the real binary. Task 4 deletes this test
// and replaces it with the real apply-protocol proof (the plan's own
// text: "The e2e case for the stub asserts exit 2 and is deleted by Task
// 4").
func TestRecoverE2E_ApplyStub(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := recoverE2ERepo(t)

	_, stderr, code := runVerdiBinary(t, bin, repo.Dir, nil, "recover", "spec/checkout", "--apply", "unwind-branch-cut:close/checkout")
	if code != 2 {
		t.Fatalf("verdi recover --apply (Task 3 stub): exit %d, want 2\nstderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "Task 4") {
		t.Fatalf("stderr = %q, want it to mention the Task 4 stub", stderr)
	}
}
