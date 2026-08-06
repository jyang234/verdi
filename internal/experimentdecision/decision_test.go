package experimentdecision

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/experiment"
)

// isZeroResult reports whether res is experiment.Result's zero value — the
// comparison Evaluate's error paths must satisfy (CO-1: an error never
// carries a Result). Result embeds slices, so it is not == comparable;
// reflect.DeepEqual is the correct zero-value test here.
func isZeroResult(res experiment.Result) bool {
	return reflect.DeepEqual(res, experiment.Result{})
}

// mustEvaluate calls Evaluate and fails the test on an unexpected error.
func mustEvaluate(t *testing.T, def experiment.Definition, obs []experiment.Observation) experiment.Result {
	t.Helper()
	res, err := Evaluate(def, obs)
	if err != nil {
		t.Fatalf("Evaluate() unexpected error: %v", err)
	}
	return res
}

// candidateResult returns the CandidateResult named id, failing the test
// if absent.
func candidateResult(t *testing.T, res experiment.Result, id string) experiment.CandidateResult {
	t.Helper()
	for _, c := range res.Candidates {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("result has no candidate %q", id)
	return experiment.CandidateResult{}
}

// TestEvaluateProvenWinner covers the straightforward two-candidate happy
// path: candidate-a passes every guard, clears the 25% relative
// improvement over baseline, and is trivially separated from the baseline
// runner-up.
func TestEvaluateProvenWinner(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)

	res := mustEvaluate(t, def, obs)
	if err := res.Validate(); err != nil {
		t.Fatalf("res.Validate() unexpected error: %v", err)
	}
	if res.Verdict != experiment.VerdictProvenWinner {
		t.Fatalf("Verdict = %q, want %q", res.Verdict, experiment.VerdictProvenWinner)
	}
	if res.Winner != "candidate-a" {
		t.Fatalf("Winner = %q, want %q", res.Winner, "candidate-a")
	}
	if len(res.Reasons) != 0 {
		t.Fatalf("Reasons = %v, want empty", res.Reasons)
	}
	cand := candidateResult(t, res, "candidate-a")
	if cand.Primary == nil || cand.Primary.Value != "19" {
		t.Fatalf("candidate-a primary = %+v, want value 19", cand.Primary)
	}
	if cand.Primary.Rounds != 3 {
		t.Fatalf("candidate-a primary.Rounds = %d, want 3", cand.Primary.Rounds)
	}
	base := candidateResult(t, res, "baseline")
	if !base.Baseline || base.ID == res.Winner {
		t.Fatalf("baseline candidate result malformed: %+v", base)
	}

	// Round-trip: an Evaluate output always round-trips
	// DecodeResult(canonjson.Marshal(...)) — RenderResult (commit 5) is
	// exactly this composition plus a Validate() call, exercised directly
	// in render_test.go.
	rendered, err := canonjson.Marshal(res)
	if err != nil {
		t.Fatalf("canonjson.Marshal(res) unexpected error: %v", err)
	}
	decoded, err := experiment.DecodeResult(rendered)
	if err != nil {
		t.Fatalf("DecodeResult(canonjson.Marshal(res)) unexpected error: %v", err)
	}
	if decoded.Winner != res.Winner {
		t.Fatalf("round-tripped winner = %q, want %q", decoded.Winner, res.Winner)
	}
}

// TestEvaluateGuardFailureIneligibleWithWitness covers CO-7's guard
// -failure coverage: a non-baseline candidate that fails a required guard
// in any round is ineligible, and its witness is preserved verbatim.
func TestEvaluateGuardFailureIneligibleWithWitness(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)
	// Round 3 (index 2) of candidate-a fails behavioral-equivalence.
	witness := "stale response after policy update in round 3"
	obs[5].Guards = []experiment.GuardResult{guardResult("behavioral-equivalence", false, witness)}

	res := mustEvaluate(t, def, obs)
	if res.Verdict != experiment.VerdictDisclosedUnproven {
		t.Fatalf("Verdict = %q, want %q (no other eligible candidate)", res.Verdict, experiment.VerdictDisclosedUnproven)
	}
	if len(res.Reasons) != 1 || res.Reasons[0].Code != experiment.ReasonNoEligibleCandidate {
		t.Fatalf("Reasons = %+v, want one no-eligible-candidate reason", res.Reasons)
	}
	cand := candidateResult(t, res, "candidate-a")
	if cand.Eligible {
		t.Fatalf("candidate-a Eligible = true, want false")
	}
	if len(cand.Violations) != 1 {
		t.Fatalf("candidate-a Violations = %+v, want exactly one", cand.Violations)
	}
	v := cand.Violations[0]
	if v.Guard != "behavioral-equivalence" || v.Round != 3 || v.Witness != witness {
		t.Fatalf("violation = %+v, want guard behavioral-equivalence round 3 witness %q", v, witness)
	}
	// Primary is still reported even though the candidate is ineligible.
	if cand.Primary == nil {
		t.Fatalf("ineligible candidate-a still needs a reported primary aggregate")
	}
}

// TestEvaluateBaselineGuardViolation covers the baseline-ineligible branch:
// verdict violated-with-witness, one reason per distinct violated guard,
// using the FIRST round's witness.
func TestEvaluateBaselineGuardViolation(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)
	firstWitness := "tenant boundary crossed in round 1"
	secondWitness := "tenant boundary crossed again in round 2"
	obs[0].Guards = []experiment.GuardResult{guardResult("behavioral-equivalence", false, firstWitness)}
	obs[1].Guards = []experiment.GuardResult{guardResult("behavioral-equivalence", false, secondWitness)}

	res := mustEvaluate(t, def, obs)
	if res.Verdict != experiment.VerdictViolatedWithWitness {
		t.Fatalf("Verdict = %q, want %q", res.Verdict, experiment.VerdictViolatedWithWitness)
	}
	if len(res.Reasons) != 1 {
		t.Fatalf("Reasons = %+v, want exactly one (one per distinct guard)", res.Reasons)
	}
	r := res.Reasons[0]
	if r.Code != experiment.ReasonBaselineGuardViolation || r.Candidate != "baseline" || r.Guard != "behavioral-equivalence" {
		t.Fatalf("reason = %+v, want baseline-guard-violation on baseline/behavioral-equivalence", r)
	}
	if r.Witness == nil || *r.Witness != firstWitness {
		t.Fatalf("reason witness = %v, want the FIRST round's witness %q", r.Witness, firstWitness)
	}
	if res.Winner != "" {
		t.Fatalf("Winner = %q, want empty", res.Winner)
	}
}

