package experimentapp

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentpolicy"
)

// DraftDefinitionInput supplies the exact strict definition bytes to install.
type DraftDefinitionInput struct {
	DefinitionBytes  []byte
	CandidatePatches map[string][]byte
}

// CaptureCandidateInput binds one exact patch replacement to the strict
// definition bytes carrying its new digest.
type CaptureCandidateInput struct {
	CandidateID     string
	PatchBytes      []byte
	DefinitionBytes []byte
}

// DraftMutationResult is the shared typed result of draft authoring,
// candidate capture, and explicit direct-edit reconciliation.
type DraftMutationResult struct {
	Outcome          Outcome
	AcceptedHead     string
	ExperimentPath   string
	ArtifactDigest   string
	ProvenanceDigest string
}

type preparedMutation struct {
	revision       DefaultBranch
	experimentPath string
	files          map[string][]byte
	policyDigest   string
}

// DraftDefinition installs exact unlocked definition bytes and appends one
// canonical draft-definition provenance record.
func (s *Service) DraftDefinition(ctx context.Context, identity Identity, input DraftDefinitionInput) DraftMutationResult {
	return s.draftDefinition(ctx, identity, input, draftmutation.Coordinator{})
}

func (s *Service) draftDefinition(ctx context.Context, identity Identity, input DraftDefinitionInput, coordinator draftmutation.Coordinator) DraftMutationResult {
	if len(input.CandidatePatches) > 0 {
		return s.createDraftDefinition(ctx, identity, input, coordinator)
	}
	prepared, outcome := s.prepareMutation(ctx, identity, input.DefinitionBytes, nil, false)
	if outcome.Classification != ClassificationClean {
		return DraftMutationResult{Outcome: outcome}
	}
	definitionPath := path.Join(prepared.experimentPath, "experiment.yaml")
	return s.commitDraftMutation(ctx, identity, prepared, experiment.MutationDraftDefinition, map[string][]byte{definitionPath: input.DefinitionBytes}, coordinator)
}

func (s *Service) createDraftDefinition(ctx context.Context, identity Identity, input DraftDefinitionInput, coordinator draftmutation.Coordinator) DraftMutationResult {
	if err := identity.validate(); err != nil {
		return DraftMutationResult{Outcome: operationalOutcome("invalid-request", err)}
	}
	accepted, err := resolveAcceptedBase(ctx, s.git, identity)
	if err != nil {
		var stale *staleAcceptedHeadError
		if errors.As(err, &stale) {
			return DraftMutationResult{Outcome: verdictOutcome("accepted-head-stale", stale.Error())}
		}
		return DraftMutationResult{Outcome: operationalOutcome("accepted-tree-invalid", err)}
	}
	if accepted.experimentPath != "" {
		return DraftMutationResult{Outcome: operationalOutcome("proposal-location-invalid", fmt.Errorf("accepted experiment exists but its worktree proposal is absent"))}
	}
	definition, err := experiment.DecodeDefinition(input.DefinitionBytes)
	if err != nil {
		return DraftMutationResult{Outcome: operationalOutcome("definition-invalid", err)}
	}
	if definition.ID != identity.ExperimentID || definition.Spike != identity.Spike {
		return DraftMutationResult{Outcome: operationalOutcome("definition-identity-mismatch", fmt.Errorf("definition identity does not match the operation"))}
	}
	locked, err := experiment.Locked(definition)
	if err != nil {
		return DraftMutationResult{Outcome: operationalOutcome("definition-invalid", err)}
	}
	if locked || definition.Lock != nil {
		return DraftMutationResult{Outcome: verdictOutcome("definition-locked", "a new experiment proposal must be unlocked")}
	}
	spikeID := strings.TrimPrefix(identity.Spike, "spec/")
	experimentPath := path.Join(".verdi/specs/active", spikeID, "experiments", identity.ExperimentID)
	spikeDirectory := filepath.Join(identity.CheckoutRoot, ".verdi", "specs", "active", spikeID)
	if info, statErr := os.Lstat(spikeDirectory); statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return DraftMutationResult{Outcome: operationalOutcome("proposal-location-invalid", fmt.Errorf("active parent-spec directory is missing, a symlink, or not a directory"))}
	}
	for _, candidatePath := range []string{
		filepath.Join(identity.CheckoutRoot, filepath.FromSlash(experimentPath), "experiment.yaml"),
		filepath.Join(identity.CheckoutRoot, ".verdi", "specs", "archive", spikeID, "experiments", identity.ExperimentID, "experiment.yaml"),
	} {
		if _, statErr := os.Lstat(candidatePath); statErr == nil || !errors.Is(statErr, os.ErrNotExist) {
			return DraftMutationResult{Outcome: operationalOutcome("proposal-location-invalid", fmt.Errorf("experiment proposal already exists or cannot be inspected"))}
		}
	}
	files := map[string][]byte{path.Join(experimentPath, "experiment.yaml"): append([]byte(nil), input.DefinitionBytes...)}
	wantCandidates := make(map[string]string, len(definition.Candidates))
	for _, candidate := range definition.Candidates {
		wantCandidates[candidate.ID] = path.Join(experimentPath, candidate.Patch)
	}
	if len(input.CandidatePatches) != len(wantCandidates) {
		return DraftMutationResult{Outcome: operationalOutcome("candidate-invalid", fmt.Errorf("new experiment proposal requires one exact patch for every registered candidate"))}
	}
	for candidateID, patchBytes := range input.CandidatePatches {
		candidatePath, ok := wantCandidates[candidateID]
		if !ok {
			return DraftMutationResult{Outcome: operationalOutcome("candidate-invalid", fmt.Errorf("candidate patch %q is not registered", candidateID))}
		}
		files[candidatePath] = append([]byte(nil), patchBytes...)
	}
	candidatePaths, err := experiment.ValidateCandidatePatchesFromSource(newSnapshotFS(files), experimentPath, definition)
	if err != nil {
		return DraftMutationResult{Outcome: verdictOutcome("candidate-refused", err.Error())}
	}
	policyDigest, outcome := s.authorizeMutation(ctx, identity, experimentPath, definition, candidatePaths)
	if outcome.Classification != ClassificationClean {
		return DraftMutationResult{Outcome: outcome}
	}
	prepared := preparedMutation{revision: accepted.revision, experimentPath: experimentPath, files: map[string][]byte{}, policyDigest: policyDigest}
	return s.commitDraftMutation(ctx, identity, prepared, experiment.MutationDraftDefinition, files, coordinator)
}

