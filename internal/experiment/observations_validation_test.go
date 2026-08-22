package experiment

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

const (
	obsGuardsOK       = `[{"id":"behavioral-equivalence","verdict":"pass","witness":null},{"id":"tenant-isolation","verdict":"pass","witness":null}]`
	obsMeasurementsOK = `[{"id":"request-latency","value":18.0,"unit":"ms","source":"evaluator-measured"},{"id":"peak-rss","value":109,"unit":"MiB","source":"harness-measured"}]`
)

// smallRoundsDefinition returns a locked definition identical to the
// shared fixture but with rounds reduced to 2, keeping observation-set
// fixtures small, plus its computed digest.
func smallRoundsDefinition(t *testing.T) (def Definition, digest string) {
	t.Helper()
	unlocked := mutate(t, "rounds: 10", "rounds: 2")
	def = mustDecodeDefinition(t, unlocked)
	digest, err := DefinitionDigest(def)
	if err != nil {
		t.Fatalf("DefinitionDigest() unexpected error: %v", err)
	}
	lockedDoc := unlocked + "lock:\n  definition_digest: " + digest + "\n"
	def = mustDecodeDefinition(t, lockedDoc)
	return def, digest
}

func obsRecord(defDigest, run, candidate string, round int, guards, measurements, disclosures string) string {
	return fmt.Sprintf(`{"schema": "verdi.experiment-observation/v1", "experiment_digest": %q, "run": %q, "candidate": %q, "round": %d, "guards": %s, "measurements": %s, "disclosures": %s}`,
		defDigest, run, candidate, round, guards, measurements, disclosures)
}

func decodeObs(t *testing.T, lines ...string) []Observation {
	t.Helper()
	data := strings.Join(lines, "\n") + "\n"
	obs, err := DecodeObservations([]byte(data))
	if err != nil {
		t.Fatalf("DecodeObservations() unexpected error: %v", err)
	}
	return obs
}

func validObservationLines(defDigest string) []string {
	return []string{
		obsRecord(defDigest, "run-1", "baseline", 1, obsGuardsOK, obsMeasurementsOK, "[]"),
		obsRecord(defDigest, "run-1", "baseline", 2, obsGuardsOK, obsMeasurementsOK, "[]"),
		obsRecord(defDigest, "run-1", "facts-cache", 1, obsGuardsOK, obsMeasurementsOK, "[]"),
		obsRecord(defDigest, "run-1", "facts-cache", 2, obsGuardsOK, obsMeasurementsOK, "[]"),
	}
}

func TestValidateObservationsHappyPath(t *testing.T) {
	def, digest := smallRoundsDefinition(t)
	obs := decodeObs(t, validObservationLines(digest)...)
	if err := ValidateObservations(def, obs); err != nil {
		t.Fatalf("ValidateObservations() unexpected error: %v", err)
	}
}

func TestValidateObservationsV2RestrictsHarnessMeasuredCustody(t *testing.T) {
	def, digest := smallRoundsDefinition(t)
	base := decodeObs(t, validObservationLines(digest)...)
	for i := range base {
		base[i].Schema = ObservationSchemaV2
		base[i].Outcome = &CandidateOutcome{Kind: OutcomeCompleted}
		base[i].Measurements[1].Source = SourceEvaluatorMeasured
	}

	t.Run("unreserved observer id", func(t *testing.T) {
		observations := append([]Observation(nil), base...)
		observations[0].Measurements = append([]Measurement(nil), base[0].Measurements...)
		observations[0].Measurements[1].Source = SourceHarnessMeasured
		if err := ValidateObservations(def, observations); err == nil {
			t.Fatalf("ValidateObservations(v2 unreserved harness measurement) = nil error")
		}
	})

	t.Run("reserved observer with wrong built-in unit", func(t *testing.T) {
		observations := append([]Observation(nil), base...)
		for i := range observations {
			observations[i].Measurements = append(append([]Measurement(nil), base[i].Measurements...), Measurement{
				ID: EvaluatorWallDurationMetricID, Value: NumberValue("1"), Unit: "ms", Source: SourceHarnessMeasured,
			})
		}
		if err := ValidateObservations(def, observations); err == nil {
			t.Fatalf("ValidateObservations(v2 harness measurement with wrong built-in unit) = nil error")
		}
	})

	t.Run("reserved observer with exact built-in definition", func(t *testing.T) {
		observations := append([]Observation(nil), base...)
		for i := range observations {
			observations[i].Measurements = append(append([]Measurement(nil), base[i].Measurements...), Measurement{
				ID: EvaluatorWallDurationMetricID, Value: NumberValue("1"), Unit: "ns", Source: SourceHarnessMeasured,
			})
		}
		if err := ValidateObservations(def, observations); err != nil {
			t.Fatalf("ValidateObservations(v2 fixed harness observer): %v", err)
		}
	})
}

