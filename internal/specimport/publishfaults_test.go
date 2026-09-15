package specimport

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
)

// TestApply_BeforeRefUpdateFault_NoVisibleBranch drives a fault before ref
// publication (spec-import-contract.md: "Failure before ref publication
// has no visible branch/import; unreachable Git objects are ordinary
// disposable plumbing").
func TestApply_BeforeRefUpdateFault_NoVisibleBranch(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	injected := errors.New("boundary fault: before ref update")
	svc.Faults.BeforeRefUpdate = func() error { return injected }
	req := evidencedRequest()

	preview, err := svc.Preview(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Apply(context.Background(), repo.Dir, req, preview.Digest, testAgent(t))
	if !errors.Is(err, injected) {
		t.Fatalf("Apply = %v, want the injected fault", err)
	}
	if exists, _ := gitx.HasLocalBranch(context.Background(), repo.Dir, "design/sample-feature"); exists {
		t.Fatal("a branch became visible despite the pre-publication fault")
	}
}

// TestApply_AfterRefUpdateFault_RetrySucceedsToSameCommit drives a fault
// after successful publication but before the response is returned,
// proving a subsequent ordinary retry reconciles to the exact same commit
// rather than erroring or minting a second identity.
func TestApply_AfterRefUpdateFault_RetrySucceedsToSameCommit(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	injected := errors.New("boundary fault: after ref update")
	svc.Faults.AfterRefUpdate = func() error { return injected }
	req := evidencedRequest()
	actor := testAgent(t)

	preview, err := svc.Preview(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Apply(context.Background(), repo.Dir, req, preview.Digest, actor)
	if !errors.Is(err, injected) {
		t.Fatalf("Apply = %v, want the injected fault", err)
	}
	tip := runGitCapture(t, repo.Dir, "rev-parse", "refs/heads/design/sample-feature")

	retrySvc := testService(t)
	retry, err := retrySvc.Apply(context.Background(), repo.Dir, req, preview.Digest, actor)
	if err != nil {
		t.Fatalf("Apply (retry after lost response): %v", err)
	}
	if retry.Status != StatusAlreadyCreated {
		t.Fatalf("Status = %q, want %q", retry.Status, StatusAlreadyCreated)
	}
	if trimmed := tip[:len(tip)-1]; retry.Commit != trimmed {
		t.Fatalf("retry.Commit = %q, want the already-published tip %q", retry.Commit, trimmed)
	}
}

// TestApply_CASLost_NoMatchingRecord_TargetExists proves a lost create-only
// CAS (the branch was absent when checked but exists by the time UpdateRef
// runs) re-enters reconciliation once, and — finding no matching import
// record at the winning tip — refuses target-exists rather than
// overwriting or resetting it.
func TestApply_CASLost_NoMatchingRecord_TargetExists(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := evidencedRequest()
	preview, err := svc.Preview(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatal(err)
	}
	svc.Faults.BeforeRefUpdate = func() error {
		// A concurrent, unrelated writer wins the race for this ref name.
		runGitFixture(t, repo.Dir, "update-ref", "refs/heads/design/sample-feature", preview.BaseCommit)
		return nil
	}

	_, err = svc.Apply(context.Background(), repo.Dir, req, preview.Digest, testAgent(t))
	if !errors.Is(err, ErrTargetExists) {
		t.Fatalf("Apply(CAS lost, unrelated winner) = %v, want ErrTargetExists", err)
	}
	tip := runGitCapture(t, repo.Dir, "rev-parse", "refs/heads/design/sample-feature")
	if trimmed := tip[:len(tip)-1]; trimmed != preview.BaseCommit {
		t.Fatalf("the unrelated winner's ref was overwritten: tip = %q, want %q", trimmed, preview.BaseCommit)
	}
}

// TestApply_ConcurrentIdenticalCalls_ExactlyOneBranch proves concurrent
// attempts over the identical request/digest/actor create at most one
// branch, and every caller observes the same commit
// (spec-import-contract.md: "Concurrent attempts create at most one
// branch").
func TestApply_ConcurrentIdenticalCalls_ExactlyOneBranch(t *testing.T) {
	repo := buildImportRepo(t)
	req := evidencedRequest()
	actor := testAgent(t)
	policy := resolvedPolicyFor(t, "draft-write")

	preview, err := (&Service{Engine: fakeEngine{}, Policy: fakePolicySource{policy: policy}}).Preview(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatal(err)
	}

	const n = 6
	results := make([]Result, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			svc := &Service{Engine: fakeEngine{}, Policy: fakePolicySource{policy: policy}}
			results[i], errs[i] = svc.Apply(context.Background(), repo.Dir, req, preview.Digest, actor)
		}(i)
	}
	wg.Wait()

	var commit string
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: Apply: %v", i, err)
		}
		if commit == "" {
			commit = results[i].Commit
		} else if results[i].Commit != commit {
			t.Fatalf("goroutine %d produced commit %q, want %q (all identical calls must agree)", i, results[i].Commit, commit)
		}
	}

	refs := runGitCapture(t, repo.Dir, "for-each-ref", "--format=%(refname)", "refs/heads/design/")
	if refs != "refs/heads/design/sample-feature\n" {
		t.Fatalf("design/ refs = %q, want exactly one branch", refs)
	}
}
