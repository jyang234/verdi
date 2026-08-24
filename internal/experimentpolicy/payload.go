// Package experimentpolicy owns the strict CSE experiment_execution payload
// and its commutative reduction after Context Integrity selects applicability.
package experimentpolicy

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// PayloadKind is the registered CSE policy payload key.
const PayloadKind = "experiment_execution"

// EvaluatorAllowance is one exact evaluator executable and its allowed
// protocol versions.
type EvaluatorAllowance struct {
	Argv0     string   `json:"argv0" yaml:"argv0"`
	Protocols []string `json:"protocols" yaml:"protocols"`
}

// Environment is one named shared execution-grant set plus exact
// policy-owned declared environment values. Grants is canonical JSON from the
// execworkspace owner and is compared byte-for-byte during refinement.
type Environment struct {
	ID                  string            `json:"id" yaml:"id"`
	Grants              json.RawMessage   `json:"grants" yaml:"-"`
	DeclaredEnvironment map[string]string `json:"declared_environment" yaml:"declared_environment"`
}

// Limits are the two CSE-owned positive evidence ceilings.
type Limits struct {
	ObservationBytes      int64 `json:"observation_bytes" yaml:"observation_bytes"`
	RetainedArtifactBytes int64 `json:"retained_artifact_bytes" yaml:"retained_artifact_bytes"`
}

// MandatoryGuard binds a sorted set of required guard IDs to one experiment
// class.
type MandatoryGuard struct {
	Class  string   `json:"class" yaml:"class"`
	Guards []string `json:"guards" yaml:"guards"`
}

// Payload is the complete strict experiment_execution v1 field set.
type Payload struct {
	ExperimentPaths           []string             `json:"experiment_paths" yaml:"experiment_paths"`
	CandidatePaths            []string             `json:"candidate_paths" yaml:"candidate_paths"`
	Classes                   []string             `json:"classes" yaml:"classes"`
	Evaluators                []EvaluatorAllowance `json:"evaluators" yaml:"evaluators"`
	Environments              []Environment        `json:"environments" yaml:"environments"`
	Limits                    Limits               `json:"limits" yaml:"limits"`
	TrustedMeasurementSources []experiment.Source  `json:"trusted_measurement_sources" yaml:"trusted_measurement_sources"`
	MandatoryGuards           []MandatoryGuard     `json:"mandatory_guards" yaml:"mandatory_guards"`
}

// PayloadKind implements policyartifact.Payload.
func (*Payload) PayloadKind() string { return PayloadKind }

