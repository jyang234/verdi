package specdocload

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/gitx/readcensus"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specstate"
)

// TestLoad_PinnedRequestReadsThePinnedHead (ledger SI-356; Wave 6 §5.3):
// in a request that pinned its accepted HEAD (one read session, one
// specstate.WithAcceptedHead), every mode loads exactly what an unpinned
// load loads — the accepted reading, a pinned commit's, and the working
// tree's — and the load itself names the default branch by its id alone:
// no discovery, no explicit resolution and no read of the ref.
func TestLoad_PinnedRequestReadsThePinnedHead(t *testing.T) {
	repo := buildRepo(t)
	git(t, repo.Dir, "update-ref", "refs/remotes/origin/main", repo.Head)
	git(t, repo.Dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	t.Setenv("CI_DEFAULT_BRANCH", "")
	acc := readcensus.Accepted{Spellings: []string{"origin/main", "refs/remotes/origin/main"}, IDs: []string{repo.Head}}
	for _, mode := range []struct {
		name string
		req  Request
	}{
		{"accepted", Request{Root: repo.Dir, Name: "lockbox", Mode: ModeAccepted, Kind: specdoc.KindSpec}},
		{"at a commit", Request{Root: repo.Dir, Name: "lockbox", Mode: ModeAt, At: repo.Head, Kind: specdoc.KindSpec}},
		{"working tree", Request{Root: repo.Dir, Name: "lockbox", Mode: ModeWorkingTree, Kind: specdoc.KindSpec}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			want, wantErr := Load(context.Background(), mode.req)
			if wantErr != nil {
				t.Fatal(wantErr)
			}
			ctx, release := gitx.WithReadSession(context.Background(), repo.Dir)
			defer release()
			ctx = specstate.WithAcceptedHead(ctx, repo.Dir)
			census := &readcensus.Census{}
			got, err := Load(gitx.WithObserver(ctx, census), mode.req)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("pinned Load = %+v\nunpinned %+v", got, want)
			}
			if got.Accepted != (AcceptedHead{Branch: "main", Ref: "origin/main", Commit: repo.Head}) {
				t.Fatalf("Accepted = %+v, want origin/main at %s", got.Accepted, repo.Head)
			}
			b := census.Budget(acc)
			if chains, explicit, operand := b.Resolutions(); chains != 0 || explicit != 0 || operand != 0 {
				t.Fatalf("the pinned load resolved the ref again: chains %d, explicit %d, operand:\n  %s", chains, explicit, strings.Join(b.Operand, "\n  "))
			}
		})
	}
}

// TestResolveDefaultHistory_Pinned: the views' history takes a pinned
// head without a read; an unpinned one resolves it, and an unresolvable
// default branch is unresolved either way.
func TestResolveDefaultHistory_Pinned(t *testing.T) {
	repo := buildRepo(t)
	unpinned := resolveDefaultHistory(context.Background(), repo.Dir)
	if !unpinned.resolved || unpinned.head != repo.Head {
		t.Fatalf("unpinned history = %+v, want main at %s", unpinned, repo.Head)
	}
	ctx := specstate.WithAcceptedHead(context.Background(), repo.Dir)
	census := &readcensus.Census{}
	pinned := resolveDefaultHistory(gitx.WithObserver(ctx, census), repo.Dir)
	if pinned.head != unpinned.head || pinned.resolved != unpinned.resolved || pinned.branch.Ref != unpinned.branch.Ref {
		t.Fatalf("pinned history = %+v, unpinned %+v", pinned, unpinned)
	}
	for _, e := range census.Events() {
		if len(e.Args) > 0 && e.Args[0] == "rev-parse" && e.Args[len(e.Args)-1] == "main" {
			t.Fatalf("the pinned history resolved the ref again: %v", e.Args)
		}
	}

	t.Setenv("CI_DEFAULT_BRANCH", "")
	none := specstate.WithAcceptedHead(context.Background(), repo.Dir)
	if h := resolveDefaultHistory(none, repo.Dir); h.resolved || h.head != unresolvedHead {
		t.Fatalf("an unresolvable default branch's history = %+v, want unresolved", h)
	}
}
