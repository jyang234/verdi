package experimentdecision

import (
	"sort"

	"github.com/jyang234/verdi/internal/experiment"
)

// candRound is the (candidate, round) lookup key Evaluate indexes
// observations by, once experiment.ValidateComplete has already
// guaranteed exactly one record per registered (candidate, round) pair.
type candRound struct {
	candidate string
	round     int
}

// Evaluate runs the closed deterministic recommendation engine
// (spec/comparative-spike-experiments AC-2) over one locked def and its
// complete observation set. A returned error is ALWAYS an operational
// failure (CO-1) — def is unlocked or tampered, or obs fails
// experiment.ValidateObservations/ValidateComplete — and never carries a
// Result. A returned Result is always a completed comparison expressing
// exactly one Verdict; Evaluate never returns both a non-nil error and a
// non-zero Result.
//
// Evaluate walks the registered evaluation order exactly as numbered in
// AC-2: (1) precondition — already checked above; (2) required-guard
// eligibility; (3) primary-metric aggregation over every candidate; (4)
// secondary bounds; (5) baseline premise and improvement; (6) separation;
// (7) variability; (8) emit. The FIRST failing step determines the
// verdict and reasons — a later step's failure is never reported once an
// earlier one has already failed the run. There is no weighted score,
// dynamic metric selection, threshold mutation, or tie-breaker anywhere in
// this function, and it has no configuration beyond def itself (DC-4).
func Evaluate(def experiment.Definition, obs []experiment.Observation) (experiment.Result, error) {
	locked, err := experiment.Locked(def)
	if err != nil {
		return experiment.Result{}, errfWrap("checking definition lock", err)
	}
	if !locked {
		return experiment.Result{}, errf("definition %q is not locked", def.ID)
	}
	if err := experiment.ValidateComplete(def, obs); err != nil {
		return experiment.Result{}, errfWrap("validating observations", err)
	}

	defDigest, err := experiment.DefinitionDigest(def)
	if err != nil {
		return experiment.Result{}, errfWrap("computing definition digest", err)
	}
	obsDigest, err := ObservationsDigest(def, obs)
	if err != nil {
		return experiment.Result{}, errfWrap("computing observations digest", err)
	}

	byCandRound := make(map[candRound]experiment.Observation, len(obs))
	for _, o := range obs {
		byCandRound[candRound{o.Candidate, o.Round}] = o
	}

	e := &evaluation{
		def:         def,
		defDigest:   defDigest,
		obsDigest:   obsDigest,
		runID:       obs[0].Run,
		byCandRound: byCandRound,
	}
	return e.run()
}

// evaluation holds the working state Evaluate's steps share: the locked
// definition, the two digests every Result carries regardless of verdict,
// and the observation index. Every step is a method so the steps stay
// readable in registered order without threading the same five arguments
// through each one.
type evaluation struct {
	def         experiment.Definition
	defDigest   string
	obsDigest   string
	runID       string
	byCandRound map[candRound]experiment.Observation

	violations map[string][]experiment.Violation
	primary    map[string]experiment.PrimaryResult
	primaryVal map[string]float64
	bounds     map[string][]experiment.Bound
	eligible   map[string]bool
}

// run executes AC-2 steps 2 through 8 in order and returns the assembled,
// self-validating Result.
func (e *evaluation) run() (experiment.Result, error) {
	e.stepGuardEligibility()
	e.stepAggregatePrimary()

	if res, done, err := e.stepSecondaryBounds(); done {
		return res, err
	}

	if res, done, err := e.stepBaselinePremiseAndImprovement(); done {
		return res, err
	}

	best, res, done, err := e.stepSeparation()
	if done {
		return res, err
	}

	if res, done, err := e.stepVariability(); done {
		return res, err
	}

	return e.assemble(experiment.VerdictProvenWinner, best, nil)
}

// primaryRoundValues returns candID's decision-eligible primary-metric
// values, one per registered round, in round order (1..Rounds).
// experiment.ValidateComplete already guarantees exactly one record per
// round and experiment.ValidateObservations already guarantees that
// record carries exactly one decision-eligible primary-metric
// measurement, so this never returns fewer than def.Execution.Rounds
// values for a validated (def, obs) pair.
func (e *evaluation) primaryRoundValues(candID string) []float64 {
	primaryID := e.def.Decision.PrimaryMetric.ID
	values := make([]float64, 0, e.def.Execution.Rounds)
	for round := 1; round <= e.def.Execution.Rounds; round++ {
		o := e.byCandRound[candRound{candID, round}]
		for _, m := range o.Measurements {
			if m.ID != primaryID || !m.Source.DecisionEligible() {
				continue
			}
			v, _ := m.Value.Float64() // already validated finite at decode time
			values = append(values, v)
		}
	}
	return values
}

