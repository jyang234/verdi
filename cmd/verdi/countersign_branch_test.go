package main

// SI-257 (superseding SI-227 contract item 3's private fallback): the
// countersign's source branch is repositoryfacts.Snapshot.BranchBeingClosed
// — the checked-out branch when there is one; on a detached checkout, the
// validated CI ref (a single provider's branch ref whose exact
// refs/remotes/origin/<name> equals HEAD); otherwise "", which the
// countersign reads as unproven "source-branch". GITHUB_HEAD_REF is never
// consulted, and an unvalidated ref-name variable is never used.

import (
	"context"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/countersign"
	"github.com/jyang234/verdi/internal/fixturegit"
	forgefake "github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/lifecyclecountersign"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
)

// recordingLifecycleCountersignResolver captures the Request
// resolveLifecycleCountersign built, so a test can inspect exactly what
// source branch it resolved, then delegates to next (or returns a stub
// unproven result when next is nil).
type recordingLifecycleCountersignResolver struct {
	got  lifecyclecountersign.Request
	next lifecycleCountersignResolver
}

func (r *recordingLifecycleCountersignResolver) Resolve(ctx context.Context, request lifecyclecountersign.Request) (lifecyclecountersign.Result, error) {
	r.got = request
	if r.next != nil {
		return r.next.Resolve(ctx, request)
	}
	return lifecyclecountersign.Result{Verdict: "unproven", Witnesses: []string{"test:stub"}}, nil
}

// countersignBranchTestRepo is a fixturegit repo with two commits on main
// (A, then B at HEAD) — resolveLifecycleCountersign's own accepted-tree
// lookup is best-effort and tolerates the absence of a constitution store.
func countersignBranchTestRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"README.md": "countersign branch fixture\n"}, Message: "seed countersign branch fixture"},
		{Files: map[string]string{"NOTES.md": "second commit\n"}, Message: "second countersign branch commit"},
	})
}

// ciRefEnvVars is every variable the CI-ref fact reads, plus
// GITHUB_HEAD_REF, which it must never read.
var ciRefEnvVars = []string{
	"GITHUB_ACTIONS", "GITHUB_REF_NAME", "GITHUB_REF_TYPE", "GITHUB_HEAD_REF",
	"GITLAB_CI", "CI_COMMIT_REF_NAME", "CI_COMMIT_BRANCH", "CI_COMMIT_TAG",
}

