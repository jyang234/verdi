package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrConfigUnset identifies a well-formed `git config --get` miss: the
// named key is simply not configured anywhere git config consults
// (repository, then global, then system config) — the benign "not set"
// state a checkout legitimately has, mirroring RemoteURL's own
// ErrNoSuchRemote split between "absent" and "operationally broken"
// (ADJ-64: never conflate unreadable with absent). Callers use errors.Is
// to tell a genuinely-unset key apart from a real read error they must
// surface as operational.
var ErrConfigUnset = errors.New("gitx: git config key is unset")

// ConfigValue reads one git-config string value (`git config --get
// <key>`) as git itself resolves it from dir's repository, distinguishing
// three failure shapes local-operator identity resolution depends on
// (2026-09-05 local-operator disposition design §2.1):
//
//   - absent: the key is simply not set anywhere git config consults.
//     `git config --get` documents exit code 1 with no stderr for exactly
//     this case; ConfigValue reports ErrConfigUnset (errors.Is-able),
//     never conflated with a real read failure.
//   - malformed key: git itself refuses the key's syntax (e.g. a key with
//     no section separator, or a variable name not starting with a
//     letter). Git also exits 1 for this, but WITH an explanatory stderr
//     message. That message is git's own localized prose (verified: it
//     is translated under a non-English locale), so absence-vs-malformed
//     is decided from whether stderr is empty, never by matching its
//     text (ADJ-64's locale-independence precedent, RemoteURL). Reported
//     as a plain operational error, never ErrConfigUnset: the caller
//     passed a key git cannot even parse, not a legitimately-unset one.
//   - broken configuration: any other git failure (a corrupt config
//     file, an unreadable repository, a missing git binary). Reported as
//     a plain operational error carrying git's own stderr.
func ConfigValue(ctx context.Context, dir, key string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "config", "--get", key)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return strings.TrimSpace(stdout.String()), nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && strings.TrimSpace(stderr.String()) == "" {
		return "", fmt.Errorf("gitx: ConfigValue(%q): %w", key, ErrConfigUnset)
	}
	return "", fmt.Errorf("gitx: ConfigValue(%q): git config --get (dir %s): %w: %s", key, dir, err, strings.TrimSpace(stderr.String()))
}
