package constitutionimpact

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyconflict"
)

func TestCompleteCoverageClosedEvaluationStates(t *testing.T) {
	consumer := testConsumer("spec/registered", "local")
	plan := testPlan(t, []Consumer{consumer}, []Consumer{consumer}, testChangedLayer())
	identity, _ := consumer.Identity()
	pass, manifest := resultForConsumer(t, plan, consumer, true)
	valid := testEvaluation(consumer, pass, manifest)

	tests := []struct {
		name        string
		evaluations []Evaluation
		wantState   State
		wantReason  ReasonCode
	}{
		{name: "complete pass is proven", evaluations: []Evaluation{valid}, wantState: StateProven},
		{name: "explicit unknown is unproven", evaluations: []Evaluation{{
			ConsumerIdentity: identity, Consumer: consumer,
			Refusal: &EvaluationRefusal{Code: ReasonEvaluationUnresolved, Witnesses: []string{"judge-unavailable"}},
		}}, wantState: StateDisclosedUnproven, wantReason: ReasonEvaluationUnresolved},
		{name: "omitted is violated", wantState: StateViolatedWithWitness, wantReason: ReasonEvaluationOmitted},
		{name: "extra is violated", evaluations: []Evaluation{valid, {
			ConsumerIdentity: "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			Consumer:         consumer, Result: pass,
		}}, wantState: StateViolatedWithWitness, wantReason: ReasonEvaluationExtra},
		{name: "duplicate is violated", evaluations: []Evaluation{valid, valid}, wantState: StateViolatedWithWitness, wantReason: ReasonEvaluationDuplicate},
		{name: "identity mismatch is violated", evaluations: []Evaluation{{
			ConsumerIdentity: identity, Consumer: testConsumer("spec/registered", "production"),
			AcceptedManifestBytes: manifest, Result: pass,
		}}, wantState: StateViolatedWithWitness, wantReason: ReasonEvaluationIdentityMismatch},
		{name: "invalid result is violated", evaluations: []Evaluation{{
			ConsumerIdentity: identity, Consumer: consumer,
			AcceptedManifestBytes: manifest,
			Result:                &policyconflict.Result{Report: pass.Report, ReportBytes: []byte("not-the-report")},
		}}, wantState: StateViolatedWithWitness, wantReason: ReasonEvaluationResultInvalid},
		{name: "operand mismatch is violated", evaluations: []Evaluation{{
			ConsumerIdentity: identity, Consumer: consumer,
			AcceptedManifestBytes: manifest, Result: mismatchedResult(t, pass),
		}}, wantState: StateViolatedWithWitness, wantReason: ReasonEvaluationOperandMismatch},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			coverage := plan.Complete(test.evaluations, nil)
			if coverage.State != test.wantState {
				t.Fatalf("state = %q, want %q; reasons=%+v", coverage.State, test.wantState, coverage.Reasons)
			}
			if test.wantReason != "" && !hasReasonCode(coverage.Reasons, test.wantReason) {
				t.Fatalf("reasons = %+v, want %q", coverage.Reasons, test.wantReason)
			}
			if len(coverage.Evaluations) != 1 {
				t.Fatalf("evaluation rows = %d, want exact registered union size", len(coverage.Evaluations))
			}
		})
	}
}

func TestCompleteCoverageTreatsConflictVerdictSeparatelyFromCompleteness(t *testing.T) {
	consumer := testConsumer("spec/registered", "local")
	plan := testPlan(t, []Consumer{consumer}, []Consumer{consumer}, testChangedLayer())
	identity, _ := consumer.Identity()
	blockedUnproven, manifest := resultForConsumer(t, plan, consumer, false)
	coverage := plan.Complete([]Evaluation{{
		ConsumerIdentity: identity, Consumer: consumer,
		AcceptedManifestBytes: manifest, Result: blockedUnproven,
	}}, nil)
	if coverage.State != StateDisclosedUnproven {
		t.Fatalf("coverage state = %q, want blocked-unproven evaluation disclosed as unproven", coverage.State)
	}
	if coverage.Evaluations[0].Report == nil || coverage.Evaluations[0].Report.Verdict != policyconflict.VerdictBlockedUnproven {
		t.Fatalf("report = %+v", coverage.Evaluations[0].Report)
	}
	if !hasReason(coverage.Reasons, ReasonEvaluationUnresolved, identity) {
		t.Fatalf("reasons = %+v, want deterministic unresolved witness for %s", coverage.Reasons, identity)
	}
}

