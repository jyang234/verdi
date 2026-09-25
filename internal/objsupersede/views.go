package objsupersede

// The surface views (design §4, §6; SI-263, SI-278): what the docs site
// and the board add to a closed spec's acceptance criterion or decision
// (ObjectView) and to a successor's decision (DecisionView), computed once
// per tree (Index) and rendered here, so the surfaces cannot drift: they
// add markup only. The views read records and never write them, and they
// never alter an object's own text, which the surface renders unchanged.

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// ObjectState classifies an ObjectView.
type ObjectState string

const (
	// ObjectNotSuperseded: the records establish no supersession, and the
	// object renders as before with no line added (design §6).
	ObjectNotSuperseded ObjectState = "not-superseded"
	// ObjectSuperseded: one establishing successor's supersession is in
	// force (SI-261, SI-270).
	ObjectSuperseded ObjectState = "superseded"
	// ObjectUnproven: whether the object is superseded cannot be proven
	// (SI-270, SI-274(3), (6), (8)); Witness says why. It is never shown
	// as superseded.
	ObjectUnproven ObjectState = "unproven"
)

// Carry is a superseded object's carrying state: what the head of the
// establishing successor's whole-spec revision chain does with the
// replacement (design §3, §6; SI-265, SI-274(5)).
type Carry string

const (
	// CarryNone: the establishing successor heads its chain and keeps its
	// edge; nothing is added.
	CarryNone Carry = ""
	// CarryCarried: the head, a later accepted revision, carries it
	// (SI-273's carry check).
	CarryCarried Carry = "carried"
	// CarryDropped: the head does not carry it; the object stays
	// superseded (§4).
	CarryDropped Carry = "dropped"
	// CarryUnproven: the chain branches, or the head is not proven
	// accepted or is absent; Witness says which, and none is picked.
	CarryUnproven Carry = "unproven"
)

// ObjectView is what a surface adds to a closed spec's acceptance
// criterion or decision. Refs are canonical and unpinned, for links.
type ObjectView struct {
	Object string // "spec/T#o"
	State  ObjectState
	// Superseded: the establishing successor's deciding decision
	// ("spec/S_k#dc-x", from its acceptance commit), the conflict that
	// established it ("conflict/<name>"), and S_k's acceptance date.
	By, Conflict, Since string
	// Superseded: T's closed date, YYYY-MM-DD; when it is not proven,
	// Closed is "" and ClosedWitness says why. No date is invented.
	Closed, ClosedWitness string
	Carry                 Carry
	Revision              string   // the revision a carry state names: "spec/S_n"
	Heads                 []string // a branching chain's heads, sorted (SI-274(5))
	Witness               string   // why the object, or its carry, is unproven
}

// Lines renders the view's design §6 lines in display order: none when the
// object is not superseded; "governed spec/T's completed work (closed
// <date>)", "superseded since <date> by spec/S#<decision-id>", and the
// carrying line when it is; "supersession unproven: <witness>" when that is
// unproven. A view missing a name, date, or witness its text needs, or
// naming a ref of the wrong shape (an unpinned spec object, decision, or
// revision ref), is an error, never a rendering.
func (v ObjectView) Lines() ([]string, error) {
	switch v.State {
	case ObjectNotSuperseded:
		return nil, nil
	case ObjectUnproven:
		if v.Witness == "" {
			return nil, fmt.Errorf("objsupersede: the unproven view of %s has no witness", v.Object)
		}
		return []string{"supersession unproven: " + v.Witness}, nil
	case ObjectSuperseded:
	default:
		return nil, fmt.Errorf("objsupersede: unknown object state %q", v.State)
	}
	ref, ok := specRef(v.Object)
	if !ok || !ref.Fragment() {
		return nil, fmt.Errorf("objsupersede: %q is not an object ref", v.Object)
	}
	// A decision id is dc-<slug> (02 §Object model; artifact.Decision).
	if by, ok := specRef(v.By); !ok || !strings.HasPrefix(by.Object, "dc-") || v.Conflict == "" || !isDay(v.Since) {
		return nil, fmt.Errorf("objsupersede: the view of %s lacks its deciding decision (spec/S#dc-x), conflict, or YYYY-MM-DD date", v.Object)
	}
	closed := v.Closed
	if !isDay(closed) {
		if closed != "" || v.ClosedWitness == "" {
			return nil, fmt.Errorf("objsupersede: the view of %s has neither a YYYY-MM-DD closing date nor a witness", v.Object)
		}
		closed = "date unproven: " + v.ClosedWitness
	}
	out := []string{
		// vocab:identity — design §6 / SI-263 binding surface wording: lifecycle ids, not display prose
		fmt.Sprintf("governed %s's completed work (closed %s)", artifact.Ref{Kind: ref.Kind, Name: ref.Name}, closed),
		// vocab:identity — design §6 / SI-263 binding surface wording: lifecycle ids, not display prose
		fmt.Sprintf("superseded since %s by %s", v.Since, v.By),
	}
	rev, ok := specRef(v.Revision)
	revision := ok && !rev.Fragment()
	switch {
	case v.Carry == CarryNone:
		return out, nil
	case v.Carry == CarryCarried && revision:
		return append(out, "carried by "+v.Revision), nil
	case v.Carry == CarryDropped && revision:
		return append(out, fmt.Sprintf("no longer carried by the current revision (%s)", v.Revision)), nil
	case v.Carry == CarryUnproven && v.Witness != "":
		return append(out, "carrying unproven: "+v.Witness), nil
	}
	return nil, fmt.Errorf("objsupersede: the view of %s has carry %q without its revision (spec/S) or witness", v.Object, v.Carry)
}