// TestEvaluateHigherDirection covers a higher-is-better primary metric
// with both relative and absolute thresholds.
func TestEvaluateHigherDirection(t *testing.T) {
	tests := []struct {
		name        string
		improvement experiment.Threshold
		separation  experiment.Threshold
		baseline    []float64
		winner      []float64
		wantVerdict experiment.Verdict
	}{
		{
			name:        "relative thresholds, winner qualifies",
			improvement: experiment.Threshold{Relative: ptr(0.25)},
			separation:  experiment.Threshold{Relative: ptr(0.05)},
			baseline:    []float64{100, 100, 100},
			winner:      []float64{200, 200, 200},
			wantVerdict: experiment.VerdictProvenWinner,
		},
		{
			name:        "absolute thresholds, winner qualifies",
			improvement: experiment.Threshold{Absolute: ptr(10)},
			separation:  experiment.Threshold{Absolute: ptr(1)},
			baseline:    []float64{100, 100, 100},
			winner:      []float64{115, 115, 115},
			wantVerdict: experiment.VerdictProvenWinner,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := lockDefinition(t, func(d *experiment.Definition) {
				d.Decision.PrimaryMetric.Direction = experiment.DirectionHigher
				d.Decision.PrimaryMetric.Aggregation = experiment.AggregationMean
				d.Decision.BaselineImprovement = tt.improvement
				d.Decision.CandidateSeparation = tt.separation
			})
			obs := happyObservations(t, def, "run-1",
				map[string][]float64{"baseline": tt.baseline, "candidate-a": tt.winner},
				map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
			)
			res := mustEvaluate(t, def, obs)
			if res.Verdict != tt.wantVerdict {
				t.Fatalf("Verdict = %q, want %q", res.Verdict, tt.wantVerdict)
			}
			if tt.wantVerdict == experiment.VerdictProvenWinner && res.Winner != "candidate-a" {
				t.Fatalf("Winner = %q, want candidate-a", res.Winner)
			}
		})
	}
}

// threeCandidateDef returns a locked definition with a third candidate,
// candidate-b, for separation/tie tests.
func threeCandidateDef(t *testing.T, mutators ...func(*experiment.Definition)) experiment.Definition {
	t.Helper()
	all := append([]func(*experiment.Definition){func(d *experiment.Definition) {
		d.Candidates = append(d.Candidates, experiment.Candidate{
			ID: "candidate-b", Patch: "candidates/candidate-b.patch", Digest: fixtureDigest("0"), Base: base40,
		})
	}}, mutators...)
	return lockDefinition(t, all...)
}

func threeCandidateObs(t *testing.T, def experiment.Definition, baselineVals, aVals, bVals []float64) []experiment.Observation {
	t.Helper()
	return happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": baselineVals, "candidate-a": aVals, "candidate-b": bVals},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}, "candidate-b": {106, 107, 105}},
	)
}

// TestEvaluatePracticalTie covers an exact-equality separation between the
// best qualifying candidate and the next eligible candidate.
func TestEvaluatePracticalTie(t *testing.T) {
	def := threeCandidateDef(t)
	obs := threeCandidateObs(t, def,
		[]float64{40, 42, 41},
		[]float64{18, 19, 17},
		[]float64{18, 19, 17},
	)
	res := mustEvaluate(t, def, obs)
	if res.Verdict != experiment.VerdictDisclosedUnproven {
		t.Fatalf("Verdict = %q, want %q", res.Verdict, experiment.VerdictDisclosedUnproven)
	}
	if len(res.Reasons) != 1 || res.Reasons[0].Code != experiment.ReasonPracticalTie {
		t.Fatalf("Reasons = %+v, want one practical-tie", res.Reasons)
	}
}

// TestEvaluateInsufficientSeparation covers a near-tie within the
// registered candidate_separation margin.
func TestEvaluateInsufficientSeparation(t *testing.T) {
	def := threeCandidateDef(t)
	obs := threeCandidateObs(t, def,
		[]float64{40, 42, 41},
		[]float64{18, 19, 17},
		[]float64{19.4, 19.5, 19.3},
	)
	res := mustEvaluate(t, def, obs)
	if res.Verdict != experiment.VerdictDisclosedUnproven {
		t.Fatalf("Verdict = %q, want %q", res.Verdict, experiment.VerdictDisclosedUnproven)
	}
	if len(res.Reasons) != 1 || res.Reasons[0].Code != experiment.ReasonInsufficientSeparation {
		t.Fatalf("Reasons = %+v, want one insufficient-separation", res.Reasons)
	}
}

// TestEvaluateSufficientSeparationAmongThree confirms a genuinely
// separated winner still proves out when a third eligible candidate is
// present but clearly behind.
func TestEvaluateSufficientSeparationAmongThree(t *testing.T) {
	def := threeCandidateDef(t)
	obs := threeCandidateObs(t, def,
		[]float64{40, 42, 41},
		[]float64{18, 19, 17},
		[]float64{24, 25, 23},
	)
	res := mustEvaluate(t, def, obs)
	if res.Verdict != experiment.VerdictProvenWinner || res.Winner != "candidate-a" {
		t.Fatalf("Verdict/Winner = %q/%q, want proven-winner/candidate-a", res.Verdict, res.Winner)
	}
}