func TestValidateObservationsV2CandidateFailureProcessFactsAreDiagnostic(t *testing.T) {
	def, digest := smallRoundsDefinition(t)
	observations := decodeObs(t, validObservationLines(digest)...)
	for i := range observations {
		observations[i].Schema = ObservationSchemaV2
		observations[i].Outcome = &CandidateOutcome{Kind: OutcomeCompleted}
		// This predecessor fixture's peak-rss id is not one of the V2 fixed
		// harness observer ids, so it represents evaluator evidence here.
		observations[i].Measurements[1].Source = SourceEvaluatorMeasured
	}
	witness := "candidate process timed out"
	failed := &observations[len(observations)-1]
	failed.Outcome = &CandidateOutcome{Kind: OutcomeCandidateTimeout, Witness: &witness}
	failed.Guards = []GuardResult{}
	failed.Measurements = []Measurement{
		{ID: EvaluatorWallDurationMetricID, Value: NumberValue("2500000"), Unit: "ns", Source: SourceHarnessMeasured},
		{ID: EvaluatorPeakRSSMetricID, Value: NumberValue("4096"), Unit: "bytes", Source: SourceHarnessMeasured},
	}

	if err := ValidateComplete(def, observations); err != nil {
		t.Fatalf("ValidateComplete(candidate failure with diagnostic process facts): %v", err)
	}
}

func TestValidateObservationsNotLocked(t *testing.T) {
	unlocked := mustDecodeDefinition(t, mutate(t, "rounds: 10", "rounds: 2"))
	digest, err := DefinitionDigest(unlocked)
	if err != nil {
		t.Fatalf("DefinitionDigest() unexpected error: %v", err)
	}
	obs := decodeObs(t, validObservationLines(digest)...)
	if err := ValidateObservations(unlocked, obs); err == nil {
		t.Errorf("ValidateObservations() on an unlocked definition = nil error, want error")
	}
}