// specRef parses s as an unpinned spec ref, the only form a view names
// (design §6: refs for links). ok is false for anything else.
func specRef(s string) (artifact.Ref, bool) {
	ref, err := artifact.ParseRef(s)
	return ref, err == nil && ref.Kind == artifact.KindSpec && !ref.Pinned()
}

// DecisionState classifies a DecisionView.
type DecisionState string

const (
	// DecisionInForce: the supersession is in force (SI-261, SI-270).
	DecisionInForce DecisionState = "in-force"
	// DecisionProposed: the decision's spec is not accepted and the
	// records match (design §3, SI-275's whole match); it takes effect
	// when the spec is accepted.
	DecisionProposed DecisionState = "proposed"
	// DecisionNotEstablished: the records do not establish it; Reason
	// says why, in the core's vocabulary (design §5, SI-274).
	DecisionNotEstablished DecisionState = "not-established"
)

// DecisionView is what a surface adds to a decision for one of its
// fragment `supersedes` edges to a spec object. Refs are canonical, for
// links.
type DecisionView struct {
	Decision string // "spec/S#dc-x"
	Edge     string // the edge's ref, as written
	Object   string // the edge's object, unpinned: "spec/T#o"
	State    DecisionState
	// Conflict is "conflict/<name>": for a new replacement in force, the
	// conflict that established it, as at its spec's acceptance commit
	// (SI-279); otherwise the one the evaluation rests on, when it names
	// one.
	Conflict string
	// Carried: in force or proposed, the edge carries a replacement an
	// earlier revision established (SI-265). Establisher and Since name
	// the establishing successor ("spec/S_k", S itself for a new
	// replacement in force) and its acceptance date.
	Carried            bool
	Establisher, Since string
	Reason             string // not established: the reason's text
}

// Lines renders the view's design §6 lines: "supersedes spec/T#<object-id>"
// in force, or "proposed — supersedes spec/T#<object-id> when spec/S is
// accepted", each followed for a carried edge by design §5's "carries the
// replacement established by spec/S_k (conflict/<name>, since <date>)";
// or "supersession not established: <reason>". A view missing what its
// text needs, or whose object is not an unpinned spec object ref, is an
// error, never a rendering.
func (v DecisionView) Lines() ([]string, error) {
	switch v.State {
	case DecisionNotEstablished:
		if v.Reason == "" {
			return nil, fmt.Errorf("objsupersede: the view of %s's edge %s has no reason", v.Decision, v.Edge)
		}
		return []string{"supersession not established: " + v.Reason}, nil
	case DecisionInForce, DecisionProposed:
	default:
		return nil, fmt.Errorf("objsupersede: unknown decision state %q", v.State)
	}
	if o, ok := specRef(v.Object); !ok || !o.Fragment() {
		return nil, fmt.Errorf("objsupersede: the view of %s has no object ref (spec/T#o): %q", v.Decision, v.Object)
	}
	first := "supersedes " + v.Object
	if v.State == DecisionProposed {
		ref, err := artifact.ParseRef(v.Decision)
		if err != nil {
			return nil, fmt.Errorf("objsupersede: decision %q: %w", v.Decision, err)
		}
		// vocab:identity — design §6 / SI-263 binding surface wording: lifecycle ids, not display prose
		first = fmt.Sprintf("proposed — supersedes %s when %s is accepted", v.Object, artifact.Ref{Kind: ref.Kind, Name: ref.Name})
	}
	if !v.Carried {
		return []string{first}, nil
	}
	carries, err := Result{Outcome: ResolvedCarried, Other: strings.TrimPrefix(v.Establisher, "spec/"),
		Conflict: strings.TrimPrefix(v.Conflict, "conflict/"), Since: v.Since}.Text()
	if err != nil {
		return nil, err
	}
	return []string{first, carries}, nil
}