// Validate enforces the closed payload grammar without normalizing authored
// authority. Every list and map must be present, sorted, and duplicate-free;
// unknown values and malformed shared grants fail closed.
func (p *Payload) Validate() error {
	if p == nil {
		return fmt.Errorf("experimentpolicy: payload is nil")
	}
	if err := validateSortedUniqueStrings("experiment_paths", p.ExperimentPaths, true); err != nil {
		return err
	}
	for _, pattern := range p.ExperimentPaths {
		if err := validatePathPattern(pattern); err != nil {
			return fmt.Errorf("experimentpolicy: experiment_paths: %w", err)
		}
	}
	if err := validateSortedUniqueStrings("candidate_paths", p.CandidatePaths, true); err != nil {
		return err
	}
	for _, pattern := range p.CandidatePaths {
		if err := validatePathPattern(pattern); err != nil {
			return fmt.Errorf("experimentpolicy: candidate_paths: %w", err)
		}
	}
	if err := validateSortedUniqueStrings("classes", p.Classes, true); err != nil {
		return err
	}
	for _, class := range p.Classes {
		if err := experiment.ValidateID(class); err != nil {
			return fmt.Errorf("experimentpolicy: class %q: %w", class, err)
		}
	}
	if p.Evaluators == nil {
		return fmt.Errorf("experimentpolicy: evaluators is missing or null")
	}
	for i, evaluator := range p.Evaluators {
		if i > 0 && p.Evaluators[i-1].Argv0 >= evaluator.Argv0 {
			return fmt.Errorf("experimentpolicy: evaluators must be sorted by argv0 and duplicate-free")
		}
		if _, err := experiment.EvaluatorRepoPath(evaluator.Argv0); err != nil {
			return fmt.Errorf("experimentpolicy: evaluator %q: %w", evaluator.Argv0, err)
		}
		if err := validateSortedUniqueStrings(fmt.Sprintf("evaluator %q protocols", evaluator.Argv0), evaluator.Protocols, true); err != nil {
			return err
		}
		for _, protocol := range evaluator.Protocols {
			if protocol != experiment.EvaluatorProtocolSchema {
				return fmt.Errorf("experimentpolicy: unknown evaluator protocol %q", protocol)
			}
		}
	}
	if p.Environments == nil {
		return fmt.Errorf("experimentpolicy: environments is missing or null")
	}
	for i, environment := range p.Environments {
		if i > 0 && p.Environments[i-1].ID >= environment.ID {
			return fmt.Errorf("experimentpolicy: environments must be sorted by id and duplicate-free")
		}
		if err := experiment.ValidateID(environment.ID); err != nil {
			return fmt.Errorf("experimentpolicy: environment %q: %w", environment.ID, err)
		}
		if _, err := execworkspace.DecodeGrantSet(environment.Grants); err != nil {
			return fmt.Errorf("experimentpolicy: environment %q grants: %w", environment.ID, err)
		}
		if environment.DeclaredEnvironment == nil {
			return fmt.Errorf("experimentpolicy: environment %q declared_environment is missing or null", environment.ID)
		}
		for name, value := range environment.DeclaredEnvironment {
			if name == "" || strings.ContainsAny(name, "=\x00") {
				return fmt.Errorf("experimentpolicy: environment %q declared_environment name %q is invalid", environment.ID, name)
			}
			if strings.ContainsRune(value, 0) {
				return fmt.Errorf("experimentpolicy: environment %q declared_environment value for %q contains NUL", environment.ID, name)
			}
		}
	}
	if p.Limits.ObservationBytes <= 0 {
		return fmt.Errorf("experimentpolicy: limits.observation_bytes must be > 0")
	}
	if p.Limits.RetainedArtifactBytes <= 0 {
		return fmt.Errorf("experimentpolicy: limits.retained_artifact_bytes must be > 0")
	}
	if p.TrustedMeasurementSources == nil {
		return fmt.Errorf("experimentpolicy: trusted_measurement_sources is missing or null")
	}
	for i, source := range p.TrustedMeasurementSources {
		if i > 0 && p.TrustedMeasurementSources[i-1] >= source {
			return fmt.Errorf("experimentpolicy: trusted_measurement_sources must be sorted and duplicate-free")
		}
		if err := source.Validate(); err != nil {
			return fmt.Errorf("experimentpolicy: unknown measurement source %q", source)
		}
	}
	if p.MandatoryGuards == nil {
		return fmt.Errorf("experimentpolicy: mandatory_guards is missing or null")
	}
	for i, mandatory := range p.MandatoryGuards {
		if i > 0 && p.MandatoryGuards[i-1].Class >= mandatory.Class {
			return fmt.Errorf("experimentpolicy: mandatory_guards must be sorted by class and duplicate-free")
		}
		if err := experiment.ValidateID(mandatory.Class); err != nil {
			return fmt.Errorf("experimentpolicy: mandatory guard class %q: %w", mandatory.Class, err)
		}
		if err := validateSortedUniqueStrings(fmt.Sprintf("mandatory guards for class %q", mandatory.Class), mandatory.Guards, true); err != nil {
			return err
		}
		for _, guard := range mandatory.Guards {
			if err := experiment.ValidateID(guard); err != nil {
				return fmt.Errorf("experimentpolicy: mandatory guard %q: %w", guard, err)
			}
		}
	}
	return nil
}

type payloadDoc struct {
	ExperimentPaths           *[]string            `yaml:"experiment_paths"`
	CandidatePaths            *[]string            `yaml:"candidate_paths"`
	Classes                   *[]string            `yaml:"classes"`
	Evaluators                *[]evaluatorDoc      `yaml:"evaluators"`
	Environments              *[]environmentDoc    `yaml:"environments"`
	Limits                    *limitsDoc           `yaml:"limits"`
	TrustedMeasurementSources *[]experiment.Source `yaml:"trusted_measurement_sources"`
	MandatoryGuards           *[]mandatoryGuardDoc `yaml:"mandatory_guards"`
}

type evaluatorDoc struct {
	Argv0     *string   `yaml:"argv0"`
	Protocols *[]string `yaml:"protocols"`
}

type environmentDoc struct {
	ID                  *string            `yaml:"id"`
	Grants              *interface{}       `yaml:"grants"`
	DeclaredEnvironment *map[string]string `yaml:"declared_environment"`
}

type limitsDoc struct {
	ObservationBytes      *int64 `yaml:"observation_bytes"`
	RetainedArtifactBytes *int64 `yaml:"retained_artifact_bytes"`
}

type mandatoryGuardDoc struct {
	Class  *string   `yaml:"class"`
	Guards *[]string `yaml:"guards"`
}

