package constitutionimpact

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestCoverageCanonicalRoundTripReusesNestedOwnerCodecs(t *testing.T) {
	consumer := testConsumer("spec/registered", "local")
	plan := testPlan(t, []Consumer{consumer}, []Consumer{consumer}, testChangedLayer())
	identity, _ := consumer.Identity()
	result := resultForPlan(t, plan, true)
	coverage := plan.Complete(
		[]Evaluation{{ConsumerIdentity: identity, Consumer: consumer, AcceptedManifestDigest: result.Report.Input.Target.Accepted.ManifestDigest, Result: result}},
		[]SupplementalTarget{testSupplemental(consumer, result)},
	)
	encoded, err := EncodeCoverage(coverage)
	if err != nil {
		t.Fatalf("EncodeCoverage: %v", err)
	}
	decoded, err := DecodeCoverage(encoded)
	if err != nil {
		t.Fatalf("DecodeCoverage: %v", err)
	}
	again, err := EncodeCoverage(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, again) {
		t.Fatalf("coverage round trip changed bytes")
	}
	var wire struct {
		SupplementalTargets []map[string]json.RawMessage `json:"supplemental_targets"`
	}
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.SupplementalTargets) != 1 {
		t.Fatalf("supplemental targets = %d, want 1", len(wire.SupplementalTargets))
	}
	for _, key := range []string{"accepted_manifest_digest", "report", "request", "request_digest"} {
		if _, ok := wire.SupplementalTargets[0][key]; !ok {
			t.Fatalf("supplemental target missing %q: %s", key, encoded)
		}
	}
	for _, key := range []string{"consumer", "environment", "governed_operations"} {
		if _, ok := wire.SupplementalTargets[0][key]; ok {
			t.Fatalf("supplemental target invented registered-consumer field %q: %s", key, encoded)
		}
	}
	if decoded.Accepted.Commit != testAcceptedCommit || decoded.Proposed.Commit != testProposedCommit || decoded.Accepted.Tree != testAcceptedTree || decoded.Proposed.Tree != testProposedTree {
		t.Fatalf("coverage lost exact tree identities: %+v %+v", decoded.Accepted, decoded.Proposed)
	}

	decoded.Consumers[0].GovernedOperations[0] = "mutated"
	decoded.Evaluations[0].Consumer.Request.Scope.Phases[0] = "review"
	decoded.SupplementalTargets[0].Request.Scope.Phases[0] = "review"
	againDecoded, err := DecodeCoverage(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if againDecoded.Consumers[0].GovernedOperations[0] != "make-verify" || againDecoded.Evaluations[0].Consumer.Request.Scope.Phases[0] != "build" || againDecoded.SupplementalTargets[0].Request.Scope.Phases[0] != "build" {
		t.Fatal("DecodeCoverage returned aliased nested values")
	}
}

func TestDecodeCoverageRejectsSupplementalRequestDigestMismatch(t *testing.T) {
	consumer := testConsumer("spec/registered", "local")
	plan := testPlan(t, []Consumer{consumer}, []Consumer{consumer}, testChangedLayer())
	result := resultForPlan(t, plan, true)
	raw, err := EncodeCoverage(plan.Complete(
		[]Evaluation{testEvaluation(consumer, result)},
		[]SupplementalTarget{testSupplemental(consumer, result)},
	))
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		SupplementalTargets []struct {
			RequestDigest string `json:"request_digest"`
		} `json:"supplemental_targets"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	want := wire.SupplementalTargets[0].RequestDigest
	corrupt := "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	mutated := bytes.Replace(raw, []byte(`"request_digest":"`+want+`"`), []byte(`"request_digest":"`+corrupt+`"`), 1)
	if _, err := DecodeCoverage(mutated); err == nil {
		t.Fatal("DecodeCoverage accepted a supplemental request digest mismatch")
	}
}

func TestCoverageMissingInventoryFixture(t *testing.T) {
	coverage := Coverage{
		Schema:   CoverageSchema,
		Accepted: InventoryEvidence{Commit: testAcceptedCommit, Tree: testAcceptedTree, Presence: PresenceMissing},
		Proposed: InventoryEvidence{Commit: testProposedCommit, Tree: testProposedTree, Presence: PresenceMissing},
		Layers:   []LayerChange{}, Consumers: []Consumer{}, Evaluations: []EvaluationCoverage{}, SupplementalTargets: []SupplementalTarget{},
		State: StateDisclosedUnproven,
		Reasons: []Reason{
			{Code: ReasonAcceptedInventoryMissing, Witnesses: []string{InventoryPath}},
			{Code: ReasonProposedInventoryMissing, Witnesses: []string{InventoryPath}},
		},
	}
	encoded, err := EncodeCoverage(coverage)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/coverage-missing.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("coverage fixture mismatch\n got: %s\nwant: %s", encoded, want)
	}
	if _, err := DecodeCoverage(want); err != nil {
		t.Fatalf("DecodeCoverage fixture: %v", err)
	}
}

func TestDecodeCoverageStrictNegatives(t *testing.T) {
	raw, err := os.ReadFile("testdata/coverage-missing.json")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"unknown field":           bytes.Replace(raw, []byte(`"schema":`), []byte(`"unknown":true,"schema":`), 1),
		"duplicate key":           bytes.Replace(raw, []byte(`"state":"disclosed-as-unproven"`), []byte(`"state":"disclosed-as-unproven","state":"disclosed-as-unproven"`), 1),
		"explicit null":           bytes.Replace(raw, []byte(`"consumers":[]`), []byte(`"consumers":null`), 1),
		"trailing data":           append(append([]byte(nil), raw...), []byte(`{}`)...),
		"noncanonical":            bytes.Replace(raw, []byte(`{"accepted":`), []byte(`{ "accepted":`), 1),
		"unknown state":           bytes.Replace(raw, []byte(`"state":"disclosed-as-unproven"`), []byte(`"state":"maybe"`), 1),
		"missing required reason": bytes.Replace(raw, []byte(`{"code":"accepted-inventory-missing","witnesses":[".verdi/constitution/consumers.json"]},`), nil, 1),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeCoverage(input); err == nil {
				t.Fatal("DecodeCoverage unexpectedly succeeded")
			}
		})
	}
}

func TestDecodeCoverageRejectsUnknownNestedRequestAndReportFields(t *testing.T) {
	consumer := testConsumer("spec/registered", "local")
	plan := testPlan(t, []Consumer{consumer}, []Consumer{consumer}, testChangedLayer())
	identity, _ := consumer.Identity()
	result := resultForPlan(t, plan, true)
	raw, err := EncodeCoverage(plan.Complete([]Evaluation{{ConsumerIdentity: identity, Consumer: consumer, AcceptedManifestDigest: result.Report.Input.Target.Accepted.ManifestDigest, Result: result}}, nil))
	if err != nil {
		t.Fatal(err)
	}
	requestUnknown := bytes.Replace(raw, []byte(`"adapter":`), []byte(`"unknown":true,"adapter":`), 1)
	reportUnknown := bytes.Replace(raw, []byte(`"digest":`), []byte(`"unknown":true,"digest":`), 1)
	for name, input := range map[string][]byte{"request": requestUnknown, "report": reportUnknown} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeCoverage(input); err == nil {
				t.Fatal("DecodeCoverage accepted unknown nested owner field")
			}
		})
	}
}
