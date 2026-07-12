package artifact

import (
	"fmt"
	"regexp"
)

// SpecClass distinguishes the three spec classes (02 §Kind registry:
// "Spec classes"). ClassStory is R4-I-9's new class, superseding v0's
// story-grained feature class.
type SpecClass string

const (
	ClassFeature   SpecClass = "feature"
	ClassStory     SpecClass = "story"
	ClassComponent SpecClass = "component"
)

var validSpecClasses = map[SpecClass]bool{ClassFeature: true, ClassStory: true, ClassComponent: true}

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

// AcceptanceCriterion is one entry in a feature or story spec's
// `acceptance_criteria:` block (02 §feature-spec frontmatter additions,
// §Object model). Anchor is a round-four addition (02 §Object model: "every
// object ... carries an anchor") left `omitempty`/optional at the Go level
// — unlike Constraint and Decision, which are wholly new blocks with no v0
// usage, AcceptanceCriterion already existed pre-round-four and v0's
// grandfathered, frozen feature-spec fixtures never populated an anchor at
// all (A8: grandfathered artifacts are never rewritten). Requiring Anchor
// unconditionally here would make those frozen fixtures fail to decode,
// which the store's immutability rule (VL-010) forbids fixing by editing
// them. See the phase report for the full judgment-call writeup.
type AcceptanceCriterion struct {
	ID       string         `yaml:"id"`
	Text     string         `yaml:"text"`
	Evidence []EvidenceKind `yaml:"evidence"`
	Anchor   string         `yaml:"anchor,omitempty"`
}

// Validate checks ID shape, Text is present, and Evidence lists at least
// one known kind (03 §Declarations: "each AC lists the evidence kinds it
// expects"). Anchor, when present, is checked for resolution separately —
// see SpecFrontmatter.ResolveObjectAnchors — since that requires the
// document body, which this decode-only validation does not have.
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

// SpecFrontmatter is the frontmatter schema for kind "spec", covering the
// feature, story, and component classes (02 §Kind registry, §feature-spec
// frontmatter additions, §Story-spec frontmatter additions). Component
// specs must leave every feature/story-only field empty; feature specs
// must set AcceptanceCriteria; story specs must set Problem, Outcome,
// Story, and either ≥1 implements edge or (spike) ≥1 resolves edge.
//
// Problem and Outcome are *Attribute rather than a required, non-pointer
// field for the same grandfathering reason as AcceptanceCriterion.Anchor:
// they are round-four-required on feature specs per 02 §Object model, but
// v0's frozen feature-spec fixtures predate the field entirely and must
// still decode (A8). ClassStory has no such legacy and requires them
// unconditionally in validateStory — see that function.
type SpecFrontmatter struct {
	Base               `yaml:",inline"`
	Class              SpecClass             `yaml:"class"`
	Status             Status                `yaml:"status"`
	Story              string                `yaml:"story,omitempty"`
	Spike              bool                  `yaml:"spike,omitempty"`
	Problem            *Attribute            `yaml:"problem,omitempty"`
	Outcome            *Attribute            `yaml:"outcome,omitempty"`
	Impacts            []string              `yaml:"impacts,omitempty"`
	Context            []string              `yaml:"context,omitempty"`
	Declares           *Declares             `yaml:"declares,omitempty"`
	AcceptanceCriteria []AcceptanceCriterion `yaml:"acceptance_criteria,omitempty"`
	Constraints        []Constraint          `yaml:"constraints,omitempty"`
	Decisions          []Decision            `yaml:"decisions,omitempty"`
	OpenQuestions      []OpenQuestion        `yaml:"open_questions,omitempty"`
	Stubs              []Stub                `yaml:"stubs,omitempty"`
	Supersession       *Supersession         `yaml:"supersession,omitempty"`
	Dispositions       []Disposition         `yaml:"dispositions,omitempty"`
}

