package contextcompile

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/store"
)

// TestClassify_ImportSidecarsExcludedBeforeAnyByteRead pins SI-200's
// compiler clause (spec-import-contract.md, "Preview, identity and atomic
// publication"): "The context compiler's separate HEAD-tree repository-file
// channel must also exclude paths beneath `.verdi/imports/` before reading
// their bytes ... add `spec-import-sidecar` to its closed exclusion-reason
// vocabulary and record these candidates in the excluded partition.
// Preserve worktree-overlay precedence, the existing ASD
// `design-provenance-sidecar` reason, and ordinary neighboring paths."
//
// The retained external source bytes are the concrete harm: they are the
// content the contract classifies as non-authoritative sidecar, and before
// this exclusion they were read through the Git port and shipped as an
// ordinary repository-file data item.
func TestClassify_ImportSidecarsExcludedBeforeAnyByteRead(t *testing.T) {
	t.Parallel()

	const (
		head   = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		slug   = "sample-feature"
		digest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		// The ASD control row, whose own reason must survive unchanged.
		provenance = ".verdi/specs/active/" + slug + "/design-provenance.jsonl"
		// Ordinary neighbours that must stay ordinary: a sibling of the
		// imports directory, a path whose name merely starts with the same
		// letters, the imported spec itself, and an unrelated repo dir.
		neighbourFile   = ".verdi/imports.md"
		neighbourPrefix = ".verdi/importsomething/notes.md"
		unrelated       = "docs/imports/readme.md"

		retained = "RETAINED-EXTERNAL-SOURCE-BYTES"
	)
	// The excluded paths come from the SAME trusted constructors Apply
	// publishes with, so this boundary cannot drift from the write set.
	var (
		importRecord = store.ImportRecordRelPath(slug, digest)
		importSource = store.ImportSourceRelPath(slug, digest, "source")
		importLink   = store.ImportSourceRelPath(slug, digest, "linked")
		// An uncommitted file inside the same subtree: overlay precedence
		// must keep reporting it as uncommitted-content, not as a sidecar.
		importOverlay = store.ImportSourceRelPath(slug, digest, "staged")
		spec          = store.ActiveSpecRelPath(slug)
	)

	blob := func(path, object string) Candidate {
		return Candidate{Source: SourceHeadTree, ID: pathID(path), Path: path, Object: object, Mode: "100644", Type: "blob"}
	}
	candidates := []Candidate{
		{Source: SourceOpaque, ID: "opaque:harness-vendor-base/codex/1"},
		blob(importRecord, "0000001"),
		blob(importSource, "0000002"),
		// A symlink-shaped sidecar: mirroring SI-90's design-provenance
		// precedence, the import reason must win over non-regular-file.
		{Source: SourceHeadTree, ID: pathID(importLink), Path: importLink, Object: "0000003", Mode: "120000", Type: "blob"},
		{Source: SourceWorktreeOverlay, ID: pathID(importOverlay), Path: importOverlay},
		{Source: SourceHeadTree, ID: pathID(provenance), Path: provenance, Object: "0000004", Mode: "100644", Type: "blob"},
		blob(neighbourFile, "0000005"),
		blob(neighbourPrefix, "0000006"),
		blob(spec, "0000007"),
		blob(unrelated, "0000008"),
	}
	git := &classifyGit{contents: map[string][]byte{
		importRecord:    []byte(`{"schema":"verdi.spec-import-record/v1"}` + "\n"),
		importSource:    []byte(retained + "\n"),
		importLink:      []byte(retained + "\n"),
		provenance:      []byte("{}\n"),
		neighbourFile:   []byte("ordinary neighbour\n"),
		neighbourPrefix: []byte("ordinary neighbour\n"),
		spec:            []byte("---\nid: spec/sample-feature\n---\n"),
		unrelated:       []byte("ordinary documentation\n"),
	}}

	got, err := Classify(context.Background(), git, "/repo", head, ClassificationInput{
		Candidates:   candidates,
		Materials:    []CandidateMaterial{},
		Phase:        PhaseBuild,
		RequestScope: explicitUniversalScope(),
		TargetRef:    "spec/sample-feature",
		Adapter:      AdapterRef{ID: "codex", Version: "1"},
	})
	if err != nil {
		t.Fatalf("Classify() error = %v", err)
	}
	assertClassifiedPartition(t, candidates, got)
	assertSortedClassificationRows(t, got)

	// The committed sidecars never reach the Git port at all: the only
	// bytes read are the ordinary neighbours' and the imported spec's.
	wantShows := []string{neighbourFile, neighbourPrefix, spec, unrelated}
	if !reflect.DeepEqual(git.shows, wantShows) {
		t.Errorf("GitReader.Show calls = %#v, want exactly %#v (no import sidecar byte may be read)", git.shows, wantShows)
	}

	wantReason := map[string]ExclusionReason{
		importRecord:  ExclusionSpecImportSidecar,
		importSource:  ExclusionSpecImportSidecar,
		importLink:    ExclusionSpecImportSidecar,
		importOverlay: ExclusionUncommittedContent,
		provenance:    ExclusionDesignProvenanceSidecar,
	}
	gotReason := map[string]ExclusionReason{}
	for _, row := range got.Excluded {
		if row.Path != nil {
			gotReason[*row.Path] = row.Reason
		}
	}
	if !reflect.DeepEqual(gotReason, wantReason) {
		t.Errorf("excluded partition = %#v, want %#v", gotReason, wantReason)
	}

	wantIncluded := map[string]IncludedKind{
		neighbourFile:   IncludedRepositoryFile,
		neighbourPrefix: IncludedRepositoryFile,
		spec:            IncludedRepositoryFile,
		unrelated:       IncludedRepositoryFile,
	}
	gotIncluded := map[string]IncludedKind{}
	for _, row := range got.Included {
		if row.Path != nil {
			gotIncluded[*row.Path] = row.Kind
		}
	}
	if !reflect.DeepEqual(gotIncluded, wantIncluded) {
		t.Errorf("included partition = %#v, want %#v (ordinary neighbours stay ordinary)", gotIncluded, wantIncluded)
	}

	for i, item := range got.DataItems {
		if strings.Contains(item.Content, retained) {
			t.Errorf("data item %d (%s) carries retained import source bytes", i, item.ID)
		}
	}
}

