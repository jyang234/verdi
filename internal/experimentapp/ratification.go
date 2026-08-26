// Authenticated ratification and accepted-state resolution (Wave 5C Task
// 8; design §§3, 7, 9–10; SI-143, SI-146). A ratification proposal is
// constructed only from an authentic, sealed authenticated
// governance-principal resolution: the record's v2 actor block copies that
// resolution's exact claim and kernel-derived principal id, never a
// caller-supplied actor field. Proposal bytes carry no authority until
// they are present at the accepted default-branch HEAD; accepted use
// re-resolves the persisted claim through the configured accepted profile
// and trust-fact reader and requires a fresh sealed authenticated
// resolution with the exact persisted principal id.
package experimentapp

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"

	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

const ratificationFileName = "ratification.yaml"

// RatificationProposalInput is the typed human ratification request. It
// deliberately carries no actor field: the only identity operand is the
// sealed kernel resolution itself.
type RatificationProposalInput struct {
	ResultDigest string
	Disposition  experiment.Disposition
	Candidate    string
	Reason       string
	Resolution   governanceprincipal.PrincipalResolution
}

// RatificationProposalResult is the typed proposal outcome.
type RatificationProposalResult struct {
	Outcome          Outcome
	AcceptedHead     string
	ExperimentPath   string
	ResultDigest     string
	PrincipalID      governanceprincipal.PrincipalID
	ArtifactDigest   string
	ProvenanceDigest string
}

// AcceptedRatificationAuthority is the configured accepted governance
// authority the caller supplies for accepted-state resolution: the sealed
// accepted governance profile and the trust-fact reader. The kernel — not
// the caller and not this package — derives every principal id.
type AcceptedRatificationAuthority struct {
	Profile governanceprincipal.Profile
	Facts   governanceprincipal.TrustFactReader
}

// AcceptedRatificationResult is the typed accepted-state outcome.
type AcceptedRatificationResult struct {
	Outcome        Outcome
	AcceptedHead   string
	ExperimentPath string
	Ratification   experiment.Ratification
	PrincipalID    governanceprincipal.PrincipalID
}

