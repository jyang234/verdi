// Built-binary end-to-end tests for `verdi recover` (spec/readiness-
// recovery-v2 ac-8..ac-10, Task 3's read path and Task 4's --apply
// protocol): the empty-branch-cut fixture drives the REAL compiled verdi
// binary as a real OS process (buildVerdiBinary/runVerdiBinary, mirroring
// document_parity_e2e_test.go and obligationseam_e2e_test.go's own
// convention) — never a package-internal call standing in for it —
// proving argument parsing, store resolution, the recovery.CommandLog
// observer wiring, and VERDI_RECOVERY_GITLOG's test-only observability
// end to end. Task 3's own --apply stub case (exit 2, mentioning "Task
// 4") is replaced here by the real apply-protocol proof, per the plan's
// own Step 1 note.
package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/recovery"
	"github.com/jyang234/verdi/internal/store"
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
	if !recoverHasState(proj, recovery.StateEmptyBranchCut) {
		t.Fatalf("no empty-branch-cut state in %+v", proj.States)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading gitlog %s: %v", logPath, err)
	}
	if len(data) == 0 {
		t.Fatal("gitlog file is empty; want at least one recorded command")
	}
	for _, ln := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		parts := strings.SplitN(ln, "\t", 2)
		if len(parts) != 2 {
			t.Fatalf("gitlog line %q is not <root>\\t<argv...>", ln)
		}
		if recovery.IsForbiddenArgv(strings.Fields(parts[1])) {
			t.Fatalf("gitlog line %q carries a forbidden token", ln)
		}
	}
}

// gitlogArgvs parses a VERDI_RECOVERY_GITLOG file's own "<root>\t<argv...>"
// lines into their argv slices, in call order.
func gitlogArgvs(t *testing.T, path string) [][]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading gitlog %s: %v", path, err)
	}
	if len(data) == 0 {
		t.Fatal("gitlog file is empty; want at least one recorded command")
	}
	var out [][]string
	for _, ln := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		parts := strings.SplitN(ln, "\t", 2)
		if len(parts) != 2 {
			t.Fatalf("gitlog line %q is not <root>\\t<argv...>", ln)
		}
		argv := strings.Fields(parts[1])
		if recovery.IsForbiddenArgv(argv) {
			t.Fatalf("gitlog line %q carries a forbidden token", ln)
		}
		out = append(out, argv)
	}
	return out
}

// TestRecoverE2E_ApplyUnwindHappyPath is Task 4's own real apply-protocol
// proof through the built binary (replacing Task 3's stub e2e case,
// which asserted exit 2 and named the Task 4 stub — the plan's own
// text: "The e2e case for the stub asserts exit 2 and is deleted by Task
// 4"): a clean empty branch-cut unwinds via branchcut.Unwind, exit 0,
// every postcondition held, the VERDI_RECOVERY_GITLOG argv sequence is
// exactly reads (rev-parse/status/diff) then a checkout then a
// branch -d, and carries no forbidden token.
func TestRecoverE2E_ApplyUnwindHappyPath(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := recoverE2ERepo(t)
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "close/checkout"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "gitlog.txt")
	env := []string{recoveryGitLogEnv + "=" + logPath}
	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "recover", "spec/checkout", "--apply", "unwind-branch-cut:close/checkout")
	if code != 0 {
		t.Fatalf("verdi recover --apply: exit %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("stdout = %q, want at least one postcondition line and the canonical JSON line", stdout)
	}
	for _, ln := range lines[:len(lines)-1] {
		if !strings.HasPrefix(ln, "postcondition: ") || !strings.Contains(ln, ": held (") {
			t.Fatalf("postcondition line %q not held", ln)
		}
	}
	proj, err := recovery.Decode([]byte(lines[len(lines)-1]))
	if err != nil {
		t.Fatalf("recovery.Decode(stdout's last line): %v\nstdout: %s", err, stdout)
	}
	if recoverHasState(proj, recovery.StateEmptyBranchCut) {
		t.Fatalf("empty-branch-cut still recognized after a successful unwind: %+v", proj.States)
	}

	// The whole run's log spans three internal Gathers plus the post-
	// execution journey re-derivation, so branchcut.Unwind's own
	// checkout-then-branch--d pair is a SUBSEQUENCE of the log, not
	// necessarily its literal tail (dispatch note (e): "contain the
	// expected argv sequence"); rev-parse reads precede it (the branch's
	// own tip and the return branch's own resolution).
	argvs := gitlogArgvs(t, logPath)
	if len(argvs) == 0 {
		t.Fatal("no commands recorded")
	}
	sawRevParse, sawCheckout, sawBranchDelete := false, false, false
	for _, argv := range argvs {
		switch {
		case argv[0] == "rev-parse":
			sawRevParse = true
		case argv[0] == "checkout" && sawRevParse:
			sawCheckout = true
		case argv[0] == "branch" && len(argv) > 1 && argv[1] == "-d" && sawCheckout:
			sawBranchDelete = true
		}
	}
	if !sawRevParse || !sawCheckout || !sawBranchDelete {
		t.Fatalf("gitlog does not contain the expected rev-parse -> checkout -> branch -d subsequence: %v", argvs)
	}
}

