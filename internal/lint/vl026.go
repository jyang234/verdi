package lint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/model"
)

// vl026 enforces "closed-spec object supersession shape: a feature or
// component spec's top-level `links:` target no object fragment; no
// top-level `supersedes` link targets an object of a closed spec; a
// decision's `supersedes` link to an object of a closed spec targets a
// declared acceptance criterion or decision; a conflict's fragment
// `challenges` all name one spec, and a superseded conflict with fragment
// `challenges` names an existing spec in `resolved_by`. The match between
// edges and conflicts is the decision-conflict gate's (evidence-model spec
// §Decision-conflict gate), not this rule's" (02 §Lint rules; 02 §Link
// taxonomy; SI-264). It checks shape only, from the committed zone, with
// no git and no spec-state reads. Each finding names its clause:
//
//   - (a) a feature or component spec's top-level links target no object
//     fragment, of any link type (BL-66);
//   - (b) no top-level supersedes link, on any artifact, targets an object
//     of a closed spec;
//   - (c) a decision's supersedes link to an object of a closed spec
//     targets a declared acceptance criterion or decision of that spec;
//   - (d) a conflict's fragment challenges all name one spec;
//   - (e) a superseded conflict whose challenges include an object
//     fragment carries resolved_by, naming a spec in the store (either
//     zone);
//   - (f) every conflict's resolved_by is inside SI-269's decode scope
//     (artifact.ConflictFrontmatter.ValidateResolvedBy): lint decodes
//     strictly without a kind's own Validate (doc.go), so without this a
//     resolved_by the index cannot read would lint clean;
//   - (g) a decision's supersedes link to an object of a closed spec is
//     unpinned (SI-271). A conflict's pinned fragment challenge is VL-003's
//     (artifact.Link.ValidateFor refuses it), so it is not repeated here.
//
// "Closed" is the archive zone alone (SI-277): the target spec's document
// sits under .verdi/specs/archive/. A spec's own status: field is never
// read, so an active-zone spec claiming `status: closed` is not closed
// here; that store is VL-002's to reject. Each clause is self-contained
// (doc.go), so a link another rule also refuses (VL-003's undeclared
// fragment) is still reported here when it breaks a VL-026 clause.
type vl026 struct{}

func (vl026) ID() string { return "VL-026" }

func (r vl026) Check(in *RunInput) []Finding {
	var findings []Finding
	for _, d := range in.Snapshot.Docs {
		if d.DecodeErr != nil {
			continue
		}
		findings = append(findings, r.checkTopLevelLinks(in, d)...)
		if d.Spec != nil {
			findings = append(findings, r.checkDecisionEdges(in.Snapshot, d)...)
		}
		if d.Conflict != nil {
			findings = append(findings, r.checkConflict(in.Snapshot, d)...)
		}
	}
	return findings
}

// vl026Finding builds a VL-026 violation on d naming clause, with the
// wall locus the calling clause self-declares (Finding.Locus; spec/
// badge-computes dc-3): ObjectLocus of the decision for a decision-edge
// clause, SpecLocus for a top-level-link clause on a spec, and nil (no
// wall) otherwise.
func vl026Finding(d *Document, clause, message string, locus *WallLocus) Finding {
	return Finding{Rule: "VL-026", Path: d.RelPath, Message: "clause (" + clause + "): " + message, Locus: locus}
}