// CaptureCandidate validates the exact binary-capable patch, its protected
// paths, and the accompanying definition digest before installing both.
func (s *Service) CaptureCandidate(ctx context.Context, identity Identity, input CaptureCandidateInput) DraftMutationResult {
	if err := identity.validate(); err != nil {
		return DraftMutationResult{Outcome: operationalOutcome("invalid-request", err)}
	}
	if err := experiment.ValidateID(input.CandidateID); err != nil {
		return DraftMutationResult{Outcome: operationalOutcome("invalid-request", err)}
	}
	definition, err := experiment.DecodeDefinition(input.DefinitionBytes)
	if err != nil {
		return DraftMutationResult{Outcome: operationalOutcome("definition-invalid", err)}
	}
	var patchPath string
	for _, candidate := range definition.Candidates {
		if candidate.ID == input.CandidateID {
			patchPath = candidate.Patch
			break
		}
	}
	if patchPath == "" {
		return DraftMutationResult{Outcome: verdictOutcome("candidate-refused", "candidate is not registered in the definition")}
	}
	experimentPath, err := proposedExperimentPath(identity)
	if err != nil {
		return DraftMutationResult{Outcome: operationalOutcome("proposal-location-invalid", err)}
	}
	patchPath = path.Join(experimentPath, patchPath)
	prepared, outcome := s.prepareMutation(ctx, identity, input.DefinitionBytes, map[string][]byte{patchPath: input.PatchBytes}, true)
	if outcome.Classification != ClassificationClean {
		return DraftMutationResult{Outcome: outcome}
	}
	definitionPath := path.Join(prepared.experimentPath, "experiment.yaml")
	return s.commitDraftMutation(ctx, identity, prepared, experiment.MutationCaptureCandidate, map[string][]byte{
		definitionPath: input.DefinitionBytes,
		patchPath:      input.PatchBytes,
	}, draftmutation.Coordinator{})
}

