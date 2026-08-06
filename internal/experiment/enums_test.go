package experiment

import "testing"

func TestMetricTypeValidate(t *testing.T) {
	valid := []MetricType{MetricDuration, MetricBytes, MetricCount, MetricRatio, MetricScalar, MetricBoolean}
	for _, v := range valid {
		if err := v.Validate(); err != nil {
			t.Errorf("MetricType(%q).Validate() = %v, want nil", v, err)
		}
	}
	invalid := []MetricType{"", "percentage", "DURATION", "duration "}
	for _, v := range invalid {
		if err := v.Validate(); err == nil {
			t.Errorf("MetricType(%q).Validate() = nil, want error", v)
		}
	}
}

func TestAggregationValidate(t *testing.T) {
	valid := []Aggregation{AggregationP50, AggregationP95, AggregationMaximum, AggregationMean, AggregationRate}
	for _, v := range valid {
		if err := v.Validate(); err != nil {
			t.Errorf("Aggregation(%q).Validate() = %v, want nil", v, err)
		}
	}
	invalid := []Aggregation{"", "p99", "median"}
	for _, v := range invalid {
		if err := v.Validate(); err == nil {
			t.Errorf("Aggregation(%q).Validate() = nil, want error", v)
		}
	}
}

func TestDirectionValidate(t *testing.T) {
	valid := []Direction{DirectionLower, DirectionHigher}
	for _, v := range valid {
		if err := v.Validate(); err != nil {
			t.Errorf("Direction(%q).Validate() = %v, want nil", v, err)
		}
	}
	invalid := []Direction{"", "up", "down"}
	for _, v := range invalid {
		if err := v.Validate(); err == nil {
			t.Errorf("Direction(%q).Validate() = nil, want error", v)
		}
	}
}

func TestSourceValidateAndDecisionEligible(t *testing.T) {
	tests := []struct {
		src      Source
		wantErr  bool
		eligible bool
	}{
		{SourceHarnessMeasured, false, true},
		{SourceEvaluatorMeasured, false, true},
		{SourceCandidateReported, false, false},
		{"", true, false},
		{"harness-guessed", true, false},
	}
	for _, tt := range tests {
		err := tt.src.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("Source(%q).Validate() = %v, wantErr %v", tt.src, err, tt.wantErr)
		}
		if !tt.wantErr {
			if got := tt.src.DecisionEligible(); got != tt.eligible {
				t.Errorf("Source(%q).DecisionEligible() = %v, want %v", tt.src, got, tt.eligible)
			}
		}
	}
}

func TestGuardVerdictValueValidate(t *testing.T) {
	valid := []GuardVerdictValue{GuardVerdictPass, GuardVerdictFail}
	for _, v := range valid {
		if err := v.Validate(); err != nil {
			t.Errorf("GuardVerdictValue(%q).Validate() = %v, want nil", v, err)
		}
	}
	invalid := []GuardVerdictValue{"", "passed", "error"}
	for _, v := range invalid {
		if err := v.Validate(); err == nil {
			t.Errorf("GuardVerdictValue(%q).Validate() = nil, want error", v)
		}
	}
}

func TestOrderValidate(t *testing.T) {
	if err := OrderDeterministicRotation.Validate(); err != nil {
		t.Errorf("Order(%q).Validate() = %v, want nil", OrderDeterministicRotation, err)
	}
	invalid := []Order{"", "random", "round-robin"}
	for _, v := range invalid {
		if err := v.Validate(); err == nil {
			t.Errorf("Order(%q).Validate() = nil, want error", v)
		}
	}
}

func TestVerdictValidate(t *testing.T) {
	valid := []Verdict{VerdictProvenWinner, VerdictViolatedWithWitness, VerdictDisclosedUnproven}
	for _, v := range valid {
		if err := v.Validate(); err != nil {
			t.Errorf("Verdict(%q).Validate() = %v, want nil", v, err)
		}
	}
	invalid := []Verdict{"", "winner", "unproven"}
	for _, v := range invalid {
		if err := v.Validate(); err == nil {
			t.Errorf("Verdict(%q).Validate() = nil, want error", v)
		}
	}
}

func TestReasonCodeValidate(t *testing.T) {
	valid := []ReasonCode{
		ReasonBaselineGuardViolation, ReasonNoEligibleCandidate, ReasonInsufficientBaselineImprovement,
		ReasonInsufficientSeparation, ReasonPracticalTie, ReasonExcessiveVariance, ReasonConflictingBounds,
	}
	for _, v := range valid {
		if err := v.Validate(); err != nil {
			t.Errorf("ReasonCode(%q).Validate() = %v, want nil", v, err)
		}
	}
	invalid := []ReasonCode{"", "unknown-reason"}
	for _, v := range invalid {
		if err := v.Validate(); err == nil {
			t.Errorf("ReasonCode(%q).Validate() = nil, want error", v)
		}
	}
}

func TestDispositionValidate(t *testing.T) {
	valid := []Disposition{
		DispositionSelectRecommended, DispositionSelectOther, DispositionRejectAll,
		DispositionMisframed, DispositionRequestNewRevision,
	}
	for _, v := range valid {
		if err := v.Validate(); err != nil {
			t.Errorf("Disposition(%q).Validate() = %v, want nil", v, err)
		}
	}
	invalid := []Disposition{"", "select-all"}
	for _, v := range invalid {
		if err := v.Validate(); err == nil {
			t.Errorf("Disposition(%q).Validate() = nil, want error", v)
		}
	}
}

func TestStateValidate(t *testing.T) {
	valid := []State{StateExploratory, StateRegistered, StateMeasured, StateRecommended, StateInconclusive, StateRatified}
	for _, v := range valid {
		if err := v.Validate(); err != nil {
			t.Errorf("State(%q).Validate() = %v, want nil", v, err)
		}
	}
	invalid := []State{"", "locked", "complete"}
	for _, v := range invalid {
		if err := v.Validate(); err == nil {
			t.Errorf("State(%q).Validate() = nil, want error", v)
		}
	}
}

func TestAlgorithmVersionValidate(t *testing.T) {
	if err := AlgorithmV1.Validate(); err != nil {
		t.Errorf("AlgorithmVersion(%q).Validate() = %v, want nil", AlgorithmV1, err)
	}
	invalid := []AlgorithmVersion{"", "verdi.experiment-recommendation/v2", "recommendation/v1"}
	for _, v := range invalid {
		if err := v.Validate(); err == nil {
			t.Errorf("AlgorithmVersion(%q).Validate() = nil, want error", v)
		}
	}
}
