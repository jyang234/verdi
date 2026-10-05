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
// (WithReadSession), a ref resolution the session memoizes runs once per
// request and is replayed after that.
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
// read session's batch process (readsession.go). A new exec site must call
// observe too — observer_test pins them, and a structural test in the same
// file parses this package's non-test sources and fails if the count of
// exec.Command/exec.CommandContext call sites ever diverges from the count
// of observe( call sites.
func execGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	observe(ctx, dir, args)
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
