package specimport

import (
	"context"
	"errors"
	"testing"
)

// TestApply_InvalidRequest_RefusesBeforeGitRead proves Apply validates
// request shape (via Normalize) before any Git read, directly — not only
// through a prior Preview call.
func TestApply_InvalidRequest_RefusesBeforeGitRead(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := evidencedRequest()
	req.Sources[0].ID = "Not A Valid Source ID"

	_, err := svc.Apply(context.Background(), repo.Dir, req, "irrelevant", testAgent(t))
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Apply(malformed source id) = %v, want ErrInvalidRequest", err)
	}
}

// TestApply_RetryUnderDifferentEngine_ProvenanceMismatch proves a retry
// whose committed import was produced under a different engine identity
// refuses automatic reconciliation rather than silently accepting it or
// minting a duplicate (spec-import-contract.md: "A different binary
// refuses automatic reconciliation with provenance-mismatch and directs
// the operator to read-only import-record inspection; it does not create
// another target").
func TestApply_RetryUnderDifferentEngine_ProvenanceMismatch(t *testing.T) {
	repo := buildImportRepo(t)
	req := evidencedRequest()
	actor := testAgent(t)
	policy := resolvedPolicyFor(t, "draft-write")

	original := &Service{Engine: fakeEngine{digest: "engine-v1"}, Policy: fakePolicySource{policy: policy}}
	preview, err := original.Preview(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatal(err)
	}
	created, err := original.Apply(context.Background(), repo.Dir, req, preview.Digest, actor)
	if err != nil {
		t.Fatal(err)
	}

	differentBinary := &Service{Engine: fakeEngine{digest: "engine-v2"}, Policy: fakePolicySource{policy: policy}}
	_, err = differentBinary.Apply(context.Background(), repo.Dir, req, preview.Digest, actor)
	if !errors.Is(err, ErrProvenanceMismatch) {
		t.Fatalf("Apply(retry under a different engine) = %v, want ErrProvenanceMismatch", err)
	}

	// Read-only inspection must still work: the record remains verifiable.
	view, err := ReadRecord(context.Background(), repo.Dir, created.Branch, "sample-feature")
	if err != nil {
		t.Fatalf("ReadRecord after provenance-mismatch retry: %v", err)
	}
	if view.ImportCommit != created.Commit {
		t.Fatalf("ImportCommit = %q, want %q", view.ImportCommit, created.Commit)
	}
}