func TestValidateObservationsRejects(t *testing.T) {
	def, digest := smallRoundsDefinition(t)

	tests := []struct {
		name  string
		lines []string
	}{
		{"empty set", nil},
		{"mismatched experiment_digest", func() []string {
			l := validObservationLines(digest)
			l[0] = obsRecord(digestOf("0"), "run-1", "baseline", 1, obsGuardsOK, obsMeasurementsOK, "[]")
			return l
		}()},
		{"mixed run ids", func() []string {
			l := validObservationLines(digest)
			l[0] = obsRecord(digest, "run-2", "baseline", 1, obsGuardsOK, obsMeasurementsOK, "[]")
			return l
		}()},
		{"unregistered candidate", func() []string {
			l := validObservationLines(digest)
			l[0] = obsRecord(digest, "run-1", "nonexistent", 1, obsGuardsOK, obsMeasurementsOK, "[]")
			return l
		}()},
		{"round out of range", func() []string {
			l := validObservationLines(digest)
			l[0] = obsRecord(digest, "run-1", "baseline", 3, obsGuardsOK, obsMeasurementsOK, "[]")
			return l
		}()},
		{"duplicate candidate/round", append(validObservationLines(digest), obsRecord(digest, "run-1", "baseline", 1, obsGuardsOK, obsMeasurementsOK, "[]"))},
		{"unregistered guard id (bound guard as verdict)", func() []string {
			l := validObservationLines(digest)
			l[0] = obsRecord(digest, "run-1", "baseline", 1,
				`[{"id":"behavioral-equivalence","verdict":"pass","witness":null},{"id":"tenant-isolation","verdict":"pass","witness":null},{"id":"peak-rss","verdict":"pass","witness":null}]`,
				obsMeasurementsOK, "[]")
			return l
		}()},
		{"missing required guard verdict", func() []string {
			l := validObservationLines(digest)
			l[0] = obsRecord(digest, "run-1", "baseline", 1,
				`[{"id":"behavioral-equivalence","verdict":"pass","witness":null}]`,
				obsMeasurementsOK, "[]")
			return l
		}()},
		{"primary metric measurement missing", func() []string {
			l := validObservationLines(digest)
			l[0] = obsRecord(digest, "run-1", "baseline", 1, obsGuardsOK,
				`[{"id":"peak-rss","value":109,"unit":"MiB","source":"harness-measured"}]`, "[]")
			return l
		}()},
		{"primary metric only candidate-reported", func() []string {
			l := validObservationLines(digest)
			l[0] = obsRecord(digest, "run-1", "baseline", 1, obsGuardsOK,
				`[{"id":"request-latency","value":18.0,"unit":"ms","source":"candidate-reported"},{"id":"peak-rss","value":109,"unit":"MiB","source":"harness-measured"}]`, "[]")
			return l
		}()},
		{"primary metric unit mismatch", func() []string {
			l := validObservationLines(digest)
			l[0] = obsRecord(digest, "run-1", "baseline", 1, obsGuardsOK,
				`[{"id":"request-latency","value":18.0,"unit":"seconds","source":"evaluator-measured"},{"id":"peak-rss","value":109,"unit":"MiB","source":"harness-measured"}]`, "[]")
			return l
		}()},
		{"bound guard measurement missing", func() []string {
			l := validObservationLines(digest)
			l[0] = obsRecord(digest, "run-1", "baseline", 1, obsGuardsOK,
				`[{"id":"request-latency","value":18.0,"unit":"ms","source":"evaluator-measured"}]`, "[]")
			return l
		}()},
		{"bound guard unit inconsistent across records", func() []string {
			l := validObservationLines(digest)
			l[1] = obsRecord(digest, "run-1", "baseline", 2, obsGuardsOK,
				`[{"id":"request-latency","value":18.0,"unit":"ms","source":"evaluator-measured"},{"id":"peak-rss","value":109,"unit":"KiB","source":"harness-measured"}]`, "[]")
			return l
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var obs []Observation
			if tt.lines != nil {
				obs = decodeObs(t, tt.lines...)
			}
			err := ValidateObservations(def, obs)
			if err == nil {
				t.Fatalf("ValidateObservations(%s) = nil error, want error", tt.name)
			}
			if !errors.Is(err, ErrObservationIntegrity) {
				t.Errorf("ValidateObservations(%s) error %v does not wrap ErrObservationIntegrity", tt.name, err)
			}
		})
	}
}

// booleanPrimaryDefinition returns a locked definition identical to
// smallRoundsDefinition's except that the primary metric is the BOOLEAN
// primitive AC-3 registers, aggregated by rate — the shape SI-46's union
// exists for — plus its computed digest.
func booleanPrimaryDefinition(t *testing.T) (def Definition, digest string) {
	t.Helper()
	doc := validDefinitionYAML()
	for _, r := range [][2]string{
		{"rounds: 10", "rounds: 2"},
		{"    type: duration\n    unit: ms\n    aggregation: p95\n    direction: lower\n",
			"    type: boolean\n    unit: bool\n    aggregation: rate\n    direction: higher\n"},
	} {
		if !strings.Contains(doc, r[0]) {
			t.Fatalf("definition fixture does not contain %q", r[0])
		}
		doc = strings.Replace(doc, r[0], r[1], 1)
	}
	def = mustDecodeDefinition(t, doc)
	digest, err := DefinitionDigest(def)
	if err != nil {
		t.Fatalf("DefinitionDigest() unexpected error: %v", err)
	}
	def = mustDecodeDefinition(t, doc+"lock:\n  definition_digest: "+digest+"\n")
	return def, digest
}