// DecodePayload strict-decodes one payload and preserves shared grants as the
// exact canonical bytes emitted by execworkspace.
func DecodePayload(raw []byte) (*Payload, error) {
	var doc payloadDoc
	if err := artifact.DecodeStrict(raw, &doc); err != nil {
		return nil, fmt.Errorf("experimentpolicy: decode payload: %w", err)
	}
	if doc.ExperimentPaths == nil {
		return nil, fmt.Errorf("experimentpolicy: experiment_paths is missing or null")
	}
	if doc.CandidatePaths == nil {
		return nil, fmt.Errorf("experimentpolicy: candidate_paths is missing or null")
	}
	if doc.Classes == nil {
		return nil, fmt.Errorf("experimentpolicy: classes is missing or null")
	}
	if doc.Evaluators == nil {
		return nil, fmt.Errorf("experimentpolicy: evaluators is missing or null")
	}
	if doc.Environments == nil {
		return nil, fmt.Errorf("experimentpolicy: environments is missing or null")
	}
	if doc.Limits == nil || doc.Limits.ObservationBytes == nil || doc.Limits.RetainedArtifactBytes == nil {
		return nil, fmt.Errorf("experimentpolicy: limits and both byte ceilings are required")
	}
	if doc.TrustedMeasurementSources == nil {
		return nil, fmt.Errorf("experimentpolicy: trusted_measurement_sources is missing or null")
	}
	if doc.MandatoryGuards == nil {
		return nil, fmt.Errorf("experimentpolicy: mandatory_guards is missing or null")
	}

	payload := &Payload{
		ExperimentPaths:           append([]string(nil), (*doc.ExperimentPaths)...),
		CandidatePaths:            append([]string(nil), (*doc.CandidatePaths)...),
		Classes:                   append([]string(nil), (*doc.Classes)...),
		Evaluators:                make([]EvaluatorAllowance, len(*doc.Evaluators)),
		Environments:              make([]Environment, len(*doc.Environments)),
		Limits:                    Limits{ObservationBytes: *doc.Limits.ObservationBytes, RetainedArtifactBytes: *doc.Limits.RetainedArtifactBytes},
		TrustedMeasurementSources: append([]experiment.Source(nil), (*doc.TrustedMeasurementSources)...),
		MandatoryGuards:           make([]MandatoryGuard, len(*doc.MandatoryGuards)),
	}
	for i, evaluator := range *doc.Evaluators {
		if evaluator.Argv0 == nil || evaluator.Protocols == nil {
			return nil, fmt.Errorf("experimentpolicy: evaluators[%d] requires argv0 and protocols", i)
		}
		payload.Evaluators[i] = EvaluatorAllowance{Argv0: *evaluator.Argv0, Protocols: append([]string(nil), (*evaluator.Protocols)...)}
	}
	for i, environment := range *doc.Environments {
		if environment.ID == nil || environment.Grants == nil || environment.DeclaredEnvironment == nil {
			return nil, fmt.Errorf("experimentpolicy: environments[%d] requires id, grants, and declared_environment", i)
		}
		grantBytes, err := canonicalGrantBytes(environment.Grants)
		if err != nil {
			return nil, fmt.Errorf("experimentpolicy: environments[%d] grants: %w", i, err)
		}
		payload.Environments[i] = Environment{
			ID: *environment.ID, Grants: grantBytes, DeclaredEnvironment: cloneStringMap(*environment.DeclaredEnvironment),
		}
	}
	for i, mandatory := range *doc.MandatoryGuards {
		if mandatory.Class == nil || mandatory.Guards == nil {
			return nil, fmt.Errorf("experimentpolicy: mandatory_guards[%d] requires class and guards", i)
		}
		payload.MandatoryGuards[i] = MandatoryGuard{Class: *mandatory.Class, Guards: append([]string(nil), (*mandatory.Guards)...)}
	}
	if err := payload.Validate(); err != nil {
		return nil, err
	}
	return payload, nil
}

func canonicalGrantBytes(value interface{}) ([]byte, error) {
	canonical, err := canonjson.Marshal(value)
	if err != nil {
		return nil, err
	}
	grants, err := execworkspace.DecodeGrantSet(canonical)
	if err != nil {
		return nil, err
	}
	return execworkspace.EncodeGrantSet(grants)
}

func validateSortedUniqueStrings(field string, values []string, requirePresent bool) error {
	if requirePresent && values == nil {
		return fmt.Errorf("experimentpolicy: %s is missing or null", field)
	}
	for i, value := range values {
		if value == "" {
			return fmt.Errorf("experimentpolicy: %s[%d] is empty", field, i)
		}
		if i > 0 && values[i-1] >= value {
			return fmt.Errorf("experimentpolicy: %s must be sorted and duplicate-free", field)
		}
	}
	return nil
}

func validatePathPattern(pattern string) error {
	if pattern == "" || strings.HasPrefix(pattern, "/") || strings.Contains(pattern, "\\") || strings.HasSuffix(pattern, "/") {
		return fmt.Errorf("path pattern %q is not canonical repo-relative grammar", pattern)
	}
	for _, segment := range strings.Split(pattern, "/") {
		if segment == "" || segment == "." || segment == ".." || (strings.Contains(segment, "**") && segment != "**") {
			return fmt.Errorf("path pattern %q is not canonical repo-relative grammar", pattern)
		}
	}
	if _, err := path.Match(pattern, pattern); err != nil {
		return fmt.Errorf("path pattern %q: %w", pattern, err)
	}
	return nil
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func init() {
	policyartifact.RegisterLayeredPayloadKind(PayloadKind, func(raw []byte) (policyartifact.Payload, error) {
		return DecodePayload(raw)
	})
}
