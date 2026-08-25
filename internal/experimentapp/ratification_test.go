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

// writeRatifiableRun materializes one complete accepted run (v1
// observations plus v1 result — decode-only run evidence needs no
// receipt) under the worktree experiment directory and returns the exact
// result digest.
func writeRatifiableRun(t *testing.T, root string, def experiment.Definition, run string, cacheValue int) string {
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
	var observationBytes bytes.Buffer
	for _, observation := range observations {
		line, err := experiment.EncodeObservation(observation)
		if err != nil {
			t.Fatal(err)
		}
		observationBytes.Write(line)
	}
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
	if err := os.WriteFile(filepath.Join(runDir, "observations.jsonl"), observationBytes.Bytes(), 0o600); err != nil {
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
