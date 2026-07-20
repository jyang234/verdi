package lint

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/model"
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
				// Object-anchored (badge-computes dc-3's "missing evidence
				// kind" bucket): this finding names exactly the AC card that
				// declares no evidence.
				findings = append(findings, Finding{Rule: "VL-006", Path: d.RelPath, Message: fmt.Sprintf("acceptance criterion %s declares no expected evidence kind", ac.ID), Locus: ObjectLocus(ac.ID)})
			}
		}
		if isNewClassSpec(d.Spec) {
			// checkRequiredness is the "missing required attribute"
			// family (problem/outcome/anchor requiredness, badge-computes
			// dc-3) — none of its findings name a single object the way
			// the evidence-kind check above does (an anchor-missing
			// finding is about the SPEC's round-four completeness, not a
			// defect the author fixes only on that one card), so the
			// whole family is spec-level.
			findings = append(findings, locusAll(r.checkRequiredness(d), SpecLocus())...)
			// checkFeatureACAttestation is L-M14 remedy 1 (03 §Declarations
			// and binding / §The feature fold's outcome floor) — object-
			// anchored to the AC card, same shape as the evidence-kind
			// check above, so it sits beside it rather than inside the
			// spec-level requiredness family.
			findings = append(findings, r.checkFeatureACAttestation(d, in.Model)...)
		}
		findings = append(findings, r.checkStubACs(d)...)
		findings = append(findings, r.checkStubResolves(d, in.Model)...)
	}
	return findings
}

// checkStubACs enforces that every entry in a stub's acceptance_criteria
// names a declared acceptance criterion of the SAME spec. Stub.Validate
// only checks each entry is a syntactically well-formed ac-<slug> id
// (object.go) — a stub naming a nonexistent ac-99 decodes and validates
// clean, leaving a dangling scoping reference. This is the syntactic
// stub-surface check the house steer (vl006.go's doc comment) keeps under
// VL-006 rather than minting a new VL number: same "is this spec's own
// declared surface internally consistent?" shape as the requiredness and
// anchor-resolution checks already folded here, not a decode-strictness
// concern (VL-001's narrow scope). Runs for every non-grandfathered,
// cleanly-decoded spec (grandfathered v0 specs never carried stubs; a
// decode-failed doc has no Spec) — the same guard the Check loop already
// applies.
func (vl006) checkStubACs(d *Document) []Finding {
	var findings []Finding
	declaredAC := make(map[string]bool, len(d.Spec.AcceptanceCriteria))
	for _, ac := range d.Spec.AcceptanceCriteria {
		declaredAC[ac.ID] = true
	}
	for _, st := range d.Spec.Stubs {
		for _, acID := range st.AcceptanceCriteria {
			if !declaredAC[acID] {
				// Object-anchored to the STUB's own card (badge-computes
				// dc-3's "dangling stub ref" bucket): the dangling claim is
				// a property of the stub (a rendered ZoneStub card,
				// internal/workbench/projection.go), not of the
				// nonexistent target it names.
				findings = append(findings, Finding{Rule: "VL-006", Path: d.RelPath, Message: fmt.Sprintf("stub %q names acceptance_criteria %s, which is not a declared acceptance criterion of this spec", st.Slug, acID), Locus: ObjectLocus("stub:" + st.Slug)})
			}
		}
	}
	return findings
}

