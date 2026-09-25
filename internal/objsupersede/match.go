package objsupersede

import (
	"context"
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// Result is one evaluated decision edge, or one completeness finding
// (Decision == "") for a fragment that a conflict naming Spec challenges
// with no matching edge on Spec (design §5).
type Result struct {
	Spec     string // the evaluated spec S (bare name)
	Decision string // the decision holding the edge; "" for completeness
	Edge     string // the edge's ref, or the challenged fragment, as written
	Outcome  Outcome
	Reason   Reason // "" unless Outcome is Unresolved
	Conflict string // the conflict the result rests on, when one was found
	// Other names the other spec the result rests on: the establishing
	// successor (carried, SI-274(3)), the other successor (condition 5),
	// resolved_by (condition 8), the named successor (SI-274(1)), or the
	// revision whose step failed (condition 9).
	Other       string
	Predecessor string // condition 9: the failed step's predecessor
	Since       string // carried: the establishing successor's acceptance date
	Detail      string // decode failures, a nested reason, or a missing witness
}

// Establisher answers whether an establishing successor's supersession of
// object was in force at its acceptance, with that acceptance's date
// (SI-270, SI-274(3)). History implements it.
type Establisher interface {
	Establishment(ctx context.Context, successor string, object artifact.Ref) Establishment
}

// Establishment is an Establisher's answer. Reason is "" when the
// supersession was in force, and otherwise one of
// ReasonEstablisherNotAccepted, ReasonEstablisherNotInForce (Detail: the
// failing reason's text), or ReasonAcceptanceUnproven (Detail: the missing
// witness).
type Establishment struct {
	Reason Reason
	Detail string
	Commit string
	Date   string // YYYY-MM-DD, UTC
}

// Evaluate evaluates every decision-level `supersedes` edge of spec that
// targets a spec object fragment, in decision and link order, then adds one
// completeness result per fragment that a superseded conflict naming spec
// (resolved_by: spec/<spec>) challenges with no matching edge on spec.
// ADR, whole-spec, and `exempts` edges are not this package's. est answers
// a carried candidate's establishment. A spec absent from recs is an error.
func Evaluate(ctx context.Context, recs *Records, spec string, est Establisher) ([]Result, error) {
	if recs == nil || est == nil {
		return nil, fmt.Errorf("objsupersede: Evaluate needs records and an establisher")
	}
	s := recs.Specs[spec]
	if s == nil {
		return nil, fmt.Errorf("objsupersede: spec/%s is not a decodable spec of the evaluated tree", spec)
	}
	var out []Result
	for _, d := range s.FM.Decisions {
		for _, l := range d.Links {
			if ref, ok := fragmentEdge(l); ok {
				out = append(out, recs.evaluateEdge(ctx, s, d.ID, l.Ref, ref, true, est))
			}
		}
	}
	return append(out, recs.completeness(s)...), nil
}

// fragmentEdge reports whether l is a `supersedes` edge to a spec object.
func fragmentEdge(l artifact.Link) (artifact.Ref, bool) {
	ref, err := artifact.ParseRef(l.Ref)
	if l.Type != artifact.LinkSupersedes || err != nil || ref.Kind != artifact.KindSpec || !ref.Fragment() {
		return artifact.Ref{}, false
	}
	return ref, true
}

// evaluateEdge applies SI-271's pin, SI-274(6), and design §5's ordered
// conditions to one edge. allowCarried false evaluates the new-replacement
// match only (SI-270's in-force check).
func (recs *Records) evaluateEdge(ctx context.Context, s *Spec, decision, edge string, ref artifact.Ref, allowCarried bool, est Establisher) Result {
	r := Result{Spec: s.Name, Decision: decision, Edge: edge, Outcome: Unresolved}
	fail := func(reason Reason) Result { r.Reason = reason; return r }
	if ref.Pinned() {
		return fail(ReasonPinned)
	}
	if len(recs.Failures) > 0 {
		r.Detail = strings.Join(recs.Failures, "; ")
		return fail(ReasonRecordsUndecodable)
	}
	t := recs.Specs[ref.Name]
	switch {
	case t == nil:
		return fail(ReasonTargetMissing)
	case !t.Closed():
		return fail(ReasonTargetNotClosed)
	case !artifact.DeclaredObjectIDs(t.FM)[ref.Object]:
		return fail(ReasonObjectNotDeclared)
	case !isCriterionOrDecision(t.FM, ref.Object):
		return fail(ReasonObjectNotTarget)
	}

	chain := recs.chain(s.Name)
	carried := allowCarried && recs.carriedCandidate(chain, decision, ref)
	namers := map[string]bool{s.Name: true}
	if carried {
		namers = map[string]bool{}
		for _, n := range chain[1:] {
			namers[n] = true
		}
	} else if c, x := recs.establishedByOther(ref, s.Name); c != nil {
		r.Conflict, r.Other = c.Name, x
		return fail(ReasonAlreadySuperseded)
	}

	c, named, reason := recs.conflictFor(ref, namers, chain)
	if c != nil {
		r.Conflict = c.Name
	}
	if reason != "" {
		r.Other = named
		return fail(reason)
	}
	if !carried {
		r.Outcome = ResolvedNew
		return r
	}
	if rev, pred, ok := recs.carriedSteps(chain, named, decision, ref); !ok {
		r.Other, r.Predecessor = rev, pred
		return fail(ReasonCarryMismatch)
	}
	r.Other = named
	e := est.Establishment(ctx, named, artifact.Ref{Kind: ref.Kind, Name: ref.Name, Object: ref.Object})
	if e.Reason != "" {
		r.Detail = e.Detail
		return fail(e.Reason)
	}
	r.Outcome, r.Since = ResolvedCarried, e.Date
	return r
}

// isCriterionOrDecision reports whether id names an acceptance criterion or
// a decision of fm (condition 4).
func isCriterionOrDecision(fm *artifact.SpecFrontmatter, id string) bool {
	for _, ac := range fm.AcceptanceCriteria {
		if ac.ID == id {
			return true
		}
	}
	for _, d := range fm.Decisions {
		if d.ID == id {
			return true
		}
	}
	return false
}

// decisionEdge reports whether fm declares decision id (any id when id is
// "") carrying the unpinned `supersedes` edge to object: the same link type
// and ref (SI-274(4)).
func decisionEdge(fm *artifact.SpecFrontmatter, id string, object artifact.Ref) bool {
	if object.Name == "" {
		return false
	}
	for _, d := range fm.Decisions {
		if id != "" && d.ID != id {
			continue
		}
		for _, l := range d.Links {
			if ref, ok := fragmentEdge(l); ok && ref == object {
				return true
			}
		}
	}
	return false
}

// chain is spec's whole-spec revision chain, spec first: each step follows
// the one artifact.WholeSpecSupersedesRefs predecessor, and a spec naming
// zero or several, a predecessor absent from the tree, or a cycle ends it
// (SI-274(9)).
func (recs *Records) chain(spec string) []string {
	out, seen := []string{spec}, map[string]bool{spec: true}
	for cur := recs.Specs[spec]; cur != nil; {
		refs := artifact.WholeSpecSupersedesRefs(cur.FM.Links)
		if len(refs) != 1 || seen[refs[0].Name] || recs.Specs[refs[0].Name] == nil {
			break
		}
		seen[refs[0].Name] = true
		out = append(out, refs[0].Name)
		cur = recs.Specs[refs[0].Name]
	}
	return out
}

// carriedCandidate is SI-273's selection (B1): S's one predecessor P
// declares the same decision with the same edge, and a superseded conflict
// challenging the object names a member of P's chain.
func (recs *Records) carriedCandidate(chain []string, decision string, object artifact.Ref) bool {
	if len(chain) < 2 || !decisionEdge(recs.Specs[chain[1]].FM, decision, object) {
		return false
	}
	for _, c := range recs.challengers(object) {
		if c.FM.Status == "superseded" && indexOf(chain[1:], resolvedBy(c)) >= 0 {
			return true
		}
	}
	return false
}

// establishedByOther is condition 5 (SI-272): a superseded conflict
// challenging object names a spec X other than spec, and X exists in the
// tree and carries, on a decision, an unpinned edge to object.
func (recs *Records) establishedByOther(object artifact.Ref, spec string) (*Conflict, string) {
	for _, c := range recs.challengers(object) {
		x := resolvedBy(c)
		if c.FM.Status == "superseded" && x != "" && x != spec && recs.Specs[x] != nil && decisionEdge(recs.Specs[x].FM, "", object) {
			return c, x
		}
	}
	return nil, ""
}

// conflictFor applies conditions 6-8 and SI-274(1)-(2): it returns the
// superseded conflict challenging object whose resolved_by is one of
// namers (for a carried candidate, the earliest in chain), the spec it
// names, and the first failing reason, if any.
func (recs *Records) conflictFor(object artifact.Ref, namers map[string]bool, chain []string) (*Conflict, string, Reason) {
	cs := recs.challengers(object)
	if len(cs) == 0 {
		return nil, "", ReasonNoConflict
	}
	var superseded []*Conflict
	for _, c := range cs {
		if c.FM.Status == "superseded" {
			superseded = append(superseded, c)
		}
	}
	if len(superseded) == 0 {
		return cs[0], "", ReasonConflictNotSuperseded
	}
	var match *Conflict
	for _, c := range superseded {
		if n := resolvedBy(c); namers[n] && (match == nil || indexOf(chain, n) > indexOf(chain, resolvedBy(match))) {
			match = c
		}
	}
	if match == nil {
		return superseded[0], resolvedBy(superseded[0]), ReasonResolvedByOther
	}
	named := resolvedBy(match)
	count := 0
	for _, c := range recs.Conflicts {
		if c.FM.Status == "superseded" && resolvedBy(c) == named && len(fragmentSpecs(c, object.Name)) > 0 {
			count++
		}
	}
	if count > 1 {
		return match, named, ReasonMultipleConflicts
	}
	if len(fragmentSpecs(match, "")) > 1 {
		return match, "", ReasonConflictSpansSpecs
	}
	return match, named, ""
}

// carriedSteps checks every step from chain[0] back to the establishing
// successor: the predecessor and the revision both declare the decision
// with the same edge, and the revision classifies it `carried`, `amended`,
// or `amended_advisory` (a story revision: the same id). It returns the
// first failing step (condition 9).
func (recs *Records) carriedSteps(chain []string, establisher, decision string, object artifact.Ref) (string, string, bool) {
	for i := 0; i < indexOf(chain, establisher); i++ {
		rev, pred := recs.Specs[chain[i]].FM, recs.Specs[chain[i+1]].FM
		if !decisionEdge(rev, decision, object) || !decisionEdge(pred, decision, object) || !classifiedCarried(rev, decision) {
			return chain[i], chain[i+1], false
		}
	}
	return "", "", true
}

func classifiedCarried(fm *artifact.SpecFrontmatter, id string) bool {
	if fm.Class == artifact.ClassStory {
		return true
	}
	if fm.Class != artifact.ClassFeature || fm.Supersession == nil {
		return false
	}
	for _, c := range fm.Supersession.Carried {
		if c == id {
			return true
		}
	}
	for _, n := range append(append([]artifact.SupersessionNote{}, fm.Supersession.Amended...), fm.Supersession.AmendedAdvisory...) {
		if n.ID == id {
			return true
		}
	}
	return false
}

// completeness is §5's completeness check for the issuing successor s: one
// result per fragment that a superseded conflict naming s challenges with no
// matching edge on s (a pinned challenge never matches, SI-274(7)). With
// undecodable records it is one unproven result (SI-274(6)).
func (recs *Records) completeness(s *Spec) []Result {
	if len(recs.Failures) > 0 {
		return []Result{{Spec: s.Name, Outcome: Unresolved, Reason: ReasonRecordsUndecodable, Detail: strings.Join(recs.Failures, "; ")}}
	}
	var out []Result
	for _, c := range recs.Conflicts {
		if c.FM.Status != "superseded" || resolvedBy(c) != s.Name {
			continue
		}
		for _, l := range c.FM.Links {
			ref, err := artifact.ParseRef(l.Ref)
			if l.Type != artifact.LinkChallenges || err != nil || !ref.Fragment() {
				continue
			}
			if ref.Pinned() || !decisionEdge(s.FM, "", ref) {
				out = append(out, Result{Spec: s.Name, Edge: l.Ref, Outcome: Unresolved, Reason: ReasonUnmatchedChallenge, Conflict: c.Name})
			}
		}
	}
	return out
}

// challengers returns, sorted by name, the conflicts with an unpinned
// fragment `challenges` link naming object.
func (recs *Records) challengers(object artifact.Ref) []*Conflict {
	var out []*Conflict
	for _, c := range recs.Conflicts {
		for _, l := range c.FM.Links {
			if ref, err := artifact.ParseRef(l.Ref); l.Type == artifact.LinkChallenges && err == nil && !ref.Pinned() &&
				ref.Kind == object.Kind && ref.Name == object.Name && ref.Object == object.Object {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

// fragmentSpecs returns the specs whose objects c's fragment challenges
// name, only spec when spec is not "".
func fragmentSpecs(c *Conflict, spec string) map[string]bool {
	out := map[string]bool{}
	for _, l := range c.FM.Links {
		if ref, err := artifact.ParseRef(l.Ref); l.Type == artifact.LinkChallenges && err == nil && ref.Fragment() && (spec == "" || ref.Name == spec) {
			out[ref.Name] = true
		}
	}
	return out
}

// resolvedBy returns the bare spec name a conflict's resolved_by names, or "".
func resolvedBy(c *Conflict) string {
	if ref, err := artifact.ParseRef(c.FM.ResolvedBy); err == nil && ref.Kind == artifact.KindSpec {
		return ref.Name
	}
	return ""
}

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}
