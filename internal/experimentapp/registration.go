package experimentapp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"sort"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// RegistrationInput binds the human mutation to the exact read-only packet.
type RegistrationInput struct {
	ReviewPacketDigest string
}

// RegistrationResult distinguishes a local proposal from accepted authority.
type RegistrationResult struct {
	Outcome          Outcome
	AcceptedHead     string
	ExperimentPath   string
	DefinitionDigest string
	ArtifactDigest   string
	ProvenanceDigest string
	Accepted         bool
}

// ReconcileDraft appends one explicit unauthenticated direct-edit admission.
// It requires a sealed human actor but intentionally records no inferred
// principal, editor, session, or intermediate history.
func (s *Service) ReconcileDraft(ctx context.Context, identity Identity) DraftMutationResult {
	if err := identity.validate(); err != nil {
		return DraftMutationResult{Outcome: operationalOutcome("invalid-request", err)}
	}
	if identity.Actor.Kind() != ActorAuthenticatedHuman {
		return DraftMutationResult{Outcome: verdictOutcome("human-actor-required", "direct-edit reconciliation is a local-human operation")}
	}
	accepted, err := resolveAcceptedBase(ctx, s.git, identity)
	if err != nil {
		var stale *staleAcceptedHeadError
		if errors.As(err, &stale) {
			return DraftMutationResult{Outcome: verdictOutcome("accepted-head-stale", stale.Error())}
		}
		return DraftMutationResult{Outcome: operationalOutcome("accepted-tree-invalid", err)}
	}
	validation := s.validateDraftAtRevision(ctx, identity, accepted.revision)
	if validation.Outcome.Classification != ClassificationClean {
		return DraftMutationResult{Outcome: validation.Outcome}
	}
	proposed, err := readProposedArtifactFiles(identity.CheckoutRoot, validation.ExperimentPath)
	if err != nil {
		return DraftMutationResult{Outcome: operationalOutcome("proposal-tree-invalid", err)}
	}
	acceptedPath := accepted.experimentPath
	if acceptedPath == "" {
		acceptedPath = validation.ExperimentPath
	}
	acceptedDigest, err := artifactSetDigest(accepted.source.files, acceptedPath)
	if err != nil {
		return DraftMutationResult{Outcome: operationalOutcome("artifact-digest-invalid", err)}
	}
	proposedDigest, err := artifactSetDigest(proposed, validation.ExperimentPath)
	if err != nil {
		return DraftMutationResult{Outcome: operationalOutcome("artifact-digest-invalid", err)}
	}
	if acceptedDigest == proposedDigest {
		return DraftMutationResult{Outcome: verdictOutcome("direct-draft-absent", "proposed artifact set does not differ from accepted bytes")}
	}
	wantIdentity := experiment.ProvenanceExperiment{Spike: identity.Spike, ID: identity.ExperimentID}
	history, err := compareProvenanceHistories(
		accepted.source.files, acceptedPath,
		proposed, validation.ExperimentPath,
		wantIdentity, acceptedDigest,
	)
	if err != nil {
		return DraftMutationResult{Outcome: operationalOutcome("mutation-provenance-invalid", err)}
	}
	if history.mismatch != "" {
		return DraftMutationResult{Outcome: verdictOutcome("direct-draft-unreconciled", history.mismatch)}
	}
	if history.reaches(proposedDigest) {
		return DraftMutationResult{Outcome: verdictOutcome("direct-draft-absent", "proposed human-authored artifact bytes are already reconciled")}
	}
	previousDigest := acceptedDigest
	if len(history.proposed) > 0 {
		previousDigest = history.proposed[len(history.proposed)-1].ResultDigest
	}
	paths := changedArtifactPaths(accepted.source.files, acceptedPath, proposed, validation.ExperimentPath)
	if len(paths) == 0 {
		return DraftMutationResult{Outcome: verdictOutcome("direct-draft-absent", "no changed proposal artifact paths were found")}
	}
	record := experiment.ProvenanceRecord{
		Schema:     experiment.ProvenanceSchema,
		Experiment: experiment.ProvenanceExperiment{Spike: identity.Spike, ID: identity.ExperimentID},
		Operation:  experiment.MutationReconcileDirect, PreviousDigest: previousDigest, ResultDigest: proposedDigest,
		PolicyDigest:   validation.PolicyDigest,
		PolicyDecision: experiment.ProvenancePolicyDecision{State: experiment.PolicyAllowed, Reasons: []experiment.ProvenancePolicyReason{}},
		Attribution:    governanceprincipal.NewUnauthenticatedAttribution(), Paths: paths,
	}
	sealed, provenanceFile, err := appendProvenance(proposed, validation.ExperimentPath, record)
	if err != nil {
		return DraftMutationResult{Outcome: operationalOutcome("mutation-provenance-invalid", err)}
	}
	if err := writeProposal(ctx, identity.CheckoutRoot, draftmutation.Coordinator{}, nil, provenanceFile, nil); err != nil {
		return DraftMutationResult{Outcome: operationalOutcome("proposal-write-failed", err)}
	}
	return DraftMutationResult{Outcome: cleanOutcome(), AcceptedHead: accepted.revision.Head, ExperimentPath: validation.ExperimentPath, ArtifactDigest: proposedDigest, ProvenanceDigest: sealed.Digest}
}