// TestEvaluateNoEligibleCandidate covers the all-non-baseline-ineligible
// path via a bound violation (not a guard violation, to keep the case
// distinct from TestEvaluateGuardFailureIneligibleWithWitness).
func TestEvaluateNoEligibleCandidate(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {200, 201, 199}}, // blows the 15% bound
	)
	res := mustEvaluate(t, def, obs)
	if res.Verdict != experiment.VerdictDisclosedUnproven {
		t.Fatalf("Verdict = %q, want %q", res.Verdict, experiment.VerdictDisclosedUnproven)
	}
	if len(res.Reasons) != 1 || res.Reasons[0].Code != experiment.ReasonNoEligibleCandidate {
		t.Fatalf("Reasons = %+v, want one no-eligible-candidate", res.Reasons)
	}
	cand := candidateResult(t, res, "candidate-a")
	if cand.Eligible {
		t.Fatalf("candidate-a Eligible = true, want false (bound violation)")
	}
	var found bool
	for _, b := range cand.Bounds {
		if b.Guard == "peak-rss" && !b.Pass {
			found = true
		}
	}
	if !found {
		t.Fatalf("candidate-a bounds = %+v, want a failing peak-rss bound", cand.Bounds)
	}
}

// TestEvaluateInsufficientBaselineImprovement covers an eligible candidate
// that simply does not clear the improvement bar.
func TestEvaluateInsufficientBaselineImprovement(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {35, 36, 34}}, // ~14% better, short of 25%
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)
	res := mustEvaluate(t, def, obs)
	if res.Verdict != experiment.VerdictDisclosedUnproven {
		t.Fatalf("Verdict = %q, want %q", res.Verdict, experiment.VerdictDisclosedUnproven)
	}
	if len(res.Reasons) != 1 || res.Reasons[0].Code != experiment.ReasonInsufficientBaselineImprovement {
		t.Fatalf("Reasons = %+v, want one insufficient-baseline-improvement", res.Reasons)
	}
}

// TestEvaluateExcessiveVariance covers the registered variability rule.
func TestEvaluateExcessiveVariance(t *testing.T) {
	def := lockDefinition(t, func(d *experiment.Definition) {
		d.Decision.Variability = &experiment.Variability{MaxRelativeSpread: 0.05}
	})
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {10, 19, 28}}, // huge spread
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)
	res := mustEvaluate(t, def, obs)
	if res.Verdict != experiment.VerdictDisclosedUnproven {
		t.Fatalf("Verdict = %q, want %q", res.Verdict, experiment.VerdictDisclosedUnproven)
	}
	if len(res.Reasons) != 1 || res.Reasons[0].Code != experiment.ReasonExcessiveVariance {
		t.Fatalf("Reasons = %+v, want one excessive-variance", res.Reasons)
	}
	if res.Reasons[0].Candidate != "candidate-a" {
		t.Fatalf("Reasons[0].Candidate = %q, want candidate-a", res.Reasons[0].Candidate)
	}
}

// TestEvaluateVariabilityPrecedence proves improvement failures are
// reported ahead of variability failures (steps 5 precedes 7): a run that
// fails BOTH must report insufficient-baseline-improvement.
func TestEvaluateVariabilityPrecedence(t *testing.T) {
	def := lockDefinition(t, func(d *experiment.Definition) {
		d.Decision.Variability = &experiment.Variability{MaxRelativeSpread: 0.01}
	})
	obs := happyObservations(t, def, "run-1",
		// candidate-a does not improve on baseline AND has huge variance.
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {30, 45, 60}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)
	res := mustEvaluate(t, def, obs)
	if res.Verdict != experiment.VerdictDisclosedUnproven {
		t.Fatalf("Verdict = %q, want %q", res.Verdict, experiment.VerdictDisclosedUnproven)
	}
	if len(res.Reasons) != 1 || res.Reasons[0].Code != experiment.ReasonInsufficientBaselineImprovement {
		t.Fatalf("Reasons = %+v, want insufficient-baseline-improvement (precedence over variability)", res.Reasons)
	}
}

// TestEvaluateConflictingBoundsZeroBaselineImprovement covers the
// degenerate baseline_improvement case: a zero-or-negative baseline
// aggregate with a relative threshold cannot be evaluated.
func TestEvaluateConflictingBoundsZeroBaselineImprovement(t *testing.T) {
	def := lockDefinition(t, func(d *experiment.Definition) {
		d.Decision.PrimaryMetric.Aggregation = experiment.AggregationMean
	})
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {0, 0, 0}, "candidate-a": {1, 1, 1}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)
	res := mustEvaluate(t, def, obs)
	if res.Verdict != experiment.VerdictDisclosedUnproven {
		t.Fatalf("Verdict = %q, want %q", res.Verdict, experiment.VerdictDisclosedUnproven)
	}
	if len(res.Reasons) != 1 || res.Reasons[0].Code != experiment.ReasonConflictingBounds {
		t.Fatalf("Reasons = %+v, want one conflicting-bounds", res.Reasons)
	}
}

// TestEvaluateConflictingBoundsZeroBaselineGuard covers the degenerate
// secondary-bound case: a zero-or-negative baseline guard aggregate with a
// relative bound cannot be evaluated.
func TestEvaluateConflictingBoundsZeroBaselineGuard(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {0, 0, 0}, "candidate-a": {108, 109, 107}},
	)
	res := mustEvaluate(t, def, obs)
	if res.Verdict != experiment.VerdictDisclosedUnproven {
		t.Fatalf("Verdict = %q, want %q", res.Verdict, experiment.VerdictDisclosedUnproven)
	}
	if len(res.Reasons) != 1 || res.Reasons[0].Code != experiment.ReasonConflictingBounds {
		t.Fatalf("Reasons = %+v, want one conflicting-bounds", res.Reasons)
	}
}

