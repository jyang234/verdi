package specimport

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/store"
)

// TestApply_NativeByteIdenticalDuplicate_NoVL002DuplicateOneIndexEntry pins
// the exclusion's own required witness (spec-import-contract.md: "Pin the
// exclusion with an actual native source duplicate-ID fixture: one
// published candidate plus its identical retained native snapshot must
// produce one corpus spec entry and zero VL-002 duplicates"; plan Task 3
// line 127). It drives a native-format request end to end through Preview
// and Apply — the candidate and its retained source sidecar are the exact
// same bytes under native mode — then materializes the published
// design/<slug> tree as a detached worktree (gitx.WorktreeAddDetached) and
// proves, over that real on-disk tree:
//
//   - the committed candidate spec.md is byte-identical to the retained
//     sources/<id>.md sidecar (and to the native primary itself);
//   - lint.BuildSnapshot walks exactly one Doc, carrying the spec's id —
//     the retained duplicate sidecar under .verdi/imports/ never enters the
//     document walk;
//   - the public lint engine (which runs VL-002 among all rules) reports
//     zero VL-002 findings over that snapshot — no duplicate-ref finding
//     from the byte-identical retained copy;
//   - index.Build resolves exactly one entry for the spec's ref (Build
//     itself would error on a genuine duplicate ref, so a nil error here is
//     part of the witness, not incidental);
//   - the published record's candidate_digest equals the sha256 of the
//     retained sidecar bytes actually committed.
func TestApply_NativeByteIdenticalDuplicate_NoVL002DuplicateOneIndexEntry(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := newSpecNativeRequest()
	ctx := context.Background()

	preview, err := svc.Preview(ctx, repo.Dir, req)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if !preview.Ready {
		t.Fatalf("preview not ready: %+v", preview.Findings)
	}

	result, err := svc.Apply(ctx, repo.Dir, req, preview.Digest, testAgent(t))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	worktreeDir := filepath.Join(t.TempDir(), "published-worktree")
	if err := gitx.WorktreeAddDetached(ctx, repo.Dir, worktreeDir, result.Commit); err != nil {
		t.Fatalf("WorktreeAddDetached(%q): %v", result.Commit, err)
	}

	candidateBytes, err := os.ReadFile(store.ActiveSpecPath(worktreeDir, "native-widget"))
	if err != nil {
		t.Fatalf("reading published candidate: %v", err)
	}
	sourceBytes, err := os.ReadFile(store.ImportSourcePath(worktreeDir, "native-widget", preview.Digest, "source"))
	if err != nil {
		t.Fatalf("reading retained source sidecar: %v", err)
	}
	if !bytes.Equal(candidateBytes, sourceBytes) {
		t.Fatalf("candidate spec.md and retained sources/source.md are not byte-identical:\ncandidate: %q\nsource:    %q", candidateBytes, sourceBytes)
	}
	if !bytes.Equal(candidateBytes, []byte(newSpecNativeFixture)) {
		t.Fatalf("published candidate is not byte-identical to the native primary:\n%q", candidateBytes)
	}

	snap, err := lint.BuildSnapshot(worktreeDir, lint.Options{})
	if err != nil {
		t.Fatalf("lint.BuildSnapshot: %v", err)
	}
	if len(snap.Docs) != 1 {
		t.Fatalf("Docs = %d, want exactly one — the retained native sidecar must never enter the document walk: %+v", len(snap.Docs), snap.Docs)
	}
	wantRelPath := store.ActiveSpecRelPath("native-widget")
	if snap.Docs[0].RelPath != wantRelPath || snap.Docs[0].Base.ID != "spec/native-widget" {
		t.Fatalf("the one walked Doc = {RelPath:%q Base.ID:%q}, want {%q spec/native-widget}", snap.Docs[0].RelPath, snap.Docs[0].Base.ID, wantRelPath)
	}

	findings, err := lint.NewEngine().Run(ctx, worktreeDir, lint.Context{}, lint.Options{})
	if err != nil {
		t.Fatalf("lint.NewEngine().Run: %v", err)
	}
	for _, f := range findings {
		if f.Rule == "VL-002" {
			t.Fatalf("unexpected VL-002 finding over a byte-identical native retained duplicate: %+v (all findings: %+v)", f, findings)
		}
	}

	ix, err := index.Build(worktreeDir)
	if err != nil {
		t.Fatalf("index.Build: %v", err)
	}
	if ix.Len() != 1 {
		t.Fatalf("index.Len() = %d, want exactly 1 — the retained sidecar must never become a second indexed entry: %+v", ix.Len(), ix.All())
	}
	if _, ok := ix.Get("spec/native-widget"); !ok {
		t.Fatalf("index has no entry for spec/native-widget: %+v", ix.All())
	}

	recordBytes, err := os.ReadFile(store.ImportRecordPath(worktreeDir, "native-widget", preview.Digest))
	if err != nil {
		t.Fatalf("reading published import record: %v", err)
	}
	record, err := DecodeRecord(recordBytes)
	if err != nil {
		t.Fatalf("DecodeRecord: %v", err)
	}
	if record.CandidateDigest != sha256Hex(sourceBytes) {
		t.Fatalf("record.CandidateDigest = %q, want sha256 of the retained sidecar bytes %q", record.CandidateDigest, sha256Hex(sourceBytes))
	}
}
