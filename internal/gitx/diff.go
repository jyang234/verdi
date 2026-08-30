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
// via the engine's Context rather than computed here).
func DiffNameStatus(ctx context.Context, dir, base, head string) ([]DiffEntry, error) {
	return diffNameStatus(ctx, "DiffNameStatus", dir, base, head, "-M")
}

// DiffNameStatusCopies returns the complete handback diff with rename and
// copy detection, including unchanged copy sources via --find-copies-harder.
// DiffNameStatus remains rename-only for its existing VL-010 callers.
func DiffNameStatusCopies(ctx context.Context, dir, base, head string) ([]DiffEntry, error) {
	return diffNameStatus(ctx, "DiffNameStatusCopies", dir, base, head, "-M", "-C", "--find-copies-harder")
}

func diffNameStatus(ctx context.Context, operation, dir, base, head string, detectionArgs ...string) ([]DiffEntry, error) {
	args := append([]string{"diff", "--name-status"}, detectionArgs...)
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
