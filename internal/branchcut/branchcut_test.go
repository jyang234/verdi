package branchcut

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

func TestUnwind_EmptyCutIsUnwound(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "a\n"}, Message: "a"}})
	ctx := context.Background()
	if err := gitx.CheckoutNewBranch(ctx, repo.Dir, "close/foo"); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if got := Unwind(ctx, repo.Dir, "main", "close/foo", repo.Head, "recover", &stderr); got != Unwound {
		t.Fatalf("outcome = %s, stderr %q", got, stderr.String())
	}
	if branches, _ := gitx.LocalBranches(ctx, repo.Dir); slices.Contains(branches, "close/foo") {
		t.Fatal("close/foo still exists")
	}
	if cur, _ := gitx.CurrentBranch(ctx, repo.Dir); cur != "main" {
		t.Fatalf("current branch = %q", cur)
	}
	// Base assertion restored (Task 1 review Minor): a clean unwind is
	// silent — only a giving-up path discloses on stderr. Proven by
	// mutation in review: an added stderr line on the Unwound path left
	// the test green without this check.
	if s := strings.TrimSpace(stderr.String()); s != "" {
		t.Fatalf("a clean unwind wrote to stderr = %q, want it silent (only giving-up branches disclose)", s)
	}
}

func TestUnwind_AheadOfCutIsLeft(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "a\n"}, Message: "a"}})
	ctx := context.Background()
	if err := gitx.CheckoutNewBranch(ctx, repo.Dir, "close/foo"); err != nil {
		t.Fatal(err)
	}
	// fixturegit.Dangle does not commit on the current branch (it commits
	// on a throwaway branch of its own, then switches back to "main" and
	// force-deletes it) — the wrong shape for "close/foo carries a commit
	// beyond its cut point". Commit directly on close/foo instead.
	if err := os.WriteFile(filepath.Join(repo.Dir, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gitx.AddPaths(ctx, repo.Dir, "b.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.CreateCommit(ctx, repo.Dir, "own commit"); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if got := Unwind(ctx, repo.Dir, "main", "close/foo", repo.Head, "recover", &stderr); got != LeftAheadOfCut {
		t.Fatalf("outcome = %s", got)
	}
	if !strings.Contains(stderr.String(), "recover: left close/foo in place: it carries commit(s) beyond its cut point") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	// Base assertion restored (Task 1 review Minor): the branch carrying
	// the extra commit must survive — Unwind never discards committed
	// work.
	if branches, _ := gitx.LocalBranches(ctx, repo.Dir); !slices.Contains(branches, "close/foo") {
		t.Fatal("close/foo carrying a commit beyond its cut point was deleted — committed work discarded")
	}
}

// TestUnwind_DetachedHEAD covers the originalBranch == "" case (Task 1
// review Minor, uncovered at the base too): a cut that ran from a
// detached HEAD has no branch name to switch back to, so Unwind restores
// the cut point itself rather than a branch — R-RR3-5 hands recover a
// computed return branch that can legitimately be absent.
func TestUnwind_DetachedHEAD(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "a\n"}, Message: "a"}})
	ctx := context.Background()
	if err := gitx.CheckoutNewBranch(ctx, repo.Dir, "close/foo"); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if got := Unwind(ctx, repo.Dir, "", "close/foo", repo.Head, "recover", &stderr); got != Unwound {
		t.Fatalf("outcome = %s, stderr %q", got, stderr.String())
	}
	if head, err := gitx.RevParse(ctx, repo.Dir, "HEAD"); err != nil || head != repo.Head {
		t.Fatalf("HEAD = %q, err %v; want the cut point %q, detached", head, err, repo.Head)
	}
	if cur, _ := gitx.CurrentBranch(ctx, repo.Dir); cur != "" {
		t.Fatalf("current branch = %q, want detached HEAD (empty)", cur)
	}
	if branches, _ := gitx.LocalBranches(ctx, repo.Dir); slices.Contains(branches, "close/foo") {
		t.Fatal("close/foo still exists")
	}
	if s := strings.TrimSpace(stderr.String()); s != "" {
		t.Fatalf("a clean unwind wrote to stderr = %q, want it silent", s)
	}
}

// TestOutcomeString covers Outcome's String() over its closed set plus
// the self-naming fallback for a value outside it (the package's usual
// shape, e.g. internal/execworkspace's Outcome.String).
func TestOutcomeString(t *testing.T) {
	cases := []struct {
		outcome Outcome
		want    string
	}{
		{Unwound, "unwound"},
		{LeftUninspectable, "left-uninspectable"},
		{LeftAheadOfCut, "left-ahead-of-cut"},
		{LeftSwitchFailed, "left-switch-failed"},
		{LeftDeleteFailed, "left-delete-failed"},
		{Outcome(99), "branchcut.Outcome(99)"},
	}
	for _, tc := range cases {
		if got := tc.outcome.String(); got != tc.want {
			t.Errorf("Outcome(%d).String() = %q, want %q", int(tc.outcome), got, tc.want)
		}
	}
}
