package experimentpolicy

import (
	"bytes"
	"fmt"
	"maps"
	"sort"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/experiment"
)

// Decision is the immutable commutative reduction of one sealed applicable
// payload selection. Payload returns only strict deep copies.
type Decision struct {
	authorityDigest string
	selectionDigest string
	payload         Payload
	seal            string
}

type decisionContent struct {
	AuthorityDigest string  `json:"authority_digest"`
	SelectionDigest string  `json:"selection_digest"`
	Payload         Payload `json:"payload"`
}

// Resolve intersects every allowlist and environment ID set, takes minimum
// limits, unions mandatory guards, and applies denial dominance over the
// already-selected Context Integrity ledger. It has no loader, scope logic,
// precedence, fallback, or shared-grant refinement.
func Resolve(selection *contextcompile.ApplicablePayloadSelection) (*Decision, error) {
	if selection == nil {
		return nil, fmt.Errorf("experimentpolicy: resolve: applicable payload selection is nil")
	}
	kind, err := selection.Kind()
	if err != nil {
		return nil, fmt.Errorf("experimentpolicy: resolve selection: %w", err)
	}
	if kind != PayloadKind {
		return nil, fmt.Errorf("experimentpolicy: resolve selection kind %q, want %q", kind, PayloadKind)
	}
	authorityDigest, err := selection.EffectiveDigest()
	if err != nil {
		return nil, fmt.Errorf("experimentpolicy: resolve selection authority: %w", err)
	}
	selectionDigest, err := selection.Digest()
	if err != nil {
		return nil, fmt.Errorf("experimentpolicy: resolve selection digest: %w", err)
	}
	layers, err := selection.Layers()
	if err != nil {
		return nil, fmt.Errorf("experimentpolicy: resolve layers: %w", err)
	}
	if len(layers) == 0 {
		return nil, fmt.Errorf("experimentpolicy: no applicable %s payload", PayloadKind)
	}

	payloads := make([]*Payload, len(layers))
	for i, layer := range layers {
		payload, ok := layer.Payload.(*Payload)
		if !ok {
			return nil, fmt.Errorf("experimentpolicy: policy %s payload is %T, want *experimentpolicy.Payload", layer.PolicyID, layer.Payload)
		}
		payloads[i] = payload
	}

	// Exact grants and declared values constrain only IDs surviving the
	// complete commutative intersection, never an intermediate pair.
	survivingEnvironmentIDs := make(map[string]bool, len(payloads[0].Environments))
	for _, environment := range payloads[0].Environments {
		survivingEnvironmentIDs[environment.ID] = true
	}
	for _, payload := range payloads[1:] {
		present := make(map[string]bool, len(payload.Environments))
		for _, environment := range payload.Environments {
			present[environment.ID] = true
		}
		for id := range survivingEnvironmentIDs {
			if !present[id] {
				delete(survivingEnvironmentIDs, id)
			}
		}
	}

	effective, err := clonePayload(payloads[0])
	if err != nil {
		return nil, fmt.Errorf("experimentpolicy: policy %s: %w", layers[0].PolicyID, err)
	}
	environments := make([]Environment, 0, len(survivingEnvironmentIDs))
	for _, environment := range effective.Environments {
		if survivingEnvironmentIDs[environment.ID] {
			environments = append(environments, environment)
		}
	}
	effective.Environments = environments
	for i, layer := range layers[1:] {
		if err := reduceInto(effective, payloads[i+1]); err != nil {
			return nil, fmt.Errorf("experimentpolicy: policy %s refinement: %w", layer.PolicyID, err)
		}
	}
	if err := refuseEmptyAllowances(effective); err != nil {
		return nil, err
	}
	if err := effective.Validate(); err != nil {
		return nil, fmt.Errorf("experimentpolicy: effective payload: %w", err)
	}

	decision := &Decision{authorityDigest: authorityDigest, selectionDigest: selectionDigest, payload: *effective}
	seal, err := decision.contentDigest()
	if err != nil {
		return nil, fmt.Errorf("experimentpolicy: seal decision: %w", err)
	}
	decision.seal = seal
	return decision, nil
}

// Payload returns a strict, non-aliasing snapshot of the effective payload.
func (d *Decision) Payload() (Payload, error) {
	if err := d.checkSeal(); err != nil {
		return Payload{}, err
	}
	cloned, err := clonePayload(&d.payload)
	if err != nil {
		return Payload{}, err
	}
	return *cloned, nil
}

// Digest returns the sealed decision identity.
func (d *Decision) Digest() (string, error) {
	if err := d.checkSeal(); err != nil {
		return "", err
	}
	return d.seal, nil
}

func (d *Decision) checkSeal() error {
	if d == nil || d.seal == "" {
		return fmt.Errorf("experimentpolicy: decision was not produced by Resolve")
	}
	digest, err := d.contentDigest()
	if err != nil {
		return err
	}
	if digest != d.seal {
		return fmt.Errorf("experimentpolicy: decision was modified after resolution")
	}
	return nil
}

func (d *Decision) contentDigest() (string, error) {
	return canonjson.Digest(decisionContent{
		AuthorityDigest: d.authorityDigest, SelectionDigest: d.selectionDigest, Payload: d.payload,
	})
}

