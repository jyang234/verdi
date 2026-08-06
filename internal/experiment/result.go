package experiment

import (
	"encoding/json"
	"fmt"
)

// ResultSchema is the only accepted result.json schema identifier.
const ResultSchema = "verdi.experiment-result/v1"

// Reason is one entry explaining why a result did not reach
// proven-winner.
type Reason struct {
	Code      ReasonCode `json:"code"`
	Detail    string     `json:"detail,omitempty"`
	Guard     string     `json:"guard,omitempty"`
	Candidate string     `json:"candidate,omitempty"`
	Witness   string     `json:"witness,omitempty"`
}

// Validate checks the code and, for any present optional field, that its
// grammar holds.
func (r Reason) Validate() error {
	if err := r.Code.Validate(); err != nil {
		return fmt.Errorf("experiment: reasons: %w", err)
	}
	if r.Guard != "" {
		if err := ValidateID(r.Guard); err != nil {
			return fmt.Errorf("experiment: reason %q: guard: %w", r.Code, err)
		}
	}
	if r.Candidate != "" {
		if err := ValidateID(r.Candidate); err != nil {
			return fmt.Errorf("experiment: reason %q: candidate: %w", r.Code, err)
		}
	}
	return nil
}

// Violation is one guard failure a candidate result records.
type Violation struct {
	Guard   string `json:"guard"`
	Round   int    `json:"round"`
	Witness string `json:"witness"`
}

// Validate checks the guard id, round bound, and nonempty witness.
func (v Violation) Validate() error {
	if err := ValidateID(v.Guard); err != nil {
		return fmt.Errorf("experiment: violation: guard: %w", err)
	}
	if v.Round < 1 {
		return fmt.Errorf("experiment: violation for guard %q: round must be >= 1, got %d", v.Guard, v.Round)
	}
	if v.Witness == "" {
		return fmt.Errorf("experiment: violation for guard %q: witness must be nonempty", v.Guard)
	}
	return nil
}

// PrimaryResult is a candidate's aggregated primary-metric outcome.
type PrimaryResult struct {
	Aggregation Aggregation `json:"aggregation"`
	Unit        string      `json:"unit"`
	Value       json.Number `json:"value"`
	Rounds      int         `json:"rounds"`
}

// Validate checks the aggregation, unit, that value is a genuine, finite
// JSON number, and the rounds bound.
func (p PrimaryResult) Validate() error {
	if err := p.Aggregation.Validate(); err != nil {
		return fmt.Errorf("experiment: primary: %w", err)
	}
	if err := ValidateUnit(p.Unit); err != nil {
		return fmt.Errorf("experiment: primary: %w", err)
	}
	if p.Value == "" {
		return fmt.Errorf("experiment: primary: value is missing")
	}
	value, err := p.Value.Float64()
	if err != nil {
		return fmt.Errorf("experiment: primary: value %q is not a JSON number: %w", string(p.Value), err)
	}
	if err := validateFinite("primary.value", value); err != nil {
		return err
	}
	if p.Rounds < 1 {
		return fmt.Errorf("experiment: primary: rounds must be >= 1, got %d", p.Rounds)
	}
	return nil
}

// Bound is one secondary-guard bound outcome.
type Bound struct {
	Guard string      `json:"guard"`
	Value json.Number `json:"value"`
	Limit json.Number `json:"limit"`
	Pass  bool        `json:"pass"`
}

// Validate checks the guard id and that both value and limit are genuine,
// finite JSON numbers.
func (b Bound) Validate() error {
	if err := ValidateID(b.Guard); err != nil {
		return fmt.Errorf("experiment: bound: guard: %w", err)
	}
	if b.Value == "" {
		return fmt.Errorf("experiment: bound for guard %q: value is missing", b.Guard)
	}
	value, err := b.Value.Float64()
	if err != nil {
		return fmt.Errorf("experiment: bound for guard %q: value %q is not a JSON number: %w", b.Guard, string(b.Value), err)
	}
	if err := validateFinite(fmt.Sprintf("bound for guard %q: value", b.Guard), value); err != nil {
		return err
	}
	if b.Limit == "" {
		return fmt.Errorf("experiment: bound for guard %q: limit is missing", b.Guard)
	}
	limit, err := b.Limit.Float64()
	if err != nil {
		return fmt.Errorf("experiment: bound for guard %q: limit %q is not a JSON number: %w", b.Guard, string(b.Limit), err)
	}
	return validateFinite(fmt.Sprintf("bound for guard %q: limit", b.Guard), limit)
}

// CandidateResult is one candidate's outcome inside a Result.
type CandidateResult struct {
	ID         string         `json:"id"`
	Baseline   bool           `json:"baseline"`
	Eligible   bool           `json:"eligible"`
	Violations []Violation    `json:"violations,omitempty"`
	Primary    *PrimaryResult `json:"primary,omitempty"`
	Bounds     []Bound        `json:"bounds,omitempty"`
}

