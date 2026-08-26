package experimentapp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentdecision"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// ratificationProfile is the sealed accepted governance profile the
// ratification tests resolve against: one forge identity trust source.
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
// sealed kernel resolution through the given fact reader.
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
	return experiment.ExecutionReceipt{
		Schema: experiment.ExecutionReceiptSchema, ExperimentDigest: definitionDigest, Run: run,
		EnvironmentPolicy: def.Execution.EnvironmentPolicy,
		AuthorityDigest:   "sha256:" + strings.Repeat("1", 64), CapabilitiesDigest: def.Evaluator.CapabilitiesDigest,
		ScheduleDigest: "sha256:" + strings.Repeat("2", 64), GrantsDigest: "sha256:" + strings.Repeat("3", 64),
		Fingerprint: experiment.ExecutionFingerprint{OS: "linux", Arch: "amd64", ToolVersions: map[string]string{"evaluator": "2.1.0", "verdi": "0.1.0"}, Env: map[string]*string{}, InputDigests: map[string]string{
			"inputs/workload.txt": strings.TrimPrefix(def.Workload.Digest, "sha256:"),
		}},
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

func TestProposeRatificationBindsExactAcceptedResult(t *testing.T) {
	root, service, identity, winnerDigest, inconclusiveDigest := ratifiableService(t)
	resolution := ratificationResolve(t, ratificationFacts("user-123"))
	human := humanRatificationIdentity(t, identity, resolution)
	ratificationPath := filepath.Join(filepath.Dir(mutationDefinitionPath(root)), "ratification.yaml")

	t.Run("malformed digest is operational", func(t *testing.T) {
		result := service.ProposeRatification(context.Background(), human, RatificationProposalInput{
			ResultDigest: "sha256:short", Disposition: experiment.DispositionSelectRecommended, Resolution: resolution,
		})
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("outcome = %+v, want operational", result.Outcome)
		}
	})

	t.Run("unknown digest is a verdict", func(t *testing.T) {
		result := service.ProposeRatification(context.Background(), human, RatificationProposalInput{
			ResultDigest: "sha256:" + strings.Repeat("9", 64), Disposition: experiment.DispositionSelectRecommended, Resolution: resolution,
		})
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-result-unknown" {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
	})

	t.Run("select-recommended against an inconclusive result is a verdict", func(t *testing.T) {
		result := service.ProposeRatification(context.Background(), human, RatificationProposalInput{
			ResultDigest: inconclusiveDigest, Disposition: experiment.DispositionSelectRecommended, Resolution: resolution,
		})
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-binding-violated" {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
	})

	t.Run("select-other naming the winner is a verdict", func(t *testing.T) {
		result := service.ProposeRatification(context.Background(), human, RatificationProposalInput{
			ResultDigest: winnerDigest, Disposition: experiment.DispositionSelectOther,
			Candidate: "cache", Reason: "prefers the winner anyway", Resolution: resolution,
		})
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-binding-violated" {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
	})

	t.Run("payload actor text cannot mint authority", func(t *testing.T) {
		// The input carries no actor field at all; the persisted actor block
		// must copy the sealed resolution's exact claim and derived id.
		result := service.ProposeRatification(context.Background(), human, RatificationProposalInput{
			ResultDigest: winnerDigest, Disposition: experiment.DispositionSelectRecommended, Resolution: resolution,
		})
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
		if record.Schema != experiment.RatificationSchemaV2 || record.ActorV2 == nil {
			t.Fatalf("proposed record = %+v, want emitted v2 actor block", record)
		}
		if record.ActorV2.TrustSource != resolution.Claim.TrustSource ||
			record.ActorV2.Subject != resolution.Claim.Subject ||
			record.ActorV2.PrincipalID != string(resolution.PrincipalID) {
			t.Fatalf("persisted actor block %+v does not copy the sealed resolution claim/id", record.ActorV2)
		}
		if record.Actor != "" {
			t.Fatalf("v2 record must not carry the v1 actor scalar, got %q", record.Actor)
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
		result := service.ProposeRatification(context.Background(), human, RatificationProposalInput{
			ResultDigest: winnerDigest, Disposition: experiment.DispositionSelectRecommended, Resolution: resolution,
		})
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-already-proposed" {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
	})
}

func TestProposeRatificationRefusesCrossRunDuplicateDigest(t *testing.T) {
	root, service, identity, winnerDigest, _ := ratifiableService(t)
	resolution := ratificationResolve(t, ratificationFacts("user-123"))
	human := humanRatificationIdentity(t, identity, resolution)
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
	service.git = gitFromExperimentDir(t, root, "request-path-v2")
	result := service.ProposeRatification(context.Background(), human, RatificationProposalInput{
		ResultDigest: winnerDigest, Disposition: experiment.DispositionSelectRecommended, Resolution: resolution,
	})
	if result.Outcome.Classification != ClassificationOperational {
		t.Fatalf("outcome = %+v, want operational duplicate-identity refusal", result.Outcome)
	}
}

func TestAcceptedRatificationResolvesPersistedClaim(t *testing.T) {
	root, service, identity, winnerDigest, _ := ratifiableService(t)
	resolution := ratificationResolve(t, ratificationFacts("user-123"))
	human := humanRatificationIdentity(t, identity, resolution)
	authority := AcceptedRatificationAuthority{Profile: ratificationProfile(t, "github"), Facts: ratificationFacts("user-123")}

	t.Run("no accepted ratification is a verdict", func(t *testing.T) {
		result := service.AcceptedRatification(context.Background(), human, authority)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-not-accepted" {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
	})

	proposal := service.ProposeRatification(context.Background(), human, RatificationProposalInput{
		ResultDigest: winnerDigest, Disposition: experiment.DispositionSelectRecommended, Resolution: resolution,
	})
	if proposal.Outcome.Classification != ClassificationClean {
		t.Fatalf("proposal outcome = %+v", proposal.Outcome)
	}

	t.Run("proposed bytes carry no accepted authority", func(t *testing.T) {
		result := service.AcceptedRatification(context.Background(), human, authority)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-not-accepted" {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
	})

	// Accept the proposed bytes: the accepted tree now carries them.
	service.git = gitFromExperimentDir(t, root, "request-path-v2")

	t.Run("accepted v2 record re-resolves cleanly", func(t *testing.T) {
		result := service.AcceptedRatification(context.Background(), human, authority)
		if result.Outcome.Classification != ClassificationClean {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
		if result.PrincipalID != resolution.PrincipalID {
			t.Fatalf("PrincipalID = %q, want %q", result.PrincipalID, resolution.PrincipalID)
		}
		if result.Ratification.ActorV2 == nil || result.Ratification.ResultDigest != winnerDigest {
			t.Fatalf("Ratification = %+v", result.Ratification)
		}
		// Defensive copy: mutating the returned record must not leak into a
		// second resolution.
		result.Ratification.ActorV2.PrincipalID = "principal/github/forged"
		again := service.AcceptedRatification(context.Background(), human, authority)
		if again.Outcome.Classification != ClassificationClean {
			t.Fatalf("second resolution after caller mutation = %+v", again.Outcome)
		}
	})

	t.Run("agent identity may inspect accepted state", func(t *testing.T) {
		result := service.AcceptedRatification(context.Background(), identity, authority)
		if result.Outcome.Classification != ClassificationClean {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
	})

	t.Run("unproven re-resolution is a verdict", func(t *testing.T) {
		result := service.AcceptedRatification(context.Background(), human, AcceptedRatificationAuthority{
			Profile: ratificationProfile(t, "github"), Facts: ratificationUnavailableFacts(),
		})
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-actor-unauthenticated" {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
	})

	t.Run("violated re-resolution is a verdict", func(t *testing.T) {
		result := service.AcceptedRatification(context.Background(), human, AcceptedRatificationAuthority{
			Profile: ratificationProfile(t, "github"), Facts: ratificationFacts("someone-else"),
		})
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-actor-unauthenticated" {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
	})

	t.Run("missing configured trust source is operational", func(t *testing.T) {
		result := service.AcceptedRatification(context.Background(), human, AcceptedRatificationAuthority{
			Profile: ratificationProfile(t, "gitlab"), Facts: ratificationFacts("user-123"),
		})
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("outcome = %+v, want operational missing-trust-source refusal", result.Outcome)
		}
	})

	t.Run("nil fact reader is operational", func(t *testing.T) {
		result := service.AcceptedRatification(context.Background(), human, AcceptedRatificationAuthority{
			Profile: ratificationProfile(t, "github"),
		})
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("outcome = %+v, want operational", result.Outcome)
		}
	})

	t.Run("stale accepted head is a verdict", func(t *testing.T) {
		stale := human
		stale.ExpectedAcceptedHEAD = strings.Repeat("d", 40)
		result := service.AcceptedRatification(context.Background(), stale, authority)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "accepted-head-stale" {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
	})
}

func TestAcceptedRatificationRefusesHistoryAndCorruption(t *testing.T) {
	root, service, identity, winnerDigest, _ := ratifiableService(t)
	resolution := ratificationResolve(t, ratificationFacts("user-123"))
	human := humanRatificationIdentity(t, identity, resolution)
	authority := AcceptedRatificationAuthority{Profile: ratificationProfile(t, "github"), Facts: ratificationFacts("user-123")}
	experimentDir := filepath.Dir(mutationDefinitionPath(root))
	writeAccepted := func(t *testing.T, contents string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(experimentDir, "ratification.yaml"), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		service.git = gitFromExperimentDir(t, root, "request-path-v2")
	}

	t.Run("accepted v1 record never mints fresh authority", func(t *testing.T) {
		writeAccepted(t, "schema: verdi.experiment-ratification/v1\n"+
			"result_digest: "+winnerDigest+"\n"+
			"actor: principal/github/dXNlci0xMjM\n"+
			"disposition: select-recommended\n")
		result := service.AcceptedRatification(context.Background(), human, authority)
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-v1-history" {
			t.Fatalf("outcome = %+v", result.Outcome)
		}
	})

	t.Run("v1 actor text never becomes a v2 principal", func(t *testing.T) {
		// A v1-shaped actor scalar under the v2 schema is malformed bytes.
		writeAccepted(t, "schema: verdi.experiment-ratification/v2\n"+
			"result_digest: "+winnerDigest+"\n"+
			"actor: principal/github/dXNlci0xMjM\n"+
			"disposition: select-recommended\n")
		result := service.AcceptedRatification(context.Background(), human, authority)
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("outcome = %+v, want operational", result.Outcome)
		}
	})

	t.Run("corrupted accepted bytes are operational", func(t *testing.T) {
		writeAccepted(t, "schema: verdi.experiment-ratification/v2\nnot yaml: [\n")
		result := service.AcceptedRatification(context.Background(), human, authority)
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("outcome = %+v, want operational", result.Outcome)
		}
	})

	t.Run("persisted id not derived from persisted claim is operational", func(t *testing.T) {
		otherID, err := governanceprincipal.CanonicalPrincipalID("github", "user-456")
		if err != nil {
			t.Fatal(err)
		}
		writeAccepted(t, "schema: verdi.experiment-ratification/v2\n"+
			"result_digest: "+winnerDigest+"\n"+
			"actor:\n"+
			"  trust_source: github\n"+
			"  subject: \"user-123\"\n"+
			"  principal_id: "+string(otherID)+"\n"+
			"disposition: select-recommended\n")
		result := service.AcceptedRatification(context.Background(), human, authority)
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("outcome = %+v, want operational derived-id mismatch", result.Outcome)
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

func ratificationV2Bytes(t *testing.T, resolution governanceprincipal.PrincipalResolution, resultDigest string) []byte {
	t.Helper()
	encoded, err := experiment.EncodeRatification(experiment.Ratification{
		Schema: experiment.RatificationSchemaV2, ResultDigest: resultDigest,
		ActorV2: &experiment.RatificationActor{
			TrustSource: resolution.Claim.TrustSource, Subject: resolution.Claim.Subject,
			PrincipalID: string(resolution.PrincipalID),
		},
		Disposition: experiment.DispositionSelectRecommended,
	})
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestProposeRatificationRefusesReceiptlessV1Evidence(t *testing.T) {
	root, service, identity, _, _ := ratifiableService(t)
	resolution := ratificationResolve(t, ratificationFacts("user-123"))
	human := humanRatificationIdentity(t, identity, resolution)
	definitionBytes, err := os.ReadFile(mutationDefinitionPath(root))
	if err != nil {
		t.Fatal(err)
	}
	locked, err := experiment.DecodeDefinition(definitionBytes)
	if err != nil {
		t.Fatal(err)
	}
	v1Digest := writeRatifiableRunV1(t, root, locked, "run-legacy", 40)
	service.git = gitFromExperimentDir(t, root, "request-path-v2")
	result := service.ProposeRatification(context.Background(), human, RatificationProposalInput{
		ResultDigest: v1Digest, Disposition: experiment.DispositionSelectRecommended, Resolution: resolution,
	})
	if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-evidence-v1" {
		t.Fatalf("ProposeRatification(v1 evidence) outcome = %+v, want ratification-evidence-v1 verdict", result.Outcome)
	}
}

func TestAcceptedRatificationRequiresProvenancePair(t *testing.T) {
	authorityFor := func(t *testing.T) AcceptedRatificationAuthority {
		return AcceptedRatificationAuthority{Profile: ratificationProfile(t, "github"), Facts: ratificationFacts("user-123")}
	}

	t.Run("valid record without its final provenance record is not clean", func(t *testing.T) {
		root, service, identity, winnerDigest, _ := ratifiableService(t)
		resolution := ratificationResolve(t, ratificationFacts("user-123"))
		human := humanRatificationIdentity(t, identity, resolution)
		plantAcceptedRatification(t, root, service, ratificationV2Bytes(t, resolution, winnerDigest), nil)
		result := service.AcceptedRatification(context.Background(), human, authorityFor(t))
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-provenance-incomplete" {
			t.Fatalf("outcome = %+v, want ratification-provenance-incomplete verdict", result.Outcome)
		}
	})

	t.Run("complete planted pair with authenticated attribution is clean", func(t *testing.T) {
		root, service, identity, winnerDigest, _ := ratifiableService(t)
		resolution := ratificationResolve(t, ratificationFacts("user-123"))
		human := humanRatificationIdentity(t, identity, resolution)
		encoded := ratificationV2Bytes(t, resolution, winnerDigest)
		attribution, err := governanceprincipal.NewPrincipalAttribution(resolution.PrincipalID)
		if err != nil {
			t.Fatal(err)
		}
		record := ratificationPairRecord(t, root, encoded, attribution)
		plantAcceptedRatification(t, root, service, encoded, &record)
		result := service.AcceptedRatification(context.Background(), human, authorityFor(t))
		if result.Outcome.Classification != ClassificationClean {
			t.Fatalf("outcome = %+v, want clean", result.Outcome)
		}
	})

	t.Run("unauthenticated final attribution is operational", func(t *testing.T) {
		root, service, identity, winnerDigest, _ := ratifiableService(t)
		resolution := ratificationResolve(t, ratificationFacts("user-123"))
		human := humanRatificationIdentity(t, identity, resolution)
		encoded := ratificationV2Bytes(t, resolution, winnerDigest)
		record := ratificationPairRecord(t, root, encoded, governanceprincipal.NewUnauthenticatedAttribution())
		plantAcceptedRatification(t, root, service, encoded, &record)
		result := service.AcceptedRatification(context.Background(), human, authorityFor(t))
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("outcome = %+v, want operational", result.Outcome)
		}
	})

	t.Run("final attribution principal must match the record actor", func(t *testing.T) {
		root, service, identity, winnerDigest, _ := ratifiableService(t)
		resolution := ratificationResolve(t, ratificationFacts("user-123"))
		human := humanRatificationIdentity(t, identity, resolution)
		encoded := ratificationV2Bytes(t, resolution, winnerDigest)
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
		result := service.AcceptedRatification(context.Background(), human, authorityFor(t))
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("outcome = %+v, want operational", result.Outcome)
		}
	})

	t.Run("final record must name the exact ratification path", func(t *testing.T) {
		root, service, identity, winnerDigest, _ := ratifiableService(t)
		resolution := ratificationResolve(t, ratificationFacts("user-123"))
		human := humanRatificationIdentity(t, identity, resolution)
		encoded := ratificationV2Bytes(t, resolution, winnerDigest)
		attribution, err := governanceprincipal.NewPrincipalAttribution(resolution.PrincipalID)
		if err != nil {
			t.Fatal(err)
		}
		record := ratificationPairRecord(t, root, encoded, attribution)
		record.Paths = []string{record.Paths[0] + ".renamed"}
		plantAcceptedRatification(t, root, service, encoded, &record)
		result := service.AcceptedRatification(context.Background(), human, authorityFor(t))
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-provenance-incomplete" {
			t.Fatalf("outcome = %+v, want ratification-provenance-incomplete verdict", result.Outcome)
		}
	})

	t.Run("final record must bind the exact accepted artifact-set digest", func(t *testing.T) {
		root, service, identity, winnerDigest, _ := ratifiableService(t)
		resolution := ratificationResolve(t, ratificationFacts("user-123"))
		human := humanRatificationIdentity(t, identity, resolution)
		encoded := ratificationV2Bytes(t, resolution, winnerDigest)
		attribution, err := governanceprincipal.NewPrincipalAttribution(resolution.PrincipalID)
		if err != nil {
			t.Fatal(err)
		}
		record := ratificationPairRecord(t, root, encoded, attribution)
		record.ResultDigest = "sha256:" + strings.Repeat("8", 64)
		plantAcceptedRatification(t, root, service, encoded, &record)
		result := service.AcceptedRatification(context.Background(), human, authorityFor(t))
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-provenance-incomplete" {
			t.Fatalf("outcome = %+v, want ratification-provenance-incomplete verdict", result.Outcome)
		}
	})
}

