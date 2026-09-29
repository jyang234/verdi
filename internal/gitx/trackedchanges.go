package gitx

import (
	"bytes"
	"context"
	"fmt"
	"strings"
)

// trackedChangesArgs is the one git invocation TrackedChanges makes.
//
//   - --no-optional-locks: `git status` otherwise refreshes a stale index
//     and writes it back, which makes a read a write (spec/wall-changes
//     co-1: the workbench performs no git write to compute its summary).
//   - --porcelain=v2: the stable machine format that, unlike v1, reports
//     each entry's HEAD and working-tree modes, so a mode-only change is
//     visible as one.
//   - -z: NUL-delimited, never-quoted paths, so every legal path byte
//     survives core.quotePath.
//   - --untracked-files=no: untracked paths are UntrackedPaths' question;
//     leaving them out here keeps one source for each fact and drops the
//     expensive untracked scan from this call.
var trackedChangesArgs = []string{"--no-optional-locks", "status", "--porcelain=v2", "-z", "--untracked-files=no"}

// TrackedChange is one tracked path whose index or working tree differs
// from HEAD, as `git status --porcelain=v2` reports it.
type TrackedChange struct {
	// Path is repository-root-relative, with forward slashes.
	Path string
	// RenamedFrom is a detected rename's source path, which departs from
	// HEAD too; "" for every other entry (a copy leaves its source
	// untouched, so its source is never named here).
	RenamedFrom string
	// Index and Worktree are the XY status columns: Index compares the
	// index with HEAD, Worktree the working tree with the index. '.' means
	// unchanged; otherwise git's letter (M, T, A, D, R, C, U).
	Index, Worktree byte
	// Unmerged marks a conflicted entry.
	Unmerged bool
	// ModeHead and ModeWorktree are the octal file modes in HEAD and in the
	// working tree ("000000" where the path is absent). An unmerged entry
	// carries no HEAD mode, so ModeHead is "" there.
	ModeHead, ModeWorktree string
}

// TrackedChanges lists every tracked path whose index or working tree
// differs from HEAD, in git's own order, without writing anything — the
// changed-path half of the wall's uncommitted-changes summary (spec/
// wall-changes ac-1); UntrackedPaths is the other half. Paths are
// repository-root-relative wherever dir sits. A clean tree answers nil.
func TrackedChanges(ctx context.Context, dir string) ([]TrackedChange, error) {
	out, err := run(ctx, dir, trackedChangesArgs...)
	if err != nil {
		return nil, fmt.Errorf("gitx: TrackedChanges(%s): %w", dir, err)
	}
	changes, err := parseTrackedStatus(out)
	if err != nil {
		return nil, fmt.Errorf("gitx: TrackedChanges(%s): %w", dir, err)
	}
	return changes, nil
}

// parseTrackedStatus decodes `git status --porcelain=v2 -z
// --untracked-files=no` output. Each entry is NUL-terminated:
//
//	1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>
//	2 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <X><score> <path>NUL<origPath>
//	u <XY> <sub> <m1> <m2> <m3> <mW> <h1> <h2> <h3> <path>
//
// The fields before the path never hold a space, so splitting on the
// fixed count keeps a path's own spaces. Any other entry — an untracked
// '?', an ignored '!', a '#' header — is one this query never asks for, and
// is refused rather than skipped.
func parseTrackedStatus(out []byte) ([]TrackedChange, error) {
	fields := bytes.Split(out, []byte{0})
	var changes []TrackedChange
	for i := 0; i < len(fields); i++ {
		entry := string(fields[i])
		if entry == "" {
			continue // the trailing NUL
		}
		switch entry[0] {
		case '1':
			parts := strings.SplitN(entry, " ", 9)
			if len(parts) != 9 || len(parts[1]) != 2 {
				return nil, fmt.Errorf("malformed `git status --porcelain=v2` entry %q", entry)
			}
			changes = append(changes, TrackedChange{Path: parts[8], Index: parts[1][0], Worktree: parts[1][1], ModeHead: parts[3], ModeWorktree: parts[5]})
		case '2':
			parts := strings.SplitN(entry, " ", 10)
			if len(parts) != 10 || len(parts[1]) != 2 || parts[8] == "" {
				return nil, fmt.Errorf("malformed `git status --porcelain=v2` rename/copy entry %q", entry)
			}
			i++
			if i >= len(fields) || len(fields[i]) == 0 {
				return nil, fmt.Errorf("`git status --porcelain=v2` rename/copy entry %q has no source-path field", entry)
			}
			change := TrackedChange{Path: parts[9], Index: parts[1][0], Worktree: parts[1][1], ModeHead: parts[3], ModeWorktree: parts[5]}
			if parts[8][0] == 'R' {
				change.RenamedFrom = string(fields[i])
			}
			changes = append(changes, change)
		case 'u':
			parts := strings.SplitN(entry, " ", 11)
			if len(parts) != 11 || len(parts[1]) != 2 {
				return nil, fmt.Errorf("malformed `git status --porcelain=v2` unmerged entry %q", entry)
			}
			changes = append(changes, TrackedChange{Path: parts[10], Index: parts[1][0], Worktree: parts[1][1], Unmerged: true, ModeWorktree: parts[6]})
		default:
			return nil, fmt.Errorf("unexpected `git status --porcelain=v2 --untracked-files=no` entry %q", entry)
		}
	}
	return changes, nil
}
