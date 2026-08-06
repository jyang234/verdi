package experiment

import (
	"fmt"
	"regexp"
	"time"
)

// DefinitionSchema is the only accepted experiment.yaml schema identifier.
const DefinitionSchema = "verdi.experiment/v1"

// spikeRe and questionRe are the grammars for a Definition's spike and
// question references: a spike is a bare "spec/<id>" ref, a question adds
// an "#<anchor>" fragment naming the specific open question.
var (
	spikeRe    = regexp.MustCompile(`^spec/[a-z0-9]+(-[a-z0-9]+)*$`)
	questionRe = regexp.MustCompile(`^spec/[a-z0-9]+(-[a-z0-9]+)*#[a-z0-9]+(-[a-z0-9]+)*$`)
)

// ArtifactRef is the shared {id, digest} identity shape a Definition's
// workload, contract, and fixtures fields use.
type ArtifactRef struct {
	ID     string `yaml:"id" json:"id"`
	Digest string `yaml:"digest" json:"digest"`
}

// Validate checks r's id and digest grammar, naming field in any error.
func (r ArtifactRef) Validate(field string) error {
	if err := ValidateID(r.ID); err != nil {
		return fmt.Errorf("experiment: %s.id: %w", field, err)
	}
	if err := ValidateDigest(r.Digest); err != nil {
		return fmt.Errorf("experiment: %s.digest: %w", field, err)
	}
	return nil
}

// Evaluator is a Definition's registered evaluator identity (AC-3): an
// argument-vector command (never a shell string), its content digest, and
// the digest of the capabilities response it produced at registration.
type Evaluator struct {
	Argv               []string `yaml:"argv" json:"argv"`
	Digest             string   `yaml:"digest" json:"digest"`
	CapabilitiesDigest string   `yaml:"capabilities_digest" json:"capabilities_digest"`
}

// Validate checks argv is a nonempty vector with a nonempty executable,
// and that both digests are well-formed.
func (e Evaluator) Validate() error {
	if len(e.Argv) == 0 {
		return fmt.Errorf("experiment: evaluator.argv must be nonempty")
	}
	if e.Argv[0] == "" {
		return fmt.Errorf("experiment: evaluator.argv[0] must be nonempty")
	}
	if err := ValidateDigest(e.Digest); err != nil {
		return fmt.Errorf("experiment: evaluator.digest: %w", err)
	}
	if err := ValidateDigest(e.CapabilitiesDigest); err != nil {
		return fmt.Errorf("experiment: evaluator.capabilities_digest: %w", err)
	}
	return nil
}

// PrimaryMetric is the Definition's single preregistered ranking metric
// (DC-4).
type PrimaryMetric struct {
	ID          string      `yaml:"id" json:"id"`
	Type        MetricType  `yaml:"type" json:"type"`
	Unit        string      `yaml:"unit" json:"unit"`
	Aggregation Aggregation `yaml:"aggregation" json:"aggregation"`
	Direction   Direction   `yaml:"direction" json:"direction"`
}

// Validate checks pm's id, type, unit, aggregation, and direction.
func (pm PrimaryMetric) Validate() error {
	if err := ValidateID(pm.ID); err != nil {
		return fmt.Errorf("experiment: decision.primary_metric.id: %w", err)
	}
	if err := pm.Type.Validate(); err != nil {
		return fmt.Errorf("experiment: decision.primary_metric.type: %w", err)
	}
	if err := ValidateUnit(pm.Unit); err != nil {
		return fmt.Errorf("experiment: decision.primary_metric.unit: %w", err)
	}
	if err := pm.Aggregation.Validate(); err != nil {
		return fmt.Errorf("experiment: decision.primary_metric.aggregation: %w", err)
	}
	if err := pm.Direction.Validate(); err != nil {
		return fmt.Errorf("experiment: decision.primary_metric.direction: %w", err)
	}
	return nil
}

// Threshold is the exclusive relative-or-absolute significance union
// baseline_improvement and candidate_separation each use.
type Threshold struct {
	Relative *float64 `yaml:"relative,omitempty" json:"relative,omitempty"`
	Absolute *float64 `yaml:"absolute,omitempty" json:"absolute,omitempty"`
}

