package lint

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
)

// vl015 enforces "supersession manifest completeness and fidelity: every
// object in the predecessor spec (at its frozen.commit) is classified
// exactly once across the superseding revision's supersession: block
// (carried/amended/amended_advisory/removed, plus added); every carried
// object's (kind, id, text) content is byte-identical to its predecessor
// (§Object model) — fail closed on drift" (02 §Lint rules; R4-I-4). The
// predecessor's own object manifest is read from real git history at its
// frozen.commit (gitx.Show), not the live working-tree document — 02 is
// explicit the manifest is checked "at its frozen.commit", and a frozen
// file is immutable (VL-010) so the two should already agree, but VL-015
// itself proves it rather than assuming it.
type vl015 struct{}

func (vl015) ID() string { return "VL-015" }

func (r vl015) Check(in *RunInput) []Finding {
	var findings []Finding
	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Spec == nil || d.Spec.Supersession == nil {
			continue
		}
		// badge-computes dc-3's "spec-stale adjacent rules" spec-level
		// bucket: supersession-manifest fidelity is a whole-revision
		// property (every predecessor object classified exactly once), not
		// any single card's own defect, even where a message names one
		// object id among many it checks.
		findings = append(findings, locusAll(r.checkOne(in, d), SpecLocus())...)
	}
	return findings
}

// checkOne runs VL-015 for one superseding revision d.
func (vl015) checkOne(in *RunInput, d *Document) []Finding {
	// I-47 (PLAN.md §7): the manifest is a statement about ONE named
	// predecessor's object set, so the revision must name exactly one
	// whole-spec, spec-kind predecessor. Zero leaves the manifest
	// un-anchored; two or more make it ambiguous — VL-015 used to validate
	// against whichever supersedes link happened to come FIRST, silently
	// leaving every other named predecessor an unchecked supersession claim
	// (which internal/specstate then turned into an unearned terminal
	// Superseded verdict). Reported and returned here rather than guessed
	// past: with no determinate predecessor there is no object manifest to
	// check completeness or byte-identity against.
	//
	// internal/artifact enforces the same invariant at decode time
	// (validateSupersessionPredecessor) — that is the primary, fail-closed
	// signal, and it is what internal/specstate relies on. lint carries its
	// own copy of the CHECK (the design note in doc.go: every semantic rule
	// is re-implemented against the raw decoded struct rather than by
	// calling a kind's Validate()) but shares the DEFINITION of "whole-spec
	// predecessor" through artifact.WholeSpecSupersedesRefs.
	predRefs := artifact.WholeSpecSupersedesRefs(d.Base.Links)
	if len(predRefs) != 1 {
		return []Finding{{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf(
			"supersession: block is present but the revision names %d whole-spec predecessors via links: {type: supersedes} — exactly one is required, since the manifest classifies exactly one predecessor's objects (02 §Lint rules VL-015, §Kind registry, §Link taxonomy; I-47). A fragment supersedes edge (spec/x#object-id) is a decision-level override and does not count",
			len(predRefs))}}
	}
	predRef := predRefs[0].String()

	predDocs, ok := in.Snapshot.ByRef[predRef]
	if !ok || len(predDocs) == 0 || predDocs[0].Spec == nil {
		// VL-003 already flags a dangling supersedes ref; nothing more this
		// rule can check without a resolved predecessor.
		return nil
	}
	pred := predDocs[0]
	if pred.Base.Frozen == nil {
		return []Finding{{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf("predecessor %s has no frozen stamp to read its object manifest from (02: the manifest is checked at its frozen.commit)", predRef)}}
	}

	content, err := gitx.Show(in.Ctx, in.Root, pred.Base.Frozen.Commit, pred.RelPath)
	if err != nil {
		return []Finding{{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf("reading predecessor %s at its frozen commit %s: %v", predRef, pred.Base.Frozen.Commit, err)}}
	}
	fm, _, err := artifact.SplitFrontmatter(content)
	if err != nil {
		return []Finding{{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf("predecessor %s frontmatter at its frozen commit does not split: %v", predRef, err)}}
	}
	var predSpec artifact.SpecFrontmatter
	if err := artifact.DecodeStrict(fm, &predSpec); err != nil {
		return []Finding{{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf("predecessor %s frontmatter at its frozen commit does not decode: %v", predRef, err)}}
	}

	predObjects := specObjects(&predSpec)
	succObjects := specObjects(d.Spec)
	sup := d.Spec.Supersession

	classified := map[string]int{}
	for _, id := range sup.Carried {
		classified[id]++
	}
	for _, n := range sup.Amended {
		classified[n.ID]++
	}
	for _, n := range sup.AmendedAdvisory {
		classified[n.ID]++
	}
	for _, n := range sup.Removed {
		classified[n.ID]++
	}

	var findings []Finding
	for id := range predObjects {
		switch classified[id] {
		case 0:
			findings = append(findings, Finding{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf("predecessor object %s is not classified anywhere in supersession: (carried/amended/amended_advisory/removed)", id)})
		case 1:
			// exactly once: fine
		default:
			findings = append(findings, Finding{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf("predecessor object %s is classified more than once across supersession: buckets", id)})
		}
	}
	for id := range classified {
		if _, ok := predObjects[id]; !ok {
			findings = append(findings, Finding{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf("supersession: names %s, which is not an object %s declares", id, predRef)})
		}
	}

	for _, id := range sup.Carried {
		predObj, ok := predObjects[id]
		if !ok {
			continue // already flagged above
		}
		succObj, ok := succObjects[id]
		if !ok {
			findings = append(findings, Finding{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf("carried object %s is classified carried but is not declared on this revision at all", id)})
			continue
		}
		if succObj.Kind != predObj.Kind || succObj.Text != predObj.Text {
			findings = append(findings, Finding{Rule: "VL-015", Path: d.RelPath, Message: fmt.Sprintf("carried object %s content drifted from its predecessor — byte-identical required (02 §Object model): predecessor=%q successor=%q", id, predObj.Text, succObj.Text)})
		}
	}

	return findings
}

// objectEntry is one frontmatter-declared object's cross-revision identity
// tuple minus id (02 §Object model, the I-37 identity): its block kind and
// text.
type objectEntry struct {
	Kind artifact.ObjectKind
	Text string
}

// specObjects indexes every frontmatter-declared object on spec (across
// all four object blocks) by id.
func specObjects(spec *artifact.SpecFrontmatter) map[string]objectEntry {
	m := make(map[string]objectEntry, len(spec.AcceptanceCriteria)+len(spec.Constraints)+len(spec.Decisions)+len(spec.OpenQuestions))
	for _, ac := range spec.AcceptanceCriteria {
		m[ac.ID] = objectEntry{artifact.ObjectKindAcceptanceCriterion, ac.Text}
	}
	for _, c := range spec.Constraints {
		m[c.ID] = objectEntry{artifact.ObjectKindConstraint, c.Text}
	}
	for _, dc := range spec.Decisions {
		m[dc.ID] = objectEntry{artifact.ObjectKindDecision, dc.Text}
	}
	for _, q := range spec.OpenQuestions {
		m[q.ID] = objectEntry{artifact.ObjectKindOpenQuestion, q.Text}
	}
	return m
}
