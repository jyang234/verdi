package experiment

import (
	"strings"
	"testing"
)

func validResultJSON() string {
	return `{
  "schema": "verdi.experiment-result/v1",
  "experiment": "cache-placement-v1",
  "definition_digest": "` + digestOf("a") + `",
  "run": "run-1",
  "algorithm": "verdi.experiment-recommendation/v1",
  "verdict": "proven-winner",
  "winner": "facts-cache",
  "candidates": [
    {"id": "baseline", "baseline": true, "eligible": true,
     "primary": {"aggregation": "p95", "unit": "ms", "value": 40, "rounds": 10}},
    {"id": "facts-cache", "baseline": false, "eligible": true,
     "primary": {"aggregation": "p95", "unit": "ms", "value": 18, "rounds": 10},
     "bounds": [{"guard": "peak-rss", "value": 109, "limit": 115, "pass": true}]},
    {"id": "final-cache", "baseline": false, "eligible": false,
     "violations": [{"guard": "behavioral-equivalence", "round": 3, "witness": "stale after policy update"}]}
  ],
  "observations_digest": "` + digestOf("b") + `"
}`
}

func validInconclusiveResultJSON() string {
	return `{
  "schema": "verdi.experiment-result/v1",
  "experiment": "cache-placement-v1",
  "definition_digest": "` + digestOf("a") + `",
  "run": "run-1",
  "algorithm": "verdi.experiment-recommendation/v1",
  "verdict": "disclosed-unproven",
  "reasons": [{"code": "practical-tie", "detail": "candidates within noise floor"}],
  "candidates": [
    {"id": "baseline", "baseline": true, "eligible": true,
     "primary": {"aggregation": "p95", "unit": "ms", "value": 40, "rounds": 10}},
    {"id": "facts-cache", "baseline": false, "eligible": true,
     "primary": {"aggregation": "p95", "unit": "ms", "value": 39, "rounds": 10}}
  ],
  "observations_digest": "` + digestOf("b") + `"
}`
}

func mutateResult(t *testing.T, doc, old, replacement string) string {
	t.Helper()
	if !strings.Contains(doc, old) {
		t.Fatalf("fixture does not contain %q", old)
	}
	return strings.Replace(doc, old, replacement, 1)
}

func TestDecodeResultHappyPath(t *testing.T) {
	res, err := DecodeResult([]byte(validResultJSON()))
	if err != nil {
		t.Fatalf("DecodeResult() unexpected error: %v", err)
	}
	if res.Winner != "facts-cache" {
		t.Errorf("res.Winner = %q, want facts-cache", res.Winner)
	}
	if len(res.Reasons) != 0 {
		t.Errorf("res.Reasons = %v, want empty for proven-winner", res.Reasons)
	}
}

func TestDecodeResultInconclusiveHappyPath(t *testing.T) {
	res, err := DecodeResult([]byte(validInconclusiveResultJSON()))
	if err != nil {
		t.Fatalf("DecodeResult() unexpected error: %v", err)
	}
	if res.Winner != "" {
		t.Errorf("res.Winner = %q, want empty for disclosed-unproven", res.Winner)
	}
	if len(res.Reasons) == 0 {
		t.Errorf("res.Reasons is empty, want nonempty for disclosed-unproven")
	}
}

func TestDecodeResultReasonWitness(t *testing.T) {
	doc := mutateResult(t, validInconclusiveResultJSON(),
		`{"code": "practical-tie", "detail": "candidates within noise floor"}`,
		`{"code": "practical-tie", "detail": "candidates within noise floor", "witness": "p95 40ms vs 39ms"}`)
	res, err := DecodeResult([]byte(doc))
	if err != nil {
		t.Fatalf("DecodeResult() unexpected error: %v", err)
	}
	if res.Reasons[0].Witness == nil || *res.Reasons[0].Witness != "p95 40ms vs 39ms" {
		t.Errorf("Reasons[0].Witness = %v, want the recorded witness", res.Reasons[0].Witness)
	}

	res, err = DecodeResult([]byte(validInconclusiveResultJSON()))
	if err != nil {
		t.Fatalf("DecodeResult() unexpected error: %v", err)
	}
	if res.Reasons[0].Witness != nil {
		t.Errorf("Reasons[0].Witness = %v, want nil for an absent witness", res.Reasons[0].Witness)
	}
}

