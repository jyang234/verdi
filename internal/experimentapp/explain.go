package experimentapp

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/jyang234/verdi/internal/experiment"
)

// ExplainInput names one exact visible run; no latest or favorable selector
// exists in the application surface.
type ExplainInput struct {
	Run string
}

// ExplainResult contains only already-validated definition, result, state,
// and reproduction facts. It adds no score or recommendation algorithm.
type ExplainResult struct {
	Outcome        Outcome
	AcceptedHead   string
	ExperimentPath string
	Definition     experiment.Definition
	Run            experiment.RunState
	Result         experiment.Result
	Decision       experiment.ResultDecision
	Reproduction   experiment.ReproductionStatus
}

// Explain returns deterministic exact facts for one caller-selected result.
func (s *Service) Explain(ctx context.Context, identity Identity, input ExplainInput) ExplainResult {
	if err := experiment.ValidateID(input.Run); err != nil {
		return ExplainResult{Outcome: operationalOutcome("invalid-request", fmt.Errorf("experimentapp: explanation run id: %w", err))}
	}
	facts, outcome := s.acceptedStatus(ctx, identity)
	if outcome.Classification != ClassificationClean {
		return ExplainResult{Outcome: outcome}
	}
	var selected experiment.RunState
	found := false
	for _, run := range facts.derived.Runs {
		if run.Run == input.Run {
			selected = run
			found = true
			break
		}
	}
	if !found || selected.ResultDigest == "" {
		return ExplainResult{Outcome: verdictOutcome("result-unavailable", "the exact requested run has no completed result")}
	}
	paths, err := experiment.PathsForRun(facts.snapshot.experimentPath, input.Run)
	if err != nil {
		return ExplainResult{Outcome: operationalOutcome("result-invalid", err)}
	}
	raw, err := fs.ReadFile(facts.snapshot.source, paths.Result)
	if errors.Is(err, fs.ErrNotExist) {
		return ExplainResult{Outcome: operationalOutcome("result-invalid", fmt.Errorf("experimentapp: state-bearing result is missing"))}
	}
	if err != nil {
		return ExplainResult{Outcome: operationalOutcome("result-invalid", err)}
	}
	result, err := experiment.DecodeResult(raw)
	if err != nil {
		return ExplainResult{Outcome: operationalOutcome("result-invalid", err)}
	}
	digest, err := experiment.ResultDigest(result)
	if err != nil || digest != selected.ResultDigest {
		if err == nil {
			err = fmt.Errorf("experimentapp: result digest %q does not match state %q", digest, selected.ResultDigest)
		}
		return ExplainResult{Outcome: operationalOutcome("result-invalid", err)}
	}
	decision, err := resultDecision(result)
	if err != nil {
		return ExplainResult{Outcome: operationalOutcome("result-invalid", err)}
	}
	return ExplainResult{
		Outcome: outcomeForVerdict(decision.Verdict), AcceptedHead: facts.snapshot.revision.Head,
		ExperimentPath: facts.snapshot.experimentPath, Definition: cloneDefinition(facts.definition),
		Run: selected, Result: result, Decision: decision, Reproduction: facts.derived.Reproduction,
	}
}
