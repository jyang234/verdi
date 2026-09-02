package constitutionimpact

import (
	"testing"

	"github.com/jyang234/verdi/internal/policyconflict"
)

func TestCompleteCoverageClosedEvaluationStates(t *testing.T) {
	consumer := testConsumer("spec/registered", "local")
	plan := testPlan(t, []Consumer{consumer}, []Consumer{consumer}, testChangedLayer())
	identity, _ := consumer.Identity()
	pass := resultForPlan(t, plan, true)
	valid := testEvaluation(consumer, pass)

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
			AcceptedManifestDigest: pass.Report.Input.Target.Accepted.ManifestDigest, Result: pass,
		}}, wantState: StateViolatedWithWitness, wantReason: ReasonEvaluationIdentityMismatch},
		{name: "invalid result is violated", evaluations: []Evaluation{{
			ConsumerIdentity: identity, Consumer: consumer,
			AcceptedManifestDigest: pass.Report.Input.Target.Accepted.ManifestDigest,
			Result:                 &policyconflict.Result{Report: pass.Report, ReportBytes: []byte("not-the-report")},
		}}, wantState: StateViolatedWithWitness, wantReason: ReasonEvaluationResultInvalid},
		{name: "operand mismatch is violated", evaluations: []Evaluation{{
			ConsumerIdentity: identity, Consumer: consumer,
			AcceptedManifestDigest: pass.Report.Input.Target.Accepted.ManifestDigest, Result: mismatchedResult(t, pass),
		}}, wantState: StateViolatedWithWitness, wantReason: ReasonEvaluationOperandMismatch},
		{name: "context mismatch is violated", evaluations: []Evaluation{{
			ConsumerIdentity: identity, Consumer: consumer,
			AcceptedManifestDigest: "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", Result: pass,
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
	blockedUnproven := resultForPlan(t, plan, false)
	coverage := plan.Complete([]Evaluation{{
		ConsumerIdentity: identity, Consumer: consumer,
		AcceptedManifestDigest: blockedUnproven.Report.Input.Target.Accepted.ManifestDigest, Result: blockedUnproven,
	}}, nil)
	if coverage.State != StateProven {
		t.Fatalf("coverage state = %q, want completeness proven independently of conflict verdict", coverage.State)
	}
	if coverage.Evaluations[0].Report == nil || coverage.Evaluations[0].Report.Verdict != policyconflict.VerdictBlockedUnproven {
		t.Fatalf("report = %+v", coverage.Evaluations[0].Report)
	}
}

func TestCompleteCoverageIsDeterministicAndAliasSafe(t *testing.T) {
	first := testConsumer("spec/first", "local")
	second := testConsumer("spec/second", "production")
	plan := testPlan(t, []Consumer{first, second}, []Consumer{second, first}, testChangedLayer())
	result := resultForPlan(t, plan, true)
	firstID, _ := first.Identity()
	secondID, _ := second.Identity()
	evaluations := []Evaluation{testEvaluation(second, result), testEvaluation(first, result)}
	if evaluations[0].ConsumerIdentity != secondID || evaluations[1].ConsumerIdentity != firstID {
		t.Fatal("test setup: identities differ")
	}
	supplemental := []SupplementalTarget{testSupplemental(second, result), testSupplemental(first, result)}
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