// Index is every object view and decision view of one tree (NewIndex).
// Its lookups are safe for concurrent use.
type Index struct {
	objects   map[string]ObjectView     // superseded or unproven, by "spec/T#o"
	decisions map[string][]DecisionView // by "spec/S#dc-x", in link order
}

// NewIndex computes every view of recs, one tree's records, against h,
// the default branch's history. A default-branch surface passes the
// default branch's records (design §6); a design-branch surface passes its
// branch's, so its own decisions read proposed. Each spec's acceptance and
// evaluation, each successor's acceptance-commit records, and each closed
// spec's closed date is read once. It never mutates recs.
func NewIndex(ctx context.Context, recs *Records, h History) (*Index, error) {
	return newIndex(ctx, recs, h)
}

// Object returns the view of spec's object id; an object the records do
// not supersede is not superseded.
func (x *Index) Object(spec, id string) ObjectView {
	key := objectRef(spec, id)
	v, ok := x.objects[key]
	if !ok {
		return ObjectView{Object: key, State: ObjectNotSuperseded}
	}
	v.Heads = slices.Clone(v.Heads)
	return v
}

// Decisions returns the views of spec's decision id, one per fragment
// `supersedes` edge in link order, or nil.
func (x *Index) Decisions(spec, id string) []DecisionView {
	return slices.Clone(x.decisions[objectRef(spec, id)])
}

// viewHistory is the default-branch history the views read; History
// implements it.
type viewHistory interface {
	Acceptance(ctx context.Context, spec string) Fact
	Closed(ctx context.Context, t *Spec) Fact
	recordsAt(ctx context.Context, commit string) (*Records, error)
}

func newIndex(ctx context.Context, recs *Records, h viewHistory) (*Index, error) {
	if recs == nil {
		return nil, fmt.Errorf("objsupersede: NewIndex needs records")
	}
	m := &memo{recs: recs, h: h, acc: map[string]Fact{}, at: map[string]accepted{}, closed: map[string]Fact{}, eval: map[string][]Result{}}
	x := &Index{objects: map[string]ObjectView{}, decisions: map[string][]DecisionView{}}
	names := make([]string, 0, len(recs.Specs))
	for name := range recs.Specs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		res, err := Evaluate(ctx, recs, name, m)
		if err != nil {
			return nil, err
		}
		m.eval[name] = res
		for _, r := range res {
			if r.Decision == "" {
				continue
			}
			v, err := m.decisionView(ctx, r)
			if err != nil {
				return nil, err
			}
			x.decisions[v.Decision] = append(x.decisions[v.Decision], v)
		}
	}
	for _, name := range names { // every spec is evaluated above; carry reads m.eval
		t := recs.Specs[name]
		if !t.Closed() {
			continue
		}
		for _, id := range criteriaAndDecisions(t.FM) {
			if v := m.objectView(ctx, t, id); v.State != ObjectNotSuperseded {
				x.objects[v.Object] = v
			}
		}
	}
	if m.err != nil {
		return nil, m.err
	}
	return x, nil
}

// memo answers one index build's history questions, each once (lane L3c
// item 3): acceptance per spec, the acceptance commit's records per
// successor (an establishment's cost), and the closed date per closed
// spec. It is the Establisher Evaluate consults.
type memo struct {
	recs   *Records
	h      viewHistory
	acc    map[string]Fact     // acceptance, per spec
	at     map[string]accepted // the acceptance commit's records, per successor
	closed map[string]Fact     // closed date, per closed spec
	eval   map[string][]Result // Evaluate, per spec
	err    error               // the first history answer outside FactState (known)
}

