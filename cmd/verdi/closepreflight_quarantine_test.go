// Real, built-binary end-to-end test for spec/evidence-resilience ac-2:
// `verdi close --preflight` (side-effect-free, and the exact evaluation
// function — runClosureGate — a real `verdi close` also calls first)
// driven against a real, hermetic fixturegit checkout whose derived
// evidence tree carries X-15's exact shape: a record under a commit-named
// directory that resolves to no real commit at all (the branch that
// produced it has since been deleted). Proves end to end, through the
// actual compiled binary rather than an in-package function call, that
// closure over a quarantined/unreachable record never exits operationally
// and reads the affected AC as disclosed-unproven.
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// preflightQuarantineCLIFixture builds a real store root — the manifest,
// the quarantine-story spec (evidence: [static], mirroring
// closureGateQuarantineStorySpecMD), and a derived verdicts.json under a
// commit-named directory that does not resolve to any real commit in this
// repo's history at all — with the story's spec on a checked-out feature
// branch, matching a real build branch's shape.
func preflightQuarantineCLIFixture(t *testing.T) (root string, unreachable string) {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                            "schema: verdi.layout/v1\nforge: gitlab\n",
			".verdi/specs/active/quarantine-story/spec.md": closureGateQuarantineStorySpecMD,
		},
		Message: "close --preflight quarantine CLI fixture",
	}})
	ctx := context.Background()
	if err := gitx.CheckoutNewBranch(ctx, repo.Dir, "feature/quarantine-story"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}

	unreachable = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	writeClosureGateDerivedRecord(t, repo.Dir, "spec/quarantine-story", unreachable, closureGateQuarantineRecordJSON(unreachable))
	return repo.Dir, unreachable
}

func TestClosePreflight_CLI_QuarantinedRecord_NeverOperational_DisclosesUnproven(t *testing.T) {
	bin := buildVerdiBinary(t)
	root, unreachable := preflightQuarantineCLIFixture(t)

	cmd := exec.Command(bin, "close", "spec/quarantine-story", "--preflight")
	cmd.Dir = root
	// No real "origin" remote exists in this hermetic fixture (co-1: no
	// network in any test) — CI_DEFAULT_BRANCH lets default-branch
	// resolution succeed without one, mirroring gc_test.go's own built-
	// binary convention. No forge credentials are exported, so
	// buildForgeBestEffort resolves to a nil forge here exactly as the
	// Go-level tests pass explicitly — condition 3 stays hermetic.
	cmd.Env = append(os.Environ(), "CI_DEFAULT_BRANCH=main")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("verdi close --preflight: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
		}
	}

	if exitCode == 2 {
		t.Fatalf("verdi close --preflight exit = 2 (operational failure) — X-15 must never brick this; stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if exitCode != 1 {
		t.Fatalf("verdi close --preflight exit = %d, want 1 (NOT READY — ac-1 has no other evidence); stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "[FAIL] closure: 1.") {
		t.Errorf("stdout = %q, want condition 1 to FAIL (never silently proven from an excluded record)", out)
	}
	if !strings.Contains(out, "disclosed-unproven [gate:evidence-quarantine]") {
		t.Errorf("stdout = %q, want the per-record disclosed-unproven line (ac-2)", out)
	}
	if !strings.Contains(out, "ac-1") {
		t.Errorf("stdout = %q, want ac-1 named as the AC the excluded record would have evidenced", out)
	}
	if !strings.Contains(out, unreachable) {
		t.Errorf("stdout = %q, want the unreachable commit %q named", out, unreachable)
	}
	if !strings.Contains(out, "close: --preflight: NOT READY") {
		t.Errorf("stdout = %q, want the preflight verdict line", out)
	}
}
