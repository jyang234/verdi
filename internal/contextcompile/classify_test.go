package contextcompile

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
)

type classifyGit struct {
	contents  map[string][]byte
	forbidden map[string]bool
	shows     []string
}

func (g *classifyGit) Show(_ context.Context, _, _, path string) ([]byte, error) {
	if g.forbidden[path] {
		panic("forbidden candidate reached GitReader.Show: " + path)
	}
	g.shows = append(g.shows, path)
	content, ok := g.contents[path]
	if !ok {
		return nil, fmt.Errorf("no committed content for %s", path)
	}
	return append([]byte(nil), content...), nil
}

func (*classifyGit) LsTreeEntries(context.Context, string, string) ([]gitx.TreeEntry, error) {
	panic("Classify must not discover a second universe")
}

func (*classifyGit) WorktreeChangedPaths(context.Context, string) ([]string, error) {
	panic("Classify must not inspect the worktree")
}

func TestClassify(t *testing.T) {
	t.Parallel()

	const (
		head       = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		fixedSHA   = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
		projection = "AGENTS.md"
	)
	regular := func(path, object string) Candidate {
		return Candidate{Source: SourceHeadTree, ID: "path:" + path, Path: path, Object: object, Mode: "100644", Type: "blob"}
	}
	refCandidate := func(source Source, ref string) Candidate {
		return Candidate{Source: source, ID: "ref:" + ref, Ref: ref}
	}

	fragment := FeatureFragment{
		Feature: FragmentFeature{
			Ref: "spec/feature", Path: ".verdi/specs/active/feature/spec.md", SourceDigest: fixedSHA,
		},
		Problem: artifact.Attribute{Text: "A real problem", Anchor: "problem"},
		Outcome: artifact.Attribute{Text: "A verified outcome", Anchor: "outcome"},
		Targets: []FragmentTarget{{
			ID: "ac-one", Text: "The first criterion", Evidence: []artifact.EvidenceKind{artifact.EvidenceStatic}, Anchor: "ac-one",
		}},
		Constraints: []artifact.Constraint{},
		Decisions:   []artifact.Decision{},
	}

	candidates := []Candidate{
		{Source: SourceOpaque, ID: "opaque:harness-vendor-base/codex/1"},
		regular(projection, "0000007"),
		{Source: SourceProjection, ID: "path:" + projection, Path: projection},
		regular("superseded.md", "0000006"),
		regular("phase-only.txt", "0000005"),
		regular("outside.txt", "0000004"),
		{Source: SourceWorktreeOverlay, ID: "path:dirty.txt", Path: "dirty.txt"},
		{Source: SourceWorktreeOverlay, ID: "path:.verdi/data", Path: ".verdi/data"},
		regular(".verdi/specs/archive/old/spec.md", "0000003"),
		// The sidecar is also a symlink so this row pins SI-90's
		// design-provenance-sidecar precedence over non-regular-file.
		{Source: SourceHeadTree, ID: "path:.verdi/specs/active/sample/design-provenance.jsonl", Path: ".verdi/specs/active/sample/design-provenance.jsonl", Object: "0000002", Mode: "120000", Type: "blob"},
		{Source: SourceHeadTree, ID: "path:link", Path: "link", Object: "0000002", Mode: "120000", Type: "blob"},
		{Source: SourceHeadTree, ID: "path:submodule", Path: "submodule", Object: "0000001", Mode: "160000", Type: "commit"},
		regular("binary.dat", "0000000"),
		regular("nul.txt", "0000000-nul"),
		regular("README.md", "1111111"),
		refCandidate(SourceDeclaredContext, "adr/architecture"),
		refCandidate(SourceStoreAuthority, "policy/build"),
		refCandidate(SourceStoreAuthority, "obligation/story--ac-one--static"),
		refCandidate(SourceStoreAuthority, "spec/feature"),
		refCandidate(SourceStoreAuthority, "spec/story"),
	}
	materials := []CandidateMaterial{
		{Source: SourceStoreAuthority, ID: "ref:spec/story", Kind: IncludedAcceptedSpec, PolicyScope: explicitUniversalScope(), Content: []byte("accepted story\n")},
		{Source: SourceStoreAuthority, ID: "ref:spec/feature", Kind: IncludedParentFeatureFragment, PolicyScope: explicitUniversalScope(), Fragment: &fragment},
		{Source: SourceStoreAuthority, ID: "ref:obligation/story--ac-one--static", Kind: IncludedObligation, PolicyScope: explicitUniversalScope(), Content: []byte("obligation\n")},
		{
			Source: SourceStoreAuthority, ID: "ref:policy/build", Kind: IncludedPolicyArtifact,
			PolicyScope: scopeWithEnvironments("production"), Content: []byte("policy\n"),
		},
		{Source: SourceDeclaredContext, ID: "ref:adr/architecture", Kind: IncludedDeclaredContextRef, PolicyScope: explicitUniversalScope(), Content: []byte("decision context\n")},
		{Source: SourceProjection, ID: "path:" + projection, Kind: IncludedInstructionProjection, PolicyScope: explicitUniversalScope(), Content: []byte("generated authority\n")},
		{Source: SourceHeadTree, ID: "path:outside.txt", Kind: IncludedRepositoryFile, PolicyScope: scopeWithPaths("inside/")},
		{Source: SourceHeadTree, ID: "path:phase-only.txt", Kind: IncludedRepositoryFile, PolicyScope: scopeWithPhases("review")},
		{Source: SourceHeadTree, ID: "path:superseded.md", Exclusion: ExclusionSupersededSpec, PolicyScope: explicitUniversalScope()},
	}

	forbidden := map[string]bool{
		projection:                         true,
		"superseded.md":                    true,
		"phase-only.txt":                   true,
		"outside.txt":                      true,
		"dirty.txt":                        true,
		".verdi/data":                      true,
		".verdi/specs/archive/old/spec.md": true,
		".verdi/specs/active/sample/design-provenance.jsonl": true,
		"link":      true,
		"submodule": true,
	}
	git := &classifyGit{
		contents: map[string][]byte{
			"README.md":  []byte("repository text\n"),
			"binary.dat": {0xff, 0x00},
			"nul.txt":    []byte("valid UTF-8\x00with NUL"),
		},
		forbidden: forbidden,
	}

	got, err := Classify(context.Background(), git, "/repo", head, ClassificationInput{
		Candidates:   candidates,
		Materials:    materials,
		Phase:        PhaseBuild,
		Environment:  "",
		RequestScope: explicitUniversalScope(),
		TargetRef:    "spec/story",
		Adapter:      AdapterRef{ID: "codex", Version: "1"},
	})
	if err != nil {
		t.Fatalf("Classify() error = %v", err)
	}

	if !reflect.DeepEqual(git.shows, []string{"README.md", "binary.dat", "nul.txt"}) {
		t.Fatalf("GitReader.Show calls = %#v, want only surviving regular blobs", git.shows)
	}
	assertClassifiedPartition(t, candidates, got)
	assertSortedClassificationRows(t, got)

	includedKinds := map[IncludedKind]bool{}
	for _, row := range got.Included {
		includedKinds[row.Kind] = true
	}
	for _, kind := range []IncludedKind{
		IncludedAcceptedSpec, IncludedParentFeatureFragment, IncludedObligation,
		IncludedPolicyArtifact, IncludedRepositoryFile, IncludedDeclaredContextRef,
		IncludedInstructionProjection,
	} {
		if !includedKinds[kind] {
			t.Errorf("included kind %q was not exercised", kind)
		}
	}

	excludedReasons := map[ExclusionReason]bool{}
	for _, row := range got.Excluded {
		excludedReasons[row.Reason] = true
	}
	for _, reason := range []ExclusionReason{
		ExclusionDesignProvenanceSidecar, ExclusionDataZoneDisposable,
		ExclusionUncommittedContent, ExclusionOutOfDeclaredScope,
		ExclusionPhaseInapplicable, ExclusionSupersededSpec, ExclusionArchivedRecord,
		ExclusionGeneratedProjectionOutput, ExclusionNonTextData, ExclusionNonRegularFile,
	} {
		if !excludedReasons[reason] {
			t.Errorf("exclusion reason %q was not exercised", reason)
		}
	}

	policyRow := findIncluded(t, got.Included, SourceStoreAuthority, "ref:policy/build")
	if policyRow.Applicability != ApplicabilityUnknown || !reflect.DeepEqual(policyRow.Disclosures, []DisclosureCode{DisclosureApplicabilityUnknown}) {
		t.Errorf("unknown policy applicability = %q/%#v", policyRow.Applicability, policyRow.Disclosures)
	}
	if row := findExcluded(t, got.Excluded, SourceHeadTree, "path:phase-only.txt"); row.Reason != ExclusionPhaseInapplicable {
		t.Errorf("phase-only reason = %q", row.Reason)
	}
	if row := findExcluded(t, got.Excluded, SourceHeadTree, "path:outside.txt"); row.Reason != ExclusionOutOfDeclaredScope {
		t.Errorf("outside reason = %q", row.Reason)
	}
	if row := findExcluded(t, got.Excluded, SourceHeadTree, "path:.verdi/specs/active/sample/design-provenance.jsonl"); row.Reason != ExclusionDesignProvenanceSidecar {
		t.Errorf("overlapping sidecar/symlink reason = %q", row.Reason)
	}
	if row := findExcluded(t, got.Excluded, SourceHeadTree, "path:nul.txt"); row.Reason != ExclusionNonTextData {
		t.Errorf("valid UTF-8 with NUL reason = %q", row.Reason)
	}
	if row := findExcluded(t, got.Excluded, SourceWorktreeOverlay, "path:.verdi/data"); row.Path == nil || *row.Path != ".verdi/data" || len(row.Disclosures) != 0 {
		t.Errorf("data boundary row = %#v", row)
	}

	if len(got.DataItems) != 6 || len(got.DataItemBytes) != 6 {
		t.Fatalf("data payload count = %d/%d, want 6/6 (projection stays raw)", len(got.DataItems), len(got.DataItemBytes))
	}
	fragmentBytes, err := EncodeFeatureFragment(fragment)
	if err != nil {
		t.Fatalf("EncodeFeatureFragment(test authority) = %v", err)
	}
	for i, item := range got.DataItems {
		decoded, err := DecodeDataItem(got.DataItemBytes[i])
		if err != nil {
			t.Fatalf("DecodeDataItem[%d] = %v", i, err)
		}
		if decoded.ID != item.ID || decoded.Digest != item.Digest {
			t.Errorf("data item[%d] bytes do not bind returned item", i)
		}
		if item.Kind == IncludedParentFeatureFragment && item.Content != string(fragmentBytes) {
			t.Errorf("parent fragment content differs from EncodeFeatureFragment bytes\ngot: %q\nwant: %q", item.Content, fragmentBytes)
		}
	}
	if len(got.ProjectionPayloads) != 1 {
		t.Fatalf("projection payload count = %d, want 1", len(got.ProjectionPayloads))
	}
	if payload := got.ProjectionPayloads[0]; payload.Path != projection || !bytes.Equal(payload.Content, []byte("generated authority\n")) || payload.Digest == "" {
		t.Errorf("projection payload = %#v", payload)
	}
	for _, item := range got.DataItems {
		if item.Kind == IncludedInstructionProjection {
			t.Fatal("instruction projection received a data wrapper")
		}
	}
}

