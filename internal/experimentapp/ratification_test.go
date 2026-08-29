package experimentapp

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentdecision"
	"github.com/jyang234/verdi/internal/experimenthuman"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// ratificationProfile is the sealed accepted governance profile the
// NEGATIVE resolution-shape tests (forged/mutated/violated/unproven
// resolutions in TestProposeRatificationRequiresSealedAuthenticatedResolution)
// resolve against directly through governanceprincipal.Resolver — those
// tests fail before ProposeRatification ever reaches the Task 10 proof
// checks, so they need no relationship to any retained proof.
func ratificationProfile(t *testing.T, sourceID string) governanceprincipal.Profile {
	t.Helper()
	profile, err := governanceprincipal.DecodeProfile([]byte(`schema: verdi.governance-profile/v1
id: solo-default
class: solo
applicable_transitions: [accept]
identity_trust_sources:
  - { id: `+sourceID+`, kind: forge }
role_mappings: []
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
`), governanceprincipal.Catalog{Transitions: []string{"accept"}})
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

// ratificationFacts attests exactly the given subjects for the github
// trust source.
func ratificationFacts(subjects ...string) trustFactReaderFunc {
	return func(context.Context, governanceprincipal.TrustSource, governanceprincipal.PrincipalClaim) (governanceprincipal.TrustFact, error) {
		return governanceprincipal.TrustFact{
			SourceID: "github", SourceKind: governanceprincipal.TrustSourceForge,
			Subjects:       append([]string(nil), subjects...),
			EvidenceDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Available:      true, Valid: true,
		}, nil
	}
}

// ratificationUnavailableFacts reports expected-missing evidence, driving
// the kernel to an unproven resolution.
func ratificationUnavailableFacts() trustFactReaderFunc {
	return func(context.Context, governanceprincipal.TrustSource, governanceprincipal.PrincipalClaim) (governanceprincipal.TrustFact, error) {
		return governanceprincipal.TrustFact{
			SourceID: "github", SourceKind: governanceprincipal.TrustSourceForge,
			Available: false, Valid: false, Reason: "identity provider unreachable in this environment",
		}, nil
	}
}

// ratificationResolve resolves the standard user-123 claim to a genuine
// sealed kernel resolution through the given fact reader. It is used only
// by tests of ProposeRatification's RESOLUTION-shape validation, which
// runs and fails before the Task 10 proof checks ever see the input.
func ratificationResolve(t *testing.T, facts trustFactReaderFunc) governanceprincipal.PrincipalResolution {
	t.Helper()
	resolution, err := governanceprincipal.NewResolver(facts).Resolve(
		context.Background(), ratificationProfile(t, "github"),
		governanceprincipal.PrincipalClaim{TrustSource: "github", Subject: "user-123"})
	if err != nil {
		t.Fatal(err)
	}
	return resolution
}

// ratificationSigner is one generated offline-human Ed25519 identity used
// only inside this test process to produce a genuine detached signature
// over a genuine challenge (design §8) — the private key is never treated
// as ambient identity or durable authority.
type ratificationSigner struct {
	public  ed25519.PublicKey
	private ed25519.PrivateKey
	subject string
}

func newRatificationSigner(t *testing.T) ratificationSigner {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return ratificationSigner{public: pub, private: priv, subject: "ed25519:" + base64.RawURLEncoding.EncodeToString(pub)}
}

// ratificationGovernanceProfileBytes builds the .verdi/policy/ constitution
// and profile pair mapping exactly the given offline-human subjects
// (design §8's identity-provider trust source), mirroring cmd/verdi's
// buildExperimentHumanRepoWithSubjects precedent.
func ratificationGovernanceProfileBytes(subjects []string) (constitution, profile []byte) {
	quoted := make([]string, len(subjects))
	for i, s := range subjects {
		quoted[i] = strconv.Quote(s)
	}
	// A role mapping's subjects list must be nonempty grammar (governanceprincipal
	// decode-time rule): the "nobody mapped" fixture omits the mapping
	// entirely rather than emitting one with an empty subjects array.
	roleMappings := "role_mappings: []\n"
	if len(subjects) > 0 {
		roleMappings = "role_mappings:\n  - {role: author, trust_source: offline-human, subjects: [" + strings.Join(quoted, ", ") + "]}\n"
	}
	constitution = []byte(`---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Ratification proof fixture constitution"
owners: [platform-team]
selected_profile: solo-default
environments: [local]
catalog:
  roles: [author, reviewer, policy-owner]
  transitions: [accept]
  evidence_sources: [ci]
  escalation_metrics: [age-days]
subjects:
  action: []
  configuration: []
  capability: []
  resource: []
  identity: []
  evidence: []
adapters:
  - id: codex
    version: "1"
    managed: [AGENTS.md]
    discovery_filenames: [AGENTS.md]
---
Hermetic ratification-proof fixture constitution.
`)
	profile = []byte(fmt.Sprintf(`---
schema: verdi.governance-profile/v1
id: solo-default
class: solo
applicable_transitions: [accept]
identity_trust_sources:
  - {id: offline-human, kind: identity-provider}
%sownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
---
Hermetic offline-human ratification profile.
`, roleMappings))
	return constitution, profile
}

// ratificationPolicySource is the in-memory accepted policy tree view an
// experimenthuman.Verify call (played by these tests in the offline-human
// adapter's role, design §8) resolves the offline-human profile from.
func ratificationPolicySource(subjects ...string) fs.FS {
	constitution, profile := ratificationGovernanceProfileBytes(subjects)
	return newSnapshotFS(map[string][]byte{
		".verdi/policy/constitution.md":          constitution,
		".verdi/policy/profiles/solo-default.md": profile,
	})
}

// plantGitBlob installs (or replaces) one blob at path in the fake
// accepted Git tree.
func plantGitBlob(git *fakeGit, path string, data []byte) {
	for i := range git.entries {
		if git.entries[i].Path == path {
			git.blobs[path] = append([]byte(nil), data...)
			return
		}
	}
	git.entries = append(git.entries, GitTreeEntry{Mode: "100644", Type: "blob", Object: "object-policy-" + path, Path: path})
	git.blobs[path] = append([]byte(nil), data...)
}

// plantRatificationGovernanceProfile installs the SAME .verdi/policy/
// tree an experimenthuman.Verify call already resolved from into the fake
// accepted Git tree, so the application's OWN accepted-use resolution
// (never a caller-supplied authority, Task 10 correction pin P5) sees the
// identical mapped subjects. Zero subjects installs a profile mapping
// nobody, for the "subject unmapped" negative case.
func plantRatificationGovernanceProfile(git *fakeGit, subjects ...string) {
	constitution, profile := ratificationGovernanceProfileBytes(subjects)
	plantGitBlob(git, ".verdi/policy/constitution.md", constitution)
	plantGitBlob(git, ".verdi/policy/profiles/solo-default.md", profile)
}

// refreshAcceptedGit rebuilds the fake accepted Git tree from the current
// worktree state and re-plants the offline-human governance profile
// mapping subjects — mirroring how a real accepted merge carries the
// repository's own .verdi/policy/ tree forward alongside worktree content.
func refreshAcceptedGit(t *testing.T, service *Service, root, experimentID string, subjects ...string) *fakeGit {
	t.Helper()
	git := gitFromExperimentDir(t, root, experimentID)
	plantRatificationGovernanceProfile(git, subjects...)
	service.git = git
	return git
}

// ratificationChallengeFacts builds the exact action-bound challenge
// facts a genuine propose-ratification offline-human adapter call would
// bind (design §8): the CURRENT accepted HEAD in use and the exact
// pre-ratification worktree artifact set.
func ratificationChallengeFacts(t *testing.T, root string, identity Identity, resultDigest string, disposition experiment.Disposition, candidate, reason string) experimenthuman.ChallengeFacts {
	t.Helper()
	experimentPath := filepath.ToSlash(filepath.Join(".verdi", "specs", "active", strings.TrimPrefix(identity.Spike, "spec/"), "experiments", identity.ExperimentID))
	proposed, err := readProposedArtifactFiles(root, experimentPath)
	if err != nil {
		t.Fatal(err)
	}
	proposalDigest, err := artifactSetDigest(proposed, experimentPath)
	if err != nil {
		t.Fatal(err)
	}
	inputDigest, err := RatificationInputDigest(resultDigest, disposition, candidate, reason)
	if err != nil {
		t.Fatal(err)
	}
	return experimenthuman.ChallengeFacts{
		Operation: experimenthuman.OperationProposeRatification, Spike: identity.Spike, ExperimentID: identity.ExperimentID,
		AcceptedHEAD: identity.ExpectedAcceptedHEAD, ProposalHEAD: identity.ExpectedAcceptedHEAD,
		TrustSource: "offline-human", InputDigest: inputDigest, ProposalDigest: proposalDigest,
	}
}

// signRatificationChallenge plays the offline-human adapter's role
// (design §8) for arbitrary challenge facts: it builds the canonical
// challenge, signs it with signer's private key, and mints the sealed
// resolution + retained proof pair through a genuine experimenthuman.Verify
// call — the ONLY way either seal may ever be minted.
func signRatificationChallenge(t *testing.T, signer ratificationSigner, facts experimenthuman.ChallengeFacts, subjects ...string) experimenthuman.Verification {
	t.Helper()
	challenge, err := experimenthuman.NewChallenge(facts)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := challenge.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	signature := ed25519.Sign(signer.private, canonical)
	verification, err := experimenthuman.Verify(context.Background(), facts, canonical, signature, experimenthuman.AcceptedAuthority{
		Head: facts.AcceptedHEAD, Source: ratificationPolicySource(subjects...),
	})
	if err != nil {
		t.Fatal(err)
	}
	return verification
}

// mintRatificationVerification mints a genuine authenticated resolution +
// retained proof pair for a real propose-ratification challenge over
// identity's current accepted HEAD and worktree state.
func mintRatificationVerification(t *testing.T, root string, identity Identity, signer ratificationSigner, subjects []string, resultDigest string, disposition experiment.Disposition, candidate, reason string) experimenthuman.Verification {
	t.Helper()
	facts := ratificationChallengeFacts(t, root, identity, resultDigest, disposition, candidate, reason)
	verification := signRatificationChallenge(t, signer, facts, subjects...)
	if verification.State != governanceprincipal.ResolutionAuthenticated {
		t.Fatalf("mintRatificationVerification: state = %q, want authenticated", verification.State)
	}
	return verification
}

// ratificationProposalInputFrom assembles a RatificationProposalInput from
// a genuine minted verification's sealed resolution AND matching sealed
// retained proof (Task 10 correction, SI-150) — never a hand-built pair.
func ratificationProposalInputFrom(verification experimenthuman.Verification, resultDigest string, disposition experiment.Disposition, candidate, reason string) RatificationProposalInput {
	return RatificationProposalInput{
		ResultDigest: resultDigest, Disposition: disposition, Candidate: candidate, Reason: reason,
		Resolution: verification.Resolution, Proof: verification.Retained,
	}
}

// buildV3RatificationRecord projects a genuine minted verification's proof
// into the exact V3 wire shape ProposeRatification's production code
// builds — used by tests that plant accepted bytes directly (mirroring
// the existing plantAcceptedRatification precedent) rather than going
// through ProposeRatification's writer path.
func buildV3RatificationRecord(t *testing.T, verification experimenthuman.Verification, resultDigest string, disposition experiment.Disposition, candidate, reason string) experiment.Ratification {
	t.Helper()
	challengeBytes, err := verification.Retained.ChallengeBytes()
	if err != nil {
		t.Fatal(err)
	}
	signature, err := verification.Retained.Signature()
	if err != nil {
		t.Fatal(err)
	}
	return experiment.Ratification{
		Schema: experiment.RatificationSchemaV3, ResultDigest: resultDigest,
		ActorV2: &experiment.RatificationActor{
			TrustSource: verification.Resolution.Claim.TrustSource, Subject: verification.Resolution.Claim.Subject,
			PrincipalID: string(verification.Resolution.PrincipalID),
		},
		Proof: &experiment.AuthenticationProof{
			Schema:             experiment.HumanProofSchema,
			ChallengeBase64URL: base64.RawURLEncoding.EncodeToString(challengeBytes),
			SignatureBase64URL: base64.RawURLEncoding.EncodeToString(signature),
		},
		Disposition: disposition, Candidate: candidate, Reason: reason,
	}
}

func ratificationV3Bytes(t *testing.T, verification experimenthuman.Verification, resultDigest string, disposition experiment.Disposition, candidate, reason string) []byte {
	t.Helper()
	record := buildV3RatificationRecord(t, verification, resultDigest, disposition, candidate, reason)
	encoded, err := experiment.EncodeRatification(record)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

// ratifiableObservations builds one complete observation set for the
// locked definition, with the cache candidate at cacheValue.
func ratifiableObservations(t *testing.T, def experiment.Definition, run string, cacheValue int, schema string) []experiment.Observation {
	t.Helper()
	definitionDigest, err := experiment.DefinitionDigest(def)
	if err != nil {
		t.Fatal(err)
	}
	observations := make([]experiment.Observation, 0, len(def.Candidates)*def.Execution.Rounds)
	for _, candidate := range def.Candidates {
		value := 100
		if candidate.ID == "cache" {
			value = cacheValue
		}
		for round := 1; round <= def.Execution.Rounds; round++ {
			observations = append(observations, experiment.Observation{
				Schema: experiment.ObservationSchema, ExperimentDigest: definitionDigest,
				Run: run, Candidate: candidate.ID, Round: round,
				Guards: []experiment.GuardResult{}, Measurements: []experiment.Measurement{{
					ID: "request-latency", Value: experiment.NumberValue(json.Number(strconv.Itoa(value))),
					Unit: "ms", Source: experiment.SourceEvaluatorMeasured,
				}}, Disclosures: []string{},
			})
		}
	}
	if schema == experiment.ObservationSchemaV2 {
		for i := range observations {
			observations[i].Schema = experiment.ObservationSchemaV2
			observations[i].Outcome = &experiment.CandidateOutcome{Kind: experiment.OutcomeCompleted}
		}
	}
	return observations
}

func encodeRatifiableObservations(t *testing.T, observations []experiment.Observation) []byte {
	t.Helper()
	var observationBytes bytes.Buffer
	for _, observation := range observations {
		line, err := experiment.EncodeObservation(observation)
		if err != nil {
			t.Fatal(err)
		}
		observationBytes.Write(line)
	}
	return observationBytes.Bytes()
}

// ratifiableReceipt builds a schema-valid execution receipt for the locked
// definition and run (the acceptingVerifier fixture judges deeper parity).
func ratifiableReceipt(t *testing.T, def experiment.Definition, run string) experiment.ExecutionReceipt {
	t.Helper()
	definitionDigest, err := experiment.DefinitionDigest(def)
	if err != nil {
		t.Fatal(err)
	}
	candidates := make([]experiment.ReceiptCandidate, 0, len(def.Candidates))
	for _, candidate := range def.Candidates {
		workspaceID, err := experiment.WorkspaceRunID(definitionDigest, run, candidate.ID)
		if err != nil {
			t.Fatal(err)
		}
		candidates = append(candidates, experiment.ReceiptCandidate{
			ID: candidate.ID, BaseCommit: candidate.Base, PatchDigest: candidate.Digest, WorkspaceRunID: workspaceID,
			Materialization: experiment.WorkspaceIdentity{Shape: experiment.WorkspaceBasePlusPatch, RunID: workspaceID, CommitSHA: candidate.Base, PatchSHA256: strings.TrimPrefix(candidate.Digest, "sha256:")},
		})
	}
	fixtureInputs := make([]experiment.ResolvedArtifact, len(def.Fixtures))
	for index, fixture := range def.Fixtures {
		fixtureInputs[index] = experiment.ResolvedArtifact{ID: fixture.ID, Path: "fixtures/" + fixture.ID + ".bin", Digest: fixture.Digest}
	}
	return experiment.ExecutionReceipt{
		Schema: experiment.ExecutionReceiptSchema, ExperimentDigest: definitionDigest, Run: run,
		EnvironmentPolicy: def.Execution.EnvironmentPolicy,
		AuthorityDigest:   "sha256:" + strings.Repeat("1", 64), CapabilitiesDigest: def.Evaluator.CapabilitiesDigest,
		ScheduleDigest: "sha256:" + strings.Repeat("2", 64), GrantsDigest: "sha256:" + strings.Repeat("3", 64),
		Fingerprint: experiment.ExecutionFingerprint{OS: "linux", Arch: "amd64", ToolVersions: map[string]string{"evaluator": "2.1.0", "verdi": "0.1.0"}, Env: map[string]*string{}, InputDigests: map[string]string{
			"inputs/workload.txt": strings.TrimPrefix(def.Workload.Digest, "sha256:"),
		}},
		Inputs: experiment.ReceiptInputs{
			Workload: experiment.ResolvedArtifact{ID: def.Workload.ID, Path: "inputs/workload.txt", Digest: def.Workload.Digest},
			Fixtures: fixtureInputs,
			Contract: experiment.ResolvedArtifact{ID: def.Contract.ID, Path: "inputs/contract.txt", Digest: def.Contract.Digest},
		},
		Enforcement: []experiment.ReceiptEnforcement{{Kind: "process-execution", Applied: true, Reason: "allowlist applied"}, {Kind: "timeouts", Applied: true, Reason: "deadline applied"}},
		Network:     experiment.ReceiptNetwork{Mode: experiment.NetworkDeny, Configured: true, Reason: "network namespace configured"},
		Candidates:  candidates,
		Versions:    experiment.ReceiptVersions{Verdi: "0.1.0", RecommendationEngine: string(experiment.AlgorithmV1)},
		Disclosures: []experiment.ReceiptDisclosure{experiment.DisclosureCPUAllocationUnproven, experiment.DisclosureMemoryAllocationUnproven},
	}
}

// writeRatifiableRun materializes one complete accepted run with
// AUTHORITATIVE v2 evidence — v2 observations, a validated execution
// receipt, and the v2 result binding that receipt — and returns the exact
// result digest (design §7: ratification is proven only with the receipt).
func writeRatifiableRun(t *testing.T, root string, def experiment.Definition, run string, cacheValue int) string {
	t.Helper()
	observations := ratifiableObservations(t, def, run, cacheValue, experiment.ObservationSchemaV2)
	core, err := experimentdecision.Evaluate(def, observations, experimentdecision.EnvironmentAttestation{PolicyID: def.Execution.EnvironmentPolicy})
	if err != nil {
		t.Fatal(err)
	}
	decision, err := experiment.DecisionFromResult(core, observations)
	if err != nil {
		t.Fatal(err)
	}
	receipt := ratifiableReceipt(t, def, run)
	receiptDigest, err := experiment.ExecutionReceiptDigest(receipt)
	if err != nil {
		t.Fatal(err)
	}
	result, err := experiment.NewResultV2(decision, experiment.ResultExecution{
		ExecutionDigest: receiptDigest,
		Isolation: experiment.ResultIsolation{
			Network:     receipt.Network,
			Disclosures: []experiment.IsolationDisclosure{},
		},
		WarmupDiagnostics: experiment.WarmupDiagnostics{
			Authority: experiment.WarmupAuthorityNonDecisionDiagnostic,
			Scope:     experiment.WarmupScopeFinalInvocation, Failures: []experiment.WarmupFailure{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	receiptBytes, err := experiment.EncodeExecutionReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	resultBytes, err := experiment.EncodeResult(result)
	if err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(filepath.Dir(mutationDefinitionPath(root)), "runs", run)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "execution.json"), receiptBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "observations.jsonl"), encodeRatifiableObservations(t, observations), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "result.json"), resultBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := experiment.ResultDigest(result)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

// writeRatifiableRunV1 materializes decode-only v1 run evidence — v1
// observations and a receiptless v1 result — which remains valid state
// history but must never become fresh ratification authority.
func writeRatifiableRunV1(t *testing.T, root string, def experiment.Definition, run string, cacheValue int) string {
	t.Helper()
	observations := ratifiableObservations(t, def, run, cacheValue, experiment.ObservationSchema)
	result, err := experimentdecision.Evaluate(def, observations, experimentdecision.EnvironmentAttestation{PolicyID: def.Execution.EnvironmentPolicy})
	if err != nil {
		t.Fatal(err)
	}
	resultBytes, err := experiment.EncodeResult(result)
	if err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(filepath.Dir(mutationDefinitionPath(root)), "runs", run)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "observations.jsonl"), encodeRatifiableObservations(t, observations), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "result.json"), resultBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := experiment.ResultDigest(result)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

// ratifiableService builds an accepted, locked, executed experiment:
// registeredExecutionService's accepted registration plus two complete
// accepted runs — run-alpha proven-winner, run-zeta inconclusive.
func ratifiableService(t *testing.T) (root string, service *Service, identity Identity, winnerDigest, inconclusiveDigest string) {
	t.Helper()
	root, service, _, identity = registeredExecutionService(t, &recordingExecutionRunner{})
	definitionBytes, err := os.ReadFile(mutationDefinitionPath(root))
	if err != nil {
		t.Fatal(err)
	}
	locked, err := experiment.DecodeDefinition(definitionBytes)
	if err != nil {
		t.Fatal(err)
	}
	winnerDigest = writeRatifiableRun(t, root, locked, "run-alpha", 50)
	inconclusiveDigest = writeRatifiableRun(t, root, locked, "run-zeta", 100)
	service.git = gitFromExperimentDir(t, root, "request-path-v2")
	return root, service, identity, winnerDigest, inconclusiveDigest
}

func humanRatificationIdentity(t *testing.T, identity Identity, resolution governanceprincipal.PrincipalResolution) Identity {
	t.Helper()
	actor, err := NewAuthenticatedHuman(resolution)
	if err != nil {
		t.Fatal(err)
	}
	identity.Actor = actor
	return identity
}

func TestProposeRatificationRequiresSealedAuthenticatedResolution(t *testing.T) {
	_, service, identity, winnerDigest, _ := ratifiableService(t)
	resolution := ratificationResolve(t, ratificationFacts("user-123"))
	human := humanRatificationIdentity(t, identity, resolution)
	input := RatificationProposalInput{ResultDigest: winnerDigest, Disposition: experiment.DispositionSelectRecommended}

	t.Run("delegated agent is refused", func(t *testing.T) {
		agentInput := input
		agentInput.Resolution = resolution
		result := service.ProposeRatification(context.Background(), identity, agentInput)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "human-actor-required" {
			t.Fatalf("ProposeRatification(agent) outcome = %+v", result.Outcome)
		}
	})

	t.Run("forged resolution is operational", func(t *testing.T) {
		id, err := governanceprincipal.CanonicalPrincipalID("github", "user-123")
		if err != nil {
			t.Fatal(err)
		}
		forged := governanceprincipal.PrincipalResolution{
			Claim:       governanceprincipal.PrincipalClaim{TrustSource: "github", Subject: "user-123"},
			PrincipalID: id, State: governanceprincipal.ResolutionAuthenticated,
			Witnesses: []governanceprincipal.Witness{},
		}
		forgedInput := input
		forgedInput.Resolution = forged
		result := service.ProposeRatification(context.Background(), human, forgedInput)
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("ProposeRatification(forged) outcome = %+v, want operational", result.Outcome)
		}
	})

	t.Run("mutated resolution is operational", func(t *testing.T) {
		mutated := resolution
		mutated.Witnesses = append([]governanceprincipal.Witness(nil), mutated.Witnesses...)
		mutated.Witnesses = append(mutated.Witnesses, governanceprincipal.Witness{Code: "injected", SourceID: "github"})
		mutatedInput := input
		mutatedInput.Resolution = mutated
		result := service.ProposeRatification(context.Background(), human, mutatedInput)
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("ProposeRatification(mutated) outcome = %+v, want operational", result.Outcome)
		}
	})

	t.Run("violated resolution is a verdict", func(t *testing.T) {
		violated := ratificationResolve(t, ratificationFacts("someone-else"))
		if violated.State != governanceprincipal.ResolutionViolated {
			t.Fatalf("fixture state = %q, want violated", violated.State)
		}
		violatedInput := input
		violatedInput.Resolution = violated
		result := service.ProposeRatification(context.Background(), human, violatedInput)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-actor-unauthenticated" {
			t.Fatalf("ProposeRatification(violated) outcome = %+v", result.Outcome)
		}
	})

	t.Run("unproven resolution is a verdict", func(t *testing.T) {
		unproven := ratificationResolve(t, ratificationUnavailableFacts())
		if unproven.State != governanceprincipal.ResolutionUnproven {
			t.Fatalf("fixture state = %q, want unproven", unproven.State)
		}
		unprovenInput := input
		unprovenInput.Resolution = unproven
		result := service.ProposeRatification(context.Background(), human, unprovenInput)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-actor-unauthenticated" {
			t.Fatalf("ProposeRatification(unproven) outcome = %+v", result.Outcome)
		}
	})

	t.Run("identity actor and resolution principal must agree", func(t *testing.T) {
		other, err := governanceprincipal.NewResolver(ratificationFacts("user-456")).Resolve(
			context.Background(), ratificationProfile(t, "github"),
			governanceprincipal.PrincipalClaim{TrustSource: "github", Subject: "user-456"})
		if err != nil {
			t.Fatal(err)
		}
		mismatchedInput := input
		mismatchedInput.Resolution = other
		result := service.ProposeRatification(context.Background(), human, mismatchedInput)
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("ProposeRatification(principal mismatch) outcome = %+v, want operational", result.Outcome)
		}
	})
}

