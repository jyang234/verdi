package experimentdecision

import (
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
)

func measuredV2(obs []experiment.Observation) []experiment.Observation {
	out := append([]experiment.Observation(nil), obs...)
	for i := range out {
		out[i].Schema = experiment.ObservationSchemaV2
		out[i].Outcome = &experiment.CandidateOutcome{Kind: experiment.OutcomeCompleted}
	}
	return out
}

func failMeasuredAttempt(obs []experiment.Observation, candidate string, round int, kind experiment.OutcomeKind, witness string) {
	for i := range obs {
		if obs[i].Candidate == candidate && obs[i].Round == round {
			obs[i].Outcome = &experiment.CandidateOutcome{Kind: kind, Witness: &witness}
			obs[i].Guards = []experiment.GuardResult{}
			obs[i].Measurements = []experiment.Measurement{}
			return
		}
	}
	panic("missing measured attempt")
}

func decisionCandidate(t *testing.T, d experiment.ResultDecision, id string) experiment.DecisionCandidate {
	t.Helper()
	for _, candidate := range d.Candidates {
		if candidate.ID == id {
			return candidate
		}
	}
	t.Fatalf("decision has no candidate %q", id)
	return experiment.DecisionCandidate{}
}

func TestEvaluateV2BaselineFailurePreservesOutcomeAndCompletedRounds(t *testing.T) {
	def := lockDefinition(t)
	obs := measuredV2(happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	))
	failMeasuredAttempt(obs, "baseline", 2, experiment.OutcomeCandidateCrash, "exit status 139")

	res, err := Evaluate(def, obs, attestation(def))
	if err != nil {
		t.Fatalf("Evaluate(): %v", err)
	}
	decision, err := experiment.DecisionFromResult(res, obs)
	if err != nil {
		t.Fatalf("DecisionFromResult(): %v", err)
	}
	if decision.Verdict != experiment.VerdictViolatedWithWitness || len(decision.Reasons) != 1 {
		t.Fatalf("decision verdict/reasons = %q/%+v", decision.Verdict, decision.Reasons)
	}
	reason := decision.Reasons[0]
	if reason.Code != experiment.ReasonBaselineCandidateFailure || reason.Candidate != "baseline" || reason.Outcome != experiment.OutcomeCandidateCrash || reason.Round != 2 || reason.Witness == nil || *reason.Witness != "exit status 139" {
		t.Fatalf("baseline failure reason = %+v", reason)
	}
	baseline := decisionCandidate(t, decision, "baseline")
	if baseline.Eligible || baseline.Primary == nil || baseline.Primary.Rounds != 2 || len(baseline.ExecutionFailures) != 1 {
		t.Fatalf("baseline decision row = %+v", baseline)
	}
}

func TestEvaluateV2NonBaselineFailureDoesNotBlockOtherWinner(t *testing.T) {
	def := threeCandidateDef(t)
	obs := measuredV2(threeCandidateObs(t, def,
		[]float64{40, 42, 41}, []float64{18, 19, 17}, []float64{10, 11, 9},
	))
	failMeasuredAttempt(obs, "candidate-a", 2, experiment.OutcomeCandidateTimeout, "attempt timed out")

	res, err := Evaluate(def, obs, attestation(def))
	if err != nil {
		t.Fatalf("Evaluate(): %v", err)
	}
	decision, err := experiment.DecisionFromResult(res, obs)
	if err != nil {
		t.Fatalf("DecisionFromResult(): %v", err)
	}
	if decision.Verdict != experiment.VerdictProvenWinner || decision.Winner != "candidate-b" {
		t.Fatalf("decision verdict/winner = %q/%q", decision.Verdict, decision.Winner)
	}
	failed := decisionCandidate(t, decision, "candidate-a")
	if failed.Eligible || failed.Primary == nil || failed.Primary.Rounds != 2 || len(failed.ExecutionFailures) != 1 || failed.ExecutionFailures[0].Kind != experiment.OutcomeCandidateTimeout {
		t.Fatalf("candidate-a decision row = %+v", failed)
	}
}

func TestObservationsDigestBindsV2Outcome(t *testing.T) {
	def := lockDefinition(t)
	obs := measuredV2(happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	))
	completed, err := ObservationsDigest(def, obs)
	if err != nil {
		t.Fatal(err)
	}
	failMeasuredAttempt(obs, "candidate-a", 2, experiment.OutcomeCandidateCrash, "crashed")
	failed, err := ObservationsDigest(def, obs)
	if err != nil {
		t.Fatal(err)
	}
	if completed == failed {
		t.Fatalf("outcome mutation did not change observations digest %q", completed)
	}
}
