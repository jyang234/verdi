package experiment

import (
	"bytes"
	"fmt"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/canonjson"
)

// ObservationSchema is the predecessor read-compatible schema identifier.
const ObservationSchema = "verdi.experiment-observation/v1"

// ObservationSchemaV2 is the harness-owned measured-attempt envelope.
const ObservationSchemaV2 = "verdi.experiment-observation/v2"

// PeakRSSUnavailableDisclosure records that the harness process observer
// could not obtain the optional Linux peak-RSS fact for a measured attempt.
const PeakRSSUnavailableDisclosure = "peak-rss-unavailable"

// GuardResult is one guard verdict inside an observation record.
type GuardResult struct {
	ID      string            `json:"id"`
	Verdict GuardVerdictValue `json:"verdict"`
	Witness *string           `json:"witness"`
}

// Validate checks id, verdict, and the witness conditional: required and
// nonempty when the verdict failed, and forbidden when it passed.
func (g GuardResult) Validate() error {
	if err := ValidateID(g.ID); err != nil {
		return fmt.Errorf("experiment: observation.guards: %w", err)
	}
	if err := g.Verdict.Validate(); err != nil {
		return fmt.Errorf("experiment: guard %q: %w", g.ID, err)
	}
	switch g.Verdict {
	case GuardVerdictFail:
		if g.Witness == nil || *g.Witness == "" || !utf8.ValidString(*g.Witness) {
			return fmt.Errorf("experiment: guard %q: witness must be a nonempty string when verdict is %q", g.ID, GuardVerdictFail)
		}
	case GuardVerdictPass:
		if g.Witness != nil {
			return fmt.Errorf("experiment: guard %q: witness must be null or absent when verdict is %q", g.ID, GuardVerdictPass)
		}
	}
	return nil
}

// Measurement is one measured value inside an observation record, carrying
// exactly one trust classification (DC-12). Value is the strict
// number-or-boolean union SI-46 registers (measurementvalue.go).
type Measurement struct {
	ID     string           `json:"id"`
	Value  MeasurementValue `json:"value"`
	Unit   string           `json:"unit"`
	Source Source           `json:"source"`
}

// Validate checks id, that a value is present and — when it is the
// numeric arm — a genuine finite JSON number, plus unit and source.
//
// It is deliberately GRAMMAR-SCOPED about which arm of the union appeared:
// whether this measurement's id may carry a boolean at all depends on the
// metric the definition registered for it, which is knowledge only the
// def-aware path has (ValidateObservations enforces it, SI-46). Checking
// it here would either duplicate that rule or, worse, guess at it.
func (m Measurement) Validate() error {
	if err := ValidateID(m.ID); err != nil {
		return fmt.Errorf("experiment: observation.measurements: %w", err)
	}
	if !m.Value.Present() {
		return fmt.Errorf("experiment: measurement %q: value is missing", m.ID)
	}
	if !m.Value.IsBool() {
		value, err := m.Value.Float64()
		if err != nil {
			return fmt.Errorf("experiment: measurement %q: %w", m.ID, err)
		}
		if err := validateFinite(fmt.Sprintf("measurement %q: value", m.ID), value); err != nil {
			return err
		}
	}
	if err := ValidateUnit(m.Unit); err != nil {
		return fmt.Errorf("experiment: measurement %q: %w", m.ID, err)
	}
	if err := m.Source.Validate(); err != nil {
		return fmt.Errorf("experiment: measurement %q: %w", m.ID, err)
	}
	return nil
}

// Observation is one V1 predecessor record or V2 harness-owned measured
// attempt, keyed to the locked definition and its run identity.
type Observation struct {
	Schema           string            `json:"schema"`
	ExperimentDigest string            `json:"experiment_digest"`
	Run              string            `json:"run"`
	Candidate        string            `json:"candidate"`
	Round            int               `json:"round"`
	Outcome          *CandidateOutcome `json:"outcome,omitempty"`
	Guards           []GuardResult     `json:"guards"`
	Measurements     []Measurement     `json:"measurements"`
	Disclosures      []string          `json:"disclosures"`
}