// checkStubResolves is checkStubACs' round-5.4 sibling (02 §Kind
// registry's DC-4: "VL-006 grows its sibling check (`resolves` must name
// declared open questions of the same spec) inside the rule that already
// validates stub `acceptance_criteria`"): every spike stub's `resolves`
// entry must name a declared open question of the same spec.
// Stub.Validate only checks each entry is a syntactically well-formed
// oq-<slug> id (object.go) — a spike stub naming a nonexistent oq-99
// decodes and validates clean, leaving a dangling attribution reference.
// Same guard as checkStubACs: runs for every non-grandfathered, cleanly-
// decoded spec.
func (vl006) checkStubResolves(d *Document, mdl *model.Model) []Finding {
	var findings []Finding
	declaredOQ := make(map[string]bool, len(d.Spec.OpenQuestions))
	for _, q := range d.Spec.OpenQuestions {
		declaredOQ[q.ID] = true
	}
	for _, st := range d.Spec.Stubs {
		for _, oqID := range st.Resolves {
			if !declaredOQ[oqID] {
				// Same object-anchor as checkStubACs: the stub's own card.
				// The variant word is display and resolves (L-M13a(6));
				// the slug and oq id echoes are identity.
				findings = append(findings, Finding{Rule: "VL-006", Path: d.RelPath, Message: fmt.Sprintf("%s stub %q names resolves %s, which is not a declared open question of this spec", mdl.DisplayClass("spike"), st.Slug, oqID), Locus: ObjectLocus("stub:" + st.Slug)})
			}
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

// checkFeatureACAttestation is L-M14 remedy 1: the evidence-model spec's
// binding text (03 §Declarations and binding) is explicit that "a feature
// AC's expected-kinds list is checked too, and MUST include `attestation`
// — the outcome floor (§The feature fold). A feature AC that omits it
// fails VL-006 and cannot activate" — a check L-M14's own recon found was
// never implemented (spec/operating-model activated with pure behavioral/
// static ACs, lint-clean). Scoped to feature-class specs only: a story AC
// carries no outcome-floor concept (03's text is explicit the floor is a
// FEATURE-level, two-level-model addition — "ratification round four").
//
// Grandfathered by ACCEPTANCE, not merely by archive location: this rule
// did not exist when every currently-accepted feature spec predating this
// fix was authored, and 03 §Declarations is explicit evidence kinds are
// declared ONCE, at authoring — amending them after acceptance is the
// supersession ladder's job, never a lint rule's (the identical reasoning
// L-M14's own adjudication already applied to spec/operating-model: "the
// feature closes against its ACs AS FROZEN ... amending evidence kinds on
// a frozen spec would require full supersession, disproportionate"). That
// reasoning holds for every already-frozen feature spec, not only the
// archived ones — and an archive-only exemption provably is NOT the
// cleanest scope here: examples/showcase's own committed corpus (this
// engine's test fixture base) carries multiple currently-ACTIVE, already-
// accepted feature specs with the identical gap (escrow-autopay/ac-2,
// loan-workflow(-v2)/ac-2, stale-decline — none of which declare
// attestation on every AC), so an archive-only exemption would make this
// very engine's own baseline lint-dirty. d.Spec.Frozen != nil is the
// correct, deterministic discriminator (never a wall-clock or commit-SHA
// threshold, CLAUDE.md): it is set exactly at `verdi accept` time and
// preserved verbatim through closure (cmd/verdi/close.go's
// flipSpecStatusToClosed keeps the frozen: stamp byte-for-byte), so it
// holds for every accepted spec whether still active or already archived
// — "specs/archive/ exempt" is the special case of this broader rule, not
// a second mechanism. The check therefore only ever gates a feature spec
// still in DRAFT — the literal "fails VL-006 and cannot activate" moment
// — never relitigates a spec's declared evidence kinds after acceptance.
func (vl006) checkFeatureACAttestation(d *Document, mdl *model.Model) []Finding {
	if d.Spec.Class != artifact.ClassFeature || d.Spec.Frozen != nil {
		return nil
	}
	var findings []Finding
	for _, ac := range d.Spec.AcceptanceCriteria {
		if hasAttestationKind(ac.Evidence) {
			continue
		}
		// Object-anchored (same shape as the empty-evidence-kind check
		// above): this finding names exactly the AC card missing the
		// outcome floor's minimum satisfying kind. The leading class word
		// is display and routes through the model (L-M13a(6): best-effort
		// in.Model, nil-safe to the bare id — the spike-stub sibling's
		// exact pattern); the "§The feature fold" spec-section title and
		// the ac id stay verbatim (a citation and an identity id).
		findings = append(findings, Finding{Rule: "VL-006", Path: d.RelPath, Message: fmt.Sprintf("%s acceptance criterion %s does not declare attestation among its expected evidence kinds (03 §The feature fold: the outcome floor requires attestation at minimum)", mdl.DisplayClass("feature"), ac.ID), Locus: ObjectLocus(ac.ID)})
	}
	return findings
}

// hasAttestationKind reports whether kinds declares the attestation kind
// — the outcome floor's minimum satisfying evidence kind (03 §The feature
// fold).
func hasAttestationKind(kinds []artifact.EvidenceKind) bool {
	for _, k := range kinds {
		if k == artifact.EvidenceAttestation {
			return true
		}
	}
	return false
}

// isNewClassSpec is this phase's judgment call settling R4-I-15's "exact
// new-vs-grandfathered discriminator" (PLAN-V1.md §7 R4-I-15: "the exact
// new-vs-grandfathered discriminator is settled at V1-P2 review"),
// adopting the brief's recommended rule: the story class is always new (no
// v0 story class ever existed at all — R4-I-9's "superseded: v0's
// story-grained feature class"); a feature spec is new iff it carries ANY
// round-four surface field — problem, outcome, stubs, supersession, or a
// constraints/decisions/open_questions object block. A v0 grandfathered
// feature spec (e.g. examples/showcase's stale-decline, loan-refi-2023)
// carries none of these.
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
