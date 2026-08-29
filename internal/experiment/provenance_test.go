package experiment

import (
	"bytes"
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

func TestProvenancePreviousFileSnapshotsRoundTripExactBytes(t *testing.T) {
	present, err := NewProvenanceFileSnapshot("experiment.yaml", []byte{0x00, 0xff, '\n'}, true)
	if err != nil {
		t.Fatal(err)
	}
	absent, err := NewProvenanceFileSnapshot("evaluator-capabilities.json", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	record := ProvenanceRecord{
		Schema:     ProvenanceSchema,
		Experiment: ProvenanceExperiment{Spike: "spec/request-path-spike", ID: "request-path-v2"},
		Operation:  MutationProposeRegistration, PreviousDigest: digestOf("a"), ResultDigest: digestOf("b"), PolicyDigest: digestOf("c"),
		PolicyDecision: ProvenancePolicyDecision{State: PolicyAllowed, Reasons: []ProvenancePolicyReason{}},
		Attribution:    governanceprincipal.NewUnauthenticatedAttribution(),
		Paths:          []string{"evaluator-capabilities.json", "experiment.yaml"},
		PreviousFiles:  []ProvenanceFileSnapshot{absent, present},
	}
	if err := record.Seal(); err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeProvenanceRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeProvenanceRecord(bytes.TrimSuffix(encoded, []byte("\n")))
	if err != nil {
		t.Fatal(err)
	}
	got, exists, err := decoded.PreviousFiles[1].Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !exists || !bytes.Equal(got, []byte{0x00, 0xff, '\n'}) {
		t.Fatalf("decoded previous bytes = %x, present=%v", got, exists)
	}
	if got, exists, err := decoded.PreviousFiles[0].Bytes(); err != nil || exists || got != nil {
		t.Fatalf("decoded absent previous file = %x, present=%v, err=%v", got, exists, err)
	}

	for _, mutate := range []func(*ProvenanceRecord){
		func(r *ProvenanceRecord) { r.PreviousFiles = []ProvenanceFileSnapshot{} },
		func(r *ProvenanceRecord) { r.PreviousFiles[1].ContentBase64URL = "AA==" },
		func(r *ProvenanceRecord) {
			r.PreviousFiles[0], r.PreviousFiles[1] = r.PreviousFiles[1], r.PreviousFiles[0]
		},
	} {
		invalid := record
		invalid.PreviousFiles = append([]ProvenanceFileSnapshot(nil), record.PreviousFiles...)
		mutate(&invalid)
		if err := invalid.Seal(); err == nil {
			t.Fatalf("Seal(invalid previous file snapshots) = nil error: %+v", invalid.PreviousFiles)
		}
	}
}
