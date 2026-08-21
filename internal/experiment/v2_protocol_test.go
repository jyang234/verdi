package experiment

import (
	"strings"
	"testing"
)

func canonicalCapabilitiesV2() string {
	return `{"evaluator_version":"cache-evaluator/2.1.0","guards":["behavioral-equivalence"],"metrics":[{"direction":"lower","id":"request-latency","type":"duration","unit":"ms"}],"protocol_versions":["verdi.experiment-evaluator/v1","verdi.experiment-observation/v2"],"requires_elevated":false,"requires_network":false,"schema":"verdi.experiment-evaluator-capabilities/v2"}` + "\n"
}

func TestCapabilitiesV2StrictCanonicalProtocol(t *testing.T) {
	c, err := DecodeCapabilities([]byte(canonicalCapabilitiesV2()))
	if err != nil {
		t.Fatalf("DecodeCapabilities(v2): %v", err)
	}
	if c.Schema != CapabilitiesSchemaV2 || c.EvaluatorVersion != "cache-evaluator/2.1.0" {
		t.Fatalf("capabilities = %+v", c)
	}

	tests := []struct {
		name string
		doc  string
	}{
		{"missing evaluator version", strings.Replace(canonicalCapabilitiesV2(), `"evaluator_version":"cache-evaluator/2.1.0",`, "", 1)},
		{"missing evaluator protocol", strings.Replace(canonicalCapabilitiesV2(), `"verdi.experiment-evaluator/v1",`, "", 1)},
		{"missing observation protocol", strings.Replace(canonicalCapabilitiesV2(), `,"verdi.experiment-observation/v2"`, "", 1)},
		{"duplicate key", strings.Replace(canonicalCapabilitiesV2(), `"schema":`, `"schema":"verdi.experiment-evaluator-capabilities/v2","schema":`, 1)},
		{"trailing data", canonicalCapabilitiesV2() + `{}`},
		{"noncanonical whitespace", strings.Replace(canonicalCapabilitiesV2(), `{"evaluator_version"`, `{ "evaluator_version"`, 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeCapabilities([]byte(tt.doc)); err == nil {
				t.Fatalf("DecodeCapabilities() = nil error")
			}
		})
	}
}

func validEvaluatorRequest() EvaluatorRequest {
	return EvaluatorRequest{
		Schema: EvaluatorProtocolSchema, ExperimentDigest: digestOf("a"), Run: "run-1", Candidate: "facts-cache",
		Cycle:    EvaluatorCycle{Kind: CycleMeasured, Number: 1},
		Workload: ResolvedArtifact{ID: "workload", Path: "testdata/workload.json", Digest: digestOf("b")},
		Fixtures: []ResolvedArtifact{},
		Contract: ResolvedArtifact{ID: "contract", Path: "testdata/contract.json", Digest: digestOf("c")},
	}
}

func TestEvaluatorRequestExactCodecAndCycleUnion(t *testing.T) {
	encoded, err := EncodeEvaluatorRequest(validEvaluatorRequest())
	if err != nil {
		t.Fatalf("EncodeEvaluatorRequest(): %v", err)
	}
	if _, err := DecodeEvaluatorRequest(encoded); err != nil {
		t.Fatalf("DecodeEvaluatorRequest(canonical): %v", err)
	}
	for _, mutate := range []func([]byte) []byte{
		func(b []byte) []byte { return append(append([]byte(nil), b...), []byte("{}")...) },
		func(b []byte) []byte { return []byte(strings.Replace(string(b), `{"candidate"`, `{ "candidate"`, 1)) },
		func(b []byte) []byte {
			return []byte(strings.Replace(string(b), `"run":"run-1"`, `"run":"run-1","run":"run-2"`, 1))
		},
	} {
		if _, err := DecodeEvaluatorRequest(mutate(encoded)); err == nil {
			t.Fatalf("DecodeEvaluatorRequest(mutated canonical bytes) = nil error")
		}
	}
	req := validEvaluatorRequest()
	req.Cycle.Number = 0
	if _, err := EncodeEvaluatorRequest(req); err == nil {
		t.Fatalf("EncodeEvaluatorRequest(cycle number zero) = nil error")
	}
}

