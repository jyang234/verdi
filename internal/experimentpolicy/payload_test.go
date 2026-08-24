package experimentpolicy

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/policyartifact"
)

func TestExperimentPolicyPayloadStrictGrammar(t *testing.T) {
	valid := readPayloadFixture(t, "organization.yaml")
	payload, err := DecodePayload(valid)
	if err != nil {
		t.Fatalf("DecodePayload(valid) error = %v", err)
	}
	if payload.PayloadKind() != PayloadKind {
		t.Fatalf("PayloadKind() = %q, want %q", payload.PayloadKind(), PayloadKind)
	}
	if cardinality, ok := policyartifact.RegisteredPayloadCardinality(PayloadKind); !ok || cardinality != policyartifact.PayloadLayered {
		t.Fatalf("registered cardinality = %q, %t; want layered, true", cardinality, ok)
	}
	if len(payload.Environments) != 1 {
		t.Fatalf("environments = %#v, want one", payload.Environments)
	}
	grantBytes := payload.Environments[0].Grants
	if _, err := execworkspace.DecodeGrantSet(grantBytes); err != nil {
		t.Fatalf("payload grant bytes do not round-trip through shared grant owner: %v", err)
	}
	grantBytes[0] ^= 0xff
	again, err := DecodePayload(valid)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(grantBytes, again.Environments[0].Grants) {
		t.Fatal("decoded payload grant bytes alias a previous decode")
	}

	tests := []struct {
		name   string
		mutate func(string) string
		want   string
	}{
		{"unknown field", func(s string) string { return s + "unknown: true\n" }, "field unknown not found"},
		{"missing list", func(s string) string {
			return strings.Replace(s, "classes: [database, request-path-performance]\n", "", 1)
		}, "classes is missing"},
		{"null list", func(s string) string {
			return strings.Replace(s, "classes: [database, request-path-performance]", "classes: null", 1)
		}, "classes is missing or null"},
		{"unsorted list", func(s string) string {
			return strings.Replace(s, "classes: [database, request-path-performance]", "classes: [request-path-performance, database]", 1)
		}, "sorted"},
		{"duplicate list", func(s string) string {
			return strings.Replace(s, "classes: [database, request-path-performance]", "classes: [database, database]", 1)
		}, "duplicate"},
		{"unknown protocol", func(s string) string {
			return strings.Replace(s, "verdi.experiment-evaluator/v1", "verdi.experiment-evaluator/v999", 1)
		}, "unknown evaluator protocol"},
		{"unknown source", func(s string) string { return strings.Replace(s, "evaluator-measured", "oracle-measured", 1) }, "unknown measurement source"},
		{"nonpositive observation limit", func(s string) string {
			return strings.Replace(s, "observation_bytes: 524288", "observation_bytes: 0", 1)
		}, "observation_bytes must be > 0"},
		{"nonpositive retained limit", func(s string) string {
			return strings.Replace(s, "retained_artifact_bytes: 16777216", "retained_artifact_bytes: -1", 1)
		}, "retained_artifact_bytes must be > 0"},
		{"unknown grant kind", func(s string) string { return strings.Replace(s, "kind: process-execution", "kind: root-shell", 1) }, "unknown grant kind"},
		{"duplicate environment map key", func(s string) string {
			return strings.Replace(s, "declared_environment: {GOMAXPROCS: \"1\"}", "declared_environment: {GOMAXPROCS: \"1\", GOMAXPROCS: \"2\"}", 1)
		}, "already defined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodePayload([]byte(tt.mutate(string(valid))))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("DecodePayload() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func readPayloadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return data
}