// ProposeRegistration writes a digest-matching lock only after a sealed human
// actor presents the exact deterministic packet digest.
func (s *Service) ProposeRegistration(ctx context.Context, identity Identity, input RegistrationInput) RegistrationResult {
	return s.proposeRegistration(ctx, identity, input, draftmutation.Coordinator{})
}

func (s *Service) proposeRegistration(ctx context.Context, identity Identity, input RegistrationInput, coordinator draftmutation.Coordinator) RegistrationResult {
	if err := identity.validate(); err != nil {
		return RegistrationResult{Outcome: operationalOutcome("invalid-request", err)}
	}
	if identity.Actor.Kind() != ActorAuthenticatedHuman {
		return RegistrationResult{Outcome: verdictOutcome("human-actor-required", "registration lock requires an authenticated human")}
	}
	if err := experiment.ValidateDigest(input.ReviewPacketDigest); err != nil {
		return RegistrationResult{Outcome: operationalOutcome("review-packet-invalid", err)}
	}
	review := s.ReviewRegistration(ctx, identity)
	if review.Outcome.Classification != ClassificationClean {
		return RegistrationResult{Outcome: review.Outcome}
	}
	if input.ReviewPacketDigest != review.PacketDigest {
		return RegistrationResult{Outcome: verdictOutcome("review-packet-mismatch", "registration input does not name the exact review packet")}
	}
	definitionPath := path.Join(review.Packet.ExperimentPath, "experiment.yaml")
	proposed, err := readProposedArtifactFiles(identity.CheckoutRoot, review.Packet.ExperimentPath)
	if err != nil {
		return RegistrationResult{Outcome: operationalOutcome("proposal-tree-invalid", err)}
	}
	definitionBytes := proposed[definitionPath]
	definition, err := experiment.DecodeDefinition(definitionBytes)
	if err != nil {
		return RegistrationResult{Outcome: operationalOutcome("definition-invalid", err)}
	}
	locked, err := experiment.Locked(definition)
	if err != nil {
		return RegistrationResult{Outcome: operationalOutcome("definition-invalid", err)}
	}
	if locked || definition.Lock != nil {
		return RegistrationResult{Outcome: verdictOutcome("definition-locked", "experiment definition is already locked")}
	}
	lockedBytes := appendRegistrationLock(definitionBytes, review.Packet.DefinitionDigest)
	lockedDefinition, err := experiment.DecodeDefinition(lockedBytes)
	if err != nil {
		return RegistrationResult{Outcome: operationalOutcome("definition-invalid", err)}
	}
	locked, err = experiment.Locked(lockedDefinition)
	if err != nil || !locked {
		return RegistrationResult{Outcome: operationalOutcome("definition-invalid", fmt.Errorf("rendered registration lock is invalid: locked=%v: %w", locked, err))}
	}
	capabilitiesPath := path.Join(review.Packet.ExperimentPath, "evaluator-capabilities.json")
	capabilitiesBytes, err := canonjson.Marshal(review.Packet.Capabilities)
	if err != nil {
		return RegistrationResult{Outcome: operationalOutcome("capabilities-invalid", err)}
	}
	if rawDigest(capabilitiesBytes) != review.Packet.CapabilitiesDigest {
		return RegistrationResult{Outcome: operationalOutcome("capabilities-digest-mismatch", fmt.Errorf("canonical review capabilities do not match the registered digest"))}
	}
	resultFiles := cloneFileMap(proposed)
	resultFiles[definitionPath] = lockedBytes
	resultFiles[capabilitiesPath] = capabilitiesBytes
	resultDigest, err := artifactSetDigest(resultFiles, review.Packet.ExperimentPath)
	if err != nil {
		return RegistrationResult{Outcome: operationalOutcome("artifact-digest-invalid", err)}
	}
	record := experiment.ProvenanceRecord{
		Schema: experiment.ProvenanceSchema, Experiment: review.Packet.Experiment,
		Operation:      experiment.MutationProposeRegistration,
		PreviousDigest: review.ProposedArtifactDigest, ResultDigest: resultDigest,
		PolicyDigest:   review.Packet.PolicyDigest,
		PolicyDecision: experiment.ProvenancePolicyDecision{State: experiment.PolicyAllowed, Reasons: []experiment.ProvenancePolicyReason{}},
		Attribution:    identity.Actor.Attribution(), Paths: []string{capabilitiesPath, definitionPath},
	}
	sealed, provenanceFile, err := appendProvenance(proposed, review.Packet.ExperimentPath, record)
	if err != nil {
		return RegistrationResult{Outcome: verdictOutcome("direct-draft-unreconciled", err.Error())}
	}
	oldCapabilities, capabilitiesExist := proposed[capabilitiesPath]
	guard := &proposalArtifactSetGuard{experimentPath: review.Packet.ExperimentPath, digest: review.ProposedArtifactDigest}
	if err := writeProposal(ctx, identity.CheckoutRoot, coordinator, []proposalFile{
		{path: capabilitiesPath, old: oldCapabilities, oldExists: capabilitiesExist, new: capabilitiesBytes},
		{path: definitionPath, old: definitionBytes, oldExists: true, new: lockedBytes},
	}, provenanceFile, guard); err != nil {
		if errors.Is(err, errProposalArtifactSetChanged) {
			return RegistrationResult{Outcome: verdictOutcome("review-packet-mismatch", "proposal artifact set changed after registration review")}
		}
		return RegistrationResult{Outcome: operationalOutcome("proposal-write-failed", err)}
	}
	return RegistrationResult{
		Outcome: cleanOutcome(), AcceptedHead: review.Packet.AcceptedHead, ExperimentPath: review.Packet.ExperimentPath,
		DefinitionDigest: review.Packet.DefinitionDigest, ArtifactDigest: resultDigest, ProvenanceDigest: sealed.Digest,
	}
}

