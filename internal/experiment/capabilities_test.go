package experiment

import (
	"strings"
	"testing"
)

func validCapabilitiesJSON() string {
	return `{
  "schema": "verdi.experiment-evaluator-capabilities/v1",
  "protocol_versions": ["verdi.experiment-evaluator-capabilities/v1", "verdi.experiment-observation/v1"],
  "metrics": [
    {"id": "request-latency", "type": "duration", "unit": "ms", "direction": "lower"},
    {"id": "peak-rss", "type": "bytes", "unit": "bytes", "direction": "lower"}
  ],
  "guards": ["behavioral-equivalence", "tenant-isolation"],
  "observers": ["process-observer"],
  "workload_inputs": ["representative-request-mix"],
  "environment": ["CACHE_TTL"],
  "requires_network": false,
  "requires_elevated": false
}`
}

func mutateCapabilities(t *testing.T, old, replacement string) string {
	t.Helper()
	doc := validCapabilitiesJSON()
	if !strings.Contains(doc, old) {
		t.Fatalf("fixture does not contain %q", old)
	}
	return strings.Replace(doc, old, replacement, 1)
}

func TestDecodeCapabilitiesHappyPath(t *testing.T) {
	c, err := DecodeCapabilities([]byte(validCapabilitiesJSON()))
	if err != nil {
		t.Fatalf("DecodeCapabilities() unexpected error: %v", err)
	}
	if len(c.Metrics) != 2 {
		t.Fatalf("len(c.Metrics) = %d, want 2", len(c.Metrics))
	}
	if c.RequiresNetwork || c.RequiresElevated {
		t.Errorf("c.RequiresNetwork=%v c.RequiresElevated=%v, want both false", c.RequiresNetwork, c.RequiresElevated)
	}
}

// TestDecodeCapabilitiesEmptyMetricsList proves the distinction the
// presence check draws: an explicitly EMPTY metrics list is a valid answer
// ("no metrics"), while an absent key is not an answer at all.
func TestDecodeCapabilitiesEmptyMetricsList(t *testing.T) {
	doc := mutateCapabilities(t, `"metrics": [
    {"id": "request-latency", "type": "duration", "unit": "ms", "direction": "lower"},
    {"id": "peak-rss", "type": "bytes", "unit": "bytes", "direction": "lower"}
  ]`, `"metrics": []`)
	c, err := DecodeCapabilities([]byte(doc))
	if err != nil {
		t.Fatalf("DecodeCapabilities() unexpected error: %v", err)
	}
	if len(c.Metrics) != 0 {
		t.Errorf("len(c.Metrics) = %d, want 0", len(c.Metrics))
	}
}

func TestDecodeCapabilitiesRejects(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{"unknown schema", mutateCapabilities(t, `"schema": "verdi.experiment-evaluator-capabilities/v1"`, `"schema": "verdi.experiment-evaluator-capabilities/v2"`)},
		{"unknown field", strings.TrimSuffix(validCapabilitiesJSON(), "}") + `,"unknown_field": true}`},
		{"trailing data", validCapabilitiesJSON() + "\n{}"},
		{"empty protocol_versions", mutateCapabilities(t, `"protocol_versions": ["verdi.experiment-evaluator-capabilities/v1", "verdi.experiment-observation/v1"]`, `"protocol_versions": []`)},
		{"unrecognized protocol version (evaluator/v1)", mutateCapabilities(t, `"verdi.experiment-observation/v1"`, `"verdi.experiment-evaluator/v1"`)},
		{"arbitrary unrecognized protocol version", mutateCapabilities(t, `"verdi.experiment-observation/v1"`, `"verdi.something-else/v1"`)},
		{"unknown metric type", mutateCapabilities(t, `"type": "duration"`, `"type": "percentage"`)},
		{"unknown metric direction", mutateCapabilities(t, `"direction": "lower"`, `"direction": "sideways"`)},
		{"malformed unit", mutateCapabilities(t, `"unit": "ms"`, `"unit": "m s"`)},
		{"duplicate metric ids", mutateCapabilities(t, `{"id": "peak-rss", "type": "bytes", "unit": "bytes", "direction": "lower"}`, `{"id": "request-latency", "type": "bytes", "unit": "bytes", "direction": "lower"}`)},
		{"duplicate guard ids", mutateCapabilities(t, `"guards": ["behavioral-equivalence", "tenant-isolation"]`, `"guards": ["behavioral-equivalence", "behavioral-equivalence"]`)},
		{"duplicate observer ids", mutateCapabilities(t, `"observers": ["process-observer"]`, `"observers": ["process-observer", "process-observer"]`)},
		{"duplicate workload input ids", mutateCapabilities(t, `"workload_inputs": ["representative-request-mix"]`, `"workload_inputs": ["representative-request-mix", "representative-request-mix"]`)},
		{"empty environment entry", mutateCapabilities(t, `"environment": ["CACHE_TTL"]`, `"environment": [""]`)},
		{"missing schema", strings.Replace(validCapabilitiesJSON(), `"schema": "verdi.experiment-evaluator-capabilities/v1",`, "", 1)},
		// AC-3 declares metrics as required response content: an evaluator
		// that never mentions the key has not answered the question, which
		// is not the same as answering "I support no metrics" ([]).
		{"metrics key missing", mutateCapabilities(t, `"metrics": [
    {"id": "request-latency", "type": "duration", "unit": "ms", "direction": "lower"},
    {"id": "peak-rss", "type": "bytes", "unit": "bytes", "direction": "lower"}
  ],`, "")},
		{"metrics explicit null", mutateCapabilities(t, `"metrics": [
    {"id": "request-latency", "type": "duration", "unit": "ms", "direction": "lower"},
    {"id": "peak-rss", "type": "bytes", "unit": "bytes", "direction": "lower"}
  ]`, `"metrics": null`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeCapabilities([]byte(tt.doc)); err == nil {
				t.Errorf("DecodeCapabilities(%s) = nil error, want error", tt.name)
			}
		})
	}
}
