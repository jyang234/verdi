package experiment

import (
	"fmt"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/canonjson"
)

const (
	EvaluatorProtocolSchema       = "verdi.experiment-evaluator/v1"
	EvaluatorWallDurationMetricID = "verdi-evaluator-wall-duration"
	EvaluatorPeakRSSMetricID      = "verdi-evaluator-peak-rss"
)

type CycleKind string

const (
	CycleWarmup   CycleKind = "warmup"
	CycleMeasured CycleKind = "measured"
)

func (k CycleKind) Validate() error {
	if k != CycleWarmup && k != CycleMeasured {
		return fmt.Errorf("experiment: unknown evaluator cycle kind %q", k)
	}
	return nil
}

type EvaluatorCycle struct {
	Kind   CycleKind `json:"kind"`
	Number int       `json:"number"`
}

func (c EvaluatorCycle) Validate() error {
	if err := c.Kind.Validate(); err != nil {
		return err
	}
	if c.Number < 1 {
		return fmt.Errorf("experiment: evaluator cycle number must be >= 1")
	}
	return nil
}

type ResolvedArtifact struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

func (a ResolvedArtifact) Validate(field string) error {
	if err := ValidateID(a.ID); err != nil {
		return fmt.Errorf("experiment: %s.id: %w", field, err)
	}
	if err := ValidateRepoRelativePath(a.Path); err != nil {
		return fmt.Errorf("experiment: %s.path: %w", field, err)
	}
	if err := ValidateDigest(a.Digest); err != nil {
		return fmt.Errorf("experiment: %s.digest: %w", field, err)
	}
	return nil
}

type EvaluatorRequest struct {
	Schema           string             `json:"schema"`
	ExperimentDigest string             `json:"experiment_digest"`
	Run              string             `json:"run"`
	Candidate        string             `json:"candidate"`
	Cycle            EvaluatorCycle     `json:"cycle"`
	Workload         ResolvedArtifact   `json:"workload"`
	Fixtures         []ResolvedArtifact `json:"fixtures"`
	Contract         ResolvedArtifact   `json:"contract"`
}

func (r EvaluatorRequest) Validate() error {
	if r.Schema != EvaluatorProtocolSchema {
		return fmt.Errorf("experiment: evaluator request schema %q", r.Schema)
	}
	if err := ValidateDigest(r.ExperimentDigest); err != nil {
		return err
	}
	if err := ValidateID(r.Run); err != nil {
		return err
	}
	if err := ValidateID(r.Candidate); err != nil {
		return err
	}
	if err := r.Cycle.Validate(); err != nil {
		return err
	}
	if err := r.Workload.Validate("workload"); err != nil {
		return err
	}
	if r.Fixtures == nil {
		return fmt.Errorf("experiment: evaluator request fixtures must be present")
	}
	seen := map[string]bool{}
	for i, fixture := range r.Fixtures {
		if err := fixture.Validate(fmt.Sprintf("fixtures[%d]", i)); err != nil {
			return err
		}
		if seen[fixture.ID] {
			return fmt.Errorf("experiment: duplicate fixture id %q", fixture.ID)
		}
		seen[fixture.ID] = true
	}
	return r.Contract.Validate("contract")
}

type CandidateOutcome struct {
	Kind    OutcomeKind `json:"kind"`
	Witness *string     `json:"witness,omitempty"`
}

func (o CandidateOutcome) Validate() error {
	if err := o.Kind.Validate(); err != nil {
		return err
	}
	if o.Kind == OutcomeCompleted {
		if o.Witness != nil {
			return fmt.Errorf("experiment: completed outcome forbids witness")
		}
		return nil
	}
	if o.Witness == nil || *o.Witness == "" || !utf8.ValidString(*o.Witness) {
		return fmt.Errorf("experiment: %s outcome requires a nonempty valid-UTF-8 witness", o.Kind)
	}
	return nil
}

