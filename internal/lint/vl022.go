package lint

import (
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

// vl022 enforces spec/attest-helper AC-3 (spec/closure-ergonomics AC-2's
// enforcement half): an attestation's own on-disk story-slug segment
// (parsed from its id, mirroring VL-011's compound-name split) must agree
// with store.RefSlug(target.Story) for the spec its `verifies` edge
// names — the D6-18 failure class (a spec-name slug substituted for the
// story-ref slug) made a witness-carrying refusal instead of a silent
// fold-time absent (internal/evidence.AttestationExists/LoadAttestationState
// are bare filesystem checks; neither can tell "misfiled" from "never
// attested").
//
// Scoped to attestations that carry a `verifies` edge AT ALL (DC-4): every
// attestation this rule ever examines is one written (or at least
// hand-annotated) with a verifies edge; a hand-authored attestation with no
// edge at all is out of this rule's scope by construction, needing no
// enumerated grandfather-baseline map (contrast VL-020's own
// obligationGateBaseline, the harder way).
//
// Mirrors vl019.go's own badVerifiesTarget pattern (an obligation's twin
// check), extended with the one genuinely new piece attestations need that
// obligations don't: a slug-derivation step. An obligation's verifies
// target IS named by its own directory (vl019.go: the obligation's
// story-slug segment is the target spec's own NAME) — but an attestation's
// path segment is the STORY's ref slug, store.RefSlug(target.Story), a
// different string entirely (I-6/D6-16).
type vl022 struct{}

func (vl022) ID() string { return "VL-022" }

func (vl022) Check(in *RunInput) []Finding {
	var findings []Finding
	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Kind != "attestation" {
			continue
		}

		var verifiesRef string
		found := false
		for _, l := range d.Base.Links {
			if l.Type == artifact.LinkVerifies {
				verifiesRef = l.Ref
				found = true
				break
			}
		}
		if !found {
			continue // DC-4: no verifies edge at all — out of scope by construction
		}

		ref, err := artifact.ParseRef(d.Base.ID)
		if err != nil {
			continue // VL-002 already reports this
		}
		slugSeg, acID, ok := strings.Cut(ref.Name, "--")
		if !ok {
			continue // shape already enforced at decode (I-6 compoundNameRe)
		}

		if reason, bad := badAttestationVerifiesTarget(in.Root, verifiesRef, slugSeg, acID); bad {
			findings = append(findings, Finding{Rule: "VL-022", Path: d.RelPath, Message: fmt.Sprintf("attestation %s verifies %s, %s", d.Base.ID, verifiesRef, reason)})
		}
	}
	return findings
}

// badAttestationVerifiesTarget classifies an attestation's verifies target
// ref (expected to be a whole story-spec ref) together with slugSeg (the
// attestation's own on-disk story-slug segment, parsed from its compound
// id) and acID (the acceptance-criterion id the attestation's own id/path
// names), and reports whether the attestation is refused, and why, always
// naming the offending value (D6-18: never a silent absence). The only
// accepted shape is a WHOLE spec ref (no object fragment) that resolves to
// a STORY-class spec in the committed zone whose own declared acceptance
// criteria include acID AND whose own story-ref slug
// (store.RefSlug(target.Story)) equals slugSeg; every other shape fails
// closed (unresolvable, non-spec, fragment-bearing, non-story-class, an
// undeclared AC, or a slug disagreement).
func badAttestationVerifiesTarget(root, verifiesRef, slugSeg, acID string) (reason string, bad bool) {
	r, err := artifact.ParseRef(verifiesRef)
	if err != nil {
		return fmt.Sprintf("which does not parse as a ref: %v", err), true
	}
	if r.Kind != artifact.KindSpec || r.Fragment() {
		return "which is not a whole spec ref — an attestation verifies the whole story spec (the AC is named by the attestation's own id, 02 §Link taxonomy's closed edge vocabulary)", true
	}

	target, err := storyresolve.LoadSpec(root, r.Name)
	if err != nil || target == nil {
		return "which does not resolve to a spec in the committed zone", true
	}

	if target.Class != artifact.ClassStory {
		return fmt.Sprintf("a %s-class spec, not a STORY — verdi attest scaffolds STORY attestations only (spec/attest-helper dc-5)", target.Class), true
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
