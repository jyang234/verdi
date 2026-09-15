package specimport

import (
	"context"
	"errors"
	"os"
	"testing"
)

// applyEvidencedRequest runs Preview+Apply once over evidencedRequest() and
// returns the resulting Result, for tests that need an already-committed
// import to inspect.
func applyEvidencedRequest(t *testing.T, root string) Result {
	t.Helper()
	svc := testService(t)
	req := evidencedRequest()
	preview, err := svc.Preview(context.Background(), root, req)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	result, err := svc.Apply(context.Background(), root, req, preview.Digest, testAgent(t))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return result
}

// TestReadRecord_VerifiesAgainstActualBytes proves ReadRecord identifies
// the original import commit and independently verifies every recorded
// digest against actual committed Git bytes, disclosing nothing when the
// active spec is unchanged since import.
func TestReadRecord_VerifiesAgainstActualBytes(t *testing.T) {
	repo := buildImportRepo(t)
	result := applyEvidencedRequest(t, repo.Dir)

	view, err := ReadRecord(context.Background(), repo.Dir, result.Branch, "sample-feature")
	if err != nil {
		t.Fatalf("ReadRecord: %v", err)
	}
	if view.ImportCommit != result.Commit {
		t.Fatalf("ImportCommit = %q, want %q", view.ImportCommit, result.Commit)
	}
	if !view.CurrentSpecMatches {
		t.Fatalf("CurrentSpecMatches = false, disclosures: %+v", view.Disclosures)
	}
	if len(view.Disclosures) != 0 {
		t.Fatalf("Disclosures = %+v, want none", view.Disclosures)
	}
	if view.Record.Schema != RecordSchema || view.Record.SpecRef != "spec/sample-feature" {
		t.Fatalf("Record = %+v", view.Record)
	}
	if len(view.Record.Sources) != 1 || view.Record.Sources[0].ID != "source" {
		t.Fatalf("Record.Sources = %+v", view.Record.Sources)
	}
}

// TestReadRecord_AfterDescendantEdit_DisclosesCurrentSpecChanged proves an
// ordinary supported edit on top of the import does not erase or corrupt
// the historical import commit's own verification — it is disclosed
// separately, honestly, as a current-spec change.
func TestReadRecord_AfterDescendantEdit_DisclosesCurrentSpecChanged(t *testing.T) {
	repo := buildImportRepo(t)
	result := applyEvidencedRequest(t, repo.Dir)

	runGitFixture(t, repo.Dir, "checkout", result.Branch)
	if err := os.WriteFile(joinPath(repo.Dir, ".verdi", "specs", "active", "sample-feature", "spec.md"), []byte("edited content, not the original candidate"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitFixture(t, repo.Dir, "add", "-A")
	runGitFixture(t, repo.Dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "ordinary supported edit")
	runGitFixture(t, repo.Dir, "checkout", "main")

	view, err := ReadRecord(context.Background(), repo.Dir, result.Branch, "sample-feature")
	if err != nil {
		t.Fatalf("ReadRecord after descendant edit: %v", err)
	}
	if view.ImportCommit != result.Commit {
		t.Fatalf("ImportCommit = %q, want the ORIGINAL import commit %q (never the edited descendant)", view.ImportCommit, result.Commit)
	}
	if view.CurrentSpecMatches {
		t.Fatal("CurrentSpecMatches = true after an edit that changed the active spec's bytes")
	}
	found := false
	for _, f := range view.Disclosures {
		if f.Code == FindingCurrentSpecChanged {
			found = true
		}
	}
	if !found {
		t.Fatalf("Disclosures = %+v, want a current-spec-changed disclosure", view.Disclosures)
	}
}

// TestReadRecord_MovedBranch_StillVerifies proves a renamed (but still
// valid descendant) branch does not erase the historical import revision.
func TestReadRecord_MovedBranch_StillVerifies(t *testing.T) {
	repo := buildImportRepo(t)
	result := applyEvidencedRequest(t, repo.Dir)

	runGitFixture(t, repo.Dir, "branch", "-m", result.Branch, "design/renamed-feature")

	view, err := ReadRecord(context.Background(), repo.Dir, "design/renamed-feature", "sample-feature")
	if err != nil {
		t.Fatalf("ReadRecord(moved branch): %v", err)
	}
	if view.ImportCommit != result.Commit || !view.CurrentSpecMatches {
		t.Fatalf("view = %+v, want the identical verified import unaffected by the rename", view)
	}
}

// TestReadRecord_TamperedSource_ProvenanceMismatch proves ReadRecord never
// verifies solely from the record's own self-reported hashes: a retained
// source sidecar edited out-of-band (never through Apply) must fail
// verification.
func TestReadRecord_TamperedSource_ProvenanceMismatch(t *testing.T) {
	repo := buildImportRepo(t)
	result := applyEvidencedRequest(t, repo.Dir)

	view, err := ReadRecord(context.Background(), repo.Dir, result.Branch, "sample-feature")
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := joinPath(repo.Dir, ".verdi", "imports", "sample-feature", view.Record.PreviewDigest, "sources", "source.md")

	runGitFixture(t, repo.Dir, "checkout", result.Branch)
	if err := os.WriteFile(sourcePath, []byte("tampered retained source bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitFixture(t, repo.Dir, "add", "-A")
	runGitFixture(t, repo.Dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "tamper with a retained source")
	runGitFixture(t, repo.Dir, "checkout", "main")

	_, err = ReadRecord(context.Background(), repo.Dir, result.Branch, "sample-feature")
	if !errors.Is(err, ErrProvenanceMismatch) {
		t.Fatalf("ReadRecord(tampered source) = %v, want ErrProvenanceMismatch", err)
	}
}

// TestReadRecord_NoImport_Refused proves ReadRecord refuses rather than
// fabricating a record for a slug that was never imported.
func TestReadRecord_NoImport_Refused(t *testing.T) {
	repo := buildImportRepo(t)
	_, err := ReadRecord(context.Background(), repo.Dir, "main", "never-imported")
	if !errors.Is(err, ErrImportRecordMissing) {
		t.Fatalf("ReadRecord(no import) = %v, want ErrImportRecordMissing", err)
	}
}
