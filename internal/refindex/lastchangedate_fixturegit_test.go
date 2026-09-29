package refindex

import (
	"context"
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specstate"
)

// countingGitRunner decorates the production GitRunner, counting each
// default-branch zone listing and recording every CommitDates call — the
// witnesses that the default-branch walk runs once per render (co-1) and
// that each walk reads its dates in one port call.
type countingGitRunner struct {
	GitRunner

	mu          sync.Mutex
	zoneListing map[string]int // ListTree calls per path
	dateBatches [][]string
}

func (c *countingGitRunner) ListTree(ctx context.Context, dir, ref, path string) ([]string, error) {
	c.mu.Lock()
	c.zoneListing[path]++
	c.mu.Unlock()
	return c.GitRunner.ListTree(ctx, dir, ref, path)
}

func (c *countingGitRunner) CommitDates(ctx context.Context, dir string, revs []string) (map[string]string, error) {
	c.mu.Lock()
	c.dateBatches = append(c.dateBatches, append([]string(nil), revs...))
	c.mu.Unlock()
	return c.GitRunner.CommitDates(ctx, dir, revs)
}

// countingStateResolver decorates the production StateResolver, recording
// each ResolveMany call's candidate paths (sorted) — the witness of exactly
// one batch per walk, components included.
type countingStateResolver struct {
	StateResolver

	mu      sync.Mutex
	batches [][]string
}

func (c *countingStateResolver) ResolveMany(ctx context.Context, root string, candidates []specstate.Candidate) ([]specstate.Result, error) {
	paths := make([]string, len(candidates))
	for i, cand := range candidates {
		paths[i] = cand.Path
	}
	sort.Strings(paths)
	c.mu.Lock()
	c.batches = append(c.batches, paths)
	c.mu.Unlock()
	return c.StateResolver.ResolveMany(ctx, root, candidates)
}

// gitArgvObserver records every git invocation's first argument.
type gitArgvObserver struct {
	mu    sync.Mutex
	verbs map[string]int
}

func (o *gitArgvObserver) Observe(_ string, args []string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(args) > 0 {
		o.verbs[args[0]]++
	}
}