// AcceptedRegistration proves the exact definition/provenance pair from one
// default-branch tree. It never reads proposal bytes from the worktree.
func (s *Service) AcceptedRegistration(ctx context.Context, identity Identity) RegistrationResult {
	if err := identity.validate(); err != nil {
		return RegistrationResult{Outcome: operationalOutcome("invalid-request", err)}
	}
	snapshot, err := resolveAccepted(ctx, s.git, identity)
	if err != nil {
		var stale *staleAcceptedHeadError
		if errors.As(err, &stale) {
			return RegistrationResult{Outcome: verdictOutcome("accepted-head-stale", stale.Error())}
		}
		return RegistrationResult{Outcome: operationalOutcome("accepted-tree-invalid", err)}
	}
	definitionBytes, err := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, "experiment.yaml"))
	if err != nil {
		return RegistrationResult{Outcome: operationalOutcome("definition-unreadable", err)}
	}
	definition, err := experiment.DecodeDefinition(definitionBytes)
	if err != nil {
		return RegistrationResult{Outcome: operationalOutcome("definition-invalid", err)}
	}
	locked, err := experiment.Locked(definition)
	if err != nil {
		return RegistrationResult{Outcome: operationalOutcome("definition-invalid", err)}
	}
	if !locked {
		return RegistrationResult{Outcome: verdictOutcome("registration-not-accepted", "accepted definition is not locked")}
	}
	if definition.Schema != experiment.DefinitionSchemaV2 {
		return RegistrationResult{Outcome: verdictOutcome("registration-not-accepted", "only definition v2 can carry a Wave 5 registration")}
	}
	_, err = experiment.ValidateCandidatePatchesFromSource(snapshot.source, snapshot.experimentPath, definition)
	if err != nil {
		return RegistrationResult{Outcome: operationalOutcome("candidate-invalid", err)}
	}
	capabilitiesBytes, err := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, "evaluator-capabilities.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return RegistrationResult{Outcome: verdictOutcome("registration-not-accepted", "accepted definition has no evaluator capabilities")}
	}
	if err != nil {
		return RegistrationResult{Outcome: operationalOutcome("capabilities-unreadable", err)}
	}
	capabilities, err := experiment.DecodeCapabilities(capabilitiesBytes)
	if err != nil {
		return RegistrationResult{Outcome: operationalOutcome("capabilities-invalid", err)}
	}
	if rawDigest(capabilitiesBytes) != definition.Evaluator.CapabilitiesDigest {
		return RegistrationResult{Outcome: operationalOutcome("capabilities-digest-mismatch", fmt.Errorf("accepted capabilities do not match definition"))}
	}
	if err := experiment.ValidateDefinitionCapabilities(definition, capabilities); err != nil {
		return RegistrationResult{Outcome: operationalOutcome("capabilities-invalid", err)}
	}
	provenanceBytes, err := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, experiment.ProvenanceFile))
	if errors.Is(err, fs.ErrNotExist) {
		return RegistrationResult{Outcome: verdictOutcome("registration-not-accepted", "accepted definition has no mutation provenance")}
	}
	if err != nil {
		return RegistrationResult{Outcome: operationalOutcome("mutation-provenance-invalid", err)}
	}
	records, err := experiment.DecodeProvenanceLog(provenanceBytes)
	if err != nil {
		return RegistrationResult{Outcome: operationalOutcome("mutation-provenance-invalid", err)}
	}
	artifactDigest, err := artifactSetDigest(snapshot.source.files, snapshot.experimentPath)
	if err != nil {
		return RegistrationResult{Outcome: operationalOutcome("artifact-digest-invalid", err)}
	}
	if len(records) == 0 {
		return RegistrationResult{Outcome: verdictOutcome("registration-not-accepted", "accepted provenance is empty")}
	}
	last := records[len(records)-1]
	wantIdentity := experiment.ProvenanceExperiment{Spike: identity.Spike, ID: identity.ExperimentID}
	wantPaths := []string{path.Join(snapshot.experimentPath, "evaluator-capabilities.json"), path.Join(snapshot.experimentPath, "experiment.yaml")}
	if last.Operation != experiment.MutationProposeRegistration || last.Experiment != wantIdentity || last.ResultDigest != artifactDigest || last.Attribution.Unauthenticated || !slices.Equal(last.Paths, wantPaths) {
		return RegistrationResult{Outcome: verdictOutcome("registration-not-accepted", "accepted lock and provenance do not form one complete registration pair")}
	}
	definitionDigest, err := experiment.DefinitionDigest(definition)
	if err != nil {
		return RegistrationResult{Outcome: operationalOutcome("definition-invalid", err)}
	}
	return RegistrationResult{
		Outcome: cleanOutcome(), AcceptedHead: snapshot.revision.Head, ExperimentPath: snapshot.experimentPath,
		DefinitionDigest: definitionDigest, ArtifactDigest: artifactDigest, ProvenanceDigest: last.Digest, Accepted: true,
	}
}

