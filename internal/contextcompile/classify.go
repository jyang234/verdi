package contextcompile

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/store"
)

// CandidateMaterial supplies the semantic facts a raw universe candidate
// cannot carry: its included kind, candidate-owned policy scope and exact
// already-resolved bytes. Parent-feature fragments use Fragment instead of
// Content so the classifier must call the one canonical fragment encoder.
// Exclusion is reserved for already-proven semantic terminal state.
type CandidateMaterial struct {
	Source      Source
	ID          string
	Kind        IncludedKind
	PolicyScope policyartifact.Scope
	Content     []byte
	Fragment    *FeatureFragment
	Exclusion   ExclusionReason
}

// ProjectionPayload is one raw authority-channel payload. It is not a data
// item and never carries a provenance wrapper.
type ProjectionPayload struct {
	Path    string
	Content []byte
	Digest  string
}

// ClassificationInput is the pure classifier's complete declared input.
// Candidates must be the already-discovered closed universe; Materials supply
// only facts resolved through earlier trusted seams.
type ClassificationInput struct {
	Candidates   []Candidate
	Materials    []CandidateMaterial
	Phase        Phase
	Environment  string
	RequestScope policyartifact.Scope
	Adapter      AdapterRef
}

// ClassificationResult is the exact total partition plus its in-memory data
// and projection payloads. DataItems and DataItemBytes are index-aligned.
type ClassificationResult struct {
	Included           []IncludedEntry
	Excluded           []ExcludedEntry
	Opaque             []OpaqueEntry
	DataItems          []DataItem
	DataItemBytes      [][]byte
	ProjectionPayloads []ProjectionPayload
}

