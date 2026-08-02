package gitx

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// blobRecordPattern matches a single `git ls-tree` record naming a plain or
// executable file: "<mode> blob <40-hex-oid>\t<path>". Any other tree entry
// — a directory (mode 040000, type tree), a symlink (mode 120000, which is
// still git object type "blob" but not a plain file), or a gitlink
// submodule (mode 160000) — does not match, since only a plain file has
// content BlobAt's callers can compare byte-for-byte.
var blobRecordPattern = regexp.MustCompile(`^(?:100644|100755) blob ([0-9a-f]{40})\t`)

// BlobAt resolves path's tracked blob object id as it exists at ref via
// `git ls-tree ref -- path` — the primitive spec-acceptance state needs to
// compare "the exact bytes a caller has in hand" against "what a ref
// currently holds at that path", without ever touching or requiring a
// checked-out working tree.
//
// It draws a clean three-way distinction: a plain (or executable) tracked
// file at ref yields its blob's 40-hex object id and found=true; a path
// absent at a resolvable ref is not an error — `git ls-tree` simply returns
// no record for it — and yields ("", false, nil); and anything else is an
// operational error wrapping the git or parse failure with
// "gitx: BlobAt(REF:PATH)" context. A tree entry that is not a single plain
// file blob is refused rather than silently coerced: a directory addressed
// directly is one non-blob-mode record (refused), a directory addressed
// with a trailing slash that expands into several records is refused as
// ambiguous, and a symlink or gitlink is refused by mode even though a
// symlink's git object type is itself "blob". path is repo-relative
// (forward slashes).
func BlobAt(ctx context.Context, dir, ref, path string) (oid string, found bool, err error) {
	out, runErr := run(ctx, dir, "ls-tree", ref, "--", path)
	if runErr != nil {
		return "", false, fmt.Errorf("gitx: BlobAt(%s:%s): %w", ref, path, runErr)
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return "", false, nil
	}

	lines := strings.Split(trimmed, "\n")
	if len(lines) != 1 {
		return "", false, fmt.Errorf("gitx: BlobAt(%s:%s): expected exactly one tree entry, got %d", ref, path, len(lines))
	}

	m := blobRecordPattern.FindStringSubmatch(lines[0])
	if m == nil {
		return "", false, fmt.Errorf("gitx: BlobAt(%s:%s): tree entry is not a plain file blob: %q", ref, path, lines[0])
	}
	return m[1], true, nil
}

// FirstParentBlobLanding finds the first-parent commit on ref that landed
// the blob identified by oid at path — the commit spec acceptance reports
// as having brought a specific spec revision's exact bytes onto the
// default branch, regardless of whether it arrived via a plain add, a
// regular (non-fast-forward) merge commit, a squash merge, or a
// rebase-then-fast-forward. All of those are real, first-parent-reachable
// commits on ref; only a commit that stays reachable solely through a
// merge's second parent (an unmerged or never-integrated side branch) is
// excluded.
//
// The walk is oldest-first over `git rev-list --first-parent --reverse
// ref -- path` — the SAME first-parent chain the semantics have always
// been stated over, PATH-LIMITED so only the commits that actually
// changed path relative to their (followed, first) parent are visited.
// The blob at path is constant between two consecutive path-touching
// commits, so comparing BlobAt(ctx, dir, commit, path) against oid at
// exactly those touch commits reproduces the full-chain answer: the
// EARLIEST commit in the FINAL contiguous run where the blob at path
// equals oid is, by construction, the last touch that SET the blob to oid
// with no later touch changing it away. "Final" is what makes
// revert-and-readd honest: if some later first-parent commit changes path
// away from oid (edited, deleted, or reverted to different content) and a
// still-later commit brings oid back, the run before that change is
// discarded and only the current run counts — FirstParentBlobLanding
// reports the commit that landed today's acceptance, never a historical
// first appearance that no longer holds. The path limit is a pure cost
// optimization (one ls-tree per path touch instead of one per first-parent
// commit — the workbench-home walk over a long history was measured in
// minutes without it); the merge/squash/rebase/revert semantics are
// unchanged and stay pinned by this package's own topology tests.
//
// oid never appearing on ref's first-parent chain — including a
// well-formed but unknown oid — is not an error: it yields ("", false,
// nil). ref failing to resolve, or dir not being a git repository, is a
// real operational error wrapping "gitx: FirstParentBlobLanding(REF:PATH@OID)"
// context.
func FirstParentBlobLanding(ctx context.Context, dir, ref, path, oid string) (commit string, found bool, err error) {
	out, runErr := run(ctx, dir, "rev-list", "--first-parent", "--reverse", ref, "--", path)
	if runErr != nil {
		return "", false, fmt.Errorf("gitx: FirstParentBlobLanding(%s:%s@%s): %w", ref, path, oid, runErr)
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return "", false, nil
	}

	var candidate string
	for _, c := range strings.Split(trimmed, "\n") {
		blobOID, present, blobErr := landingBlobAt(ctx, dir, c, path)
		if blobErr != nil {
			return "", false, fmt.Errorf("gitx: FirstParentBlobLanding(%s:%s@%s): %w", ref, path, oid, blobErr)
		}
		if present && blobOID == oid {
			if candidate == "" {
				candidate = c
			}
		} else {
			candidate = ""
		}
	}

	if candidate == "" {
		return "", false, nil
	}
	return candidate, true, nil
}

// landingBlobAt is FirstParentBlobLanding's BlobAt indirection — a package-
// level seam so the walk-cost regression test can count exactly how many
// per-commit tree lookups one landing walk performs (few path touches must
// mean few lookups, regardless of total history length). Production is
// always BlobAt itself; tests override and restore via defer.
var landingBlobAt = BlobAt