// boolObservationLines returns a complete observation set for the boolean
// -primary definition: request-latency carries the JSON literal true or
// false, and peak-rss (a bound guard, which registers no metric type) stays
// numeric.
func boolObservationLines(defDigest string, primary map[string][]bool) []string {
	var lines []string
	for _, candidate := range []string{"baseline", "facts-cache"} {
		for round := 1; round <= 2; round++ {
			literal := "false"
			if primary[candidate][round-1] {
				literal = "true"
			}
			measurements := `[{"id":"request-latency","value":` + literal + `,"unit":"bool","source":"evaluator-measured"},` +
				`{"id":"peak-rss","value":109,"unit":"MiB","source":"harness-measured"}]`
			lines = append(lines, obsRecord(defDigest, "run-1", candidate, round, obsGuardsOK, measurements, "[]"))
		}
	}
	return lines
}

// TestValidateObservationsBooleanMetricHappyPath is SI-46's positive arm:
// with a boolean-typed primary metric registered, true/false literals
// validate, and they survive decode as booleans rather than as numbers.
func TestValidateObservationsBooleanMetricHappyPath(t *testing.T) {
	def, digest := booleanPrimaryDefinition(t)
	obs := decodeObs(t, boolObservationLines(digest, map[string][]bool{
		"baseline":    {false, true},
		"facts-cache": {true, true},
	})...)

	if err := ValidateComplete(def, obs); err != nil {
		t.Fatalf("ValidateComplete() unexpected error: %v", err)
	}
	for i, o := range obs {
		for _, m := range o.Measurements {
			if m.ID != "request-latency" {
				continue
			}
			if !m.Value.IsBool() {
				t.Errorf("observation %d: primary measurement decoded as %s, want a boolean literal", i, m.Value)
			}
		}
	}
}

// TestValidateObservationsRejectsMismatchedValueKinds is the negative
// arm, both directions (SI-46): a number where the registered metric type
// is boolean, and a boolean where it is not.
func TestValidateObservationsRejectsMismatchedValueKinds(t *testing.T) {
	numericDef, numericDigest := smallRoundsDefinition(t)
	boolDef, boolDigest := booleanPrimaryDefinition(t)

	boolLines := boolObservationLines(boolDigest, map[string][]bool{
		"baseline":    {false, true},
		"facts-cache": {true, true},
	})

	tests := []struct {
		name  string
		def   Definition
		lines []string
	}{
		{"number for a boolean metric", boolDef, func() []string {
			l := append([]string(nil), boolLines...)
			l[0] = obsRecord(boolDigest, "run-1", "baseline", 1, obsGuardsOK,
				`[{"id":"request-latency","value":1,"unit":"bool","source":"evaluator-measured"},{"id":"peak-rss","value":109,"unit":"MiB","source":"harness-measured"}]`, "[]")
			return l
		}()},
		{"zero for a boolean metric", boolDef, func() []string {
			l := append([]string(nil), boolLines...)
			l[0] = obsRecord(boolDigest, "run-1", "baseline", 1, obsGuardsOK,
				`[{"id":"request-latency","value":0,"unit":"bool","source":"evaluator-measured"},{"id":"peak-rss","value":109,"unit":"MiB","source":"harness-measured"}]`, "[]")
			return l
		}()},
		{"boolean for a numeric metric", numericDef, func() []string {
			l := validObservationLines(numericDigest)
			l[0] = obsRecord(numericDigest, "run-1", "baseline", 1, obsGuardsOK,
				`[{"id":"request-latency","value":true,"unit":"ms","source":"evaluator-measured"},{"id":"peak-rss","value":109,"unit":"MiB","source":"harness-measured"}]`, "[]")
			return l
		}()},
		{"boolean for a bound guard", numericDef, func() []string {
			l := validObservationLines(numericDigest)
			l[0] = obsRecord(numericDigest, "run-1", "baseline", 1, obsGuardsOK,
				`[{"id":"request-latency","value":18.0,"unit":"ms","source":"evaluator-measured"},{"id":"peak-rss","value":true,"unit":"MiB","source":"harness-measured"}]`, "[]")
			return l
		}()},
		{"boolean for an unregistered diagnostic measurement", numericDef, func() []string {
			l := validObservationLines(numericDigest)
			l[0] = obsRecord(numericDigest, "run-1", "baseline", 1, obsGuardsOK,
				`[{"id":"request-latency","value":18.0,"unit":"ms","source":"evaluator-measured"},{"id":"peak-rss","value":109,"unit":"MiB","source":"harness-measured"},{"id":"cache-hit","value":true,"unit":"bool","source":"candidate-reported"}]`, "[]")
			return l
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs := decodeObs(t, tt.lines...)
			err := ValidateObservations(tt.def, obs)
			if err == nil {
				t.Fatalf("ValidateObservations(%s) = nil error, want error", tt.name)
			}
			if !errors.Is(err, ErrObservationIntegrity) {
				t.Errorf("ValidateObservations(%s) error %v does not wrap ErrObservationIntegrity", tt.name, err)
			}
			// The rejection must be THIS rule's, not an unrelated integrity
			// check that happens to fire on the same record.
			if !strings.Contains(err.Error(), "must carry") {
				t.Errorf("ValidateObservations(%s) error %v does not report the value-kind mismatch", tt.name, err)
			}
		})
	}
}