// observationDoc is the strict decode target: disclosures is a pointer so
// an omitted (or explicit null) field is distinguishable from an
// explicitly present empty list, matching the artifact's "required key;
// may be empty" grammar.
type observationDoc struct {
	Schema           string            `json:"schema"`
	ExperimentDigest string            `json:"experiment_digest"`
	Run              string            `json:"run"`
	Candidate        string            `json:"candidate"`
	Round            int               `json:"round"`
	Outcome          *CandidateOutcome `json:"outcome,omitempty"`
	Guards           []GuardResult     `json:"guards"`
	Measurements     []Measurement     `json:"measurements"`
	Disclosures      *[]string         `json:"disclosures"`
}

// DecodeObservation strict-decodes data as one observation record and
// fully validates it (decodeStrictJSON: the shared strict seam plus this
// package's duplicate-key guard, applied to every JSONL line).
func DecodeObservation(data []byte) (Observation, error) {
	var doc observationDoc
	if err := decodeStrictJSON(data, &doc); err != nil {
		return Observation{}, fmt.Errorf("experiment: decoding observation: %w", err)
	}
	if doc.Disclosures == nil {
		return Observation{}, fmt.Errorf("experiment: observation.disclosures is missing (an explicitly empty list is [])")
	}
	o := Observation{
		Schema:           doc.Schema,
		ExperimentDigest: doc.ExperimentDigest,
		Run:              doc.Run,
		Candidate:        doc.Candidate,
		Round:            doc.Round,
		Outcome:          doc.Outcome,
		Guards:           doc.Guards,
		Measurements:     doc.Measurements,
		Disclosures:      *doc.Disclosures,
	}
	if err := o.Validate(); err != nil {
		return Observation{}, err
	}
	if o.Schema == ObservationSchemaV2 {
		if err := requireCanonicalJSON(data, o); err != nil {
			return Observation{}, fmt.Errorf("experiment: observation v2: %w", err)
		}
	}
	return o, nil
}

// EncodeObservation validates and canonically encodes one observation.
func EncodeObservation(o Observation) ([]byte, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	return canonjson.Marshal(o)
}

// DecodeObservations strict-decodes data as an observations.jsonl file:
// one record per line, split on "\n". Blank lines (including a blank
// trailing line) are skipped; every other line strict-decodes through
// DecodeObservation. File order is preserved — order is significant for
// later completeness and rotation checks.
func DecodeObservations(data []byte) ([]Observation, error) {
	lines := bytes.Split(data, []byte("\n"))
	out := make([]Observation, 0, len(lines))
	for i, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		record := append([]byte(nil), line...)
		if i < len(lines)-1 {
			record = append(record, '\n')
		}
		o, err := DecodeObservation(record)
		if err != nil {
			return nil, fmt.Errorf("experiment: observations.jsonl line %d: %w", i+1, err)
		}
		out = append(out, o)
	}
	if len(out) > 0 && out[0].Schema == ObservationSchemaV2 {
		var canonical bytes.Buffer
		for i, observation := range out {
			if observation.Schema != ObservationSchemaV2 {
				return nil, fmt.Errorf("experiment: observations.jsonl line %d mixes observation schemas", i+1)
			}
			encoded, err := EncodeObservation(observation)
			if err != nil {
				return nil, fmt.Errorf("experiment: observations.jsonl line %d: %w", i+1, err)
			}
			canonical.Write(encoded)
		}
		if !bytes.Equal(data, canonical.Bytes()) {
			return nil, fmt.Errorf("experiment: observations v2 file is not the exact canonical JSONL encoding")
		}
	}
	return out, nil
}

