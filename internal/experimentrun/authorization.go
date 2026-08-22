package experimentrun

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
)

// AuthorizationResolver is the consumer-owned authority boundary for an
// execution run. Wave 3B intentionally supplies no concrete resolver.
type AuthorizationResolver interface {
	ResolveExecutionAuthorization(
		ctx context.Context,
		def experiment.Definition,
		capabilities experiment.Capabilities,
	) (ExecutionAuthorization, error)
}

// ExecutionAuthorization is the exact authority fact an AuthorizationResolver
// returns for one locked definition and capabilities document.
type ExecutionAuthorization struct {
	EnvironmentPolicy string
	AuthorityDigest   string
	GrantBytes        []byte
	DeclaredEnv       map[string]string
}

// AuthorizedExecution is a validated authorization plus the decoded shared
// grant set it names. Its values are independent copies of resolver output.
type AuthorizedExecution struct {
	Authorization ExecutionAuthorization
	Grants        execworkspace.GrantSet
}

// ResolveAuthorization obtains one authorization from resolver and rejects any
// unavailable, malformed, or mismatched required execution fact.
func ResolveAuthorization(ctx context.Context, resolver AuthorizationResolver, def experiment.Definition, capabilities experiment.Capabilities) (AuthorizedExecution, error) {
	if ctx == nil {
		return AuthorizedExecution{}, fmt.Errorf("experimentrun: resolve authorization: nil context")
	}
	if resolver == nil {
		return AuthorizedExecution{}, fmt.Errorf("experimentrun: resolve authorization: resolver is nil")
	}
	if err := def.Validate(); err != nil {
		return AuthorizedExecution{}, fmt.Errorf("experimentrun: resolve authorization definition: %w", err)
	}
	if err := capabilities.Validate(); err != nil {
		return AuthorizedExecution{}, fmt.Errorf("experimentrun: resolve authorization capabilities: %w", err)
	}
	authorization, err := resolver.ResolveExecutionAuthorization(ctx, def, capabilities)
	if err != nil {
		return AuthorizedExecution{}, fmt.Errorf("experimentrun: resolve execution authorization: %w", err)
	}
	return validateAuthorization(def, capabilities, authorization)
}

func validateAuthorization(def experiment.Definition, capabilities experiment.Capabilities, authorization ExecutionAuthorization) (AuthorizedExecution, error) {
	if authorization.EnvironmentPolicy != def.Execution.EnvironmentPolicy {
		return AuthorizedExecution{}, fmt.Errorf("experimentrun: authorization environment policy %q does not match definition %q", authorization.EnvironmentPolicy, def.Execution.EnvironmentPolicy)
	}
	if err := experiment.ValidateDigest(authorization.AuthorityDigest); err != nil {
		return AuthorizedExecution{}, fmt.Errorf("experimentrun: authorization authority digest: %w", err)
	}
	grants, err := execworkspace.DecodeGrantSet(authorization.GrantBytes)
	if err != nil {
		return AuthorizedExecution{}, fmt.Errorf("experimentrun: authorization grant bytes: %w", err)
	}
	if err := experiment.ValidateDefinitionCapabilities(def, capabilities); err != nil {
		return AuthorizedExecution{}, fmt.Errorf("experimentrun: capability membership: %w", err)
	}
	if err := validateFixedObservers(def, capabilities); err != nil {
		return AuthorizedExecution{}, err
	}
	if capabilities.RequiresElevated {
		return AuthorizedExecution{}, fmt.Errorf("experimentrun: evaluator requires elevated execution, which Wave 3B does not support")
	}
	if err := validateDeclaredEnvironment(capabilities, authorization.DeclaredEnv); err != nil {
		return AuthorizedExecution{}, err
	}
	if err := validateGrantRequirements(def, capabilities, grants); err != nil {
		return AuthorizedExecution{}, err
	}
	return AuthorizedExecution{
		Authorization: cloneExecutionAuthorization(authorization),
		Grants:        cloneGrantSet(grants),
	}, nil
}