// Classify partitions every candidate exactly once. It reads committed bytes
// only through git.Show, only for regular HEAD blobs that survive every
// path-level and applicability exclusion.
func Classify(ctx context.Context, git GitReader, root, head string, in ClassificationInput) (ClassificationResult, error) {
	if ctx == nil {
		return ClassificationResult{}, fmt.Errorf("contextcompile: Classify: context is nil")
	}
	if git == nil {
		return ClassificationResult{}, fmt.Errorf("contextcompile: Classify: GitReader is nil")
	}
	if root == "" {
		return ClassificationResult{}, fmt.Errorf("contextcompile: Classify: repository root is empty")
	}
	if err := validateGitHash("Classify HEAD", head); err != nil {
		return ClassificationResult{}, err
	}
	if err := in.Phase.Validate(); err != nil {
		return ClassificationResult{}, err
	}
	if err := in.RequestScope.Validate(); err != nil {
		return ClassificationResult{}, fmt.Errorf("contextcompile: Classify request scope: %w", err)
	}
	if err := in.Adapter.validate("Classify adapter"); err != nil {
		return ClassificationResult{}, err
	}
	if in.Candidates == nil {
		return ClassificationResult{}, fmt.Errorf("contextcompile: Classify: candidates must be an explicit array")
	}
	if in.Materials == nil {
		in.Materials = []CandidateMaterial{}
	}

	candidates := append([]Candidate(nil), in.Candidates...)
	candidateSet := make(map[string]bool, len(candidates))
	projectionPaths := make(map[string]bool)
	opaqueCount := 0
	for i, candidate := range candidates {
		if err := validateClassificationCandidate(candidate, in.Adapter); err != nil {
			return ClassificationResult{}, fmt.Errorf("contextcompile: Classify candidates[%d]: %w", i, err)
		}
		key := candidateKey(candidate.Source, candidate.ID)
		if candidateSet[key] {
			return ClassificationResult{}, fmt.Errorf("contextcompile: Classify: duplicate candidate identity %s/%s", candidate.Source, candidate.ID)
		}
		candidateSet[key] = true
		if candidate.Source == SourceProjection {
			projectionPaths[candidate.Path] = true
		}
		if candidate.Source == SourceOpaque {
			opaqueCount++
		}
	}
	if opaqueCount != 1 {
		return ClassificationResult{}, fmt.Errorf("contextcompile: Classify: universe must contain exactly one opaque harness base, got %d", opaqueCount)
	}

	materials := make(map[string]CandidateMaterial, len(in.Materials))
	for i, material := range in.Materials {
		key := candidateKey(material.Source, material.ID)
		if materials[key].ID != "" {
			return ClassificationResult{}, fmt.Errorf("contextcompile: Classify: duplicate material identity %s/%s", material.Source, material.ID)
		}
		if !candidateSet[key] {
			return ClassificationResult{}, fmt.Errorf("contextcompile: Classify materials[%d]: %s/%s is absent from the candidate universe", i, material.Source, material.ID)
		}
		if err := validateCandidateMaterial(material); err != nil {
			return ClassificationResult{}, fmt.Errorf("contextcompile: Classify materials[%d]: %w", i, err)
		}
		materials[key] = cloneCandidateMaterial(material)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidateKey(candidates[i].Source, candidates[i].ID) < candidateKey(candidates[j].Source, candidates[j].ID)
	})
	result := ClassificationResult{
		Included: []IncludedEntry{}, Excluded: []ExcludedEntry{}, Opaque: []OpaqueEntry{},
		DataItems: []DataItem{}, DataItemBytes: [][]byte{}, ProjectionPayloads: []ProjectionPayload{},
	}

	for _, candidate := range candidates {
		material, hasMaterial := materials[candidateKey(candidate.Source, candidate.ID)]
		if candidate.Source == SourceOpaque {
			if hasMaterial {
				return ClassificationResult{}, fmt.Errorf("contextcompile: Classify: opaque candidate %s must not carry material", candidate.ID)
			}
			result.Opaque = append(result.Opaque, OpaqueEntry{
				ID: candidate.ID, Kind: OpaqueKindHarnessVendorBase, Adapter: in.Adapter,
				Disclosures: []DisclosureCode{DisclosureOpaqueHarnessVendorBase},
			})
			continue
		}

		policyScope := universalApplicabilityScope()
		if hasMaterial {
			policyScope = material.PolicyScope
		}
		applicability, err := evaluateApplicability(ApplicabilityInput{
			Policy: policyScope, Request: in.RequestScope, CandidatePath: candidate.Path,
			CandidateRef: candidate.Ref, Phase: in.Phase, Environment: in.Environment,
		})
		if err != nil {
			return ClassificationResult{}, fmt.Errorf("contextcompile: Classify %s/%s applicability: %w", candidate.Source, candidate.ID, err)
		}

		if reason, excluded, err := fixedExclusion(candidate, material, hasMaterial, projectionPaths); err != nil {
			return ClassificationResult{}, err
		} else if excluded {
			result.Excluded = append(result.Excluded, excludedEntry(candidate, reason, applicability.ApplicabilityResult))
			continue
		}
		if applicability.State == ApplicabilityInapplicable {
			reason := ExclusionOutOfDeclaredScope
			if applicability.phaseInapplicable {
				reason = ExclusionPhaseInapplicable
			}
			result.Excluded = append(result.Excluded, excludedEntry(candidate, reason, applicability.ApplicabilityResult))
			continue
		}

		kind, content, err := includedContent(ctx, git, root, head, candidate, material, hasMaterial)
		if err != nil {
			return ClassificationResult{}, err
		}
		if kind != IncludedInstructionProjection && nonTextContent(content) {
			result.Excluded = append(result.Excluded, excludedEntry(candidate, ExclusionNonTextData, applicability.ApplicabilityResult))
			continue
		}
		if kind == IncludedInstructionProjection {
			if !utf8.Valid(content) {
				return ClassificationResult{}, fmt.Errorf("contextcompile: Classify projection %s is not valid UTF-8", candidate.Path)
			}
			digest := rawContentDigest(content)
			result.Included = append(result.Included, includedEntry(candidate, kind, ChannelAuthority, digest, digest, applicability.ApplicabilityResult))
			result.ProjectionPayloads = append(result.ProjectionPayloads, ProjectionPayload{
				Path: candidate.Path, Content: append([]byte(nil), content...), Digest: digest,
			})
			continue
		}

		item, encoded, err := BuildDataItem(candidate, kind, content)
		if err != nil {
			return ClassificationResult{}, fmt.Errorf("contextcompile: Classify %s/%s data payload: %w", candidate.Source, candidate.ID, err)
		}
		result.Included = append(result.Included, includedEntry(candidate, kind, ChannelData, item.ContentDigest, item.Digest, applicability.ApplicabilityResult))
		result.DataItems = append(result.DataItems, item)
		result.DataItemBytes = append(result.DataItemBytes, append([]byte(nil), encoded...))
	}

	sort.Slice(result.Included, func(i, j int) bool {
		return candidateKey(result.Included[i].Source, result.Included[i].ID) < candidateKey(result.Included[j].Source, result.Included[j].ID)
	})
	sort.Slice(result.Excluded, func(i, j int) bool {
		return candidateKey(result.Excluded[i].Source, result.Excluded[i].ID) < candidateKey(result.Excluded[j].Source, result.Excluded[j].ID)
	})
	sort.Slice(result.Opaque, func(i, j int) bool { return result.Opaque[i].ID < result.Opaque[j].ID })
	sort.Slice(result.ProjectionPayloads, func(i, j int) bool {
		return result.ProjectionPayloads[i].Path < result.ProjectionPayloads[j].Path
	})
	return result, nil
}

