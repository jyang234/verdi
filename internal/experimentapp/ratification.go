// Authenticated ratification and accepted-state resolution (Wave 5C Task
// 8; design §§3, 7, 9-10; SI-143, SI-146). Task 10's independent closure
// review proved that a persisted claim/id pair plus role-mapping
// membership cannot itself establish that the named human asserted the
// ratification operation (SI-150): a proposal is now constructed only
// from a sealed, authenticated governance-principal resolution AND its
// matching sealed action-bound retained proof, and accepted use
// re-verifies that retained proof's Ed25519 signature against the exact
// HISTORICAL accepted policy tree its own challenge names, instead of
// re-resolving the persisted claim through a caller-supplied trust-fact
// reader.
package experimentapp

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimenthuman"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyauthority"
)

const ratificationFileName = "ratification.yaml"

// RatificationProposalInput is the typed human ratification request. It
// deliberately carries no actor field: the only identity operands are the
// sealed kernel resolution and the sealed action-bound retained proof a
// successful experimenthuman.Verify minted alongside it (Task 10
// correction, SI-150, design §7). The resolution alone is no longer
// sufficient authority — accepted use re-verifies the retained proof, not
// the persisted claim/id pair.
type RatificationProposalInput struct {
	ResultDigest string
	Disposition  experiment.Disposition
	Candidate    string
	Reason       string
	Resolution   governanceprincipal.PrincipalResolution
	Proof        experimenthuman.RetainedProof
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

// AcceptedRatificationResult is the typed accepted-state outcome.
type AcceptedRatificationResult struct {
	Outcome        Outcome
	AcceptedHead   string
	ExperimentPath string
	Ratification   experiment.Ratification
	PrincipalID    governanceprincipal.PrincipalID
}

// ratificationInputDigestFields is the closed four-field semantic
// projection the design's canonical typed ratification-input digest binds
// (design §7): result_digest, disposition, candidate, and reason, with
// all four keys always present regardless of disposition. Actor,
// resolution, and proof transport are deliberately excluded — no
// `omitempty` tag, so an empty candidate/reason still serializes its key.
type ratificationInputDigestFields struct {
	ResultDigest string                 `json:"result_digest"`
	Disposition  experiment.Disposition `json:"disposition"`
	Candidate    string                 `json:"candidate"`
	Reason       string                 `json:"reason"`
}

// RatificationInputDigest computes the canonical typed ratification-input
// digest design §7 defines: exactly the four semantic fields, always
// present. Both ProposeRatification (proposal-time binding) and
// verifyRetainedRatificationProof (accepted-use rebinding) call this SAME
// function so the two can never silently diverge. CLI adapters use this
// seam to bind the same typed request without rederiving the projection.
func RatificationInputDigest(resultDigest string, disposition experiment.Disposition, candidate, reason string) (string, error) {
	digest, err := canonjson.Digest(ratificationInputDigestFields{
		ResultDigest: resultDigest, Disposition: disposition, Candidate: candidate, Reason: reason,
	})
	if err != nil {
		return "", fmt.Errorf("experimentapp: ratification input digest: %w", err)
	}
	return digest, nil
}

// resolutionEvidenceDigest recovers the SI-147 evidence digest a
// successful governanceprincipal.Resolver.Resolve minted for an
// authenticated resolution. Resolve's authenticated arm always mints
// exactly one witness (ReasonTrustSubjectVerified) carrying the fact's
// EvidenceDigest (resolve.go); AttributionFromResolution has already
// proven the resolution's seal is unmodified resolver output before this
// runs, so an authenticated resolution missing that witness is an
// internally inconsistent shape this refuses rather than silently passes.
func resolutionEvidenceDigest(res governanceprincipal.PrincipalResolution) (string, error) {
	for _, witness := range res.Witnesses {
		if witness.Code == governanceprincipal.ReasonTrustSubjectVerified {
			return witness.EvidenceDigest, nil
		}
	}
	return "", fmt.Errorf("experimentapp: authenticated resolution carries no %s witness", governanceprincipal.ReasonTrustSubjectVerified)
}

// subjectMappedInProfile reports whether subject is listed under
// trustSource by ANY role mapping of profile (design §7: "the same
// subject must still be mapped under the same trust source in the current
// exact accepted profile"). Role-mapping membership alone is never itself
// reported as evidence; this only gates whether a re-verified signature
// fact may even reach kernel resolution against the CURRENT profile.
func subjectMappedInProfile(profile governanceprincipal.Profile, trustSource, subject string) bool {
	for _, mapping := range profile.RoleMappings {
		if mapping.TrustSource != trustSource {
			continue
		}
		if slices.Contains(mapping.Subjects, subject) {
			return true
		}
	}
	return false
}

// oneShotTrustFactReader always answers with exactly the fact
// experimenthuman.VerifyRetained already derived from a genuinely
// re-verified historical Ed25519 signature (design §7). It carries
// signature-derived evidence only, is minted fresh for exactly one
// resolution, and is never caller-supplied (controller pin P5: no
// accepted-use API accepts a caller trust-fact source).
type oneShotTrustFactReader struct {
	fact governanceprincipal.TrustFact
}

func (r oneShotTrustFactReader) ReadTrustFact(context.Context, governanceprincipal.TrustSource, governanceprincipal.PrincipalClaim) (governanceprincipal.TrustFact, error) {
	fact := r.fact
	fact.Subjects = append([]string(nil), r.fact.Subjects...)
	return fact, nil
}

// resolveGovernancePolicySource builds a read-only repo-rooted fs.FS view
// of the .verdi/policy/ subtree at commit head, through the SAME
// AcceptedGit port every other accepted resolution uses. It reuses the
// accepted snapshot's own ORIGINAL complete enumeration (issuing no
// second ListTree call) when head equals the snapshot's already-resolved
// accepted HEAD (design §7: Git enumeration executes once per commit) and
// runs exactly one fresh ListTree only when head names a genuinely
// different historical commit — the retained proof's signed accepted_head
// need not remain the current accepted HEAD once later commits land on
// the default branch (design §7). Unreadable or non-regular-blob policy
// bytes are refused outright; the current worktree or current profile is
// never substituted for a historical tree that cannot be read.
func (s *Service) resolveGovernancePolicySource(ctx context.Context, identity Identity, snapshot acceptedSnapshot, head string) (fs.FS, error) {
	entries := snapshot.entries
	if head != snapshot.revision.Head {
		fresh, err := s.git.ListTree(ctx, identity.CheckoutRoot, head)
		if err != nil {
			return nil, fmt.Errorf("experimentapp: enumerate accepted policy tree at %s: %w", head, err)
		}
		entries = fresh
	}
	const policyPrefix = ".verdi/policy/"
	files := map[string][]byte{}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Path, policyPrefix) {
			continue
		}
		if err := experiment.ValidateRepoRelativePath(entry.Path); err != nil {
			return nil, fmt.Errorf("experimentapp: accepted policy path: %w", err)
		}
		if entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") || strings.TrimSpace(entry.Object) == "" {
			return nil, fmt.Errorf("experimentapp: accepted policy entry %q is not a regular blob", entry.Path)
		}
		if _, duplicate := files[entry.Path]; duplicate {
			return nil, fmt.Errorf("experimentapp: duplicate accepted policy path %q", entry.Path)
		}
		data, err := s.git.ReadBlob(ctx, identity.CheckoutRoot, head, entry.Object, entry.Path)
		if err != nil {
			return nil, fmt.Errorf("experimentapp: read accepted policy blob %s:%s: %w", head, entry.Path, err)
		}
		files[entry.Path] = append([]byte(nil), data...)
	}
	return newSnapshotFS(files), nil
}