// Validate enforces the exclusive union — exactly one arm set — and that
// the set arm's value is strictly positive. field names the threshold in
// any error.
func (t Threshold) Validate(field string) error {
	hasRelative := t.Relative != nil
	hasAbsolute := t.Absolute != nil
	if hasRelative == hasAbsolute {
		return fmt.Errorf("experiment: %s must set exactly one of relative or absolute", field)
	}
	if hasRelative && *t.Relative <= 0 {
		return fmt.Errorf("experiment: %s.relative must be > 0, got %v", field, *t.Relative)
	}
	if hasAbsolute && *t.Absolute <= 0 {
		return fmt.Errorf("experiment: %s.absolute must be > 0, got %v", field, *t.Absolute)
	}
	return nil
}

// Guard is one registered guard entry (AC-1). A guard without the bound
// is a required correctness/safety guard whose verdict comes from
// evaluator observations; a guard with the bound is a secondary resource
// bound evaluated over a measurement whose id equals the guard id.
type Guard struct {
	ID                        string   `yaml:"id" json:"id"`
	MaximumRelativeToBaseline *float64 `yaml:"maximum_relative_to_baseline,omitempty" json:"maximum_relative_to_baseline,omitempty"`
}

// Bounded reports whether g carries a secondary resource bound rather
// than being a required correctness/safety guard.
func (g Guard) Bounded() bool { return g.MaximumRelativeToBaseline != nil }

// Validate checks g's id and, when bounded, that the bound is strictly
// positive.
func (g Guard) Validate() error {
	if err := ValidateID(g.ID); err != nil {
		return fmt.Errorf("experiment: decision.guards: %w", err)
	}
	if g.MaximumRelativeToBaseline != nil && *g.MaximumRelativeToBaseline <= 0 {
		return fmt.Errorf("experiment: guard %q: maximum_relative_to_baseline must be > 0, got %v", g.ID, *g.MaximumRelativeToBaseline)
	}
	return nil
}

// Variability is the optional registered variability rule (AC-2 step 7).
type Variability struct {
	MaxRelativeSpread float64 `yaml:"max_relative_spread" json:"max_relative_spread"`
}

// Validate checks the spread bound is strictly positive.
func (v Variability) Validate() error {
	if v.MaxRelativeSpread <= 0 {
		return fmt.Errorf("experiment: decision.variability.max_relative_spread must be > 0, got %v", v.MaxRelativeSpread)
	}
	return nil
}

// DecisionSpec is a Definition's decision contract (AC-1, AC-2, DC-4,
// DC-5): the primary metric, baseline, significance thresholds, guards,
// and optional variability rule.
type DecisionSpec struct {
	PrimaryMetric       PrimaryMetric `yaml:"primary_metric" json:"primary_metric"`
	Baseline            string        `yaml:"baseline" json:"baseline"`
	BaselineImprovement Threshold     `yaml:"baseline_improvement" json:"baseline_improvement"`
	CandidateSeparation Threshold     `yaml:"candidate_separation" json:"candidate_separation"`
	Guards              []Guard       `yaml:"guards" json:"guards"`
	Variability         *Variability  `yaml:"variability,omitempty" json:"variability,omitempty"`
}

// Execution is a Definition's registered execution schedule (AC-4,
// DC-13).
type Execution struct {
	Warmups           int    `yaml:"warmups" json:"warmups"`
	Rounds            int    `yaml:"rounds" json:"rounds"`
	Order             Order  `yaml:"order" json:"order"`
	TimeoutPerRound   string `yaml:"timeout_per_round" json:"timeout_per_round"`
	EnvironmentPolicy string `yaml:"environment_policy" json:"environment_policy"`
}

