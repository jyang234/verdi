package gitx

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
)

// stagedPathsArgs is the ONE git invocation StagedPaths makes, spelled out
// here so the reasoning for each token stays attached to it.
//
// `git status --porcelain` — NOT `git diff --cached` and NOT
// `git diff-index --cached` — is the primitive, because this guard is a
// safety refusal: an empty answer permits a closure commit to proceed, so any
// configuration that makes the command under-report is a fail-open hole, and
// verdi runs inside OTHER PEOPLE'S repositories where "we have no submodules"
// is not a defense. Two ordinary configurations make both diff forms report an
// empty index while `git commit` still records the staged change (both proven
// against real git in stagedpaths_test.go):
//
//   - Submodule pointer bumps. `diff.ignoreSubmodules=all` in any config
//     scope, or a COMMITTED .gitmodules carrying `ignore = all` (so every
//     clone inherits it), hides a staged gitlink change from `git diff
//     --cached`. `git diff-index --cached` is measurably NOT a fix: the
//     .gitmodules form hides the bump from it too.
//   - Staged paths outside dir. `diff.relative=true` scopes diff output to the
//     process's working directory, and this package always runs with
//     cmd.Dir = the store root — which store.FindRoot resolves by walking up
//     to the nearest .verdi, so it can legitimately sit BELOW the git root.
//     Everything staged above it then vanishes from `git diff --cached` while
//     `git commit` from that same directory still commits it.
//
// `git status --porcelain` answers both correctly: its v1 format is documented
// as backward-compatible, its paths are repository-root-relative regardless of
// the working directory or status.relativePaths, and it reports index-vs-HEAD
// differences it is not asked to suppress. The explicit flags close its own
// two configurable holes and one cost:
//
//   - --ignore-submodules=none overrides both diff.ignoreSubmodules and
//     .gitmodules' per-submodule `ignore`, so no repository config can blind
//     the guard to a staged pointer bump.
//   - --untracked-files=no drops the untracked scan entirely: untracked files
//     are legal during closure and are filtered out below anyway, and the scan
//     is the expensive part of status on a large checkout.
//   - -z emits NUL-delimited, never-quoted paths, so every legal path byte
//     (whitespace, newlines, non-ASCII) survives core.quotePath.
var stagedPathsArgs = []string{"status", "--porcelain", "-z", "--ignore-submodules=none", "--untracked-files=no"}

// StagedPaths returns the repository-relative paths whose index entries
// differ from HEAD, sorted so callers can report them deterministically.
//
// BOTH sides of a staged rename are returned. Git's `--name-only` reports only
// a rename's destination (an R100 old->new renders as `new` alone), which
// under-names what a refusal is actually refusing over: the source path's
// deletion is staged too, and the operator has to deal with it. A staged COPY
// contributes only its destination, since a copy leaves its source's index
// entry equal to HEAD.
//
// Worktree-only modifications and untracked files are deliberately absent:
// they are legal alongside the rituals that consult this, which refuse only on
// index entries a subsequent `git commit` would silently absorb.
func StagedPaths(ctx context.Context, dir string) ([]string, error) {
	out, err := run(ctx, dir, stagedPathsArgs...)
	if err != nil {
		return nil, fmt.Errorf("gitx: StagedPaths(%s): %w", dir, err)
	}

	paths, err := parseStagedStatus(out)
	if err != nil {
		return nil, fmt.Errorf("gitx: StagedPaths(%s): %w", dir, err)
	}
	sort.Strings(paths)
	return paths, nil
}

// RepoPrefix returns dir's own path relative to the repository root — `git
// rev-parse --show-prefix` — as a slash path ending in "/", or "" when dir IS
// the repository root.
//
// It is StagedPaths' companion, and exists for the same layout StagedPaths'
// own reasoning names. StagedPaths answers in REPOSITORY-root-relative paths
// deliberately (that is precisely what makes it immune to diff.relative),
// while a caller that passed the store root as dir reasons about paths
// relative to THAT root. When the two roots coincide — the common layout —
// this is "" and the two vocabularies are the same one; when the store root
// sits below the git root, this is the only thing that relates them, and a
// caller that skips it silently reads every staged store path as foreign.
//
// Only the single trailing newline is trimmed, never surrounding whitespace:
// a directory name may legally end in a space, and TrimSpace would corrupt the
// prefix it is supposed to relate paths through.
func RepoPrefix(ctx context.Context, dir string) (string, error) {
	out, err := run(ctx, dir, "rev-parse", "--show-prefix")
	if err != nil {
		return "", fmt.Errorf("gitx: RepoPrefix(%s): %w", dir, err)
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

// parseStagedStatus extracts the index-differs-from-HEAD paths from `git
// status --porcelain -z` output.
//
// Each entry is "XY <path>", where X is the index status and Y the worktree
// status, followed by a NUL. Under -z a rename or copy (X of 'R' or 'C')
// appends its ORIGINAL path as a second NUL-terminated field immediately after
// — git's -z format reverses the human format's "from -> to" into "to\0from\0"
// and drops the arrow. An entry is staged exactly when X is neither ' '
// (index matches HEAD; the whole entry is a worktree-only change) nor '?'
// (untracked) nor '!' (ignored).
func parseStagedStatus(out []byte) ([]string, error) {
	fields := bytes.Split(out, []byte{0})
	var paths []string
	for i := 0; i < len(fields); i++ {
		entry := fields[i]
		if len(entry) == 0 {
			// The trailing NUL yields one empty final field; a leading or
			// interior empty field cannot occur because every entry carries at
			// least the two status bytes.
			continue
		}
		if len(entry) < 4 || entry[2] != ' ' {
			return nil, fmt.Errorf("malformed `git status --porcelain -z` entry %q", entry)
		}
		index, path := entry[0], string(entry[3:])

		orig := ""
		if index == 'R' || index == 'C' {
			i++
			if i >= len(fields) || len(fields[i]) == 0 {
				return nil, fmt.Errorf("`git status --porcelain -z` entry %q declares a rename/copy with no original-path field", entry)
			}
			orig = string(fields[i])
		}

		if index == ' ' || index == '?' || index == '!' {
			continue
		}
		paths = append(paths, path)
		// A rename's source is staged as a deletion; a copy's source is
		// untouched, so only the rename contributes its original path.
		if index == 'R' {
			paths = append(paths, orig)
		}
	}
	return paths, nil
}
