package experiment

import "fmt"

// ReproductionRun is one visible run after the state owner has validated its
// durable artifacts. Result is nil for an incomplete or merely measured run;
// a present result is revalidated here and must bind the row and definition.
type ReproductionRun struct {
	Run    string
	Result *Result
}

// ReproductionStatus is the deterministic posture of every visible run.
// Winner is populated only when Reproduced is true, so a below-minimum or
// disagreeing result set is never accidentally presented as reproduced.
type ReproductionStatus struct {
	Reproduced       bool
	Winner           string
	ValidRuns        int
	MinimumValidRuns int
}

// DeriveReproduction evaluates every visible run without selecting or
// filtering one. Incomplete rows remain visible to the caller but do not count.
// Malformed rows and ambiguous ratification binding are operational errors.
// Ratification is validated when present but never changes the result winner:
// in particular, select-other is a human selection, not reproduced evidence.
func DeriveReproduction(def Definition, runs []ReproductionRun, ratification *Ratification) (ReproductionStatus, error) {
	if err := def.Validate(); err != nil {
		return ReproductionStatus{}, err
	}
	definitionDigest, err := DefinitionDigest(def)
	if err != nil {
		return ReproductionStatus{}, err
	}
	locked, err := Locked(def)
	if err != nil {
		return ReproductionStatus{}, err
	}

	status := ReproductionStatus{}
	if def.Schema == DefinitionSchemaV2 && def.Reproduction != nil {
		status.MinimumValidRuns = def.Reproduction.MinimumValidRuns
	}

	seenRuns := make(map[string]bool, len(runs))
	results := make([]Result, 0, len(runs))
	unanimous := true
	winner := ""
	for i, run := range runs {
		if err := ValidateID(run.Run); err != nil {
			return ReproductionStatus{}, fmt.Errorf("experiment: reproduction runs[%d].run: %w", i, err)
		}
		if seenRuns[run.Run] {
			return ReproductionStatus{}, fmt.Errorf("experiment: reproduction runs: duplicate run %q", run.Run)
		}
		seenRuns[run.Run] = true
		if run.Result == nil {
			continue
		}

		result := *run.Result
		if err := result.Validate(); err != nil {
			return ReproductionStatus{}, fmt.Errorf("experiment: reproduction run %q result: %w", run.Run, err)
		}
		decision := result.decisionDocument()
		if decision.Experiment != def.ID || decision.DefinitionDigest != definitionDigest || decision.Run != run.Run || decision.Algorithm != def.Algorithm {
			return ReproductionStatus{}, fmt.Errorf("experiment: reproduction run %q result identity does not match the locked definition/run", run.Run)
		}
		status.ValidRuns++
		results = append(results, result)
		if decision.Winner != "" && !definitionHasCandidate(def, decision.Winner) {
			return ReproductionStatus{}, fmt.Errorf("experiment: reproduction run %q winner %q is not registered by definition %q", run.Run, decision.Winner, def.ID)
		}
		if decision.Verdict != VerdictProvenWinner || decision.Winner == "" {
			unanimous = false
			continue
		}
		if winner == "" {
			winner = decision.Winner
		} else if winner != decision.Winner {
			unanimous = false
		}
	}

	if ratification != nil {
		if err := validateReproductionRatification(def, results, *ratification); err != nil {
			return ReproductionStatus{}, err
		}
	}

	if locked && status.MinimumValidRuns >= 2 && status.ValidRuns >= status.MinimumValidRuns && unanimous && winner != "" {
		status.Reproduced = true
		status.Winner = winner
	}
	return status, nil
}

func definitionHasCandidate(def Definition, candidate string) bool {
	for _, registered := range def.Candidates {
		if registered.ID == candidate {
			return true
		}
	}
	return false
}

func validateReproductionRatification(def Definition, results []Result, ratification Ratification) error {
	if err := ratification.Validate(); err != nil {
		return err
	}
	matches := make([]Result, 0, 1)
	for _, result := range results {
		digest, err := ResultDigest(result)
		if err != nil {
			return err
		}
		if digest == ratification.ResultDigest {
			matches = append(matches, result)
		}
	}
	if len(matches) != 1 {
		return fmt.Errorf("experiment: reproduction ratification result_digest %q matches %d visible results, want exactly one", ratification.ResultDigest, len(matches))
	}
	if err := ValidateRatificationBinding(def, matches[0], ratification); err != nil {
		return fmt.Errorf("experiment: reproduction ratification: %w", err)
	}
	return nil
}