func appendRegistrationLock(definitionBytes []byte, digest string) []byte {
	out := append([]byte(nil), definitionBytes...)
	if len(out) > 0 && !bytes.HasSuffix(out, []byte("\n")) {
		out = append(out, '\n')
	}
	return append(out, []byte("lock:\n  definition_digest: "+digest+"\n")...)
}

func changedArtifactPaths(accepted map[string][]byte, acceptedPath string, proposed map[string][]byte, proposedPath string) []string {
	acceptedByRelative := relativeArtifactFiles(accepted, acceptedPath)
	proposedByRelative := relativeArtifactFiles(proposed, proposedPath)
	changed := make([]string, 0)
	seen := map[string]bool{}
	for relative, data := range acceptedByRelative {
		seen[relative] = true
		proposedData, present := proposedByRelative[relative]
		if !present || !bytes.Equal(data, proposedData) {
			changed = append(changed, path.Join(proposedPath, relative))
		}
	}
	for relative, data := range proposedByRelative {
		if seen[relative] {
			continue
		}
		acceptedData, present := acceptedByRelative[relative]
		if !present || !bytes.Equal(data, acceptedData) {
			changed = append(changed, path.Join(proposedPath, relative))
		}
	}
	sort.Strings(changed)
	return changed
}

func relativeArtifactFiles(files map[string][]byte, experimentPath string) map[string][]byte {
	prefix := experimentPath + "/"
	relative := map[string][]byte{}
	for name, data := range files {
		if len(name) <= len(prefix) || name[:len(prefix)] != prefix {
			continue
		}
		relativePath := name[len(prefix):]
		if !isMutationArtifact(relativePath) {
			continue
		}
		relative[relativePath] = data
	}
	return relative
}
