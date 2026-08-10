package experiment

import (
	"strings"
	"testing"
)

func validObservationJSON() string {
	return `{
  "schema": "verdi.experiment-observation/v1",
  "experiment_digest": "` + digestOf("a") + `",
  "run": "run-1",
  "candidate": "facts-cache",
  "round": 4,
  "guards": [
    {"id": "behavioral-equivalence", "verdict": "pass", "witness": null}
  ],
  "measurements": [
    {"id": "request-latency", "value": 18.0, "unit": "ms", "source": "evaluator-measured"}
  ],
  "disclosures": []
}`
}

func mutateObservation(t *testing.T, old, replacement string) string {
	t.Helper()
	doc := validObservationJSON()
	if !strings.Contains(doc, old) {
		t.Fatalf("fixture does not contain %q", old)
	}
	return strings.Replace(doc, old, replacement, 1)
}

func TestDecodeObservationHappyPath(t *testing.T) {
	o, err := DecodeObservation([]byte(validObservationJSON()))
	if err != nil {
		t.Fatalf("DecodeObservation() unexpected error: %v", err)
	}
	if o.Candidate != "facts-cache" || o.Round != 4 {
		t.Errorf("o = %+v, want candidate=facts-cache round=4", o)
	}
	if len(o.Disclosures) != 0 {
		t.Errorf("o.Disclosures = %v, want empty (but present) slice", o.Disclosures)
	}
}

func TestDecodeObservationWitnessOnFail(t *testing.T) {
	doc := mutateObservation(t,
		`{"id": "behavioral-equivalence", "verdict": "pass", "witness": null}`,
		`{"id": "behavioral-equivalence", "verdict": "fail", "witness": "stale after policy update"}`,
	)
	o, err := DecodeObservation([]byte(doc))
	if err != nil {
		t.Fatalf("DecodeObservation() unexpected error: %v", err)
	}
	if o.Guards[0].Witness == nil || *o.Guards[0].Witness != "stale after policy update" {
		t.Errorf("Guards[0].Witness = %v, want non-nil witness", o.Guards[0].Witness)
	}
}

func TestDecodeObservationRejects(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{"unknown schema", mutateObservation(t, `"schema": "verdi.experiment-observation/v1"`, `"schema": "verdi.experiment-observation/v2"`)},
		{"unknown field", strings.TrimSuffix(validObservationJSON(), "}") + `,"unknown_field": true}`},
		{"trailing data", validObservationJSON() + "\n{}"},
		{"bad experiment_digest", mutateObservation(t, `"experiment_digest": "`+digestOf("a")+`"`, `"experiment_digest": "not-a-digest"`)},
		{"bad run id", mutateObservation(t, `"run": "run-1"`, `"run": "Run_1"`)},
		{"bad candidate id", mutateObservation(t, `"candidate": "facts-cache"`, `"candidate": "Facts_Cache"`)},
		{"round zero", mutateObservation(t, `"round": 4`, `"round": 0`)},
		{"round negative", mutateObservation(t, `"round": 4`, `"round": -1`)},
		{"unknown guard verdict", mutateObservation(t, `"verdict": "pass"`, `"verdict": "unknown"`)},
		{"witness present on pass", mutateObservation(t, `"witness": null`, `"witness": "should not be here"`)},
		{"witness missing on fail", mutateObservation(t, `"verdict": "pass", "witness": null`, `"verdict": "fail", "witness": null`)},
		{"witness empty string on fail", mutateObservation(t, `"verdict": "pass", "witness": null`, `"verdict": "fail", "witness": ""`)},
		{"duplicate guard ids", mutateObservation(t,
			`"guards": [
    {"id": "behavioral-equivalence", "verdict": "pass", "witness": null}
  ]`,
			`"guards": [
    {"id": "behavioral-equivalence", "verdict": "pass", "witness": null},
    {"id": "behavioral-equivalence", "verdict": "pass", "witness": null}
  ]`)},
		{"unknown measurement source", mutateObservation(t, `"source": "evaluator-measured"`, `"source": "candidate-guessed"`)},
		{"malformed measurement unit", mutateObservation(t, `"unit": "ms"`, `"unit": "m s"`)},
		{"duplicate measurement ids", mutateObservation(t,
			`"measurements": [
    {"id": "request-latency", "value": 18.0, "unit": "ms", "source": "evaluator-measured"}
  ]`,
			`"measurements": [
    {"id": "request-latency", "value": 18.0, "unit": "ms", "source": "evaluator-measured"},
    {"id": "request-latency", "value": 20.0, "unit": "ms", "source": "evaluator-measured"}
  ]`)},
		{"measurement value not a number", mutateObservation(t, `"value": 18.0`, `"value": "eighteen"`)},
		{"empty disclosure entry", mutateObservation(t, `"disclosures": []`, `"disclosures": [""]`)},
		{"disclosures field missing", strings.Replace(validObservationJSON(), `,
  "disclosures": []`, "", 1)},
		{"disclosures explicit null", mutateObservation(t, `"disclosures": []`, `"disclosures": null`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeObservation([]byte(tt.doc)); err == nil {
				t.Errorf("DecodeObservation(%s) = nil error, want error", tt.name)
			}
		})
	}
}

func TestDecodeObservationsJSONL(t *testing.T) {
	line1 := validObservationJSON()
	line2 := mutateObservation(t, `"round": 4`, `"round": 5`)
	// Compact each record onto one line the way a real JSONL file would
	// store it, joined by newlines with a blank trailing line.
	oneLine := func(s string) string {
		return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
	}
	data := oneLine(line1) + "\n" + oneLine(line2) + "\n"

	obs, err := DecodeObservations([]byte(data))
	if err != nil {
		t.Fatalf("DecodeObservations() unexpected error: %v", err)
	}
	if len(obs) != 2 {
		t.Fatalf("len(obs) = %d, want 2", len(obs))
	}
	if obs[0].Round != 4 || obs[1].Round != 5 {
		t.Errorf("obs rounds = [%d, %d], want [4, 5] (file order preserved)", obs[0].Round, obs[1].Round)
	}
}

func TestDecodeObservationsRejectsBadLine(t *testing.T) {
	data := "not json\n"
	if _, err := DecodeObservations([]byte(data)); err == nil {
		t.Errorf("DecodeObservations() with a malformed line = nil error, want error")
	}
}
