package specimport

import (
	"context"
	"testing"
)

// TestApply_DeferStatements_SetsFlagAndDisclosureIncludingRetry proves a
// successful apply with deferral always sets statements_deferred true and
// emits a nonblocking statements-deferred disclosure, including on an
// already-created retry (spec-import-contract.md: "Successful apply with
// deferral always sets statements_deferred true and emits a nonblocking
// statements-deferred disclosure in its own result, including already-
// created retries").
func TestApply_DeferStatements_SetsFlagAndDisclosureIncludingRetry(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := evidencedRequest()
	req.DeferStatements = true
	actor := testAgent(t)

	preview, err := svc.Preview(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Ready {
		t.Fatalf("preview not ready: %+v", preview.Findings)
	}

	created, err := svc.Apply(context.Background(), repo.Dir, req, preview.Digest, actor)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	requireStatementsDeferredDisclosure(t, created)

	again, err := svc.Apply(context.Background(), repo.Dir, req, preview.Digest, actor)
	if err != nil {
		t.Fatalf("Apply (retry): %v", err)
	}
	if again.Status != StatusAlreadyCreated || again.Commit != created.Commit {
		t.Fatalf("retry = %+v, want already-created at %q", again, created.Commit)
	}
	requireStatementsDeferredDisclosure(t, again)
}

func requireStatementsDeferredDisclosure(t *testing.T, result Result) {
	t.Helper()
	if !result.StatementsDeferred {
		t.Fatalf("StatementsDeferred = false, result: %+v", result)
	}
	found := 0
	for _, f := range result.Disclosures {
		if f.Code == FindingStatementsDeferred {
			found++
			if f.Blocking {
				t.Fatalf("statements-deferred disclosure is blocking: %+v", f)
			}
		}
	}
	if found != 2 { // problem and outcome.
		t.Fatalf("Disclosures = %+v, want exactly 2 statements-deferred entries (problem, outcome)", result.Disclosures)
	}
}