// TestComputeIndex_LastChangeDatesFixturegit is ac-1's behavioral
// obligation over a fixturegit repository whose every commit date is
// pinned and DIFFERENT, through the real production adapters
// (NewGitRunner, NewStateResolver), each wrapped in a counting decorator:
//
//   - a component and a story landed in the seed commit, then a LATER,
//     unrelated default-branch commit — so the default branch's tip is not
//     any entry's landing commit, and a date read from the tip is caught;
//   - a component landed, then edited in place by a later landed commit —
//     so its date must move to the edit's commit;
//   - a design branch whose tip is dated later still.
//
// Each entry's date equals its own tip's or landing commit's committer
// date; the default-branch walk lists each zone exactly once (never
// repeated); each walk makes exactly one ResolveMany call, the
// default-branch one carrying the components alongside the story; and each
// walk reads its dates in exactly one port call, one git process apiece.
func TestComputeIndex_LastChangeDatesFixturegit(t *testing.T) {
	// fixturegit pins its layer to 2024-01-01T00:00:00Z.
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/root-spec/spec.md":    componentSpecMD("root-spec", "active"),
			".verdi/specs/active/landed-story/spec.md": storySpecAcceptedMD("landed-story"),
		},
		Message: "seed: a component and a story land together",
	}})
	setDefaultBranchSymref(t, repo.Dir, "main")
	seed := repo.Head

	writeAndCommitAt(t, repo.Dir, "1704844800 +0000", map[string]string{ // 2024-01-10
		".verdi/specs/active/edited-spec/spec.md": componentSpecMD("edited-spec", "active"),
	}, "edited-spec lands")
	editedLanding := writeAndCommitAt(t, repo.Dir, "1705708800 +0000", map[string]string{ // 2024-01-20
		".verdi/specs/active/edited-spec/spec.md": componentSpecMD("edited-spec", "active") + "\nAn in-place edit that lands.\n",
	}, "edited-spec: an in-place edit lands")
	writeAndCommitAt(t, repo.Dir, "1706745600 +0000", map[string]string{ // 2024-02-01
		"NOTES.md": "an unrelated default-branch commit\n",
	}, "an unrelated later default-branch commit")

	checkoutNewBranch(t, repo.Dir, "design/aged-draft")
	writeAndCommitAt(t, repo.Dir, "1707955200 +0000", map[string]string{ // 2024-02-15
		".verdi/specs/active/aged-draft/spec.md": componentSpecMD("aged-draft", "draft"),
	}, "aged-draft")
	checkoutExisting(t, repo.Dir, "main")

	deps := &countingGitRunner{GitRunner: NewGitRunner(), zoneListing: map[string]int{}}
	resolver := &countingStateResolver{StateResolver: NewStateResolver()}
	obs := &gitArgvObserver{verbs: map[string]int{}}
	ctx := gitx.WithObserver(context.Background(), obs)

	got, err := ComputeIndex(ctx, repo.Dir, deps, resolver)
	if err != nil {
		t.Fatalf("ComputeIndex: %v", err)
	}

	for _, tc := range []struct {
		ref, want, why string
	}{
		{"spec/root-spec", "2024-01-01T00:00:00+00:00", "its landing (seed) commit, not the default branch's later tip (2024-02-01)"},
		{"spec/landed-story", "2024-01-01T00:00:00+00:00", "a feature/story entry's landing commit, not the tip"},
		{"spec/edited-spec", "2024-01-20T00:00:00+00:00", "the landed in-place edit moved it from 2024-01-10"},
		{"spec/aged-draft", "2024-02-15T00:00:00+00:00", "its own branch tip"},
	} {
		e := entryByRef(t, got, tc.ref)
		if e.Date != tc.want {
			t.Errorf("%s Date = %q, want %q (%s)", tc.ref, e.Date, tc.want, tc.why)
		}
		if e.DateDisclosed != nil {
			t.Errorf("%s DateDisclosed = %+v, want nil", tc.ref, e.DateDisclosed)
		}
	}

	// The default-branch walk runs once: each zone is listed exactly once.
	for _, zone := range []string{".verdi/specs/active", ".verdi/specs/archive"} {
		if n := deps.zoneListing[zone]; n != 1 {
			t.Errorf("default-branch zone %s listed %d times, want exactly 1 (the walk must not repeat)", zone, n)
		}
	}

	// One ResolveMany per walk; the default-branch batch carries the
	// components beside the story.
	wantBatches := [][]string{
		{".verdi/specs/active/edited-spec/spec.md", ".verdi/specs/active/landed-story/spec.md", ".verdi/specs/active/root-spec/spec.md"},
		{".verdi/specs/active/aged-draft/spec.md"},
	}
	if !reflect.DeepEqual(resolver.batches, wantBatches) {
		t.Errorf("ResolveMany calls = %q, want exactly one per walk: %q", resolver.batches, wantBatches)
	}

	// One date read per walk: the landing commits (the shared seed asked
	// once), then the design tip.
	wantLandings := []string{seed, editedLanding}
	sort.Strings(wantLandings)
	wantDateBatches := [][]string{wantLandings, {"design/aged-draft"}}
	if !reflect.DeepEqual(deps.dateBatches, wantDateBatches) {
		t.Errorf("CommitDates calls = %q, want %q", deps.dateBatches, wantDateBatches)
	}
	// ...and each of those is one git process, with no per-entry `git log`.
	if n := obs.verbs["cat-file"]; n != 2 {
		t.Errorf("git cat-file invocations = %d, want 2 (one date read per walk)", n)
	}
	if n := obs.verbs["log"]; n != 0 {
		t.Errorf("git log invocations = %d, want 0 (no per-entry date read)", n)
	}
}