// ProposeRatification writes the exact deterministic v2 ratification
// proposal for one accepted result, with authority drawn only from the
// sealed authenticated resolution.
func (s *Service) ProposeRatification(ctx context.Context, identity Identity, input RatificationProposalInput) RatificationProposalResult {
	if ctx == nil {
		return RatificationProposalResult{Outcome: operationalOutcome("invalid-request", fmt.Errorf("experimentapp: ratification context is nil"))}
	}
	if err := identity.validate(); err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("invalid-request", err)}
	}
	if identity.Actor.Kind() != ActorAuthenticatedHuman {
		return RatificationProposalResult{Outcome: verdictOutcome("human-actor-required", "ratification is a human-only operation")}
	}
	attribution, err := governanceprincipal.AttributionFromResolution(input.Resolution)
	if err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("ratification-resolution-invalid", err)}
	}
	if attribution.Unauthenticated {
		return RatificationProposalResult{Outcome: verdictOutcome("ratification-actor-unauthenticated", fmt.Sprintf("principal resolution state is %q", input.Resolution.State))}
	}
	if identity.Actor.Attribution().PrincipalID != input.Resolution.PrincipalID {
		return RatificationProposalResult{Outcome: operationalOutcome("ratification-actor-mismatch", fmt.Errorf("experimentapp: identity actor principal %q does not match the sealed resolution principal %q", identity.Actor.Attribution().PrincipalID, input.Resolution.PrincipalID))}
	}
	if err := experiment.ValidateDigest(input.ResultDigest); err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("invalid-request", fmt.Errorf("experimentapp: ratification result digest: %w", err))}
	}

	snapshot, err := resolveAccepted(ctx, s.git, identity)
	if err != nil {
		var stale *staleAcceptedHeadError
		if errors.As(err, &stale) {
			return RatificationProposalResult{Outcome: verdictOutcome("accepted-head-stale", stale.Error())}
		}
		return RatificationProposalResult{Outcome: operationalOutcome("accepted-tree-invalid", err)}
	}
	if _, err := fs.Stat(snapshot.source, path.Join(snapshot.experimentPath, ratificationFileName)); err == nil {
		return RatificationProposalResult{Outcome: verdictOutcome("ratification-already-accepted", "the accepted tree already carries a ratification for this experiment")}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return RatificationProposalResult{Outcome: operationalOutcome("accepted-tree-invalid", err)}
	}
	// The registration pair is judged against the SAME resolved snapshot:
	// the accepted HEAD and tree are resolved exactly once (design §7).
	registration := s.acceptedRegistrationAt(identity, snapshot)
	if registration.Outcome.Classification != ClassificationClean {
		return RatificationProposalResult{Outcome: registration.Outcome}
	}
	definitionBytes, err := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, "experiment.yaml"))
	if err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("definition-unreadable", err)}
	}
	definition, err := experiment.DecodeDefinition(definitionBytes)
	if err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("definition-invalid", err)}
	}
	candidatePaths, err := experiment.ValidateCandidatePatchesFromSource(snapshot.source, snapshot.experimentPath, definition)
	if err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("candidate-invalid", err)}
	}
	derived, err := experiment.DeriveStateDetailsFromSource(snapshot.source, snapshot.experimentPath, s.results.VerifyResult)
	if err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("state-invalid", err)}
	}
	if _, err := os.Lstat(filepath.Join(identity.CheckoutRoot, filepath.FromSlash(snapshot.experimentPath), ratificationFileName)); err == nil {
		return RatificationProposalResult{Outcome: verdictOutcome("ratification-already-proposed", "the worktree already carries a proposed ratification")}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return RatificationProposalResult{Outcome: operationalOutcome("proposal-tree-invalid", err)}
	}

	// Bind the exact accepted result: exactly one accepted run may carry
	// the named digest. The state derivation above already refused
	// duplicated result identities inside the accepted tree.
	matchedRun := ""
	for _, run := range derived.Runs {
		if run.ResultDigest == input.ResultDigest {
			if matchedRun != "" {
				return RatificationProposalResult{Outcome: operationalOutcome("ratification-result-ambiguous", fmt.Errorf("experimentapp: result digest %q matches more than one accepted run", input.ResultDigest))}
			}
			matchedRun = run.Run
		}
	}
	if matchedRun == "" {
		return RatificationProposalResult{Outcome: verdictOutcome("ratification-result-unknown", fmt.Sprintf("result digest %q names no accepted run result", input.ResultDigest))}
	}
	resultRaw, err := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, "runs", matchedRun, "result.json"))
	if err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("result-unreadable", err)}
	}
	result, err := experiment.DecodeResult(resultRaw)
	if err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("result-invalid", err)}
	}
	resultDigest, err := experiment.ResultDigest(result)
	if err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("result-invalid", err)}
	}
	if resultDigest != input.ResultDigest {
		return RatificationProposalResult{Outcome: operationalOutcome("result-invalid", fmt.Errorf("experimentapp: accepted run %q result digest %q does not match requested %q", matchedRun, resultDigest, input.ResultDigest))}
	}
	if result.Schema != experiment.ResultSchemaV2 {
		return RatificationProposalResult{Outcome: verdictOutcome("ratification-evidence-v1", "the selected result is decode-only v1 evidence without an execution receipt and cannot become fresh ratification authority")}
	}

	record := experiment.Ratification{
		Schema:       experiment.RatificationSchemaV2,
		ResultDigest: input.ResultDigest,
		ActorV2: &experiment.RatificationActor{
			TrustSource: input.Resolution.Claim.TrustSource,
			Subject:     input.Resolution.Claim.Subject,
			PrincipalID: string(input.Resolution.PrincipalID),
		},
		Disposition: input.Disposition, Candidate: input.Candidate, Reason: input.Reason,
	}
	if err := record.Validate(); err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("ratification-record-invalid", err)}
	}
	if err := experiment.ValidateRatificationBinding(definition, result, record); err != nil {
		return RatificationProposalResult{Outcome: verdictOutcome("ratification-binding-violated", err.Error())}
	}
	encoded, err := experiment.EncodeRatification(record)
	if err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("ratification-record-invalid", err)}
	}

	policyDigest, outcome := s.authorizeMutation(ctx, identity, snapshot.experimentPath, definition, candidatePaths)
	if outcome.Classification != ClassificationClean {
		return RatificationProposalResult{Outcome: outcome}
	}
	proposed, err := readProposedArtifactFiles(identity.CheckoutRoot, snapshot.experimentPath)
	if err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("proposal-tree-invalid", err)}
	}
	proposedDigest, err := artifactSetDigest(proposed, snapshot.experimentPath)
	if err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("artifact-digest-invalid", err)}
	}
	if proposedDigest != registration.ArtifactDigest {
		return RatificationProposalResult{Outcome: operationalOutcome("locked-input-mismatch", fmt.Errorf("experimentapp: worktree mutation-artifact digest %s does not match accepted registration %s", proposedDigest, registration.ArtifactDigest))}
	}
	ratificationPath := path.Join(snapshot.experimentPath, ratificationFileName)
	resultFiles := cloneFileMap(proposed)
	resultFiles[ratificationPath] = encoded
	resultSetDigest, err := artifactSetDigest(resultFiles, snapshot.experimentPath)
	if err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("artifact-digest-invalid", err)}
	}
	provenanceRecord := experiment.ProvenanceRecord{
		Schema: experiment.ProvenanceSchema, Experiment: experiment.ProvenanceExperiment{Spike: identity.Spike, ID: identity.ExperimentID},
		Operation: experiment.MutationProposeRatification, PreviousDigest: proposedDigest, ResultDigest: resultSetDigest,
		PolicyDigest:   policyDigest,
		PolicyDecision: experiment.ProvenancePolicyDecision{State: experiment.PolicyAllowed, Reasons: []experiment.ProvenancePolicyReason{}},
		Attribution:    identity.Actor.Attribution(), Paths: []string{ratificationPath},
	}
	sealed, provenanceFile, err := appendProvenance(proposed, snapshot.experimentPath, provenanceRecord)
	if err != nil {
		return RatificationProposalResult{Outcome: verdictOutcome("direct-draft-unreconciled", err.Error())}
	}
	guard := &proposalArtifactSetGuard{experimentPath: snapshot.experimentPath, digest: proposedDigest}
	if err := writeProposal(ctx, identity.CheckoutRoot, draftmutation.Coordinator{}, []proposalFile{
		{path: ratificationPath, oldExists: false, new: encoded},
	}, provenanceFile, guard); err != nil {
		if errors.Is(err, errProposalArtifactSetChanged) {
			return RatificationProposalResult{Outcome: verdictOutcome("ratification-proposal-conflict", "proposal artifact set changed while the ratification was being written")}
		}
		return RatificationProposalResult{Outcome: operationalOutcome("proposal-write-failed", err)}
	}
	return RatificationProposalResult{
		Outcome: cleanOutcome(), AcceptedHead: registration.AcceptedHead, ExperimentPath: snapshot.experimentPath,
		ResultDigest: input.ResultDigest, PrincipalID: input.Resolution.PrincipalID,
		ArtifactDigest: resultSetDigest, ProvenanceDigest: sealed.Digest,
	}
}

