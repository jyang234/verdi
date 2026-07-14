package main

// The single git-invocation seam (file-topics ac-4): every scratch-store git
// call — the corpus seed commit, the bare local origin init, and the design
// branch's fixture commit — used to run through its own hand-typed closure,
// and only one of the three pinned deterministic dates. runGit replaces all
// three so every commit e2eharness produces has a fixed SHA (nothing here
// asserts a specific hash — this is determinism-for-its-own-sake, matching
// the guarantee internal/fixturegit gives the Go test suites, at
// test-harness weight rather than fixturegit's golden-SHA machinery).

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
