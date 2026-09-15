package designapp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/store"
)

// TestImportSidecarExcludedFromDesignContext pins the spec-import
// provenance area's exclusion from "normal design/build context"
// (spec-import-contract.md: ".verdi/imports is an admitted top-level
// provenance area, ignored by artifact index classification ... and normal
// design/build context"). A committed-shaped record.json/source sidecar
// under .verdi/imports/ carries no artifact `id:` at all, so the shared
// internal/index seam get_design_context's pinned-context resolution uses
// must never surface it as an indexed entry, and GetDesignContext itself
// must resolve exactly as it would without the sidecar present.
func TestImportSidecarExcludedFromDesignContext(t *testing.T) {
	root := newTestStore(t, "draft-write")
	head := gitHead(t, root)
	writeImportSidecar(t, root, "sample", "deadbeef")
	writePinnedContextSpec(t, root, "adr/0001-context@"+head)

	ix, err := index.Build(root)
	if err != nil {
		t.Fatalf("index.Build: %v", err)
	}
	for _, e := range ix.All() {
		if e.Kind == "" || e.Ref == "" {
			t.Fatalf("index.Build produced a degenerate entry %+v; an imports/ sidecar must never be indexed at all", e)
		}
	}

	svc := NewService()
	svc.Align = fakeAlignFindings{err: ErrAlignUnavailable}
	result, getErr := svc.GetDesignContext(context.Background(), root, GetDesignContextRequest{Spec: "spec/sample"})
	if getErr != nil {
		t.Fatalf("GetDesignContext: %v", getErr)
	}
	if len(result.PinnedContext) != 1 || result.PinnedContext[0].Ref != "adr/0001-context" {
		t.Fatalf("PinnedContext = %+v, want the ordinary ADR resolved unaffected by the imports/ sidecar", result.PinnedContext)
	}
}

// writeImportSidecar writes a record.json plus one retained-source sidecar
// directly to disk at the exact layout Apply publishes (store.ImportDir and
// friends), mirroring writeChildStory's own "no Git add/commit needed"
// convention: index.Build/GetDesignContext both read the working tree.
func writeImportSidecar(t *testing.T, root, slug, previewDigest string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(store.ImportRecordPath(root, slug, previewDigest)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.ImportRecordPath(root, slug, previewDigest), []byte(`{"schema":"verdi.spec-import-record/v1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(store.ImportSourceDir(root, slug, previewDigest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.ImportSourcePath(root, slug, previewDigest, "source"), []byte("retained-only source text, not frontmatter"), 0o644); err != nil {
		t.Fatal(err)
	}
}
