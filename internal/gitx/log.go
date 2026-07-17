package gitx

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Commit is one entry from `git log`: enough to render dex's temporal
// banners (living-gated build stamp, authored-living last-modified) and
// "what changed" feed (05 §Verdi-dex mechanics) without dex needing to
// shell out to git itself.
type Commit struct {
	// SHA is the full 40-character commit object id.
	SHA string
	// Date is the commit's committer date in strict ISO-8601
	// (`git log --format=%cI`), the timezone-stable form fixturegit itself
	// pins commits to — never wall-clock, per dex's determinism
	// requirement (PLAN.md Phase 12: "no wall-clock or randomness in
	// generated artifacts except declared stamps").
	Date string
	// Author is the commit's author name (`%an`).
	Author string
	// Subject is the commit's first message line (`%s`).
	Subject string
}

// logRecordSep and logFieldSep delimit `git log --format` records/fields
// with control characters that never legitimately appear in a commit's
// author name or subject line, so a subject containing an ordinary
// tab/newline cannot be mistaken for a field boundary.
const (
	logFieldSep  = "\x1f"
	logRecordSep = "\x1e"
)

const logFormat = "%H" + logFieldSep + "%cI" + logFieldSep + "%an" + logFieldSep + "%s" + logRecordSep

// Log returns the commit history reachable from rev that touched any of
// paths (or the whole tree, if paths is empty), most-recent-first —
// `git log --format=... rev -- paths...`. paths are repo-relative,
// forward-slashed. A rev with no matching history (e.g. paths that were
// never touched) is not an error: it returns a nil slice.
func Log(ctx context.Context, dir, rev string, paths ...string) ([]Commit, error) {
	args := []string{"log", "--format=" + logFormat, rev}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	out, err := run(ctx, dir, args...)
	if err != nil {
		return nil, fmt.Errorf("gitx: Log(%s): %w", rev, err)
	}
	return parseLog(string(out))
}

// LastCommit returns the single most recent commit that touched path as of
// rev — `git log -1 rev -- path` — the mechanism behind dex's
// authored-living "last-modified from git" banner (01 §Temporal classes).
// ok is false, with a nil error, when path has no history at rev (an
// operational impossibility for a real committed file, but LastCommit stays
// total rather than panicking on a caller's mistake).
func LastCommit(ctx context.Context, dir, rev, path string) (commit Commit, ok bool, err error) {
	commits, err := Log(ctx, dir, rev, path)
	if err != nil {
		return Commit{}, false, err
	}
	if len(commits) == 0 {
		return Commit{}, false, nil
	}
	return commits[0], true, nil
}

// CommitDate returns rev's own committer date in strict ISO-8601 form —
// the "build stamp" date dex's living-gated banners and by-kind/by-service
// index pages use (`main @ <sha> · <date>`), always the resolved commit's
// own date, never time.Now().
func CommitDate(ctx context.Context, dir, rev string) (string, error) {
	out, err := run(ctx, dir, "log", "-1", "--format=%cI", rev)
	if err != nil {
		return "", fmt.Errorf("gitx: CommitDate(%s): %w", rev, err)
	}
	date := strings.TrimSpace(string(out))
	if date == "" {
		return "", fmt.Errorf("gitx: CommitDate(%s): no such commit", rev)
	}
	// git's %cI renders a UTC offset as "+00:00" or "Z" depending on the git
	// version — non-deterministic output (CLAUDE.md: deterministic artifacts).
	// Normalize to a canonical numeric offset, git-version-independent,
	// preserving any non-UTC offset.
	t, perr := time.Parse(time.RFC3339, date)
	if perr != nil {
		return "", fmt.Errorf("gitx: CommitDate(%s): parsing %q: %w", rev, date, perr)
	}
	return t.Format("2006-01-02T15:04:05-07:00"), nil
}

// CommitDateOnly returns rev's own committer date as a YYYY-MM-DD string —
// CommitDate's first 10 characters — the frozen.at derivation every
// commit-derived stamp uses (L-M4: never wall clock). Shared by every
// caller that stamps a Frozen record from a commit (cmd/verdi's align and
// accept verbs inline this same two-step derivation today; internal/
// workbench's obligation author is the first non-cmd/verdi caller), so the
// "derive a date from a commit, fail closed if git's own output is somehow
// too short" logic exists in one shared home rather than copy-pasted at
// each new call site (CLAUDE.md shared-homes rule).
func CommitDateOnly(ctx context.Context, dir, rev string) (string, error) {
	full, err := CommitDate(ctx, dir, rev)
	if err != nil {
		return "", err
	}
	if len(full) < 10 {
		return "", fmt.Errorf("gitx: CommitDateOnly(%s): commit date %q too short to derive a YYYY-MM-DD date", rev, full)
	}
	return full[:10], nil
}

// PickaxeCommit runs `git log -S<identity> -1 --format=%H -- <paths...>`
// (or the whole tree, if paths is empty) — spec/verification-extractor
// dc-4's witness-commit discovery mechanism, the smallest honest tool
// available without building history-walking graph-diff machinery. ok is
// false, with a nil error, when no commit in the repository's history ever
// changed identity's occurrence count under paths (a moved directory, an
// unrelated rename): the caller discloses this as witness-unresolved,
// never fabricates a placeholder commit.
//
// This is a strict, fixed-string pickaxe search, not a causal claim: a hit
// only proves identity's occurrence count changed in that commit somewhere
// under paths (a comment, a test, an unrelated symbol sharing the name) —
// never that the commit removed one specific graph element. Every caller
// of this function must disclose its result as a CANDIDATE witness, never
// as a causally-verified removal (dc-4's corrected candor).
func PickaxeCommit(ctx context.Context, dir, identity string, paths ...string) (sha string, ok bool, err error) {
	args := []string{"log", "-S" + identity, "-1", "--format=%H"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	out, err := run(ctx, dir, args...)
	if err != nil {
		return "", false, fmt.Errorf("gitx: PickaxeCommit(%s): %w", identity, err)
	}
	sha = strings.TrimSpace(string(out))
	if sha == "" {
		return "", false, nil
	}
	return sha, true, nil
}

// parseLog splits raw `git log --format=logFormat` output into Commits.
func parseLog(raw string) ([]Commit, error) {
	records := strings.Split(raw, logRecordSep)
	var commits []Commit
	for _, rec := range records {
		rec = strings.TrimPrefix(rec, "\n")
		if strings.TrimSpace(rec) == "" {
			continue
		}
		fields := strings.Split(rec, logFieldSep)
		if len(fields) != 4 {
			return nil, fmt.Errorf("gitx: parseLog: malformed record %q (want 4 fields, got %d)", rec, len(fields))
		}
		commits = append(commits, Commit{SHA: fields[0], Date: fields[1], Author: fields[2], Subject: fields[3]})
	}
	return commits, nil
}
