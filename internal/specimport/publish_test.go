package specimport

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/gitx"
)

func testAgent(t *testing.T) draftmutation.Actor {
	t.Helper()
	actor, err := draftmutation.NewDelegatedAgent("test-harness", "session-1")
	if err != nil {
		t.Fatal(err)
	}
	return actor
}

// TestApply_Create_ExactWriteSetAndBoardPath proves Apply publishes one
// commit whose write set is EXACTLY spec.md + record.json + every retained
// source, on a fresh create-only refs/heads/design/<slug>, without ever
// moving the caller's HEAD/branch/index (spec-import-contract.md: "Use
// shared Git plumbing to build the base tree with the complete write set
// in sorted path order, create one child commit, then create-only publish
// ... No checkout/index change").
func TestApply_Create_ExactWriteSetAndBoardPath(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := evidencedRequest()

	preview, err := svc.Preview(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if !preview.Ready {
		t.Fatalf("preview not ready: %+v", preview.Findings)
	}

	beforeHead := preview.BaseCommit
	beforeStatus := runGitCapture(t, repo.Dir, "status", "--porcelain=v1", "-z", "--untracked-files=all")

	result, err := svc.Apply(context.Background(), repo.Dir, req, preview.Digest, testAgent(t))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.Status != StatusCreated {
		t.Fatalf("Status = %q, want %q", result.Status, StatusCreated)
	}
	if result.Schema != ResultSchema || result.Branch != "design/sample-feature" || result.SpecRef != "spec/sample-feature" {
		t.Fatalf("result = %+v", result)
	}
	if result.PreviewDigest != preview.Digest {
		t.Fatalf("PreviewDigest = %q, want %q", result.PreviewDigest, preview.Digest)
	}
	if result.BoardPath != "/b/design%2Fsample-feature/board/spec/sample-feature" {
		t.Fatalf("BoardPath = %q", result.BoardPath)
	}
	if result.StatementsDeferred {
		t.Fatal("StatementsDeferred = true for a non-deferred request")
	}

	// HEAD/branch/index must be exactly unchanged.
	afterHead := runGitCapture(t, repo.Dir, "rev-parse", "HEAD")
	if trimmed := afterHead[:len(afterHead)-1]; trimmed != beforeHead {
		t.Fatalf("caller HEAD moved: %q -> %q", beforeHead, trimmed)
	}
	afterStatus := runGitCapture(t, repo.Dir, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if beforeStatus != afterStatus {
		t.Fatalf("caller worktree/index changed: %q -> %q", beforeStatus, afterStatus)
	}

	// The new branch must exist, its tip must be result.Commit, and its
	// parent must be exactly the base commit.
	tip := runGitCapture(t, repo.Dir, "rev-parse", "refs/heads/design/sample-feature")
	if trimmed := tip[:len(tip)-1]; trimmed != result.Commit {
		t.Fatalf("branch tip = %q, want result.Commit %q", trimmed, result.Commit)
	}
	parent := runGitCapture(t, repo.Dir, "rev-parse", result.Commit+"^")
	if trimmed := parent[:len(parent)-1]; trimmed != beforeHead {
		t.Fatalf("commit parent = %q, want base commit %q", trimmed, beforeHead)
	}

	diff, err := gitx.DiffNameStatus(context.Background(), repo.Dir, beforeHead, result.Commit)
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := map[string]bool{
		".verdi/specs/active/sample-feature/spec.md":                             true,
		".verdi/imports/sample-feature/" + preview.Digest + "/record.json":       true,
		".verdi/imports/sample-feature/" + preview.Digest + "/sources/source.md": true,
	}
	if len(diff) != len(wantPaths) {
		t.Fatalf("write set has %d entries, want %d: %+v", len(diff), len(wantPaths), diff)
	}
	for _, e := range diff {
		if e.Status != "A" {
			t.Fatalf("entry %+v is not a pure add", e)
		}
		if !wantPaths[e.Path] {
			t.Fatalf("unexpected write-set path %q", e.Path)
		}
	}
}

// TestApply_Retry_SameDigest_ReturnsExistingCommit proves an identical
// retry — even after the caller's checkout has since become dirty —
// reconciles to the exact same commit rather than refusing or minting a
// second identity (spec-import-contract.md's core positive test shape).
func TestApply_Retry_SameDigest_ReturnsExistingCommit(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := evidencedRequest()
	actor := testAgent(t)

	preview, err := svc.Preview(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatal(err)
	}
	created, err := svc.Apply(context.Background(), repo.Dir, req, preview.Digest, actor)
	if err != nil {
		t.Fatal(err)
	}

	// Dirty the checkout — the retry must still reconcile.
	if err := os.WriteFile(joinPath(repo.Dir, "untracked.txt"), []byte("stray"), 0o644); err != nil {
		t.Fatal(err)
	}

	again, err := svc.Apply(context.Background(), repo.Dir, req, preview.Digest, actor)
	if err != nil {
		t.Fatalf("Apply (retry over dirty checkout): %v", err)
	}
	if again.Status != StatusAlreadyCreated || again.Commit != created.Commit || again.Branch != created.Branch {
		t.Fatalf("retry = %+v, want already-created at commit %q", again, created.Commit)
	}

	// Only ever one branch: refs/heads/design/sample-feature.
	refs := runGitCapture(t, repo.Dir, "for-each-ref", "--format=%(refname)", "refs/heads/design/")
	if refs != "refs/heads/design/sample-feature\n" {
		t.Fatalf("design/ refs = %q, want exactly one branch", refs)
	}
}

// TestApply_Retry_MismatchedActor_TargetExists proves a retry under a
// DIFFERENT actor does not reconcile — it is refused as an ordinary
// collision, never silently attributed to the new actor.
func TestApply_Retry_MismatchedActor_TargetExists(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := evidencedRequest()

	preview, err := svc.Preview(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Apply(context.Background(), repo.Dir, req, preview.Digest, testAgent(t)); err != nil {
		t.Fatal(err)
	}

	other, err := draftmutation.NewDelegatedAgent("other-harness", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Apply(context.Background(), repo.Dir, req, preview.Digest, other)
	if !errors.Is(err, ErrTargetExists) {
		t.Fatalf("Apply(mismatched actor retry) = %v, want ErrTargetExists", err)
	}
}

// TestApply_StaleDigest_Refused proves Apply rejects a changed digest
// before Git publication and creates no branch.
func TestApply_StaleDigest_Refused(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := evidencedRequest()

	_, err := svc.Apply(context.Background(), repo.Dir, req, "0000000000000000000000000000000000000000000000000000000000000000", testAgent(t))
	if !errors.Is(err, ErrStalePreview) {
		t.Fatalf("Apply(stale digest) = %v, want ErrStalePreview", err)
	}
	if exists, _ := gitx.HasLocalBranch(context.Background(), repo.Dir, "design/sample-feature"); exists {
		t.Fatal("a branch was created despite the stale-digest refusal")
	}
}

// TestApply_BlockingFinding_Unresolved proves Apply refuses to publish a
// candidate whose preview carries a blocking finding, even when the caller
// supplies the CORRECT digest for that (not-ready) preview.
func TestApply_BlockingFinding_Unresolved(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := minimalRequest() // no evidence: missing-evidence is blocking.

	preview, err := svc.Preview(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Ready {
		t.Fatal("preview unexpectedly ready")
	}

	_, err = svc.Apply(context.Background(), repo.Dir, req, preview.Digest, testAgent(t))
	if !errors.Is(err, ErrUnresolved) {
		t.Fatalf("Apply(blocking finding) = %v, want ErrUnresolved", err)
	}
	if exists, _ := gitx.HasLocalBranch(context.Background(), repo.Dir, "design/sample-feature"); exists {
		t.Fatal("a branch was created despite the unresolved refusal")
	}
}

// TestApply_ExistingSpecIdentity_TargetExists proves Apply refuses to
// create a duplicate spec identity even when no design/<slug> branch has
// ever existed (spec-import-contract.md: "Reject any active/archive spec
// identity or target branch collision").
func TestApply_ExistingSpecIdentity_TargetExists(t *testing.T) {
	repo := buildImportRepo(t)
	runGitFixture(t, repo.Dir, "checkout", "main")
	if err := os.MkdirAll(joinPath(repo.Dir, ".verdi", "specs", "active", "sample-feature"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(joinPath(repo.Dir, ".verdi", "specs", "active", "sample-feature", "spec.md"), []byte("placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitFixture(t, repo.Dir, "add", "-A")
	runGitFixture(t, repo.Dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "pre-existing spec")

	svc := testService(t)
	req := evidencedRequest()
	preview, err := svc.Preview(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Apply(context.Background(), repo.Dir, req, preview.Digest, testAgent(t))
	if !errors.Is(err, ErrTargetExists) {
		t.Fatalf("Apply(existing spec identity) = %v, want ErrTargetExists", err)
	}
}

// TestApply_DirtyContext_Refused proves a dirty checkout refuses a FRESH
// (non-retry) apply.
func TestApply_DirtyContext_Refused(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := evidencedRequest()
	preview, err := svc.Preview(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(joinPath(repo.Dir, "untracked.txt"), []byte("stray"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Apply(context.Background(), repo.Dir, req, preview.Digest, testAgent(t))
	if !errors.Is(err, ErrDirtyContext) {
		t.Fatalf("Apply(dirty, fresh) = %v, want ErrDirtyContext", err)
	}
}