func TestClassifyRejectsDuplicatesAndDataDescendants(t *testing.T) {
	t.Parallel()

	const head = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tests := []struct {
		name       string
		candidates []Candidate
		materials  []CandidateMaterial
		want       string
	}{
		{
			name: "duplicate candidate identity",
			candidates: []Candidate{
				{Source: SourceWorktreeOverlay, ID: "path:dirty", Path: "dirty"},
				{Source: SourceWorktreeOverlay, ID: "path:dirty", Path: "dirty"},
			},
			want: "duplicate candidate",
		},
		{
			name:       "data zone descendant violates collapsed boundary",
			candidates: []Candidate{{Source: SourceWorktreeOverlay, ID: "path:.verdi/data/secret", Path: ".verdi/data/secret"}},
			want:       "data-zone descendant",
		},
		{
			name: "duplicate material identity",
			candidates: []Candidate{
				{Source: SourceProjection, ID: "path:AGENTS.md", Path: "AGENTS.md"},
				{Source: SourceOpaque, ID: "opaque:harness-vendor-base/codex/1"},
			},
			materials: []CandidateMaterial{
				{Source: SourceProjection, ID: "path:AGENTS.md", Kind: IncludedInstructionProjection, PolicyScope: explicitUniversalScope(), Content: []byte("one")},
				{Source: SourceProjection, ID: "path:AGENTS.md", Kind: IncludedInstructionProjection, PolicyScope: explicitUniversalScope(), Content: []byte("two")},
			},
			want: "duplicate material",
		},
		{
			name: "worktree overlay refuses supplied bytes",
			candidates: []Candidate{
				{Source: SourceWorktreeOverlay, ID: "path:dirty.txt", Path: "dirty.txt"},
				{Source: SourceOpaque, ID: "opaque:harness-vendor-base/codex/1"},
			},
			materials: []CandidateMaterial{{
				Source: SourceWorktreeOverlay, ID: "path:dirty.txt", Kind: IncludedRepositoryFile,
				PolicyScope: explicitUniversalScope(), Content: []byte("secret"),
			}},
			want: "must not carry material",
		},
		{
			name: "inapplicable candidate still validates source kind",
			candidates: []Candidate{
				{Source: SourceDeclaredContext, ID: "ref:adr/architecture", Ref: "adr/architecture"},
				{Source: SourceOpaque, ID: "opaque:harness-vendor-base/codex/1"},
			},
			materials: []CandidateMaterial{{
				Source: SourceDeclaredContext, ID: "ref:adr/architecture", Kind: IncludedPolicyArtifact,
				PolicyScope: scopeWithPhases("review"), Content: []byte("wrong kind"),
			}},
			want: "cannot carry included kind",
		},
		{
			name: "store authority candidate rejects path traversal as ref",
			candidates: []Candidate{
				{Source: SourceStoreAuthority, ID: "ref:../x", Ref: "../x"},
				{Source: SourceOpaque, ID: "opaque:harness-vendor-base/codex/1"},
			},
			materials: []CandidateMaterial{{
				Source: SourceStoreAuthority, ID: "ref:../x", Kind: IncludedAcceptedSpec,
				PolicyScope: explicitUniversalScope(), Content: []byte("spec\n"),
			}},
			want: "artifact ref",
		},
		{
			name: "declared context candidate rejects malformed fragment ref",
			candidates: []Candidate{
				{Source: SourceDeclaredContext, ID: "ref:spec/story#", Ref: "spec/story#"},
				{Source: SourceOpaque, ID: "opaque:harness-vendor-base/codex/1"},
			},
			materials: []CandidateMaterial{{
				Source: SourceDeclaredContext, ID: "ref:spec/story#", Kind: IncludedDeclaredContextRef,
				PolicyScope: explicitUniversalScope(), Content: []byte("context\n"),
			}},
			want: "artifact ref",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Classify(context.Background(), &classifyGit{}, "/repo", head, ClassificationInput{
				Candidates: tt.candidates, Materials: tt.materials, Phase: PhaseBuild,
				RequestScope: explicitUniversalScope(), TargetRef: "spec/story",
				Adapter: AdapterRef{ID: "codex", Version: "1"},
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Classify() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

// TestClassifyRejectsCandidateSourceShapeMismatch proves Classify's real,
// production candidate-shape check (validateClassificationCandidate)
// refuses a candidate whose Source tag disagrees with its own ID/path/ref
// shape — a declared-context-shaped candidate mismarked SourceHeadTree, a
// head-tree-shaped one mismarked SourceProjection, a projection-shaped one
// mismarked SourceHeadTree, and the opaque base candidate mismarked
// SourceHeadTree. Ported from the deleted dead ComposeCapsule seam's own
// TestComposeCapsule_RejectsWrongSemanticSource (contextcompile Wave-3
// Task 9 C3): that seam had zero production callers, but the underlying
// contract — Compile refuses a source/shape mismatch — is real and is
// enforced here, on the code path Classify (and therefore Compile) always
// calls.
func TestClassifyRejectsCandidateSourceShapeMismatch(t *testing.T) {
	t.Parallel()
	const head = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	opaque := Candidate{Source: SourceOpaque, ID: "opaque:harness-vendor-base/codex/1"}

	tests := []struct {
		name      string
		candidate Candidate
	}{
		{
			name:      "declared-context-shaped candidate mismarked head-tree",
			candidate: Candidate{Source: SourceHeadTree, ID: "ref:spec/reference@" + strings.Repeat("a", 40), Ref: "spec/reference@" + strings.Repeat("a", 40)},
		},
		{
			name:      "head-tree-shaped candidate mismarked projection",
			candidate: Candidate{Source: SourceProjection, ID: "path:README.md", Path: "README.md", Object: strings.Repeat("b", 40), Mode: "100644", Type: "blob"},
		},
		{
			name:      "projection-shaped candidate mismarked head-tree",
			candidate: Candidate{Source: SourceHeadTree, ID: "path:AGENTS.md", Path: "AGENTS.md"},
		},
		{
			name:      "opaque candidate mismarked head-tree",
			candidate: Candidate{Source: SourceHeadTree, ID: "opaque:harness-vendor-base/codex/1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			candidates := []Candidate{tt.candidate}
			if tt.candidate.Source != SourceOpaque {
				candidates = append(candidates, opaque)
			}
			_, err := Classify(context.Background(), &classifyGit{}, "/repo", head, ClassificationInput{
				Candidates: candidates, Phase: PhaseBuild,
				RequestScope: explicitUniversalScope(), TargetRef: "spec/story",
				Adapter: AdapterRef{ID: "codex", Version: "1"},
			})
			if err == nil || !strings.Contains(err.Error(), "noncanonical") {
				t.Fatalf("Classify() error = %v, want containing %q", err, "noncanonical")
			}
		})
	}
}

func TestClassifyEvaluatesPolicyScopeAgainstTargetRef(t *testing.T) {
	t.Parallel()

	const head = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	policyCandidate := Candidate{Source: SourceStoreAuthority, ID: "ref:policy/build", Ref: "policy/build"}
	policyMaterial := CandidateMaterial{
		Source: SourceStoreAuthority, ID: policyCandidate.ID, Kind: IncludedPolicyArtifact,
		PolicyScope: scopeWithRefs("spec/story"), Content: []byte("policy\n"),
	}
	candidates := []Candidate{
		policyCandidate,
		{Source: SourceOpaque, ID: "opaque:harness-vendor-base/codex/1"},
	}
	tests := []struct {
		name       string
		targetRef  string
		wantLedger string
	}{
		{name: "matching target includes scoped policy", targetRef: "spec/story", wantLedger: "included"},
		{name: "different target excludes scoped policy", targetRef: "spec/other", wantLedger: "excluded"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Classify(context.Background(), &classifyGit{}, "/repo", head, ClassificationInput{
				Candidates: candidates, Materials: []CandidateMaterial{policyMaterial}, Phase: PhaseBuild,
				RequestScope: explicitUniversalScope(), TargetRef: tt.targetRef,
				Adapter: AdapterRef{ID: "codex", Version: "1"},
			})
			if err != nil {
				t.Fatalf("Classify() error = %v", err)
			}
			switch tt.wantLedger {
			case "included":
				row := findIncluded(t, got.Included, SourceStoreAuthority, policyCandidate.ID)
				if row.Applicability != ApplicabilityApplicable {
					t.Fatalf("policy applicability = %q, want applicable", row.Applicability)
				}
			case "excluded":
				row := findExcluded(t, got.Excluded, SourceStoreAuthority, policyCandidate.ID)
				if row.Reason != ExclusionOutOfDeclaredScope || row.Applicability != ApplicabilityInapplicable {
					t.Fatalf("policy exclusion = %#v, want out-of-declared-scope/inapplicable", row)
				}
			default:
				t.Fatalf("test setup: unknown ledger %q", tt.wantLedger)
			}
		})
	}
}

func assertClassifiedPartition(t *testing.T, candidates []Candidate, got ClassificationResult) {
	t.Helper()
	want := map[string]bool{}
	for _, candidate := range candidates {
		want[classificationKey(candidate.Source, candidate.ID)] = true
	}
	seen := map[string]string{}
	add := func(source Source, id, ledger string) {
		key := classificationKey(source, id)
		if prior := seen[key]; prior != "" {
			t.Errorf("candidate %s appears in both %s and %s", key, prior, ledger)
		}
		seen[key] = ledger
	}
	for _, row := range got.Included {
		add(row.Source, row.ID, "included")
	}
	for _, row := range got.Excluded {
		add(row.Source, row.ID, "excluded")
	}
	for _, row := range got.Opaque {
		add(SourceOpaque, row.ID, "opaque")
	}
	if !reflect.DeepEqual(seenKeys(seen), boolKeys(want)) {
		t.Errorf("classified union = %v, want %v", seenKeys(seen), boolKeys(want))
	}
}

func assertSortedClassificationRows(t *testing.T, got ClassificationResult) {
	t.Helper()
	var included, excluded, opaque []string
	for _, row := range got.Included {
		included = append(included, classificationKey(row.Source, row.ID))
	}
	for _, row := range got.Excluded {
		excluded = append(excluded, classificationKey(row.Source, row.ID))
	}
	for _, row := range got.Opaque {
		opaque = append(opaque, row.ID)
	}
	for name, keys := range map[string][]string{"included": included, "excluded": excluded, "opaque": opaque} {
		if !sort.StringsAreSorted(keys) {
			t.Errorf("%s rows are not sorted: %v", name, keys)
		}
	}
}

func findIncluded(t *testing.T, rows []IncludedEntry, source Source, id string) IncludedEntry {
	t.Helper()
	for _, row := range rows {
		if row.Source == source && row.ID == id {
			return row
		}
	}
	t.Fatalf("included row %s/%s not found", source, id)
	return IncludedEntry{}
}

func findExcluded(t *testing.T, rows []ExcludedEntry, source Source, id string) ExcludedEntry {
	t.Helper()
	for _, row := range rows {
		if row.Source == source && row.ID == id {
			return row
		}
	}
	t.Fatalf("excluded row %s/%s not found", source, id)
	return ExcludedEntry{}
}

func classificationKey(source Source, id string) string { return string(source) + "\x00" + id }

func seenKeys(in map[string]string) []string {
	out := make([]string, 0, len(in))
	for key := range in {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func boolKeys(in map[string]bool) []string {
	out := make([]string, 0, len(in))
	for key := range in {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