type accepted struct {
	recs *Records
	err  error
}

func (m *memo) acceptance(ctx context.Context, spec string) Fact {
	f, ok := m.acc[spec]
	if !ok {
		f = m.known(m.h.Acceptance(ctx, spec), fmt.Sprintf("spec/%s's acceptance", spec))
		m.acc[spec] = f
	}
	return f
}

// known holds a history answer to FactState's three values. Any other
// state fails the index closed: NewIndex returns it as an error, and until
// then it reads as unproven, never as a state it does not name.
func (m *memo) known(f Fact, what string) Fact {
	switch f.State {
	case FactProven, FactAbsent, FactUnproven:
		return f
	}
	err := fmt.Errorf("objsupersede: the history answered %s with unknown state %q", what, f.State)
	if m.err == nil {
		m.err = err
	}
	return unproven(err.Error())
}

// accepted returns the records of successor's acceptance commit, read
// once; empty when its acceptance is not proven.
func (m *memo) accepted(ctx context.Context, successor string) accepted {
	a, ok := m.at[successor]
	if !ok {
		if acc := m.acceptance(ctx, successor); acc.State == FactProven {
			a.recs, a.err = m.h.recordsAt(ctx, acc.Commit)
		}
		m.at[successor] = a
	}
	return a
}

// Establishment implements Establisher, as History does, over the memo's
// acceptance facts and acceptance-commit records.
func (m *memo) Establishment(ctx context.Context, successor string, object artifact.Ref) Establishment {
	a := m.accepted(ctx, successor)
	return establishment(ctx, m.acceptance(ctx, successor), a.recs, a.err, successor, object)
}

func (m *memo) closedDate(ctx context.Context, t *Spec) Fact {
	f, ok := m.closed[t.Name]
	if !ok {
		// vocab:identity — SI-270's closed-date fact named in an operational error: a lifecycle id, not display prose
		f = m.known(m.h.Closed(ctx, t), fmt.Sprintf("spec/%s's closed date", t.Name))
		m.closed[t.Name] = f
	}
	return f
}

// decisionView is the view of one evaluated edge. A new replacement is in
// force when its establishment is (inForce), whatever this tree's
// evaluation says; before its spec's acceptance it is proposed only while
// §3's whole match for its spec and closed spec holds in this tree
// (SI-275), since a record set that does not match is never shown as a
// supersession (§6). A carried edge is in force once its own revision is
// accepted, and proposed before.
func (m *memo) decisionView(ctx context.Context, r Result) (DecisionView, error) {
	ref, err := artifact.ParseRef(r.Edge)
	if err != nil {
		return DecisionView{}, fmt.Errorf("objsupersede: edge %q: %w", r.Edge, err)
	}
	object := artifact.Ref{Kind: ref.Kind, Name: ref.Name, Object: ref.Object}
	v := DecisionView{Decision: objectRef(r.Spec, r.Decision), Edge: r.Edge, Object: object.String()}
	if e, ok := m.inForce(ctx, r, ref, object); ok {
		// The conflict as it stood at the acceptance commit (SI-279), where
		// inForceAt proved the one superseded conflict naming r.Spec.
		c := m.accepted(ctx, r.Spec).recs.namedBy(object, r.Spec)
		v.State, v.Conflict, v.Establisher, v.Since = DecisionInForce, "conflict/"+c.Name, "spec/"+r.Spec, e.Date
		return v, nil
	}
	if r.Conflict != "" {
		v.Conflict = "conflict/" + r.Conflict
	}
	fail := r
	switch r.Outcome {
	case ResolvedCarried:
		switch acc := m.acceptance(ctx, r.Spec); acc.State {
		case FactProven, FactAbsent:
			v.State = DecisionProposed
			if acc.State == FactProven {
				v.State = DecisionInForce
			}
			v.Carried, v.Establisher, v.Since = true, "spec/"+r.Other, r.Since
			return v, nil
		default:
			fail = Result{Spec: r.Spec, Edge: r.Edge, Outcome: Unresolved, Reason: ReasonAcceptanceUnproven, Detail: acc.Witness}
		}
	case ResolvedNew: // not in force (inForce): proposed, or not established
		e := checked(r.Spec, m.Establishment(ctx, r.Spec, object))
		if e.Reason == ReasonEstablisherNotAccepted {
			v.State = DecisionProposed
			if reason, detail := m.recs.inForceAt(ctx, r.Spec, object); reason != "" {
				v.State, v.Reason = DecisionNotEstablished, detail
			}
			return v, nil
		}
		fail = Result{Spec: r.Spec, Edge: r.Edge, Outcome: Unresolved, Reason: e.Reason, Other: r.Spec, Detail: e.Detail}
	}
	text, err := fail.Text()
	if err != nil {
		return DecisionView{}, err
	}
	v.State, v.Reason = DecisionNotEstablished, text
	return v, nil
}