// TestRecoverE2E_ApplyRefusesWhenPreconditionMoved proves R-RR3-9's own
// re-proof over a SINGLE static repository state a subprocess test can
// set up directly (no in-process hook is reachable across a real OS
// process boundary): an unrelated, uncommitted change left in the
// working tree does not affect recognizeEmptyBranchCut's own recognition
// gate at all (it checks only branch existence and emptiness, never
// Dirty/StagedPaths), so the choice is still found — but the SAME fact
// fails the choice's own "working tree is clean" precondition on
// re-proof. Exit 1, nothing executed: the gitlog carries no checkout or
// branch command at all.
func TestRecoverE2E_ApplyRefusesWhenPreconditionMoved(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := recoverE2ERepo(t)
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "close/checkout"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "leftover.txt"), []byte("uncommitted\n"), 0o644); err != nil {
		t.Fatalf("writing leftover.txt: %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "gitlog.txt")
	env := []string{recoveryGitLogEnv + "=" + logPath}
	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "recover", "spec/checkout", "--apply", "unwind-branch-cut:close/checkout")
	if code != 1 {
		t.Fatalf("verdi recover --apply: exit %d, want 1\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "no longer clean") {
		t.Fatalf("stderr = %q, want it to name the violated precondition", stderr)
	}

	for _, argv := range gitlogArgvs(t, logPath) {
		if argv[0] == "checkout" || argv[0] == "branch" {
			t.Fatalf("a precondition-moved run issued a checkout/branch command: %v", argv)
		}
	}
	if cur, err := gitx.CurrentBranch(context.Background(), repo.Dir); err != nil || cur != "close/checkout" {
		t.Fatalf("current branch = %q, err = %v, want close/checkout untouched", cur, err)
	}
}

// TestRecoverE2E_ApplyNoExecutor proves a manual-only choice (the stale
// writer lock's own "rm <path>") refuses through the real binary: exit
// 1, the manual command named on stderr, and the lock file itself
// untouched.
func TestRecoverE2E_ApplyNoExecutor(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := recoverE2ERepo(t)
	lockPath := store.WriterLockPath(repo.Dir)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("creating lock dir: %v", err)
	}
	deadCmd := exec.Command("true")
	if err := deadCmd.Run(); err != nil {
		t.Fatalf("running short-lived child: %v", err)
	}
	lockBody, err := json.Marshal(filelock.Info{PID: deadCmd.Process.Pid, Start: time.Now().Add(-time.Hour).Unix()})
	if err != nil {
		t.Fatalf("marshaling lock info: %v", err)
	}
	if err := os.WriteFile(lockPath, lockBody, 0o644); err != nil {
		t.Fatalf("writing %s: %v", lockPath, err)
	}

	// The choice id names the lock path AS THE STORE ITSELF RESOLVED IT
	// (store.FindRoot(".") inside the subprocess), which need not be
	// byte-identical to lockPath here (e.g. macOS's /var -> /private/var
	// symlink) — discovered via a read pass rather than assumed.
	readOut, _, readCode := runVerdiBinary(t, bin, repo.Dir, nil, "recover", "--json", "spec/checkout")
	if readCode != 1 {
		t.Fatalf("verdi recover (read): exit %d, want 1\nstdout:\n%s", readCode, readOut)
	}
	proj, err := recovery.Decode([]byte(strings.TrimRight(readOut, "\n")))
	if err != nil {
		t.Fatalf("recovery.Decode: %v\nstdout: %s", err, readOut)
	}
	if len(proj.States) == 0 || len(proj.States[0].Choices) == 0 {
		t.Fatalf("no choice recognized: %+v", proj.States)
	}
	id := proj.States[0].Choices[0].ID

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, nil, "recover", "spec/checkout", "--apply", id)
	if code != 1 {
		t.Fatalf("verdi recover --apply: exit %d, want 1\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "rm ") {
		t.Fatalf("stderr = %q, want the manual command", stderr)
	}
	if _, statErr := os.Stat(lockPath); statErr != nil {
		t.Fatalf("lock removed by a choice with no executor: %v", statErr)
	}
}
