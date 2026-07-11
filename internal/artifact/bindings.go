package artifact

import "fmt"

const bindingsSchema = "verdi.bindings/v1"

// Binding is one entry in a `verdi.bindings.yaml` sidecar's `bindings:`
// list (I-2, 03 §Declarations and binding): a producer id (an obligation
// name, a golden flow name, or a runtime check id — verdi does not
// constrain the string, since it is upstream's own identifier namespace)
// bound to the evidence kind it supplies and the spec AC ids it is
// evidence-for.
type Binding struct {
	Producer string       `yaml:"producer"`
	Kind     EvidenceKind `yaml:"kind"`
	ACs      []string     `yaml:"acs"`
}

// Validate checks Producer is non-empty, Kind is a known evidence kind, and
// ACs lists at least one well-formed ac-<slug> id.
func (b Binding) Validate() error {
	if b.Producer == "" {
		return fmt.Errorf("artifact: binding has an empty producer")
	}
	if !validEvidenceKinds[b.Kind] {
		return fmt.Errorf("artifact: binding %q: kind %q is not a known evidence kind", b.Producer, b.Kind)
	}
	if len(b.ACs) == 0 {
		return fmt.Errorf("artifact: binding %q declares no ACs", b.Producer)
	}
	for _, ac := range b.ACs {
		if !acIDRe.MatchString(ac) {
			return fmt.Errorf("artifact: binding %q: ac id %q must look like ac-<slug>", b.Producer, ac)
		}
	}
	return nil
}

// Bindings is schema verdi.bindings/v1 (I-2, per ledger): the sidecar a
// service root carries at `verdi.bindings.yaml`, joining upstream producer
// ids to a single spec's AC ids. Verdi-go strict-decodes its own
// `.flowmap.yaml` and has no field for this join, so the sidecar is the
// contract (PLAN.md §3 "AC bindings" row) rather than a stopgap.
type Bindings struct {
	Schema   string    `yaml:"schema"`
	Spec     string    `yaml:"spec"`
	Bindings []Binding `yaml:"bindings"`
}

// DecodeBindings strict-decodes and validates a verdi.bindings.yaml
// sidecar. Unlike .flowmap.yaml, this file is verdi-owned, so it goes
// through the ordinary strict-decode seam like every other schema in this
// package.
func DecodeBindings(data []byte) (*Bindings, error) {
	var bs Bindings
	if err := DecodeStrict(data, &bs); err != nil {
		return nil, err
	}
	if err := bs.Validate(); err != nil {
		return nil, err
	}
	return &bs, nil
}

// Validate checks the schema literal, that Spec parses as an unpinned spec
// ref, that at least one binding is declared, and that every binding is
// individually valid and no producer is bound twice.
func (bs Bindings) Validate() error {
	if bs.Schema != bindingsSchema {
		return fmt.Errorf("artifact: bindings schema %q, want %q", bs.Schema, bindingsSchema)
	}
	ref, err := ParseRef(bs.Spec)
	if err != nil {
		return fmt.Errorf("artifact: bindings spec: %w", err)
	}
	if ref.Kind != KindSpec {
		return fmt.Errorf("artifact: bindings spec %q must be a spec/... ref", bs.Spec)
	}
	if len(bs.Bindings) == 0 {
		return fmt.Errorf("artifact: bindings must declare at least one binding")
	}
	seen := make(map[string]bool, len(bs.Bindings))
	for i, b := range bs.Bindings {
		if err := b.Validate(); err != nil {
			return fmt.Errorf("artifact: bindings[%d]: %w", i, err)
		}
		if seen[b.Producer] {
			return fmt.Errorf("artifact: bindings: producer %q is bound more than once", b.Producer)
		}
		seen[b.Producer] = true
	}
	return nil
}