// guardRoundValues returns candID's decision-eligible measurement values
// for guard id guardID, one per registered round, in round order.
func (e *evaluation) guardRoundValues(candID, guardID string) []float64 {
	values := make([]float64, 0, e.def.Execution.Rounds)
	for round := 1; round <= e.def.Execution.Rounds; round++ {
		o := e.byCandRound[candRound{candID, round}]
		for _, m := range o.Measurements {
			if m.ID != guardID || !m.Source.DecisionEligible() {
				continue
			}
			v, _ := m.Value.Float64()
			values = append(values, v)
		}
	}
	return values
}

// stepGuardEligibility is AC-2 step 2: a candidate with any fail verdict
// on any required (unbounded) guard in any round is ineligible; every
// (guard, round, witness) is recorded as a Violation on its candidate,
// witnesses preserved verbatim. Violations are collected in round-major,
// then registered-guard-order, order — deterministic and independent of
// an observation record's own guard-list order.
func (e *evaluation) stepGuardEligibility() {
	e.violations = make(map[string][]experiment.Violation, len(e.def.Candidates))
	for _, c := range e.def.Candidates {
		for round := 1; round <= e.def.Execution.Rounds; round++ {
			o := e.byCandRound[candRound{c.ID, round}]
			for _, g := range e.def.Decision.Guards {
				if g.Bounded() {
					continue
				}
				for _, gr := range o.Guards {
					if gr.ID == g.ID && gr.Verdict == experiment.GuardVerdictFail {
						e.violations[c.ID] = append(e.violations[c.ID], experiment.Violation{
							Guard:   g.ID,
							Round:   round,
							Witness: *gr.Witness,
						})
					}
				}
			}
		}
	}
}

// stepAggregatePrimary is AC-2 step 3: the primary metric is aggregated
// for EVERY candidate, eligible or not — the result stays honest about a
// disqualified candidate's actual performance — using only decision
// -eligible measurements.
func (e *evaluation) stepAggregatePrimary() {
	e.primary = make(map[string]experiment.PrimaryResult, len(e.def.Candidates))
	e.primaryVal = make(map[string]float64, len(e.def.Candidates))
	pm := e.def.Decision.PrimaryMetric
	for _, c := range e.def.Candidates {
		values := e.primaryRoundValues(c.ID)
		agg := aggregate(pm.Aggregation, values)
		e.primaryVal[c.ID] = agg
		e.primary[c.ID] = experiment.PrimaryResult{
			Aggregation: pm.Aggregation,
			Unit:        pm.Unit,
			Value:       formatFloat(agg),
			Rounds:      len(values),
		}
	}
}

// stepSecondaryBounds is AC-2 step 4. For each registered bounded guard,
// in registered order, it aggregates that guard's decision-eligible
// measurement by MAXIMUM over rounds per candidate, computes
// limit = baselineAggregate * (1+m), and records a Bound{pass} for every
// candidate including the baseline. A candidate whose value exceeds the
// limit becomes ineligible from this step. A non-positive baseline
// aggregate against a relative bound cannot be evaluated at all: the run
// completes as disclosed-unproven/conflicting-bounds instead (a valid run
// whose registered bounds cannot be evaluated, never an error), and no
// guard registered after the degenerate one is evaluated.
//
// done is true whenever this step already determined the final Result;
// the caller must return immediately in that case.
func (e *evaluation) stepSecondaryBounds() (res experiment.Result, done bool, err error) {
	e.bounds = make(map[string][]experiment.Bound, len(e.def.Candidates))
	boundOK := make(map[string]bool, len(e.def.Candidates))
	for _, c := range e.def.Candidates {
		boundOK[c.ID] = true
	}

	for _, g := range e.def.Decision.Guards {
		if !g.Bounded() {
			continue
		}
		maxByCand := make(map[string]float64, len(e.def.Candidates))
		for _, c := range e.def.Candidates {
			maxByCand[c.ID] = maximum(e.guardRoundValues(c.ID, g.ID))
		}
		baselineAgg := maxByCand[e.def.Decision.Baseline]
		if baselineAgg <= 0 {
			e.finalizeEligibility(boundOK)
			reason := experiment.Reason{
				Code:   experiment.ReasonConflictingBounds,
				Guard:  g.ID,
				Detail: "baseline aggregate for guard " + g.ID + " is non-positive; a relative bound cannot be evaluated",
			}
			res, err = e.assemble(experiment.VerdictDisclosedUnproven, "", []experiment.Reason{reason})
			return res, true, err
		}
		limit := baselineAgg * (1 + *g.MaximumRelativeToBaseline)
		for _, c := range e.def.Candidates {
			v := maxByCand[c.ID]
			pass := v <= limit
			e.bounds[c.ID] = append(e.bounds[c.ID], experiment.Bound{
				Guard: g.ID,
				Value: formatFloat(v),
				Limit: formatFloat(limit),
				Pass:  pass,
			})
			if !pass {
				boundOK[c.ID] = false
			}
		}
	}

	e.finalizeEligibility(boundOK)
	return experiment.Result{}, false, nil
}