func TestAcceptedRatificationRefusesReceiptlessV1Evidence(t *testing.T) {
	root, service, identity, _, _ := ratifiableService(t)
	resolution := ratificationResolve(t, ratificationFacts("user-123"))
	human := humanRatificationIdentity(t, identity, resolution)
	definitionBytes, err := os.ReadFile(mutationDefinitionPath(root))
	if err != nil {
		t.Fatal(err)
	}
	locked, err := experiment.DecodeDefinition(definitionBytes)
	if err != nil {
		t.Fatal(err)
	}
	v1Digest := writeRatifiableRunV1(t, root, locked, "run-legacy", 40)
	encoded := ratificationV2Bytes(t, resolution, v1Digest)
	attribution, err := governanceprincipal.NewPrincipalAttribution(resolution.PrincipalID)
	if err != nil {
		t.Fatal(err)
	}
	record := ratificationPairRecord(t, root, encoded, attribution)
	plantAcceptedRatification(t, root, service, encoded, &record)
	result := service.AcceptedRatification(context.Background(), human, AcceptedRatificationAuthority{
		Profile: ratificationProfile(t, "github"), Facts: ratificationFacts("user-123"),
	})
	if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-evidence-v1" {
		t.Fatalf("outcome = %+v, want ratification-evidence-v1 verdict", result.Outcome)
	}
}