// checkTopLevelLinks is clauses (a) and (b), over d's own top-level links.
// Top-level links belong to the artifact itself, not to any one object,
// so on a spec they badge the case file (SpecLocus, as VL-003's
// top-level links do); a non-spec carrier (an ADR) has no wall, so its
// findings declare no locus.
func (vl026) checkTopLevelLinks(in *RunInput, d *Document) []Finding {
	fragmentFree := d.Spec != nil && (d.Spec.Class == artifact.ClassFeature || d.Spec.Class == artifact.ClassComponent)
	var locus *WallLocus
	if d.Spec != nil {
		locus = SpecLocus()
	}
	var findings []Finding
	for _, l := range d.Base.Links {
		ref, ok := vl026FragmentRef(l)
		if !ok {
			continue
		}
		if fragmentFree {
			findings = append(findings, vl026Finding(d, "a", fmt.Sprintf("top-level links[].ref %q targets object fragment #%s, but %s spec's top-level links target no object fragment (02 §Link taxonomy; VL-026)", l.Ref, ref.Object, model.Indefinite(in.Model.DisplayClass(string(d.Spec.Class)))), locus))
		}
		if l.Type != artifact.LinkSupersedes {
			continue
		}
		if target := vl026ClosedSpec(in.Snapshot, ref); target != nil {
			// vocab:identity — VL-026 rule citation: "closed" is the target's archive-zone placement (SI-277), not display prose
			const msg = "top-level supersedes link %q targets #%s of closed spec %s, but a top-level supersedes link never targets an object of a closed spec: that edge belongs on a decision (02 §Link taxonomy; VL-026)"
			findings = append(findings, vl026Finding(d, "b", fmt.Sprintf(msg, l.Ref, ref.Object, target.Base.ID), locus))
		}
	}
	return findings
}

// checkDecisionEdges is clauses (c) and (g), over every decision's own
// supersedes links on spec d. A decision's own links name its rendered
// card, so each finding badges that decision (ObjectLocus, as VL-003's
// decision links do).
func (vl026) checkDecisionEdges(snap *Snapshot, d *Document) []Finding {
	var findings []Finding
	for _, dc := range d.Spec.Decisions {
		for _, l := range dc.Links {
			if l.Type != artifact.LinkSupersedes {
				continue
			}
			ref, ok := vl026FragmentRef(l)
			if !ok {
				continue
			}
			target := vl026ClosedSpec(snap, ref)
			if target == nil {
				continue
			}
			if what, ok := vl026SupersedableObject(target.Spec, ref.Object); !ok {
				// vocab:identity — VL-026 rule citation: "closed" is the target's archive-zone placement (SI-277), not display prose
				const msg = "decisions[%s].links[].ref %q supersedes #%s of closed spec %s, which %s, but a decision's supersedes link to an object of a closed spec targets a declared acceptance criterion or decision (02 §Link taxonomy, §Object model; VL-026)"
				findings = append(findings, vl026Finding(d, "c", fmt.Sprintf(msg, dc.ID, l.Ref, ref.Object, target.Base.ID, what), ObjectLocus(dc.ID)))
			}
			if ref.Pinned() {
				// vocab:identity — VL-026 rule citation: "closed" is the target's archive-zone placement (SI-277), not display prose
				const msg = "decisions[%s].links[].ref %q supersedes #%s of closed spec %s pinned at %s, but refs inside links are unpinned and the edge names the object as spec/<name>#<object-id> (02 §Link taxonomy, §Common frontmatter; SI-271; VL-026)"
				findings = append(findings, vl026Finding(d, "g", fmt.Sprintf(msg, dc.ID, l.Ref, ref.Object, target.Base.ID, ref.Commit), ObjectLocus(dc.ID)))
			}
		}
	}
	return findings
}

// checkConflict is clauses (d), (e), and (f), over conflict d. A conflict
// has no wall, so its findings declare no locus.
func (vl026) checkConflict(snap *Snapshot, d *Document) []Finding {
	var findings []Finding

	// named is the set of artifacts, by kind/name, the conflict's fragment
	// challenges name. Whole-artifact challenges are not counted: clause
	// (d) constrains fragments only.
	named := map[string]bool{}
	for _, l := range d.Base.Links {
		if l.Type != artifact.LinkChallenges {
			continue
		}
		if ref, ok := vl026FragmentRef(l); ok {
			named[artifact.Ref{Kind: ref.Kind, Name: ref.Name}.String()] = true
		}
	}
	if len(named) > 1 {
		names := make([]string, 0, len(named))
		for n := range named {
			names = append(names, n)
		}
		sort.Strings(names)
		findings = append(findings, vl026Finding(d, "d", fmt.Sprintf("fragment challenges name objects of %d artifacts (%s), but a conflict's fragment challenges all name objects of one spec (02 §Link taxonomy; VL-026)", len(names), strings.Join(names, ", ")), nil))
	}

	if d.Conflict.Status == "superseded" && len(named) > 0 {
		switch {
		case d.Conflict.ResolvedBy == "":
			// vocab:identity — conflict status and field ids (status: superseded, resolved_by), quoting 02 §Kind registry
			const msg = "status superseded with fragment challenges but no resolved_by: a superseded conflict whose challenges name object fragments carries resolved_by: spec/<name>, the successor spec that resolved it (02 §Link taxonomy, §Kind registry; VL-026)"
			findings = append(findings, vl026Finding(d, "e", msg, nil))
		case !vl026SpecExists(snap, d.Conflict.ResolvedBy):
			// vocab:identity — conflict status and field ids (status: superseded, resolved_by), quoting 02 §Kind registry
			const msg = "resolved_by %q does not name a spec in the store (either zone), but a superseded conflict with fragment challenges names an existing spec in resolved_by (02 §Link taxonomy, §Kind registry; VL-026)"
			findings = append(findings, vl026Finding(d, "e", fmt.Sprintf(msg, d.Conflict.ResolvedBy), nil))
		}
	}

	if err := d.Conflict.ValidateResolvedBy(); err != nil {
		findings = append(findings, vl026Finding(d, "f", fmt.Sprintf("%v (02 §Link taxonomy; SI-269; VL-026)", err), nil))
	}
	return findings
}