func validateClassificationCandidate(candidate Candidate, adapter AdapterRef) error {
	if err := candidate.Source.Validate(); err != nil {
		return err
	}
	if candidate.ID == "" {
		return fmt.Errorf("candidate id is empty")
	}
	if candidate.Path != "" && inDataZone(candidate.Path) {
		return fmt.Errorf("data-zone descendant is forbidden: the universe must carry only the unnamed collapsed boundary")
	}
	switch candidate.Source {
	case SourceHeadTree:
		if candidate.Path == "" || candidate.ID != pathID(candidate.Path) || candidate.Ref != "" || candidate.Object == "" || candidate.Mode == "" || candidate.Type == "" {
			return fmt.Errorf("noncanonical head-tree candidate %q", candidate.ID)
		}
		if err := validateCandidatePath(candidate.Path); err != nil {
			return err
		}
	case SourceWorktreeOverlay:
		if candidate.Path == "" || candidate.ID != pathID(candidate.Path) || candidate.Ref != "" || candidate.Object != "" || candidate.Mode != "" || candidate.Type != "" {
			return fmt.Errorf("noncanonical worktree-overlay candidate %q", candidate.ID)
		}
		if err := validateCandidatePath(candidate.Path); err != nil {
			return err
		}
	case SourceStoreAuthority, SourceDeclaredContext:
		if candidate.Ref == "" || candidate.ID != refID(candidate.Ref) || candidate.Path != "" || candidate.Object != "" || candidate.Mode != "" || candidate.Type != "" {
			return fmt.Errorf("noncanonical %s candidate %q", candidate.Source, candidate.ID)
		}
	case SourceProjection:
		if candidate.Path == "" || candidate.ID != pathID(candidate.Path) || candidate.Ref != "" || candidate.Object != "" || candidate.Mode != "" || candidate.Type != "" {
			return fmt.Errorf("noncanonical projection candidate %q", candidate.ID)
		}
		if err := validateCandidatePath(candidate.Path); err != nil {
			return err
		}
	case SourceOpaque:
		want := fmt.Sprintf("opaque:%s/%s/%s", OpaqueKindHarnessVendorBase, adapter.ID, adapter.Version)
		if candidate.ID != want || candidate.Path != "" || candidate.Ref != "" || candidate.Object != "" || candidate.Mode != "" || candidate.Type != "" {
			return fmt.Errorf("opaque candidate %q, want exact identity %q", candidate.ID, want)
		}
	}
	return nil
}