// finalizeEligibility combines step 2's guard violations with step 4's
// bound-pass state into the final per-candidate eligibility map. It is
// called both when step 4 completes normally and, with whatever bound
// checks were computed before a degenerate baseline aggregate stopped
// further processing, on the conflicting-bounds early-return path — a
// candidate's eligibility as far as the engine got is always reported
// accurately, never defaulted to false merely because evaluation stopped
// early.
func (e *evaluation) finalizeEligibility(boundOK map[string]bool) {
	e.eligible = make(map[string]bool, len(e.def.Candidates))
	for _, c := range e.def.Candidates {
		e.eligible[c.ID] = len(e.violations[c.ID]) == 0 && boundOK[c.ID]
	}
}

// stepBaselinePremiseAndImprovement is AC-2 step 5. If the baseline
// itself failed a required guard in step 2, the run completes as
// violated-with-witness — one reason per distinct violated guard, using
// that guard's FIRST round witness, in registered guard order — and steps
// 6-8 never run (a candidate cannot out-improve a baseline whose own
// premise is broken). A relative baseline_improvement threshold against a
// non-positive baseline aggregate is a second degenerate case
// (conflicting-bounds). Otherwise every eligible non-baseline candidate is
// checked for material improvement; no eligible candidate at all, or no
// improving eligible candidate, both complete the run as
// disclosed-unproven with the matching reason.
func (e *evaluation) stepBaselinePremiseAndImprovement() (res experiment.Result, done bool, err error) {
	baselineID := e.def.Decision.Baseline

	if baselineViolations := e.violations[baselineID]; len(baselineViolations) > 0 {
		var reasons []experiment.Reason
		seen := make(map[string]bool)
		for _, g := range e.def.Decision.Guards {
			if g.Bounded() || seen[g.ID] {
				continue
			}
			for _, v := range baselineViolations {
				if v.Guard == g.ID {
					witness := v.Witness
					reasons = append(reasons, experiment.Reason{
						Code:      experiment.ReasonBaselineGuardViolation,
						Candidate: baselineID,
						Guard:     g.ID,
						Witness:   &witness,
					})
					seen[g.ID] = true
					break
				}
			}
		}
		res, err = e.assemble(experiment.VerdictViolatedWithWitness, "", reasons)
		return res, true, err
	}

	threshold := e.def.Decision.BaselineImprovement
	baselineVal := e.primaryVal[baselineID]
	if threshold.Relative != nil && baselineVal <= 0 {
		reason := experiment.Reason{
			Code:   experiment.ReasonConflictingBounds,
			Detail: "baseline primary aggregate is non-positive; a relative baseline_improvement threshold cannot be evaluated",
		}
		res, err = e.assemble(experiment.VerdictDisclosedUnproven, "", []experiment.Reason{reason})
		return res, true, err
	}

	direction := e.def.Decision.PrimaryMetric.Direction
	improved := func(v float64) bool {
		switch direction {
		case experiment.DirectionLower:
			if threshold.Relative != nil {
				return v <= baselineVal*(1-*threshold.Relative)
			}
			return v <= baselineVal-*threshold.Absolute
		default: // experiment.DirectionHigher
			if threshold.Relative != nil {
				return v >= baselineVal*(1+*threshold.Relative)
			}
			return v >= baselineVal+*threshold.Absolute
		}
	}

	nonBaselineEligible := 0
	qualifying := 0
	for _, c := range e.def.Candidates {
		if c.ID == baselineID || !e.eligible[c.ID] {
			continue
		}
		nonBaselineEligible++
		if improved(e.primaryVal[c.ID]) {
			qualifying++
		}
	}

	if nonBaselineEligible == 0 {
		reason := experiment.Reason{Code: experiment.ReasonNoEligibleCandidate}
		res, err = e.assemble(experiment.VerdictDisclosedUnproven, "", []experiment.Reason{reason})
		return res, true, err
	}
	if qualifying == 0 {
		reason := experiment.Reason{Code: experiment.ReasonInsufficientBaselineImprovement}
		res, err = e.assemble(experiment.VerdictDisclosedUnproven, "", []experiment.Reason{reason})
		return res, true, err
	}
	return experiment.Result{}, false, nil
}

