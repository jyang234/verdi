package fixturegit

import (
	"strings"
	"testing"
)

// Dangle commits one throwaway layer on a new branch off repo's current
// HEAD, then force-deletes that branch — leaving the new commit as a real,
// locally-present loose object that no branch or ref anywhere reaches. It
// returns the dangling commit's SHA and leaves repo checked out back on
// "main" at its original HEAD, unchanged.
//
// This is the exact X-11/X-11b shape (extensibility-chronicle
// 2026-07-17): a commit that "still dangled in the worktree's object
// store" after losing every ref that kept it reachable — VL-009's own
// false green (a locally-dangling object satisfies "is a real commit"
// even though no branch or ref anywhere reaches it) and the shape any
// reachability check built on mere object existence, rather than
// ancestry, must be tightened against (spec/evidence-resilience ac-3).
// Uses the same fixed author/committer identity and date Build uses, so
// the returned SHA is byte-stable across machines and runs.
func Dangle(t testing.TB, repo *Repo, files map[string]string, message string) string {
	t.Helper()
	if len(files) == 0 {
		t.Fatal("fixturegit: Dangle called with no files")
	}
	if strings.TrimSpace(message) == "" {
		t.Fatal("fixturegit: Dangle called with an empty commit message")
	}

	const throwawayBranch = "fixturegit-dangle-throwaway"
	runGit(t, repo.Dir, nil, "checkout", "--quiet", "-b", throwawayBranch)

	for path, content := range files {
		writeFile(t, repo.Dir, path, content)
	}
	runGit(t, repo.Dir, nil, "add", "-A")
	runGit(t, repo.Dir, commitEnvironment(), "commit", "--quiet", "--no-verify", "-m", message)
	sha := strings.TrimSpace(runGitOutput(t, repo.Dir, nil, "rev-parse", "HEAD"))

	// Back to main at its original head, then drop the only ref that kept
	// the new commit reachable — it remains a real loose object (no gc
	// runs in these fixtures; Build already disables auto gc/maintenance
	// on this repo) but is no longer an ancestor of anything.
	runGit(t, repo.Dir, nil, "checkout", "--quiet", "main")
	runGit(t, repo.Dir, nil, "branch", "-D", throwawayBranch)

	return sha
}