// ProposeRatification writes the exact deterministic v3 ratification
// proposal for one accepted result, with authority drawn only from the
// sealed authenticated resolution AND its matching sealed retained proof
// (Task 10 correction, SI-150).
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

	// Task 10 correction (SI-150, design §7): the retained proof seal must
	// be genuine sealed Verify output, and it must name EXACTLY the same
	// claim, kernel principal, and evidence digest as the sealed
	// resolution above — a resolution and a proof minted by different
	// Verify calls (or a hand-built proof) can never both be true at once.
	proofChallengeBytes, err := input.Proof.ChallengeBytes()
	if err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("ratification-proof-unsealed", err)}
	}
	proofSignature, err := input.Proof.Signature()
	if err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("ratification-proof-unsealed", err)}
	}
	proofClaim, err := input.Proof.Claim()
	if err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("ratification-proof-unsealed", err)}
	}
	proofPrincipalID, err := input.Proof.PrincipalID()
	if err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("ratification-proof-unsealed", err)}
	}
	proofEvidenceDigest, err := input.Proof.EvidenceDigest()
	if err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("ratification-proof-unsealed", err)}
	}
	if proofClaim != input.Resolution.Claim || proofPrincipalID != input.Resolution.PrincipalID {
		return RatificationProposalResult{Outcome: operationalOutcome("ratification-proof-mismatch", fmt.Errorf("experimentapp: retained proof claim/principal does not match the sealed resolution"))}
	}
	resolutionDigest, err := resolutionEvidenceDigest(input.Resolution)
	if err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("ratification-resolution-invalid", err)}
	}
	if proofEvidenceDigest != resolutionDigest {
		return RatificationProposalResult{Outcome: operationalOutcome("ratification-proof-mismatch", fmt.Errorf("experimentapp: retained proof evidence digest %q does not match the sealed resolution's evidence digest %q", proofEvidenceDigest, resolutionDigest))}
	}
	challenge, err := experimenthuman.DecodeChallenge(proofChallengeBytes)
	if err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("ratification-proof-invalid", err)}
	}
	if challenge.Operation != experimenthuman.OperationProposeRatification {
		return RatificationProposalResult{Outcome: operationalOutcome("ratification-proof-mismatch", fmt.Errorf("experimentapp: retained proof operation %q is not %q", challenge.Operation, experimenthuman.OperationProposeRatification))}
	}
	if challenge.Spike != identity.Spike || challenge.ExperimentID != identity.ExperimentID {
		return RatificationProposalResult{Outcome: operationalOutcome("ratification-proof-mismatch", fmt.Errorf("experimentapp: retained proof identity %s/%s does not match %s/%s", challenge.Spike, challenge.ExperimentID, identity.Spike, identity.ExperimentID))}
	}
	if challenge.TrustSource != proofClaim.TrustSource {
		return RatificationProposalResult{Outcome: operationalOutcome("ratification-proof-mismatch", fmt.Errorf("experimentapp: retained proof trust source %q does not match the sealed claim %q", challenge.TrustSource, proofClaim.TrustSource))}
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
	if challenge.AcceptedHEAD != snapshot.revision.Head {
		return RatificationProposalResult{Outcome: operationalOutcome("ratification-proof-mismatch", fmt.Errorf("experimentapp: retained proof accepted_head %q does not match the accepted HEAD in use %q", challenge.AcceptedHEAD, snapshot.revision.Head))}
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
		Schema:       experiment.RatificationSchemaV3,
		ResultDigest: input.ResultDigest,
		ActorV2: &experiment.RatificationActor{
			TrustSource: input.Resolution.Claim.TrustSource,
			Subject:     input.Resolution.Claim.Subject,
			PrincipalID: string(input.Resolution.PrincipalID),
		},
		Proof: &experiment.AuthenticationProof{
			Schema:             experiment.HumanProofSchema,
			ChallengeBase64URL: base64.RawURLEncoding.EncodeToString(proofChallengeBytes),
			SignatureBase64URL: base64.RawURLEncoding.EncodeToString(proofSignature),
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

	// Task 10 correction (SI-150, design §7): STALE-CHALLENGE checks — the
	// retained proof must bind exactly this proposal's typed semantic
	// input and exactly this pre-ratification artifact set. Pin P1
	// symmetry: these are verdicts, not operational failures, because the
	// record and proof are each individually well-formed and merely fail
	// to bind each other (the record is well-formed but stale evidence).
	wantInputDigest, err := RatificationInputDigest(input.ResultDigest, input.Disposition, input.Candidate, input.Reason)
	if err != nil {
		return RatificationProposalResult{Outcome: operationalOutcome("ratification-record-invalid", err)}
	}
	if challenge.InputDigest != wantInputDigest {
		return RatificationProposalResult{Outcome: verdictOutcome("ratification-proof-stale", "retained proof input_digest does not match this proposal's typed ratification input")}
	}
	if challenge.ProposalDigest != proposedDigest {
		return RatificationProposalResult{Outcome: verdictOutcome("ratification-proof-stale", "retained proof proposal_digest does not match the pre-ratification human artifact projection")}
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
// re-verifies its retained V3 proof (Task 10 correction, SI-150). It is
// read-only, exact-tree, stale-HEAD-safe, and independent of divergent
// worktree bytes. Controller pin P5: no caller-supplied authority remains
// — every operand this needs is resolved from the accepted Git tree
// itself.
func (s *Service) AcceptedRatification(ctx context.Context, identity Identity) AcceptedRatificationResult {
	if ctx == nil {
		return AcceptedRatificationResult{Outcome: operationalOutcome("invalid-request", fmt.Errorf("experimentapp: ratification context is nil"))}
	}
	if err := identity.validate(); err != nil {
		return AcceptedRatificationResult{Outcome: operationalOutcome("invalid-request", err)}
	}
	snapshot, err := resolveAccepted(ctx, s.git, identity)
	if err != nil {
		var stale *staleAcceptedHeadError
		if errors.As(err, &stale) {
			return AcceptedRatificationResult{Outcome: verdictOutcome("accepted-head-stale", stale.Error())}
		}
		return AcceptedRatificationResult{Outcome: operationalOutcome("accepted-tree-invalid", err)}
	}
	facts, outcome := s.acceptedRatificationAt(ctx, identity, snapshot)
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
// classification are pinned by the existing tests). V1 and V2 are
// decode-only predecessor history (SI-150 retires V2's caller-Facts
// re-resolution); only V3's retained proof may authorize release or
// closure.
func (s *Service) acceptedRatificationAt(ctx context.Context, identity Identity, snapshot acceptedSnapshot) (acceptedRatificationFacts, Outcome) {
	// Ratification presupposes the registration lock: the accepted
	// definition must be the locked v2 record. The registration
	// provenance-pair check is deliberately NOT reused here — it pins the
	// LAST provenance record to the lock, which is true only until the
	// ratification mutation lands; design §7's ratification proof is the
	// exact-tree artifact resolution below plus retained-proof
	// re-verification.
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
	switch record.Schema {
	case experiment.RatificationSchema:
		return acceptedRatificationFacts{}, verdictOutcome("ratification-v1-history", "ratification v1 is decode-only predecessor history and cannot authorize release or closure")
	case experiment.RatificationSchemaV2:
		// Task 10 correction (SI-150, design §7): a persisted claim/id
		// pair plus role-mapping membership can never re-establish that
		// the named human asserted this operation — v2 joins v1 as
		// decode-only history and can never again authorize release or
		// closure.
		return acceptedRatificationFacts{}, verdictOutcome("ratification-v2-history", "ratification v2 is decode-only predecessor history (SI-150) and cannot authorize release or closure")
	}
	// record.Schema == RatificationSchemaV3 for every remaining path:
	// Validate already refused any other value at decode time.

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
	registrationPriorMatches, err := registrationPreviousDigestMatches(preimageFiles, snapshot.experimentPath, definitionBytes, definition, predecessor.PreviousDigest)
	if err != nil {
		return acceptedRatificationFacts{}, operationalOutcome("artifact-digest-invalid", err)
	}
	if !registrationPriorMatches {
		return acceptedRatificationFacts{}, verdictOutcome("ratification-provenance-incomplete", "the accepted propose-registration record does not bind the exact pre-registration artifact set")
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

	principal, outcome := s.verifyRetainedRatificationProof(ctx, identity, snapshot, record, preimageDigest)
	if outcome.Classification != ClassificationClean {
		return acceptedRatificationFacts{}, outcome
	}

	return acceptedRatificationFacts{
		record: record, definition: definition, derived: derived,
		run: matchedRun, result: boundResult, principal: principal,
	}, cleanOutcome()
}

// verifyRetainedRatificationProof is the Task 10 correction's accepted-use
// core (SI-150, design §7): it strict-decodes the retained V3 proof,
// re-verifies its Ed25519 signature against the HISTORICAL accepted
// policy tree named by the proof's own signed accepted_head (never the
// current worktree or current profile), requires the verified subject to
// remain mapped under the same trust source in the CURRENT accepted
// profile, resolves the re-verified fact through the governance kernel
// against that CURRENT profile, requires exact principal equality with
// the persisted actor and the accepted provenance attribution, and
// rebinds the proof to THIS accepted ratification: the canonical typed
// ratification-input digest and preimageDigest — the exact
// pre-ratification artifact-set digest the caller already computed
// (design §7's proposal_digest rebinding target). The signed accepted_head
// itself is deliberately NOT required to equal the current accepted HEAD:
// design §7 only requires it to remain a resolvable historical commit,
// because later unrelated commits routinely land after a ratification
// merges and before release or closure runs.
func (s *Service) verifyRetainedRatificationProof(ctx context.Context, identity Identity, snapshot acceptedSnapshot, record experiment.Ratification, preimageDigest string) (governanceprincipal.PrincipalID, Outcome) {
	if record.Proof == nil {
		// Unreachable: Validate already requires a v3 record to carry this
		// block. Kept as a fail-closed guard, never a silent pass.
		return "", operationalOutcome("ratification-proof-invalid", fmt.Errorf("experimentapp: v3 ratification carries no authentication_proof block"))
	}
	challengeBytes, err := record.Proof.ChallengeBytes()
	if err != nil {
		return "", operationalOutcome("ratification-proof-invalid", err)
	}
	signature, err := record.Proof.SignatureBytes()
	if err != nil {
		return "", operationalOutcome("ratification-proof-invalid", err)
	}
	challenge, err := experimenthuman.DecodeChallenge(challengeBytes)
	if err != nil {
		return "", operationalOutcome("ratification-proof-invalid", err)
	}

	historicalSource, err := s.resolveGovernancePolicySource(ctx, identity, snapshot, challenge.AcceptedHEAD)
	if err != nil {
		return "", operationalOutcome("ratification-proof-head-unreachable", err)
	}
	retained, err := experimenthuman.VerifyRetained(challengeBytes, signature, experimenthuman.AcceptedAuthority{
		Head: challenge.AcceptedHEAD, Source: historicalSource,
	})
	if err != nil {
		return "", operationalOutcome("ratification-proof-invalid", err)
	}
	if !retained.Verified {
		return "", verdictOutcome("ratification-proof-unsatisfied", fmt.Sprintf("retained proof did not re-verify against the historical accepted profile: %s", retained.Code))
	}

	// IDENTITY inconsistencies (controller pin P1): the re-verified claim
	// must be exactly the persisted actor's claim, and the challenge must
	// bind exactly this operation and this experiment.
	if challenge.TrustSource != record.ActorV2.TrustSource || retained.Subject != record.ActorV2.Subject {
		return "", operationalOutcome("ratification-provenance-identity", fmt.Errorf("experimentapp: retained proof claim %s/%s does not match the persisted ratification actor %s/%s", challenge.TrustSource, retained.Subject, record.ActorV2.TrustSource, record.ActorV2.Subject))
	}
	if challenge.Operation != experimenthuman.OperationProposeRatification {
		return "", operationalOutcome("ratification-provenance-identity", fmt.Errorf("experimentapp: retained proof operation %q is not %q", challenge.Operation, experimenthuman.OperationProposeRatification))
	}
	if challenge.Spike != identity.Spike || challenge.ExperimentID != identity.ExperimentID {
		return "", operationalOutcome("ratification-provenance-identity", fmt.Errorf("experimentapp: retained proof identity %s/%s does not match the accepted experiment %s/%s", challenge.Spike, challenge.ExperimentID, identity.Spike, identity.ExperimentID))
	}

	// The historical and current accepted policy tree are frequently the
	// SAME commit (a ratification proposed against the still-current
	// accepted HEAD) — reuse the already-fetched blobs rather than
	// re-reading the identical policy tree a second time.
	currentSource := historicalSource
	if challenge.AcceptedHEAD != snapshot.revision.Head {
		currentSource, err = s.resolveGovernancePolicySource(ctx, identity, snapshot, snapshot.revision.Head)
		if err != nil {
			return "", operationalOutcome("ratification-proof-head-unreachable", err)
		}
	}
	currentStore, err := policyauthority.LoadFromSource(currentSource)
	if err != nil {
		return "", operationalOutcome("ratification-authority-invalid", err)
	}
	currentProfile, err := currentStore.SelectedProfile()
	if err != nil {
		return "", operationalOutcome("ratification-authority-invalid", err)
	}
	if !subjectMappedInProfile(currentProfile, challenge.TrustSource, retained.Subject) {
		return "", verdictOutcome("ratification-proof-unsatisfied", fmt.Sprintf("subject %q is no longer mapped under trust source %q in the current accepted profile", retained.Subject, challenge.TrustSource))
	}

	resolution, err := governanceprincipal.NewResolver(oneShotTrustFactReader{fact: retained.Fact}).Resolve(ctx, currentProfile, governanceprincipal.PrincipalClaim{
		TrustSource: challenge.TrustSource, Subject: retained.Subject,
	})
	if err != nil {
		return "", operationalOutcome("ratification-principal-invalid", err)
	}
	// UNREACHABLE fail-closed guards (Task 10 correction lane review F2 —
	// coverage record, not dead code to delete). Both arms below can only
	// fire on a resolution that is not authenticated, and no such resolution
	// can reach here: the mapping check immediately above already refused
	// unless challenge.TrustSource carries a role mapping naming
	// retained.Subject in the CURRENT profile, and
	// governanceprincipal/validate.go:117 (resolveTrustSource, called from
	// validateRoleMappings) makes a role mapping decodable only when its
	// trust source also exists in identity_trust_sources. So a current
	// profile that dropped the trust source necessarily dropped the mapping
	// too and is refused as "no longer mapped" — never as
	// ratification-trust-source-missing — and oneShotTrustFactReader always
	// answers with the available/valid signature-derived fact for exactly
	// that mapped subject, so the kernel's authenticated arm always applies.
	// The reasoning also leans on the kernel's validateTrustFact kind check:
	// subjectMappedInProfile ignores trust-source kind, so a role mapping
	// naming a trust source whose CURRENT kind has drifted away from
	// identity-provider (e.g. edited to forge) still passes the mapping
	// check above, but then diverges into the operational
	// ratification-principal-invalid arm via the kernel's own
	// validateTrustFact SourceKind mismatch (resolve.go), never into this
	// guard. The guards stay: an unauthenticated resolution reaching here
	// would be an internally inconsistent kernel result that must refuse,
	// never silently pass.
	if resolution.State != governanceprincipal.ResolutionAuthenticated {
		for _, witness := range resolution.Witnesses {
			if witness.Code == governanceprincipal.ReasonTrustSourceForbidden {
				return "", operationalOutcome("ratification-trust-source-missing", fmt.Errorf("experimentapp: persisted trust source %q is not configured by the current accepted governance profile", challenge.TrustSource))
			}
		}
		return "", verdictOutcome("ratification-actor-unauthenticated", fmt.Sprintf("re-verified proof fact resolved to state %q", resolution.State))
	}
	// Also UNREACHABLE, for a distinct reason (same lane review F2): the
	// identity check above forced record.ActorV2.TrustSource ==
	// challenge.TrustSource and record.ActorV2.Subject == retained.Subject,
	// experiment.RatificationActor.Validate refused the record at decode time
	// unless its principal_id is exactly CanonicalPrincipalID of that same
	// pair, and the kernel derives resolution.PrincipalID from exactly that
	// pair too. Kept as the fail-closed guard against any future divergence
	// between the wire derivation and the kernel derivation.
	if string(resolution.PrincipalID) != record.ActorV2.PrincipalID {
		return "", operationalOutcome("ratification-principal-mismatch", fmt.Errorf("experimentapp: kernel-derived principal %q does not equal the persisted principal %q", resolution.PrincipalID, record.ActorV2.PrincipalID))
	}

	// STATE rebinding (controller pin P1): the retained proof must bind
	// exactly this accepted ratification's semantic fields and exactly
	// this accepted pre-ratification artifact projection.
	wantInputDigest, err := RatificationInputDigest(record.ResultDigest, record.Disposition, record.Candidate, record.Reason)
	if err != nil {
		return "", operationalOutcome("ratification-proof-invalid", err)
	}
	if challenge.InputDigest != wantInputDigest {
		return "", verdictOutcome("ratification-proof-stale", "retained proof input_digest does not match the accepted ratification's own semantic fields")
	}
	if challenge.ProposalDigest != preimageDigest {
		return "", verdictOutcome("ratification-proof-stale", "retained proof proposal_digest does not match the accepted pre-ratification artifact projection")
	}

	return resolution.PrincipalID, cleanOutcome()
}
