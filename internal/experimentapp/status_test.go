package experimentapp

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentdecision"
)

func acceptedStatusGit(t *testing.T) *fakeGit {
	t.Helper()
	git := gitFixture(t, "experiment-v1", "request-path-v1")
	definitionPath := acceptedExperimentFilePath("request-path-v1", "experiment.yaml")
	definition, err := experiment.DecodeDefinition(git.blobs[definitionPath])
	if err != nil {
		t.Fatal(err)
	}
	definitionDigest, err := experiment.DefinitionDigest(definition)
	if err != nil {
		t.Fatal(err)
	}
	git.blobs[definitionPath] = appendRegistrationLock(git.blobs[definitionPath], definitionDigest)
	locked, err := experiment.DecodeDefinition(git.blobs[definitionPath])
	if err != nil {
		t.Fatal(err)
	}
	addAcceptedRun(t, git, locked, "run-alpha", 50)
	addAcceptedRun(t, git, locked, "run-zeta", 100)
	return git
}

func addAcceptedRun(t *testing.T, git *fakeGit, definition experiment.Definition, run string, cacheValue int) {
	t.Helper()
	definitionDigest, err := experiment.DefinitionDigest(definition)
	if err != nil {
		t.Fatal(err)
	}
	observations := make([]experiment.Observation, 0, len(definition.Candidates)*definition.Execution.Rounds)
	for _, candidate := range definition.Candidates {
		value := 100
		if candidate.ID == "cache" {
			value = cacheValue
		}
		for round := 1; round <= definition.Execution.Rounds; round++ {
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
	result, err := experimentdecision.Evaluate(definition, observations, experimentdecision.EnvironmentAttestation{PolicyID: definition.Execution.EnvironmentPolicy})
	if err != nil {
		t.Fatal(err)
	}
	resultBytes, err := experiment.EncodeResult(result)
	if err != nil {
		t.Fatal(err)
	}
	setAcceptedExperimentFile(t, git, "request-path-v1", "runs/"+run+"/observations.jsonl", observationBytes.Bytes())
	setAcceptedExperimentFile(t, git, "request-path-v1", "runs/"+run+"/result.json", resultBytes)
}

func TestStatusEnumeratesAllAcceptedRunsWithoutSelectingFavorableEvidence(t *testing.T) {
	git := acceptedStatusGit(t)
	policy := &fakePolicyResolver{decision: resolveTestPolicy(t)}
	service, err := NewService(policy, git, &fakeCapabilities{}, &acceptingVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	status := service.Status(context.Background(), testIdentity(t, t.TempDir(), "request-path-v1"))
	if status.Outcome.Classification != ClassificationVerdict || status.Outcome.Code != "comparison-inconclusive" || status.Outcome.ExitCode() != 1 {
		t.Fatalf("Status() outcome = %+v", status.Outcome)
	}
	if status.State != experiment.StateInconclusive || len(status.Runs) != 2 {
		t.Fatalf("Status() = %+v", status)
	}
	if status.Runs[0].Run != "run-alpha" || status.Runs[0].State != experiment.StateRecommended || status.Runs[1].Run != "run-zeta" || status.Runs[1].State != experiment.StateInconclusive {
		t.Fatalf("Status() runs = %+v", status.Runs)
	}
	if policy.calls != 0 {
		t.Fatalf("Status() policy calls = %d, want no second policy interpretation", policy.calls)
	}
}

func TestExplainIsDeterministicForOneExactResultAndAddsNoRecommendation(t *testing.T) {
	git := acceptedStatusGit(t)
	service, err := NewService(&fakePolicyResolver{decision: resolveTestPolicy(t)}, git, &fakeCapabilities{}, &acceptingVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	identity := testIdentity(t, t.TempDir(), "request-path-v1")
	first := service.Explain(context.Background(), identity, ExplainInput{Run: "run-alpha"})
	second := service.Explain(context.Background(), identity, ExplainInput{Run: "run-alpha"})
	if first.Outcome.Classification != ClassificationClean || first.Decision.Verdict != experiment.VerdictProvenWinner || first.Decision.Winner != "cache" {
		t.Fatalf("Explain(proven) = %+v", first)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("Explain() is nondeterministic\nfirst=%+v\nsecond=%+v", first, second)
	}
	unproven := service.Explain(context.Background(), identity, ExplainInput{Run: "run-zeta"})
	if unproven.Outcome.Classification != ClassificationVerdict || unproven.Outcome.ExitCode() != 1 || unproven.Decision.Verdict != experiment.VerdictDisclosedUnproven || len(unproven.Decision.Reasons) == 0 {
		t.Fatalf("Explain(unproven) = %+v", unproven)
	}
	missing := service.Explain(context.Background(), identity, ExplainInput{Run: "run-missing"})
	if missing.Outcome.Classification != ClassificationVerdict || missing.Outcome.Code != "result-unavailable" {
		t.Fatalf("Explain(missing) = %+v", missing)
	}
}

func TestStatusExplainPreserveMalformedAcceptedEvidenceAsOperational(t *testing.T) {
	git := acceptedStatusGit(t)
	resultPath := acceptedExperimentFilePath("request-path-v1", "runs/run-zeta/result.json")
	git.blobs[resultPath] = []byte("not json\n")
	service, err := NewService(&fakePolicyResolver{decision: resolveTestPolicy(t)}, git, &fakeCapabilities{}, &acceptingVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	identity := testIdentity(t, t.TempDir(), "request-path-v1")
	status := service.Status(context.Background(), identity)
	explained := service.Explain(context.Background(), identity, ExplainInput{Run: "run-zeta"})
	if status.Outcome.Classification != ClassificationOperational || status.Outcome.Code != "state-invalid" || status.Outcome.ExitCode() != 2 {
		t.Fatalf("Status(malformed) = %+v", status.Outcome)
	}
	if explained.Outcome.Classification != ClassificationOperational || explained.Outcome.Code != "state-invalid" || explained.Outcome.ExitCode() != 2 {
		t.Fatalf("Explain(malformed) = %+v", explained.Outcome)
	}
}
