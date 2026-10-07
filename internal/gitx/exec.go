package gitx

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// run execs `git <args...>` with its working directory set to dir, returning
// stdout on success. A non-zero exit becomes an error naming the command and
// stderr, never a silent empty result. Inside a read session for dir
// (WithReadSession), a memoizable ref read whose identical argv already
// ran in the request is replayed instead of run again: the replay runs no
// git, so it passes no observe point and records nothing (one record per
// git execution, ledger SI-359 (1)); its first run was execGit's.
func run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if s := sessionFor(ctx, dir); s != nil && memoizable(args) {
		return s.memoized(ctx, dir, args)
	}
	return execGit(ctx, dir, args...)
}

// execGit is one git process: run's exec, every time.
//
// gitx has exactly four exec sites, and each calls observe first: execGit
// (here), ConfigValue (configvalue.go), runStdin (plumbing.go), and the
// read session's batch process (startCatFileBatch, readsession.go). A new
// exec site must call observe too — observer_test and
// readsession_observe_test pin all four, and a structural test in
// observer_test parses this package's non-test sources and fails if the
// count of exec.Command/exec.CommandContext call sites ever diverges from
// the count of observe( call sites. observe's error (a VERDI_GITLOG record
// that failed, gitlog.go) is returned before git runs.
func execGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if err := observe(ctx, dir, args); err != nil {
		return nil, fmt.Errorf("gitx: git %s (dir %s): %w", strings.Join(args, " "), dir, err)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gitx: git %s (dir %s): %w: %s", strings.Join(args, " "), dir, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
