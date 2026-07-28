package artifact

import "fmt"

const bindingsSchema = "verdi.bindings/v1"

// Binding is one entry in a `verdi.bindings.yaml` sidecar's `bindings:`
// list (I-2, 03 §Declarations and binding): a producer id (an obligation
// name, a golden flow name, or a runtime check id — verdi does not
// constrain the string, since it is upstream's own identifier namespace)
// bound to the evidence kind it supplies and the spec AC ids it is
// evidence-for.
//
// ACs entries are normally bare ac-<slug> ids, implicitly scoped to the
// owning Bindings.Spec. A entry may instead be a fragment-qualified
// spec/<name>#<ac-id> ref (round-6, spec/close-verb ac-3/dc-1) naming a
// DIFFERENT spec's AC outright — 03 §Declarations and binding already
// describes this: "A bindings file that serves both a story and its
// feature disambiguates with the object-fragment form ... whenever a bare
// id would be ambiguous between the two specs." This is that mechanism,
// generalized to any number of other specs (not only "a story and its
// feature") — the self-hosted evidence producer's own bindings file needs
// it to bind several self-hosted stories' ACs (e.g. remote-and-ci#ac-1,
// close-verb#ac-1/#ac-3) from one file. See ResolveBindingAC.
type Binding struct {
	Producer string       `yaml:"producer"`
	Kind     EvidenceKind `yaml:"kind"`
	ACs      []string     `yaml:"acs"`
}

// Validate checks Producer is non-empty, Kind is a known evidence kind, and
// ACs lists at least one well-formed ac-<slug> id or spec/<name>#<ac-id>
// fragment ref.
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
		if IsBareACEntry(ac) {
			continue
		}
		if ref, err := ParseRef(ac); err == nil && ref.Kind == KindSpec && ref.Fragment() {
			continue
		}
		return fmt.Errorf("artifact: binding %q: ac entry %q must be a bare ac-<slug> id or a spec/<name>#<ac-id> fragment ref", b.Producer, ac)
	}
	return nil
}

// IsBareACEntry reports whether a Binding.ACs entry is the bare ac-<slug>
// form — the form that resolves against the owning Bindings.Spec — rather than
// the fragment-qualified spec/<name>#<ac-id> form, which names its own target
// spec and resolves independently of any owning spec (see ResolveBindingAC).
// It is the single classifier both Binding.Validate and ResolveBindingAC use
// to tell the two forms apart, exported so consumers that must treat the legs
// differently — e.g. internal/lint VL-003, which still validates a
// fragment-qualified entry even when a file's own owning `spec:` ref does not
// resolve — classify them identically rather than re-deriving the shape test.
func IsBareACEntry(entry string) bool {
	return acIDRe.MatchString(entry)
}

// ResolveBindingAC resolves one Binding.ACs entry against defaultSpecRef
// (the owning Bindings.Spec): a bare "ac-1" entry resolves to
// (defaultSpecRef, "ac-1"); a fragment-qualified "spec/<name>#<ac-id>" entry
// resolves to its own named spec instead, ignoring defaultSpecRef entirely
// (03 §Declarations and binding's object-fragment disambiguation). Both
// forms are already shape-validated by Binding.Validate(); this function
// additionally requires defaultSpecRef itself to be a valid, unpinned spec
// ref when a bare entry needs it.
//
// A fragment entry that ALSO pins a revision (the out-of-grammar
// spec/<name>@<commit>#<ac-id> form) fails closed here (spec/ritual-traps
// ac-4, finding judged-ac4-pinned-fragment-entry-silently-unpinned).
// Binding.Validate() admits the shape (it checks only Kind==spec &&
// Fragment()), but resolution validates an ac id against the CURRENT
// committed spec and has no way to honor a revision pin; silently dropping
// the @commit — this function's prior behavior — let a caller pin an entry to
// a revision whose AC set differs from HEAD and get a verdict about the wrong
// revision with nothing disclosing it. Honoring the pin (resolving against the
// pinned revision's own AC set) is a disclosed future extension; until then
// the pin is rejected, not silently ignored, so both consumers (internal/lint
// VL-003 and cmd/verdi/selfevidence) stay fail-closed.
func ResolveBindingAC(defaultSpecRef, entry string) (specRef, acID string, err error) {
	if IsBareACEntry(entry) {
		ref, err := ParseRef(defaultSpecRef)
		if err != nil {
			return "", "", fmt.Errorf("artifact: resolving bare ac entry %q: default spec ref %q: %w", entry, defaultSpecRef, err)
		}
		if ref.Kind != KindSpec || ref.Fragment() || ref.Pinned() {
			return "", "", fmt.Errorf("artifact: resolving bare ac entry %q: default spec ref %q must be an unpinned, unfragmented spec/<name> ref", entry, defaultSpecRef)
		}
		return ref.String(), entry, nil
	}
	ref, err := ParseRef(entry)
	if err != nil || ref.Kind != KindSpec || !ref.Fragment() {
		return "", "", fmt.Errorf("artifact: ac entry %q is neither a bare ac-<slug> id nor a spec/<name>#<ac-id> fragment ref", entry)
	}
	if ref.Pinned() {
		return "", "", fmt.Errorf("artifact: ac entry %q pins a revision (the spec/<name>@<commit>#<ac-id> form); ac-id resolution validates against the current committed spec and cannot honor a revision pin — honoring pins is a disclosed future extension", entry)
	}
	return Ref{Kind: ref.Kind, Name: ref.Name}.String(), ref.Object, nil
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