// TestExclusionReason_SpecImportSidecarIsAClosedMember pins the enum half
// of the same clause: SI-200 supplements the context-compiler authority
// design's §5.2 closed v1 vocabulary with exactly this one member, and
// every prior member plus fail-closed decoding of an unknown value remains.
func TestExclusionReason_SpecImportSidecarIsAClosedMember(t *testing.T) {
	t.Parallel()

	closed := []ExclusionReason{
		ExclusionDesignProvenanceSidecar, ExclusionDataZoneDisposable, ExclusionUncommittedContent,
		ExclusionOutOfDeclaredScope, ExclusionPhaseInapplicable, ExclusionSupersededSpec,
		ExclusionArchivedRecord, ExclusionGeneratedProjectionOutput, ExclusionNonTextData,
		ExclusionNonRegularFile, ExclusionSpecImportSidecar,
	}
	if len(closed) != 11 {
		t.Fatalf("closed vocabulary has %d members, want the ten v1 reasons plus spec-import-sidecar", len(closed))
	}
	for _, reason := range closed {
		if err := reason.Validate(); err != nil {
			t.Errorf("ExclusionReason(%q).Validate() = %v, want nil", reason, err)
		}
	}
	if ExclusionSpecImportSidecar != "spec-import-sidecar" {
		t.Errorf("ExclusionSpecImportSidecar = %q, want the contract's %q", ExclusionSpecImportSidecar, "spec-import-sidecar")
	}
	for _, unknown := range []ExclusionReason{"", "spec-import", "spec_import_sidecar", "import-sidecar"} {
		if err := unknown.Validate(); err == nil {
			t.Errorf("ExclusionReason(%q).Validate() = nil, want a fail-closed error", unknown)
		}
	}
}