// TestBooleanObservationsRoundTripLiterals proves the decoded record
// re-encodes with its boolean literals intact, which is what keeps the
// observations digest stable across a read/write cycle (CO-3).
func TestBooleanObservationsRoundTripLiterals(t *testing.T) {
	_, digest := booleanPrimaryDefinition(t)
	obs := decodeObs(t, boolObservationLines(digest, map[string][]bool{
		"baseline":    {false, true},
		"facts-cache": {true, true},
	})...)

	encoded, err := json.Marshal(obs[0].Measurements)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}
	if !strings.Contains(string(encoded), `"value":false`) {
		t.Errorf("re-encoded measurements = %s, want the boolean literal false preserved", encoded)
	}
	if strings.Contains(string(encoded), `"value":0`) {
		t.Errorf("re-encoded measurements = %s, want no 0/1 coercion of a boolean", encoded)
	}
}

func TestValidateCompleteHappyPath(t *testing.T) {
	def, digest := smallRoundsDefinition(t)
	obs := decodeObs(t, validObservationLines(digest)...)
	if err := ValidateComplete(def, obs); err != nil {
		t.Fatalf("ValidateComplete() unexpected error: %v", err)
	}
}

func TestValidateCompleteMissingEntry(t *testing.T) {
	def, digest := smallRoundsDefinition(t)
	lines := validObservationLines(digest)
	obs := decodeObs(t, lines[:3]...) // drop facts-cache round 2

	err := ValidateComplete(def, obs)
	if err == nil {
		t.Fatalf("ValidateComplete() with a missing (candidate, round) = nil error, want error")
	}
	if !errors.Is(err, ErrObservationIncomplete) {
		t.Errorf("ValidateComplete() error %v does not wrap ErrObservationIncomplete", err)
	}
	if !strings.Contains(err.Error(), "facts-cache") {
		t.Errorf("ValidateComplete() error %v does not name the missing candidate", err)
	}
}

func TestValidateCompletePropagatesIntegrityErrors(t *testing.T) {
	def, digest := smallRoundsDefinition(t)
	lines := validObservationLines(digest)
	lines[0] = obsRecord(digestOf("0"), "run-1", "baseline", 1, obsGuardsOK, obsMeasurementsOK, "[]")
	obs := decodeObs(t, lines...)

	err := ValidateComplete(def, obs)
	if err == nil {
		t.Fatalf("ValidateComplete() with an integrity violation = nil error, want error")
	}
	if !errors.Is(err, ErrObservationIntegrity) {
		t.Errorf("ValidateComplete() error %v does not wrap ErrObservationIntegrity (integrity failures must propagate, not be reported as incompleteness)", err)
	}
	if errors.Is(err, ErrObservationIncomplete) {
		t.Errorf("ValidateComplete() error %v wraps ErrObservationIncomplete, want only ErrObservationIntegrity for an integrity-violating input", err)
	}
}