func reduceInto(effective *Payload, layer *Payload) error {
	if err := layer.Validate(); err != nil {
		return err
	}
	effective.ExperimentPaths = intersectStrings(effective.ExperimentPaths, layer.ExperimentPaths)
	effective.CandidatePaths = intersectStrings(effective.CandidatePaths, layer.CandidatePaths)
	effective.Classes = intersectStrings(effective.Classes, layer.Classes)
	effective.Evaluators = intersectEvaluators(effective.Evaluators, layer.Evaluators)
	environments, err := intersectEnvironments(effective.Environments, layer.Environments)
	if err != nil {
		return err
	}
	effective.Environments = environments
	if layer.Limits.ObservationBytes < effective.Limits.ObservationBytes {
		effective.Limits.ObservationBytes = layer.Limits.ObservationBytes
	}
	if layer.Limits.RetainedArtifactBytes < effective.Limits.RetainedArtifactBytes {
		effective.Limits.RetainedArtifactBytes = layer.Limits.RetainedArtifactBytes
	}
	effective.TrustedMeasurementSources = intersectSources(effective.TrustedMeasurementSources, layer.TrustedMeasurementSources)
	effective.MandatoryGuards = unionMandatoryGuards(effective.MandatoryGuards, layer.MandatoryGuards)
	return nil
}

func refuseEmptyAllowances(payload *Payload) error {
	tests := []struct {
		name  string
		empty bool
	}{
		{"experiment_paths", len(payload.ExperimentPaths) == 0},
		{"candidate_paths", len(payload.CandidatePaths) == 0},
		{"classes", len(payload.Classes) == 0},
		{"evaluators/protocols", countEvaluatorProtocols(payload.Evaluators) == 0},
		{"environments", len(payload.Environments) == 0},
		{"trusted_measurement_sources", len(payload.TrustedMeasurementSources) == 0},
	}
	for _, test := range tests {
		if test.empty {
			return fmt.Errorf("experimentpolicy: %s intersection is empty; denial dominates and no lower layer may restore it", test.name)
		}
	}
	return nil
}

func intersectStrings(left, right []string) []string {
	out := make([]string, 0)
	i, j := 0, 0
	for i < len(left) && j < len(right) {
		switch {
		case left[i] == right[j]:
			out = append(out, left[i])
			i++
			j++
		case left[i] < right[j]:
			i++
		default:
			j++
		}
	}
	return out
}

func intersectEvaluators(left, right []EvaluatorAllowance) []EvaluatorAllowance {
	rightByID := make(map[string][]string, len(right))
	for _, evaluator := range right {
		rightByID[evaluator.Argv0] = evaluator.Protocols
	}
	out := make([]EvaluatorAllowance, 0)
	for _, evaluator := range left {
		protocols, ok := rightByID[evaluator.Argv0]
		if !ok {
			continue
		}
		intersection := intersectStrings(evaluator.Protocols, protocols)
		if len(intersection) > 0 {
			out = append(out, EvaluatorAllowance{Argv0: evaluator.Argv0, Protocols: intersection})
		}
	}
	return out
}

func countEvaluatorProtocols(evaluators []EvaluatorAllowance) int {
	total := 0
	for _, evaluator := range evaluators {
		total += len(evaluator.Protocols)
	}
	return total
}

func intersectEnvironments(left, right []Environment) ([]Environment, error) {
	rightByID := make(map[string]Environment, len(right))
	for _, environment := range right {
		rightByID[environment.ID] = environment
	}
	out := make([]Environment, 0)
	for _, environment := range left {
		refinement, ok := rightByID[environment.ID]
		if !ok {
			continue
		}
		if !bytes.Equal(environment.Grants, refinement.Grants) {
			return nil, fmt.Errorf("environment %q grant bytes differ across naming layers", environment.ID)
		}
		if !maps.Equal(environment.DeclaredEnvironment, refinement.DeclaredEnvironment) {
			return nil, fmt.Errorf("environment %q declared environment differs across naming layers", environment.ID)
		}
		out = append(out, Environment{
			ID: environment.ID, Grants: append([]byte(nil), environment.Grants...), DeclaredEnvironment: cloneStringMap(environment.DeclaredEnvironment),
		})
	}
	return out, nil
}

func intersectSources(left, right []experiment.Source) []experiment.Source {
	out := make([]experiment.Source, 0)
	i, j := 0, 0
	for i < len(left) && j < len(right) {
		switch {
		case left[i] == right[j]:
			out = append(out, left[i])
			i++
			j++
		case left[i] < right[j]:
			i++
		default:
			j++
		}
	}
	return out
}

func unionMandatoryGuards(left, right []MandatoryGuard) []MandatoryGuard {
	byClass := make(map[string][]string, len(left)+len(right))
	for _, mandatory := range left {
		byClass[mandatory.Class] = append([]string(nil), mandatory.Guards...)
	}
	for _, mandatory := range right {
		byClass[mandatory.Class] = unionStrings(byClass[mandatory.Class], mandatory.Guards)
	}
	classes := make([]string, 0, len(byClass))
	for class := range byClass {
		classes = append(classes, class)
	}
	sort.Strings(classes)
	out := make([]MandatoryGuard, len(classes))
	for i, class := range classes {
		out[i] = MandatoryGuard{Class: class, Guards: byClass[class]}
	}
	return out
}

func unionStrings(left, right []string) []string {
	seen := make(map[string]bool, len(left)+len(right))
	for _, value := range left {
		seen[value] = true
	}
	for _, value := range right {
		seen[value] = true
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func clonePayload(in *Payload) (*Payload, error) {
	raw, err := canonjson.Marshal(in)
	if err != nil {
		return nil, err
	}
	return DecodePayload(raw)
}