// stepSeparation is AC-2 step 6. It ranks every ELIGIBLE candidate
// (baseline included) by primary aggregate in the registered direction,
// best first, with ties broken by registered candidate order (a stable
// sort over a registered-order input never reorders equal values). The
// best-ranked candidate is always the best QUALIFYING candidate from step
// 5: a candidate that improves on the baseline enough to qualify can never
// rank worse, in the registered direction, than one that does not (the
// improvement test and the ranking order use the same monotonic
// comparison). The runner-up is simply the next entry in that same
// ranking — which is the baseline itself whenever it is the only other
// eligible candidate, exactly as AC-2 intends.
//
// A RELATIVE separation margin is a fraction OF the runner-up's aggregate,
// so a non-positive runner-up aggregate is the fourth degenerate case in
// this engine (alongside a non-positive baseline aggregate against a
// relative baseline_improvement, a non-positive baseline aggregate against
// a relative secondary bound, and a non-positive p50 against a registered
// variability spread): at zero the margin collapses to nothing, and below
// zero it inverts — runnerUp*(1-r) moves AWAY from the runner-up in the
// wrong direction, so an arbitrarily small difference would satisfy an
// arbitrarily large registered bar. The comparison completes as
// disclosed-unproven/conflicting-bounds rather than emitting a winner the
// registered threshold never actually cleared. The ABSOLUTE arm states a
// fixed distance, which stays meaningful at and below zero, and is
// deliberately left alone.
func (e *evaluation) stepSeparation() (best string, res experiment.Result, done bool, err error) {
	type ranked struct {
		id    string
		value float64
	}
	var ranking []ranked
	for _, c := range e.def.Candidates {
		if e.eligible[c.ID] {
			ranking = append(ranking, ranked{c.ID, e.primaryVal[c.ID]})
		}
	}
	direction := e.def.Decision.PrimaryMetric.Direction
	sort.SliceStable(ranking, func(i, j int) bool {
		if direction == experiment.DirectionLower {
			return ranking[i].value < ranking[j].value
		}
		return ranking[i].value > ranking[j].value
	})

	// ranking has at least 2 entries here: the baseline plus at least one
	// qualifying non-baseline candidate (stepBaselinePremiseAndImprovement
	// already returned otherwise), and both are eligible by construction.
	bestEntry, runnerUp := ranking[0], ranking[1]

	if bestEntry.value == runnerUp.value {
		reason := experiment.Reason{
			Code:      experiment.ReasonPracticalTie,
			Candidate: bestEntry.id,
			Detail:    "exact primary-aggregate equality with candidate " + runnerUp.id,
		}
		res, err = e.assemble(experiment.VerdictDisclosedUnproven, "", []experiment.Reason{reason})
		return "", res, true, err
	}

	sep := e.def.Decision.CandidateSeparation
	if sep.Relative != nil && runnerUp.value <= 0 {
		reason := experiment.Reason{
			Code:      experiment.ReasonConflictingBounds,
			Candidate: runnerUp.id,
			Detail: "runner-up primary aggregate for candidate " + runnerUp.id + " is " +
				string(formatFloat(runnerUp.value)) +
				"; a relative candidate_separation threshold cannot be evaluated against a non-positive aggregate",
		}
		res, err = e.assemble(experiment.VerdictDisclosedUnproven, "", []experiment.Reason{reason})
		return "", res, true, err
	}

	var separated bool
	switch direction {
	case experiment.DirectionLower:
		if sep.Relative != nil {
			separated = bestEntry.value <= runnerUp.value*(1-*sep.Relative)
		} else {
			separated = bestEntry.value <= runnerUp.value-*sep.Absolute
		}
	default: // experiment.DirectionHigher
		if sep.Relative != nil {
			separated = bestEntry.value >= runnerUp.value*(1+*sep.Relative)
		} else {
			separated = bestEntry.value >= runnerUp.value+*sep.Absolute
		}
	}
	if !separated {
		reason := experiment.Reason{
			Code:      experiment.ReasonInsufficientSeparation,
			Candidate: bestEntry.id,
			Detail:    "insufficient separation from candidate " + runnerUp.id,
		}
		res, err = e.assemble(experiment.VerdictDisclosedUnproven, "", []experiment.Reason{reason})
		return "", res, true, err
	}

	return bestEntry.id, experiment.Result{}, false, nil
}

