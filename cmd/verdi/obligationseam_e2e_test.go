// Built-binary end-to-end tests for spec/obligation-seam's two behavioral
// surfaces (ac-1..ac-3's accept backstop, ac-5's `verdi obligation
// author`): both execute the REAL compiled verdi binary as a real OS
// process against a real, local fixturegit repository — never a
// package-internal call standing in for it (mirroring model_test.go's own
// buildVerdiBinary-driven style, the established convention for this
// package's CLI-behavioral-path proofs). The obligation-author frozen
// case in particular exercises the FULL production default-branch/
// merge-base resolution (internal/lint.BuildContext -> gitx.DefaultBranch/
// MergeBase), not the injected-diffBase shortcut obligation_test.go's
// in-process tests use — the one path only a real subprocess can prove
// end to end.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// runVerdiBinary execs bin with args, cwd=dir, optional extra environment
// (appended to the inherited os.Environ()), capturing stdout/stderr
// separately — mirroring runModelCheckBinary/runDispositionBinary's exact
// pattern (model_test.go/disposition_test.go).
func runVerdiBinary(t *testing.T, bin, dir string, extraEnv []string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return outBuf.String(), errBuf.String(), ee.ExitCode()
		}
		t.Fatalf("running verdi %v: %v", args, err)
	}
	return outBuf.String(), errBuf.String(), 0
}

// TestObligationSeamE2E_AcceptScaffoldsMissingObligations is ac-1's
// built-binary proof: `verdi accept` against a draft story with two
// missing (ac, kind) pairs, run as a real subprocess, exits 0 and leaves
// both obligations decodable on disk.
func TestObligationSeamE2E_AcceptScaffoldsMissingObligations(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildObligationSeamStoryRepo(t, nil)

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, nil, "accept", "spec/widget-story")
	if code != 0 {
		t.Fatalf("verdi accept exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	for _, p := range []string{
		obligationPathFor(repo.Dir, "ac-1", "static"),
		obligationPathFor(repo.Dir, "ac-2", "behavioral"),
	} {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("obligation not written at %s: %v", p, err)
		}
		fm, _, err := artifact.SplitFrontmatter(raw)
		if err != nil {
			t.Fatalf("split %s: %v", p, err)
		}
		if _, err := artifact.DecodeObligation(fm); err != nil {
			t.Fatalf("decode %s: %v\n%s", p, err, raw)
		}
	}
}

// TestObligationSeamE2E_ObligationAuthorCreate is ac-5's CREATE case as a
// real subprocess: no obligation yet, no default branch configured (a bare
// fixture repo has no origin remote) — the verb proceeds and writes an
// unauthored scaffold.
func TestObligationSeamE2E_ObligationAuthorCreate(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildObligationAuthorRepo(t, nil)

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, nil, "obligation", "author", "spec/widget-story", "ac-1", "static")
	if code != 0 {
		t.Fatalf("verdi obligation author exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	path := obligationPathFor(repo.Dir, "ac-1", "static")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("obligation not written at %s: %v", path, err)
	}
	fm, body, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		t.Fatalf("split %s: %v", path, err)
	}
	if _, err := artifact.DecodeObligation(fm); err != nil {
		t.Fatalf("decode %s: %v\n%s", path, err, raw)
	}
	if !contains(string(body), "verdi:obligation-unauthored") {
		t.Fatalf("body missing the unauthored marker:\n%s", body)
	}
}

// TestObligationSeamE2E_ObligationAuthorRefusesOnAlreadyFrozen is ac-5's
// frozen case driven through the REAL production default-branch/
// merge-base resolution: a fixturegit repo initializes on branch "main"
// (fixturegit.go's own --initial-branch=main), and CI_DEFAULT_BRANCH=main
// makes internal/lint.ResolveDefaultBranch resolve deterministically
// without any network or fabricated origin remote (its own first,
// highest-priority check) — merge-base(HEAD, main) is then HEAD itself,
// since the fixture never diverges from it, so the already-committed
// obligation is trivially "reachable from the merge-base".
func TestObligationSeamE2E_ObligationAuthorRefusesOnAlreadyFrozen(t *testing.T) {
	bin := buildVerdiBinary(t)
	frozenObligationMD := `---
id: obligation/widget-story--ac-1--static
kind: obligation
title: "already frozen by a prior merge"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/widget-story" }
frozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# already frozen by a prior merge

Reachable from the merge-base: nothing may touch this.
`
	repo := buildObligationAuthorRepo(t, map[string]string{
		".verdi/obligations/widget-story/ac-1--static.md": frozenObligationMD,
	})

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, "obligation", "author", "spec/widget-story", "ac-1", "static")
	if code != 2 {
		t.Fatalf("verdi obligation author (frozen) exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !contains(stderr, "ac-1--static.md") {
		t.Fatalf("stderr = %q, want it to name the frozen path", stderr)
	}

	got, err := os.ReadFile(filepath.Join(repo.Dir, ".verdi", "obligations", "widget-story", "ac-1--static.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != frozenObligationMD {
		t.Fatalf("a frozen obligation must never be touched by the real subprocess:\n--- got ---\n%s\n--- want ---\n%s", got, frozenObligationMD)
	}
}
