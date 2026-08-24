package experimentrun

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/experiment"
)

// InputBindingSchema is the sole operation-scoped input-binding wire schema.
const InputBindingSchema = "verdi.experiment-input-bindings/v1"

// InputSlot identifies one closed role in a locked experiment definition.
type InputSlot string

const (
	InputSlotContract InputSlot = "contract"
	InputSlotWorkload InputSlot = "workload"
)

const fixtureSlotPrefix = "fixture:"

// FixtureInputSlot returns the slot for one definition fixture ID.
func FixtureInputSlot(id string) InputSlot {
	return InputSlot(fixtureSlotPrefix + id)
}

// Validate rejects slots outside workload, contract, and fixture:<fixture-id>.
func (s InputSlot) Validate() error {
	switch s {
	case InputSlotContract, InputSlotWorkload:
		return nil
	}
	value := string(s)
	if !strings.HasPrefix(value, fixtureSlotPrefix) {
		return fmt.Errorf("experimentrun: unknown input slot %q", s)
	}
	id := strings.TrimPrefix(value, fixtureSlotPrefix)
	if err := experiment.ValidateID(id); err != nil {
		return fmt.Errorf("experimentrun: input slot %q fixture id: %w", s, err)
	}
	return nil
}

// InputBinding binds one closed definition slot to an explicit repository path.
type InputBinding struct {
	Slot   InputSlot `json:"slot"`
	ID     string    `json:"id"`
	Digest string    `json:"digest"`
	Path   string    `json:"path"`
}

func (b InputBinding) validate(field string) error {
	if err := b.Slot.Validate(); err != nil {
		return fmt.Errorf("experimentrun: %s: %w", field, err)
	}
	if err := experiment.ValidateID(b.ID); err != nil {
		return fmt.Errorf("experimentrun: %s.id: %w", field, err)
	}
	if err := experiment.ValidateDigest(b.Digest); err != nil {
		return fmt.Errorf("experimentrun: %s.digest: %w", field, err)
	}
	if err := experiment.ValidateRepoRelativePath(b.Path); err != nil {
		return fmt.Errorf("experimentrun: %s.path: %w", field, err)
	}
	if strings.HasPrefix(string(b.Slot), fixtureSlotPrefix) && b.Slot != FixtureInputSlot(b.ID) {
		return fmt.Errorf("experimentrun: %s fixture slot %q does not match id %q", field, b.Slot, b.ID)
	}
	return nil
}

// InputBindings is the complete typed binding document for one Start or Resume.
type InputBindings struct {
	Schema string         `json:"schema"`
	Inputs []InputBinding `json:"inputs"`
}

// Validate checks schema, explicit array presence, closed fields, ordering,
// uniqueness, and every entry's identity and repository-path grammar.
func (b InputBindings) Validate() error {
	if b.Schema != InputBindingSchema {
		return fmt.Errorf("experimentrun: input binding schema %q, want %q", b.Schema, InputBindingSchema)
	}
	if b.Inputs == nil {
		return fmt.Errorf("experimentrun: input bindings.inputs must be present and non-null")
	}
	seenPaths := make(map[string]bool, len(b.Inputs))
	var previous InputSlot
	for index, input := range b.Inputs {
		field := fmt.Sprintf("input bindings.inputs[%d]", index)
		if err := input.validate(field); err != nil {
			return err
		}
		if index > 0 && previous >= input.Slot {
			return fmt.Errorf("experimentrun: input binding slots must be lexically sorted without duplicates: %q then %q", previous, input.Slot)
		}
		if seenPaths[input.Path] {
			return fmt.Errorf("experimentrun: input binding path %q is duplicated", input.Path)
		}
		seenPaths[input.Path] = true
		previous = input.Slot
	}
	return nil
}

// Clone returns a deep copy with no shared input-array storage.
func (b InputBindings) Clone() InputBindings {
	return InputBindings{Schema: b.Schema, Inputs: append([]InputBinding(nil), b.Inputs...)}
}

