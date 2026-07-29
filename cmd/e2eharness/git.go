package main

// The single git-invocation seam (file-topics ac-4): every scratch-store git
// call in this package goes through runGit (command) or gitOutput (query)
// — the corpus seed commit, the bare local origin init, the design branch's
// fixture commit, and every provisioner's reads. Each used to run through
// its own hand-typed closure carrying its own env, and only one of them
// pinned the deterministic dates; the seam is why every commit e2eharness
// produces now has a fixed SHA (nothing here asserts a specific hash — this
// is determinism-for-its-own-sake, matching the guarantee
// internal/fixturegit gives the Go test suites, at test-harness weight
// rather than fixturegit's golden-SHA machinery).
//
// The seam has drifted once and been repaired: provision_diagram.go grew a
// near-identical runGitOut after this file shipped, and it has been folded
// back into gitOutput. There is exactly ONE deliberate exception, and it
// documents its own reason: provision_showcase_draft.go's gitShowBytes,
// which must return RAW bytes where gitOutput trims. Anything else that
// needs to shell out to git belongs here.

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// deterministicGitEnv pins author/committer identity and timestamps so
// every commit runGit makes is byte-for-byte reproducible.
var deterministicGitEnv = []string{
	"GIT_AUTHOR_NAME=verdi-e2e", "GIT_AUTHOR_EMAIL=e2e@verdi.invalid", "GIT_AUTHOR_DATE=1704067200 +0000",
	"GIT_COMMITTER_NAME=verdi-e2e", "GIT_COMMITTER_EMAIL=e2e@verdi.invalid", "GIT_COMMITTER_DATE=1704067200 +0000",
}

// runGit runs git in dir, carrying deterministicGitEnv plus any extraEnv on
// top of the ambient environment. On failure the error wraps the command's
// combined output.
func runGit(dir string, extraEnv []string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), deterministicGitEnv...), extraEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v: %w\n%s", args, err, out)
	}
	return nil
}

// gitOutput runs git in dir and returns its trimmed stdout — the query
// twin of runGit (same env pinning), for provisioning steps that need a
// value back (e.g. the store HEAD sha the sealed badge fixture's frozen
// stamp pins). On failure the error wraps stderr.
func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), deterministicGitEnv...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %v: %w\n%s", args, err, stderr.String())
	}
	return strings.TrimSpace(string(out)), nil
}