// TestEvaluateConflictingBoundsNonPositiveSeparationRunnerUp covers the
// fourth degenerate case, and the reason it is degenerate: a RELATIVE
// candidate_separation margin is a fraction OF the runner-up's aggregate,
// so once that aggregate is zero or negative the margin collapses toward
// zero or flips sign, and the comparison stops meaning "materially better".
// Every case below would otherwise emit a proven-winner on a difference of
// one ten-thousandth against a registered 5% bar.
func TestEvaluateConflictingBoundsNonPositiveSeparationRunnerUp(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*experiment.Definition)
		baseline    []float64
		aVals       []float64
		bVals       []float64
		wantVerdict experiment.Verdict
		wantReason  experiment.ReasonCode
	}{
		{
			name: "lower direction, negative runner-up",
			mutate: func(d *experiment.Definition) {
				d.Decision.PrimaryMetric.Aggregation = experiment.AggregationMean
			},
			baseline:    []float64{100, 100, 100},
			aVals:       []float64{-10.0001, -10.0001, -10.0001},
			bVals:       []float64{-10, -10, -10},
			wantVerdict: experiment.VerdictDisclosedUnproven,
			wantReason:  experiment.ReasonConflictingBounds,
		},
		{
			name: "higher direction, negative runner-up",
			mutate: func(d *experiment.Definition) {
				d.Decision.PrimaryMetric.Aggregation = experiment.AggregationMean
				d.Decision.PrimaryMetric.Direction = experiment.DirectionHigher
				d.Decision.BaselineImprovement = experiment.Threshold{Absolute: ptr(1)}
			},
			baseline:    []float64{-100, -100, -100},
			aVals:       []float64{-9.9999, -9.9999, -9.9999},
			bVals:       []float64{-10, -10, -10},
			wantVerdict: experiment.VerdictDisclosedUnproven,
			wantReason:  experiment.ReasonConflictingBounds,
		},
		{
			name: "lower direction, runner-up exactly zero",
			mutate: func(d *experiment.Definition) {
				d.Decision.PrimaryMetric.Aggregation = experiment.AggregationMean
			},
			baseline:    []float64{100, 100, 100},
			aVals:       []float64{-5, -5, -5},
			bVals:       []float64{0, 0, 0},
			wantVerdict: experiment.VerdictDisclosedUnproven,
			wantReason:  experiment.ReasonConflictingBounds,
		},
		{
			// Control: with a positive runner-up the relative arm is
			// unchanged — a genuinely separated winner still proves out.
			name: "control: positive aggregates still prove a winner",
			mutate: func(d *experiment.Definition) {
				d.Decision.PrimaryMetric.Aggregation = experiment.AggregationMean
			},
			baseline:    []float64{100, 100, 100},
			aVals:       []float64{10, 10, 10},
			bVals:       []float64{20, 20, 20},
			wantVerdict: experiment.VerdictProvenWinner,
		},
		{
			// Control: with a positive runner-up inside the margin the
			// relative arm still reports insufficient separation.
			name: "control: positive aggregates inside the margin",
			mutate: func(d *experiment.Definition) {
				d.Decision.PrimaryMetric.Aggregation = experiment.AggregationMean
			},
			baseline:    []float64{100, 100, 100},
			aVals:       []float64{20, 20, 20},
			bVals:       []float64{20.5, 20.5, 20.5},
			wantVerdict: experiment.VerdictDisclosedUnproven,
			wantReason:  experiment.ReasonInsufficientSeparation,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := threeCandidateDef(t, tt.mutate)
			obs := threeCandidateObs(t, def, tt.baseline, tt.aVals, tt.bVals)
			res := mustEvaluate(t, def, obs)
			if res.Verdict != tt.wantVerdict {
				t.Fatalf("Verdict = %q (winner %q, reasons %+v), want %q", res.Verdict, res.Winner, res.Reasons, tt.wantVerdict)
			}
			if tt.wantVerdict == experiment.VerdictProvenWinner {
				return
			}
			if len(res.Reasons) != 1 || res.Reasons[0].Code != tt.wantReason {
				t.Fatalf("Reasons = %+v, want exactly one %q", res.Reasons, tt.wantReason)
			}
			if tt.wantReason == experiment.ReasonConflictingBounds {
				r := res.Reasons[0]
				if r.Candidate != "candidate-b" {
					t.Errorf("Reasons[0].Candidate = %q, want the runner-up candidate-b", r.Candidate)
				}
				if !strings.Contains(r.Detail, "candidate-b") {
					t.Errorf("Reasons[0].Detail = %q, want it to name the runner-up", r.Detail)
				}
			}
		})
	}
}

// TestEvaluateAbsoluteSeparationArm proves the ABSOLUTE separation arm is
// unaffected by the relative arm's degenerate case: an absolute margin is
// a fixed distance, which stays meaningful at and below zero.
func TestEvaluateAbsoluteSeparationArm(t *testing.T) {
	tests := []struct {
		name        string
		aVals       []float64
		bVals       []float64
		wantVerdict experiment.Verdict
	}{
		{
			name:        "inside the absolute margin",
			aVals:       []float64{18, 18, 18},
			bVals:       []float64{19, 19, 19},
			wantVerdict: experiment.VerdictDisclosedUnproven,
		},
		{
			name:        "outside the absolute margin",
			aVals:       []float64{10, 10, 10},
			bVals:       []float64{19, 19, 19},
			wantVerdict: experiment.VerdictProvenWinner,
		},
		{
			// Negative aggregates: -20 beats -10 by 10, clearing the
			// absolute margin of 5 — the arm must NOT be diverted into the
			// relative arm's degenerate handling.
			name:        "negative aggregates outside the absolute margin",
			aVals:       []float64{-20, -20, -20},
			bVals:       []float64{-10, -10, -10},
			wantVerdict: experiment.VerdictProvenWinner,
		},
		{
			name:        "negative aggregates inside the absolute margin",
			aVals:       []float64{-11, -11, -11},
			bVals:       []float64{-10, -10, -10},
			wantVerdict: experiment.VerdictDisclosedUnproven,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := threeCandidateDef(t, func(d *experiment.Definition) {
				d.Decision.PrimaryMetric.Aggregation = experiment.AggregationMean
				d.Decision.CandidateSeparation = experiment.Threshold{Absolute: ptr(5)}
			})
			obs := threeCandidateObs(t, def, []float64{100, 100, 100}, tt.aVals, tt.bVals)
			res := mustEvaluate(t, def, obs)
			if res.Verdict != tt.wantVerdict {
				t.Fatalf("Verdict = %q (reasons %+v), want %q", res.Verdict, res.Reasons, tt.wantVerdict)
			}
			if tt.wantVerdict == experiment.VerdictDisclosedUnproven {
				if len(res.Reasons) != 1 || res.Reasons[0].Code != experiment.ReasonInsufficientSeparation {
					t.Fatalf("Reasons = %+v, want one insufficient-separation", res.Reasons)
				}
			}
			if tt.wantVerdict == experiment.VerdictProvenWinner && res.Winner != "candidate-a" {
				t.Fatalf("Winner = %q, want candidate-a", res.Winner)
			}
		})
	}
}

