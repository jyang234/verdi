package main

import (
	"context"
	"testing"

	"github.com/OWNER/verdi/internal/forge"
	"github.com/OWNER/verdi/internal/forge/fake"
)

// newFeedForTest wires a forgeCommentFeed over the fake with a resolvable
// default branch (CI_DEFAULT_BRANCH wins in lint.ResolveDefaultBranch, so
// no git is touched — hermetic, CLAUDE.md: no network in any test).
func newFeedForTest(t *testing.T, f forge.Forge) *forgeCommentFeed {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	return newForgeCommentFeed(f, t.TempDir())
}

// TestForgeCommentFeed_JoinsCommentsAndResolution proves the adapter finds
// the spec's open MR (source branch design/<name>), lists its full feed in
// forge order (token-bearing AND token-free comments both surface — the
// caller does the inbox-tray split, never the feed), and stamps Resolved
// from the comment's forge thread state.
func TestForgeCommentFeed_JoinsCommentsAndResolution(t *testing.T) {
	f := fake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "42", SourceBranch: "design/refi-decline-flow"})
	// A token-bearing comment on a resolved thread.
	f.SeedComment("42", forge.Comment{ID: "c1", Author: "reviewer", Body: "[vd:ac-1] please tighten", ThreadID: "t1"})
	// A token-free comment on an unresolved thread.
	f.SeedComment("42", forge.Comment{ID: "c2", Author: "reviewer", Body: "general remark", ThreadID: "t2"})
	f.SeedThreadResolution("42", forge.ThreadResolution{ThreadID: "t1", Resolved: true})

	feed := newFeedForTest(t, f)
	comments, ok, err := feed.ListMRComments(context.Background(), "refi-decline-flow")
	if err != nil {
		t.Fatalf("ListMRComments: %v", err)
	}
	if !ok {
		t.Fatal("ok = false, want true (spec has an open MR)")
	}
	if len(comments) != 2 {
		t.Fatalf("got %d comments, want 2 (feed never drops the token-free comment)", len(comments))
	}
	// Forge order preserved.
	if comments[0].ID != "c1" || comments[1].ID != "c2" {
		t.Fatalf("comment order = %q,%q, want c1,c2", comments[0].ID, comments[1].ID)
	}
	if comments[0].Body != "[vd:ac-1] please tighten" || comments[0].Author != "reviewer" {
		t.Fatalf("comment c1 body/author mismatch: %+v", comments[0])
	}
	if !comments[0].Resolved {
		t.Error("c1 (thread t1 resolved) Resolved = false, want true")
	}
	if comments[1].Resolved {
		t.Error("c2 (thread t2 unresolved) Resolved = true, want false")
	}
}

// TestForgeCommentFeed_NoThreadNeverResolved proves a comment with no
// thread id at all (a bare general note) is never reported resolved.
func TestForgeCommentFeed_NoThreadNeverResolved(t *testing.T) {
	f := fake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "7", SourceBranch: "design/refi-decline-flow"})
	f.SeedComment("7", forge.Comment{ID: "c1", Author: "reviewer", Body: "no thread here"})

	feed := newFeedForTest(t, f)
	comments, ok, err := feed.ListMRComments(context.Background(), "refi-decline-flow")
	if err != nil {
		t.Fatalf("ListMRComments: %v", err)
	}
	if !ok || len(comments) != 1 {
		t.Fatalf("ok=%v len=%d, want true,1", ok, len(comments))
	}
	if comments[0].Resolved {
		t.Error("thread-less comment Resolved = true, want false")
	}
}

// TestForgeCommentFeed_NoOpenMR proves a spec whose design branch has no
// open MR is honestly not under review (ok=false), not an error.
func TestForgeCommentFeed_NoOpenMR(t *testing.T) {
	f := fake.New()
	// An open MR exists, but for a DIFFERENT design branch.
	f.SeedOpenMR("main", forge.OpenMR{ID: "9", SourceBranch: "design/other-spec"})

	feed := newFeedForTest(t, f)
	comments, ok, err := feed.ListMRComments(context.Background(), "refi-decline-flow")
	if err != nil {
		t.Fatalf("ListMRComments: %v", err)
	}
	if ok {
		t.Error("ok = true, want false (no open MR for this spec's design branch)")
	}
	if comments != nil {
		t.Errorf("comments = %v, want nil", comments)
	}
}

// TestForgeCommentFeed_NoDefaultBranch proves an unresolvable default
// branch (no CI env, no git remote) yields ok=false, never an error — the
// board simply cannot locate an MR to mirror.
func TestForgeCommentFeed_NoDefaultBranch(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "")
	feed := newForgeCommentFeed(fake.New(), t.TempDir())
	comments, ok, err := feed.ListMRComments(context.Background(), "refi-decline-flow")
	if err != nil {
		t.Fatalf("ListMRComments: %v", err)
	}
	if ok || comments != nil {
		t.Errorf("ok=%v comments=%v, want false,nil", ok, comments)
	}
}
