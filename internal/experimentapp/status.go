package experimentapp

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"

	"github.com/jyang234/verdi/internal/experiment"
)

// StatusResult is the exact source-backed aggregate and every visible run.
type StatusResult struct {
	Outcome        Outcome
	AcceptedHead   string
	ExperimentPath string
	Definition     experiment.Definition
	State          experiment.State
	Runs           []experiment.RunState
	Disclosures    []experiment.StateDisclosure
	Reproduction   experiment.ReproductionStatus
}

type acceptedStatusFacts struct {
	snapshot   acceptedSnapshot
	definition experiment.Definition
	derived    experiment.StateDerivation
}

// Status derives all visible accepted runs through experiment's sole state
// owner and never chooses a preferred run.
func (s *Service) Status(ctx context.Context, identity Identity) StatusResult {
	facts, outcome := s.acceptedStatus(ctx, identity)
	if outcome.Classification != ClassificationClean {
		return StatusResult{Outcome: outcome}
	}
	if facts.derived.State == experiment.StateInconclusive {
		outcome = verdictOutcome("comparison-inconclusive", "accepted result-bearing runs do not prove one winner")
	}
	return StatusResult{
		Outcome: outcome, AcceptedHead: facts.snapshot.revision.Head,
		ExperimentPath: facts.snapshot.experimentPath, Definition: cloneDefinition(facts.definition),
		State: facts.derived.State, Runs: append([]experiment.RunState(nil), facts.derived.Runs...),
		Disclosures:  append([]experiment.StateDisclosure(nil), facts.derived.Disclosures...),
		Reproduction: facts.derived.Reproduction,
	}
}

func (s *Service) acceptedStatus(ctx context.Context, identity Identity) (acceptedStatusFacts, Outcome) {
	if ctx == nil {
		return acceptedStatusFacts{}, operationalOutcome("invalid-request", fmt.Errorf("experimentapp: status context is nil"))
	}
	if err := identity.validate(); err != nil {
		return acceptedStatusFacts{}, operationalOutcome("invalid-request", err)
	}
	snapshot, err := resolveAccepted(ctx, s.git, identity)
	if err != nil {
		var stale *staleAcceptedHeadError
		if errors.As(err, &stale) {
			return acceptedStatusFacts{}, verdictOutcome("accepted-head-stale", stale.Error())
		}
		return acceptedStatusFacts{}, operationalOutcome("accepted-tree-invalid", err)
	}
	definitionBytes, err := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, "experiment.yaml"))
	if err != nil {
		return acceptedStatusFacts{}, operationalOutcome("definition-unreadable", err)
	}
	definition, err := experiment.DecodeDefinition(definitionBytes)
	if err != nil {
		return acceptedStatusFacts{}, operationalOutcome("definition-invalid", err)
	}
	derived, err := experiment.DeriveStateDetailsFromSource(snapshot.source, snapshot.experimentPath, s.results.VerifyResult)
	if err != nil {
		return acceptedStatusFacts{}, operationalOutcome("state-invalid", err)
	}
	return acceptedStatusFacts{snapshot: snapshot, definition: definition, derived: derived}, cleanOutcome()
}
