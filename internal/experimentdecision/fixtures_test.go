package experimentdecision

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
)

// base40 is a well-formed 40-hex commit value shared by every in-memory
// definition fixture in this package's tests.
const base40 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// fixtureDigest returns a well-formed "sha256:<64 hex>" digest built by
// repeating char, distinct fixture digests differing only in which
// character they repeat so tests can eyeball which fixture field a
// mismatch error names.
func fixtureDigest(char string) string {
	return "sha256:" + strings.Repeat(char, 64)
}

// ptr returns a pointer to v — the shared helper every Threshold/Guard
// bound fixture in this file uses to populate an *float64 field inline.
func ptr(v float64) *float64 { return &v }

// baseDefinition returns a fresh, valid, UNLOCKED Definition matching the
// spec's caching example shape: two candidates (baseline, candidate-a),
// one unbounded guard (behavioral-equivalence), one bounded guard
// (peak-rss, 15% over baseline), a lower-is-better primary metric with a
// 25% relative baseline-improvement threshold and a 5% relative
// candidate-separation threshold, and 3 rounds. Every test in this
// package starts from this shape and mutates only what it needs to,
// through lockDefinition's mutator argument.
func baseDefinition() experiment.Definition {
	return experiment.Definition{
		Schema:     experiment.DefinitionSchema,
		ID:         "cache-placement-v1",
		Spike:      "spec/cache-placement-spike",
		Question:   "spec/request-path#oq-cache-placement",
		BaseCommit: base40,
		Candidates: []experiment.Candidate{
			{ID: "baseline", Patch: "candidates/baseline.patch", Digest: fixtureDigest("a"), Base: base40},
			{ID: "candidate-a", Patch: "candidates/candidate-a.patch", Digest: fixtureDigest("b"), Base: base40},
		},
		Evaluator: experiment.Evaluator{
			Argv:               []string{"./tools/cache-evaluator", "run"},
			Digest:             fixtureDigest("c"),
			CapabilitiesDigest: fixtureDigest("d"),
		},
		Workload: experiment.ArtifactRef{ID: "representative-request-mix", Digest: fixtureDigest("e")},
		Contract: experiment.ArtifactRef{ID: "behavior-contract", Digest: fixtureDigest("f")},
		Decision: experiment.DecisionSpec{
			PrimaryMetric: experiment.PrimaryMetric{
				ID:          "request-latency",
				Type:        experiment.MetricDuration,
				Unit:        "ms",
				Aggregation: experiment.AggregationP95,
				Direction:   experiment.DirectionLower,
			},
			Baseline:            "baseline",
			BaselineImprovement: experiment.Threshold{Relative: ptr(0.25)},
			CandidateSeparation: experiment.Threshold{Relative: ptr(0.05)},
			Guards: []experiment.Guard{
				{ID: "behavioral-equivalence"},
				{ID: "peak-rss", MaximumRelativeToBaseline: ptr(0.15)},
			},
		},
		Execution: experiment.Execution{
			Warmups:           1,
			Rounds:            3,
			Order:             experiment.OrderDeterministicRotation,
			TimeoutPerRound:   "30s",
			EnvironmentPolicy: "local-isolated-v1",
		},
		Algorithm:       experiment.AlgorithmV1,
		RetentionPolicy: "standard-retention",
	}
}

// lockDefinition applies every mutator to a baseDefinition() copy, then
// computes and attaches its lock block. Tests use this instead of
// baseDefinition directly whenever they need a locked (evidence-eligible)
// definition — which is every Evaluate test, since Evaluate requires
// experiment.Locked.
func lockDefinition(t *testing.T, mutators ...func(*experiment.Definition)) experiment.Definition {
	t.Helper()
	def := baseDefinition()
	for _, m := range mutators {
		m(&def)
	}
	digest, err := experiment.DefinitionDigest(def)
	if err != nil {
		t.Fatalf("experiment.DefinitionDigest() unexpected error: %v", err)
	}
	def.Lock = &experiment.Lock{DefinitionDigest: digest}
	return def
}

