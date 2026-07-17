package gitx

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrNoSuchRemote is returned (wrapped) by RemoteURL when git reports the
// named remote is simply not configured (git: "No such remote", exit 2) —
// the benign "remote absent" state a local checkout legitimately has,
// distinct from an operational failure reading a remote that DOES exist (a
// broken git config, an unreadable repo, a missing git binary). Callers use
// errors.Is to tell a genuinely-absent remote (fall through to other
// identification, e.g. the CI env) apart from a real read error they must
// surface as operational (ADJ-64: never conflate unreadable with absent).
var ErrNoSuchRemote = errors.New("gitx: no such remote")

// RemoteURL returns the URL configured for remote (typically "origin") in
// dir's git repository (`git remote get-url <name>`) — used by phase 5's
// `sync` to auto-detect the forge kind (gitlab/github) when verdi.yaml
// carries no explicit `forge:` key (I-22). A genuinely-absent remote yields
// ErrNoSuchRemote (errors.Is-able); every other failure stays a plain
// operational error carrying git's own stderr.
func RemoteURL(ctx context.Context, dir, name string) (string, error) {
	out, err := run(ctx, dir, "remote", "get-url", name)
	if err != nil {
		// git prints "No such remote '<name>'" and exits 2 when the remote is
		// simply not configured — the benign absent case, marked with the
		// ErrNoSuchRemote sentinel so a caller can distinguish it from a real
		// read failure (which keeps its full operational error). Any other
		// exit (e.g. 128 "not a git repository", or an exec/start failure)
		// falls through to the plain error.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 && strings.Contains(err.Error(), "No such remote") {
			return "", fmt.Errorf("gitx: RemoteURL(%q): %w", name, ErrNoSuchRemote)
		}
		return "", fmt.Errorf("gitx: RemoteURL(%q): %w", name, err)
	}
	return strings.TrimSpace(string(out)), nil
}
