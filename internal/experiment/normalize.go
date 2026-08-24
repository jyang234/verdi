package experiment

import (
	"fmt"
	"slices"

	"github.com/jyang234/verdi/internal/canonjson"
)

// NormalizedDefinition is the canonical, lock-excluded projection of a
// validated Definition that DefinitionDigest hashes: every registered
// input of the decision contract, with candidate order preserved (order
// is significant for deterministic rotation). The lock block is never
// part of this projection — the lock's own digest is computed OVER it, so
// including the lock would be circular and would let a lock's mere
// presence perturb the identity it is supposed to attest to.
type NormalizedDefinition struct {
	Schema          string            `json:"schema"`
	Class           string            `json:"class,omitempty"`
	Reproduction    *ReproductionRule `json:"reproduction,omitempty"`
	ID              string            `json:"id"`
	Spike           string            `json:"spike"`
	Question        string            `json:"question"`
	BaseCommit      string            `json:"base_commit"`
	Candidates      []Candidate       `json:"candidates"`
	Evaluator       Evaluator         `json:"evaluator"`
	Workload        ArtifactRef       `json:"workload"`
	Fixtures        []ArtifactRef     `json:"fixtures,omitempty"`
	Contract        ArtifactRef       `json:"contract"`
	Decision        DecisionSpec      `json:"decision"`
	Execution       Execution         `json:"execution"`
	Algorithm       AlgorithmVersion  `json:"algorithm"`
	RetentionPolicy string            `json:"retention_policy"`
	Policy          *PolicyRef        `json:"policy,omitempty"`
	ProtectedPaths  []string          `json:"protected_paths,omitempty"`
}

// NormalizeDefinition validates def and returns its canonical,
// lock-excluded projection. Two decodes of identical registered bytes
// normalize to equal values (and therefore equal digests); changing any
// registered input — a candidate digest, a threshold, rounds,
// environment_policy, and so on — changes the projection.
func NormalizeDefinition(def Definition) (NormalizedDefinition, error) {
	if err := def.Validate(); err != nil {
		return NormalizedDefinition{}, err
	}
	evaluator := def.Evaluator
	evaluator.Argv = slices.Clone(def.Evaluator.Argv)
	decision := cloneDecisionSpec(def.Decision)
	return NormalizedDefinition{
		Schema:          def.Schema,
		Class:           def.Class,
		Reproduction:    cloneReproductionRule(def.Reproduction),
		ID:              def.ID,
		Spike:           def.Spike,
		Question:        def.Question,
		BaseCommit:      def.BaseCommit,
		Candidates:      slices.Clone(def.Candidates),
		Evaluator:       evaluator,
		Workload:        def.Workload,
		Fixtures:        slices.Clone(def.Fixtures),
		Contract:        def.Contract,
		Decision:        decision,
		Execution:       def.Execution,
		Algorithm:       def.Algorithm,
		RetentionPolicy: def.RetentionPolicy,
		Policy:          clonePolicyRef(def.Policy),
		ProtectedPaths:  slices.Clone(def.ProtectedPaths),
	}, nil
}

func cloneDecisionSpec(decision DecisionSpec) DecisionSpec {
	cloned := decision
	cloned.BaselineImprovement = cloneThreshold(decision.BaselineImprovement)
	cloned.CandidateSeparation = cloneThreshold(decision.CandidateSeparation)
	cloned.Guards = slices.Clone(decision.Guards)
	for i := range cloned.Guards {
		if bound := decision.Guards[i].MaximumRelativeToBaseline; bound != nil {
			value := *bound
			cloned.Guards[i].MaximumRelativeToBaseline = &value
		}
	}
	if decision.Variability != nil {
		value := *decision.Variability
		cloned.Variability = &value
	}
	return cloned
}

func cloneThreshold(threshold Threshold) Threshold {
	cloned := threshold
	if threshold.Relative != nil {
		value := *threshold.Relative
		cloned.Relative = &value
	}
	if threshold.Absolute != nil {
		value := *threshold.Absolute
		cloned.Absolute = &value
	}
	return cloned
}

func clonePolicyRef(policy *PolicyRef) *PolicyRef {
	if policy == nil {
		return nil
	}
	cloned := *policy
	if policy.Digest != nil {
		value := *policy.Digest
		cloned.Digest = &value
	}
	return &cloned
}

func cloneReproductionRule(rule *ReproductionRule) *ReproductionRule {
	if rule == nil {
		return nil
	}
	cloned := *rule
	return &cloned
}

// DefinitionDigest returns def's immutable experiment identity: the
// internal/canonjson.Digest of its NormalizeDefinition projection. This IS
// the identity a lock block pins (Locked) — any change to a registered
// input yields a different digest, never a mutation of the same identity.
//
// NUMERIC NORMALIZATION differs between this digest and ResultDigest, and
// the difference is load-bearing for CO-3 (byte-identity across writers):
//
//   - DefinitionDigest hashes the PROJECTION, whose numeric fields are
//     typed float64. The decoder has already parsed "0.25", "0.250" and
//     "2.5e-1" to the same float64, and Go re-encodes it in one fixed
//     shortest-round-trip form, so a definition's digest is independent of
//     how the YAML spelled its numbers.
//   - ResultDigest hashes the decoded Result, whose numeric fields are
//     json.Number — the document's EXACT literal, preserved verbatim
//     through canonjson. "18", "18.0" and "1.8e1" are three different
//     digests.
//
// Result WRITERS therefore have an obligation definition authors do not:
// every writer must emit result numerics in ONE fixed formatting, or two
// writers that agree on every value still produce different bytes and CO-3
// fails. The decision-engine lane emits via strconv.FormatFloat(v, 'f', -1,
// 64) consistently; any other writer of verdi.experiment-result/v1 must
// match that formatting exactly.
func DefinitionDigest(def Definition) (string, error) {
	n, err := NormalizeDefinition(def)
	if err != nil {
		return "", err
	}
	d, err := canonjson.Digest(n)
	if err != nil {
		return "", fmt.Errorf("experiment: computing definition digest: %w", err)
	}
	return d, nil
}

// Locked reports whether def carries a lock block whose digest matches
// def's own computed DefinitionDigest. A present lock block whose digest
// does NOT match the computed digest is a hard error — a tampered or
// stale lock — and is never treated as "unlocked": only the true absence
// of a lock block reports (false, nil).
func Locked(def Definition) (bool, error) {
	if def.Lock == nil {
		return false, nil
	}
	want, err := DefinitionDigest(def)
	if err != nil {
		return false, err
	}
	if def.Lock.DefinitionDigest != want {
		return false, fmt.Errorf("experiment: lock.definition_digest %q does not match the computed definition digest %q (tampered or stale lock)", def.Lock.DefinitionDigest, want)
	}
	return true, nil
}

// ResultDigest returns res's canonical content digest: the
// internal/canonjson.Digest of the validated Result value itself.
// ratification.yaml's result_digest field must equal this value to bind a
// ratification to the exact result it responds to.
//
// Unlike DefinitionDigest, this digest BINDS the decoded Result's exact
// json.Number literals rather than normalizing them through typed floats —
// see DefinitionDigest's numeric-normalization note for the writer
// obligation that follows from it.
func ResultDigest(res Result) (string, error) {
	if err := res.Validate(); err != nil {
		return "", err
	}
	d, err := canonjson.Digest(res)
	if err != nil {
		return "", fmt.Errorf("experiment: computing result digest: %w", err)
	}
	return d, nil
}
