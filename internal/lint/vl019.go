package lint

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/storyresolve"
)

// vl019 enforces spec/obligation-artifact AC-2 (spec/evidence-obligations
// DC-3): an obligation's single `verifies` edge must target a WHOLE STORY
// spec (a bare spec/<story> ref, no object fragment — mirroring an
// attestation's own verifies edge; see obligation.go), and the acceptance
// criterion named by the obligation's OWN id (its <ac-id> segment, via
// artifact.SplitObligationName) must be one that story genuinely declares.
// An obligation is refused when its target is unresolvable, is not a spec,
// carries a fragment, resolves to a FEATURE-class spec rather than a STORY
// (the feature-blind/story-scoped split 03 §The feature fold already
// enforces elsewhere — obligations are a story-level concern only), or is a
// STORY that does not declare the id's <ac-id> as an acceptance criterion.
// Every refusal names the offending target (D6-18: never a silent absence).
//
// The target's class is resolved through storyresolve.LoadSpec, mirroring
// cmd/verdi/accept.go's supersedesTargetsStory/supersedesTargetsFeature
// (spec/obligation-artifact DC-3: "reuse that pattern; do not reinvent
// class resolution") rather than re-deriving spec-class resolution here.
//
// This rule deliberately does not re-check anything internal/artifact's
// ObligationFrontmatter.Validate already owns (id/for_kind agreement, a
// malformed id, exactly one verifies link with a whole-spec ref, frozen
// requiredness) — per doc.go's design note, each rule owns exactly the
// semantic slice 02 assigns it, and the verifies-target's CLASS plus the
// id-named AC's existence are this rule's own slice alone: neither can be
// checked at decode time because a bare frontmatter decode cannot see the
// corpus/index.
type vl019 struct{}

func (vl019) ID() string { return "VL-019" }

func (vl019) Check(in *RunInput) []Finding {
	var findings []Finding
	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Obligation == nil {
			continue
		}
		// ObligationFrontmatter.Validate already requires exactly one
		// links entry of type verifies; guarded defensively (rather than
		// indexing blindly) so a future decode relaxation degrades to
		// "nothing to check" here instead of a panic.
		if len(d.Obligation.Links) != 1 {
			continue
		}
		ref := d.Obligation.Links[0].Ref

		// The acceptance criterion is named by the obligation's OWN id, not
		// by its verifies edge — that edge targets the whole story spec,
		// exactly as an attestation's does (obligation.go). Parse the id for
		// its <ac-id> segment; a malformed id is VL-002's finding, not this
		// rule's, so degrade to "nothing to check" here.
		idRef, err := artifact.ParseRef(d.Obligation.ID)
		if err != nil {
			continue
		}
		_, acID, _, ok := artifact.SplitObligationName(idRef.Name)
		if !ok {
			continue // shape already enforced at decode (obligationNameRe)
		}

		if reason, bad := badVerifiesTarget(in.Root, ref, acID); bad {
			findings = append(findings, Finding{Rule: "VL-019", Path: d.RelPath, Message: fmt.Sprintf("obligation %s verifies %s, %s", d.Obligation.ID, ref, reason)})
		}
	}
	return findings
}

// badVerifiesTarget classifies an obligation's verifies target ref (expected
// to be a whole story-spec ref) together with acID — the acceptance-criterion
// id the obligation's own id names — and reports whether the obligation is
// refused, and why, always naming the offending target ref itself (D6-18:
// never a silent absence). The only accepted shape is a WHOLE spec ref (no
// object fragment) that resolves to a STORY-class spec in the committed zone
// whose own declared acceptance criteria include acID; every other shape
// fails closed (unresolvable, non-spec, fragment-bearing, feature-class, or a
// story that does not declare acID). The AC is carried by the id, never the
// edge — exactly as it is for an attestation (obligation.go).
func badVerifiesTarget(root, ref, acID string) (reason string, bad bool) {
	r, err := artifact.ParseRef(ref)
	if err != nil {
		return fmt.Sprintf("which does not parse as a ref: %v", err), true
	}
	if r.Kind != artifact.KindSpec || r.Fragment() {
		return "which is not a whole story-spec ref — an obligation verifies the whole spec/<story> (the AC is named by the obligation's own id, mirroring attestations)", true
	}

	target, err := storyresolve.LoadSpec(root, r.Name)
	if err != nil || target == nil {
		return "which does not resolve to a spec in the committed zone", true
	}

	if target.Class != artifact.ClassStory {
		return fmt.Sprintf("a %s-class spec, not a STORY — obligations attach to STORY ACs only (03 §The feature fold)", target.Class), true
	}

	for _, ac := range target.AcceptanceCriteria {
		if ac.ID == acID {
			return "", false
		}
	}
	return fmt.Sprintf("but its id names ac %q, which %s does not declare as an acceptance criterion", acID, target.ID), true
}