// Validate checks warmups/rounds bounds, the order enum, that
// timeout_per_round parses as a positive duration (the registered string
// itself is never reformatted), and that environment_policy is a
// nonempty opaque identifier this package does not interpret.
func (e Execution) Validate() error {
	if e.Warmups < 0 {
		return fmt.Errorf("experiment: execution.warmups must be >= 0, got %d", e.Warmups)
	}
	if e.Rounds < 1 {
		return fmt.Errorf("experiment: execution.rounds must be >= 1, got %d", e.Rounds)
	}
	if err := e.Order.Validate(); err != nil {
		return fmt.Errorf("experiment: execution.order: %w", err)
	}
	d, err := time.ParseDuration(e.TimeoutPerRound)
	if err != nil {
		return fmt.Errorf("experiment: execution.timeout_per_round %q does not parse as a duration: %w", e.TimeoutPerRound, err)
	}
	if d <= 0 {
		return fmt.Errorf("experiment: execution.timeout_per_round %q must be > 0", e.TimeoutPerRound)
	}
	if err := nonemptyString("execution.environment_policy", e.EnvironmentPolicy); err != nil {
		return err
	}
	return nil
}

// PolicyRef is a Definition's optional opaque policy reference (owner
// adjudication OD-5): this package never resolves or interprets ref or
// digest, only checks their grammar.
type PolicyRef struct {
	Ref    string  `yaml:"ref" json:"ref"`
	Digest *string `yaml:"digest,omitempty" json:"digest,omitempty"`
}

// Validate checks ref is nonempty and, when present, digest is
// well-formed.
func (p PolicyRef) Validate() error {
	if err := nonemptyString("policy.ref", p.Ref); err != nil {
		return err
	}
	if p.Digest != nil {
		if err := ValidateDigest(*p.Digest); err != nil {
			return fmt.Errorf("experiment: policy.digest: %w", err)
		}
	}
	return nil
}

// Lock is a Definition's optional lock block: present only after a human
// has locked the registration (AC-1). See Locked (normalize.go) for the
// digest-matching semantics that turn this field into an authoritative
// lock.
type Lock struct {
	DefinitionDigest string `yaml:"definition_digest"`
}

// Definition is one verdi.experiment/v1 registration artifact (AC-1):
// the immutable decision contract a human locks before execution counts
// as evidence.
type Definition struct {
	Schema          string           `yaml:"schema"`
	ID              string           `yaml:"id"`
	Spike           string           `yaml:"spike"`
	Question        string           `yaml:"question"`
	BaseCommit      string           `yaml:"base_commit"`
	Candidates      []Candidate      `yaml:"candidates"`
	Evaluator       Evaluator        `yaml:"evaluator"`
	Workload        ArtifactRef      `yaml:"workload"`
	Fixtures        []ArtifactRef    `yaml:"fixtures,omitempty"`
	Contract        ArtifactRef      `yaml:"contract"`
	Decision        DecisionSpec     `yaml:"decision"`
	Execution       Execution        `yaml:"execution"`
	Algorithm       AlgorithmVersion `yaml:"algorithm"`
	RetentionPolicy string           `yaml:"retention_policy"`
	Policy          *PolicyRef       `yaml:"policy,omitempty"`
	ProtectedPaths  []string         `yaml:"protected_paths,omitempty"`
	Lock            *Lock            `yaml:"lock,omitempty"`
}

// DecodeDefinition strict-decodes raw as an experiment.yaml document and
// fully validates it. Unknown fields, unknown enum values, and the
// restricted YAML dialect fail closed through internal/artifact's single
// decode seam; a second YAML document appended to the file fails closed
// through this package's decodeStrictYAML guard (strictdecode.go), so a
// registration can never validate on its first document while carrying
// unseen content behind a trailing "---".
func DecodeDefinition(raw []byte) (Definition, error) {
	var def Definition
	if err := decodeStrictYAML(raw, &def); err != nil {
		return Definition{}, fmt.Errorf("experiment: decoding definition: %w", err)
	}
	if err := def.Validate(); err != nil {
		return Definition{}, err
	}
	return def, nil
}

