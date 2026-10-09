package main

// The single git-invocation seam (file-topics ac-4): every scratch-store git
// call in this package goes through runGit (command) or gitOutput (query;
// gitRawOutput is its untrimmed form) — the corpus seed commit, the bare
// local origin init, the design branch's fixture commit, and every
// provisioner's reads. Each used to run through
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
	"context"
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

// runGit runs git in dir under ctx, carrying deterministicGitEnv plus any
// extraEnv on top of the ambient environment — extraEnv entries override a
// same-keyed deterministicGitEnv/ambient entry outright (mergeGitEnv below),
// never relying on the OS's own unspecified handling of a duplicate key in
// a raw envp array. On failure the error wraps the command's combined
// output. ctx is honoured for real (CommandContext): an already-cancelled
// ctx refuses before git is spawned, and a cancellation mid-run kills the
// child — the seam main.go's interrupt handling relies on to unwind
// provisioning instead of leaking a git process.
func runGit(ctx context.Context, dir string, extraEnv []string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = mergeGitEnv(append(os.Environ(), deterministicGitEnv...), extraEnv)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v: %w\n%s", args, err, out)
	}
	return nil
}

// gitOutput runs git in dir under ctx and returns its trimmed stdout — the
// query twin of runGit (same env pinning, same ctx honouring), for
// provisioning steps that need a value back (e.g. the store HEAD sha the
// sealed badge fixture's frozen stamp pins) and for the loopback inspection
// routes, which pass their request's ctx. On failure the error wraps stderr.
func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := gitRawOutput(ctx, dir, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// initRepo creates a repository at dir on branch main, bare when bare is
// set (a store's local origin), and switches off git's background
// housekeeping in it: gc.autoDetach and maintenance.auto both false, as
// internal/fixturegit sets them (D6-31). It is the harness's one `git
// init` (BL-148; BL-113): a detached `git gc --auto`, which a commit or a
// push into an origin can start, may otherwise still be writing into the
// repository while whatever created it removes it — a test's TempDir
// cleanup, or main.go's own scratch removal.
func initRepo(ctx context.Context, dir string, bare bool) error {
	args := []string{"init", "--quiet", "--initial-branch=main"}
	if bare {
		args = append(args, "--bare")
	}
	if err := runGit(ctx, "", nil, append(args, dir)...); err != nil {
		return err
	}
	for _, key := range []string{"gc.autoDetach", "maintenance.auto"} {
		if err := runGit(ctx, dir, nil, "config", key, "false"); err != nil {
			return err
		}
	}
	return nil
}

// commitAt is runGit's dated-commit convenience (spec/index-data ac-3): it
// overrides deterministicGitEnv's single fixed GIT_AUTHOR_DATE/
// GIT_COMMITTER_DATE with date (git's "<unix-seconds> <tz-offset>" form) for
// exactly this one command — a fixture branch's commit at a KNOWN, chosen
// date, distinct from every other commit's shared default, so a later
// reader (the index-dates control endpoint below, or a Playwright spec) can
// assert an entry's age deterministically.
func commitAt(ctx context.Context, dir, date string, args ...string) error {
	return runGit(ctx, dir, []string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date}, args...)
}

// mergeGitEnv returns base with every key present in overrides removed
// first, so appending overrides is an unambiguous replace rather than a
// duplicate envp entry whose winner is platform-dependent (getenv()
// conventions differ on which duplicate wins) — the same care
// internal/fixturegit's own mergeEnv takes, reimplemented here since this
// package cannot import a _test.go-only helper from another module's test
// tree.
func mergeGitEnv(base, overrides []string) []string {
	if len(overrides) == 0 {
		return base
	}
	overrideKeys := make(map[string]bool, len(overrides))
	for _, kv := range overrides {
		overrideKeys[gitEnvKey(kv)] = true
	}
	merged := make([]string, 0, len(base)+len(overrides))
	for _, kv := range base {
		if !overrideKeys[gitEnvKey(kv)] {
			merged = append(merged, kv)
		}
	}
	return append(merged, overrides...)
}

// gitEnvKey returns the "NAME" half of a "NAME=value" environment entry.
func gitEnvKey(kv string) string {
	if i := strings.IndexByte(kv, '='); i >= 0 {
		return kv[:i]
	}
	return kv
}

// gitRawOutput is gitOutput untrimmed: git's stdout exactly as printed,
// for an answer that must stay byte-exact — a porcelain line's leading
// status column, a file's final newline (the new-story fixture's
// read-only routes). Same env pinning and ctx honouring as gitOutput; on
// failure the error wraps stderr.
func gitRawOutput(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), deterministicGitEnv...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %v: %w\n%s", args, err, stderr.String())
	}
	return out, nil
}
