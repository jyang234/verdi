package experiment

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/canonjson"
)

// ResultSchema is the predecessor read-compatible result schema identifier.
const ResultSchema = "verdi.experiment-result/v1"

const ResultSchemaV2 = "verdi.experiment-result/v2"

// Reason is one entry explaining why a result did not reach
// proven-winner.
//
// Witness is a pointer for the same reason GuardResult.Witness is
// (observation.go): a witness that is PRESENT must exhibit something, so
// its presence has to be distinguishable from its absence. Absent stays
// absent (nil, omitted from the canonical form); an explicitly empty
// witness is a claim to have exhibited nothing and is rejected.
type Reason struct {
	Code      ReasonCode  `json:"code"`
	Detail    string      `json:"detail,omitempty"`
	Guard     string      `json:"guard,omitempty"`
	Candidate string      `json:"candidate,omitempty"`
	Witness   *string     `json:"witness,omitempty"`
	Outcome   OutcomeKind `json:"outcome,omitempty"`
	Round     int         `json:"round,omitempty"`
}

// Validate checks the code and, for any present optional field, that its
// grammar holds — including that a present witness is nonempty.
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
	if !utf8.ValidString(r.Detail) {
		return fmt.Errorf("experiment: reason %q: detail must be valid UTF-8", r.Code)
	}
	if r.Witness != nil && (*r.Witness == "" || !utf8.ValidString(*r.Witness)) {
		return fmt.Errorf("experiment: reason %q: witness must be nonempty when present", r.Code)
	}
	if r.Code == ReasonBaselineCandidateFailure {
		if r.Candidate == "" || r.Round < 1 || (r.Outcome != OutcomeCandidateCrash && r.Outcome != OutcomeCandidateTimeout) || r.Witness == nil {
			return fmt.Errorf("experiment: baseline-candidate-failure requires candidate, round, crash/timeout outcome, and witness")
		}
	} else if r.Outcome != "" || r.Round != 0 {
		return fmt.Errorf("experiment: reason %q forbids outcome and round", r.Code)
	}
	return nil
}

type CandidateExecutionFailure struct {
	Round   int         `json:"round"`
	Kind    OutcomeKind `json:"kind"`
	Witness string      `json:"witness"`
}

func (f CandidateExecutionFailure) Validate() error {
	if f.Round < 1 {
		return fmt.Errorf("experiment: candidate execution failure round must be >= 1")
	}
	if f.Kind != OutcomeCandidateCrash && f.Kind != OutcomeCandidateTimeout {
		return fmt.Errorf("experiment: candidate execution failure kind %q", f.Kind)
	}
	if !utf8.ValidString(f.Witness) {
		return fmt.Errorf("experiment: candidate execution failure witness must be valid UTF-8")
	}
	return nonemptyString("candidate execution failure witness", f.Witness)
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
	Experiment         string            `json:"experiment,omitempty"`
	DefinitionDigest   string            `json:"definition_digest,omitempty"`
	Run                string            `json:"run,omitempty"`
	Algorithm          AlgorithmVersion  `json:"algorithm,omitempty"`
	Verdict            Verdict           `json:"verdict,omitempty"`
	Winner             string            `json:"winner,omitempty"`
	Reasons            []Reason          `json:"reasons,omitempty"`
	Candidates         []CandidateResult `json:"candidates,omitempty"`
	ObservationsDigest string            `json:"observations_digest,omitempty"`
	Decision           *ResultDecision   `json:"decision,omitempty"`
	Execution          *ResultExecution  `json:"execution,omitempty"`
}

type DecisionCandidate struct {
	ID                string                      `json:"id"`
	Baseline          bool                        `json:"baseline"`
	Eligible          bool                        `json:"eligible"`
	Violations        []Violation                 `json:"violations,omitempty"`
	Primary           *PrimaryResult              `json:"primary,omitempty"`
	Bounds            []Bound                     `json:"bounds,omitempty"`
	ExecutionFailures []CandidateExecutionFailure `json:"execution_failures"`
}