// TestProposeRatificationRequiresGenuineRetainedProof is the Task 10
// correction's proposal-time proof matrix (SI-150, design §7, controller
// pin P1 symmetry): a sealed authenticated resolution alone is no longer
// sufficient — the sealed retained proof must independently seal-check,
// name the exact same claim/principal/evidence digest as the resolution,
// and bind the exact accepted HEAD in use.
func TestProposeRatificationRequiresGenuineRetainedProof(t *testing.T) {
	root, service, identity, winnerDigest, _ := ratifiableService(t)
	signer := newRatificationSigner(t)
	subjects := []string{signer.subject}
	refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
	verification := mintRatificationVerification(t, root, identity, signer, subjects, winnerDigest, experiment.DispositionSelectRecommended, "", "")
	human := humanRatificationIdentity(t, identity, verification.Resolution)

	t.Run("missing proof is operational", func(t *testing.T) {
		input := RatificationProposalInput{ResultDigest: winnerDigest, Disposition: experiment.DispositionSelectRecommended, Resolution: verification.Resolution}
		result := service.ProposeRatification(context.Background(), human, input)
		if result.Outcome.Classification != ClassificationOperational || result.Outcome.Code != "ratification-proof-unsealed" {
			t.Fatalf("outcome = %+v, want ratification-proof-unsealed operational", result.Outcome)
		}
	})

	t.Run("proof from a different claim is operational", func(t *testing.T) {
		other := newRatificationSigner(t)
		otherSubjects := []string{signer.subject, other.subject}
		refreshAcceptedGit(t, service, root, "request-path-v2", otherSubjects...)
		otherVerification := mintRatificationVerification(t, root, identity, other, otherSubjects, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		mismatched := ratificationProposalInputFrom(verification, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		mismatched.Proof = otherVerification.Retained
		result := service.ProposeRatification(context.Background(), human, mismatched)
		if result.Outcome.Classification != ClassificationOperational || result.Outcome.Code != "ratification-proof-mismatch" {
			t.Fatalf("outcome = %+v, want ratification-proof-mismatch operational", result.Outcome)
		}
		refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
	})

	t.Run("proof accepted_head mismatch is operational", func(t *testing.T) {
		staleFacts := ratificationChallengeFacts(t, root, identity, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		staleFacts.AcceptedHEAD = strings.Repeat("c", 40)
		staleVerification := signRatificationChallenge(t, signer, staleFacts, subjects...)
		input := ratificationProposalInputFrom(staleVerification, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		result := service.ProposeRatification(context.Background(), human, input)
		if result.Outcome.Classification != ClassificationOperational || result.Outcome.Code != "ratification-proof-mismatch" {
			t.Fatalf("outcome = %+v, want ratification-proof-mismatch operational", result.Outcome)
		}
	})

	t.Run("stale input digest is a verdict", func(t *testing.T) {
		staleInput := mintRatificationVerification(t, root, identity, signer, subjects, winnerDigest, experiment.DispositionRejectAll, "", "")
		input := ratificationProposalInputFrom(staleInput, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		result := service.ProposeRatification(context.Background(), human, input)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-proof-stale" {
			t.Fatalf("outcome = %+v, want ratification-proof-stale verdict", result.Outcome)
		}
	})

	t.Run("stale proposal digest is a verdict", func(t *testing.T) {
		staleFacts := ratificationChallengeFacts(t, root, identity, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		staleFacts.ProposalDigest = "sha256:" + strings.Repeat("7", 64)
		staleVerification := signRatificationChallenge(t, signer, staleFacts, subjects...)
		input := ratificationProposalInputFrom(staleVerification, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		result := service.ProposeRatification(context.Background(), human, input)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-proof-stale" {
			t.Fatalf("outcome = %+v, want ratification-proof-stale verdict", result.Outcome)
		}
	})

	t.Run("genuine matching proof proposes cleanly", func(t *testing.T) {
		input := ratificationProposalInputFrom(verification, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		result := service.ProposeRatification(context.Background(), human, input)
		if result.Outcome.Classification != ClassificationClean {
			t.Fatalf("outcome = %+v, want clean", result.Outcome)
		}
	})
}

func TestProposeRatificationBindsExactAcceptedResult(t *testing.T) {
	root, service, identity, winnerDigest, inconclusiveDigest := ratifiableService(t)
	signer := newRatificationSigner(t)
	subjects := []string{signer.subject}
	refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
	verification := mintRatificationVerification(t, root, identity, signer, subjects, winnerDigest, experiment.DispositionSelectRecommended, "", "")
	human := humanRatificationIdentity(t, identity, verification.Resolution)
	ratificationPath := filepath.Join(filepath.Dir(mutationDefinitionPath(root)), "ratification.yaml")

	t.Run("malformed digest is operational", func(t *testing.T) {
		input := ratificationProposalInputFrom(verification, "sha256:short", experiment.DispositionSelectRecommended, "", "")
		result := service.ProposeRatification(context.Background(), human, input)
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("outcome = %+v, want operational", result.Outcome)
		}
	})

	t.Run("unknown digest is a verdict", func(t *testing.T) {
		input := ratificationProposalInputFrom(verification, "sha256:"+strings.Repeat("9", 64), experiment.DispositionSelectRecommended, "", "")
		result := service.ProposeRatification(context.Background(), human, input)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-result-unknown" {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
	})

	t.Run("select-recommended against an inconclusive result is a verdict", func(t *testing.T) {
		input := ratificationProposalInputFrom(verification, inconclusiveDigest, experiment.DispositionSelectRecommended, "", "")
		result := service.ProposeRatification(context.Background(), human, input)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-binding-violated" {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
	})

	t.Run("select-other naming the winner is a verdict", func(t *testing.T) {
		input := ratificationProposalInputFrom(verification, winnerDigest, experiment.DispositionSelectOther, "cache", "prefers the winner anyway")
		result := service.ProposeRatification(context.Background(), human, input)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-binding-violated" {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
	})

	t.Run("payload actor text cannot mint authority", func(t *testing.T) {
		// The input carries no actor field at all; the persisted actor block
		// must copy the sealed resolution's exact claim and derived id, and
		// the persisted proof block must copy the sealed retained proof.
		input := ratificationProposalInputFrom(verification, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		result := service.ProposeRatification(context.Background(), human, input)
		if result.Outcome.Classification != ClassificationClean {
			t.Fatalf("outcome = %+v, want clean", result.Outcome)
		}
		raw, err := os.ReadFile(ratificationPath)
		if err != nil {
			t.Fatal(err)
		}
		record, err := experiment.DecodeRatification(raw)
		if err != nil {
			t.Fatal(err)
		}
		if record.Schema != experiment.RatificationSchemaV3 || record.ActorV2 == nil || record.Proof == nil {
			t.Fatalf("proposed record = %+v, want emitted v3 actor+proof blocks", record)
		}
		if record.ActorV2.TrustSource != verification.Resolution.Claim.TrustSource ||
			record.ActorV2.Subject != verification.Resolution.Claim.Subject ||
			record.ActorV2.PrincipalID != string(verification.Resolution.PrincipalID) {
			t.Fatalf("persisted actor block %+v does not copy the sealed resolution claim/id", record.ActorV2)
		}
		if record.Actor != "" {
			t.Fatalf("v3 record must not carry the v1 actor scalar, got %q", record.Actor)
		}
		// Deterministic emission: re-encoding the decoded record reproduces
		// the exact proposed bytes.
		reencoded, err := experiment.EncodeRatification(record)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(reencoded, raw) {
			t.Fatalf("re-encoded bytes differ from proposed bytes:\n%q\n%q", reencoded, raw)
		}
		// Provenance records the propose-ratification mutation.
		provenance, err := os.ReadFile(filepath.Join(filepath.Dir(mutationDefinitionPath(root)), "mutation-provenance.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(provenance), string(experiment.MutationProposeRatification)) {
			t.Fatalf("provenance omits the propose-ratification record:\n%s", provenance)
		}
		if result.ArtifactDigest == "" || result.ProvenanceDigest == "" {
			t.Fatalf("result digests missing: %+v", result)
		}
	})

	t.Run("second proposal is refused", func(t *testing.T) {
		input := ratificationProposalInputFrom(verification, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		result := service.ProposeRatification(context.Background(), human, input)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-already-proposed" {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
	})
}

func TestProposeRatificationRefusesCrossRunDuplicateDigest(t *testing.T) {
	root, service, identity, winnerDigest, _ := ratifiableService(t)
	signer := newRatificationSigner(t)
	subjects := []string{signer.subject}
	refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
	verification := mintRatificationVerification(t, root, identity, signer, subjects, winnerDigest, experiment.DispositionSelectRecommended, "", "")
	human := humanRatificationIdentity(t, identity, verification.Resolution)
	// Copy run-alpha's exact result bytes into run-zeta: two accepted runs
	// now claim the same result identity, which the one state algorithm
	// refuses before any ratification authority can bind.
	experimentDir := filepath.Dir(mutationDefinitionPath(root))
	data, err := os.ReadFile(filepath.Join(experimentDir, "runs", "run-alpha", "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(experimentDir, "runs", "run-zeta", "result.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
	input := ratificationProposalInputFrom(verification, winnerDigest, experiment.DispositionSelectRecommended, "", "")
	result := service.ProposeRatification(context.Background(), human, input)
	if result.Outcome.Classification != ClassificationOperational {
		t.Fatalf("outcome = %+v, want operational duplicate-identity refusal", result.Outcome)
	}
}

func TestAcceptedRatificationResolvesPersistedClaim(t *testing.T) {
	root, service, identity, winnerDigest, _ := ratifiableService(t)
	signer := newRatificationSigner(t)
	subjects := []string{signer.subject}
	refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
	verification := mintRatificationVerification(t, root, identity, signer, subjects, winnerDigest, experiment.DispositionSelectRecommended, "", "")
	human := humanRatificationIdentity(t, identity, verification.Resolution)

	t.Run("no accepted ratification is a verdict", func(t *testing.T) {
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-not-accepted" {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
	})

	proposal := service.ProposeRatification(context.Background(), human, ratificationProposalInputFrom(verification, winnerDigest, experiment.DispositionSelectRecommended, "", ""))
	if proposal.Outcome.Classification != ClassificationClean {
		t.Fatalf("proposal outcome = %+v", proposal.Outcome)
	}

	t.Run("proposed bytes carry no accepted authority", func(t *testing.T) {
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-not-accepted" {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
	})

	// Accept the proposed bytes: the accepted tree now carries them, along
	// with the SAME offline-human governance profile the proof re-verifies
	// against.
	refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)

	t.Run("accepted v3 record re-resolves cleanly", func(t *testing.T) {
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationClean {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
		if result.PrincipalID != verification.Resolution.PrincipalID {
			t.Fatalf("PrincipalID = %q, want %q", result.PrincipalID, verification.Resolution.PrincipalID)
		}
		if result.Ratification.ActorV2 == nil || result.Ratification.ResultDigest != winnerDigest || result.Ratification.Proof == nil {
			t.Fatalf("Ratification = %+v", result.Ratification)
		}
		// Defensive copy: mutating the returned record must not leak into a
		// second resolution.
		result.Ratification.ActorV2.PrincipalID = "principal/offline-human/forged"
		again := service.AcceptedRatification(context.Background(), human)
		if again.Outcome.Classification != ClassificationClean {
			t.Fatalf("second resolution after caller mutation = %+v", again.Outcome)
		}
	})

	t.Run("agent identity may inspect accepted state", func(t *testing.T) {
		result := service.AcceptedRatification(context.Background(), identity)
		if result.Outcome.Classification != ClassificationClean {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
	})

	t.Run("stale accepted head is a verdict", func(t *testing.T) {
		stale := human
		stale.ExpectedAcceptedHEAD = strings.Repeat("d", 40)
		result := service.AcceptedRatification(context.Background(), stale)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "accepted-head-stale" {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
	})
}

func TestAcceptedRatificationRefusesHistoryAndCorruption(t *testing.T) {
	root, service, identity, winnerDigest, _ := ratifiableService(t)
	signer := newRatificationSigner(t)
	subjects := []string{signer.subject}
	refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
	verification := mintRatificationVerification(t, root, identity, signer, subjects, winnerDigest, experiment.DispositionSelectRecommended, "", "")
	human := humanRatificationIdentity(t, identity, verification.Resolution)
	experimentDir := filepath.Dir(mutationDefinitionPath(root))
	writeAccepted := func(t *testing.T, contents []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(experimentDir, "ratification.yaml"), contents, 0o600); err != nil {
			t.Fatal(err)
		}
		refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
	}

	t.Run("accepted v1 record never mints fresh authority", func(t *testing.T) {
		writeAccepted(t, []byte("schema: verdi.experiment-ratification/v1\n"+
			"result_digest: "+winnerDigest+"\n"+
			"actor: principal/github/dXNlci0xMjM\n"+
			"disposition: select-recommended\n"))
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-v1-history" {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
	})

	t.Run("accepted well-formed v2 record is decode-only history (SI-150)", func(t *testing.T) {
		legacy := ratificationResolve(t, ratificationFacts("user-123"))
		writeAccepted(t, ratificationV2Bytes(t, legacy, winnerDigest))
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-v2-history" {
			t.Fatalf("outcome = %+v, want ratification-v2-history verdict", result.Outcome)
		}
	})

	t.Run("v1 actor text never becomes a v2 principal", func(t *testing.T) {
		// A v1-shaped actor scalar under the v2 schema is malformed bytes.
		writeAccepted(t, []byte("schema: verdi.experiment-ratification/v2\n"+
			"result_digest: "+winnerDigest+"\n"+
			"actor: principal/github/dXNlci0xMjM\n"+
			"disposition: select-recommended\n"))
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("outcome = %+v, want operational", result.Outcome)
		}
	})

	t.Run("corrupted accepted bytes are operational", func(t *testing.T) {
		writeAccepted(t, []byte("schema: verdi.experiment-ratification/v2\nnot yaml: [\n"))
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("outcome = %+v, want operational", result.Outcome)
		}
	})

	t.Run("persisted id not derived from persisted claim is operational", func(t *testing.T) {
		otherID, err := governanceprincipal.CanonicalPrincipalID("github", "user-456")
		if err != nil {
			t.Fatal(err)
		}
		writeAccepted(t, []byte("schema: verdi.experiment-ratification/v2\n"+
			"result_digest: "+winnerDigest+"\n"+
			"actor:\n"+
			"  trust_source: github\n"+
			"  subject: \"user-123\"\n"+
			"  principal_id: "+string(otherID)+"\n"+
			"disposition: select-recommended\n"))
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("outcome = %+v, want operational derived-id mismatch", result.Outcome)
		}
	})

	t.Run("tampered signature is a verdict", func(t *testing.T) {
		// Fresh fixture: this and the next subtest plant their own accepted
		// provenance record directly rather than through ProposeRatification,
		// so each needs its own clean pre-ratification provenance chain
		// rather than extending whatever the outer scope's earlier subtests
		// already planted.
		root, service, identity, winnerDigest, _ := ratifiableService(t)
		signer := newRatificationSigner(t)
		subjects := []string{signer.subject}
		refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
		verification := mintRatificationVerification(t, root, identity, signer, subjects, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		human := humanRatificationIdentity(t, identity, verification.Resolution)

		signature, err := verification.Retained.Signature()
		if err != nil {
			t.Fatal(err)
		}
		tampered := append([]byte(nil), signature...)
		tampered[0] ^= 0xFF
		challengeBytes, err := verification.Retained.ChallengeBytes()
		if err != nil {
			t.Fatal(err)
		}
		record := experiment.Ratification{
			Schema: experiment.RatificationSchemaV3, ResultDigest: winnerDigest,
			ActorV2: &experiment.RatificationActor{
				TrustSource: verification.Resolution.Claim.TrustSource, Subject: verification.Resolution.Claim.Subject,
				PrincipalID: string(verification.Resolution.PrincipalID),
			},
			Proof: &experiment.AuthenticationProof{
				Schema:             experiment.HumanProofSchema,
				ChallengeBase64URL: base64.RawURLEncoding.EncodeToString(challengeBytes),
				SignatureBase64URL: base64.RawURLEncoding.EncodeToString(tampered),
			},
			Disposition: experiment.DispositionSelectRecommended,
		}
		encoded, err := experiment.EncodeRatification(record)
		if err != nil {
			t.Fatal(err)
		}
		attribution, err := governanceprincipal.NewPrincipalAttribution(verification.Resolution.PrincipalID)
		if err != nil {
			t.Fatal(err)
		}
		pair := ratificationPairRecord(t, root, encoded, attribution)
		plantAcceptedRatification(t, root, service, encoded, &pair)
		plantRatificationGovernanceProfile(service.git.(*fakeGit), subjects...)
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-proof-unsatisfied" {
			t.Fatalf("outcome = %+v, want ratification-proof-unsatisfied verdict", result.Outcome)
		}
	})

	// F4 (Task 10 correction lane review): this subtest pins VerifyRetained's
	// ZERO-CANDIDATE-KEY arm, not verifyRetainedRatificationProof's
	// current-profile mapping requirement. Its fixture plants ONE accepted
	// governance profile, so the historical tree the retained proof names and
	// the current accepted tree are the SAME bytes: the refusal is issued
	// inside experimenthuman.VerifyRetained (ReasonHumanKeyUnmapped) before
	// ratification.go's current-profile check is ever reached. The genuine
	// historical/current divergence control for that check lives in
	// TestAcceptedRatificationRequiresCurrentProfileMapping.
	t.Run("historical profile mapping no candidate key is a verdict", func(t *testing.T) {
		root, service, identity, winnerDigest, _ := ratifiableService(t)
		signer := newRatificationSigner(t)
		subjects := []string{signer.subject}
		refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
		verification := mintRatificationVerification(t, root, identity, signer, subjects, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		human := humanRatificationIdentity(t, identity, verification.Resolution)

		encoded := ratificationV3Bytes(t, verification, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		attribution, err := governanceprincipal.NewPrincipalAttribution(verification.Resolution.PrincipalID)
		if err != nil {
			t.Fatal(err)
		}
		pair := ratificationPairRecord(t, root, encoded, attribution)
		plantAcceptedRatification(t, root, service, encoded, &pair)
		// The ONLY accepted governance profile now maps nobody: the
		// historical and current profile coincide in this fixture, so the
		// retained proof's own re-verification already refuses (P1: zero
		// candidate keys is a verdict, not an invitation to substitute a
		// different tree).
		plantRatificationGovernanceProfile(service.git.(*fakeGit))
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-proof-unsatisfied" {
			t.Fatalf("outcome = %+v, want ratification-proof-unsatisfied verdict", result.Outcome)
		}
	})
}

// plantAcceptedRatification writes ratification bytes (and, when record is
// non-nil, a chain-valid propose-ratification provenance record) directly
// into the worktree and regenerates the accepted tree — the direct-Git-edit
// shapes the accepted resolver must judge.
func plantAcceptedRatification(t *testing.T, root string, service *Service, encoded []byte, provenance *experiment.ProvenanceRecord) {
	t.Helper()
	experimentDir := filepath.Dir(mutationDefinitionPath(root))
	experimentPath := filepath.ToSlash(filepath.Join(".verdi", "specs", "active", "request-path-spike", "experiments", "request-path-v2"))
	if err := os.WriteFile(filepath.Join(experimentDir, "ratification.yaml"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	if provenance != nil {
		proposed, err := readProposedArtifactFiles(root, experimentPath)
		if err != nil {
			t.Fatal(err)
		}
		_, provenanceFile, err := appendProvenance(proposed, experimentPath, *provenance)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(provenanceFile.path)), provenanceFile.new, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	service.git = gitFromExperimentDir(t, root, "request-path-v2")
}

// ratificationPairRecord builds the exact final provenance record a genuine
// propose-ratification mutation appends, with the given attribution.
func ratificationPairRecord(t *testing.T, root string, encoded []byte, attribution governanceprincipal.Attribution) experiment.ProvenanceRecord {
	t.Helper()
	experimentPath := filepath.ToSlash(filepath.Join(".verdi", "specs", "active", "request-path-spike", "experiments", "request-path-v2"))
	proposed, err := readProposedArtifactFiles(root, experimentPath)
	if err != nil {
		t.Fatal(err)
	}
	ratificationPath := experimentPath + "/ratification.yaml"
	previous := cloneFileMap(proposed)
	delete(previous, ratificationPath)
	previousDigest, err := artifactSetDigest(previous, experimentPath)
	if err != nil {
		t.Fatal(err)
	}
	withRatification := cloneFileMap(previous)
	withRatification[ratificationPath] = encoded
	resultDigest, err := artifactSetDigest(withRatification, experimentPath)
	if err != nil {
		t.Fatal(err)
	}
	return experiment.ProvenanceRecord{
		Schema: experiment.ProvenanceSchema, Experiment: experiment.ProvenanceExperiment{Spike: "spec/request-path-spike", ID: "request-path-v2"},
		Operation: experiment.MutationProposeRatification, PreviousDigest: previousDigest, ResultDigest: resultDigest,
		PolicyDigest:   "sha256:" + strings.Repeat("4", 64),
		PolicyDecision: experiment.ProvenancePolicyDecision{State: experiment.PolicyAllowed, Reasons: []experiment.ProvenancePolicyReason{}},
		Attribution:    attribution, Paths: []string{ratificationPath},
	}
}

// ratificationV2Bytes hand-writes a decode-only-history v2 ratification
// record (SI-150: v2 may still be decoded and described, but
// EncodeRatification now refuses to EMIT anything but v3 — v2 fixture
// bytes must be hand-written, exactly like the existing v1 fixtures in
// this file) directly from a github/forge resolution. v2 never carries a
// retained proof block.
func ratificationV2Bytes(t *testing.T, resolution governanceprincipal.PrincipalResolution, resultDigest string) []byte {
	t.Helper()
	raw := "schema: verdi.experiment-ratification/v2\n" +
		"result_digest: " + resultDigest + "\n" +
		"actor:\n" +
		"  trust_source: " + resolution.Claim.TrustSource + "\n" +
		"  subject: " + strconv.Quote(resolution.Claim.Subject) + "\n" +
		"  principal_id: " + string(resolution.PrincipalID) + "\n" +
		"disposition: select-recommended\n"
	// Prove the hand-written bytes are genuinely well-formed decode-only
	// v2 history before handing them to a caller.
	if _, err := experiment.DecodeRatification([]byte(raw)); err != nil {
		t.Fatalf("hand-written v2 ratification bytes do not decode: %v", err)
	}
	return []byte(raw)
}

func TestProposeRatificationRefusesReceiptlessV1Evidence(t *testing.T) {
	root, service, identity, _, _ := ratifiableService(t)
	signer := newRatificationSigner(t)
	subjects := []string{signer.subject}
	refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
	definitionBytes, err := os.ReadFile(mutationDefinitionPath(root))
	if err != nil {
		t.Fatal(err)
	}
	locked, err := experiment.DecodeDefinition(definitionBytes)
	if err != nil {
		t.Fatal(err)
	}
	v1Digest := writeRatifiableRunV1(t, root, locked, "run-legacy", 40)
	refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
	verification := mintRatificationVerification(t, root, identity, signer, subjects, v1Digest, experiment.DispositionSelectRecommended, "", "")
	human := humanRatificationIdentity(t, identity, verification.Resolution)
	input := ratificationProposalInputFrom(verification, v1Digest, experiment.DispositionSelectRecommended, "", "")
	result := service.ProposeRatification(context.Background(), human, input)
	if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-evidence-v1" {
		t.Fatalf("ProposeRatification(v1 evidence) outcome = %+v, want ratification-evidence-v1 verdict", result.Outcome)
	}
}

func TestAcceptedRatificationRequiresProvenancePair(t *testing.T) {
	setup := func(t *testing.T) (root string, service *Service, human Identity, verification experimenthuman.Verification, winnerDigest string, subjects []string) {
		t.Helper()
		root, service, identity, winnerDigest, _ := ratifiableService(t)
		signer := newRatificationSigner(t)
		subjects = []string{signer.subject}
		refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
		verification = mintRatificationVerification(t, root, identity, signer, subjects, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		human = humanRatificationIdentity(t, identity, verification.Resolution)
		return root, service, human, verification, winnerDigest, subjects
	}

	t.Run("valid record without its final provenance record is not clean", func(t *testing.T) {
		root, service, human, verification, winnerDigest, subjects := setup(t)
		encoded := ratificationV3Bytes(t, verification, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		plantAcceptedRatification(t, root, service, encoded, nil)
		plantRatificationGovernanceProfile(service.git.(*fakeGit), subjects...)
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-provenance-incomplete" {
			t.Fatalf("outcome = %+v, want ratification-provenance-incomplete verdict", result.Outcome)
		}
	})

	t.Run("complete planted pair with authenticated attribution is clean", func(t *testing.T) {
		root, service, human, verification, winnerDigest, subjects := setup(t)
		encoded := ratificationV3Bytes(t, verification, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		attribution, err := governanceprincipal.NewPrincipalAttribution(verification.Resolution.PrincipalID)
		if err != nil {
			t.Fatal(err)
		}
		record := ratificationPairRecord(t, root, encoded, attribution)
		plantAcceptedRatification(t, root, service, encoded, &record)
		plantRatificationGovernanceProfile(service.git.(*fakeGit), subjects...)
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationClean {
			t.Fatalf("outcome = %+v, want clean", result.Outcome)
		}
	})

	t.Run("unauthenticated final attribution is operational", func(t *testing.T) {
		root, service, human, verification, winnerDigest, subjects := setup(t)
		encoded := ratificationV3Bytes(t, verification, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		record := ratificationPairRecord(t, root, encoded, governanceprincipal.NewUnauthenticatedAttribution())
		plantAcceptedRatification(t, root, service, encoded, &record)
		plantRatificationGovernanceProfile(service.git.(*fakeGit), subjects...)
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("outcome = %+v, want operational", result.Outcome)
		}
	})

	t.Run("final attribution principal must match the record actor", func(t *testing.T) {
		root, service, human, verification, winnerDigest, subjects := setup(t)
		encoded := ratificationV3Bytes(t, verification, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		otherID, err := governanceprincipal.CanonicalPrincipalID("github", "user-456")
		if err != nil {
			t.Fatal(err)
		}
		otherAttribution, err := governanceprincipal.NewPrincipalAttribution(otherID)
		if err != nil {
			t.Fatal(err)
		}
		record := ratificationPairRecord(t, root, encoded, otherAttribution)
		plantAcceptedRatification(t, root, service, encoded, &record)
		plantRatificationGovernanceProfile(service.git.(*fakeGit), subjects...)
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("outcome = %+v, want operational", result.Outcome)
		}
	})

	t.Run("final record must name the exact ratification path", func(t *testing.T) {
		root, service, human, verification, winnerDigest, subjects := setup(t)
		encoded := ratificationV3Bytes(t, verification, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		attribution, err := governanceprincipal.NewPrincipalAttribution(verification.Resolution.PrincipalID)
		if err != nil {
			t.Fatal(err)
		}
		record := ratificationPairRecord(t, root, encoded, attribution)
		record.Paths = []string{record.Paths[0] + ".renamed"}
		plantAcceptedRatification(t, root, service, encoded, &record)
		plantRatificationGovernanceProfile(service.git.(*fakeGit), subjects...)
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-provenance-incomplete" {
			t.Fatalf("outcome = %+v, want ratification-provenance-incomplete verdict", result.Outcome)
		}
	})

	t.Run("final record must bind the exact accepted artifact-set digest", func(t *testing.T) {
		root, service, human, verification, winnerDigest, subjects := setup(t)
		encoded := ratificationV3Bytes(t, verification, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		attribution, err := governanceprincipal.NewPrincipalAttribution(verification.Resolution.PrincipalID)
		if err != nil {
			t.Fatal(err)
		}
		record := ratificationPairRecord(t, root, encoded, attribution)
		record.ResultDigest = "sha256:" + strings.Repeat("8", 64)
		plantAcceptedRatification(t, root, service, encoded, &record)
		plantRatificationGovernanceProfile(service.git.(*fakeGit), subjects...)
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-provenance-incomplete" {
			t.Fatalf("outcome = %+v, want ratification-provenance-incomplete verdict", result.Outcome)
		}
	})
}

func TestAcceptedRatificationRefusesReceiptlessV1Evidence(t *testing.T) {
	root, service, identity, _, _ := ratifiableService(t)
	signer := newRatificationSigner(t)
	subjects := []string{signer.subject}
	refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
	verification := mintRatificationVerification(t, root, identity, signer, subjects, "sha256:"+strings.Repeat("1", 64), experiment.DispositionSelectRecommended, "", "")
	human := humanRatificationIdentity(t, identity, verification.Resolution)
	definitionBytes, err := os.ReadFile(mutationDefinitionPath(root))
	if err != nil {
		t.Fatal(err)
	}
	locked, err := experiment.DecodeDefinition(definitionBytes)
	if err != nil {
		t.Fatal(err)
	}
	v1Digest := writeRatifiableRunV1(t, root, locked, "run-legacy", 40)
	encoded := ratificationV3Bytes(t, verification, v1Digest, experiment.DispositionSelectRecommended, "", "")
	attribution, err := governanceprincipal.NewPrincipalAttribution(verification.Resolution.PrincipalID)
	if err != nil {
		t.Fatal(err)
	}
	record := ratificationPairRecord(t, root, encoded, attribution)
	plantAcceptedRatification(t, root, service, encoded, &record)
	plantRatificationGovernanceProfile(service.git.(*fakeGit), subjects...)
	result := service.AcceptedRatification(context.Background(), human)
	if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-evidence-v1" {
		t.Fatalf("outcome = %+v, want ratification-evidence-v1 verdict", result.Outcome)
	}
}

func TestProposeRatificationResolvesAcceptedTreeOnce(t *testing.T) {
	root, service, identity, winnerDigest, _ := ratifiableService(t)
	signer := newRatificationSigner(t)
	subjects := []string{signer.subject}
	refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
	verification := mintRatificationVerification(t, root, identity, signer, subjects, winnerDigest, experiment.DispositionSelectRecommended, "", "")
	human := humanRatificationIdentity(t, identity, verification.Resolution)
	git, ok := service.git.(*fakeGit)
	if !ok {
		t.Fatalf("service.git is %T, want *fakeGit", service.git)
	}
	git.headCalls = 0
	git.treeCalls = nil
	input := ratificationProposalInputFrom(verification, winnerDigest, experiment.DispositionSelectRecommended, "", "")
	result := service.ProposeRatification(context.Background(), human, input)
	if result.Outcome.Classification != ClassificationClean {
		t.Fatalf("ProposeRatification() outcome = %+v", result.Outcome)
	}
	if git.headCalls != 1 || len(git.treeCalls) != 1 {
		t.Fatalf("accepted resolution ran %d HEAD resolutions and %d tree enumerations, want exactly one of each (design §7)", git.headCalls, len(git.treeCalls))
	}
}

// writeRatificationProvenanceLog replaces the worktree provenance log with
// exactly the given sealed records and regenerates the accepted tree.
func writeRatificationProvenanceLog(t *testing.T, root string, service *Service, records []experiment.ProvenanceRecord) {
	t.Helper()
	var combined []byte
	for i := range records {
		if err := records[i].Seal(); err != nil {
			t.Fatal(err)
		}
		encoded, err := experiment.EncodeProvenanceRecord(records[i])
		if err != nil {
			t.Fatal(err)
		}
		combined = append(combined, encoded...)
	}
	experimentDir := filepath.Dir(mutationDefinitionPath(root))
	if err := os.WriteFile(filepath.Join(experimentDir, "mutation-provenance.jsonl"), combined, 0o600); err != nil {
		t.Fatal(err)
	}
	service.git = gitFromExperimentDir(t, root, "request-path-v2")
}

// ratificationChainDigests returns the pre-ratification (preimage) and
// complete accepted mutation-artifact digests for the current worktree with
// encoded ratification bytes installed.
func ratificationChainDigests(t *testing.T, root string, encoded []byte) (preimage, full string) {
	t.Helper()
	experimentPath := filepath.ToSlash(filepath.Join(".verdi", "specs", "active", "request-path-spike", "experiments", "request-path-v2"))
	proposed, err := readProposedArtifactFiles(root, experimentPath)
	if err != nil {
		t.Fatal(err)
	}
	ratificationPath := experimentPath + "/ratification.yaml"
	withoutRatification := cloneFileMap(proposed)
	delete(withoutRatification, ratificationPath)
	preimage, err = artifactSetDigest(withoutRatification, experimentPath)
	if err != nil {
		t.Fatal(err)
	}
	withRatification := cloneFileMap(withoutRatification)
	withRatification[ratificationPath] = encoded
	full, err = artifactSetDigest(withRatification, experimentPath)
	if err != nil {
		t.Fatal(err)
	}
	return preimage, full
}

func ratificationHistoryRecord(operation experiment.MutationOperation, previous, result string, paths []string, attribution governanceprincipal.Attribution) experiment.ProvenanceRecord {
	return experiment.ProvenanceRecord{
		Schema: experiment.ProvenanceSchema, Experiment: experiment.ProvenanceExperiment{Spike: "spec/request-path-spike", ID: "request-path-v2"},
		Operation: operation, PreviousDigest: previous, ResultDigest: result,
		PolicyDigest:   "sha256:" + strings.Repeat("4", 64),
		PolicyDecision: experiment.ProvenancePolicyDecision{State: experiment.PolicyAllowed, Reasons: []experiment.ProvenancePolicyReason{}},
		Attribution:    attribution, Paths: paths,
	}
}

// TestAcceptedRatificationRequiresRegistrationHistory is the adjudicated
// closure regression: a canonical accepted tree whose provenance carries
// ONE authenticated propose-ratification record but NO earlier accepted
// propose-registration record must never return clean — AC-5's two
// temporally distinct human moments (design §3.3).
func TestAcceptedRatificationRequiresRegistrationHistory(t *testing.T) {
	experimentPath := filepath.ToSlash(filepath.Join(".verdi", "specs", "active", "request-path-spike", "experiments", "request-path-v2"))
	registrationPaths := []string{experimentPath + "/evaluator-capabilities.json", experimentPath + "/experiment.yaml"}
	ratificationPaths := []string{experimentPath + "/ratification.yaml"}
	seed := "sha256:" + strings.Repeat("5", 64)

	build := func(t *testing.T) (root string, service *Service, human Identity, registrationPrior, preimage, full string, principal governanceprincipal.PrincipalID, subjects []string) {
		t.Helper()
		root, service, identity, winnerDigest, _ := ratifiableService(t)
		registrationRecords := mutationProvenance(t, root)
		if len(registrationRecords) == 0 || registrationRecords[len(registrationRecords)-1].Operation != experiment.MutationProposeRegistration {
			t.Fatalf("ratifiable fixture registration provenance = %+v", registrationRecords)
		}
		registrationPrior = registrationRecords[len(registrationRecords)-1].PreviousDigest
		signerA := newRatificationSigner(t)
		signerB := newRatificationSigner(t)
		subjects = []string{signerA.subject, signerB.subject}
		refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
		verification := mintRatificationVerification(t, root, identity, signerA, subjects, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		human = humanRatificationIdentity(t, identity, verification.Resolution)
		encoded := ratificationV3Bytes(t, verification, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		experimentDir := filepath.Dir(mutationDefinitionPath(root))
		if err := os.WriteFile(filepath.Join(experimentDir, "ratification.yaml"), encoded, 0o600); err != nil {
			t.Fatal(err)
		}
		preimage, full = ratificationChainDigests(t, root, encoded)
		return root, service, human, registrationPrior, preimage, full, verification.Resolution.PrincipalID, subjects
	}

	t.Run("single propose-ratification record is not clean", func(t *testing.T) {
		root, service, human, _, preimage, full, principal, subjects := build(t)
		attribution, err := governanceprincipal.NewPrincipalAttribution(principal)
		if err != nil {
			t.Fatal(err)
		}
		writeRatificationProvenanceLog(t, root, service, []experiment.ProvenanceRecord{
			ratificationHistoryRecord(experiment.MutationProposeRatification, preimage, full, ratificationPaths, attribution),
		})
		plantRatificationGovernanceProfile(service.git.(*fakeGit), subjects...)
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification == ClassificationClean {
			t.Fatalf("outcome = %+v, want not clean: no accepted propose-registration record precedes the ratification", result.Outcome)
		}
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-provenance-incomplete" {
			t.Fatalf("outcome = %+v, want ratification-provenance-incomplete verdict", result.Outcome)
		}
	})

	t.Run("non-registration predecessor is refused", func(t *testing.T) {
		root, service, human, _, preimage, full, principal, subjects := build(t)
		attribution, err := governanceprincipal.NewPrincipalAttribution(principal)
		if err != nil {
			t.Fatal(err)
		}
		writeRatificationProvenanceLog(t, root, service, []experiment.ProvenanceRecord{
			ratificationHistoryRecord(experiment.MutationReconcileDirect, seed, preimage, []string{experimentPath + "/human-note.txt"}, governanceprincipal.NewUnauthenticatedAttribution()),
			ratificationHistoryRecord(experiment.MutationProposeRatification, preimage, full, ratificationPaths, attribution),
		})
		plantRatificationGovernanceProfile(service.git.(*fakeGit), subjects...)
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-provenance-incomplete" {
			t.Fatalf("outcome = %+v, want ratification-provenance-incomplete verdict", result.Outcome)
		}
	})

	t.Run("chain digests must match the ratification preimage", func(t *testing.T) {
		root, service, human, _, _, full, principal, subjects := build(t)
		attribution, err := governanceprincipal.NewPrincipalAttribution(principal)
		if err != nil {
			t.Fatal(err)
		}
		wrong := "sha256:" + strings.Repeat("6", 64)
		writeRatificationProvenanceLog(t, root, service, []experiment.ProvenanceRecord{
			ratificationHistoryRecord(experiment.MutationProposeRegistration, seed, wrong, registrationPaths, attribution),
			ratificationHistoryRecord(experiment.MutationProposeRatification, wrong, full, ratificationPaths, attribution),
		})
		plantRatificationGovernanceProfile(service.git.(*fakeGit), subjects...)
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-provenance-incomplete" {
			t.Fatalf("outcome = %+v, want ratification-provenance-incomplete verdict", result.Outcome)
		}
	})

	t.Run("registration predecessor must bind the exact pre-registration artifact set", func(t *testing.T) {
		root, service, human, _, preimage, full, principal, subjects := build(t)
		attribution, err := governanceprincipal.NewPrincipalAttribution(principal)
		if err != nil {
			t.Fatal(err)
		}
		writeRatificationProvenanceLog(t, root, service, []experiment.ProvenanceRecord{
			ratificationHistoryRecord(experiment.MutationProposeRegistration, seed, preimage, registrationPaths, attribution),
			ratificationHistoryRecord(experiment.MutationProposeRatification, preimage, full, ratificationPaths, attribution),
		})
		plantRatificationGovernanceProfile(service.git.(*fakeGit), subjects...)
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-provenance-incomplete" {
			t.Fatalf("outcome = %+v, want ratification-provenance-incomplete verdict", result.Outcome)
		}
	})

	t.Run("distinct authenticated principals may register and ratify", func(t *testing.T) {
		root, service, human, registrationPrior, preimage, full, principal, subjects := build(t)
		ratifierAttribution, err := governanceprincipal.NewPrincipalAttribution(principal)
		if err != nil {
			t.Fatal(err)
		}
		registrarID, err := governanceprincipal.CanonicalPrincipalID("github", "user-456")
		if err != nil {
			t.Fatal(err)
		}
		registrarAttribution, err := governanceprincipal.NewPrincipalAttribution(registrarID)
		if err != nil {
			t.Fatal(err)
		}
		writeRatificationProvenanceLog(t, root, service, []experiment.ProvenanceRecord{
			ratificationHistoryRecord(experiment.MutationProposeRegistration, registrationPrior, preimage, registrationPaths, registrarAttribution),
			ratificationHistoryRecord(experiment.MutationProposeRatification, preimage, full, ratificationPaths, ratifierAttribution),
		})
		plantRatificationGovernanceProfile(service.git.(*fakeGit), subjects...)
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationClean {
			t.Fatalf("outcome = %+v, want clean: registration and ratification principals may legitimately differ", result.Outcome)
		}
	})
}

// -----------------------------------------------------------------------
// Task 10 correction lane review F1/F2: INDEPENDENT pins for every
// accepted-use rebinding and identity check in
// verifyRetainedRatificationProof, and for the proposal-time parity and
// identity checks in ProposeRatification. Before these tests the reviewer
// proved by overlay-deletion that each check below could be removed
// without turning any test red.
// -----------------------------------------------------------------------

// acceptedRebindingCase is one accepted-use negative fixture. Every operand
// except the single named divergence is genuine: a real Ed25519 signature
// over a real challenge, minted only through experimenthuman.Verify,
// projected into real v3 accepted bytes carrying a chain-valid
// propose-ratification provenance pair, resolved against a real accepted
// governance profile mapping both fixture signers. Exactly one check may
// therefore refuse, which is what makes each row an independent pin.
type acceptedRebindingCase struct {
	name string
	// mintDisposition is the disposition the CHALLENGE binds. The accepted
	// record always binds select-recommended, so any other value here is
	// exactly a stale canonical typed ratification-input digest.
	mintDisposition experiment.Disposition
	mutateFacts     func(*experimenthuman.ChallengeFacts)
	mutateRecord    func(t *testing.T, signerA, signerB ratificationSigner, record *experiment.Ratification)
	wantClass       Classification
	wantCode        string
}

// buildAcceptedRebindingFixture materializes one acceptedRebindingCase into
// an accepted tree and returns the service and identity to resolve it with.
func buildAcceptedRebindingFixture(t *testing.T, tc acceptedRebindingCase) (*Service, Identity) {
	t.Helper()
	root, service, identity, winnerDigest, _ := ratifiableService(t)
	signerA := newRatificationSigner(t)
	signerB := newRatificationSigner(t)
	subjects := []string{signerA.subject, signerB.subject}
	refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)

	mintDisposition := tc.mintDisposition
	if mintDisposition == "" {
		mintDisposition = experiment.DispositionSelectRecommended
	}
	facts := ratificationChallengeFacts(t, root, identity, winnerDigest, mintDisposition, "", "")
	if tc.mutateFacts != nil {
		tc.mutateFacts(&facts)
	}
	verification := signRatificationChallenge(t, signerA, facts, subjects...)
	human := humanRatificationIdentity(t, identity, verification.Resolution)

	record := buildV3RatificationRecord(t, verification, winnerDigest, experiment.DispositionSelectRecommended, "", "")
	if tc.mutateRecord != nil {
		tc.mutateRecord(t, signerA, signerB, &record)
	}
	encoded, err := experiment.EncodeRatification(record)
	if err != nil {
		t.Fatal(err)
	}
	// The provenance pair always attributes the record's OWN persisted
	// principal, so the earlier provenance-identity check can never be the
	// one that refuses.
	attribution, err := governanceprincipal.NewPrincipalAttribution(governanceprincipal.PrincipalID(record.ActorV2.PrincipalID))
	if err != nil {
		t.Fatal(err)
	}
	pair := ratificationPairRecord(t, root, encoded, attribution)
	plantAcceptedRatification(t, root, service, encoded, &pair)
	plantRatificationGovernanceProfile(service.git.(*fakeGit), subjects...)
	return service, human
}

// setActorClaim rewrites a v3 record's persisted actor block to the given
// claim, deriving the principal id exactly as the kernel does (the wire
// grammar refuses any other value).
func setActorClaim(t *testing.T, record *experiment.Ratification, trustSource, subject string) {
	t.Helper()
	id, err := governanceprincipal.CanonicalPrincipalID(trustSource, subject)
	if err != nil {
		t.Fatal(err)
	}
	record.ActorV2.TrustSource = trustSource
	record.ActorV2.Subject = subject
	record.ActorV2.PrincipalID = string(id)
}

func TestAcceptedRatificationRebindsRetainedProof(t *testing.T) {
	cases := []acceptedRebindingCase{
		{
			// Persisted actor trust source diverges from the signed challenge's
			// own trust source. The historical re-verification still succeeds
			// (it uses the challenge's source), so only the identity check can
			// refuse this record.
			name: "persisted actor trust source is not the challenge trust source",
			mutateRecord: func(t *testing.T, signerA, _ ratificationSigner, record *experiment.Ratification) {
				setActorClaim(t, record, "github", signerA.subject)
			},
			wantClass: ClassificationOperational, wantCode: "ratification-provenance-identity",
		},
		{
			// The proof is signer A's; the record names signer B, who is
			// equally mapped in the accepted profile. Only the re-verified
			// subject — never the persisted text — may name the human.
			name: "persisted actor subject is a second mapped signer",
			mutateRecord: func(t *testing.T, _, signerB ratificationSigner, record *experiment.Ratification) {
				setActorClaim(t, record, "offline-human", signerB.subject)
			},
			wantClass: ClassificationOperational, wantCode: "ratification-provenance-identity",
		},
		{
			name:        "challenge binds a different operation",
			mutateFacts: func(f *experimenthuman.ChallengeFacts) { f.Operation = experimenthuman.OperationProposeRegistration },
			wantClass:   ClassificationOperational, wantCode: "ratification-provenance-identity",
		},
		{
			name:        "challenge binds another experiment id",
			mutateFacts: func(f *experimenthuman.ChallengeFacts) { f.ExperimentID = "request-path-v3" },
			wantClass:   ClassificationOperational, wantCode: "ratification-provenance-identity",
		},
		{
			// STATE rebinding (controller pin P1): a genuine signature over a
			// genuine reject-all challenge cannot authorize the select-recommended
			// bytes the accepted tree actually carries.
			name:            "challenge binds a different disposition",
			mintDisposition: experiment.DispositionRejectAll,
			wantClass:       ClassificationVerdict, wantCode: "ratification-proof-stale",
		},
		{
			name: "challenge binds a fabricated proposal digest",
			mutateFacts: func(f *experimenthuman.ChallengeFacts) {
				f.ProposalDigest = "sha256:" + strings.Repeat("7", 64)
			},
			wantClass: ClassificationVerdict, wantCode: "ratification-proof-stale",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, human := buildAcceptedRebindingFixture(t, tc)
			result := service.AcceptedRatification(context.Background(), human)
			if result.Outcome.Classification != tc.wantClass || result.Outcome.Code != tc.wantCode {
				t.Fatalf("outcome = %+v, want %s/%s", result.Outcome, tc.wantClass, tc.wantCode)
			}
		})
	}

	t.Run("the same fixture without any divergence is clean", func(t *testing.T) {
		// The live control for every row above: the harness itself produces a
		// cleanly resolving accepted ratification, so each refusal above is
		// caused by its own single divergence and not by the fixture.
		service, human := buildAcceptedRebindingFixture(t, acceptedRebindingCase{name: "control"})
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationClean {
			t.Fatalf("control outcome = %+v, want clean", result.Outcome)
		}
	})
}

// headScopedGit wraps a fakeGit with a DIVERGENT tree for exactly one
// designated historical commit: ResolveDefaultBranch and every non-historical
// read delegate to the base fake (the CURRENT accepted tree), while
// ListTree/ReadBlob at the historical commit answer from the divergent
// snapshot. It is the only way to exercise the accepted-use path where the
// retained proof's signed accepted_head names a policy tree that is not the
// current one.
//
// historicalErr additionally makes that ONE commit unresolvable while the
// current accepted HEAD keeps resolving normally — the shape design §11's
// "historical accepted-HEAD unreachability" family and SI-150 option (c)
// require: an unresolvable historical HEAD is refused OPERATIONALLY, never
// by silently substituting the current worktree or current profile.
type headScopedGit struct {
	base          *fakeGit
	historical    string
	historicalErr error
	entries       []GitTreeEntry
	blobs         map[string][]byte
}

func (g *headScopedGit) ResolveDefaultBranch(ctx context.Context, root string) (DefaultBranch, error) {
	return g.base.ResolveDefaultBranch(ctx, root)
}

func (g *headScopedGit) ListTree(ctx context.Context, root, commit string) ([]GitTreeEntry, error) {
	if commit != g.historical {
		return g.base.ListTree(ctx, root, commit)
	}
	if g.historicalErr != nil {
		return nil, g.historicalErr
	}
	return append([]GitTreeEntry(nil), g.entries...), nil
}

func (g *headScopedGit) ReadBlob(ctx context.Context, root, commit, object, path string) ([]byte, error) {
	if commit != g.historical {
		return g.base.ReadBlob(ctx, root, commit, object, path)
	}
	if g.historicalErr != nil {
		return nil, g.historicalErr
	}
	data, ok := g.blobs[path]
	if !ok {
		return nil, fmt.Errorf("missing historical fake blob %s", path)
	}
	return append([]byte(nil), data...), nil
}

// stripHistoricalPolicyTree makes the designated historical commit RESOLVE
// normally while carrying no .verdi/policy/ tree at all — the companion
// negative to an outright unresolvable historical HEAD.
func stripHistoricalPolicyTree(scoped *headScopedGit) {
	const policyPrefix = ".verdi/policy/"
	kept := make([]GitTreeEntry, 0, len(scoped.entries))
	for _, entry := range scoped.entries {
		if !strings.HasPrefix(entry.Path, policyPrefix) {
			kept = append(kept, entry)
		}
	}
	scoped.entries = kept
	for name := range scoped.blobs {
		if strings.HasPrefix(name, policyPrefix) {
			delete(scoped.blobs, name)
		}
	}
}

// newHeadScopedGit snapshots base and overwrites the snapshot's .verdi/policy/
// tree with a profile mapping exactly historicalSubjects, serving those bytes
// only at commit historical.
func newHeadScopedGit(base *fakeGit, historical string, historicalSubjects ...string) *headScopedGit {
	divergent := &fakeGit{revision: base.revision, entries: append([]GitTreeEntry(nil), base.entries...), blobs: map[string][]byte{}}
	for name, data := range base.blobs {
		divergent.blobs[name] = append([]byte(nil), data...)
	}
	plantRatificationGovernanceProfile(divergent, historicalSubjects...)
	return &headScopedGit{base: base, historical: historical, entries: divergent.entries, blobs: divergent.blobs}
}

// plantSourcelessGovernanceProfile installs a CURRENT accepted profile that
// dropped the offline-human trust source entirely. Because
// governanceprincipal/validate.go:117 requires every role mapping's trust
// source to resolve within identity_trust_sources, dropping the source
// necessarily drops the mapping too — there is no decodable profile that
// keeps the mapping without the source.
func plantSourcelessGovernanceProfile(git *fakeGit) {
	constitution, _ := ratificationGovernanceProfileBytes(nil)
	profile := []byte(`---
schema: verdi.governance-profile/v1
id: solo-default
class: solo
applicable_transitions: [accept]
identity_trust_sources: []
role_mappings: []
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
---
Current accepted profile that dropped the offline-human trust source.
`)
	plantGitBlob(git, ".verdi/policy/constitution.md", constitution)
	plantGitBlob(git, ".verdi/policy/profiles/solo-default.md", profile)
}

// TestAcceptedRatificationRequiresCurrentProfileMapping is the F2 divergence
// fixture: the retained proof's signed accepted_head names a HISTORICAL
// policy tree that maps signer A, while the CURRENT accepted tree carries a
// different profile. It is the only test in which the historical and current
// governance profiles genuinely differ, so it is the only one that can reach
// verifyRetainedRatificationProof's current-profile mapping requirement (the
// historical re-verification always succeeds here).
func TestAcceptedRatificationRequiresCurrentProfileMapping(t *testing.T) {
	// currentProfile installs the CURRENT accepted tree's governance profile.
	// plantAcceptedRatification regenerates the fake accepted tree from the
	// worktree alone, so the current policy tree is always (re)planted here
	// explicitly — never inherited from the historical fixture.
	// tuneHistorical (nil for a healthy historical tree) reshapes the wrapper
	// serving the HISTORICAL commit only, so the current accepted HEAD keeps
	// resolving while its predecessor does not.
	build := func(t *testing.T, currentProfile func(git *fakeGit, signer ratificationSigner), tuneHistorical func(*headScopedGit)) (*Service, Identity) {
		t.Helper()
		root, service, identity, winnerDigest, _ := ratifiableService(t)
		signerA := newRatificationSigner(t)
		refreshAcceptedGit(t, service, root, "request-path-v2", signerA.subject)
		// Sign against the historical commit oldHead; the current accepted
		// HEAD stays testHead. design §7 deliberately allows the signed
		// accepted_head to fall behind once later commits land.
		facts := ratificationChallengeFacts(t, root, identity, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		facts.AcceptedHEAD = oldHead
		verification := signRatificationChallenge(t, signerA, facts, signerA.subject)
		human := humanRatificationIdentity(t, identity, verification.Resolution)

		encoded := ratificationV3Bytes(t, verification, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		attribution, err := governanceprincipal.NewPrincipalAttribution(verification.Resolution.PrincipalID)
		if err != nil {
			t.Fatal(err)
		}
		pair := ratificationPairRecord(t, root, encoded, attribution)
		plantAcceptedRatification(t, root, service, encoded, &pair)
		base := service.git.(*fakeGit)
		currentProfile(base, signerA)
		scoped := newHeadScopedGit(base, oldHead, signerA.subject)
		if tuneHistorical != nil {
			tuneHistorical(scoped)
		}
		service.git = scoped
		return service, human
	}

	t.Run("current profile still mapping the subject is clean", func(t *testing.T) {
		// The live control: historical and current trees genuinely diverge in
		// identity (different fake commits, separately served bytes) yet the
		// subject remains mapped, so the accepted use resolves cleanly.
		service, human := build(t, func(git *fakeGit, signer ratificationSigner) {
			plantRatificationGovernanceProfile(git, signer.subject)
		}, nil)
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationClean {
			t.Fatalf("outcome = %+v, want clean", result.Outcome)
		}
	})

	t.Run("subject no longer mapped in the current profile is a verdict", func(t *testing.T) {
		other := newRatificationSigner(t)
		service, human := build(t, func(git *fakeGit, _ ratificationSigner) {
			plantRatificationGovernanceProfile(git, other.subject)
		}, nil)
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-proof-unsatisfied" {
			t.Fatalf("outcome = %+v, want ratification-proof-unsatisfied verdict", result.Outcome)
		}
	})

	t.Run("current profile dropping the trust source is a verdict", func(t *testing.T) {
		// F8's successor. At HEAD this shape reached the kernel and returned
		// an OPERATIONAL ratification-trust-source-missing. It cannot any
		// more: the current-profile mapping requirement refuses first, so the
		// classification is now a VERDICT (ratification-proof-unsatisfied).
		// That is P1-conformant — the record and its proof are each perfectly
		// well-formed, and the accepted profile simply no longer supplies the
		// authority evidence they need, which P1 classifies as unsatisfied
		// authority evidence rather than an internal inconsistency.
		service, human := build(t, func(git *fakeGit, _ ratificationSigner) {
			plantSourcelessGovernanceProfile(git)
		}, nil)
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-proof-unsatisfied" {
			t.Fatalf("outcome = %+v, want ratification-proof-unsatisfied verdict", result.Outcome)
		}
	})

	// design §11's historical accepted-HEAD unreachability family, SI-150
	// option (c): the retained proof names a historical commit the accepted
	// Git port cannot serve. The current accepted HEAD still resolves
	// perfectly — the whole point is that the operation must refuse
	// OPERATIONALLY rather than fall back to the current tree it can read.
	t.Run("unresolvable historical head is operational", func(t *testing.T) {
		service, human := build(t, func(git *fakeGit, signer ratificationSigner) {
			plantRatificationGovernanceProfile(git, signer.subject)
		}, func(scoped *headScopedGit) {
			scoped.historicalErr = fmt.Errorf("fatal: bad object %s", oldHead)
		})
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationOperational || result.Outcome.Code != "ratification-proof-head-unreachable" {
			t.Fatalf("outcome = %+v, want operational ratification-proof-head-unreachable", result.Outcome)
		}
		// The refusal must carry the port's own failure, not a silent
		// substitution of the readable current tree.
		if !strings.Contains(result.Outcome.Detail, oldHead) {
			t.Fatalf("Detail = %q, want the unreadable historical commit named", result.Outcome.Detail)
		}
	})

	t.Run("historical head without a policy tree is operational", func(t *testing.T) {
		// The companion negative: the historical commit RESOLVES, so the
		// head-unreachable arm cannot fire, but it carries no .verdi/policy/
		// tree for the retained proof to re-verify against. The current tree's
		// perfectly good profile must never stand in for it — the refusal
		// arrives from experimenthuman.VerifyRetained's own policy load, still
		// operational (controller pin P1: an unreadable historical policy tree
		// is never a verdict about the human).
		service, human := build(t, func(git *fakeGit, signer ratificationSigner) {
			plantRatificationGovernanceProfile(git, signer.subject)
		}, stripHistoricalPolicyTree)
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationOperational || result.Outcome.Code != "ratification-proof-invalid" {
			t.Fatalf("outcome = %+v, want operational ratification-proof-invalid", result.Outcome)
		}
	})

	t.Run("undecodable current policy tree is operational", func(t *testing.T) {
		// F-3: the HISTORICAL re-verification succeeds (its tree is intact),
		// and only the CURRENT accepted policy authority fails to load. That is
		// corrupted local authority, not a statement about the ratifier, so it
		// is operational and distinct from the mapping verdicts above.
		service, human := build(t, func(git *fakeGit, signer ratificationSigner) {
			plantRatificationGovernanceProfile(git, signer.subject)
			plantGitBlob(git, ".verdi/policy/profiles/solo-default.md", []byte("---\nnot: [a profile\n"))
		}, nil)
		result := service.AcceptedRatification(context.Background(), human)
		if result.Outcome.Classification != ClassificationOperational || result.Outcome.Code != "ratification-authority-invalid" {
			t.Fatalf("outcome = %+v, want operational ratification-authority-invalid", result.Outcome)
		}
	})
}

// TestProposeRatificationBindsProofParityAndIdentity pins each proposal-time
// parity and identity check individually (controller pin P1 symmetry).
//
// The remaining guard — challenge.TrustSource != proofClaim.TrustSource — has
// deliberately no row: it is structurally unreachable from sealed values.
// experimenthuman.verifyWith refuses unless the decoded challenge equals
// NewChallenge(current), and it mints the retained proof's claim with
// TrustSource: current.TrustSource, so a sealed proof's claim trust source is
// always its own challenge's trust source. A proof carrying a genuinely
// different trust source can only come from a different Verify call, and the
// claim-parity check below refuses that first (PrincipalClaim comparison
// covers TrustSource). The guard stays as a fail-closed assertion.
func TestProposeRatificationBindsProofParityAndIdentity(t *testing.T) {
	t.Run("proof claim from another signer is operational", func(t *testing.T) {
		root, service, identity, winnerDigest, _ := ratifiableService(t)
		signerA := newRatificationSigner(t)
		signerB := newRatificationSigner(t)
		subjects := []string{signerA.subject, signerB.subject}
		refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
		first := mintRatificationVerification(t, root, identity, signerA, subjects, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		second := mintRatificationVerification(t, root, identity, signerB, subjects, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		human := humanRatificationIdentity(t, identity, first.Resolution)
		input := ratificationProposalInputFrom(first, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		input.Proof = second.Retained
		result := service.ProposeRatification(context.Background(), human, input)
		if result.Outcome.Classification != ClassificationOperational || result.Outcome.Code != "ratification-proof-mismatch" {
			t.Fatalf("outcome = %+v, want operational ratification-proof-mismatch", result.Outcome)
		}
		// The code alone cannot isolate this leg: a second-signer proof also
		// carries a different SI-147 evidence digest, so the NEXT check would
		// refuse with the same code. The detail is what pins the claim/principal
		// parity check itself.
		if !strings.Contains(result.Outcome.Detail, "claim/principal does not match") {
			t.Fatalf("Detail = %q, want the claim/principal parity refusal", result.Outcome.Detail)
		}
	})

	t.Run("proof evidence digest from another challenge is operational", func(t *testing.T) {
		// The claim-parity leg above cannot fire here: both seals are the SAME
		// signer's, so claim and principal id are identical. Only the SI-147
		// evidence digest differs, because it is taken over the exact challenge
		// and signature bytes — which differ once the two challenges bind
		// different typed ratification inputs. This is the one construction
		// that reaches the evidence-digest check alone.
		root, service, identity, winnerDigest, _ := ratifiableService(t)
		signer := newRatificationSigner(t)
		subjects := []string{signer.subject}
		refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
		first := mintRatificationVerification(t, root, identity, signer, subjects, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		second := mintRatificationVerification(t, root, identity, signer, subjects, winnerDigest, experiment.DispositionRejectAll, "", "")
		if first.Resolution.Claim != second.Resolution.Claim || first.Resolution.PrincipalID != second.Resolution.PrincipalID {
			t.Fatalf("fixture claims/principals differ; this row must isolate the evidence-digest leg")
		}
		human := humanRatificationIdentity(t, identity, first.Resolution)
		input := ratificationProposalInputFrom(first, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		input.Proof = second.Retained
		result := service.ProposeRatification(context.Background(), human, input)
		if result.Outcome.Classification != ClassificationOperational || result.Outcome.Code != "ratification-proof-mismatch" {
			t.Fatalf("outcome = %+v, want operational ratification-proof-mismatch", result.Outcome)
		}
		if !strings.Contains(result.Outcome.Detail, "evidence digest") {
			t.Fatalf("Detail = %q, want the evidence-digest parity refusal", result.Outcome.Detail)
		}
	})

	identityRows := []struct {
		name   string
		mutate func(*experimenthuman.ChallengeFacts)
	}{
		{name: "proof operation is not propose-ratification", mutate: func(f *experimenthuman.ChallengeFacts) {
			f.Operation = experimenthuman.OperationProposeRegistration
		}},
		{name: "proof identity names another experiment", mutate: func(f *experimenthuman.ChallengeFacts) {
			f.ExperimentID = "request-path-v3"
		}},
	}
	for _, row := range identityRows {
		t.Run(row.name+" is operational", func(t *testing.T) {
			root, service, identity, winnerDigest, _ := ratifiableService(t)
			signer := newRatificationSigner(t)
			subjects := []string{signer.subject}
			refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
			facts := ratificationChallengeFacts(t, root, identity, winnerDigest, experiment.DispositionSelectRecommended, "", "")
			row.mutate(&facts)
			// Both seals come from this ONE genuine Verify call, so the parity
			// checks above pass and only the identity check can refuse.
			verification := signRatificationChallenge(t, signer, facts, subjects...)
			human := humanRatificationIdentity(t, identity, verification.Resolution)
			input := ratificationProposalInputFrom(verification, winnerDigest, experiment.DispositionSelectRecommended, "", "")
			result := service.ProposeRatification(context.Background(), human, input)
			if result.Outcome.Classification != ClassificationOperational || result.Outcome.Code != "ratification-proof-mismatch" {
				t.Fatalf("outcome = %+v, want operational ratification-proof-mismatch", result.Outcome)
			}
		})
	}
}