func TestProposeRatificationResolvesAcceptedTreeOnce(t *testing.T) {
	_, service, identity, winnerDigest, _ := ratifiableService(t)
	resolution := ratificationResolve(t, ratificationFacts("user-123"))
	human := humanRatificationIdentity(t, identity, resolution)
	git, ok := service.git.(*fakeGit)
	if !ok {
		t.Fatalf("service.git is %T, want *fakeGit", service.git)
	}
	git.headCalls = 0
	git.treeCalls = nil
	result := service.ProposeRatification(context.Background(), human, RatificationProposalInput{
		ResultDigest: winnerDigest, Disposition: experiment.DispositionSelectRecommended, Resolution: resolution,
	})
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
	authority := func(t *testing.T) AcceptedRatificationAuthority {
		return AcceptedRatificationAuthority{Profile: ratificationProfile(t, "github"), Facts: ratificationFacts("user-123", "user-456")}
	}
	build := func(t *testing.T) (root string, service *Service, human Identity, encoded []byte, preimage, full string, principal governanceprincipal.PrincipalID) {
		t.Helper()
		root, service, identity, winnerDigest, _ := ratifiableService(t)
		resolution := ratificationResolve(t, ratificationFacts("user-123", "user-456"))
		human = humanRatificationIdentity(t, identity, resolution)
		encoded = ratificationV2Bytes(t, resolution, winnerDigest)
		experimentDir := filepath.Dir(mutationDefinitionPath(root))
		if err := os.WriteFile(filepath.Join(experimentDir, "ratification.yaml"), encoded, 0o600); err != nil {
			t.Fatal(err)
		}
		preimage, full = ratificationChainDigests(t, root, encoded)
		return root, service, human, encoded, preimage, full, resolution.PrincipalID
	}

	t.Run("single propose-ratification record is not clean", func(t *testing.T) {
		root, service, human, _, preimage, full, principal := build(t)
		attribution, err := governanceprincipal.NewPrincipalAttribution(principal)
		if err != nil {
			t.Fatal(err)
		}
		writeRatificationProvenanceLog(t, root, service, []experiment.ProvenanceRecord{
			ratificationHistoryRecord(experiment.MutationProposeRatification, preimage, full, ratificationPaths, attribution),
		})
		result := service.AcceptedRatification(context.Background(), human, authority(t))
		if result.Outcome.Classification == ClassificationClean {
			t.Fatalf("outcome = %+v, want not clean: no accepted propose-registration record precedes the ratification", result.Outcome)
		}
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-provenance-incomplete" {
			t.Fatalf("outcome = %+v, want ratification-provenance-incomplete verdict", result.Outcome)
		}
	})

	t.Run("non-registration predecessor is refused", func(t *testing.T) {
		root, service, human, _, preimage, full, principal := build(t)
		attribution, err := governanceprincipal.NewPrincipalAttribution(principal)
		if err != nil {
			t.Fatal(err)
		}
		writeRatificationProvenanceLog(t, root, service, []experiment.ProvenanceRecord{
			ratificationHistoryRecord(experiment.MutationReconcileDirect, seed, preimage, []string{experimentPath + "/human-note.txt"}, governanceprincipal.NewUnauthenticatedAttribution()),
			ratificationHistoryRecord(experiment.MutationProposeRatification, preimage, full, ratificationPaths, attribution),
		})
		result := service.AcceptedRatification(context.Background(), human, authority(t))
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-provenance-incomplete" {
			t.Fatalf("outcome = %+v, want ratification-provenance-incomplete verdict", result.Outcome)
		}
	})

	t.Run("chain digests must match the ratification preimage", func(t *testing.T) {
		root, service, human, _, _, full, principal := build(t)
		attribution, err := governanceprincipal.NewPrincipalAttribution(principal)
		if err != nil {
			t.Fatal(err)
		}
		wrong := "sha256:" + strings.Repeat("6", 64)
		writeRatificationProvenanceLog(t, root, service, []experiment.ProvenanceRecord{
			ratificationHistoryRecord(experiment.MutationProposeRegistration, seed, wrong, registrationPaths, attribution),
			ratificationHistoryRecord(experiment.MutationProposeRatification, wrong, full, ratificationPaths, attribution),
		})
		result := service.AcceptedRatification(context.Background(), human, authority(t))
		if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-provenance-incomplete" {
			t.Fatalf("outcome = %+v, want ratification-provenance-incomplete verdict", result.Outcome)
		}
	})

	t.Run("distinct authenticated principals may register and ratify", func(t *testing.T) {
		root, service, human, _, preimage, full, principal := build(t)
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
			ratificationHistoryRecord(experiment.MutationProposeRegistration, seed, preimage, registrationPaths, registrarAttribution),
			ratificationHistoryRecord(experiment.MutationProposeRatification, preimage, full, ratificationPaths, ratifierAttribution),
		})
		result := service.AcceptedRatification(context.Background(), human, authority(t))
		if result.Outcome.Classification != ClassificationClean {
			t.Fatalf("outcome = %+v, want clean: registration and ratification principals may legitimately differ", result.Outcome)
		}
	})
}