// attestation returns the canned execution-layer environment-policy
// attestation for def: the policy def itself registers (SI-42). Wave 2
// has no execution unit to produce a real one, so every test and fixture
// in this package supplies this contract value; the mismatch and
// zero-value arms are exercised explicitly in attestation_test.go.
func attestation(def experiment.Definition) EnvironmentAttestation {
	return EnvironmentAttestation{PolicyID: def.Execution.EnvironmentPolicy}
}

// guard builds one passing (nil witness) or failing (nonempty witness)
// GuardResult.
func guardResult(id string, pass bool, witness string) experiment.GuardResult {
	if pass {
		return experiment.GuardResult{ID: id, Verdict: experiment.GuardVerdictPass, Witness: nil}
	}
	w := witness
	return experiment.GuardResult{ID: id, Verdict: experiment.GuardVerdictFail, Witness: &w}
}

// measurement builds one decision-eligible or diagnostic Measurement
// carrying the NUMERIC arm of SI-46's value union.
func measurement(id string, value float64, unit string, source experiment.Source) experiment.Measurement {
	return experiment.Measurement{ID: id, Value: experiment.NumberValue(formatFloat(value)), Unit: unit, Source: source}
}

// boolMeasurement builds one Measurement carrying the BOOLEAN arm — legal
// only for a measurement whose registered metric type is boolean (SI-46).
func boolMeasurement(id string, value bool, unit string, source experiment.Source) experiment.Measurement {
	return experiment.Measurement{ID: id, Value: experiment.BoolValue(value), Unit: unit, Source: source}
}

// observation builds one complete Observation record for def's locked
// digest and the given run identity.
func observation(def experiment.Definition, run, candidate string, round int, guards []experiment.GuardResult, measurements []experiment.Measurement) experiment.Observation {
	digest, err := experiment.DefinitionDigest(def)
	if err != nil {
		panic(err)
	}
	return experiment.Observation{
		Schema:           experiment.ObservationSchema,
		ExperimentDigest: digest,
		Run:              run,
		Candidate:        candidate,
		Round:            round,
		Guards:           guards,
		Measurements:     measurements,
		Disclosures:      []string{},
	}
}

// requiredGuardIDs returns the ids of def's unbounded (required) guards,
// in registered order — the exact set every observation record in a happy
// -path fixture must carry a verdict for.
func requiredGuardIDs(def experiment.Definition) []string {
	var ids []string
	for _, g := range def.Decision.Guards {
		if !g.Bounded() {
			ids = append(ids, g.ID)
		}
	}
	return ids
}

// passingGuards builds a passing GuardResult for every required guard in
// def, in registered order.
func passingGuards(def experiment.Definition) []experiment.GuardResult {
	var out []experiment.GuardResult
	for _, id := range requiredGuardIDs(def) {
		out = append(out, guardResult(id, true, ""))
	}
	return out
}

// happyObservations builds a complete, integrity-valid observation set for
// def across every registered candidate and round: every candidate passes
// every required guard every round, and carries a primary-metric value of
// primaryByCandRound[candidate][round-1] (evaluator-measured) plus a
// peak-rss value of boundByCandRound[candidate][round-1] (harness
// -measured) whenever def registers a "peak-rss" bounded guard.
func happyObservations(t *testing.T, def experiment.Definition, run string, primaryByCandRound map[string][]float64, boundByCandRound map[string][]float64) []experiment.Observation {
	t.Helper()
	var obs []experiment.Observation
	for _, c := range def.Candidates {
		for round := 1; round <= def.Execution.Rounds; round++ {
			measurements := []experiment.Measurement{
				measurement(def.Decision.PrimaryMetric.ID, primaryByCandRound[c.ID][round-1], def.Decision.PrimaryMetric.Unit, experiment.SourceEvaluatorMeasured),
			}
			if boundByCandRound != nil {
				if vals, ok := boundByCandRound[c.ID]; ok {
					measurements = append(measurements, measurement("peak-rss", vals[round-1], "MiB", experiment.SourceHarnessMeasured))
				}
			}
			obs = append(obs, observation(def, run, c.ID, round, passingGuards(def), measurements))
		}
	}
	return obs
}
