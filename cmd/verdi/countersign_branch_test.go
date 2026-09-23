package main

// SI-227 contract item 3: when gitx.CurrentBranch is empty (a detached
// checkout — a normal CI state) and the process is in CI, the countersign
// source branch comes from the dispatching ref, reusing the same helper
// `verdi sync` already uses for GITHUB_REF_NAME (resolveRefCommit,
// forgeboot.go). Outside CI, or when a branch is checked out, behavior is
// unchanged — never inferred from a SHA.

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/lifecyclecountersign"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
)

// recordingLifecycleCountersignResolver captures the Request
// resolveLifecycleCountersign built, so a test can inspect exactly what
// source branch it resolved without driving the full forge/profile
// pipeline lifecyclecountersign.Resolver itself owns.
type recordingLifecycleCountersignResolver struct {
	got lifecyclecountersign.Request
}

func (r *recordingLifecycleCountersignResolver) Resolve(_ context.Context, request lifecyclecountersign.Request) (lifecyclecountersign.Result, error) {
	r.got = request
	return lifecyclecountersign.Result{Verdict: "unproven", Witnesses: []string{"test:stub"}}, nil
}

// countersignBranchTestRepo is a bare fixturegit repo with one commit on
// main and nothing else — resolveLifecycleCountersign's own accepted-tree
// lookup is best-effort and tolerates the absence of a constitution store.
func countersignBranchTestRepo(t *testing.T) string {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{"README.md": "countersign branch fixture\n"},
		Message: "seed countersign branch fixture",
	}})
	return repo.Dir
}

func clearCIRefEnv(t *testing.T) {
	t.Helper()
	for _, v := range []string{"CI_COMMIT_REF_NAME", "GITHUB_HEAD_REF", "GITHUB_REF_NAME"} {
		t.Setenv(v, "")
	}
}

func TestResolveLifecycleCountersign_DetachedHEADInCIUsesDispatchingRef(t *testing.T) {
	root := countersignBranchTestRepo(t)
	gitOutput(t, root, "checkout", "--detach", "HEAD")
	clearCIRefEnv(t)
	t.Setenv("GITHUB_REF_NAME", "feature/dispatched")

	resolver := &recordingLifecycleCountersignResolver{}
	if _, err := resolveLifecycleCountersign(context.Background(), resolver, root, &store.Manifest{}, &model.Model{}, "story", "main", "deadbeef"); err != nil {
		t.Fatalf("resolveLifecycleCountersign: %v", err)
	}
	if resolver.got.SourceBranch != "feature/dispatched" {
		t.Fatalf("SourceBranch = %q, want the dispatching ref feature/dispatched", resolver.got.SourceBranch)
	}
}

func TestResolveLifecycleCountersign_DetachedHEADOutsideCIStaysEmpty(t *testing.T) {
	root := countersignBranchTestRepo(t)
	gitOutput(t, root, "checkout", "--detach", "HEAD")
	clearCIRefEnv(t)

	resolver := &recordingLifecycleCountersignResolver{}
	if _, err := resolveLifecycleCountersign(context.Background(), resolver, root, &store.Manifest{}, &model.Model{}, "story", "main", "deadbeef"); err != nil {
		t.Fatalf("resolveLifecycleCountersign: %v", err)
	}
	if resolver.got.SourceBranch != "" {
		t.Fatalf("SourceBranch = %q, want empty (detached, no CI ref) — unchanged from today", resolver.got.SourceBranch)
	}
}

func TestResolveLifecycleCountersign_CheckedOutBranchIgnoresCIRefEnv(t *testing.T) {
	root := countersignBranchTestRepo(t)
	// Still on "main" — fixturegit.Build leaves it checked out, not detached.
	t.Setenv("GITHUB_REF_NAME", "some/other/dispatched/ref")

	resolver := &recordingLifecycleCountersignResolver{}
	if _, err := resolveLifecycleCountersign(context.Background(), resolver, root, &store.Manifest{}, &model.Model{}, "story", "main", "deadbeef"); err != nil {
		t.Fatalf("resolveLifecycleCountersign: %v", err)
	}
	if resolver.got.SourceBranch != "main" {
		t.Fatalf("SourceBranch = %q, want the actually checked-out branch main, never the CI ref env var", resolver.got.SourceBranch)
	}
}
