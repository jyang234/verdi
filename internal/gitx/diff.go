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
	// (deleted), or "R" (renamed) — "C" (copied) never appears since
	// DiffNameStatus does not pass -C.
	Status string
	// Score is the similarity percentage git attached to a rename (0-100);
	// meaningful only when Status == "R". A pure rename (identical content)
	// scores 100.
	Score int
	// Path is the current (post-change) path.
	Path string
	// OldPath is the pre-change path; empty unless Status == "R".
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
	out, err := run(ctx, dir, "diff", "--name-status", "-M", base, head)
	if err != nil {
		return nil, fmt.Errorf("gitx: DiffNameStatus(%s..%s): %w", base, head, err)
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil
	}

	var entries []DiffEntry
	for _, line := range strings.Split(trimmed, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			return nil, fmt.Errorf("gitx: DiffNameStatus(%s..%s): malformed line %q", base, head, line)
		}
		code := fields[0]
		if strings.HasPrefix(code, "R") {
			if len(fields) != 3 {
				return nil, fmt.Errorf("gitx: DiffNameStatus(%s..%s): malformed rename line %q", base, head, line)
			}
			score, _ := strconv.Atoi(strings.TrimPrefix(code, "R"))
			entries = append(entries, DiffEntry{Status: "R", Score: score, OldPath: fields[1], Path: fields[2]})
			continue
		}
		entries = append(entries, DiffEntry{Status: code, Path: fields[1]})
	}
	return entries, nil
}