func validateFixedObservers(def experiment.Definition, capabilities experiment.Capabilities) error {
	required := map[string]bool{}
	if isFixedObserver(def.Decision.PrimaryMetric.ID) {
		required[def.Decision.PrimaryMetric.ID] = true
	}
	for _, guard := range def.Decision.Guards {
		if guard.Bounded() && isFixedObserver(guard.ID) {
			required[guard.ID] = true
		}
	}
	available := make(map[string]bool, len(capabilities.Observers))
	for _, observer := range capabilities.Observers {
		available[observer] = true
	}
	for observer := range required {
		if !available[observer] {
			return fmt.Errorf("experimentrun: required fixed observer %q is absent from capabilities", observer)
		}
		if observer == experiment.EvaluatorPeakRSSMetricID && runtime.GOOS != "linux" {
			return fmt.Errorf("experimentrun: required fixed observer %q is unavailable on %s", observer, runtime.GOOS)
		}
	}
	return nil
}

func isFixedObserver(id string) bool {
	return id == experiment.EvaluatorWallDurationMetricID || id == experiment.EvaluatorPeakRSSMetricID
}

func validateDeclaredEnvironment(capabilities experiment.Capabilities, declared map[string]string) error {
	want := make(map[string]bool, len(capabilities.Environment))
	for _, name := range capabilities.Environment {
		if want[name] {
			return fmt.Errorf("experimentrun: capabilities duplicate environment %q", name)
		}
		want[name] = true
	}
	if len(declared) != len(want) {
		return fmt.Errorf("experimentrun: declared environment has %d entries, want exactly %d capability-declared entries", len(declared), len(want))
	}
	for name := range want {
		if _, ok := declared[name]; !ok {
			return fmt.Errorf("experimentrun: declared environment omits required %q", name)
		}
	}
	for name, value := range declared {
		if !want[name] {
			return fmt.Errorf("experimentrun: declared environment %q is not capability-declared", name)
		}
		if name == "" || containsNULOrEquals(name) {
			return fmt.Errorf("experimentrun: declared environment name %q is invalid", name)
		}
		if containsNUL(value) {
			return fmt.Errorf("experimentrun: declared environment value for %q contains NUL", name)
		}
	}
	return nil
}

func validateGrantRequirements(def experiment.Definition, capabilities experiment.Capabilities, grants execworkspace.GrantSet) error {
	process, ok := grants.Get(execworkspace.GrantProcessExecution)
	if !ok || !containsExact(process.Argv0s, def.Evaluator.Argv[0]) {
		return fmt.Errorf("experimentrun: execution grants do not allow exact evaluator argv0 %q", def.Evaluator.Argv[0])
	}
	timeout, ok := grants.Get(execworkspace.GrantTimeouts)
	if !ok {
		return fmt.Errorf("experimentrun: execution grants omit required timeout")
	}
	registeredTimeout, err := time.ParseDuration(def.Execution.TimeoutPerRound)
	if err != nil {
		return fmt.Errorf("experimentrun: parse definition timeout: %w", err)
	}
	if time.Duration(timeout.Seconds)*time.Second != registeredTimeout {
		return fmt.Errorf("experimentrun: granted timeout %ds does not equal registered timeout %q", timeout.Seconds, def.Execution.TimeoutPerRound)
	}
	_, networkGranted := grants.Get(execworkspace.GrantNetwork)
	if networkGranted != capabilities.RequiresNetwork {
		return fmt.Errorf("experimentrun: network grant presence %t does not match evaluator network requirement %t", networkGranted, capabilities.RequiresNetwork)
	}
	return nil
}

func containsExact(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsNULOrEquals(value string) bool {
	for _, rune := range value {
		if rune == 0 || rune == '=' {
			return true
		}
	}
	return false
}

func containsNUL(value string) bool {
	for _, rune := range value {
		if rune == 0 {
			return true
		}
	}
	return false
}

func cloneExecutionAuthorization(in ExecutionAuthorization) ExecutionAuthorization {
	out := in
	out.GrantBytes = append([]byte(nil), in.GrantBytes...)
	out.DeclaredEnv = make(map[string]string, len(in.DeclaredEnv))
	for key, value := range in.DeclaredEnv {
		out.DeclaredEnv[key] = value
	}
	return out
}

func cloneGrantSet(in execworkspace.GrantSet) execworkspace.GrantSet {
	out := execworkspace.GrantSet{Grants: make([]execworkspace.Grant, len(in.Grants))}
	for i, grant := range in.Grants {
		out.Grants[i] = grant
		out.Grants[i].Paths = append([]string(nil), grant.Paths...)
		out.Grants[i].Argv0s = append([]string(nil), grant.Argv0s...)
		if grant.Ceilings != nil {
			out.Grants[i].Ceilings = make(map[string]int, len(grant.Ceilings))
			for key, value := range grant.Ceilings {
				out.Grants[i].Ceilings[key] = value
			}
		}
	}
	return out
}
