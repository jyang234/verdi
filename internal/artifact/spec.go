package artifact

import (
	"fmt"
	"regexp"
)

// SpecClass distinguishes the two spec classes (02 §Kind registry:
// "Spec classes").
type SpecClass string

const (
	ClassFeature   SpecClass = "feature"
	ClassComponent SpecClass = "component"
)

var validSpecClasses = map[SpecClass]bool{ClassFeature: true, ClassComponent: true}

// EvidenceKind is one of the four evidence kinds an AC can expect
// (03 §Evidence kinds), reused by acceptance criteria's `evidence:` list
// and by evidence records' `kind` field (evidence.go).
type EvidenceKind string

const (
	EvidenceStatic      EvidenceKind = "static"
	EvidenceBehavioral  EvidenceKind = "behavioral"
	EvidenceRuntime     EvidenceKind = "runtime"
	EvidenceAttestation EvidenceKind = "attestation"
)

var validEvidenceKinds = map[EvidenceKind]bool{
	EvidenceStatic:      true,
	EvidenceBehavioral:  true,
	EvidenceRuntime:     true,
	EvidenceAttestation: true,
}

var acIDRe = regexp.MustCompile(`^ac-[a-z0-9]+(?:-[a-z0-9]+)*$`)

// AcceptanceCriterion is one entry in a feature spec's
// `acceptance_criteria:` block (02 §feature-spec frontmatter additions).
type AcceptanceCriterion struct {
	ID       string         `yaml:"id"`
	Text     string         `yaml:"text"`
	Evidence []EvidenceKind `yaml:"evidence"`
}

// Validate checks ID shape, Text is present, and Evidence lists at least
// one known kind (03 §Declarations: "each AC lists the evidence kinds it
// expects").
func (ac AcceptanceCriterion) Validate() error {
	if !acIDRe.MatchString(ac.ID) {
		return fmt.Errorf("artifact: acceptance criterion id %q must look like ac-<slug>", ac.ID)
	}
	if ac.Text == "" {
		return fmt.Errorf("artifact: acceptance criterion %s has no text", ac.ID)
	}
	if len(ac.Evidence) == 0 {
		return fmt.Errorf("artifact: acceptance criterion %s declares no expected evidence kind", ac.ID)
	}
	for _, k := range ac.Evidence {
		if !validEvidenceKinds[k] {
			return fmt.Errorf("artifact: acceptance criterion %s: unknown evidence kind %q", ac.ID, k)
		}
	}
	return nil
}

// Boundary is one entry in a feature spec's `declares.boundaries:` block
// (02 §feature-spec frontmatter additions).
type Boundary struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
	Via  string `yaml:"via"`
}

// Validate checks From, To, and Via are all present.
func (b Boundary) Validate() error {
	if b.From == "" || b.To == "" || b.Via == "" {
		return fmt.Errorf("artifact: declared boundary %+v must set from, to, and via", b)
	}
	return nil
}

// Declares is a feature spec's `declares:` block: intended boundaries the
// alignment report's computed section later diffs against reality.
type Declares struct {
	Boundaries []Boundary `yaml:"boundaries,omitempty"`
}

// DispositionValue is the legal value of a disposition entry
// (02 §feature-spec frontmatter additions; I-5).
type DispositionValue string

const (
	DispositionIncorporated DispositionValue = "incorporated"
	DispositionContradicted DispositionValue = "contradicted"
	DispositionOpenQuestion DispositionValue = "open-question"
)

var validDispositionValues = map[DispositionValue]bool{
	DispositionIncorporated: true,
	DispositionContradicted: true,
	DispositionOpenQuestion: true,
}

// annotationIDRe matches the "a-<ULID>" shape ratified by I-11: a literal
// "a-" prefix followed by a 26-character Crockford base32 ULID (uppercase
// alphanumerics, excluding I, L, O, U).
var annotationIDRe = regexp.MustCompile(`^a-[0-9A-HJKMNP-TV-Z]{26}$`)

// Disposition is one entry in a feature spec's `dispositions:` block, the
// commit-to-design ritual's durable output (I-5, hardened per plan
// review): every sticky in the frozen board.json lands here.
type Disposition struct {
	Sticky      string           `yaml:"sticky"`
	Disposition DispositionValue `yaml:"disposition"`
	Where       string           `yaml:"where,omitempty"`
	Note        string           `yaml:"note,omitempty"`
}

// Validate checks Sticky looks like an annotation id, Disposition is a
// known value, and the per-value required field is present: `incorporated`
// requires Where (a resolving heading anchor — VL-014 verifies resolution
// in phase 4; this package only checks the field is present and looks like
// an anchor), `contradicted` requires Note.
func (d Disposition) Validate() error {
	if !annotationIDRe.MatchString(d.Sticky) {
		return fmt.Errorf("artifact: disposition sticky %q is not a valid annotation id (a-<ULID>, I-11)", d.Sticky)
	}
	if !validDispositionValues[d.Disposition] {
		return fmt.Errorf("artifact: disposition %q for sticky %s is not a known value", d.Disposition, d.Sticky)
	}
	switch d.Disposition {
	case DispositionIncorporated:
		if d.Where == "" {
			return fmt.Errorf("artifact: disposition for sticky %s is incorporated but has no where anchor (I-5)", d.Sticky)
		}
	case DispositionContradicted:
		if d.Note == "" {
			return fmt.Errorf("artifact: disposition for sticky %s is contradicted but has no note (I-5)", d.Sticky)
		}
	}
	return nil
}

