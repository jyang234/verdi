package lint

import (
	"fmt"

	"github.com/OWNER/verdi/internal/artifact"
)

// vl006 enforces "every AC declares ≥1 expected evidence kind (activation
// lint)" (02 §Lint rules), reading the raw decoded AC list directly (see
// doc.go's design note: this is why the overlay's empty-evidence AC
// decodes successfully under VL-001 yet still fires here) — plus, per
// R4-I-15 (ratified at the V1-P1 phase review; enforcement assigned to
// this phase), the broader round-four activation-completeness
// requirement: `problem`/`outcome` and every declared object's `anchor:`
// are decode-optional (internal/artifact's SpecFrontmatter must decode
// both grandfathered v0 specs, which never populated them, and
// round-four specs, which must carry them) but lint-REQUIRED once a spec
// is "new-class" — see isNewClassSpec's doc comment for the discriminator
// this phase settled.
//
// Judgment call (recorded here and in the phase report): the brief left
// the rule-id choice open between extending VL-006's row (already the
// activation-completeness home — 02 literally labels it "(activation
// lint)") or VL-001's decode-adjacent scope. VL-006 was chosen: this is
// exactly the same shape of check ("is this spec complete enough to
// activate?"), not a decode-strictness concern — VL-001 is defined,
// deliberately narrowly, as exactly artifact.DecodeStrict succeeding or
// failing (doc.go), and the requiredness split is semantic, not
// syntactic. No new VL number is minted, per the brief's steer.
type vl006 struct{}

func (vl006) ID() string { return "VL-006" }

func (r vl006) Check(in *RunInput) []Finding {
	var findings []Finding
	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Spec == nil {
			continue
		}
		for _, ac := range d.Spec.AcceptanceCriteria {
			if len(ac.Evidence) == 0 {
				findings = append(findings, Finding{Rule: "VL-006", Path: d.RelPath, Message: fmt.Sprintf("acceptance criterion %s declares no expected evidence kind", ac.ID)})
			}
		}
		if isNewClassSpec(d.Spec) {
			findings = append(findings, r.checkRequiredness(d)...)
		}
	}
	return findings
}

// checkRequiredness is R4-I-15's enforcement: for a new-class spec (see
// isNewClassSpec), problem and outcome must both be present, and every
// declared object (acceptance criterion, constraint, decision, open
// question) must carry a non-empty anchor. It then additionally resolves
// every present anchor against the document body via
// SpecFrontmatter.ResolveObjectAnchors — 02 §Object model's general
// exact-match anchor-resolution rule, which no rule in this engine called
// before this phase (object.go's method existed since V1-P1 but had no
// caller here); folding it into the same requiredness pass this phase
// already added is the smallest way to make that general rule live,
// rather than leaving it permanently unreachable from lint.
func (vl006) checkRequiredness(d *Document) []Finding {
	var findings []Finding
	spec := d.Spec

	if spec.Problem == nil {
		findings = append(findings, Finding{Rule: "VL-006", Path: d.RelPath, Message: "new-class spec has no problem attribute (R4-I-15: required for round-four specs, 02 §Object model)"})
	}
	if spec.Outcome == nil {
		findings = append(findings, Finding{Rule: "VL-006", Path: d.RelPath, Message: "new-class spec has no outcome attribute (R4-I-15: required for round-four specs, 02 §Object model)"})
	}
	for _, ac := range spec.AcceptanceCriteria {
		if ac.Anchor == "" {
			findings = append(findings, Finding{Rule: "VL-006", Path: d.RelPath, Message: fmt.Sprintf("new-class spec's acceptance criterion %s has no anchor (R4-I-15: required for round-four specs, 02 §Object model)", ac.ID)})
		}
	}

	if err := spec.ResolveObjectAnchors([]byte(d.Body)); err != nil {
		findings = append(findings, Finding{Rule: "VL-006", Path: d.RelPath, Message: err.Error()})
	}

	return findings
}

// isNewClassSpec is this phase's judgment call settling R4-I-15's "exact
// new-vs-grandfathered discriminator" (PLAN-V1.md §7 R4-I-15: "the exact
// new-vs-grandfathered discriminator is settled at V1-P2 review"),
// adopting the brief's recommended rule: the story class is always new (no
// v0 story class ever existed at all — R4-I-9's "superseded: v0's
// story-grained feature class"); a feature spec is new iff it carries ANY
// round-four surface field — problem, outcome, stubs, supersession, or a
// constraints/decisions/open_questions object block. A v0 grandfathered
// feature spec (e.g. testdata/corpus's stale-decline, new-feature-x,
// loan-refi-2023) carries none of these.
//
// acceptance_criteria is deliberately excluded from "any object block"
// despite 02 §Object model naming it as one of the object-model blocks
// alongside constraints/decisions/open_questions: ACs predate round four —
// every v0 feature spec already required at least one (see
// object.go's own AcceptanceCriterion.Anchor doc comment: "v0's
// grandfathered, frozen feature-spec fixtures never populated an anchor at
// all") — so an AC block's mere presence never discriminates round-four
// surface from v0. Including it would make every valid feature spec "new"
// unconditionally (a feature spec without any ACs fails decode outright),
// defeating grandfathering entirely. constraints/decisions/open_questions,
// by contrast, are wholly round-four blocks no v0 spec ever declared
// (object.go: "a wholly round-four block (no v0 spec ever declared one)").
func isNewClassSpec(spec *artifact.SpecFrontmatter) bool {
	if spec.Class == artifact.ClassStory {
		return true
	}
	if spec.Class != artifact.ClassFeature {
		return false // component: no object model at all (02 §Kind registry)
	}
	return spec.Problem != nil ||
		spec.Outcome != nil ||
		len(spec.Stubs) != 0 ||
		spec.Supersession != nil ||
		len(spec.Constraints) != 0 ||
		len(spec.Decisions) != 0 ||
		len(spec.OpenQuestions) != 0
}
