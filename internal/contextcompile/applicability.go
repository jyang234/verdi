package contextcompile

import (
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/policyartifact"
)

// ApplicabilityInput is the complete narrow comparison boundary ratified by
// SI-83. Policy is the candidate authority's scope, Request is the compile's
// declared scope, and the remaining fields are the concrete candidate
// operands against which both scopes are evaluated.
type ApplicabilityInput struct {
	Policy        policyartifact.Scope
	Request       policyartifact.Scope
	CandidatePath string
	CandidateRef  string
	Phase         Phase
	Environment   string
}

// ApplicabilityResult is the three-valued result of the compiler-local scope
// comparison. Unknown always carries DisclosureApplicabilityUnknown.
type ApplicabilityResult struct {
	State       Applicability
	Disclosures []DisclosureCode
}

type dimensionResult uint8

const (
	dimensionApplicable dimensionResult = iota
	dimensionInapplicable
	dimensionUnknown
)

type applicabilityEvaluation struct {
	ApplicabilityResult
	phaseInapplicable bool
}

// EvaluateApplicability evaluates the exact, deliberately narrow SI-83
// conjunction. Empty dimensions are universal; phase/environment/ref are
// exact; paths are exact unless a scope member ends in '/', in which case it
// matches descendants only. A known empty dimension dominates unknowns in
// other dimensions.
func EvaluateApplicability(in ApplicabilityInput) (ApplicabilityResult, error) {
	evaluation, err := evaluateApplicability(in)
	if err != nil {
		return ApplicabilityResult{}, err
	}
	return evaluation.ApplicabilityResult, nil
}

func evaluateApplicability(in ApplicabilityInput) (applicabilityEvaluation, error) {
	if err := in.Policy.Validate(); err != nil {
		return applicabilityEvaluation{}, fmt.Errorf("contextcompile: applicability policy scope: %w", err)
	}
	if err := in.Request.Validate(); err != nil {
		return applicabilityEvaluation{}, fmt.Errorf("contextcompile: applicability request scope: %w", err)
	}
	if err := in.Phase.Validate(); err != nil {
		return applicabilityEvaluation{}, fmt.Errorf("contextcompile: applicability phase: %w", err)
	}
	if in.CandidatePath != "" {
		if err := validateCandidatePath(in.CandidatePath); err != nil {
			return applicabilityEvaluation{}, fmt.Errorf("contextcompile: applicability candidate path: %w", err)
		}
	}
	if in.Environment != "" {
		candidateEnvironment := policyartifact.Scope{
			Phases: []string{}, Environments: []string{in.Environment}, Paths: []string{}, Refs: []string{},
		}
		if err := candidateEnvironment.Validate(); err != nil {
			return applicabilityEvaluation{}, fmt.Errorf("contextcompile: applicability candidate environment: %w", err)
		}
	}
	if exactScopesDisjoint(in.Policy.Phases, in.Request.Phases) {
		return knownInapplicableEvaluation(true), nil
	}
	if exactScopesDisjoint(in.Policy.Environments, in.Request.Environments) ||
		exactScopesDisjoint(in.Policy.Refs, in.Request.Refs) ||
		pathScopesDisjoint(in.Policy.Paths, in.Request.Paths) {
		return knownInapplicableEvaluation(false), nil
	}

	phase := string(in.Phase)
	checks := []struct {
		phase bool
		state dimensionResult
	}{
		{phase: true, state: exactDimension(in.Policy.Phases, phase)},
		{phase: true, state: exactDimension(in.Request.Phases, phase)},
		{state: exactDimension(in.Policy.Environments, in.Environment)},
		{state: exactDimension(in.Request.Environments, in.Environment)},
		{state: pathDimension(in.Policy.Paths, in.CandidatePath)},
		{state: pathDimension(in.Request.Paths, in.CandidatePath)},
		{state: exactDimension(in.Policy.Refs, in.CandidateRef)},
		{state: exactDimension(in.Request.Refs, in.CandidateRef)},
	}

	unknown := false
	phaseInapplicable := false
	knownInapplicable := false
	for _, check := range checks {
		switch check.state {
		case dimensionInapplicable:
			knownInapplicable = true
			phaseInapplicable = phaseInapplicable || check.phase
		case dimensionUnknown:
			unknown = true
		}
	}
	if knownInapplicable {
		return knownInapplicableEvaluation(phaseInapplicable), nil
	}
	if unknown {
		return applicabilityEvaluation{ApplicabilityResult: ApplicabilityResult{
			State: ApplicabilityUnknown, Disclosures: []DisclosureCode{DisclosureApplicabilityUnknown},
		}}, nil
	}
	return applicabilityEvaluation{ApplicabilityResult: ApplicabilityResult{
		State: ApplicabilityApplicable, Disclosures: []DisclosureCode{},
	}}, nil
}

func knownInapplicableEvaluation(phase bool) applicabilityEvaluation {
	return applicabilityEvaluation{
		ApplicabilityResult: ApplicabilityResult{State: ApplicabilityInapplicable, Disclosures: []DisclosureCode{}},
		phaseInapplicable:   phase,
	}
}

func exactScopesDisjoint(left, right []string) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	for _, l := range left {
		for _, r := range right {
			if l == r {
				return false
			}
		}
	}
	return true
}

func pathScopesDisjoint(left, right []string) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	for _, l := range left {
		for _, r := range right {
			if pathScopePatternsOverlap(l, r) {
				return false
			}
		}
	}
	return true
}

func pathScopePatternsOverlap(left, right string) bool {
	leftDir := strings.HasSuffix(left, "/")
	rightDir := strings.HasSuffix(right, "/")
	switch {
	case leftDir && rightDir:
		return strings.HasPrefix(left, right) || strings.HasPrefix(right, left)
	case leftDir:
		return pathScopeMatches(left, right)
	case rightDir:
		return pathScopeMatches(right, left)
	default:
		return left == right
	}
}

func exactDimension(scope []string, operand string) dimensionResult {
	if len(scope) == 0 {
		return dimensionApplicable
	}
	if operand == "" {
		return dimensionUnknown
	}
	for _, member := range scope {
		if member == operand {
			return dimensionApplicable
		}
	}
	return dimensionInapplicable
}

func pathDimension(scope []string, candidate string) dimensionResult {
	if len(scope) == 0 {
		return dimensionApplicable
	}
	if candidate == "" {
		return dimensionUnknown
	}
	for _, member := range scope {
		if pathScopeMatches(member, candidate) {
			return dimensionApplicable
		}
	}
	return dimensionInapplicable
}

func pathScopeMatches(member, candidate string) bool {
	if strings.HasSuffix(member, "/") {
		return strings.HasPrefix(candidate, member) && len(candidate) > len(member)
	}
	return candidate == member
}
