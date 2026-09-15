package index

// TestBuild_ImportsSidecarExcluded pins the spec-import provenance area's
// exclusion (spec-import-contract.md: ".verdi/imports is an admitted
// top-level provenance area, ignored by artifact index classification...
// and Snapshot.ByRef"). A committed record.json (not valid frontmatter at
// all) and a retained-source sidecar under .verdi/imports/<slug>/<digest>/
// must never be walked into the index: if walkCommittedZone's shared
// artifact.ClassifyPath ever classified an imports/ path, decodeEntry would
// hard-fail on the non-frontmatter record.json and Build would error.
import "testing"

func TestBuild_ImportsSidecarExcluded(t *testing.T) {
	root := buildSyntheticStore(t)
	baseline, err := Build(root)
	if err != nil {
		t.Fatalf("Build (baseline): %v", err)
	}
	baseLen := baseline.Len()

	writeIndexFile(t, root, ".verdi/imports/sample-feature/deadbeef/record.json", `{"schema":"verdi.spec-import-record/v1"}`)
	writeIndexFile(t, root, ".verdi/imports/sample-feature/deadbeef/sources/source.md", "retained-only source text, not frontmatter")

	ix, err := Build(root)
	if err != nil {
		t.Fatalf("Build (with imports/ sidecar present): %v", err)
	}
	if ix.Len() != baseLen {
		t.Fatalf("Len() = %d after adding an imports/ sidecar, want unchanged %d — the sidecar must never become an indexed entry", ix.Len(), baseLen)
	}
}