// AcceptedRatification resolves the exact accepted-tree ratification and
// re-authenticates its persisted claim through the configured accepted
// governance authority. It is read-only, exact-tree, stale-HEAD-safe, and
// independent of divergent worktree bytes.
func (s *Service) AcceptedRatification(ctx context.Context, identity Identity, authority AcceptedRatificationAuthority) AcceptedRatificationResult {
	if ctx == nil {
		return AcceptedRatificationResult{Outcome: operationalOutcome("invalid-request", fmt.Errorf("experimentapp: ratification context is nil"))}
	}
	if err := identity.validate(); err != nil {
		return AcceptedRatificationResult{Outcome: operationalOutcome("invalid-request", err)}
	}
	if authority.Facts == nil {
		return AcceptedRatificationResult{Outcome: operationalOutcome("ratification-authority-invalid", fmt.Errorf("experimentapp: accepted ratification requires a configured trust-fact reader"))}
	}
	snapshot, err := resolveAccepted(ctx, s.git, identity)
	if err != nil {
		var stale *staleAcceptedHeadError
		if errors.As(err, &stale) {
			return AcceptedRatificationResult{Outcome: verdictOutcome("accepted-head-stale", stale.Error())}
		}
		return AcceptedRatificationResult{Outcome: operationalOutcome("accepted-tree-invalid", err)}
	}
	facts, outcome := s.acceptedRatificationAt(ctx, identity, authority, snapshot)
	if outcome.Classification != ClassificationClean {
		return AcceptedRatificationResult{Outcome: outcome}
	}
	return AcceptedRatificationResult{
		Outcome: cleanOutcome(), AcceptedHead: snapshot.revision.Head,
		ExperimentPath: snapshot.experimentPath, Ratification: facts.record.Clone(),
		PrincipalID: facts.principal,
	}
}

