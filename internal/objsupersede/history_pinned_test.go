package objsupersede

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/gitx/readcensus"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
	"github.com/jyang234/verdi/internal/specstate"
)

// TestNewHistory_PinnedRequestTakesThePinnedCommit (ledger SI-356): in a
// request that pinned its accepted HEAD, a History pins that commit
// without resolving the ref again, and answers every fact as an unpinned
// History does; an unresolvable default branch stays unproven.
func TestNewHistory_PinnedRequestTakesThePinnedCommit(t *testing.T) {
	hermetic(t)
	repo := scenario.Build(t, "chain")
	ctx := context.Background()
	unpinned := NewHistory(ctx, repo.Dir)
	if !unpinned.ok || unpinned.head == "" {
		t.Fatalf("unpinned History = %+v, want the default branch pinned", unpinned)
	}

	pinned := specstate.WithAcceptedHead(ctx, repo.Dir)
	census := &readcensus.Census{}
	h := NewHistory(gitx.WithObserver(pinned, census), repo.Dir)
	if h.head != unpinned.head || h.branch.Ref != unpinned.branch.Ref || h.unread != "" {
		t.Fatalf("pinned History = (%s, %s, %q), unpinned (%s, %s)", h.head, h.branch.Ref, h.unread, unpinned.head, unpinned.branch.Ref)
	}
	if events := census.Events(); len(events) != 0 {
		t.Fatalf("a pinned History ran git to pin its head: %+v", events)
	}
	for _, spec := range []string{"successor", "successor-v2", "closed-feature", "nowhere"} {
		if got, want := h.Acceptance(pinned, spec), unpinned.Acceptance(ctx, spec); got != want {
			t.Fatalf("pinned Acceptance(%s) = %+v, unpinned %+v", spec, got, want)
		}
	}

	unresolved := scenario.Build(t, "accepted")
	gitIn(t, unresolved.Dir, "update-ref", "-d", "refs/remotes/origin/main")
	none := specstate.WithAcceptedHead(ctx, unresolved.Dir)
	if got := NewHistory(none, unresolved.Dir).Acceptance(none, "successor"); got.State != FactUnproven {
		t.Fatalf("an unresolvable default branch's Acceptance = %+v, want unproven", got)
	}
}