// DeclaredObjectIDs is the set of every frontmatter-declared object id a
// spec carries — acceptance criteria, constraints, decisions, and open
// questions (02 §Object model) — the resolution target for a fragment ref
// (§Identity and references) and for a forge review comment's
// `[vd:<object-id>]` token (02 §Record schemas' comment-token grammar).
// Exported so both internal/lint's VL-003 fragment resolution and any
// other consumer resolving an object id against a spec's declared objects
// (cmd/verdi/gate_threads.go, internal/mcpserve) share one definition
// rather than each re-deriving it (CLAUDE.md: shared code lives in a
// shared internal/ package).
func DeclaredObjectIDs(spec *SpecFrontmatter) map[string]bool {
	ids := make(map[string]bool, len(spec.AcceptanceCriteria)+len(spec.Constraints)+len(spec.Decisions)+len(spec.OpenQuestions))
	for _, ac := range spec.AcceptanceCriteria {
		ids[ac.ID] = true
	}
	for _, c := range spec.Constraints {
		ids[c.ID] = true
	}
	for _, dc := range spec.Decisions {
		ids[dc.ID] = true
	}
	for _, q := range spec.OpenQuestions {
		ids[q.ID] = true
	}
	return ids
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
	case ClassStory:
		return fm.validateStory()
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
	if fm.Spike {
		return fmt.Errorf("artifact: feature spec must not carry spike: true (spike is a story-class variant, 02 §Kind registry)")
	}
	// Story is OPTIONAL on the feature class as of round four (R4-I-2: "an
	// epic/objective tracker ref, not a per-story binding" — moved from a
	// required scalar to the story class). Validated only when present.
	if fm.Story != "" && !storyRefRe.MatchString(fm.Story) {
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
	if err := validateObjectBlocks(fm.Problem, fm.Outcome, fm.Constraints, fm.Decisions, fm.OpenQuestions); err != nil {
		return err
	}
	if err := validateDispositions(fm.Dispositions); err != nil {
		return err
	}
	seenStub := make(map[string]bool, len(fm.Stubs))
	for i, s := range fm.Stubs {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("artifact: stubs[%d]: %w", i, err)
		}
		if seenStub[s.Slug] {
			return fmt.Errorf("artifact: stub slug %q is duplicated", s.Slug)
		}
		seenStub[s.Slug] = true
	}
	if fm.Supersession != nil {
		if err := fm.Supersession.Validate(); err != nil {
			return fmt.Errorf("artifact: supersession: %w", err)
		}
	}

	frozenRequired := fm.Status == "accepted-pending-build" || fm.Status == "closed" || fm.Status == "superseded"
	return requireFrozen(fm.Frozen, frozenRequired, "feature spec", string(fm.Status))
}