func TestEvaluatorResponseOutcomePresenceAndCustody(t *testing.T) {
	completed := EvaluatorResponse{Schema: EvaluatorProtocolSchema, Outcome: CandidateOutcome{Kind: OutcomeCompleted}, Guards: []GuardResult{}, Measurements: []Measurement{}, Disclosures: []string{}}
	if _, err := EncodeEvaluatorResponse(completed); err != nil {
		t.Fatalf("EncodeEvaluatorResponse(completed): %v", err)
	}
	witness := "segmentation fault"
	failed := completed
	failed.Outcome = CandidateOutcome{Kind: OutcomeCandidateCrash, Witness: &witness}
	if _, err := EncodeEvaluatorResponse(failed); err != nil {
		t.Fatalf("EncodeEvaluatorResponse(crash): %v", err)
	}

	tests := []struct {
		name string
		res  EvaluatorResponse
	}{
		{"completed witness forbidden", func() EvaluatorResponse { r := completed; r.Outcome.Witness = &witness; return r }()},
		{"failure witness required", func() EvaluatorResponse { r := completed; r.Outcome.Kind = OutcomeCandidateTimeout; return r }()},
		{"failure guards forbidden", func() EvaluatorResponse {
			r := failed
			r.Guards = []GuardResult{{ID: "behavioral-equivalence", Verdict: GuardVerdictPass}}
			return r
		}()},
		{"harness source forbidden", func() EvaluatorResponse {
			r := completed
			r.Measurements = []Measurement{{ID: "metric", Value: NumberValue("1"), Unit: "ms", Source: SourceHarnessMeasured}}
			return r
		}()},
		{"reserved metric forbidden", func() EvaluatorResponse {
			r := completed
			r.Measurements = []Measurement{{ID: EvaluatorWallDurationMetricID, Value: NumberValue("1"), Unit: "ns", Source: SourceEvaluatorMeasured}}
			return r
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := EncodeEvaluatorResponse(tt.res); err == nil {
				t.Fatalf("EncodeEvaluatorResponse() = nil error")
			}
		})
	}
}

func TestObservationV2OutcomeUnionAndCanonicalBytes(t *testing.T) {
	o := Observation{Schema: ObservationSchemaV2, ExperimentDigest: digestOf("a"), Run: "run-1", Candidate: "facts-cache", Round: 1,
		Outcome: &CandidateOutcome{Kind: OutcomeCompleted}, Guards: []GuardResult{}, Measurements: []Measurement{}, Disclosures: []string{}}
	b, err := EncodeObservation(o)
	if err != nil {
		t.Fatalf("EncodeObservation(v2): %v", err)
	}
	if _, err := DecodeObservation(b); err != nil {
		t.Fatalf("DecodeObservation(v2): %v", err)
	}
	without := o
	without.Outcome = nil
	if _, err := EncodeObservation(without); err == nil {
		t.Fatalf("EncodeObservation(v2 without outcome) = nil error")
	}
	w := "timeout"
	failed := o
	failed.Outcome = &CandidateOutcome{Kind: OutcomeCandidateTimeout, Witness: &w}
	failed.Guards = nil
	failed.Measurements = nil
	if _, err := EncodeObservation(failed); err != nil {
		t.Fatalf("EncodeObservation(timeout): %v", err)
	}
	if _, err := DecodeObservation([]byte(strings.Replace(string(b), `{"candidate"`, `{ "candidate"`, 1))); err == nil {
		t.Fatalf("DecodeObservation(noncanonical v2) = nil error")
	}
}

func TestDefinitionCapabilitiesMembership(t *testing.T) {
	def := mustDecodeDefinition(t, validDefinitionYAML())
	caps, err := DecodeCapabilities([]byte(canonicalCapabilitiesV2()))
	if err != nil {
		t.Fatal(err)
	}
	caps.Guards = []string{"behavioral-equivalence", "tenant-isolation", "invalidation-deadline"}
	caps.Metrics = append(caps.Metrics, CapabilityMetric{ID: "peak-rss", Type: MetricBytes, Unit: "MiB", Direction: DirectionLower})
	if err := ValidateDefinitionCapabilities(def, caps); err != nil {
		t.Fatalf("ValidateDefinitionCapabilities(): %v", err)
	}
	caps.Guards = []string{"behavioral-equivalence"}
	if err := ValidateDefinitionCapabilities(def, caps); err == nil {
		t.Fatalf("ValidateDefinitionCapabilities(missing guard) = nil error")
	}
}

func TestRunPathsAndWorkspaceRunID(t *testing.T) {
	paths, err := PathsForRun("experiments/cache-placement-v1", "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if paths.Execution != "experiments/cache-placement-v1/runs/run-1/execution.json" || paths.Observations != "experiments/cache-placement-v1/runs/run-1/observations.jsonl" || paths.Result != "experiments/cache-placement-v1/runs/run-1/result.json" {
		t.Fatalf("paths = %+v", paths)
	}
	id1, err := WorkspaceRunID(digestOf("a"), "run-1", "facts-cache")
	if err != nil {
		t.Fatal(err)
	}
	id2, _ := WorkspaceRunID(digestOf("a"), "run-2", "facts-cache")
	if len(id1) != 64 || id1 == id2 {
		t.Fatalf("workspace ids = %q, %q", id1, id2)
	}
}