func TestCompleteCoverageKeepsBlockedViolatedEvaluationProven(t *testing.T) {
	consumer := testConsumer("spec/registered", "local")
	plan := testPlan(t, []Consumer{consumer}, []Consumer{consumer}, testChangedLayer())
	base, manifest := resultForConsumer(t, plan, consumer, true)
	blockedViolated := blockedViolatedResult(t, base)

	coverage := plan.Complete([]Evaluation{testEvaluation(consumer, blockedViolated, manifest)}, nil)
	if coverage.State != StateProven || len(coverage.Reasons) != 0 {
		t.Fatalf("coverage = %+v, want complete blocked-violated evaluation to remain proven", coverage)
	}
	if coverage.Evaluations[0].Report == nil || coverage.Evaluations[0].Report.Verdict != policyconflict.VerdictBlockedViolated {
		t.Fatalf("report = %+v, want blocked-violated", coverage.Evaluations[0].Report)
	}
}

func TestCompleteCoverageRefusesReportReplayAcrossDistinctConsumers(t *testing.T) {
	first := testConsumer("spec/first", "local")
	second := testConsumer("spec/second", "production")
	plan := testPlan(t, []Consumer{first, second}, []Consumer{first, second}, testChangedLayer())
	result, manifest := resultForConsumer(t, plan, first, true)

	coverage := plan.Complete([]Evaluation{
		testEvaluation(first, result, manifest),
		testEvaluation(second, result, manifest),
	}, nil)

	if coverage.State != StateViolatedWithWitness {
		t.Fatalf("coverage state = %q, want replay violation; reasons=%+v", coverage.State, coverage.Reasons)
	}
	secondID, err := second.Identity()
	if err != nil {
		t.Fatal(err)
	}
	if !hasReason(coverage.Reasons, ReasonEvaluationOperandMismatch, secondID) {
		t.Fatalf("reasons = %+v, want operand mismatch for replayed second consumer %s", coverage.Reasons, secondID)
	}
}

func TestCompleteCoverageRefusesSupplementalReportFromForeignProposal(t *testing.T) {
	consumer := testConsumer("spec/registered", "local")
	plan := testPlan(t, []Consumer{consumer}, []Consumer{consumer}, testChangedLayer())
	result, manifest := resultForConsumer(t, plan, consumer, true)
	foreign := mismatchedResult(t, result)

	coverage := plan.Complete(
		[]Evaluation{testEvaluation(consumer, result, manifest)},
		[]SupplementalTarget{testSupplemental(consumer, foreign, manifest)},
	)

	if coverage.State != StateProven {
		t.Fatalf("canonical coverage state = %q, want supplemental evidence not to alter completeness", coverage.State)
	}
	if got := coverage.SupplementalTargets[0]; got.Result != nil || got.Refusal == nil || got.Refusal.Code != ReasonEvaluationOperandMismatch {
		t.Fatalf("foreign supplemental result = %+v, want typed operand-mismatch refusal", got)
	}
}

