package gitx

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// TestReadSession_ReachableFromHEADMatchesExec proves the walk answers
// every commit shape exactly as the per-commit path does: HEAD, an
// ancestor, a dangling commit, an unknown full id, an abbreviated id, an
// annotated tag's own object id, a blob id, a non-hex name — in a full
// clone and in a shallow one.
func TestReadSession_ReachableFromHEADMatchesExec(t *testing.T) {
	repo := sessionRepo(t)
	dangling := fixturegit.Dangle(t, repo, map[string]string{"gone.txt": "gone\n"}, "dangling")
	gitT(t, repo.Dir, "-c", "user.name=t", "-c", "user.email=t@t", "tag", "-a", "-m", "annotated", "v1", repo.Heads[0])
	tagObject := gitT(t, repo.Dir, "rev-parse", "v1")
	blob := gitT(t, repo.Dir, "rev-parse", "HEAD:plain.txt")
	head := gitT(t, repo.Dir, "rev-parse", "HEAD")
	commits := []string{head, repo.Heads[0], repo.Heads[1], dangling, strings.Repeat("0", 40), repo.Heads[0][:7], tagObject, blob, "not-a-commit", strings.ToUpper(repo.Heads[0])}

	shallow := fixturegit.ShallowClone(t, repo, 1)
	for _, dir := range []string{repo.Dir, shallow} {
		ResetShallowCache()
		ctx, release := WithReadSession(context.Background(), dir)
		for _, headRev := range []string{"HEAD", head} {
			for _, c := range commits {
				want, wantErr := ReachableFromHEAD(context.Background(), dir, c, headRev)
				got, gotErr := ReachableFromHEAD(ctx, dir, c, headRev)
				if got != want || errText(gotErr) != errText(wantErr) {
					t.Errorf("ReachableFromHEAD(%s, %s) in %s in a session = (%v, %v), want (%v, %v)", c, headRev, filepath.Base(dir), got, gotErr, want, wantErr)
				}
			}
		}
		release()
	}
}

// TestReadSession_ReachabilityIsOneWalk pins the cost and kills a walk
// that drops a commit: every reachable full commit id is answered from one
// rev-list walk, with no rev-parse or merge-base per commit. A walk that
// missed any of them would send it down the per-commit path, which this
// count sees.
func TestReadSession_ReachabilityIsOneWalk(t *testing.T) {
	repo := sessionRepo(t)
	obs := newCountingObserver()
	ctx, release := WithReadSession(WithObserver(context.Background(), obs), repo.Dir)
	defer release()
	reachable := append([]string{gitT(t, repo.Dir, "rev-parse", "HEAD")}, repo.Heads...)
	for range 3 {
		for _, c := range reachable {
			got, err := ReachableFromHEAD(ctx, repo.Dir, c, "HEAD")
			if err != nil || got != Reachable {
				t.Fatalf("ReachableFromHEAD(%s) = (%v, %v), want reachable", c, got, err)
			}
		}
	}
	if obs.count("rev-list") != 1 || obs.total() != 1 {
		t.Fatalf("%d reachability queries launched %v, want exactly one rev-list walk", 3*len(reachable), obs.argv)
	}
}