// EncodeInputBindings returns the exact canonical binding-document bytes.
func EncodeInputBindings(bindings InputBindings) ([]byte, error) {
	if err := bindings.Validate(); err != nil {
		return nil, err
	}
	data, err := canonjson.Marshal(bindings)
	if err != nil {
		return nil, fmt.Errorf("experimentrun: encode input bindings: %w", err)
	}
	return data, nil
}

// DecodeInputBindings strict-decodes one byte-canonical binding document.
func DecodeInputBindings(data []byte) (InputBindings, error) {
	var bindings InputBindings
	if err := artifact.DecodeExactJSON(data, &bindings); err != nil {
		return InputBindings{}, fmt.Errorf("experimentrun: decode input bindings: %w", err)
	}
	canonical, err := EncodeInputBindings(bindings)
	if err != nil {
		return InputBindings{}, err
	}
	if !bytes.Equal(data, canonical) {
		return InputBindings{}, fmt.Errorf("experimentrun: input bindings are not canonical JSON")
	}
	return bindings.Clone(), nil
}

type boundInputResolver struct {
	inputs map[InputSlot]ResolvedInput
}

// NewBoundInputResolver proves exact total correspondence between bindings
// and one locked definition, then returns an operation-scoped read-only resolver.
func NewBoundInputResolver(def experiment.Definition, bindings InputBindings) (InputResolver, error) {
	if err := def.Validate(); err != nil {
		return nil, fmt.Errorf("experimentrun: bind inputs definition: %w", err)
	}
	locked, err := experiment.Locked(def)
	if err != nil {
		return nil, fmt.Errorf("experimentrun: bind inputs definition lock: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("experimentrun: bind inputs requires a locked definition")
	}
	if err := bindings.Validate(); err != nil {
		return nil, err
	}
	expected := make(map[InputSlot]experiment.ArtifactRef, len(def.Fixtures)+2)
	expected[InputSlotWorkload] = def.Workload
	expected[InputSlotContract] = def.Contract
	for _, fixture := range def.Fixtures {
		expected[FixtureInputSlot(fixture.ID)] = fixture
	}
	if len(bindings.Inputs) != len(expected) {
		return nil, fmt.Errorf("experimentrun: input binding count %d, want %d exact definition slots", len(bindings.Inputs), len(expected))
	}
	resolved := make(map[InputSlot]ResolvedInput, len(expected))
	for _, binding := range bindings.Inputs {
		ref, ok := expected[binding.Slot]
		if !ok {
			return nil, fmt.Errorf("experimentrun: input binding slot %q is absent from the locked definition", binding.Slot)
		}
		if binding.ID != ref.ID || binding.Digest != ref.Digest {
			return nil, fmt.Errorf("experimentrun: input binding slot %q identity {%q,%q}, want {%q,%q}", binding.Slot, binding.ID, binding.Digest, ref.ID, ref.Digest)
		}
		resolved[binding.Slot] = ResolvedInput{ID: binding.ID, Path: binding.Path, Digest: binding.Digest}
	}
	return &boundInputResolver{inputs: resolved}, nil
}

func (r *boundInputResolver) ResolveExperimentInput(ctx context.Context, _ string, request ResolveInputRequest) (ResolvedInput, error) {
	if ctx == nil {
		return ResolvedInput{}, fmt.Errorf("experimentrun: bound input resolver: nil context")
	}
	if r == nil {
		return ResolvedInput{}, fmt.Errorf("experimentrun: bound input resolver is nil")
	}
	if err := request.Validate(); err != nil {
		return ResolvedInput{}, err
	}
	resolved, ok := r.inputs[request.Slot]
	if !ok {
		return ResolvedInput{}, fmt.Errorf("experimentrun: bound input slot %q is unavailable", request.Slot)
	}
	if resolved.ID != request.Ref.ID || resolved.Digest != request.Ref.Digest {
		return ResolvedInput{}, fmt.Errorf("experimentrun: bound input slot %q does not match requested locked reference", request.Slot)
	}
	return resolved, nil
}