func TestCompleteCoverageIsDeterministicAndAliasSafe(t *testing.T) {
	first := testConsumer("spec/first", "local")
	second := testConsumer("spec/second", "production")
	plan := testPlan(t, []Consumer{first, second}, []Consumer{second, first}, testChangedLayer())
	firstResult, firstManifest := resultForConsumer(t, plan, first, true)
	secondResult, secondManifest := resultForConsumer(t, plan, second, true)
	firstID, _ := first.Identity()
	secondID, _ := second.Identity()
	evaluations := []Evaluation{testEvaluation(second, secondResult, secondManifest), testEvaluation(first, firstResult, firstManifest)}
	if evaluations[0].ConsumerIdentity != secondID || evaluations[1].ConsumerIdentity != firstID {
		t.Fatal("test setup: identities differ")
	}
	supplemental := []SupplementalTarget{testSupplemental(second, secondResult, secondManifest), testSupplemental(first, firstResult, firstManifest)}
	coverage := plan.Complete(evaluations, supplemental)
	if coverage.State != StateProven {
		t.Fatalf("coverage = %+v", coverage)
	}
	if coverage.Evaluations[0].ConsumerIdentity >= coverage.Evaluations[1].ConsumerIdentity {
		t.Fatal("evaluations did not follow canonical union order")
	}
	left, _ := supplementalRequestDigest(coverage.SupplementalTargets[0].Request)
	right, _ := supplementalRequestDigest(coverage.SupplementalTargets[1].Request)
	if left > right {
		t.Fatal("supplemental targets were not canonicalized")
	}

	evaluations[0].Consumer.GovernedOperations[0] = "mutated"
	supplemental[0].Request.Scope.Phases[0] = "review"
	if coverage.Evaluations[1].Consumer.GovernedOperations[0] != "make-verify" || coverage.SupplementalTargets[0].Request.Scope.Phases[0] != "build" {
		t.Fatal("coverage retained aliases to caller inputs")
	}
}

func mismatchedResult(t *testing.T, original *policyconflict.Result) *policyconflict.Result {
	t.Helper()
	report := original.Report
	report.Input.Repository.Head.Value = "dddddddddddddddddddddddddddddddddddddddd"
	report.Digest = ""
	encoded, err := policyconflict.EncodeReport(report)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := policyconflict.DecodeReport(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return &policyconflict.Result{Report: canonical, ReportBytes: encoded}
}

func blockedViolatedResult(t *testing.T, original *policyconflict.Result) *policyconflict.Result {
	t.Helper()
	scope := policyartifact.Scope{
		Phases: []string{policyartifact.PhaseBuild}, Environments: []string{"local"},
		Paths: []string{}, Refs: []string{},
	}
	claims := []contextcompile.TypedClaim{
		typedConfigurationClaim(t, "policy-a", "first", "gold", scope),
		typedConfigurationClaim(t, "policy-b", "second", "silver", scope),
	}
	mechanical, err := policyconflict.EvaluateMechanical(context.Background(), policyconflict.MechanicalInput{Claims: claims})
	if err != nil {
		t.Fatal(err)
	}
	if len(mechanical.Evaluations) != 1 || mechanical.Evaluations[0].State != policyconflict.ProofViolatedWithWitness {
		t.Fatalf("mechanical setup = %+v, want one violated row", mechanical.Evaluations)
	}
	report := original.Report
	report.Mechanical = mechanical.Evaluations
	report.Semantic = []policyconflict.SemanticEvaluation{}
	report.Disclosures = []policyconflict.Disclosure{}
	report.Verdict = policyconflict.VerdictBlockedViolated
	report.Digest = ""
	encoded, err := policyconflict.EncodeReport(report)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := policyconflict.DecodeReport(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return &policyconflict.Result{Report: canonical, ReportBytes: encoded}
}

func typedConfigurationClaim(t *testing.T, policyID, claimID, value string, scope policyartifact.Scope) contextcompile.TypedClaim {
	t.Helper()
	claim := policyartifact.Claim{
		ID: claimID, Family: policyartifact.FamilyConfiguration, Operator: policyartifact.OpEquals,
		Subject: "go-toolchain", Values: []string{value}, Scope: scope,
	}
	digest, err := policyartifact.ClaimDigest(claim)
	if err != nil {
		t.Fatal(err)
	}
	return contextcompile.TypedClaim{
		PolicyID:     policyID,
		PolicyDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ClaimDigest:  digest,
		Claim:        claim,
	}
}
