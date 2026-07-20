package lint

import (
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

// vl022 enforces spec/attest-helper AC-3 (spec/closure-ergonomics AC-2's
// enforcement half): a STORY-targeting attestation's own on-disk story-slug
// segment (parsed from its id, mirroring VL-011's compound-name split) must
// agree with store.RefSlug(target.Story) for the class: story spec its
// `verifies` edge names — the D6-18 failure class (a spec-name slug
// substituted for the story-ref slug) made a witness-carrying refusal
// instead of a silent fold-time absent
// (internal/evidence.AttestationExists/LoadAttestationState are bare
// filesystem checks; neither can tell "misfiled" from "never attested").
//
// Subject — STORY-targeting attestations only (Controller adjudication
// ADJ-51, 2026-07-16): the rule fires only on an attestation whose `verifies`
// edge resolves to a class: story spec — the D6-18 misfiling class the story
// exists to kill, symmetric with dc-5's own story-scoped verb ("verdi attest
// scaffolds STORY attestations only"). A `verifies` edge to a NON-story spec
// is a legitimate R4-I-11 feature-outcome attestation (hand-authored, keyed
// by the feature's own slug), OUTSIDE this rule's subject: skipped, never
// refused. This is the smallest reversible reading forced by the store's own
// real data: frozen dc-4 asserts "every attestation in the store as of this
// contract carries no verifies edge," but that empirical premise is FALSE —
// 11 legitimate, frozen feature-outcome attestations across this repo's own
// store and examples/showcase DO carry a verifies edge to a class: feature
// spec, and refusing them (dc-4's own letter) broke `make verify`. ADJ-51's
// ruling: a rule's subject is defined by the defect class it kills, not by a
// premise the store itself refutes; no enumerated grandfather-baseline map is
// introduced (dc-4 explicitly rejects one), because a non-story target is out
// of scope by CONSTRUCTION, not by exemption. Mis-slug protection for
// feature-outcome attestations is a genuine residual gap, filed as a future
// story per ADJ-51 (broadening VL-022 to class-coherent story|feature pairing
// would invent feature-slug-derivation semantics inside a build — the
// quiet-widening class ADJ-51 declined).
//
// Scoped, further, to attestations that carry a `verifies` edge AT ALL
// (DC-4): a hand-authored attestation with no edge at all is out of scope by
// construction. Mirrors vl019.go's own badVerifiesTarget pattern (an
// obligation's twin check), extended with the one genuinely new piece
// attestations need that obligations don't: a slug-derivation step. An
// obligation's verifies target IS named by its own directory (vl019.go: the
// obligation's story-slug segment is the target spec's own NAME) — but an
// attestation's path segment is the STORY's ref slug,
// store.RefSlug(target.Story), a different string entirely (I-6/D6-16).
type vl022 struct{}

func (vl022) ID() string { return "VL-022" }

func (vl022) Check(in *RunInput) []Finding {
	var findings []Finding
	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Kind != "attestation" {
			continue
		}

		ref, err := artifact.ParseRef(d.Base.ID)
		if err != nil {
			continue // VL-002 already reports this
		}
		slugSeg, acID, ok := strings.Cut(ref.Name, "--")
		if !ok {
			continue // shape already enforced at decode (I-6 compoundNameRe)
		}

		// Validate EVERY verifies edge, not only the first (ADJ-51 finding 4):
		// AttestationFrontmatter.Validate places no cardinality constraint on
		// links, so a hand-annotated attestation may carry more than one, and
		// a misfiled edge after a clean one must not fold silently. An
		// attestation with no verifies edge at all iterates zero times — out
		// of scope by construction (DC-4).
		for _, l := range d.Base.Links {
			if l.Type != artifact.LinkVerifies {
				continue
			}
			if reason, bad := badAttestationVerifiesTarget(in.Root, l.Ref, slugSeg, acID, in.Model); bad {
				findings = append(findings, Finding{Rule: "VL-022", Path: d.RelPath, Message: fmt.Sprintf("attestation %s verifies %s, %s", d.Base.ID, l.Ref, reason)})
			}
		}
	}
	return findings
}

// badAttestationVerifiesTarget classifies one of an attestation's verifies
// target refs (expected to be a whole story-spec ref) together with slugSeg
// (the attestation's own on-disk story-slug segment, parsed from its compound
// id) and acID (the acceptance-criterion id the attestation's own id/path
// names), and reports whether the attestation is refused, and why, always
// naming the offending value (D6-18: never a silent absence). The only
// accepted shape is a WHOLE spec ref (no object fragment) that resolves to
// a STORY-class spec in the committed zone whose own declared acceptance
// criteria include acID AND whose own story-ref slug
// (store.RefSlug(target.Story)) equals slugSeg; a malformed edge
// (unresolvable, non-spec, fragment-bearing) fails closed; an in-scope
// story target with an undeclared AC or a slug disagreement is refused.
//
// A whole-spec-ref edge that resolves to a NON-story spec (feature-outcome
// attestation, R4-I-11) is OUT OF SCOPE (ADJ-51): it returns no refusal —
// the rule's subject is story-targeting attestations only.
func badAttestationVerifiesTarget(root, verifiesRef, slugSeg, acID string, mdl *model.Model) (reason string, bad bool) {
	r, err := artifact.ParseRef(verifiesRef)
	if err != nil {
		return fmt.Sprintf("which does not parse as a ref: %v", err), true
	}
	if r.Kind != artifact.KindSpec || r.Fragment() {
		// The spoken class word is display and resolves (L-M13a(6) work
		// order); "closed edge vocabulary" is 02's own term for the edge
		// taxonomy being a closed SET — not the lifecycle state.
		return fmt.Sprintf("which is not a whole spec ref — an attestation verifies the whole %s spec (the AC is named by the attestation's own id, 02 §Link taxonomy's closed edge vocabulary)", mdl.DisplayClass("story")), true
	}

	target, err := storyresolve.LoadSpec(root, r.Name)
	if err != nil || target == nil {
		return "which does not resolve to a spec in the committed zone", true
	}

	if target.Class != artifact.ClassStory {
		// Out of scope (ADJ-51): a verifies edge to a non-story spec is a
		// legitimate feature-outcome attestation, not the D6-18 misfiling
		// class this rule kills. Skipped, never refused, no baseline map.
		return "", false
	}

	declared := false
	for _, ac := range target.AcceptanceCriteria {
		if ac.ID == acID {
			declared = true
			break
		}
	}
	if !declared {
		return fmt.Sprintf("but its own id names ac %q, which %s does not declare as an acceptance criterion", acID, target.ID), true
	}

	wantSlug := store.RefSlug(target.Story)
	if slugSeg != wantSlug {
		return fmt.Sprintf("whose own story-ref slug is %q, but this attestation's own directory/id segment is %q (D6-18: a spec-name/story-slug mismatch used to fold as a silent absent, never a misfiled attestation)", wantSlug, slugSeg), true
	}

	return "", false
}
