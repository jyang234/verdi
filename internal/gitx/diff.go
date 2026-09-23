package gitx

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// DiffEntry is one line of `git diff --name-status`'s output: a changed
// path between two revisions, with git's own status code — VL-010's
// immutability check needs this to find which committed files a diff
// touches, and to tell a pure rename (the sole legal diff on a frozen
// file: an active→archive spec move) from a content change.
type DiffEntry struct {
	// Status is git's raw status letter: "A" (added), "M" (modified), "D"
	// (deleted), "R" (renamed), or "C" (copied). Copy entries are emitted
	// only by the handback-specific DiffNameStatusCopies seam.
	Status string
	// Score is the similarity percentage git attached to a rename (0-100);
	// meaningful only when Status is "R" or "C". Identical content scores 100.
	Score int
	// Path is the current (post-change) path.
	Path string
	// OldPath is the pre-change/source path; empty unless Status is "R" or "C".
	OldPath string
}

// Pure reports whether e is a 100%-similarity rename — the only diff shape
// VL-010 permits on an otherwise-frozen file (an active→archive move that
// changes no bytes).
func (e DiffEntry) Pure() bool {
	return e.Status == "R" && e.Score == 100
}

// DiffNameStatus returns the changed paths between base and head in dir
// (`git diff --name-status -M`, rename detection enabled) — VL-010's diff
// base per I-14 (merge-base(HEAD, default branch), supplied by the caller
// via the engine's Context rather than computed here). The answer is the
// full diff in repository-root-relative paths whatever the repository's or
// user's diff.ignoreSubmodules, submodule.<name>.ignore, .gitmodules
// `ignore`, or diff.relative settings say: see diffNameStatus.
func DiffNameStatus(ctx context.Context, dir, base, head string) ([]DiffEntry, error) {
	return diffNameStatus(ctx, "DiffNameStatus", dir, base, head, "-M")
}

// DiffNameStatusCopies returns the complete handback diff with rename and
// copy detection, including unchanged copy sources via --find-copies-harder.
// DiffNameStatus remains rename-only for its existing VL-010 callers. Like
// DiffNameStatus, it answers independently of the submodule-ignore and
// diff.relative settings (see diffNameStatus).
func DiffNameStatusCopies(ctx context.Context, dir, base, head string) ([]DiffEntry, error) {
	return diffNameStatus(ctx, "DiffNameStatusCopies", dir, base, head, "-M", "-C", "--find-copies-harder")
}

// diffNameStatus is the one place both variants build their git command, so
// the two flags below are decided once.
//
// Its callers use the answer as the COMPLETE list of what base..head
// changes: the SI-231 one-behind predicate accepts a commit whose sole
// change is a report, VL-010 and VL-016 check every changed path, spec
// import checks an exact write set, and the sealed-execution handback looks
// for protected paths. Plain `git diff` honors two families of ordinary
// settings that make it report less, and verdi runs inside other people's
// repositories, where either may be set:
//
//   - --ignore-submodules=none. A submodule `ignore = all`, from
//     diff.ignoreSubmodules or submodule.<name>.ignore in any config scope,
//     or from a COMMITTED .gitmodules that every clone inherits, drops a
//     gitlink change from the output. The one-behind predicate then
//     accepted a commit that also bumped a submodule, and close froze a
//     report whose covers named code HEAD no longer has (L3b re-review N-1).
//     For a commit-to-commit diff the flag only changes that; it compares
//     gitlink commits and never looks inside a submodule's working tree.
//   - --no-relative. diff.relative=true, with git run from a subdirectory
//     (cmd.Dir is the caller's store root, which store.FindRoot may find
//     below the git root), drops every path outside that directory and
//     reports the rest relative to it. Callers compare in repository-root
//     paths. The flag needs git 2.28 or later.
//
// SI-226 fixed the same kind of defect in StatusDirty: a git setting
// changing what a verdi check sees.
func diffNameStatus(ctx context.Context, operation, dir, base, head string, detectionArgs ...string) ([]DiffEntry, error) {
	args := append([]string{"diff", "--name-status", "--no-relative", "--ignore-submodules=none"}, detectionArgs...)
	args = append(args, base, head)
	out, err := run(ctx, dir, args...)
	if err != nil {
		return nil, fmt.Errorf("gitx: %s(%s..%s): %w", operation, base, head, err)
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil
	}

	var entries []DiffEntry
	for _, line := range strings.Split(trimmed, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			return nil, fmt.Errorf("gitx: %s(%s..%s): malformed line %q", operation, base, head, line)
		}
		code := fields[0]
		if strings.HasPrefix(code, "R") || strings.HasPrefix(code, "C") {
			if len(fields) != 3 {
				return nil, fmt.Errorf("gitx: %s(%s..%s): malformed rename/copy line %q", operation, base, head, line)
			}
			status := code[:1]
			score, _ := strconv.Atoi(strings.TrimPrefix(code, status))
			entries = append(entries, DiffEntry{Status: status, Score: score, OldPath: fields[1], Path: fields[2]})
			continue
		}
		entries = append(entries, DiffEntry{Status: code, Path: fields[1]})
	}
	return entries, nil
}