// Validate checks every field of def against its grammar and every
// cross-field rule the spec requires: candidate count and uniqueness,
// candidate base agreement, a baseline naming a registered candidate,
// unique guard ids disjoint from the primary metric id, and unique,
// well-formed fixture and protected-path lists.
func (def Definition) Validate() error {
	if def.Schema != DefinitionSchema {
		return fmt.Errorf("experiment: unknown definition schema %q, want %q", def.Schema, DefinitionSchema)
	}
	if err := ValidateID(def.ID); err != nil {
		return fmt.Errorf("experiment: definition id: %w", err)
	}
	if !spikeRe.MatchString(def.Spike) {
		return fmt.Errorf("experiment: spike %q does not match ^spec/<id>$", def.Spike)
	}
	if !questionRe.MatchString(def.Question) {
		return fmt.Errorf("experiment: question %q does not match ^spec/<id>#<anchor>$", def.Question)
	}
	if err := ValidateCommit(def.BaseCommit); err != nil {
		return fmt.Errorf("experiment: base_commit: %w", err)
	}

	if len(def.Candidates) < 2 {
		return fmt.Errorf("experiment: candidates must register at least 2 entries, got %d", len(def.Candidates))
	}
	candidateIDs := make(map[string]bool, len(def.Candidates))
	for i, c := range def.Candidates {
		if err := c.Validate(def.BaseCommit); err != nil {
			return fmt.Errorf("experiment: candidates[%d]: %w", i, err)
		}
		if candidateIDs[c.ID] {
			return fmt.Errorf("experiment: candidates: duplicate id %q", c.ID)
		}
		candidateIDs[c.ID] = true
	}

	if err := def.Evaluator.Validate(); err != nil {
		return err
	}
	if err := def.Workload.Validate("workload"); err != nil {
		return err
	}
	if err := def.Contract.Validate("contract"); err != nil {
		return err
	}

	fixtureIDs := make(map[string]bool, len(def.Fixtures))
	for i, f := range def.Fixtures {
		field := fmt.Sprintf("fixtures[%d]", i)
		if err := f.Validate(field); err != nil {
			return err
		}
		if fixtureIDs[f.ID] {
			return fmt.Errorf("experiment: fixtures: duplicate id %q", f.ID)
		}
		fixtureIDs[f.ID] = true
	}

	if err := def.Decision.PrimaryMetric.Validate(); err != nil {
		return err
	}
	if !candidateIDs[def.Decision.Baseline] {
		return fmt.Errorf("experiment: decision.baseline %q does not name a registered candidate", def.Decision.Baseline)
	}
	if err := def.Decision.BaselineImprovement.Validate("decision.baseline_improvement"); err != nil {
		return err
	}
	if err := def.Decision.CandidateSeparation.Validate("decision.candidate_separation"); err != nil {
		return err
	}
	guardIDs := make(map[string]bool, len(def.Decision.Guards))
	for _, g := range def.Decision.Guards {
		if err := g.Validate(); err != nil {
			return err
		}
		if guardIDs[g.ID] {
			return fmt.Errorf("experiment: decision.guards: duplicate id %q", g.ID)
		}
		guardIDs[g.ID] = true
		if g.ID == def.Decision.PrimaryMetric.ID {
			return fmt.Errorf("experiment: decision.guards: guard id %q must not equal the primary metric id", g.ID)
		}
	}
	if def.Decision.Variability != nil {
		if err := def.Decision.Variability.Validate(); err != nil {
			return err
		}
	}

	if err := def.Execution.Validate(); err != nil {
		return err
	}
	if err := def.Algorithm.Validate(); err != nil {
		return fmt.Errorf("experiment: algorithm: %w", err)
	}
	if err := ValidateID(def.RetentionPolicy); err != nil {
		return fmt.Errorf("experiment: retention_policy: %w", err)
	}
	if def.Policy != nil {
		if err := def.Policy.Validate(); err != nil {
			return err
		}
	}

	protectedSeen := make(map[string]bool, len(def.ProtectedPaths))
	for i, p := range def.ProtectedPaths {
		if err := ValidateRepoRelativePath(p); err != nil {
			return fmt.Errorf("experiment: protected_paths[%d]: %w", i, err)
		}
		if protectedSeen[p] {
			return fmt.Errorf("experiment: protected_paths: duplicate entry %q", p)
		}
		protectedSeen[p] = true
	}

	if def.Lock != nil {
		if err := ValidateDigest(def.Lock.DefinitionDigest); err != nil {
			return fmt.Errorf("experiment: lock.definition_digest: %w", err)
		}
	}

	return nil
}