type ResultDecision struct {
	Experiment         string              `json:"experiment"`
	DefinitionDigest   string              `json:"definition_digest"`
	Run                string              `json:"run"`
	Algorithm          AlgorithmVersion    `json:"algorithm"`
	Verdict            Verdict             `json:"verdict"`
	Winner             string              `json:"winner,omitempty"`
	Reasons            []Reason            `json:"reasons,omitempty"`
	Candidates         []DecisionCandidate `json:"candidates"`
	ObservationsDigest string              `json:"observations_digest"`
}

type IsolationDisclosure string

const IsolationWeaker IsolationDisclosure = "weaker-isolation"

type ResultIsolation struct {
	Network     ReceiptNetwork        `json:"network"`
	Disclosures []IsolationDisclosure `json:"disclosures"`
}
type WarmupAuthority string

const WarmupAuthorityNonDecisionDiagnostic WarmupAuthority = "non-decision-diagnostic"

type WarmupScope string

const WarmupScopeFinalInvocation WarmupScope = "final-invocation"

type WarmupFailure struct {
	Candidate string      `json:"candidate"`
	Warmup    int         `json:"warmup"`
	Kind      OutcomeKind `json:"kind"`
	Witness   string      `json:"witness"`
}
type WarmupDiagnostics struct {
	Authority WarmupAuthority `json:"authority"`
	Scope     WarmupScope     `json:"scope"`
	Failures  []WarmupFailure `json:"failures"`
}
type ResultExecution struct {
	ExecutionDigest   string            `json:"execution_digest"`
	Isolation         ResultIsolation   `json:"isolation"`
	WarmupDiagnostics WarmupDiagnostics `json:"warmup_diagnostics"`
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
	if res.Schema == ResultSchemaV2 {
		if err := requireCanonicalJSON(raw, res); err != nil {
			return Result{}, fmt.Errorf("experiment: result v2: %w", err)
		}
	}
	return res, nil
}

func EncodeResult(res Result) ([]byte, error) {
	if err := res.Validate(); err != nil {
		return nil, err
	}
	return canonjson.Marshal(res)
}

// Validate checks every field's grammar and the verdict-conditional rules
// binding winner, reasons, candidates, and observations_digest together,
// including that a named winner is an eligible, non-baseline entry of
// candidates.
func (res Result) Validate() error {
	if res.Schema == ResultSchemaV2 {
		return res.validateV2()
	}
	if res.Schema != ResultSchema {
		return fmt.Errorf("experiment: unknown result schema %q", res.Schema)
	}
	if res.Decision != nil || res.Execution != nil {
		return fmt.Errorf("experiment: result v1 forbids decision and execution envelope fields")
	}
	return validateDecisionFields(res.Experiment, res.DefinitionDigest, res.Run, res.Algorithm, res.Verdict, res.Winner, res.Reasons, res.Candidates, res.ObservationsDigest)
}

