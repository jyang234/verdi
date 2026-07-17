package gitx

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrNoSuchRemote is returned (wrapped) by RemoteURL when the named remote is
// simply not configured — the benign "remote absent" state a local checkout
// legitimately has, distinct from an operational failure reading a remote
// that DOES exist (a broken git config, an unreadable repo, a missing git
// binary). Callers use errors.Is to tell a genuinely-absent remote (fall
// through to other identification, e.g. the CI env) apart from a real read
// error they must surface as operational (ADJ-64: never conflate unreadable
// with absent).
var ErrNoSuchRemote = errors.New("gitx: no such remote")

// RemoteURL returns the URL configured for remote (typically "origin") in
// dir's git repository (`git remote get-url <name>`) — used by phase 5's
// `sync` to auto-detect the forge kind (gitlab/github) when verdi.yaml
// carries no explicit `forge:` key (I-22). A genuinely-absent remote yields
// ErrNoSuchRemote (errors.Is-able); every other failure stays a plain
// operational error carrying git's own stderr.
//
// Absence is decided from the LOCALE-INDEPENDENT `git remote` name list,
// never by matching git's "No such remote" stderr: git localizes that message
// (e.g. "No existe el remoto", "Pas de serveur remote"), so a stderr match
// would misclassify a benign absent remote as a read failure on any
// non-English host (ADJ-64).
func RemoteURL(ctx context.Context, dir, name string) (string, error) {
	names, err := remoteNames(ctx, dir)
	if err != nil {
		return "", fmt.Errorf("gitx: RemoteURL(%q): %w", name, err)
	}
	if !contains(names, name) {
		return "", fmt.Errorf("gitx: RemoteURL(%q): %w", name, ErrNoSuchRemote)
	}
	out, err := run(ctx, dir, "remote", "get-url", name)
	if err != nil {
		return "", fmt.Errorf("gitx: RemoteURL(%q): %w", name, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// remoteNames returns dir's configured remote names (`git remote`), one per
// line — a locale-independent listing (the literal remote names, never
// localized prose). An empty repo yields an empty slice (exit 0); a non-repo
// or git failure returns the operational error.
func remoteNames(ctx context.Context, dir string) ([]string, error) {
	out, err := run(ctx, dir, "remote")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

func contains(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}
