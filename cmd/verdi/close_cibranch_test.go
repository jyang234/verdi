package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/provider/fake"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// TestCloseConflictGateExpectedBranchOnDetachedCheckout pins SI-257 for
// close's conflict gate: on a detached checkout (the CI close run's
// shape), the gate's computed expected branch is the branch being closed —
// the validated CI ref — and without one the branch stays empty, so the
// request's own validation refuses operationally (exit 2) before any
// provider call, exactly as a detached close always has.
func TestCloseConflictGateExpectedBranchOnDetachedCheckout(t *testing.T) {
	const closeBranch = "close/close-fixture"
	tests := []struct {
		name       string
		tracking   bool
		env        map[string]string
		wantCode   int
		wantBranch string // "" means the provider must never be called
	}{
		{name: "validated GitHub ref", tracking: true, env: githubBranchEnv(closeBranch), wantCode: 1, wantBranch: closeBranch},
		{
			name: "validated GitLab ref", tracking: true,
			env:      map[string]string{"GITLAB_CI": "true", "CI_COMMIT_REF_NAME": closeBranch, "CI_COMMIT_BRANCH": closeBranch},
			wantCode: 1, wantBranch: closeBranch,
		},
		{name: "outside CI", tracking: true, env: nil, wantCode: 2},
		{name: "remote-tracking ref missing", tracking: false, env: githubBranchEnv(closeBranch), wantCode: 2},
		{
			name: "GITHUB_HEAD_REF alone", tracking: true,
			env:      map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REF_TYPE": "branch", "GITHUB_HEAD_REF": closeBranch},
			wantCode: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := buildCloseFixtureRepo(t)
			installConflictPolicyStore(t, repo.Dir)
			requestPath := contextLifecycleRequestFile(t, repo.Dir, "close-context.json", "spec/close-fixture", contextcompile.PhaseReview, nil)
			if tt.tracking {
				gitOutput(t, repo.Dir, "update-ref", "refs/remotes/origin/"+closeBranch, repo.Head)
			}
			gitOutput(t, repo.Dir, "checkout", "--quiet", "--detach", "HEAD")
			setCIRefEnv(t, tt.env)

			calls := 0
			provider := contextConflictProviderFunc(func(_ context.Context, request policyconflict.Request) (policyconflict.Result, error) {
				calls++
				accepted := request.Target.AcceptedContext
				if request.Target.Kind != policyconflict.TargetAcceptedContext || accepted == nil || accepted.Phase != contextcompile.PhaseReview {
					t.Fatalf("close target = %+v, want accepted review context", request.Target)
				}
				want := contextcompile.Expected{Branch: tt.wantBranch, Head: repo.Head}
				if accepted.Expected == nil || *accepted.Expected != want {
					t.Fatalf("close expected = %+v, want %+v", accepted.Expected, want)
				}
				return lifecycleConflictResult(policyconflict.VerdictPass), nil
			})
			deps := closeDeps{
				Registry:            fake.New(),
				State:               fakeScaffoldResolver{result: specstate.Result{State: specstate.Proposed}},
				ConflictRequestPath: requestPath,
				ConflictProvider:    provider,
			}
			var stdout, stderr bytes.Buffer
			got := runClose(context.Background(), repo.Dir, "spec/close-fixture", &store.Manifest{}, deps, &stdout, &stderr)
			if got != tt.wantCode {
				t.Fatalf("runClose = %d, want %d; stdout=%s stderr=%s", got, tt.wantCode, stdout.String(), stderr.String())
			}
			if tt.wantBranch == "" {
				if calls != 0 {
					t.Fatalf("provider calls = %d, want 0: an unknown branch must refuse before evaluation", calls)
				}
				if !strings.Contains(stderr.String(), "expected.branch") {
					t.Fatalf("stderr = %q, want the empty expected-branch refusal", stderr.String())
				}
				return
			}
			if calls != 1 {
				t.Fatalf("provider calls = %d, want 1", calls)
			}
			// A passing verdict proceeds to close's existing precondition,
			// which refuses this still-proposed fixture (exit 1).
			if !strings.Contains(stdout.String(), "constitutional conflict: state: "+string(policyconflict.VerdictPass)) {
				t.Fatalf("stdout = %q, want the conflict summary", stdout.String())
			}
		})
	}
}
