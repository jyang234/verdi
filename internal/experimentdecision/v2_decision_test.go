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
		for j := range out[i].Measurements {
			if out[i].Measurements[j].Source == experiment.SourceHarnessMeasured {
				out[i].Measurements[j].ID = experiment.EvaluatorPeakRSSMetricID
				out[i].Measurements[j].Unit = "bytes"
			}
		}
	}
	return out
}

func failMeasuredAttempt(obs []experiment.Observation, candidate string, round int, kind experiment.OutcomeKind, witness string) {
	for i := range obs {
		if obs[i].Candidate == candidate && obs[i].Round == round {
			obs[i].Outcome = &experiment.CandidateOutcome{Kind: kind, Witness: &witness}
			obs[i].Guards = []experiment.GuardResult{}
			obs[i].Measurements = []experiment.Measurement{
				measurement(experiment.EvaluatorWallDurationMetricID, 1, "ns", experiment.SourceHarnessMeasured),
				measurement(experiment.EvaluatorPeakRSSMetricID, 1, "bytes", experiment.SourceHarnessMeasured),
			}
			obs[i].Disclosures = []string{}
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

func threeCandidateV2Def(t *testing.T) experiment.Definition {
	t.Helper()
	return lockV2Definition(t, func(def *experiment.Definition) {
		def.Candidates = append(def.Candidates, experiment.Candidate{
			ID: "candidate-b", Patch: "candidates/candidate-b.patch", Digest: fixtureDigest("0"), Base: base40,
		})
	})
}

func TestEvaluateV2BaselineFailurePreservesOutcomeAndCompletedRounds(t *testing.T) {
	def := lockV2Definition(t)
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
	def := threeCandidateV2Def(t)
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

func TestEvaluateV2FailureProcessFactsDoNotEnterDecisionAggregates(t *testing.T) {
	def := lockV2Definition(t, func(def *experiment.Definition) {
		def.Decision.PrimaryMetric.ID = experiment.EvaluatorWallDurationMetricID
		def.Decision.PrimaryMetric.Unit = "ns"
		def.Decision.PrimaryMetric.Aggregation = experiment.AggregationMean
	})
	obs := measuredV2(happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	))
	for i := range obs {
		for j := range obs[i].Measurements {
			if obs[i].Measurements[j].ID == experiment.EvaluatorWallDurationMetricID {
				obs[i].Measurements[j].Source = experiment.SourceHarnessMeasured
			}
		}
	}
	witness := "attempt timed out"
	for i := range obs {
		if obs[i].Candidate == "candidate-a" && obs[i].Round == 2 {
			obs[i].Outcome = &experiment.CandidateOutcome{Kind: experiment.OutcomeCandidateTimeout, Witness: &witness}
			obs[i].Guards = []experiment.GuardResult{}
			obs[i].Measurements = []experiment.Measurement{
				measurement(experiment.EvaluatorWallDurationMetricID, 1, "ns", experiment.SourceHarnessMeasured),
				measurement(experiment.EvaluatorPeakRSSMetricID, 1000000, "bytes", experiment.SourceHarnessMeasured),
			}
			break
		}
	}

	res, err := Evaluate(def, obs, attestation(def))
	if err != nil {
		t.Fatalf("Evaluate(): %v", err)
	}
	decision, err := experiment.DecisionFromResult(res, obs)
	if err != nil {
		t.Fatalf("DecisionFromResult(): %v", err)
	}
	failed := decisionCandidate(t, decision, "candidate-a")
	if failed.Primary == nil || failed.Primary.Value != "17.5" || failed.Primary.Rounds != 2 {
		t.Fatalf("candidate-a primary = %+v, want completed-round mean 17.5 over 2 rounds", failed.Primary)
	}
	if len(failed.Bounds) != 1 || failed.Bounds[0].Value != "108" {
		t.Fatalf("candidate-a bounds = %+v, want completed-round peak RSS maximum 108", failed.Bounds)
	}
}

func TestObservationsDigestBindsV2Outcome(t *testing.T) {
	def := lockV2Definition(t)
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