func (s *Service) prepareMutation(ctx context.Context, identity Identity, definitionBytes []byte, replacements map[string][]byte, candidateErrorsVerdict bool) (preparedMutation, Outcome) {
	if err := identity.validate(); err != nil {
		return preparedMutation{}, operationalOutcome("invalid-request", err)
	}
	revision, err := resolveAcceptedHead(ctx, s.git, identity)
	if err != nil {
		var stale *staleAcceptedHeadError
		if errors.As(err, &stale) {
			return preparedMutation{}, verdictOutcome("accepted-head-stale", stale.Error())
		}
		return preparedMutation{}, operationalOutcome("accepted-head-invalid", err)
	}
	experimentPath, err := proposedExperimentPath(identity)
	if err != nil {
		return preparedMutation{}, operationalOutcome("proposal-location-invalid", err)
	}
	files, err := readProposedArtifactFiles(identity.CheckoutRoot, experimentPath)
	if err != nil {
		return preparedMutation{}, operationalOutcome("proposal-tree-invalid", err)
	}
	currentBytes, ok := files[path.Join(experimentPath, "experiment.yaml")]
	if !ok {
		return preparedMutation{}, operationalOutcome("definition-unreadable", fs.ErrNotExist)
	}
	current, err := experiment.DecodeDefinition(currentBytes)
	if err != nil {
		return preparedMutation{}, operationalOutcome("definition-invalid", err)
	}
	locked, err := experiment.Locked(current)
	if err != nil {
		return preparedMutation{}, operationalOutcome("definition-invalid", err)
	}
	if locked {
		return preparedMutation{}, verdictOutcome("definition-locked", "locked experiment definitions are immutable")
	}
	definition, err := experiment.DecodeDefinition(definitionBytes)
	if err != nil {
		return preparedMutation{}, operationalOutcome("definition-invalid", err)
	}
	if definition.ID != identity.ExperimentID || definition.Spike != identity.Spike {
		return preparedMutation{}, operationalOutcome("definition-identity-mismatch", fmt.Errorf("definition identity %s/%s does not match operation %s/%s", definition.Spike, definition.ID, identity.Spike, identity.ExperimentID))
	}
	locked, err = experiment.Locked(definition)
	if err != nil {
		return preparedMutation{}, operationalOutcome("definition-invalid", err)
	}
	if locked || definition.Lock != nil {
		return preparedMutation{}, verdictOutcome("definition-locked", "authoring operations cannot install a registration lock")
	}
	prospective := cloneFileMap(files)
	prospective[path.Join(experimentPath, "experiment.yaml")] = append([]byte(nil), definitionBytes...)
	for name, data := range replacements {
		prospective[name] = append([]byte(nil), data...)
	}
	candidatePaths, err := experiment.ValidateCandidatePatchesFromSource(newSnapshotFS(prospective), experimentPath, definition)
	if err != nil {
		if candidateErrorsVerdict {
			return preparedMutation{}, verdictOutcome("candidate-refused", err.Error())
		}
		return preparedMutation{}, operationalOutcome("candidate-invalid", err)
	}
	policyDigest, outcome := s.authorizeMutation(ctx, identity, experimentPath, definition, candidatePaths)
	if outcome.Classification != ClassificationClean {
		return preparedMutation{}, outcome
	}
	return preparedMutation{revision: revision, experimentPath: experimentPath, files: files, policyDigest: policyDigest}, cleanOutcome()
}

func (s *Service) authorizeMutation(ctx context.Context, identity Identity, experimentPath string, definition experiment.Definition, candidatePaths []string) (string, Outcome) {
	discovery, err := s.capabilities.DiscoverCapabilities(ctx, CapabilityRequest{CheckoutRoot: identity.CheckoutRoot, Definition: cloneDefinition(definition)})
	if err != nil {
		return "", operationalOutcome("capability-discovery-failed", err)
	}
	capabilities, err := experiment.DecodeCapabilities(discovery.Bytes)
	if err != nil {
		return "", operationalOutcome("capabilities-invalid", err)
	}
	if rawDigest(discovery.Bytes) != definition.Evaluator.CapabilitiesDigest {
		return "", operationalOutcome("capabilities-digest-mismatch", fmt.Errorf("discovered capabilities do not match definition"))
	}
	decision, err := s.policy.ResolvePolicy(ctx, clonePolicyRequest(PolicyRequest{
		CheckoutRoot: identity.CheckoutRoot, ExperimentPath: experimentPath, Spike: identity.Spike,
		Definition: definition, Capabilities: capabilities, CandidatePaths: candidatePaths,
	}))
	if err != nil {
		return "", policyResolutionErrorOutcome(err)
	}
	if decision == nil {
		return "", operationalOutcome("policy-resolution-invalid", fmt.Errorf("experimentapp: policy resolver returned nil decision"))
	}
	if _, err := experimentpolicy.Authorize(decision, experimentpolicy.AuthorizationInput{
		Definition: definition, Capabilities: capabilities, ExperimentPath: experimentPath, CandidatePaths: candidatePaths,
	}); err != nil {
		return "", verdictOutcome("policy-refused", err.Error())
	}
	policyDigest, err := decision.EffectivePolicyDigest()
	if err != nil {
		return "", operationalOutcome("policy-resolution-invalid", err)
	}
	return policyDigest, cleanOutcome()
}