// Validate checks every field's grammar and the guard/measurement
// uniqueness and disclosures-entry rules. It does NOT check cross-record
// integrity (run consistency, candidate/round registration, completeness)
// — that is ValidateObservations' job (observations_validation.go).
func (o Observation) Validate() error {
	if o.Schema != ObservationSchema && o.Schema != ObservationSchemaV2 {
		return fmt.Errorf("experiment: unknown observation schema %q", o.Schema)
	}
	if o.Schema == ObservationSchema && o.Outcome != nil {
		return fmt.Errorf("experiment: observation v1 forbids outcome")
	}
	if o.Schema == ObservationSchemaV2 {
		if o.Outcome == nil {
			return fmt.Errorf("experiment: observation v2 requires outcome")
		}
		if err := o.Outcome.Validate(); err != nil {
			return err
		}
		if o.Outcome.Kind != OutcomeCompleted {
			if o.Guards == nil || o.Measurements == nil {
				return fmt.Errorf("experiment: candidate failure observation requires present guards and measurements arrays")
			}
			if err := validateCandidateFailureEvidence(o); err != nil {
				return err
			}
		}
	}
	if err := ValidateDigest(o.ExperimentDigest); err != nil {
		return fmt.Errorf("experiment: observation.experiment_digest: %w", err)
	}
	if err := ValidateID(o.Run); err != nil {
		return fmt.Errorf("experiment: observation.run: %w", err)
	}
	if err := ValidateID(o.Candidate); err != nil {
		return fmt.Errorf("experiment: observation.candidate: %w", err)
	}
	if o.Round < 1 {
		return fmt.Errorf("experiment: observation.round must be >= 1, got %d", o.Round)
	}

	guardIDs := make(map[string]bool, len(o.Guards))
	for _, g := range o.Guards {
		if err := g.Validate(); err != nil {
			return err
		}
		if guardIDs[g.ID] {
			return fmt.Errorf("experiment: observation.guards: duplicate id %q", g.ID)
		}
		guardIDs[g.ID] = true
	}

	measurementIDs := make(map[string]bool, len(o.Measurements))
	for _, m := range o.Measurements {
		if err := m.Validate(); err != nil {
			return err
		}
		if measurementIDs[m.ID] {
			return fmt.Errorf("experiment: observation.measurements: duplicate id %q", m.ID)
		}
		measurementIDs[m.ID] = true
	}

	for i, d := range o.Disclosures {
		if d == "" || !utf8.ValidString(d) {
			return fmt.Errorf("experiment: observation.disclosures[%d] must be nonempty", i)
		}
	}

	return nil
}

func validateCandidateFailureEvidence(o Observation) error {
	if len(o.Guards) != 0 {
		return fmt.Errorf("experiment: candidate failure observation requires empty guards")
	}
	if o.Disclosures == nil {
		return fmt.Errorf("experiment: candidate failure observation requires a present disclosures array")
	}

	seenWallDuration := false
	seenPeakRSS := false
	for _, measurement := range o.Measurements {
		if measurement.Source != SourceHarnessMeasured {
			return fmt.Errorf("experiment: candidate failure measurement %q must be harness-measured", measurement.ID)
		}
		if err := validateHarnessMeasurement(measurement); err != nil {
			return fmt.Errorf("experiment: candidate failure observation: %v", err)
		}
		switch measurement.ID {
		case EvaluatorWallDurationMetricID:
			if seenWallDuration {
				return fmt.Errorf("experiment: candidate failure observation repeats fixed wall-duration measurement")
			}
			seenWallDuration = true
		case EvaluatorPeakRSSMetricID:
			if seenPeakRSS {
				return fmt.Errorf("experiment: candidate failure observation repeats optional peak-RSS measurement")
			}
			seenPeakRSS = true
		default:
			return fmt.Errorf("experiment: candidate failure observation measurement %q is not a fixed harness process measurement", measurement.ID)
		}
	}
	if !seenWallDuration {
		return fmt.Errorf("experiment: candidate failure observation requires fixed wall-duration measurement %q", EvaluatorWallDurationMetricID)
	}
	if seenPeakRSS {
		if len(o.Disclosures) != 0 {
			return fmt.Errorf("experiment: candidate failure observation with peak RSS requires empty disclosures")
		}
		return nil
	}
	if len(o.Disclosures) != 1 || o.Disclosures[0] != PeakRSSUnavailableDisclosure {
		return fmt.Errorf("experiment: candidate failure observation without peak RSS requires exactly disclosure %q", PeakRSSUnavailableDisclosure)
	}
	return nil
}