// TestEvaluateBaselineGuardViolationOutranksDegenerateBound holds AC-2's
// verdict biconditional: violated-with-witness is emitted exactly when the
// baseline failed a required guard. A run whose baseline ALSO carries a
// degenerate secondary bound must still report the violation and its
// witness — a broken baseline premise is a fact about the run, and an
// unevaluable bound must never swallow it.
func TestEvaluateBaselineGuardViolationOutranksDegenerateBound(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		// The baseline's peak-rss aggregate is zero: the registered relative
		// bound cannot be evaluated against it.
		map[string][]float64{"baseline": {0, 0, 0}, "candidate-a": {108, 109, 107}},
	)
	witness := "tenant boundary crossed in round 1"
	obs[0].Guards = []experiment.GuardResult{guardResult("behavioral-equivalence", false, witness)}

	res := mustEvaluate(t, def, obs)
	if res.Verdict != experiment.VerdictViolatedWithWitness {
		t.Fatalf("Verdict = %q (reasons %+v), want %q", res.Verdict, res.Reasons, experiment.VerdictViolatedWithWitness)
	}
	if len(res.Reasons) != 1 || res.Reasons[0].Code != experiment.ReasonBaselineGuardViolation {
		t.Fatalf("Reasons = %+v, want one baseline-guard-violation", res.Reasons)
	}
	if res.Reasons[0].Witness == nil || *res.Reasons[0].Witness != witness {
		t.Fatalf("Reasons[0].Witness = %v, want the baseline's witness %q", res.Reasons[0].Witness, witness)
	}
	base := candidateResult(t, res, "baseline")
	if len(base.Violations) != 1 || base.Violations[0].Witness != witness {
		t.Fatalf("baseline Violations = %+v, want the preserved witness", base.Violations)
	}
}

// TestEvaluateDegenerateBoundStillRecordsEvaluableBounds proves the
// conflicting-bounds path stays maximally honest: every bound that CAN be
// evaluated is still evaluated and recorded (pass and fail alike, for every
// candidate), a real bound failure still makes its candidate ineligible,
// and only the degenerate bound contributes a reason.
func TestEvaluateDegenerateBoundStillRecordsEvaluableBounds(t *testing.T) {
	def := lockDefinition(t, func(d *experiment.Definition) {
		// Bounded guards in registered order: peak-fds is degenerate,
		// peak-rss is evaluable and genuinely failed, peak-cpu is a second
		// degenerate bound registered after both. Registered order also
		// fixes the order of the reasons.
		d.Decision.Guards = []experiment.Guard{
			{ID: "behavioral-equivalence"},
			{ID: "peak-fds", MaximumRelativeToBaseline: ptr(0.10)},
			{ID: "peak-rss", MaximumRelativeToBaseline: ptr(0.15)},
			{ID: "peak-cpu", MaximumRelativeToBaseline: ptr(0.20)},
		}
	})
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		// peak-rss: candidate-a blows the 15% bound over the baseline's 100.
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {200, 201, 199}},
	)
	// peak-fds and peak-cpu: the baseline aggregate is zero for both, so
	// their relative bounds are unevaluable, while candidate-a reports real
	// values.
	for i := range obs {
		v := 0.0
		if obs[i].Candidate != "baseline" {
			v = 5
		}
		obs[i].Measurements = append(obs[i].Measurements,
			measurement("peak-fds", v, "count", experiment.SourceHarnessMeasured),
			measurement("peak-cpu", v, "percent", experiment.SourceHarnessMeasured),
		)
	}

	res := mustEvaluate(t, def, obs)
	if res.Verdict != experiment.VerdictDisclosedUnproven {
		t.Fatalf("Verdict = %q, want %q", res.Verdict, experiment.VerdictDisclosedUnproven)
	}
	if len(res.Reasons) != 2 {
		t.Fatalf("Reasons = %+v, want exactly two (one per degenerate bound, none for the evaluable one)", res.Reasons)
	}
	for i, wantGuard := range []string{"peak-fds", "peak-cpu"} {
		if res.Reasons[i].Code != experiment.ReasonConflictingBounds || res.Reasons[i].Guard != wantGuard {
			t.Fatalf("Reasons[%d] = %+v, want conflicting-bounds on %q (registered order)", i, res.Reasons[i], wantGuard)
		}
	}

	cand := candidateResult(t, res, "candidate-a")
	if cand.Eligible {
		t.Fatalf("candidate-a Eligible = true, want false (its peak-rss bound genuinely failed)")
	}
	var rssRecorded bool
	for _, b := range cand.Bounds {
		if b.Guard == "peak-rss" {
			rssRecorded = true
			if b.Pass {
				t.Errorf("candidate-a peak-rss bound = %+v, want a recorded FAILURE", b)
			}
		}
		if b.Guard == "peak-fds" || b.Guard == "peak-cpu" {
			t.Errorf("candidate-a has a %s bound record %+v, want none (the bound is unevaluable)", b.Guard, b)
		}
	}
	if !rssRecorded {
		t.Fatalf("candidate-a Bounds = %+v, want the evaluable peak-rss check recorded", cand.Bounds)
	}
	base := candidateResult(t, res, "baseline")
	var baseRSS bool
	for _, b := range base.Bounds {
		if b.Guard == "peak-rss" {
			baseRSS = true
			if !b.Pass {
				t.Errorf("baseline peak-rss bound = %+v, want a recorded PASS", b)
			}
		}
	}
	if !baseRSS {
		t.Fatalf("baseline Bounds = %+v, want the evaluable peak-rss check recorded", base.Bounds)
	}
}