// acceptedRatificationFacts is the complete accepted-ratification proof a
// snapshot-holding lifecycle operation consumes without resolving the
// accepted HEAD or tree a second time.
type acceptedRatificationFacts struct {
	record     experiment.Ratification
	definition experiment.Definition
	derived    experiment.StateDerivation
	run        string
	result     experiment.Result
	principal  governanceprincipal.PrincipalID
}

// acceptedRatificationAt judges the full accepted-ratification proof
// against one already-resolved accepted snapshot (behavior-identical
// extraction of AcceptedRatification's body; the public result shape and
// classification are pinned by the existing tests).
func (s *Service) acceptedRatificationAt(ctx context.Context, identity Identity, authority AcceptedRatificationAuthority, snapshot acceptedSnapshot) (acceptedRatificationFacts, Outcome) {
	// Ratification presupposes the registration lock: the accepted
	// definition must be the locked v2 record. The registration
	// provenance-pair check is deliberately NOT reused here — it pins the
	// LAST provenance record to the lock, which is true only until the
	// ratification mutation lands; design §7's ratification proof is the
	// exact-tree artifact resolution below plus claim re-resolution.
	definitionBytes, err := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, "experiment.yaml"))
	if err != nil {
		return acceptedRatificationFacts{}, operationalOutcome("definition-unreadable", err)
	}
	definition, err := experiment.DecodeDefinition(definitionBytes)
	if err != nil {
		return acceptedRatificationFacts{}, operationalOutcome("definition-invalid", err)
	}
	locked, err := experiment.Locked(definition)
	if err != nil {
		return acceptedRatificationFacts{}, operationalOutcome("definition-invalid", err)
	}
	if !locked || definition.Schema != experiment.DefinitionSchemaV2 {
		return acceptedRatificationFacts{}, verdictOutcome("registration-not-accepted", "accepted definition is not a locked v2 registration")
	}

	raw, err := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, ratificationFileName))
	if errors.Is(err, fs.ErrNotExist) {
		return acceptedRatificationFacts{}, verdictOutcome("ratification-not-accepted", "no ratification is present at the accepted HEAD; proposal bytes carry no authority")
	}
	if err != nil {
		return acceptedRatificationFacts{}, operationalOutcome("ratification-unreadable", err)
	}
	record, err := experiment.DecodeRatification(raw)
	if err != nil {
		return acceptedRatificationFacts{}, operationalOutcome("ratification-invalid", err)
	}
	if record.Schema == experiment.RatificationSchema {
		return acceptedRatificationFacts{}, verdictOutcome("ratification-v1-history", "ratification v1 is decode-only predecessor history and cannot authorize release or closure")
	}

	// One exact-tree pass of the one state algorithm proves the record
	// binds exactly one accepted result and the derived state is ratified;
	// duplicated result identities and binding violations surface here as
	// operational corruption of accepted evidence.
	derived, err := experiment.DeriveStateDetailsFromSource(snapshot.source, snapshot.experimentPath, s.results.VerifyResult)
	if err != nil {
		return acceptedRatificationFacts{}, operationalOutcome("state-invalid", err)
	}
	if derived.State != experiment.StateRatified {
		return acceptedRatificationFacts{}, operationalOutcome("state-invalid", fmt.Errorf("experimentapp: accepted tree carries ratification bytes but derives state %q", derived.State))
	}

	// The bound result must be authoritative v2 evidence with its validated
	// execution receipt: v1 runs remain decode-only state history and can
	// never anchor fresh release or closure authority (design §7, AC-1).
	matchedRun := ""
	for _, run := range derived.Runs {
		if run.ResultDigest == record.ResultDigest {
			matchedRun = run.Run
		}
	}
	if matchedRun == "" {
		return acceptedRatificationFacts{}, operationalOutcome("state-invalid", fmt.Errorf("experimentapp: ratified state names no run for result digest %q", record.ResultDigest))
	}
	resultRaw, err := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, "runs", matchedRun, "result.json"))
	if err != nil {
		return acceptedRatificationFacts{}, operationalOutcome("result-unreadable", err)
	}
	boundResult, err := experiment.DecodeResult(resultRaw)
	if err != nil {
		return acceptedRatificationFacts{}, operationalOutcome("result-invalid", err)
	}
	if boundResult.Schema != experiment.ResultSchemaV2 {
		return acceptedRatificationFacts{}, verdictOutcome("ratification-evidence-v1", "the bound result is decode-only v1 evidence without an execution receipt and cannot carry ratification authority")
	}

	// The accepted resolver proves the complete artifact/provenance pair
	// (design §6): the final provenance record must be the exact
	// authenticated propose-ratification mutation for these bytes.
	provenanceRaw, err := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, experiment.ProvenanceFile))
	if errors.Is(err, fs.ErrNotExist) {
		return acceptedRatificationFacts{}, verdictOutcome("ratification-provenance-incomplete", "accepted ratification bytes carry no mutation-provenance record")
	}
	if err != nil {
		return acceptedRatificationFacts{}, operationalOutcome("mutation-provenance-invalid", err)
	}
	records, err := experiment.DecodeProvenanceLog(provenanceRaw)
	if err != nil {
		return acceptedRatificationFacts{}, operationalOutcome("mutation-provenance-invalid", err)
	}
	if len(records) == 0 {
		return acceptedRatificationFacts{}, verdictOutcome("ratification-provenance-incomplete", "accepted mutation provenance is empty")
	}
	acceptedDigest, err := artifactSetDigest(snapshot.source.files, snapshot.experimentPath)
	if err != nil {
		return acceptedRatificationFacts{}, operationalOutcome("artifact-digest-invalid", err)
	}
	last := records[len(records)-1]
	wantIdentity := experiment.ProvenanceExperiment{Spike: identity.Spike, ID: identity.ExperimentID}
	wantPaths := []string{path.Join(snapshot.experimentPath, ratificationFileName)}
	if last.Operation != experiment.MutationProposeRatification || last.Experiment != wantIdentity ||
		last.ResultDigest != acceptedDigest || !slices.Equal(last.Paths, wantPaths) {
		return acceptedRatificationFacts{}, verdictOutcome("ratification-provenance-incomplete", "accepted ratification and provenance do not form one complete propose-ratification pair")
	}
	// AC-5's two temporally distinct human moments (design §3.3): the
	// ratification tail must extend the exact pre-ratification artifact
	// set, and the immediately preceding record must be the complete
	// accepted propose-registration pair. Registration and ratification
	// principals may legitimately differ, so no cross-record principal
	// equality is imposed.
	preimageFiles := cloneFileMap(snapshot.source.files)
	delete(preimageFiles, path.Join(snapshot.experimentPath, ratificationFileName))
	preimageDigest, err := artifactSetDigest(preimageFiles, snapshot.experimentPath)
	if err != nil {
		return acceptedRatificationFacts{}, operationalOutcome("artifact-digest-invalid", err)
	}
	if last.PreviousDigest != preimageDigest {
		return acceptedRatificationFacts{}, verdictOutcome("ratification-provenance-incomplete", "the accepted propose-ratification record does not extend the exact pre-ratification artifact set")
	}
	if len(records) < 2 {
		return acceptedRatificationFacts{}, verdictOutcome("ratification-provenance-incomplete", "no accepted propose-registration record precedes the ratification")
	}
	predecessor := records[len(records)-2]
	wantRegistrationPaths := []string{path.Join(snapshot.experimentPath, "evaluator-capabilities.json"), path.Join(snapshot.experimentPath, "experiment.yaml")}
	if predecessor.Operation != experiment.MutationProposeRegistration || predecessor.Experiment != wantIdentity ||
		predecessor.ResultDigest != preimageDigest || !slices.Equal(predecessor.Paths, wantRegistrationPaths) {
		return acceptedRatificationFacts{}, verdictOutcome("ratification-provenance-incomplete", "the record preceding the ratification is not the complete accepted propose-registration pair")
	}
	if predecessor.Attribution.Unauthenticated || predecessor.Attribution.PrincipalID == "" {
		return acceptedRatificationFacts{}, operationalOutcome("ratification-provenance-identity", fmt.Errorf("experimentapp: the accepted propose-registration record is not attributed to an authenticated principal"))
	}
	if last.Attribution.Unauthenticated || last.Attribution.PrincipalID == "" {
		return acceptedRatificationFacts{}, operationalOutcome("ratification-provenance-identity", fmt.Errorf("experimentapp: the accepted propose-ratification record is not attributed to an authenticated principal"))
	}
	if string(last.Attribution.PrincipalID) != record.ActorV2.PrincipalID {
		return acceptedRatificationFacts{}, operationalOutcome("ratification-provenance-identity", fmt.Errorf("experimentapp: provenance attribution principal %q does not correspond to the ratification actor %q", last.Attribution.PrincipalID, record.ActorV2.PrincipalID))
	}

	claim := governanceprincipal.PrincipalClaim{TrustSource: record.ActorV2.TrustSource, Subject: record.ActorV2.Subject}
	resolution, err := governanceprincipal.NewResolver(authority.Facts).Resolve(ctx, authority.Profile, claim)
	if err != nil {
		return acceptedRatificationFacts{}, operationalOutcome("ratification-principal-invalid", err)
	}
	if resolution.State != governanceprincipal.ResolutionAuthenticated {
		for _, witness := range resolution.Witnesses {
			if witness.Code == governanceprincipal.ReasonTrustSourceForbidden {
				return acceptedRatificationFacts{}, operationalOutcome("ratification-trust-source-missing", fmt.Errorf("experimentapp: persisted trust source %q is not configured by the accepted governance profile", claim.TrustSource))
			}
		}
		return acceptedRatificationFacts{}, verdictOutcome("ratification-actor-unauthenticated", fmt.Sprintf("persisted claim re-resolved to state %q", resolution.State))
	}
	if string(resolution.PrincipalID) != record.ActorV2.PrincipalID {
		return acceptedRatificationFacts{}, operationalOutcome("ratification-principal-mismatch", fmt.Errorf("experimentapp: kernel-derived principal %q does not equal the persisted principal %q", resolution.PrincipalID, record.ActorV2.PrincipalID))
	}

	return acceptedRatificationFacts{
		record: record, definition: definition, derived: derived,
		run: matchedRun, result: boundResult, principal: resolution.PrincipalID,
	}, cleanOutcome()
}