func validateDecisionFields(experimentID, definitionDigest, run string, algorithm AlgorithmVersion, verdict Verdict, winnerID string, reasons []Reason, candidates []CandidateResult, observationsDigest string) error {
	if err := ValidateID(experimentID); err != nil {
		return fmt.Errorf("experiment: result.experiment: %w", err)
	}
	if err := ValidateDigest(definitionDigest); err != nil {
		return fmt.Errorf("experiment: result.definition_digest: %w", err)
	}
	if err := ValidateID(run); err != nil {
		return fmt.Errorf("experiment: result.run: %w", err)
	}
	if err := algorithm.Validate(); err != nil {
		return fmt.Errorf("experiment: result.algorithm: %w", err)
	}
	if err := verdict.Validate(); err != nil {
		return fmt.Errorf("experiment: result.verdict: %w", err)
	}

	provenWinner := verdict == VerdictProvenWinner
	if provenWinner {
		if winnerID == "" {
			return fmt.Errorf("experiment: result.winner is required when verdict is %q", VerdictProvenWinner)
		}
	} else if winnerID != "" {
		return fmt.Errorf("experiment: result.winner must be absent when verdict is %q", verdict)
	}
	if winnerID != "" {
		if err := ValidateID(winnerID); err != nil {
			return fmt.Errorf("experiment: result.winner: %w", err)
		}
	}

	if provenWinner {
		if len(reasons) != 0 {
			return fmt.Errorf("experiment: result.reasons must be empty when verdict is %q", VerdictProvenWinner)
		}
	} else if len(reasons) == 0 {
		return fmt.Errorf("experiment: result.reasons must be nonempty when verdict is %q", verdict)
	}
	for i, r := range reasons {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("experiment: reasons[%d]: %w", i, err)
		}
	}

	if len(candidates) == 0 {
		return fmt.Errorf("experiment: result.candidates must be nonempty")
	}
	candidateIDs := make(map[string]bool, len(candidates))
	baselines := 0
	var winnerCandidate *CandidateResult
	for i, c := range candidates {
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
		if c.ID == winnerID {
			winnerCandidate = &candidates[i]
		}
	}
	if baselines != 1 {
		return fmt.Errorf("experiment: result.candidates must have exactly one baseline=true entry, got %d", baselines)
	}
	if winnerID != "" && winnerCandidate == nil {
		return fmt.Errorf("experiment: result.winner %q does not name an entry in result.candidates", winnerID)
	}
	// A proven winner is a claim about ONE named candidate, so the entry it
	// names has to be able to carry the claim: a candidate ruled out by a
	// guard cannot win, and the baseline cannot win against itself —
	// winning means having materially improved OVER the baseline.
	if winnerCandidate != nil {
		if !winnerCandidate.Eligible {
			return fmt.Errorf("experiment: result.winner %q names an ineligible candidate", winnerID)
		}
		if winnerCandidate.Baseline {
			return fmt.Errorf("experiment: result.winner %q names the baseline candidate", winnerID)
		}
	}

	if err := ValidateDigest(observationsDigest); err != nil {
		return fmt.Errorf("experiment: result.observations_digest: %w", err)
	}
	return nil
}

func (d ResultDecision) Validate() error {
	candidates := make([]CandidateResult, len(d.Candidates))
	for i, c := range d.Candidates {
		if c.ExecutionFailures == nil {
			return fmt.Errorf("experiment: decision candidate %q execution_failures must be present", c.ID)
		}
		lastRound := 0
		for _, f := range c.ExecutionFailures {
			if err := f.Validate(); err != nil {
				return err
			}
			if f.Round <= lastRound {
				return fmt.Errorf("experiment: decision candidate %q execution failures must be round-ordered without duplicates", c.ID)
			}
			lastRound = f.Round
		}
		if len(c.ExecutionFailures) > 0 && c.Eligible {
			return fmt.Errorf("experiment: decision candidate %q with an execution failure cannot be eligible", c.ID)
		}
		candidates[i] = CandidateResult{ID: c.ID, Baseline: c.Baseline, Eligible: c.Eligible, Violations: c.Violations, Primary: c.Primary, Bounds: c.Bounds}
	}
	return validateDecisionFields(d.Experiment, d.DefinitionDigest, d.Run, d.Algorithm, d.Verdict, d.Winner, d.Reasons, candidates, d.ObservationsDigest)
}

