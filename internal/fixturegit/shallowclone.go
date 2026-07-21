package fixturegit

import (
	"path/filepath"
	"strconv"
	"testing"
)

// ShallowClone makes a depth-limited clone of repo into a fresh temp dir and
// returns the clone's working directory. It clones through a file:// URL —
// a plain local-path clone silently ignores --depth and copies the whole
// history — so `git rev-parse --is-shallow-repository` reports true in the
// clone and every commit older than depth from the tip is genuinely ABSENT
// from the clone's object store.
//
// This is the exact GitHub-Actions shallow-checkout shape P2-10b pins: a
// pull_request checkout that is shallow (fetch-depth: 0 notwithstanding)
// where a genuinely-reachable commit beyond the horizon cannot be resolved
// locally at all, so a naive object-presence-or-ancestry check reads it as
// "not reachable" content-dependently by horizon depth. depth counts commits
// from each ref tip: with depth N the tip and its N-1 nearest ancestors are
// present and everything older is beyond the horizon.
//
// -c protocol.file.allow=always is set so the clone works even where the
// file:// transport is otherwise restricted (git >= 2.38.1 hardening); an
// older git that does not know the key ignores it harmlessly. Background
// gc/maintenance is disabled in the clone (D6-31) so no detached writer
// races the test's TempDir cleanup.
func ShallowClone(t testing.TB, repo *Repo, depth int) string {
	t.Helper()
	if depth < 1 {
		t.Fatalf("fixturegit: ShallowClone depth must be >= 1, got %d", depth)
	}
	parent := t.TempDir()
	dest := filepath.Join(parent, "shallow-clone")
	runGit(t, parent, nil,
		"-c", "protocol.file.allow=always",
		"clone", "--quiet", "--depth", strconv.Itoa(depth), "file://"+repo.Dir, dest)
	runGit(t, dest, nil, "config", "gc.autoDetach", "false")
	runGit(t, dest, nil, "config", "maintenance.auto", "false")
	return dest
}
