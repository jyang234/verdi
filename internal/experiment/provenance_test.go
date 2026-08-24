package experiment

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/governanceprincipal"
)

func TestProvenanceStrictCanonicalAllowedDecision(t *testing.T) {
	record := ProvenanceRecord{
		Schema: ProvenanceSchema,
		Experiment: ProvenanceExperiment{
			Spike: "spec/request-path-spike",
			ID:    "request-path-v2",
		},
		Operation:      MutationDraftDefinition,
		PreviousDigest: digestOf("a"),
		ResultDigest:   digestOf("b"),
		PolicyDigest:   digestOf("c"),
		PolicyDecision: ProvenancePolicyDecision{State: PolicyAllowed, Reasons: []ProvenancePolicyReason{}},
		Attribution:    governanceprincipal.NewUnauthenticatedAttribution(),
		Harness:        "codex",
		Session:        "session-1",
		Paths:          []string{"experiment.yaml"},
	}
	if err := record.Seal(); err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	encoded, err := EncodeProvenanceRecord(record)
	if err != nil {
		t.Fatalf("EncodeProvenanceRecord() error = %v", err)
	}
	decoded, err := DecodeProvenanceLog(encoded)
	if err != nil || len(decoded) != 1 || decoded[0].Digest != record.Digest {
		t.Fatalf("DecodeProvenanceLog() = %#v, %v", decoded, err)
	}

	for _, malformed := range []string{
		strings.Replace(string(encoded), `"policy_decision":{"reasons":[],"state":"allowed"},`, "", 1),
		strings.Replace(string(encoded), `"policy_decision":{"reasons":[],"state":"allowed"}`, `"policy_decision":null`, 1),
		strings.Replace(string(encoded), `"policy_decision":{"reasons":[],"state":"allowed"}`, `"policy_decision":{"state":"allowed"}`, 1),
		strings.Replace(string(encoded), `"reasons":[]`, `"reasons":null`, 1),
		strings.Replace(string(encoded), `"reasons":[]`, `"reasons":["invented"]`, 1),
		strings.Replace(string(encoded), `"state":"allowed"`, `"state":"unknown"`, 1),
		strings.Replace(string(encoded), `{"attribution"`, `{ "attribution"`, 1),
	} {
		if _, err := DecodeProvenanceLog([]byte(malformed)); err == nil {
			t.Fatalf("DecodeProvenanceLog(%q) succeeded", malformed)
		}
	}
}