func (e ResultExecution) Validate() error {
	if err := ValidateDigest(e.ExecutionDigest); err != nil {
		return fmt.Errorf("experiment: result execution digest: %w", err)
	}
	if err := e.Isolation.Network.Validate(); err != nil {
		return err
	}
	if e.Isolation.Disclosures == nil {
		return fmt.Errorf("experiment: result isolation disclosures must be present")
	}
	wantWeaker := e.Isolation.Network.Mode == NetworkAllow
	if wantWeaker {
		if len(e.Isolation.Disclosures) != 1 || e.Isolation.Disclosures[0] != IsolationWeaker {
			return fmt.Errorf("experiment: allowed network requires exactly the weaker-isolation disclosure")
		}
	} else if len(e.Isolation.Disclosures) != 0 {
		return fmt.Errorf("experiment: default-deny network forbids isolation disclosures")
	}
	if e.WarmupDiagnostics.Authority != WarmupAuthorityNonDecisionDiagnostic {
		return fmt.Errorf("experiment: warmup diagnostics authority %q", e.WarmupDiagnostics.Authority)
	}
	if e.WarmupDiagnostics.Scope != WarmupScopeFinalInvocation {
		return fmt.Errorf("experiment: warmup diagnostics scope %q", e.WarmupDiagnostics.Scope)
	}
	if e.WarmupDiagnostics.Failures == nil {
		return fmt.Errorf("experiment: warmup diagnostic failures must be present")
	}
	seen := map[string]bool{}
	for _, f := range e.WarmupDiagnostics.Failures {
		if err := ValidateID(f.Candidate); err != nil {
			return err
		}
		if f.Warmup < 1 {
			return fmt.Errorf("experiment: warmup failure number must be >= 1")
		}
		if f.Kind != OutcomeCandidateCrash && f.Kind != OutcomeCandidateTimeout {
			return fmt.Errorf("experiment: warmup failure kind %q", f.Kind)
		}
		if f.Witness == "" || !utf8.ValidString(f.Witness) {
			return fmt.Errorf("experiment: warmup failure witness must be nonempty")
		}
		key := fmt.Sprintf("%d/%s", f.Warmup, f.Candidate)
		if seen[key] {
			return fmt.Errorf("experiment: duplicate warmup failure %s", key)
		}
		seen[key] = true
	}
	return nil
}

// ValidateWarmupDiagnosticsOrder proves failures are a subsequence of the
// definition's deterministic warmup schedule.
func ValidateWarmupDiagnosticsOrder(def Definition, diagnostics WarmupDiagnostics) error {
	if err := def.Validate(); err != nil {
		return err
	}
	ranks := make(map[string]int, def.Execution.Warmups*len(def.Candidates))
	rank := 0
	for warmup := 1; warmup <= def.Execution.Warmups; warmup++ {
		offset := (warmup - 1) % len(def.Candidates)
		for position := range def.Candidates {
			candidate := def.Candidates[(offset+position)%len(def.Candidates)]
			ranks[fmt.Sprintf("%d/%s", warmup, candidate.ID)] = rank
			rank++
		}
	}
	last := -1
	for _, failure := range diagnostics.Failures {
		current, ok := ranks[fmt.Sprintf("%d/%s", failure.Warmup, failure.Candidate)]
		if !ok {
			return fmt.Errorf("experiment: warmup failure %s@%d is outside the registered schedule", failure.Candidate, failure.Warmup)
		}
		if current <= last {
			return fmt.Errorf("experiment: warmup failures are not in exact schedule order")
		}
		last = current
	}
	return nil
}

func (res Result) validateV2() error {
	legacy := Result{Experiment: res.Experiment, DefinitionDigest: res.DefinitionDigest, Run: res.Run, Algorithm: res.Algorithm, Verdict: res.Verdict, Winner: res.Winner, Reasons: res.Reasons, Candidates: res.Candidates, ObservationsDigest: res.ObservationsDigest}
	if !reflect.DeepEqual(legacy, Result{}) {
		return fmt.Errorf("experiment: result v2 forbids legacy decision fields at the envelope root")
	}
	if res.Decision == nil || res.Execution == nil {
		return fmt.Errorf("experiment: result v2 requires decision and execution")
	}
	if err := res.Decision.Validate(); err != nil {
		return err
	}
	return res.Execution.Validate()
}

func (res Result) decisionDocument() ResultDecision {
	if res.Schema == ResultSchemaV2 && res.Decision != nil {
		return *res.Decision
	}
	candidates := make([]DecisionCandidate, len(res.Candidates))
	for i, candidate := range res.Candidates {
		candidates[i] = DecisionCandidate{
			ID: candidate.ID, Baseline: candidate.Baseline, Eligible: candidate.Eligible,
			Violations: candidate.Violations, Primary: candidate.Primary, Bounds: candidate.Bounds,
			ExecutionFailures: []CandidateExecutionFailure{},
		}
	}
	return ResultDecision{
		Experiment: res.Experiment, DefinitionDigest: res.DefinitionDigest, Run: res.Run,
		Algorithm: res.Algorithm, Verdict: res.Verdict, Winner: res.Winner,
		Reasons: res.Reasons, Candidates: candidates, ObservationsDigest: res.ObservationsDigest,
	}
}