func validateCandidateMaterial(material CandidateMaterial) error {
	if material.ID == "" {
		return fmt.Errorf("material id is empty")
	}
	if err := material.Source.Validate(); err != nil {
		return err
	}
	if err := material.PolicyScope.Validate(); err != nil {
		return fmt.Errorf("material policy scope: %w", err)
	}
	if material.Source == SourceWorktreeOverlay || material.Source == SourceOpaque {
		return fmt.Errorf("source %q must not carry material", material.Source)
	}
	if material.Exclusion != "" {
		if err := material.Exclusion.Validate(); err != nil {
			return err
		}
		if material.Exclusion != ExclusionSupersededSpec && material.Exclusion != ExclusionArchivedRecord {
			return fmt.Errorf("material exclusion %q is not a semantic-state exclusion", material.Exclusion)
		}
		if material.Content != nil || material.Fragment != nil {
			return fmt.Errorf("excluded material %s/%s must not carry content", material.Source, material.ID)
		}
		return nil
	}
	if err := material.Kind.Validate(); err != nil {
		return err
	}
	if material.Source != SourceHeadTree {
		if err := validateMaterialSourceKind(material.Source, material.Kind); err != nil {
			return err
		}
	}
	if material.Kind == IncludedParentFeatureFragment {
		if material.Fragment == nil || material.Content != nil {
			// vocab:identity — SI-84/SI-88 schema diagnostic naming the fixed parent-feature-fragment identity
			return fmt.Errorf("parent-feature-fragment material requires Fragment and no Content")
		}
	} else if material.Fragment != nil {
		// vocab:identity — SI-84/SI-88 schema diagnostic naming the fixed feature-fragment identity
		return fmt.Errorf("material kind %q must not carry a feature fragment", material.Kind)
	}
	if material.Source == SourceHeadTree {
		if material.Kind != IncludedRepositoryFile || material.Content != nil || material.Fragment != nil {
			return fmt.Errorf("head-tree material may only scope or semantically exclude repository-file candidates")
		}
	} else if material.Kind != IncludedParentFeatureFragment && material.Content == nil {
		return fmt.Errorf("material %s/%s requires exact content bytes", material.Source, material.ID)
	}
	return nil
}

func fixedExclusion(candidate Candidate, material CandidateMaterial, hasMaterial bool, projectionPaths map[string]bool) (ExclusionReason, bool, error) {
	if candidate.Path == dataZoneBoundaryPath {
		return ExclusionDataZoneDisposable, true, nil
	}
	if candidate.Source == SourceWorktreeOverlay {
		return ExclusionUncommittedContent, true, nil
	}
	if candidate.Source == SourceHeadTree && designProvenancePath(candidate.Path) {
		return ExclusionDesignProvenanceSidecar, true, nil
	}
	if candidate.Source == SourceHeadTree && !regularBlob(candidate) {
		return ExclusionNonRegularFile, true, nil
	}
	if candidate.Source == SourceHeadTree && projectionPaths[candidate.Path] {
		return ExclusionGeneratedProjectionOutput, true, nil
	}
	if candidate.Source == SourceHeadTree && strings.HasPrefix(candidate.Path, ".verdi/specs/archive/") {
		return ExclusionArchivedRecord, true, nil
	}
	if hasMaterial && material.Exclusion != "" {
		return material.Exclusion, true, nil
	}
	return "", false, nil
}

func includedContent(ctx context.Context, git GitReader, root, head string, candidate Candidate, material CandidateMaterial, hasMaterial bool) (IncludedKind, []byte, error) {
	if candidate.Source == SourceHeadTree {
		content, err := git.Show(ctx, root, head, candidate.Path)
		if err != nil {
			return "", nil, fmt.Errorf("contextcompile: Classify: read HEAD blob %s: %w", candidate.Path, err)
		}
		return IncludedRepositoryFile, append([]byte(nil), content...), nil
	}
	if !hasMaterial {
		return "", nil, fmt.Errorf("contextcompile: Classify: included candidate %s/%s has no resolved material", candidate.Source, candidate.ID)
	}
	if err := validateMaterialSourceKind(candidate.Source, material.Kind); err != nil {
		return "", nil, err
	}
	if material.Kind == IncludedParentFeatureFragment {
		content, err := EncodeFeatureFragment(*material.Fragment)
		if err != nil {
			return "", nil, fmt.Errorf("contextcompile: Classify: encode %s: %w", candidate.ID, err)
		}
		return material.Kind, content, nil
	}
	if material.Content == nil {
		return "", nil, fmt.Errorf("contextcompile: Classify: material %s/%s has no content", material.Source, material.ID)
	}
	return material.Kind, append([]byte(nil), material.Content...), nil
}