type EvaluatorResponse struct {
	Schema       string           `json:"schema"`
	Outcome      CandidateOutcome `json:"outcome"`
	Guards       []GuardResult    `json:"guards"`
	Measurements []Measurement    `json:"measurements"`
	Disclosures  []string         `json:"disclosures"`
}

func (r EvaluatorResponse) Validate() error {
	if r.Schema != EvaluatorProtocolSchema {
		return fmt.Errorf("experiment: evaluator response schema %q", r.Schema)
	}
	if err := r.Outcome.Validate(); err != nil {
		return err
	}
	if r.Guards == nil || r.Measurements == nil || r.Disclosures == nil {
		return fmt.Errorf("experiment: evaluator response guards, measurements, and disclosures must be present")
	}
	if r.Outcome.Kind != OutcomeCompleted && (len(r.Guards) != 0 || len(r.Measurements) != 0) {
		return fmt.Errorf("experiment: candidate failure outcome requires empty guards and measurements")
	}
	seenG := map[string]bool{}
	for _, g := range r.Guards {
		if err := g.Validate(); err != nil {
			return err
		}
		if seenG[g.ID] {
			return fmt.Errorf("experiment: duplicate evaluator guard %q", g.ID)
		}
		seenG[g.ID] = true
	}
	seenM := map[string]bool{}
	for _, m := range r.Measurements {
		if err := m.Validate(); err != nil {
			return err
		}
		if m.Source == SourceHarnessMeasured {
			return fmt.Errorf("experiment: evaluator response cannot claim harness-measured source")
		}
		if isHarnessMetric(m.ID) {
			return fmt.Errorf("experiment: evaluator response cannot supply reserved metric %q", m.ID)
		}
		if seenM[m.ID] {
			return fmt.Errorf("experiment: duplicate evaluator measurement %q", m.ID)
		}
		seenM[m.ID] = true
	}
	for i, d := range r.Disclosures {
		if d == "" || !utf8.ValidString(d) {
			return fmt.Errorf("experiment: evaluator disclosure %d must be nonempty valid UTF-8", i)
		}
		if d == PeakRSSUnavailableDisclosure {
			return fmt.Errorf("experiment: evaluator response cannot supply reserved disclosure %q", d)
		}
	}
	return nil
}

func encodeExact(value interface{ Validate() error }) ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, err
	}
	return canonjson.Marshal(value)
}

func EncodeEvaluatorRequest(r EvaluatorRequest) ([]byte, error)   { return encodeExact(r) }
func EncodeEvaluatorResponse(r EvaluatorResponse) ([]byte, error) { return encodeExact(r) }

func DecodeEvaluatorRequest(raw []byte) (EvaluatorRequest, error) {
	var r EvaluatorRequest
	if err := decodeStrictJSON(raw, &r); err != nil {
		return r, err
	}
	if err := r.Validate(); err != nil {
		return r, err
	}
	if err := requireCanonicalJSON(raw, r); err != nil {
		return r, err
	}
	return r, nil
}
func DecodeEvaluatorResponse(raw []byte) (EvaluatorResponse, error) {
	var r EvaluatorResponse
	if err := decodeStrictJSON(raw, &r); err != nil {
		return r, err
	}
	if err := r.Validate(); err != nil {
		return r, err
	}
	if err := requireCanonicalJSON(raw, r); err != nil {
		return r, err
	}
	return r, nil
}

type fixedHarnessMetric struct {
	Type      MetricType
	Unit      string
	Direction Direction
}

func harnessMetric(id string) (fixedHarnessMetric, bool) {
	switch id {
	case EvaluatorWallDurationMetricID:
		return fixedHarnessMetric{MetricDuration, "ns", DirectionLower}, true
	case EvaluatorPeakRSSMetricID:
		return fixedHarnessMetric{MetricBytes, "bytes", DirectionLower}, true
	}
	return fixedHarnessMetric{}, false
}
func isHarnessMetric(id string) bool { _, ok := harnessMetric(id); return ok }