// TestEvaluateNonBaselineGuardFailureNeverViolatedWithWitness holds the
// other half of AC-2's biconditional: a required-guard failure on a
// NON-baseline candidate makes that candidate ineligible but never turns
// the run's verdict into violated-with-witness, whatever the rest of the
// field does.
func TestEvaluateNonBaselineGuardFailureNeverViolatedWithWitness(t *testing.T) {
	t.Run("no other eligible candidate", func(t *testing.T) {
		def := lockDefinition(t)
		obs := happyObservations(t, def, "run-1",
			map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
			map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
		)
		obs[5].Guards = []experiment.GuardResult{guardResult("behavioral-equivalence", false, "stale in round 3")}

		res := mustEvaluate(t, def, obs)
		if res.Verdict == experiment.VerdictViolatedWithWitness {
			t.Fatalf("Verdict = %q, want anything but violated-with-witness (the BASELINE passed every guard)", res.Verdict)
		}
		if res.Verdict != experiment.VerdictDisclosedUnproven {
			t.Fatalf("Verdict = %q, want %q", res.Verdict, experiment.VerdictDisclosedUnproven)
		}
	})

	t.Run("another candidate still wins", func(t *testing.T) {
		def := threeCandidateDef(t)
		obs := threeCandidateObs(t, def,
			[]float64{40, 42, 41},
			[]float64{10, 11, 12},
			[]float64{18, 19, 17},
		)
		for i := range obs {
			if obs[i].Candidate == "candidate-a" && obs[i].Round == 3 {
				obs[i].Guards = []experiment.GuardResult{guardResult("behavioral-equivalence", false, "stale after policy update")}
			}
		}
		res := mustEvaluate(t, def, obs)
		if res.Verdict == experiment.VerdictViolatedWithWitness {
			t.Fatalf("Verdict = %q, want anything but violated-with-witness", res.Verdict)
		}
		if res.Verdict != experiment.VerdictProvenWinner || res.Winner != "candidate-b" {
			t.Fatalf("Verdict/Winner = %q/%q, want proven-winner/candidate-b", res.Verdict, res.Winner)
		}
	})
}

// TestEvaluateConflictingBoundsZeroMedianVariability covers the
// degenerate variability case: a zero-or-negative primary p50 cannot
// support a spread ratio.
func TestEvaluateConflictingBoundsZeroMedianVariability(t *testing.T) {
	def := lockDefinition(t, func(d *experiment.Definition) {
		d.Decision.Variability = &experiment.Variability{MaxRelativeSpread: 0.5}
		d.Decision.PrimaryMetric.Aggregation = experiment.AggregationMean
		d.Decision.PrimaryMetric.Direction = experiment.DirectionHigher
		d.Decision.BaselineImprovement = experiment.Threshold{Absolute: ptr(1)}
		d.Decision.CandidateSeparation = experiment.Threshold{Absolute: ptr(1)}
	})
	// baseline's primary values are exactly zero every round -> p50 == 0.
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {0, 0, 0}, "candidate-a": {5, 6, 7}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)
	res := mustEvaluate(t, def, obs)
	if res.Verdict != experiment.VerdictDisclosedUnproven {
		t.Fatalf("Verdict = %q, want %q", res.Verdict, experiment.VerdictDisclosedUnproven)
	}
	if len(res.Reasons) != 1 || res.Reasons[0].Code != experiment.ReasonConflictingBounds {
		t.Fatalf("Reasons = %+v, want one conflicting-bounds", res.Reasons)
	}
}

// TestEvaluateCandidateReportedNeverShiftsAggregate proves a
// candidate-reported diagnostic measurement never enters the primary
// aggregate or affects eligibility, even when it names a value that would
// change the outcome if (incorrectly) counted.
func TestEvaluateCandidateReportedNeverShiftsAggregate(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {35, 36, 34}}, // insufficient improvement
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)
	// Attach a candidate-reported diagnostic that, if wrongly treated as
	// eligible, would look like a much faster candidate.
	for i := range obs {
		if obs[i].Candidate != "candidate-a" {
			continue
		}
		obs[i].Measurements = append(obs[i].Measurements, measurement("cache-hits", 1.0, "count", experiment.SourceCandidateReported))
	}

	res := mustEvaluate(t, def, obs)
	if res.Verdict != experiment.VerdictDisclosedUnproven {
		t.Fatalf("Verdict = %q, want %q (candidate-reported measurement must not change the outcome)", res.Verdict, experiment.VerdictDisclosedUnproven)
	}
	if len(res.Reasons) != 1 || res.Reasons[0].Code != experiment.ReasonInsufficientBaselineImprovement {
		t.Fatalf("Reasons = %+v, want insufficient-baseline-improvement unchanged", res.Reasons)
	}
	cand := candidateResult(t, res, "candidate-a")
	if cand.Primary.Value != "36" {
		t.Fatalf("candidate-a primary = %+v, want p95 (max of 35,36,34) = 36, unaffected by the diagnostic measurement", cand.Primary)
	}
}

