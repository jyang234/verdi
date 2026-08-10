package experiment

import (
	"errors"
	"fmt"
	"strings"
)

// ErrObservationIntegrity is the sentinel every ValidateObservations
// structural or cross-record integrity failure wraps: a digest or run
// mismatch, an unregistered candidate/guard, an out-of-range or duplicate
// round, or a missing required measurement. It is distinct from
// ErrObservationIncomplete so a caller can tell "this evidence is wrong"
// from "this evidence is merely not finished yet" (both are operational
// for the decision engine, but callers and later units need to
// distinguish them).
var ErrObservationIntegrity = errors.New("experiment: observation integrity violation")

// ErrObservationIncomplete is the sentinel ValidateComplete wraps when an
// otherwise integrity-valid observation set is missing one or more
// required (candidate, round) entries.
//
// It is the SEAM the future resume lane keys on. At this rung an
// incomplete observations.jsonl is a hard error (DeriveState reports it
// rather than selecting a lower state), because incomplete evidence
// supports no verdict. DC-15's resume behavior arrives in a later unit and is the one
// caller allowed to treat this condition as a continuation point rather
// than a failure: it matches on this sentinel with errors.Is, reads the
// enumerated missing (candidate, round) pairs out of the error text's
// source — the same set ValidateComplete computes — and resumes execution
// there. Nothing else may special-case incompleteness, and integrity
// failures (ErrObservationIntegrity) are never resumable.
var ErrObservationIncomplete = errors.New("experiment: observation set incomplete")

// candidateRound is the (candidate, round) key ValidateObservations and
// ValidateComplete both use to detect duplicates and completeness.
type candidateRound struct {
	candidate string
	round     int
}

// ValidateObservations checks obs for cross-record integrity against the
// locked def: def must be locked; every record's experiment_digest must
// equal the locked definition digest; every record must share one run
// identity; every candidate id must be registered; every round must fall
// in [1, def.Execution.Rounds]; no (candidate, round) pair may repeat;
// every guard verdict id must be a registered required (unbounded) guard,
// and every required guard must carry a verdict in every record (bound
// guards never appear as guard verdicts); the primary metric must be
// present with a decision-eligible source and its registered unit in
// every record; and every bound guard must be present as a
// decision-eligible measurement, with a consistent unit across records,
// in every record.
//
// It does NOT check completeness across the full candidate x round
// matrix — that is ValidateComplete's job, layered on top of this
// function's integrity guarantee.
func ValidateObservations(def Definition, obs []Observation) error {
	locked, err := Locked(def)
	if err != nil {
		return err
	}
	if !locked {
		return fmt.Errorf("%w: definition is not locked", ErrObservationIntegrity)
	}
	if len(obs) == 0 {
		return fmt.Errorf("%w: no observations", ErrObservationIntegrity)
	}
	defDigest, err := DefinitionDigest(def)
	if err != nil {
		return err
	}

	candidateIDs := make(map[string]bool, len(def.Candidates))
	for _, c := range def.Candidates {
		candidateIDs[c.ID] = true
	}
	requiredGuardIDs := make(map[string]bool)
	boundGuardIDs := make(map[string]bool)
	for _, g := range def.Decision.Guards {
		if g.Bounded() {
			boundGuardIDs[g.ID] = true
		} else {
			requiredGuardIDs[g.ID] = true
		}
	}
	primaryID := def.Decision.PrimaryMetric.ID
	primaryUnit := def.Decision.PrimaryMetric.Unit
	boundGuardUnits := make(map[string]string, len(boundGuardIDs))

	var runID string
	seenCR := make(map[candidateRound]bool, len(obs))

	for i, o := range obs {
		if o.ExperimentDigest != defDigest {
			return fmt.Errorf("%w: observation %d: experiment_digest %q does not match the locked definition digest %q", ErrObservationIntegrity, i, o.ExperimentDigest, defDigest)
		}
		if runID == "" {
			runID = o.Run
		} else if o.Run != runID {
			return fmt.Errorf("%w: observation %d: run %q does not match the shared run identity %q", ErrObservationIntegrity, i, o.Run, runID)
		}
		if !candidateIDs[o.Candidate] {
			return fmt.Errorf("%w: observation %d: candidate %q is not registered", ErrObservationIntegrity, i, o.Candidate)
		}
		if o.Round < 1 || o.Round > def.Execution.Rounds {
			return fmt.Errorf("%w: observation %d: round %d is out of the registered range [1,%d]", ErrObservationIntegrity, i, o.Round, def.Execution.Rounds)
		}
		key := candidateRound{o.Candidate, o.Round}
		if seenCR[key] {
			return fmt.Errorf("%w: duplicate observation for candidate %q round %d", ErrObservationIntegrity, o.Candidate, o.Round)
		}
		seenCR[key] = true

		seenGuards := make(map[string]bool, len(o.Guards))
		for _, g := range o.Guards {
			if !requiredGuardIDs[g.ID] {
				return fmt.Errorf("%w: observation %d: guard %q is not a registered required guard", ErrObservationIntegrity, i, g.ID)
			}
			seenGuards[g.ID] = true
		}
		for id := range requiredGuardIDs {
			if !seenGuards[id] {
				return fmt.Errorf("%w: observation %d (candidate %q round %d): missing a verdict for required guard %q", ErrObservationIntegrity, i, o.Candidate, o.Round, id)
			}
		}

		for _, m := range o.Measurements {
			if err := validateMeasurementValueKind(def, m); err != nil {
				return fmt.Errorf("%w: observation %d (candidate %q round %d): %v", ErrObservationIntegrity, i, o.Candidate, o.Round, err)
			}
		}

		primaryPresent := false
		for _, m := range o.Measurements {
			if m.ID != primaryID || !m.Source.DecisionEligible() {
				continue
			}
			primaryPresent = true
			if m.Unit != primaryUnit {
				return fmt.Errorf("%w: observation %d: primary metric %q unit %q does not match the registered unit %q", ErrObservationIntegrity, i, primaryID, m.Unit, primaryUnit)
			}
		}
		if !primaryPresent {
			return fmt.Errorf("%w: observation %d (candidate %q round %d): missing a decision-eligible measurement for primary metric %q", ErrObservationIntegrity, i, o.Candidate, o.Round, primaryID)
		}

		for guardID := range boundGuardIDs {
			found := false
			for _, m := range o.Measurements {
				if m.ID != guardID || !m.Source.DecisionEligible() {
					continue
				}
				found = true
				if want, ok := boundGuardUnits[guardID]; ok {
					if m.Unit != want {
						return fmt.Errorf("%w: observation %d: bound guard %q unit %q is inconsistent with an earlier record's unit %q", ErrObservationIntegrity, i, guardID, m.Unit, want)
					}
				} else {
					boundGuardUnits[guardID] = m.Unit
				}
			}
			if !found {
				return fmt.Errorf("%w: observation %d (candidate %q round %d): missing a decision-eligible measurement for bound guard %q", ErrObservationIntegrity, i, o.Candidate, o.Round, guardID)
			}
		}
	}

	return nil
}