func (s *Service) commitDraftMutation(ctx context.Context, identity Identity, prepared preparedMutation, operation experiment.MutationOperation, replacements map[string][]byte, coordinator draftmutation.Coordinator) DraftMutationResult {
	previousDigest, err := artifactSetDigest(prepared.files, prepared.experimentPath)
	if err != nil {
		return DraftMutationResult{Outcome: operationalOutcome("artifact-digest-invalid", err)}
	}
	prospective := cloneFileMap(prepared.files)
	paths := make([]string, 0, len(replacements))
	files := make([]proposalFile, 0, len(replacements))
	for name, data := range replacements {
		old, exists := prepared.files[name]
		files = append(files, proposalFile{path: name, old: old, oldExists: exists, new: append([]byte(nil), data...)})
		prospective[name] = append([]byte(nil), data...)
		paths = append(paths, name)
	}
	sort.Strings(paths)
	resultDigest, err := artifactSetDigest(prospective, prepared.experimentPath)
	if err != nil {
		return DraftMutationResult{Outcome: operationalOutcome("artifact-digest-invalid", err)}
	}
	if resultDigest == previousDigest {
		return DraftMutationResult{Outcome: verdictOutcome("draft-unchanged", "typed authoring mutation did not change the artifact set")}
	}
	record := experiment.ProvenanceRecord{
		Schema:     experiment.ProvenanceSchema,
		Experiment: experiment.ProvenanceExperiment{Spike: identity.Spike, ID: identity.ExperimentID},
		Operation:  operation, PreviousDigest: previousDigest, ResultDigest: resultDigest,
		PolicyDigest:   prepared.policyDigest,
		PolicyDecision: experiment.ProvenancePolicyDecision{State: experiment.PolicyAllowed, Reasons: []experiment.ProvenancePolicyReason{}},
		Attribution:    identity.Actor.Attribution(), Harness: identity.Actor.Harness(), Session: identity.Actor.Session(), Paths: paths,
	}
	provenance, provenanceFile, err := appendProvenance(prepared.files, prepared.experimentPath, record)
	if err != nil {
		return DraftMutationResult{Outcome: verdictOutcome("direct-draft-unreconciled", err.Error())}
	}
	if err := writeProposal(ctx, identity.CheckoutRoot, coordinator, files, provenanceFile, nil); err != nil {
		return DraftMutationResult{Outcome: operationalOutcome("proposal-write-failed", err)}
	}
	return DraftMutationResult{
		Outcome: cleanOutcome(), AcceptedHead: prepared.revision.Head, ExperimentPath: prepared.experimentPath,
		ArtifactDigest: resultDigest, ProvenanceDigest: provenance.Digest,
	}
}

func appendProvenance(files map[string][]byte, experimentPath string, record experiment.ProvenanceRecord) (experiment.ProvenanceRecord, proposalFile, error) {
	provenancePath := path.Join(experimentPath, experiment.ProvenanceFile)
	old, exists := files[provenancePath]
	records, err := experiment.DecodeProvenanceLog(old)
	if err != nil {
		return experiment.ProvenanceRecord{}, proposalFile{}, err
	}
	if len(records) > 0 && records[len(records)-1].ResultDigest != record.PreviousDigest {
		return experiment.ProvenanceRecord{}, proposalFile{}, fmt.Errorf("mutation provenance does not reach the current artifact digest")
	}
	if err := record.Seal(); err != nil {
		return experiment.ProvenanceRecord{}, proposalFile{}, err
	}
	encoded, err := experiment.EncodeProvenanceRecord(record)
	if err != nil {
		return experiment.ProvenanceRecord{}, proposalFile{}, err
	}
	combined := append(append([]byte(nil), old...), encoded...)
	if _, err := experiment.DecodeProvenanceLog(combined); err != nil {
		return experiment.ProvenanceRecord{}, proposalFile{}, err
	}
	return record, proposalFile{path: provenancePath, old: old, oldExists: exists, new: combined}, nil
}

func cloneFileMap(files map[string][]byte) map[string][]byte {
	cloned := make(map[string][]byte, len(files))
	for name, data := range files {
		cloned[name] = append([]byte(nil), data...)
	}
	return cloned
}