// TestEvaluateBoundGuardCandidateReportedOnlyRejected is the bounded-guard
// counterpart to TestEvaluateCandidateReportedNeverShiftsAggregate: it
// pins "a candidate-reported value cannot affect eligibility" for a
// SECONDARY BOUND (not just the primary aggregate), at the strongest
// reachable layer.
//
// A record whose bound-guard measurement (here peak-rss) carries ONLY a
// candidate-reported value, with no decision-eligible reading, is not a
// shape the engine can quietly ignore-and-fall-back-to-honest on: it is a
// shape experiment.ValidateObservations already refuses to let through at
// all. Two Lane-1 rules combine to make this so: Observation.Validate
// forbids two measurements sharing one id in the same record regardless
// of source (so a decision-eligible peak-rss reading can never coexist
// with a candidate-reported one under the same id in one record), and
// ValidateObservations separately requires a decision-eligible
// measurement under every registered bound guard's id in EVERY record. A
// record that swaps the decision-eligible peak-rss reading for a
// candidate-reported one therefore fails integrity validation outright —
// Evaluate (which runs ValidateComplete, and therefore
// ValidateObservations, as its precondition) rejects it as an
// operational error before eligibility is ever computed, and never
// silently treats the inflated candidate-reported value as if it were a
// bound check.
func TestEvaluateBoundGuardCandidateReportedOnlyRejected(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)
	// Swap candidate-a's decision-eligible peak-rss reading, every round,
	// for a candidate-reported one carrying a wildly-over-limit value. If
	// the engine (incorrectly) honored a candidate-reported bound
	// measurement, this would make candidate-a spuriously ineligible; the
	// correct outcome is that this input is refused before Evaluate ever
	// reaches eligibility at all.
	for i := range obs {
		if obs[i].Candidate != "candidate-a" {
			continue
		}
		for j := range obs[i].Measurements {
			if obs[i].Measurements[j].ID == "peak-rss" {
				obs[i].Measurements[j] = measurement("peak-rss", 100000, "MiB", experiment.SourceCandidateReported)
			}
		}
	}

	res, err := Evaluate(def, obs)
	if err == nil {
		t.Fatalf("Evaluate() with a candidate-reported-only bound-guard measurement = nil error, want error")
	}
	if !errors.Is(err, experiment.ErrObservationIntegrity) {
		t.Fatalf("Evaluate() error = %v, want it to wrap ErrObservationIntegrity (missing decision-eligible bound-guard measurement)", err)
	}
	if !isZeroResult(res) {
		t.Fatalf("Evaluate() returned a nonzero Result alongside an error: %+v", res)
	}

	// Pin the invariant at its own strongest layer directly, not merely as
	// observed through Evaluate's wrapped error.
	if err := experiment.ValidateObservations(def, obs); !errors.Is(err, experiment.ErrObservationIntegrity) {
		t.Fatalf("ValidateObservations() error = %v, want it to wrap ErrObservationIntegrity", err)
	}
}

// TestEvaluateFasterIncorrectLoses is the unit-level version of the
// spec's caching scenario (CO-7): a faster candidate that fails a
// required guard must lose to a slower candidate that passes everything
// and clears the improvement/separation bars.
func TestEvaluateFasterIncorrectLoses(t *testing.T) {
	def := threeCandidateDef(t)
	// candidate-a ("final-cache") is fastest but fails behavioral
	// -equivalence in round 3; candidate-b ("facts-cache") is slower but
	// correct and clears both bars against the baseline and candidate-a.
	obs := threeCandidateObs(t, def,
		[]float64{40, 42, 41},
		[]float64{10, 11, 12},
		[]float64{18, 19, 17},
	)
	for i := range obs {
		if obs[i].Candidate == "candidate-a" && obs[i].Round == 3 {
			obs[i].Guards = []experiment.GuardResult{guardResult("behavioral-equivalence", false, "stale after policy update")}
		}
	}

	res := mustEvaluate(t, def, obs)
	if res.Verdict != experiment.VerdictProvenWinner || res.Winner != "candidate-b" {
		t.Fatalf("Verdict/Winner = %q/%q, want proven-winner/candidate-b", res.Verdict, res.Winner)
	}
	loser := candidateResult(t, res, "candidate-a")
	if loser.Eligible {
		t.Fatalf("candidate-a Eligible = true, want false")
	}
	if len(loser.Violations) != 1 || loser.Violations[0].Witness != "stale after policy update" {
		t.Fatalf("candidate-a violations = %+v, want the preserved staleness witness", loser.Violations)
	}
}

// TestEvaluateIncompleteRun proves an incomplete observation set is an
// operational error wrapping experiment.ErrObservationIncomplete, and
// carries no Result.
func TestEvaluateIncompleteRun(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)
	obs = obs[:len(obs)-1] // drop the last (candidate-a, round 3) record

	res, err := Evaluate(def, obs)
	if err == nil {
		t.Fatalf("Evaluate() on an incomplete run = nil error, want error")
	}
	if !errors.Is(err, experiment.ErrObservationIncomplete) {
		t.Fatalf("Evaluate() error = %v, want it to wrap ErrObservationIncomplete", err)
	}
	if !isZeroResult(res) {
		t.Fatalf("Evaluate() returned a nonzero Result alongside an error: %+v", res)
	}
	if !strings.HasPrefix(err.Error(), "experimentdecision: ") {
		t.Fatalf("Evaluate() error = %q, want the experimentdecision: prefix", err.Error())
	}
}

// TestEvaluateDigestMismatch proves a corrupted experiment_digest is an
// operational error wrapping experiment.ErrObservationIntegrity.
func TestEvaluateDigestMismatch(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)
	obs[0].ExperimentDigest = fixtureDigest("9")

	_, err := Evaluate(def, obs)
	if err == nil {
		t.Fatalf("Evaluate() with a mismatched digest = nil error, want error")
	}
	if !errors.Is(err, experiment.ErrObservationIntegrity) {
		t.Fatalf("Evaluate() error = %v, want it to wrap ErrObservationIntegrity", err)
	}
}

// TestEvaluateMixedRunIDs proves inconsistent run identities are an
// operational error.
func TestEvaluateMixedRunIDs(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)
	obs[0].Run = "run-2"

	_, err := Evaluate(def, obs)
	if err == nil {
		t.Fatalf("Evaluate() with mixed run identities = nil error, want error")
	}
	if !errors.Is(err, experiment.ErrObservationIntegrity) {
		t.Fatalf("Evaluate() error = %v, want it to wrap ErrObservationIntegrity", err)
	}
}