// stepVariability is AC-2 step 7, run only once steps 2-6 have already
// produced a would-be winner (improvement and separation failures take
// precedence over variability, per AC-2's numbered order). When
// decision.variability is registered, every ELIGIBLE candidate's spread —
// (max-min)/p50 over its primary-metric round values, p50 by nearest rank
// — must not exceed max_relative_spread. A non-positive p50 cannot support
// a spread ratio at all (conflicting-bounds); any candidate whose spread
// exceeds the bound fails the run as excessive-variance, one reason per
// offending candidate in registered order.
func (e *evaluation) stepVariability() (res experiment.Result, done bool, err error) {
	v := e.def.Decision.Variability
	if v == nil {
		return experiment.Result{}, false, nil
	}

	var varianceReasons []experiment.Reason
	for _, c := range e.def.Candidates {
		if !e.eligible[c.ID] {
			continue
		}
		values := e.primaryRoundValues(c.ID)
		p50 := percentile(values, 50)
		if p50 <= 0 {
			reason := experiment.Reason{
				Code:      experiment.ReasonConflictingBounds,
				Candidate: c.ID,
				Detail:    "primary-metric p50 is non-positive; the registered variability spread cannot be evaluated",
			}
			res, err = e.assemble(experiment.VerdictDisclosedUnproven, "", []experiment.Reason{reason})
			return res, true, err
		}
		spread := (maximum(values) - minimum(values)) / p50
		if spread > v.MaxRelativeSpread {
			varianceReasons = append(varianceReasons, experiment.Reason{
				Code:      experiment.ReasonExcessiveVariance,
				Candidate: c.ID,
				Detail:    "primary-metric relative spread exceeds the registered max_relative_spread",
			})
		}
	}
	if len(varianceReasons) > 0 {
		res, err = e.assemble(experiment.VerdictDisclosedUnproven, "", varianceReasons)
		return res, true, err
	}
	return experiment.Result{}, false, nil
}

// assemble builds the final Result from the evaluation's accumulated
// per-candidate state (violations, primary aggregates, bounds,
// eligibility), in registered candidate order, and validates it before
// returning — every Evaluate exit path shares this one assembly point, so
// no verdict branch can accidentally skip a required field.
func (e *evaluation) assemble(verdict experiment.Verdict, winner string, reasons []experiment.Reason) (experiment.Result, error) {
	candidates := make([]experiment.CandidateResult, 0, len(e.def.Candidates))
	for _, c := range e.def.Candidates {
		primary := e.primary[c.ID]
		candidates = append(candidates, experiment.CandidateResult{
			ID:         c.ID,
			Baseline:   c.ID == e.def.Decision.Baseline,
			Eligible:   e.eligible[c.ID],
			Violations: e.violations[c.ID],
			Primary:    &primary,
			Bounds:     e.bounds[c.ID],
		})
	}

	res := experiment.Result{
		Schema:             experiment.ResultSchema,
		Experiment:         e.def.ID,
		DefinitionDigest:   e.defDigest,
		Run:                e.runID,
		Algorithm:          e.def.Algorithm,
		Verdict:            verdict,
		Winner:             winner,
		Reasons:            reasons,
		Candidates:         candidates,
		ObservationsDigest: e.obsDigest,
	}
	if err := res.Validate(); err != nil {
		return experiment.Result{}, errfWrap("internal: assembled result failed validation", err)
	}
	return res, nil
}
