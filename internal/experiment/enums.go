package experiment

import "fmt"

// MetricType is the closed core metric primitive vocabulary (AC-3). A new
// primitive requires an explicit protocol revision; unknown values fail
// closed.
type MetricType string

// The six metric primitives.
const (
	MetricDuration MetricType = "duration"
	MetricBytes    MetricType = "bytes"
	MetricCount    MetricType = "count"
	MetricRatio    MetricType = "ratio"
	MetricScalar   MetricType = "scalar"
	MetricBoolean  MetricType = "boolean"
)

// Validate fails closed on any metric type outside the vocabulary.
func (t MetricType) Validate() error {
	switch t {
	case MetricDuration, MetricBytes, MetricCount, MetricRatio, MetricScalar, MetricBoolean:
		return nil
	}
	return fmt.Errorf("experiment: unknown metric type %q", string(t))
}

// Aggregation is the closed initial aggregation vocabulary (AC-3). Unknown
// values fail closed.
type Aggregation string

// The five registered aggregations.
const (
	AggregationP50     Aggregation = "p50"
	AggregationP95     Aggregation = "p95"
	AggregationMaximum Aggregation = "maximum"
	AggregationMean    Aggregation = "mean"
	AggregationRate    Aggregation = "rate"
)

// Validate fails closed on any aggregation outside the vocabulary.
func (a Aggregation) Validate() error {
	switch a {
	case AggregationP50, AggregationP95, AggregationMaximum, AggregationMean, AggregationRate:
		return nil
	}
	return fmt.Errorf("experiment: unknown aggregation %q", string(a))
}

// Direction is the closed primary-metric comparison-direction vocabulary.
type Direction string

// The two directions.
const (
	DirectionLower  Direction = "lower"
	DirectionHigher Direction = "higher"
)

// Validate fails closed on any direction outside the vocabulary.
func (d Direction) Validate() error {
	switch d {
	case DirectionLower, DirectionHigher:
		return nil
	}
	return fmt.Errorf("experiment: unknown direction %q", string(d))
}

// Source is the closed measurement trust classification (DC-12). Every
// measurement carries exactly one.
type Source string

// The three trust classifications.
const (
	SourceHarnessMeasured   Source = "harness-measured"
	SourceEvaluatorMeasured Source = "evaluator-measured"
	SourceCandidateReported Source = "candidate-reported"
)

// Validate fails closed on any source outside the vocabulary.
func (s Source) Validate() error {
	switch s {
	case SourceHarnessMeasured, SourceEvaluatorMeasured, SourceCandidateReported:
		return nil
	}
	return fmt.Errorf("experiment: unknown measurement source %q", string(s))
}

// DecisionEligible reports whether measurements carrying this trust
// classification may determine candidate eligibility or the recommendation
// (DC-12): true only for harness-measured and evaluator-measured sources.
// Candidate-reported values remain diagnostic.
func (s Source) DecisionEligible() bool {
	return s == SourceHarnessMeasured || s == SourceEvaluatorMeasured
}

// GuardVerdictValue is the closed guard-verdict vocabulary an observation
// record's guard entries carry.
type GuardVerdictValue string

// The two guard verdicts.
const (
	GuardVerdictPass GuardVerdictValue = "pass"
	GuardVerdictFail GuardVerdictValue = "fail"
)

// Validate fails closed on any guard verdict outside the vocabulary.
func (v GuardVerdictValue) Validate() error {
	switch v {
	case GuardVerdictPass, GuardVerdictFail:
		return nil
	}
	return fmt.Errorf("experiment: unknown guard verdict %q", string(v))
}

// Order is the closed execution-schedule ordering vocabulary (DC-13).
type Order string

// The one registered v1 execution order.
const (
	OrderDeterministicRotation Order = "deterministic-rotation"
)

// Validate fails closed on any order outside the vocabulary.
func (o Order) Validate() error {
	switch o {
	case OrderDeterministicRotation:
		return nil
	}
	return fmt.Errorf("experiment: unknown execution order %q", string(o))
}

// Verdict is the closed three-valued decision result (CO-1, the decision-
// proof model).
type Verdict string