// TestEvaluateUnlockedDefinition proves an unlocked definition is an
// operational error and never produces a Result.
func TestEvaluateUnlockedDefinition(t *testing.T) {
	def := baseDefinition() // never locked
	res, err := Evaluate(def, nil)
	if err == nil {
		t.Fatalf("Evaluate() on an unlocked definition = nil error, want error")
	}
	if !isZeroResult(res) {
		t.Fatalf("Evaluate() returned a nonzero Result alongside an error: %+v", res)
	}
}

// TestEvaluateTamperedLock proves a lock block whose digest does not
// match the computed definition digest is an operational error, not a
// silent "unlocked" downgrade.
func TestEvaluateTamperedLock(t *testing.T) {
	def := lockDefinition(t)
	def.Lock.DefinitionDigest = fixtureDigest("9")

	_, err := Evaluate(def, nil)
	if err == nil {
		t.Fatalf("Evaluate() on a tampered lock = nil error, want error")
	}
}

// TestEvaluateOperationalErrorNeverCarriesResult is a focused CO-1 check:
// every Evaluate error path returns the Result zero value, and every
// non-error path returns a Result that validates.
func TestEvaluateOperationalErrorNeverCarriesResult(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)

	if res, err := Evaluate(def, obs); err != nil || res.Validate() != nil {
		t.Fatalf("Evaluate() happy path = (%+v, %v), want a validating Result and nil error", res, err)
	}

	// Copy before mutating: `broken := obs` would alias the SAME backing
	// array, so the mutation below would corrupt obs for anything later in
	// this test rather than isolating the broken input.
	broken := append([]experiment.Observation(nil), obs...)
	broken[0].Round = 999 // out of registered range
	if res, err := Evaluate(def, broken); err == nil || !isZeroResult(res) {
		t.Fatalf("Evaluate() on broken input = (%+v, %v), want (zero Result, error)", res, err)
	}
}

// TestEvaluateOrderIndependent proves CO-3's other half: the SAME evidence
// supplied in a different slice order is the same evidence. The
// observations digest, the whole Result, and the rendered bytes must all
// be identical — file order is a property of the transport, never of the
// decision.
func TestEvaluateOrderIndependent(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)

	// Reverse the slice: same records, different order.
	reordered := make([]experiment.Observation, 0, len(obs))
	for i := len(obs) - 1; i >= 0; i-- {
		reordered = append(reordered, obs[i])
	}

	d1, err := ObservationsDigest(def, obs)
	if err != nil {
		t.Fatalf("ObservationsDigest(obs) unexpected error: %v", err)
	}
	d2, err := ObservationsDigest(def, reordered)
	if err != nil {
		t.Fatalf("ObservationsDigest(reordered) unexpected error: %v", err)
	}
	if d1 != d2 {
		t.Fatalf("ObservationsDigest differs across slice orders: %q vs %q", d1, d2)
	}

	res1 := mustEvaluate(t, def, obs)
	res2 := mustEvaluate(t, def, reordered)
	if !reflect.DeepEqual(res1, res2) {
		t.Fatalf("Evaluate differs across slice orders:\n%+v\n---\n%+v", res1, res2)
	}

	rendered1, err := RenderResult(res1)
	if err != nil {
		t.Fatalf("RenderResult(res1) unexpected error: %v", err)
	}
	rendered2, err := RenderResult(res2)
	if err != nil {
		t.Fatalf("RenderResult(res2) unexpected error: %v", err)
	}
	if string(rendered1) != string(rendered2) {
		t.Fatalf("RenderResult output differs across slice orders:\n%s\n---\n%s", rendered1, rendered2)
	}
}

// TestEvaluateDoesNotMutateInputs proves Evaluate is a pure function of its
// arguments: a caller's locked definition and observation set are exactly
// as they were afterwards, so re-evaluating, re-digesting, or writing the
// same inputs later cannot silently see engine-side edits.
func TestEvaluateDoesNotMutateInputs(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)
	// An independently built pair of the same shape is the reference: the
	// fixture builders allocate fresh values on every call, so nothing is
	// shared between the two.
	wantDef := lockDefinition(t)
	wantObs := happyObservations(t, wantDef, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)
	if !reflect.DeepEqual(def, wantDef) || !reflect.DeepEqual(obs, wantObs) {
		t.Fatalf("fixture builders are not deterministic; the immutability check below would be meaningless")
	}

	if _, err := Evaluate(def, obs); err != nil {
		t.Fatalf("Evaluate() unexpected error: %v", err)
	}

	if !reflect.DeepEqual(def, wantDef) {
		t.Errorf("Evaluate mutated its Definition argument:\ngot:  %+v\nwant: %+v", def, wantDef)
	}
	if !reflect.DeepEqual(obs, wantObs) {
		t.Errorf("Evaluate mutated its observation slice:\ngot:  %+v\nwant: %+v", obs, wantObs)
	}
}

// TestEvaluateDeterministic proves CO-3: two independent Evaluate calls
// over the same locked definition and observation set produce
// byte-identical canonical output and equal ResultDigest.
func TestEvaluateDeterministic(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)

	res1 := mustEvaluate(t, def, obs)
	res2 := mustEvaluate(t, def, obs)

	rendered1, err := canonjson.Marshal(res1)
	if err != nil {
		t.Fatalf("canonjson.Marshal(res1) unexpected error: %v", err)
	}
	rendered2, err := canonjson.Marshal(res2)
	if err != nil {
		t.Fatalf("canonjson.Marshal(res2) unexpected error: %v", err)
	}
	if string(rendered1) != string(rendered2) {
		t.Fatalf("canonjson.Marshal output differs across identical Evaluate calls:\n%s\n---\n%s", rendered1, rendered2)
	}

	d1, err := experiment.ResultDigest(res1)
	if err != nil {
		t.Fatalf("ResultDigest(res1) unexpected error: %v", err)
	}
	d2, err := experiment.ResultDigest(res2)
	if err != nil {
		t.Fatalf("ResultDigest(res2) unexpected error: %v", err)
	}
	if d1 != d2 {
		t.Fatalf("ResultDigest differs across identical Evaluate calls: %q vs %q", d1, d2)
	}
}