// setCIRefEnv clears every CI-ref variable (so the host's own CI
// environment never leaks into a test), then sets env.
func setCIRefEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, v := range ciRefEnvVars {
		t.Setenv(v, "")
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func githubBranchEnv(name string) map[string]string {
	return map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REF_TYPE": "branch", "GITHUB_REF_NAME": name}
}

func TestResolveLifecycleCountersign_SourceBranch(t *testing.T) {
	tests := []struct {
		name string
		// detach leaves HEAD detached at B; otherwise main stays checked out.
		detach bool
		// trackingAtHead/trackingAtParent name remote-tracking refs created
		// at HEAD (B) and at its parent (A).
		trackingAtHead, trackingAtParent []string
		env                              map[string]string
		want                             string
	}{
		{
			name: "detached, validated GitHub ref", detach: true,
			trackingAtHead: []string{"feature/dispatched"},
			env:            githubBranchEnv("feature/dispatched"),
			want:           "feature/dispatched",
		},
		{
			name: "detached, validated GitLab ref", detach: true,
			trackingAtHead: []string{"feature/dispatched"},
			env:            map[string]string{"GITLAB_CI": "true", "CI_COMMIT_REF_NAME": "feature/dispatched", "CI_COMMIT_BRANCH": "feature/dispatched"},
			want:           "feature/dispatched",
		},
		{
			name: "detached, GITHUB_HEAD_REF is ignored", detach: true,
			trackingAtHead: []string{"feature/head-ref"},
			env:            map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REF_TYPE": "branch", "GITHUB_REF_NAME": "17/merge", "GITHUB_HEAD_REF": "feature/head-ref"},
			want:           "",
		},
		{
			name: "detached, GITHUB_HEAD_REF alone is ignored", detach: true,
			trackingAtHead: []string{"feature/head-ref"},
			env:            map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REF_TYPE": "branch", "GITHUB_HEAD_REF": "feature/head-ref"},
			want:           "",
		},
		{
			name: "detached, remote-tracking head is not HEAD", detach: true,
			trackingAtParent: []string{"feature/dispatched"},
			env:              githubBranchEnv("feature/dispatched"),
			want:             "",
		},
		{
			name: "detached, remote-tracking ref missing", detach: true,
			env:  githubBranchEnv("feature/dispatched"),
			want: "",
		},
		{
			// The pre-SI-257 private fallback read this variable unvalidated.
			name: "detached, unvalidated ref name without a provider flag", detach: true,
			trackingAtHead: []string{"feature/dispatched"},
			env:            map[string]string{"GITHUB_REF_NAME": "feature/dispatched", "CI_COMMIT_REF_NAME": "feature/dispatched"},
			want:           "",
		},
		{
			name: "detached outside CI", detach: true,
			want: "",
		},
		{
			name:           "checked-out branch wins over a validated CI ref",
			trackingAtHead: []string{"feature/dispatched"},
			env:            githubBranchEnv("feature/dispatched"),
			want:           "main",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := countersignBranchTestRepo(t)
			for _, name := range tt.trackingAtHead {
				gitOutput(t, repo.Dir, "update-ref", "refs/remotes/origin/"+name, repo.Heads[1])
			}
			for _, name := range tt.trackingAtParent {
				gitOutput(t, repo.Dir, "update-ref", "refs/remotes/origin/"+name, repo.Heads[0])
			}
			if tt.detach {
				gitOutput(t, repo.Dir, "checkout", "--quiet", "--detach", "HEAD")
			}
			setCIRefEnv(t, tt.env)

			resolver := &recordingLifecycleCountersignResolver{}
			if _, err := resolveLifecycleCountersign(context.Background(), resolver, repo.Dir, &store.Manifest{}, &model.Model{}, "story", "main", repo.Head); err != nil {
				t.Fatalf("resolveLifecycleCountersign: %v", err)
			}
			if resolver.got.SourceBranch != tt.want {
				t.Fatalf("SourceBranch = %q, want %q", resolver.got.SourceBranch, tt.want)
			}
		})
	}
}

// TestResolveLifecycleCountersign_RemoteTrackingMismatchIsUnprovenSourceBranch
// drives the real lifecycle countersign resolver: a detached checkout whose
// CI ref's remote-tracking head is not HEAD yields no source branch, which
// the countersign reports as blocking unproven "source-branch".
func TestResolveLifecycleCountersign_RemoteTrackingMismatchIsUnprovenSourceBranch(t *testing.T) {
	repo := countersignBranchTestRepo(t)
	gitOutput(t, repo.Dir, "update-ref", "refs/remotes/origin/feature/dispatched", repo.Heads[0])
	gitOutput(t, repo.Dir, "checkout", "--quiet", "--detach", "HEAD")
	setCIRefEnv(t, githubBranchEnv("feature/dispatched"))

	mdl := &model.Model{Lifecycle: map[string]model.Lifecycle{
		"story": {Transitions: []model.Transition{{Verb: "close", Obligations: []model.Obligation{{Scheme: "attestation", Kind: "countersign", Count: 1}}}}},
	}}
	manifest := &store.Manifest{Countersign: &store.CountersignConfig{
		TrustSource: "forge-live", FreshnessPolicyID: "forge-current",
		MaximumObservationAgeSeconds: 300, MaximumApprovalAgeSeconds: 3600,
	}}
	resolver := &recordingLifecycleCountersignResolver{next: lifecyclecountersign.Resolver{Forge: forgefake.New()}}
	result, err := resolveLifecycleCountersign(context.Background(), resolver, repo.Dir, manifest, mdl, "story", "main", repo.Head)
	if err != nil {
		t.Fatalf("resolveLifecycleCountersign: %v", err)
	}
	if resolver.got.SourceBranch != "" {
		t.Fatalf("SourceBranch = %q, want empty", resolver.got.SourceBranch)
	}
	if result.Verdict != countersign.VerdictUnproven || len(result.Witnesses) != 1 || !strings.HasPrefix(result.Witnesses[0], "lifecycle-countersign:source-branch:unproven:") {
		t.Fatalf("result = %+v, want the blocking unproven source-branch witness", result)
	}
}
