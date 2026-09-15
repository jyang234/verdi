package lint

import (
	"os"
	"path/filepath"
	"testing"
)

// TestImportsTopLevelAdmitted pins the spec-import provenance area's
// exclusion (spec-import-contract.md: ".verdi/imports is an admitted
// top-level provenance area, ignored by artifact index classification,
// lint.BuildSnapshot's document walk and Snapshot.ByRef"). A committed
// record.json and source sidecar under .verdi/imports/<slug>/<digest>/ must
// produce no VL-007 unrecognized-top-level-entry finding and must never be
// walked into Snapshot.Docs — a plain .md sidecar has no frontmatter at all,
// so if it were walked it would report a VL-001 decode failure instead.
func TestImportsTopLevelAdmitted(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".verdi", "verdi.yaml"), "schema: verdi.layout/v1\n")
	mustWriteFile(t, filepath.Join(root, ".verdi", "imports", "sample-feature", "abc123", "record.json"), `{"schema":"verdi.spec-import-record/v1"}`)
	mustWriteFile(t, filepath.Join(root, ".verdi", "imports", "sample-feature", "abc123", "sources", "source.md"), "not frontmatter, retained-only source text")

	snap, err := BuildSnapshot(root, Options{})
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}

	foundImports := false
	for _, name := range snap.TopLevelEntries {
		if name == "imports" {
			foundImports = true
		}
	}
	if !foundImports {
		t.Fatalf("TopLevelEntries = %v, want \"imports\" present", snap.TopLevelEntries)
	}
	if len(snap.Docs) != 0 {
		t.Fatalf("Docs = %+v, want zero — imports/ sidecars must never enter the document walk", snap.Docs)
	}

	findings := (vl007{}).Check(&RunInput{Snapshot: snap})
	if len(findings) != 0 {
		t.Fatalf("VL-007 findings = %+v, want none: \"imports\" must be an admitted top-level entry", findings)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