// validateStory validates the story class (02 §Kind registry "story
// (NEW)"), including its spike variant. Unlike the feature class, story is
// wholly new as of round four — no v0 fixture ever carried class: story —
// so Problem, Outcome, and Story are all required unconditionally here,
// with no grandfathering tension.
func (fm SpecFrontmatter) validateStory() error {
	if !specFeatureStatuses[fm.Status] {
		return fmt.Errorf("artifact: story spec status %q is not a known status (same lifecycle as feature, 02 §Kind registry)", fm.Status)
	}
	if fm.Problem == nil {
		return fmt.Errorf("artifact: story spec requires a problem attribute (02 §Object model)")
	}
	if err := fm.Problem.Validate(); err != nil {
		return fmt.Errorf("artifact: problem: %w", err)
	}
	if fm.Outcome == nil {
		return fmt.Errorf("artifact: story spec requires an outcome attribute (02 §Object model)")
	}
	if err := fm.Outcome.Validate(); err != nil {
		return fmt.Errorf("artifact: outcome: %w", err)
	}
	if fm.Story == "" {
		return fmt.Errorf("artifact: story spec requires a story: scheme:key tracker ref (R4-I-2, VL-005)")
	}
	if !storyRefRe.MatchString(fm.Story) {
		return fmt.Errorf("artifact: story spec story %q must be scheme:key form (e.g. jira:LOAN-1482)", fm.Story)
	}
	// story class carries no feature-only fields.
	if len(fm.Impacts) != 0 || len(fm.Context) != 0 || fm.Declares != nil ||
		len(fm.Stubs) != 0 || fm.Supersession != nil || len(fm.Dispositions) != 0 {
		return fmt.Errorf("artifact: story spec must not carry feature-only fields (impacts/context/declares/stubs/supersession/dispositions)")
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
	if err := validateObjectBlocks(nil, nil, fm.Constraints, fm.Decisions, fm.OpenQuestions); err != nil {
		return err
	}

	var implementsCount, resolvesCount int
	for i, l := range fm.Links {
		switch l.Type {
		case LinkImplements:
			implementsCount++
			if err := requireFragment(l); err != nil {
				return fmt.Errorf("artifact: links[%d]: %w", i, err)
			}
		case LinkResolves:
			resolvesCount++
			if err := requireFragment(l); err != nil {
				return fmt.Errorf("artifact: links[%d]: %w", i, err)
			}
		}
	}
	if fm.Spike {
		if implementsCount != 0 {
			return fmt.Errorf("artifact: spike story must carry no implements edges (02 §Kind registry: spike variant)")
		}
		if resolvesCount == 0 {
			return fmt.Errorf("artifact: spike story requires >=1 resolves edge to an open-question fragment (02 §Kind registry: spike variant)")
		}
	} else {
		if implementsCount == 0 {
			return fmt.Errorf("artifact: story spec requires >=1 implements edge to a feature AC fragment (02 §Kind registry)")
		}
	}

	frozenRequired := fm.Status == "accepted-pending-build" || fm.Status == "closed" || fm.Status == "superseded"
	return requireFrozen(fm.Frozen, frozenRequired, "story spec", string(fm.Status))
}

// requireFragment checks l's ref carries an object-id fragment — the
// implements/resolves edges a story or spike declares must target a
// feature AC / open-question fragment, never a whole spec (02 §Kind
// registry, §Identity and references).
func requireFragment(l Link) error {
	ref, err := ParseRef(l.Ref)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	if !ref.Fragment() {
		return fmt.Errorf("artifact: %s edge %q must target an object fragment (<kind>/<name>#<object-id>)", l.Type, l.Ref)
	}
	return nil
}

func (fm SpecFrontmatter) validateComponent() error {
	if !specComponentStatuses[fm.Status] {
		return fmt.Errorf("artifact: component spec status %q is not a known status", fm.Status)
	}
	if fm.Story != "" {
		return fmt.Errorf("artifact: component spec must not carry a story (02: 'No story, no ACs')")
	}
	if fm.Spike || fm.Problem != nil || fm.Outcome != nil ||
		len(fm.Impacts) != 0 || len(fm.Context) != 0 || fm.Declares != nil ||
		len(fm.AcceptanceCriteria) != 0 || len(fm.Constraints) != 0 || len(fm.Decisions) != 0 ||
		len(fm.OpenQuestions) != 0 ||
		len(fm.Stubs) != 0 || fm.Supersession != nil || len(fm.Dispositions) != 0 {
		return fmt.Errorf("artifact: component spec must not carry feature/story-only fields (02: 'no object model')")
	}
	// component specs are authored-living and never frozen (01 §Temporal
	// classes); superseded component specs stay in specs/active/ rather
	// than moving/freezing.
	return requireFrozen(fm.Frozen, false, "component spec", string(fm.Status))
}

// validateObjectBlocks validates a spec's optional attribute and
// object-model blocks together: Problem/Outcome (if present, individually
// valid — presence itself is enforced by the caller per class), and every
// Constraint/Decision entry, with ids unique within their own block.
func validateObjectBlocks(problem, outcome *Attribute, constraints []Constraint, decisions []Decision, openQuestions []OpenQuestion) error {
	if problem != nil {
		if err := problem.Validate(); err != nil {
			return fmt.Errorf("artifact: problem: %w", err)
		}
	}
	if outcome != nil {
		if err := outcome.Validate(); err != nil {
			return fmt.Errorf("artifact: outcome: %w", err)
		}
	}
	seenCo := make(map[string]bool, len(constraints))
	for i, c := range constraints {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("artifact: constraints[%d]: %w", i, err)
		}
		if seenCo[c.ID] {
			return fmt.Errorf("artifact: constraint id %q is duplicated", c.ID)
		}
		seenCo[c.ID] = true
	}
	seenDc := make(map[string]bool, len(decisions))
	for i, d := range decisions {
		if err := d.Validate(); err != nil {
			return fmt.Errorf("artifact: decisions[%d]: %w", i, err)
		}
		if seenDc[d.ID] {
			return fmt.Errorf("artifact: decision id %q is duplicated", d.ID)
		}
		seenDc[d.ID] = true
	}
	seenOQ := make(map[string]bool, len(openQuestions))
	for i, q := range openQuestions {
		if err := q.Validate(); err != nil {
			return fmt.Errorf("artifact: open_questions[%d]: %w", i, err)
		}
		if seenOQ[q.ID] {
			return fmt.Errorf("artifact: open question id %q is duplicated", q.ID)
		}
		seenOQ[q.ID] = true
	}
	return nil
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
