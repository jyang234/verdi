package specimport

import (
	"context"
	"errors"
	"os"
	"testing"
)

// evidencedRequest returns minimalRequest() with evidence supplied for both
// automatically-extracted acceptance criteria, so the candidate is
// unconditionally ready.
func evidencedRequest() Request {
	req := minimalRequest()
	req.Mappings = []Mapping{
		{Target: "ac-1", Evidence: []string{"static", "attestation"}},
		{Target: "ac-2", Evidence: []string{"static", "attestation"}},
	}
	return req
}

// TestPreview_Deterministic_NoMutation proves Preview is read-only
// (spec-import-contract.md: "Preview ... is read-only") and produces the
// exact deterministic bindings the contract names: base_commit is current
// HEAD, model/engine digests come from their respective sources, and two
// calls over the same bytes/options/project context reproduce the same
// digest.
func TestPreview_Deterministic_NoMutation(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := evidencedRequest()

	before := snapshotRepository(t, repo.Dir)
	first, err := svc.Preview(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	assertRepositoryEqual(t, before, snapshotRepository(t, repo.Dir))

	second, err := svc.Preview(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatalf("Preview (second call): %v", err)
	}
	assertRepositoryEqual(t, before, snapshotRepository(t, repo.Dir))

	if first.Digest == "" || first.Digest != second.Digest {
		t.Fatalf("Digest not deterministic: %q vs %q", first.Digest, second.Digest)
	}
	recomputed, err := computePreviewDigest(first)
	if err != nil {
		t.Fatal(err)
	}
	if recomputed != first.Digest {
		t.Fatalf("Digest = %q, recomputed = %q", first.Digest, recomputed)
	}
	if first.Schema != PreviewResultSchema {
		t.Fatalf("Schema = %q, want %q", first.Schema, PreviewResultSchema)
	}
	if first.BaseCommit != repo.Head {
		t.Fatalf("BaseCommit = %q, want current HEAD %q", first.BaseCommit, repo.Head)
	}
	if first.EngineDigest != "fixed-engine-digest" {
		t.Fatalf("EngineDigest = %q, want the injected fake's value", first.EngineDigest)
	}
	if first.ModelDigest == "" || first.ConfigDigest == "" || first.RequestDigest == "" {
		t.Fatalf("a digest field is empty: %+v", first)
	}
	if first.SpecRef != "spec/sample-feature" {
		t.Fatalf("SpecRef = %q", first.SpecRef)
	}
	if !first.Ready {
		t.Fatalf("Ready = false, findings: %+v", first.Findings)
	}
	if len(first.Candidate) == 0 {
		t.Fatal("Candidate is empty despite Ready being true")
	}
}

// TestPreview_InvalidRequest_RefusesBeforeGitRead proves shape validation
// happens before any request-derived path or Git read is attempted
// (spec-import-contract.md: "Preview and Apply funnel through Normalize
// before any request-derived path or write is constructed").
func TestPreview_InvalidRequest_RefusesBeforeGitRead(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := evidencedRequest()
	req.Schema = "wrong-schema"

	before := snapshotRepository(t, repo.Dir)
	_, err := svc.Preview(context.Background(), repo.Dir, req)
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Preview(invalid schema) = %v, want ErrInvalidRequest", err)
	}
	assertRepositoryEqual(t, before, snapshotRepository(t, repo.Dir))
}

// TestPreview_DirtyTrackedFile_Refused proves an uncommitted tracked edit
// refuses preview with dirty-context rather than silently reading it
// (spec-import-contract.md: "Prepare requires a clean tracked checkout/
// index ... This first version may refuse unrelated tracked edits; it
// never resets them").
func TestPreview_DirtyTrackedFile_Refused(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := evidencedRequest()

	if err := os.WriteFile(joinPath(repo.Dir, ".verdi", "verdi.yaml"), []byte("schema: verdi.layout/v1\nextra: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshotRepository(t, repo.Dir)

	_, err := svc.Preview(context.Background(), repo.Dir, req)
	if !errors.Is(err, ErrDirtyContext) {
		t.Fatalf("Preview(dirty tracked file) = %v, want ErrDirtyContext", err)
	}
	// The refusal must never reset the caller's edit.
	assertRepositoryEqual(t, before, snapshotRepository(t, repo.Dir))
}

// TestPreview_UntrackedFile_Refused proves an untracked file outside the
// ignored .verdi/data zone also refuses preview.
func TestPreview_UntrackedFile_Refused(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := evidencedRequest()

	if err := os.WriteFile(joinPath(repo.Dir, "untracked.txt"), []byte("stray"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := svc.Preview(context.Background(), repo.Dir, req)
	if !errors.Is(err, ErrDirtyContext) {
		t.Fatalf("Preview(untracked file) = %v, want ErrDirtyContext", err)
	}
}

// TestPreview_IgnoredDataZone_StillClean proves the ignored .verdi/data
// zone does not trip the cleanliness gate (spec-import-contract.md:
// "ignored .verdi/data is allowed").
func TestPreview_IgnoredDataZone_StillClean(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := evidencedRequest()

	if err := os.MkdirAll(joinPath(repo.Dir, ".verdi", "data", "cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(joinPath(repo.Dir, ".verdi", "data", "cache", "whatever.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Preview(context.Background(), repo.Dir, req); err != nil {
		t.Fatalf("Preview with only ignored .verdi/data present: %v", err)
	}
}

// TestPreview_BlockingFinding_ReadyFalse proves a request with a real
// blocking finding (missing evidence) reports ready:false alongside its
// explicit findings — not an error (spec-import-contract.md: "A completed
// preview uses 200 with ready:false and its explicit findings").
func TestPreview_BlockingFinding_ReadyFalse(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := minimalRequest() // no evidence supplied for either AC.

	result, err := svc.Preview(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if result.Ready {
		t.Fatalf("Ready = true, want false: %+v", result.Findings)
	}
	found := false
	for _, f := range result.Findings {
		if f.Code == FindingMissingEvidence && f.Blocking {
			found = true
		}
	}
	if !found {
		t.Fatalf("findings %+v missing a blocking missing-evidence finding", result.Findings)
	}
}
