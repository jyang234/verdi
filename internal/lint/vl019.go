package lint

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/storyresolve"
)

// vl019 enforces spec/obligation-artifact AC-2 (spec/evidence-obligations
// DC-3): an obligation's single `verifies` edge must target a STORY
// acceptance-criterion fragment — never a FEATURE AC, a non-AC fragment (a
// constraint/decision/open-question, or any id the target spec does not
// itself declare as one of its own acceptance criteria), or a whole spec
// (no fragment at all). The feature-blind/story-scoped split 03 §The
// feature fold already enforces elsewhere (obligations are a story-level
// concern only) is carried to obligations unchanged. Every refusal names
// the offending target (D6-18: never a silent absence).
//
// The target's class is resolved through storyresolve.LoadSpec, mirroring
// cmd/verdi/accept.go's supersedesTargetsStory/supersedesTargetsFeature
// (spec/obligation-artifact DC-3: "reuse that pattern; do not reinvent
// class resolution") rather than re-deriving spec-class resolution here.
//
// This rule deliberately does not re-check anything internal/artifact's
// ObligationFrontmatter.Validate already owns (id/for_kind agreement, a
// malformed id, exactly one verifies link, frozen requiredness) — per
// doc.go's design note, each rule owns exactly the semantic slice 02
// assigns it, and the verifies-target's CLASS is this rule's own slice
// alone: it cannot be checked at decode time because a bare frontmatter
// decode cannot see the corpus/index.
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

		if reason, bad := badVerifiesTarget(in.Root, ref); bad {
			findings = append(findings, Finding{Rule: "VL-019", Path: d.RelPath, Message: fmt.Sprintf("obligation %s verifies %s, %s", d.Obligation.ID, ref, reason)})
		}
	}
	return findings
}

// badVerifiesTarget classifies ref — an obligation's verifies target — and
// reports whether it is refused, and why, always naming the offending
// target ref itself (D6-18: never a silent absence). The only accepted
// shape is a fragment ref into a STORY-class spec, naming one of that
// spec's own declared acceptance criteria; every other shape fails closed
// (unresolvable, non-spec, whole-spec, non-AC fragment, or feature AC).
func badVerifiesTarget(root, ref string) (reason string, bad bool) {
	r, err := artifact.ParseRef(ref)
	if err != nil {
		return fmt.Sprintf("which does not parse as a ref: %v", err), true
	}
	if r.Kind != artifact.KindSpec || !r.Fragment() {
		return "which does not target an acceptance-criterion fragment of a spec (a whole spec, or a non-spec kind) — obligations attach to STORY ACs only", true
	}

	target, err := storyresolve.LoadSpec(root, r.Name)
	if err != nil || target == nil {
		return "which does not resolve to a spec in the committed zone", true
	}

	isAC := false
	for _, ac := range target.AcceptanceCriteria {
		if ac.ID == r.Object {
			isAC = true
			break
		}
	}
	if !isAC {
		return fmt.Sprintf("which targets non-AC fragment %q on %s — obligations attach to STORY acceptance criteria only", r.Object, target.ID), true
	}

	if target.Class != artifact.ClassStory {
		return fmt.Sprintf("which targets a %s-class AC (%s) on %s — obligations attach to STORY ACs only (03 §The feature fold)", target.Class, r.Object, target.ID), true
	}

	return "", false
}
