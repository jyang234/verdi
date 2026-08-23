package experimentpolicy

import (
	"fmt"
	"path"
	"strings"

	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentrun"
)

// AuthorizationInput supplies every exact CSE operand needed to validate one
// definition against an immutable effective policy. CandidatePaths are the
// already-derived repo paths touched by candidates; this package does not
// parse patches or discover ambient state.
type AuthorizationInput struct {
	Definition     experiment.Definition
	Capabilities   experiment.Capabilities
	ExperimentPath string
	CandidatePaths []string
}

// Authorize validates the exact definition/capabilities/path operands and
// projects the selected environment without changing grant or environment
// bytes. Shared grant meanings remain owned by experimentrun/execworkspace.
func Authorize(decision *Decision, input AuthorizationInput) (experimentrun.ExecutionAuthorization, error) {
	if err := decision.checkSeal(); err != nil {
		return experimentrun.ExecutionAuthorization{}, fmt.Errorf("experimentpolicy: authorize decision: %w", err)
	}
	if err := input.Definition.Validate(); err != nil {
		return experimentrun.ExecutionAuthorization{}, fmt.Errorf("experimentpolicy: authorize definition: %w", err)
	}
	if err := input.Capabilities.Validate(); err != nil {
		return experimentrun.ExecutionAuthorization{}, fmt.Errorf("experimentpolicy: authorize capabilities: %w", err)
	}
	if err := experiment.ValidateDefinitionCapabilities(input.Definition, input.Capabilities); err != nil {
		return experimentrun.ExecutionAuthorization{}, fmt.Errorf("experimentpolicy: authorize definition capabilities: %w", err)
	}
	if !containsString(decision.payload.Classes, input.Definition.Class) {
		return experimentrun.ExecutionAuthorization{}, fmt.Errorf("experimentpolicy: experiment class %q is outside the effective allowance", input.Definition.Class)
	}
	if err := validateAuthorizedPath("experiment path", decision.payload.ExperimentPaths, input.ExperimentPath); err != nil {
		return experimentrun.ExecutionAuthorization{}, err
	}
	for _, candidatePath := range input.CandidatePaths {
		if err := validateAuthorizedPath("candidate path", decision.payload.CandidatePaths, candidatePath); err != nil {
			return experimentrun.ExecutionAuthorization{}, err
		}
	}
	if !allowsEvaluatorProtocol(decision.payload.Evaluators, input.Definition.Evaluator.Argv[0], experiment.EvaluatorProtocolSchema) {
		return experimentrun.ExecutionAuthorization{}, fmt.Errorf("experimentpolicy: evaluator %q protocol %q is outside the effective allowance", input.Definition.Evaluator.Argv[0], experiment.EvaluatorProtocolSchema)
	}
	if !containsString(input.Capabilities.ProtocolVersions, experiment.EvaluatorProtocolSchema) {
		return experimentrun.ExecutionAuthorization{}, fmt.Errorf("experimentpolicy: evaluator capabilities omit required protocol %q", experiment.EvaluatorProtocolSchema)
	}
	if err := validateTrustedMeasurementSources(decision.payload.TrustedMeasurementSources, input.Definition); err != nil {
		return experimentrun.ExecutionAuthorization{}, err
	}
	if err := validateMandatoryGuards(decision.payload.MandatoryGuards, input.Definition); err != nil {
		return experimentrun.ExecutionAuthorization{}, err
	}
	environment, ok := findEnvironment(decision.payload.Environments, input.Definition.Execution.EnvironmentPolicy)
	if !ok {
		return experimentrun.ExecutionAuthorization{}, fmt.Errorf("experimentpolicy: environment %q is outside the effective allowance", input.Definition.Execution.EnvironmentPolicy)
	}
	if err := validateExactDeclaredEnvironment(environment.DeclaredEnvironment, input.Capabilities.Environment); err != nil {
		return experimentrun.ExecutionAuthorization{}, err
	}
	return experimentrun.ExecutionAuthorization{
		EnvironmentPolicy: environment.ID,
		AuthorityDigest:   decision.authorityDigest,
		GrantBytes:        append([]byte(nil), environment.Grants...),
		DeclaredEnv:       cloneStringMap(environment.DeclaredEnvironment),
		ObservationBytes:  decision.payload.Limits.ObservationBytes,
	}, nil
}