func validateMaterialSourceKind(source Source, kind IncludedKind) error {
	switch source {
	case SourceStoreAuthority:
		switch kind {
		case IncludedAcceptedSpec, IncludedParentFeatureFragment, IncludedObligation, IncludedPolicyArtifact:
			return nil
		}
	case SourceDeclaredContext:
		if kind == IncludedDeclaredContextRef {
			return nil
		}
	case SourceProjection:
		if kind == IncludedInstructionProjection {
			return nil
		}
	}
	return fmt.Errorf("contextcompile: Classify: source %q cannot carry included kind %q", source, kind)
}

func excludedEntry(candidate Candidate, reason ExclusionReason, applicability ApplicabilityResult) ExcludedEntry {
	row := ExcludedEntry{
		ID: candidate.ID, Source: candidate.Source, Reason: reason,
		Applicability: applicability.State, Disclosures: append([]DisclosureCode{}, applicability.Disclosures...),
	}
	setEntryLocation(candidate, &row.Path, &row.Ref)
	return row
}

func includedEntry(candidate Candidate, kind IncludedKind, channel PayloadChannel, contentDigest, payloadDigest string, applicability ApplicabilityResult) IncludedEntry {
	row := IncludedEntry{
		ID: candidate.ID, Source: candidate.Source, Kind: kind, Applicability: applicability.State,
		PayloadChannel: channel, ContentDigest: contentDigest, PayloadDigest: payloadDigest,
		Disclosures: append([]DisclosureCode{}, applicability.Disclosures...),
	}
	setEntryLocation(candidate, &row.Path, &row.Ref)
	return row
}

func setEntryLocation(candidate Candidate, path, ref **string) {
	if candidate.Path != "" {
		value := candidate.Path
		*path = &value
	}
	if candidate.Ref != "" {
		value := candidate.Ref
		*ref = &value
	}
}

func regularBlob(candidate Candidate) bool {
	return candidate.Type == "blob" && (candidate.Mode == "100644" || candidate.Mode == "100755")
}

func designProvenancePath(repoPath string) bool {
	parts := strings.Split(repoPath, "/")
	if len(parts) != 5 || parts[0] != ".verdi" || parts[1] != "specs" {
		return false
	}
	zone := parts[2]
	if zone != store.ZoneActive && zone != store.ZoneArchive {
		return false
	}
	identity, err := designprovenance.ResolveIdentity("spec/"+parts[3], zone)
	return err == nil && identity.RelPath == repoPath && identity.ExclusionReason == string(ExclusionDesignProvenanceSidecar)
}

func universalApplicabilityScope() policyartifact.Scope {
	return policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}}
}

func candidateKey(source Source, id string) string { return string(source) + "\x00" + id }

func cloneCandidateMaterial(in CandidateMaterial) CandidateMaterial {
	out := in
	out.PolicyScope = policyartifact.Scope{
		Phases: append([]string{}, in.PolicyScope.Phases...), Environments: append([]string{}, in.PolicyScope.Environments...),
		Paths: append([]string{}, in.PolicyScope.Paths...), Refs: append([]string{}, in.PolicyScope.Refs...),
	}
	out.Content = append([]byte(nil), in.Content...)
	if in.Content == nil {
		out.Content = nil
	}
	if in.Fragment != nil {
		fragments := cloneFeatureFragments([]FeatureFragment{*in.Fragment})
		out.Fragment = &fragments[0]
	}
	return out
}