// vl026FragmentRef parses l's ref and reports whether it names an object
// fragment. A tracker (story) ref, an svc/... external ref, or a ref that
// does not parse (VL-001's and VL-003's to report) names none.
func vl026FragmentRef(l artifact.Link) (artifact.Ref, bool) {
	if l.Type == artifact.LinkStory {
		return artifact.Ref{}, false
	}
	ref, err := artifact.ParseRef(l.Ref)
	if err != nil || !ref.Fragment() {
		return artifact.Ref{}, false
	}
	return ref, true
}

// vl026ArchivePrefix is the spec store's archive zone, store-relative
// (01 §Directory layout).
const vl026ArchivePrefix = ".verdi/specs/archive/"

// vl026ClosedSpec returns the committed-zone spec document that ref's
// kind/name half names when that spec is closed for VL-026: its document
// sits in the archive zone, and that alone decides it (SI-277), with no
// git and no read of the spec's status: field. It returns nil when ref
// names no spec, a spec that does not resolve (VL-003's), or one outside
// the archive zone, including an active-zone spec claiming
// `status: closed` (VL-002's to reject).
func vl026ClosedSpec(snap *Snapshot, ref artifact.Ref) *Document {
	if ref.Kind != artifact.KindSpec {
		return nil
	}
	for _, t := range snap.ByRef[artifact.Ref{Kind: ref.Kind, Name: ref.Name}.String()] {
		if t.Kind == "spec" && t.Spec != nil && strings.HasPrefix(t.RelPath, vl026ArchivePrefix) {
			return t
		}
	}
	return nil
}

// vl026SupersedableObject reports whether id names a declared acceptance
// criterion or decision of spec — the only objects a closed-spec object
// supersession may target (design §2) — and otherwise says what id is.
func vl026SupersedableObject(spec *artifact.SpecFrontmatter, id string) (what string, ok bool) {
	for _, ac := range spec.AcceptanceCriteria {
		if ac.ID == id {
			return "", true
		}
	}
	for _, dc := range spec.Decisions {
		if dc.ID == id {
			return "", true
		}
	}
	for _, c := range spec.Constraints {
		if c.ID == id {
			return "is a constraint", false
		}
	}
	for _, q := range spec.OpenQuestions {
		if q.ID == id {
			return "is an open question", false
		}
	}
	if artifact.DeclaredStubSlugs(spec)[id] {
		return "is a stub", false
	}
	return "is not declared", false
}

// vl026SpecExists reports whether s names, by its kind/name half, a spec
// document in the committed store, in either zone.
func vl026SpecExists(snap *Snapshot, s string) bool {
	ref, err := artifact.ParseRef(s)
	if err != nil || ref.Kind != artifact.KindSpec {
		return false
	}
	for _, t := range snap.ByRef[artifact.Ref{Kind: ref.Kind, Name: ref.Name}.String()] {
		if t.Kind == "spec" && t.Spec != nil {
			return true
		}
	}
	return false
}
