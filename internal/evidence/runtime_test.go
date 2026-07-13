package evidence

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	runtimeprobe "github.com/jyang234/verdi/internal/runtime"
)

// This file is spec/runtime-evidence ac-2's end-to-end proof: a fixture
// story declaring evidence: [runtime], combined with a REAL runtime.json
// record built via internal/runtime.Emit (not hand-crafted JSON) and loaded
// through the real LoadRecords path (ancestry-checked against a real git
// history, exactly as `verdi close`/`verdi gate`/`verdi rollup` do), proves
// the whole chain — probe emits, LoadRecords loads runtime.json, Fold
// consumes it — works together, not merely that each piece works in
// isolation.

// runtimeFixtureSpec declares one AC expecting only runtime evidence — the
// exact shape ac-2 targets: "a story declaring evidence: [runtime] folds
// from pending to evidenced once a matching source: ci runtime record is
// present."
func runtimeFixtureSpec() *artifact.SpecFrontmatter {
	return testSpec("jira:RUNTIME-FIXTURE-1", ac("ac-1", artifact.EvidenceRuntime))
}

// writeDerivedRuntimeRecord builds one runtime record via
// internal/runtime.Emit — proving real integration with that package, not
// just that hand-crafted JSON happens to satisfy the fold — and writes it to
// derivedRoot/commit/runtime.json.
func writeDerivedRuntimeRecord(t *testing.T, derivedRoot, commit string, inCI bool) {
	t.Helper()
	rec, err := runtimeprobe.Emit(runtimeprobe.ProbeInput{
		StoryRef: "jira:RUNTIME-FIXTURE-1",
		ACID:     "ac-1",
		Verdict:  artifact.VerdictPass,
		Witness:  "GET /healthz -> 200",
		Commit:   commit,
		InCI:     inCI,
	})
	if err != nil {
		t.Fatalf("runtime.Emit: %v", err)
	}

	dir := filepath.Join(derivedRoot, commit)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	data, err := json.Marshal([]artifact.Evidence{rec})
	if err != nil {
		t.Fatalf("marshaling runtime.json fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "runtime.json"), data, 0o644); err != nil {
		t.Fatalf("writing runtime.json: %v", err)
	}
}

// foldRuntimeFixture loads derivedRoot/<commit's ancestry> through the real
// LoadRecords path and folds runtimeFixtureSpec's single AC at commit,
// returning its status — the whole ac-2 chain, Preview always false (co-1:
// the closure/rollup/gate posture, never the --preview escape hatch).
func foldRuntimeFixture(t *testing.T, gitDir, derivedRoot, commit string) ACResult {
	t.Helper()
	records, err := LoadRecords(context.Background(), gitDir, derivedRoot, commit)
	if err != nil {
		t.Fatalf("LoadRecords: %v", err)
	}
	result, err := Fold(Input{
		Spec:      runtimeFixtureSpec(),
		Records:   records,
		Preview:   false,
		StoreRoot: gitDir,
		StorySlug: "runtime-fixture-1",
	})
	if err != nil {
		t.Fatalf("Fold: %v", err)
	}
	if len(result.ACs) != 1 {
		t.Fatalf("Fold produced %d AC results, want 1", len(result.ACs))
	}
	return result.ACs[0]
}

// TestRuntimeEvidence_SourceCIRecordFoldsToEvidenced proves the load-bearing
// half of ac-2: a story declaring evidence: [runtime] folds to evidenced
// once a matching source: ci runtime record is present, loaded the same way
// `verdi close` loads every other kind.
func TestRuntimeEvidence_SourceCIRecordFoldsToEvidenced(t *testing.T) {
	repo := buildRecordsRepo(t)
	derivedRoot := filepath.Join(repo.Dir, "derived", "spec--runtime-fixture")
	writeDerivedRuntimeRecord(t, derivedRoot, repo.Head, true /* inCI: stamps source: ci */)

	got := foldRuntimeFixture(t, repo.Dir, derivedRoot, repo.Head)
	if got.Status != StatusEvidenced {
		t.Fatalf("status = %q, want evidenced (a source: ci runtime record satisfies the declared runtime kind): summary=%q", got.Status, got.Summary)
	}
}

// TestRuntimeEvidence_AbsentRecordIsPending proves the other half: absent
// any runtime record at all, the AC reads pending (awaited post-merge),
// never no-signal and never evidenced — 03 §The fold's runtime-specific
// carve-out (foldAC), unchanged by this story's producer existing.
func TestRuntimeEvidence_AbsentRecordIsPending(t *testing.T) {
	repo := buildRecordsRepo(t)
	derivedRoot := filepath.Join(repo.Dir, "derived", "spec--runtime-fixture")
	// No runtime.json written at all: derivedRoot's commit dir does not
	// even exist yet (a story that has never been probed).

	got := foldRuntimeFixture(t, repo.Dir, derivedRoot, repo.Head)
	if got.Status != StatusPending {
		t.Fatalf("status = %q, want pending (no runtime record yet, always awaited post-merge)", got.Status)
	}
}

// TestRuntimeEvidence_LocalRecordIgnoredUnderPreviewFalse proves a
// source: local runtime record — a probe run outside genuine CI, or with
// --force-local — is loaded (LoadRecords returns both provenance classes)
// but never folded to evidenced with Preview false: dc-3's advisory-only
// posture for local runs, identical to static/behavioral's existing
// discipline (03 §Evidence records: "gates consume authoritative evidence
// only").
func TestRuntimeEvidence_LocalRecordIgnoredUnderPreviewFalse(t *testing.T) {
	repo := buildRecordsRepo(t)
	derivedRoot := filepath.Join(repo.Dir, "derived", "spec--runtime-fixture")
	writeDerivedRuntimeRecord(t, derivedRoot, repo.Head, false /* inCI: false -> stamps source: local */)

	got := foldRuntimeFixture(t, repo.Dir, derivedRoot, repo.Head)
	if got.Status != StatusPending {
		t.Fatalf("status = %q, want pending (a source: local runtime record must be ignored when Preview is false): summary=%q", got.Status, got.Summary)
	}
}