// The three decision verdicts.
const (
	VerdictProvenWinner        Verdict = "proven-winner"
	VerdictViolatedWithWitness Verdict = "violated-with-witness"
	VerdictDisclosedUnproven   Verdict = "disclosed-unproven"
)

// Validate fails closed on any verdict outside the vocabulary.
func (v Verdict) Validate() error {
	switch v {
	case VerdictProvenWinner, VerdictViolatedWithWitness, VerdictDisclosedUnproven:
		return nil
	}
	return fmt.Errorf("experiment: unknown verdict %q", string(v))
}

// ReasonCode is the closed vocabulary explaining why a result did not
// reach proven-winner (AC-2's evaluation order).
type ReasonCode string

// The seven registered reason codes.
const (
	ReasonBaselineGuardViolation          ReasonCode = "baseline-guard-violation"
	ReasonNoEligibleCandidate             ReasonCode = "no-eligible-candidate"
	ReasonInsufficientBaselineImprovement ReasonCode = "insufficient-baseline-improvement"
	ReasonInsufficientSeparation          ReasonCode = "insufficient-separation"
	ReasonPracticalTie                    ReasonCode = "practical-tie"
	ReasonExcessiveVariance               ReasonCode = "excessive-variance"
	ReasonConflictingBounds               ReasonCode = "conflicting-bounds"
)

// Validate fails closed on any reason code outside the vocabulary.
func (r ReasonCode) Validate() error {
	switch r {
	case ReasonBaselineGuardViolation, ReasonNoEligibleCandidate, ReasonInsufficientBaselineImprovement,
		ReasonInsufficientSeparation, ReasonPracticalTie, ReasonExcessiveVariance, ReasonConflictingBounds:
		return nil
	}
	return fmt.Errorf("experiment: unknown reason code %q", string(r))
}

// Disposition is the closed vocabulary of human ratification responses
// (AC-5, DC-16).
type Disposition string

// The five registered dispositions.
const (
	DispositionSelectRecommended  Disposition = "select-recommended"
	DispositionSelectOther        Disposition = "select-other"
	DispositionRejectAll          Disposition = "reject-all"
	DispositionMisframed          Disposition = "misframed"
	DispositionRequestNewRevision Disposition = "request-new-revision"
)

// Validate fails closed on any disposition outside the vocabulary.
func (d Disposition) Validate() error {
	switch d {
	case DispositionSelectRecommended, DispositionSelectOther, DispositionRejectAll,
		DispositionMisframed, DispositionRequestNewRevision:
		return nil
	}
	return fmt.Errorf("experiment: unknown disposition %q", string(d))
}

// State is the closed derived lifecycle-state vocabulary (AC-1's state
// table, DC-2). It is never decoded from an artifact; DeriveState computes
// it from the presence and validity of artifacts in an experiment
// directory.
type State string

// The six derived lifecycle states, in ladder order.
const (
	StateExploratory  State = "exploratory"
	StateRegistered   State = "registered"
	StateMeasured     State = "measured"
	StateRecommended  State = "recommended"
	StateInconclusive State = "inconclusive"
	StateRatified     State = "ratified"
)

// Validate fails closed on any state outside the vocabulary.
func (s State) Validate() error {
	switch s {
	case StateExploratory, StateRegistered, StateMeasured, StateRecommended, StateInconclusive, StateRatified:
		return nil
	}
	return fmt.Errorf("experiment: unknown state %q", string(s))
}

// AlgorithmVersion is the closed recommendation-algorithm version
// vocabulary. Only one version is registered; a new algorithm requires an
// explicit protocol revision.
type AlgorithmVersion string

// AlgorithmV1 is the only accepted recommendation-algorithm version.
const AlgorithmV1 AlgorithmVersion = "verdi.experiment-recommendation/v1"

// Validate fails closed on any algorithm version outside the vocabulary.
func (a AlgorithmVersion) Validate() error {
	switch a {
	case AlgorithmV1:
		return nil
	}
	return fmt.Errorf("experiment: unknown algorithm version %q", string(a))
}
