package gitx

import (
	"context"
	"fmt"
	"strings"
)

// ResolveExactRef returns the object id that the full refname ref names in
// dir, reading exactly that ref and nothing else (`git show-ref --verify
// --hash <ref>`). RevParse is different: when the exact ref is absent,
// `git rev-parse --verify` goes on through git's ref lookup rules, so a tag
// or branch literally named refs/remotes/origin/<name>
// (refs/tags/refs/remotes/origin/<name>, refs/heads/refs/remotes/origin/<name>)
// would answer for the missing remote-tracking ref. show-ref --verify
// accepts only the exact path and evaluates no revision syntax.
//
// ref must be a full refname under refs/. Anything else is refused before
// git runs, so neither a revision expression nor an option reaches git. An
// absent ref is an error, like every git failure: show-ref --verify reports
// a missing ref as fatal, so the two cannot be told apart by exit status.
func ResolveExactRef(ctx context.Context, dir, ref string) (string, error) {
	if !strings.HasPrefix(ref, "refs/") {
		return "", fmt.Errorf("gitx: ResolveExactRef(%q): not a full refname under refs/", ref)
	}
	out, err := run(ctx, dir, "show-ref", "--verify", "--hash", ref)
	if err != nil {
		return "", fmt.Errorf("gitx: ResolveExactRef(%q): %w", ref, err)
	}
	id := strings.TrimSuffix(string(out), "\n")
	if !isFullObjectID(id) {
		return "", fmt.Errorf("gitx: ResolveExactRef(%q): git printed %q, not one full object id", ref, id)
	}
	return id, nil
}

// isFullObjectID reports whether id is one full lowercase-hex object id:
// 40 digits (SHA-1) or 64 (SHA-256).
func isFullObjectID(id string) bool {
	if len(id) != 40 && len(id) != 64 {
		return false
	}
	for _, r := range id {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