// registeredMetricType resolves the metric type def registers for
// measurement id, and whether it registers one at all.
//
// v1 registers a metric TYPE in exactly one place: decision.primary_metric
// (AC-1's registration shape). A bound guard registers an id and a bound
// but no type, and a diagnostic measurement is not registered at all —
// neither has a declared primitive, so neither can be the boolean arm of
// the union. Should a later revision give guards a declared type, this is
// the one function that changes.
func registeredMetricType(def Definition, id string) (MetricType, bool) {
	if id == def.Decision.PrimaryMetric.ID {
		return def.Decision.PrimaryMetric.Type, true
	}
	return "", false
}

// validateMeasurementValueKind enforces SI-46's typed union against the
// registered metric: a measurement whose id resolves to a boolean-typed
// registered metric MUST carry the JSON literal true/false, and every
// other measurement MUST carry a number. Both directions fail closed (CO-2)
// — a 0/1 number standing in for a boolean is exactly the undetectable
// coercion the union exists to prevent, and a boolean standing in for a
// duration is a misdeclared metric that would otherwise aggregate as 1 or
// 0 without complaint.
func validateMeasurementValueKind(def Definition, m Measurement) error {
	metricType, registered := registeredMetricType(def, m.ID)
	wantBool := registered && metricType == MetricBoolean

	switch {
	case wantBool && !m.Value.IsBool():
		return fmt.Errorf("measurement %q is registered as metric type %q and must carry the JSON literal true or false, got %s",
			m.ID, MetricBoolean, m.Value.String())
	case !wantBool && m.Value.IsBool():
		if registered {
			return fmt.Errorf("measurement %q is registered as metric type %q and must carry a JSON number, got the boolean literal %s",
				m.ID, metricType, m.Value.String())
		}
		return fmt.Errorf("measurement %q registers no boolean metric type and must carry a JSON number, got the boolean literal %s",
			m.ID, m.Value.String())
	}
	return nil
}

// ValidateComplete runs ValidateObservations and then checks that obs
// covers every registered candidate x every round in [1,
// def.Execution.Rounds]; any missing entries are enumerated in the
// returned error, which wraps ErrObservationIncomplete. An integrity
// violation ValidateObservations itself detects is returned unchanged
// (still wrapping ErrObservationIntegrity, never ErrObservationIncomplete)
// — completeness is only meaningful once integrity already holds.
func ValidateComplete(def Definition, obs []Observation) error {
	if err := ValidateObservations(def, obs); err != nil {
		return err
	}

	present := make(map[candidateRound]bool, len(obs))
	for _, o := range obs {
		present[candidateRound{o.Candidate, o.Round}] = true
	}

	var missing []string
	for _, c := range def.Candidates {
		for r := 1; r <= def.Execution.Rounds; r++ {
			if !present[candidateRound{c.ID, r}] {
				missing = append(missing, fmt.Sprintf("%s@round-%d", c.ID, r))
			}
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: missing observations: %s", ErrObservationIncomplete, strings.Join(missing, ", "))
	}
	return nil
}