// SpecFrontmatter is the frontmatter schema for kind "spec", covering both
// the feature and component classes (02 §Kind registry, §feature-spec
// frontmatter additions). Component specs must leave every feature-only
// field empty; feature specs must set Story and AcceptanceCriteria.
type SpecFrontmatter struct {
	Base               `yaml:",inline"`
	Class              SpecClass             `yaml:"class"`
	Status             Status                `yaml:"status"`
	Story              string                `yaml:"story,omitempty"`
	Impacts            []string              `yaml:"impacts,omitempty"`
	Context            []string              `yaml:"context,omitempty"`
	Declares           *Declares             `yaml:"declares,omitempty"`
	AcceptanceCriteria []AcceptanceCriterion `yaml:"acceptance_criteria,omitempty"`
	Dispositions       []Disposition         `yaml:"dispositions,omitempty"`
}

// DecodeSpec strict-decodes and validates spec frontmatter (either class).
func DecodeSpec(data []byte) (*SpecFrontmatter, error) {
	var fm SpecFrontmatter
	if err := DecodeStrict(data, &fm); err != nil {
		return nil, err
	}
	if err := fm.Validate(); err != nil {
		return nil, err
	}
	return &fm, nil
}

// Validate checks the common fields, the class enum, the status enum
// (class-scoped), class-specific field requirements, and Frozen
// requiredness (feature specs freeze at acceptance: required once
// accepted-pending-build or closed; component specs are authored-living
// and never frozen).
func (fm SpecFrontmatter) Validate() error {
	if err := fm.validateBase(KindSpec); err != nil {
		return err
	}
	if !validSpecClasses[fm.Class] {
		return fmt.Errorf("artifact: spec class %q is not a known class", fm.Class)
	}

	switch fm.Class {
	case ClassFeature:
		return fm.validateFeature()
	case ClassComponent:
		return fm.validateComponent()
	default:
		return fmt.Errorf("artifact: unreachable: unhandled spec class %q", fm.Class)
	}
}

func (fm SpecFrontmatter) validateFeature() error {
	if !specFeatureStatuses[fm.Status] {
		return fmt.Errorf("artifact: feature spec status %q is not a known status", fm.Status)
	}
	if !storyRefRe.MatchString(fm.Story) {
		return fmt.Errorf("artifact: feature spec story %q must be scheme:key form (e.g. jira:LOAN-1482, VL-005)", fm.Story)
	}
	if len(fm.AcceptanceCriteria) == 0 {
		return fmt.Errorf("artifact: feature spec must declare at least one acceptance criterion")
	}
	seenAC := make(map[string]bool, len(fm.AcceptanceCriteria))
	for i, ac := range fm.AcceptanceCriteria {
		if err := ac.Validate(); err != nil {
			return fmt.Errorf("artifact: acceptance_criteria[%d]: %w", i, err)
		}
		if seenAC[ac.ID] {
			return fmt.Errorf("artifact: acceptance criterion id %q is duplicated", ac.ID)
		}
		seenAC[ac.ID] = true
	}
	for i, imp := range fm.Impacts {
		if imp == "" {
			return fmt.Errorf("artifact: impacts[%d] is empty", i)
		}
	}
	for i, ctx := range fm.Context {
		if _, err := ParsePinnedRef(ctx); err != nil {
			return fmt.Errorf("artifact: context[%d]: %w", i, err)
		}
	}
	if fm.Declares != nil {
		for i, b := range fm.Declares.Boundaries {
			if err := b.Validate(); err != nil {
				return fmt.Errorf("artifact: declares.boundaries[%d]: %w", i, err)
			}
		}
	}
	if err := validateDispositions(fm.Dispositions); err != nil {
		return err
	}

	frozenRequired := fm.Status == "accepted-pending-build" || fm.Status == "closed"
	return requireFrozen(fm.Frozen, frozenRequired, "feature spec", string(fm.Status))
}

func (fm SpecFrontmatter) validateComponent() error {
	if !specComponentStatuses[fm.Status] {
		return fmt.Errorf("artifact: component spec status %q is not a known status", fm.Status)
	}
	if fm.Story != "" {
		return fmt.Errorf("artifact: component spec must not carry a story (02: 'No story, no ACs')")
	}
	if len(fm.Impacts) != 0 || len(fm.Context) != 0 || fm.Declares != nil ||
		len(fm.AcceptanceCriteria) != 0 || len(fm.Dispositions) != 0 {
		return fmt.Errorf("artifact: component spec must not carry feature-only fields (impacts/context/declares/acceptance_criteria/dispositions)")
	}
	// component specs are authored-living and never frozen (01 §Temporal
	// classes); superseded component specs stay in specs/active/ rather
	// than moving/freezing.
	return requireFrozen(fm.Frozen, false, "component spec", string(fm.Status))
}

// validateDispositions checks each disposition individually and enforces
// I-5's bidirectional completeness at the syntactic level this package can
// see: no two entries may name the same sticky (a real duplicate is never
// legitimate, whatever VL-014's board-side cross-check later finds).
// Bidirectional completeness against the board's actual sticky set is
// VL-014's job (phase 4) — this package cannot see board.json from a bare
// frontmatter decode.
func validateDispositions(ds []Disposition) error {
	seen := make(map[string]bool, len(ds))
	for i, d := range ds {
		if err := d.Validate(); err != nil {
			return fmt.Errorf("artifact: dispositions[%d]: %w", i, err)
		}
		if seen[d.Sticky] {
			return fmt.Errorf("artifact: dispositions: sticky %s is dispositioned more than once", d.Sticky)
		}
		seen[d.Sticky] = true
	}
	return nil
}