// Validate checks the id and every nested violation, primary result, and
// bound.
func (c CandidateResult) Validate() error {
	if err := ValidateID(c.ID); err != nil {
		return fmt.Errorf("experiment: candidates: %w", err)
	}
	for i, v := range c.Violations {
		if err := v.Validate(); err != nil {
			return fmt.Errorf("experiment: candidate %q: violations[%d]: %w", c.ID, i, err)
		}
	}
	if c.Primary != nil {
		if err := c.Primary.Validate(); err != nil {
			return fmt.Errorf("experiment: candidate %q: %w", c.ID, err)
		}
	}
	for i, b := range c.Bounds {
		if err := b.Validate(); err != nil {
			return fmt.Errorf("experiment: candidate %q: bounds[%d]: %w", c.ID, i, err)
		}
	}
	return nil
}

// Result is one verdi.experiment-result/v1 record (AC-2): the closed
// recommendation engine's deterministic, canonical output for one locked
// definition and complete observation set. DecodeResult performs shape,
// enum, and grammar validation only — it never recomputes the decision.
type Result struct {
	Schema             string            `json:"schema"`
	Experiment         string            `json:"experiment"`
	DefinitionDigest   string            `json:"definition_digest"`
	Run                string            `json:"run"`
	Algorithm          AlgorithmVersion  `json:"algorithm"`
	Verdict            Verdict           `json:"verdict"`
	Winner             string            `json:"winner,omitempty"`
	Reasons            []Reason          `json:"reasons,omitempty"`
	Candidates         []CandidateResult `json:"candidates"`
	ObservationsDigest string            `json:"observations_digest"`
}

// DecodeResult strict-decodes raw as a result.json document and fully
// validates its shape, enums, and grammar. It performs no recomputation
// of the decision itself. decodeStrictJSON adds this package's
// duplicate-key guard over the shared seam, so a document that says one
// verdict textually and another to the decoder is rejected outright
// rather than resolved last-key-wins.
func DecodeResult(raw []byte) (Result, error) {
	var res Result
	if err := decodeStrictJSON(raw, &res); err != nil {
		return Result{}, fmt.Errorf("experiment: decoding result: %w", err)
	}
	if err := res.Validate(); err != nil {
		return Result{}, err
	}
	return res, nil
}

// Validate checks every field's grammar and the verdict-conditional rules
// binding winner, reasons, candidates, and observations_digest together.
func (res Result) Validate() error {
	if res.Schema != ResultSchema {
		return fmt.Errorf("experiment: unknown result schema %q, want %q", res.Schema, ResultSchema)
	}
	if err := ValidateID(res.Experiment); err != nil {
		return fmt.Errorf("experiment: result.experiment: %w", err)
	}
	if err := ValidateDigest(res.DefinitionDigest); err != nil {
		return fmt.Errorf("experiment: result.definition_digest: %w", err)
	}
	if err := ValidateID(res.Run); err != nil {
		return fmt.Errorf("experiment: result.run: %w", err)
	}
	if err := res.Algorithm.Validate(); err != nil {
		return fmt.Errorf("experiment: result.algorithm: %w", err)
	}
	if err := res.Verdict.Validate(); err != nil {
		return fmt.Errorf("experiment: result.verdict: %w", err)
	}

	provenWinner := res.Verdict == VerdictProvenWinner
	if provenWinner {
		if res.Winner == "" {
			return fmt.Errorf("experiment: result.winner is required when verdict is %q", VerdictProvenWinner)
		}
	} else if res.Winner != "" {
		return fmt.Errorf("experiment: result.winner must be absent when verdict is %q", res.Verdict)
	}
	if res.Winner != "" {
		if err := ValidateID(res.Winner); err != nil {
			return fmt.Errorf("experiment: result.winner: %w", err)
		}
	}

	if provenWinner {
		if len(res.Reasons) != 0 {
			return fmt.Errorf("experiment: result.reasons must be empty when verdict is %q", VerdictProvenWinner)
		}
	} else if len(res.Reasons) == 0 {
		return fmt.Errorf("experiment: result.reasons must be nonempty when verdict is %q", res.Verdict)
	}
	for i, r := range res.Reasons {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("experiment: reasons[%d]: %w", i, err)
		}
	}

	if len(res.Candidates) == 0 {
		return fmt.Errorf("experiment: result.candidates must be nonempty")
	}
	candidateIDs := make(map[string]bool, len(res.Candidates))
	baselines := 0
	winnerFound := false
	for i, c := range res.Candidates {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("experiment: candidates[%d]: %w", i, err)
		}
		if candidateIDs[c.ID] {
			return fmt.Errorf("experiment: result.candidates: duplicate id %q", c.ID)
		}
		candidateIDs[c.ID] = true
		if c.Baseline {
			baselines++
		}
		if c.ID == res.Winner {
			winnerFound = true
		}
	}
	if baselines != 1 {
		return fmt.Errorf("experiment: result.candidates must have exactly one baseline=true entry, got %d", baselines)
	}
	if res.Winner != "" && !winnerFound {
		return fmt.Errorf("experiment: result.winner %q does not name an entry in result.candidates", res.Winner)
	}

	if err := ValidateDigest(res.ObservationsDigest); err != nil {
		return fmt.Errorf("experiment: result.observations_digest: %w", err)
	}
	return nil
}
