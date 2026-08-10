package experimentdecision

import (
	"errors"
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
)

// booleanPrimaryDefinition returns a locked definition whose primary
// metric is AC-3's boolean primitive, aggregated by agg and compared
// higher-is-better. Everything else — candidates, guards, the peak-rss
// bound, rounds — is baseDefinition's shape.
func booleanPrimaryDefinition(t *testing.T, agg experiment.Aggregation) experiment.Definition {
	t.Helper()
	return lockDefinition(t, func(def *experiment.Definition) {
		def.Decision.PrimaryMetric.Type = experiment.MetricBoolean
		def.Decision.PrimaryMetric.Unit = "bool"
		def.Decision.PrimaryMetric.Aggregation = agg
		def.Decision.PrimaryMetric.Direction = experiment.DirectionHigher
	})
}

// booleanObservations builds a complete observation set carrying boolean
// primary-metric literals per candidate and round, with the numeric
// peak-rss bound-guard measurement every record still needs.
func booleanObservations(t *testing.T, def experiment.Definition, primary map[string][]bool) []experiment.Observation {
	t.Helper()
	var obs []experiment.Observation
	for _, c := range def.Candidates {
		for round := 1; round <= def.Execution.Rounds; round++ {
			measurements := []experiment.Measurement{
				boolMeasurement(def.Decision.PrimaryMetric.ID, primary[c.ID][round-1], def.Decision.PrimaryMetric.Unit, experiment.SourceEvaluatorMeasured),
				measurement("peak-rss", 100, "MiB", experiment.SourceHarnessMeasured),
			}
			obs = append(obs, observation(def, "run-1", c.ID, round, passingGuards(def), measurements))
		}
	}
	return obs
}

// primaryValue returns candidate id's aggregated primary value from res.
func primaryValue(t *testing.T, res experiment.Result, id string) string {
	t.Helper()
	c := candidateResult(t, res, id)
	if c.Primary == nil {
		t.Fatalf("candidate %q carries no primary result", id)
	}
	return string(c.Primary.Value)
}

// TestEvaluateBooleanRateIsFractionOfTrue is SI-45's headline aggregation
// case: rate over a boolean metric is the fraction of rounds that measured
// true, because true maps to 1 and false to 0 and rate reduces by
// arithmetic mean (SI-40).
func TestEvaluateBooleanRateIsFractionOfTrue(t *testing.T) {
	def := booleanPrimaryDefinition(t, experiment.AggregationRate)
	obs := booleanObservations(t, def, map[string][]bool{
		"baseline":    {true, false, false}, // 1 of 3
		"candidate-a": {true, true, true},   // 3 of 3
	})

	res := mustEvaluate(t, def, obs)
	if got, want := primaryValue(t, res, "baseline"), string(formatFloat(1.0/3.0)); got != want {
		t.Errorf("baseline rate = %s, want %s", got, want)
	}
	if got, want := primaryValue(t, res, "candidate-a"), "1"; got != want {
		t.Errorf("candidate-a rate = %s, want %s", got, want)
	}
	if res.Verdict != experiment.VerdictProvenWinner || res.Winner != "candidate-a" {
		t.Fatalf("Verdict/Winner = %q/%q, want proven-winner/candidate-a", res.Verdict, res.Winner)
	}
}

// TestEvaluateBooleanAggregations proves all five registered aggregations
// stay defined over the mapped values (SI-45), with the same nearest-rank
// percentile and maximum/mean rules SI-40 fixed for numbers.
func TestEvaluateBooleanAggregations(t *testing.T) {
	// baseline maps to [1,0,0]; candidate-a maps to [1,1,0].
	primary := map[string][]bool{
		"baseline":    {true, false, false},
		"candidate-a": {true, true, false},
	}

	tests := []struct {
		name          string
		aggregation   experiment.Aggregation
		wantBaseline  string
		wantCandidate string
	}{
		{"p50 nearest rank", experiment.AggregationP50, "0", "1"},
		{"p95 nearest rank", experiment.AggregationP95, "1", "1"},
		{"maximum", experiment.AggregationMaximum, "1", "1"},
		{"mean", experiment.AggregationMean, string(formatFloat(1.0 / 3.0)), string(formatFloat(2.0 / 3.0))},
		{"rate", experiment.AggregationRate, string(formatFloat(1.0 / 3.0)), string(formatFloat(2.0 / 3.0))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := booleanPrimaryDefinition(t, tt.aggregation)
			res := mustEvaluate(t, def, booleanObservations(t, def, primary))
			if got := primaryValue(t, res, "baseline"); got != tt.wantBaseline {
				t.Errorf("baseline %s = %s, want %s", tt.aggregation, got, tt.wantBaseline)
			}
			if got := primaryValue(t, res, "candidate-a"); got != tt.wantCandidate {
				t.Errorf("candidate-a %s = %s, want %s", tt.aggregation, got, tt.wantCandidate)
			}
		})
	}
}

// TestEvaluateRejectsMismatchedMeasurementKinds proves the union is
// enforced at Evaluate's precondition, in both directions, and carries no
// Result: a number for a boolean-typed metric and a boolean for a numeric
// one are integrity violations, never quietly coerced inputs (SI-45,
// CO-1).
func TestEvaluateRejectsMismatchedMeasurementKinds(t *testing.T) {
	tests := []struct {
		name  string
		build func(t *testing.T) (experiment.Definition, []experiment.Observation)
	}{
		{
			name: "number for a boolean metric",
			build: func(t *testing.T) (experiment.Definition, []experiment.Observation) {
				def := booleanPrimaryDefinition(t, experiment.AggregationRate)
				obs := booleanObservations(t, def, map[string][]bool{
					"baseline":    {true, false, false},
					"candidate-a": {true, true, true},
				})
				obs[0].Measurements[0] = measurement(def.Decision.PrimaryMetric.ID, 1, "bool", experiment.SourceEvaluatorMeasured)
				return def, obs
			},
		},
		{
			name: "boolean for a numeric metric",
			build: func(t *testing.T) (experiment.Definition, []experiment.Observation) {
				def := lockDefinition(t)
				obs := happyObservations(t, def, "run-1",
					map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
					map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
				)
				obs[0].Measurements[0] = boolMeasurement(def.Decision.PrimaryMetric.ID, true, "ms", experiment.SourceEvaluatorMeasured)
				return def, obs
			},
		},
		{
			name: "boolean for a bound guard",
			build: func(t *testing.T) (experiment.Definition, []experiment.Observation) {
				def := lockDefinition(t)
				obs := happyObservations(t, def, "run-1",
					map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
					map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
				)
				obs[0].Measurements[1] = boolMeasurement("peak-rss", true, "MiB", experiment.SourceHarnessMeasured)
				return def, obs
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, obs := tt.build(t)
			res, err := Evaluate(def, obs, attestation(def))
			if err == nil {
				t.Fatalf("Evaluate(%s) = nil error, want an operational error", tt.name)
			}
			if !errors.Is(err, experiment.ErrObservationIntegrity) {
				t.Errorf("Evaluate(%s) error = %v, want it to wrap ErrObservationIntegrity", tt.name, err)
			}
			if !isZeroResult(res) {
				t.Errorf("Evaluate(%s) returned a nonzero Result alongside an error: %+v", tt.name, res)
			}
		})
	}
}