func NewResultV2(decision ResultDecision, execution ResultExecution) (Result, error) {
	res := Result{Schema: ResultSchemaV2, Decision: &decision, Execution: &execution}
	if err := res.Validate(); err != nil {
		return Result{}, err
	}
	return res, nil
}

// DecisionFromResult projects a V1-shaped engine result plus measured
// outcomes into the engine-owned V2 decision document.
func DecisionFromResult(res Result, obs []Observation) (ResultDecision, error) {
	if res.Schema != ResultSchema || res.Decision != nil || res.Execution != nil {
		return ResultDecision{}, fmt.Errorf("experiment: decision projection requires a V1 engine result")
	}
	if err := res.Validate(); err != nil {
		return ResultDecision{}, err
	}
	failures := map[string][]CandidateExecutionFailure{}
	for _, o := range obs {
		if o.Schema != ObservationSchemaV2 || o.Outcome == nil || o.Outcome.Kind == OutcomeCompleted {
			continue
		}
		failures[o.Candidate] = append(failures[o.Candidate], CandidateExecutionFailure{Round: o.Round, Kind: o.Outcome.Kind, Witness: *o.Outcome.Witness})
	}
	candidates := make([]DecisionCandidate, len(res.Candidates))
	for i, c := range res.Candidates {
		fs := failures[c.ID]
		if fs == nil {
			fs = []CandidateExecutionFailure{}
		}
		sort.SliceStable(fs, func(i, j int) bool { return fs[i].Round < fs[j].Round })
		candidates[i] = DecisionCandidate{ID: c.ID, Baseline: c.Baseline, Eligible: c.Eligible, Violations: c.Violations, Primary: c.Primary, Bounds: c.Bounds, ExecutionFailures: fs}
	}
	d := ResultDecision{Experiment: res.Experiment, DefinitionDigest: res.DefinitionDigest, Run: res.Run, Algorithm: res.Algorithm, Verdict: res.Verdict, Winner: res.Winner, Reasons: res.Reasons, Candidates: candidates, ObservationsDigest: res.ObservationsDigest}
	if err := d.Validate(); err != nil {
		return ResultDecision{}, err
	}
	return d, nil
}

func ValidateResultReceipt(receipt ExecutionReceipt, res Result) error {
	if err := receipt.Validate(); err != nil {
		return err
	}
	if res.Schema != ResultSchemaV2 || res.Decision == nil || res.Execution == nil {
		return fmt.Errorf("experiment: receipt proof requires result v2")
	}
	if err := res.Validate(); err != nil {
		return err
	}
	digest, err := ExecutionReceiptDigest(receipt)
	if err != nil {
		return err
	}
	if res.Execution.ExecutionDigest != digest {
		return fmt.Errorf("experiment: result execution digest %q does not match receipt %q", res.Execution.ExecutionDigest, digest)
	}
	if receipt.ExperimentDigest != res.Decision.DefinitionDigest || receipt.Run != res.Decision.Run {
		return fmt.Errorf("experiment: receipt identity does not match result decision")
	}
	if !reflect.DeepEqual(receipt.Network, res.Execution.Isolation.Network) {
		return fmt.Errorf("experiment: result isolation network does not match receipt")
	}
	want := []IsolationDisclosure{}
	if receipt.Network.Mode == NetworkAllow {
		want = []IsolationDisclosure{IsolationWeaker}
	}
	if !reflect.DeepEqual(want, res.Execution.Isolation.Disclosures) {
		return fmt.Errorf("experiment: result isolation disclosures do not match receipt")
	}
	for _, row := range receipt.Enforcement {
		if !row.Applied {
			return fmt.Errorf("experiment: receipt enforcement kind %q was not applied", row.Kind)
		}
	}
	if receipt.Network.Mode == NetworkDeny && !receipt.Network.Configured {
		return fmt.Errorf("experiment: receipt default-deny network was not configured")
	}
	return nil
}