// inForce reports whether r's edge is its spec's own replacement in force:
// not a carried candidate in this tree (SI-273), whose spec's
// establishment of object held at its acceptance commit (SI-270, SI-275).
// A supersession in force is permanent (design §4, SI-261), so a later
// record of this tree, a proposed rival (condition 5) or a later
// duplicate conflict (SI-274(1)), never unseats it or names the rival.
// A pinned edge is never resolved (SI-271), and records that do not decode
// leave every edge of this tree dependent on them (SI-274(6)): neither is
// read in force.
func (m *memo) inForce(ctx context.Context, r Result, edge, object artifact.Ref) (Establishment, bool) {
	// A carried result is always a carried candidate here: Evaluate chose it
	// by the same check over the same records.
	if edge.Pinned() || len(m.recs.Failures) > 0 || m.recs.carriedCandidate(m.recs.chain(r.Spec), r.Decision, object) {
		return Establishment{}, false
	}
	e := checked(r.Spec, m.Establishment(ctx, r.Spec, object))
	return e, e.Reason == ""
}

// objectView is the view of t's object id: superseded when exactly one
// successor a superseded conflict challenging it names is in force at its
// acceptance (SI-270, SI-275), unproven when more than one is (SI-274(8))
// or any one's acceptance is unproven, and otherwise not superseded. Any
// undecodable record leaves every closed spec's object unproven
// (SI-274(6)).
func (m *memo) objectView(ctx context.Context, t *Spec, id string) ObjectView {
	o := artifact.Ref{Kind: artifact.KindSpec, Name: t.Name, Object: id}
	v := ObjectView{Object: o.String(), State: ObjectNotSuperseded}
	unproven := func(witness string) ObjectView {
		v.State, v.Witness = ObjectUnproven, witness
		return v
	}
	if len(m.recs.Failures) > 0 {
		return unproven(textOrError(Result{Outcome: Unresolved, Reason: ReasonRecordsUndecodable, Detail: strings.Join(m.recs.Failures, "; ")}))
	}
	var inForce []string
	var est Establishment
	witness := ""
	for _, s := range m.recs.successorsNaming(o) {
		switch e := checked(s, m.Establishment(ctx, s, o)); e.Reason {
		case "":
			inForce, est = append(inForce, s), e
		case ReasonAcceptanceUnproven:
			if witness == "" {
				witness = fmt.Sprintf("spec/%s: %s", s, textOrError(Result{Outcome: Unresolved, Reason: e.Reason, Detail: e.Detail}))
			}
		}
	}
	switch {
	case len(inForce) > 1:
		return unproven("more than one successor's supersession is in force: spec/" + strings.Join(inForce, ", spec/"))
	case witness != "":
		return unproven(witness)
	case len(inForce) == 0:
		return v
	}
	sk := inForce[0]
	// In force: inForceAt proved, on these records, a decision of sk with
	// the new edge to o and the one superseded conflict naming sk for it.
	at := m.accepted(ctx, sk).recs
	v.State, v.Since = ObjectSuperseded, est.Date
	v.By = objectRef(sk, decidingDecision(at.Specs[sk].FM, o))
	v.Conflict = "conflict/" + at.namedBy(o, sk).Name
	switch c := m.closedDate(ctx, t); c.State {
	case FactProven:
		v.Closed = c.Date
	case FactUnproven:
		v.ClosedWitness = c.Witness
	default:
		// vocab:identity — SI-270's closed-date witness: a lifecycle id, not display prose
		v.ClosedWitness = fmt.Sprintf("no closed baseline of spec/%s on the default branch", t.Name)
	}
	m.carry(ctx, &v, sk, o)
	return v
}