func TestDecodeResultRejects(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{"unknown schema", mutateResult(t, validResultJSON(), `"schema": "verdi.experiment-result/v1"`, `"schema": "verdi.experiment-result/v2"`)},
		{"unknown field", strings.TrimSuffix(validResultJSON(), "}") + `,"unknown_field": true}`},
		{"trailing data", validResultJSON() + "\n{}"},
		{"unknown verdict", mutateResult(t, validResultJSON(), `"verdict": "proven-winner"`, `"verdict": "unknown-verdict"`)},
		{"unknown algorithm", mutateResult(t, validResultJSON(), `"algorithm": "verdi.experiment-recommendation/v1"`, `"algorithm": "verdi.experiment-recommendation/v2"`)},
		{"winner missing on proven-winner", mutateResult(t, validResultJSON(), `"winner": "facts-cache",`, "")},
		{"winner present on non-proven-winner", mutateResult(t, validInconclusiveResultJSON(), `"verdict": "disclosed-unproven",`, `"verdict": "disclosed-unproven",
  "winner": "facts-cache",`)},
		{"winner not among candidates", mutateResult(t, validResultJSON(), `"winner": "facts-cache",`, `"winner": "nonexistent",`)},
		{"winner is an ineligible candidate", mutateResult(t, validResultJSON(), `"winner": "facts-cache",`, `"winner": "final-cache",`)},
		{"winner is the baseline", mutateResult(t, validResultJSON(), `"winner": "facts-cache",`, `"winner": "baseline",`)},
		{"reason witness present but empty", mutateResult(t, validInconclusiveResultJSON(), `{"code": "practical-tie", "detail": "candidates within noise floor"}`, `{"code": "practical-tie", "detail": "candidates within noise floor", "witness": ""}`)},
		{"reasons nonempty on proven-winner", mutateResult(t, validResultJSON(), `"winner": "facts-cache",`, `"winner": "facts-cache",
  "reasons": [{"code": "practical-tie"}],`)},
		{"reasons empty on disclosed-unproven", mutateResult(t, validInconclusiveResultJSON(), `"reasons": [{"code": "practical-tie", "detail": "candidates within noise floor"}],`, "")},
		{"unknown reason code", mutateResult(t, validInconclusiveResultJSON(), `"code": "practical-tie"`, `"code": "unknown-reason"`)},
		{"candidates empty", mutateResultCandidatesBlock(t, validResultJSON(), `[]`)},
		{"duplicate candidate ids", mutateResult(t, validResultJSON(), `{"id": "final-cache", "baseline": false, "eligible": false,`, `{"id": "facts-cache", "baseline": false, "eligible": false,`)},
		{"no baseline true", mutateResult(t, validResultJSON(), `{"id": "baseline", "baseline": true, "eligible": true,`, `{"id": "baseline", "baseline": false, "eligible": true,`)},
		{"two baselines true", mutateResult(t, validResultJSON(), `{"id": "facts-cache", "baseline": false, "eligible": true,`, `{"id": "facts-cache", "baseline": true, "eligible": true,`)},
		{"violation missing witness", mutateResult(t, validResultJSON(), `"witness": "stale after policy update"`, `"witness": ""`)},
		{"violation round zero", mutateResult(t, validResultJSON(), `"round": 3, "witness"`, `"round": 0, "witness"`)},
		{"primary value not a number", mutateResult(t, validResultJSON(), `"value": 40, "rounds": 10`, `"value": "forty", "rounds": 10`)},
		{"primary rounds zero", mutateResult(t, validResultJSON(), `"value": 40, "rounds": 10`, `"value": 40, "rounds": 0`)},
		{"unknown primary aggregation", mutateResult(t, validResultJSON(), `"aggregation": "p95", "unit": "ms", "value": 40`, `"aggregation": "p99", "unit": "ms", "value": 40`)},
		{"malformed primary unit", mutateResult(t, validResultJSON(), `"aggregation": "p95", "unit": "ms", "value": 40`, `"aggregation": "p95", "unit": "m s", "value": 40`)},
		{"bad definition digest", mutateResult(t, validResultJSON(), `"definition_digest": "`+digestOf("a")+`"`, `"definition_digest": "not-a-digest"`)},
		{"bad observations digest", mutateResult(t, validResultJSON(), `"observations_digest": "`+digestOf("b")+`"`, `"observations_digest": "not-a-digest"`)},
		{"bound guard not id", mutateResult(t, validResultJSON(), `"guard": "peak-rss"`, `"guard": "Peak_RSS"`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeResult([]byte(tt.doc)); err == nil {
				t.Errorf("DecodeResult(%s) = nil error, want error", tt.name)
			}
		})
	}
}

// mutateResultCandidatesBlock replaces the entire multi-line "candidates"
// array in doc with replacement — used where a single string.Replace
// anchor would be too fragile across the block's several lines.
func mutateResultCandidatesBlock(t *testing.T, doc, replacement string) string {
	t.Helper()
	start := strings.Index(doc, `"candidates": [`)
	if start < 0 {
		t.Fatalf("fixture does not contain a candidates array")
	}
	end := strings.Index(doc[start:], "],\n")
	if end < 0 {
		t.Fatalf("fixture candidates array is not terminated as expected")
	}
	end += start + len("],\n")
	return doc[:start] + `"candidates": ` + replacement + ",\n" + doc[end:]
}