func validateAuthorizedPath(field string, patterns []string, candidate string) error {
	if err := experiment.ValidateRepoRelativePath(candidate); err != nil {
		return fmt.Errorf("experimentpolicy: %s %q: %w", field, candidate, err)
	}
	for _, pattern := range patterns {
		if matchesPathPattern(pattern, candidate) {
			return nil
		}
	}
	return fmt.Errorf("experimentpolicy: %s %q is outside the effective allowance", field, candidate)
}

func matchesPathPattern(pattern, candidate string) bool {
	patternSegments := strings.Split(pattern, "/")
	candidateSegments := strings.Split(candidate, "/")
	var match func(int, int) bool
	match = func(patternIndex, candidateIndex int) bool {
		if patternIndex == len(patternSegments) {
			return candidateIndex == len(candidateSegments)
		}
		if patternSegments[patternIndex] == "**" {
			for next := candidateIndex; next <= len(candidateSegments); next++ {
				if match(patternIndex+1, next) {
					return true
				}
			}
			return false
		}
		if candidateIndex == len(candidateSegments) {
			return false
		}
		matched, err := path.Match(patternSegments[patternIndex], candidateSegments[candidateIndex])
		return err == nil && matched && match(patternIndex+1, candidateIndex+1)
	}
	return match(0, 0)
}

func allowsEvaluatorProtocol(evaluators []EvaluatorAllowance, argv0, protocol string) bool {
	for _, evaluator := range evaluators {
		if evaluator.Argv0 == argv0 && containsString(evaluator.Protocols, protocol) {
			return true
		}
	}
	return false
}

func validateTrustedMeasurementSources(allowed []experiment.Source, definition experiment.Definition) error {
	required := map[experiment.Source]bool{}
	if isHarnessMeasurement(definition.Decision.PrimaryMetric.ID) {
		required[experiment.SourceHarnessMeasured] = true
	} else {
		required[experiment.SourceEvaluatorMeasured] = true
	}
	for _, guard := range definition.Decision.Guards {
		if !guard.Bounded() {
			continue
		}
		if isHarnessMeasurement(guard.ID) {
			required[experiment.SourceHarnessMeasured] = true
		} else {
			required[experiment.SourceEvaluatorMeasured] = true
		}
	}
	for source := range required {
		if !containsSource(allowed, source) {
			return fmt.Errorf("experimentpolicy: required measurement source %q is outside the effective allowance", source)
		}
	}
	return nil
}

func isHarnessMeasurement(id string) bool {
	return id == experiment.EvaluatorWallDurationMetricID || id == experiment.EvaluatorPeakRSSMetricID
}

func validateMandatoryGuards(mandatory []MandatoryGuard, definition experiment.Definition) error {
	required := map[string]bool{}
	for _, entry := range mandatory {
		if entry.Class == definition.Class {
			for _, guard := range entry.Guards {
				required[guard] = true
			}
		}
	}
	present := make(map[string]bool, len(definition.Decision.Guards))
	for _, guard := range definition.Decision.Guards {
		present[guard.ID] = true
	}
	for guard := range required {
		if !present[guard] {
			return fmt.Errorf("experimentpolicy: mandatory guard %q for class %q is absent from the definition", guard, definition.Class)
		}
	}
	return nil
}

func findEnvironment(environments []Environment, id string) (Environment, bool) {
	for _, environment := range environments {
		if environment.ID == id {
			return environment, true
		}
	}
	return Environment{}, false
}

func validateExactDeclaredEnvironment(declared map[string]string, names []string) error {
	if len(declared) != len(names) {
		return fmt.Errorf("experimentpolicy: policy declared environment has %d values, evaluator capabilities declare %d names", len(declared), len(names))
	}
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if seen[name] {
			return fmt.Errorf("experimentpolicy: evaluator capabilities duplicate environment name %q", name)
		}
		seen[name] = true
		if _, ok := declared[name]; !ok {
			return fmt.Errorf("experimentpolicy: policy declared environment omits evaluator name %q", name)
		}
	}
	return nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsSource(values []experiment.Source, want experiment.Source) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