// carry sets v's carrying state from the head of sk's revision chain in
// this tree: carried when that head, a later accepted revision, carries
// the edge by SI-273's carry check (Evaluate reads it carried); dropped
// when it does not, or when sk heads the chain and no longer holds the
// edge; unproven when the chain branches (SI-274(5)), the head is not
// proven accepted, or sk is absent from this tree.
func (m *memo) carry(ctx context.Context, v *ObjectView, sk string, o artifact.Ref) {
	s := m.recs.Specs[sk]
	if s == nil {
		v.Carry, v.Revision, v.Witness = CarryUnproven, "spec/"+sk, fmt.Sprintf("spec/%s is not in the evaluated tree", sk)
		return
	}
	heads := m.recs.heads(sk)
	if len(heads) > 1 {
		for _, h := range heads {
			v.Heads = append(v.Heads, "spec/"+h)
		}
		v.Carry, v.Witness = CarryUnproven, fmt.Sprintf("the revision chain of spec/%s branches: %s", sk, strings.Join(v.Heads, ", "))
		return
	}
	head := heads[0]
	if head == sk {
		if !decisionEdge(s.FM, "", o) {
			v.Carry, v.Revision = CarryDropped, "spec/"+sk
		}
		return
	}
	v.Revision = "spec/" + head
	switch acc := m.acceptance(ctx, head); acc.State {
	case FactAbsent:
		v.Carry, v.Witness = CarryUnproven, textOrError(Result{Outcome: Unresolved, Reason: ReasonEstablisherNotAccepted, Other: head})
		return
	case FactUnproven:
		v.Carry, v.Witness = CarryUnproven, textOrError(Result{Outcome: Unresolved, Reason: ReasonAcceptanceUnproven, Detail: acc.Witness})
		return
	}
	v.Carry = CarryDropped
	for _, r := range m.eval[head] {
		if ref, err := artifact.ParseRef(r.Edge); err == nil && r.Outcome == ResolvedCarried && ref == o {
			v.Carry = CarryCarried
		}
	}
}

// heads returns, sorted, the heads of spec's whole-spec revision chain
// forward: the revisions reached from spec by following every spec whose
// one predecessor ref (artifact.WholeSpecSupersedesRefs) names the
// current one and is in the tree, the relation chain walks back
// (SI-274(9)); spec itself when no revision names it. More than one head
// is a branching chain (SI-274(5)).
func (recs *Records) heads(spec string) []string {
	next := map[string][]string{}
	for name, s := range recs.Specs {
		if refs := artifact.WholeSpecSupersedesRefs(s.FM.Links); len(refs) == 1 && recs.Specs[refs[0].Name] != nil {
			next[refs[0].Name] = append(next[refs[0].Name], name)
		}
	}
	var out []string
	seen := map[string]bool{spec: true}
	var walk func(string)
	walk = func(s string) {
		leaf := true
		for _, r := range next[s] {
			if !seen[r] {
				seen[r], leaf = true, false
				walk(r)
			}
		}
		if leaf {
			out = append(out, s)
		}
	}
	walk(spec)
	sort.Strings(out)
	return out
}

// successorsNaming returns, sorted, the specs that superseded conflicts
// challenging object name in resolved_by.
func (recs *Records) successorsNaming(object artifact.Ref) []string {
	var out []string
	for _, c := range recs.challengers(object) {
		if s := resolvedBy(c); c.superseded() && s != "" && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// decidingDecision returns the first decision of fm, in declaration
// order, carrying an edge to object.
func decidingDecision(fm *artifact.SpecFrontmatter, object artifact.Ref) string {
	for _, d := range fm.Decisions {
		if decisionEdge(fm, d.ID, object) {
			return d.ID
		}
	}
	return ""
}

// criteriaAndDecisions returns fm's acceptance criterion and decision ids,
// the objects a closed-spec object supersession may target (design §2).
func criteriaAndDecisions(fm *artifact.SpecFrontmatter) []string {
	var out []string
	for _, ac := range fm.AcceptanceCriteria {
		out = append(out, ac.ID)
	}
	for _, d := range fm.Decisions {
		out = append(out, d.ID)
	}
	return out
}

func objectRef(spec, id string) string {
	return artifact.Ref{Kind: artifact.KindSpec, Name: spec, Object: id}.String()
}
