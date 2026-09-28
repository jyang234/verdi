package objsupersede

// The acceptance walk: the one computation of whether an establishing
// successor's supersession of an object is in force, and since when
// (SI-270 and SI-272 as amended after the whole-wave review, F-4 and F-1;
// SI-274(3); SI-275). History runs it for align and the gate, and the
// views' memo for the docs site and the board, so neither re-derives it.

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// walkSource is the default-branch history the acceptance walk reads.
// History implements it.
type walkSource interface {
	// walk lists, oldest first, the first-parent commits of the default
	// branch that change successor's or closed's spec.md in either zone or
	// the conflicts directory, from the first that holds successor (SI-270
	// as amended; SI-281's walk set, per (successor, closed spec), so a
	// target closed after the successor landed is seen at its archive
	// commit). Its fact is FactProven, naming that first commit, when the
	// branch holds successor; FactAbsent when it never did; and
	// FactUnproven, with the witness, when that cannot be read.
	walk(ctx context.Context, successor, closed string) ([]string, Fact)
	// position is a walked commit's place on the default branch's
	// first-parent chain: a later commit has a larger position.
	position(ctx context.Context, commit string) (int, error)
	// recordsAt reads the records of commit's tree.
	recordsAt(ctx context.Context, commit string) (*Records, error)
	// dated is commit's committer date as a proven fact, or the witness.
	dated(ctx context.Context, commit string) Fact
}

// engine answers establishments over one default-branch head. It
// memoizes each (successor, closed spec) walk list and walk (SI-270's
// point and SI-281's walk set are per (S_k, T)), each commit's records and
// date, and each answer at a walked commit, so one engine reads each at
// most once.
// It is not safe for concurrent use; History serializes its callers.
type engine struct {
	src     walkSource
	lists   map[string]walkList // per (successor, closed spec)
	walks   map[string]*walkState
	recs    map[string]recordsAt
	dates   map[string]Fact
	answers map[string]Establishment // per successor, object, and walked commit
	err     error                    // the first history answer outside FactState, or a defect
}

// walkList is one (successor, closed spec) walk list (walkSource): its
// commits, their positions, and their fact.
type walkList struct {
	commits []string
	pos     []int
	fact    Fact
}

// recordsAt is one commit's records, or the error reading them.
type recordsAt struct {
	recs *Records
	err  error
}

// walkState is one (successor, closed spec) walk, evaluated lazily and in
// order: evals[i] is commit i's evaluation, and the walk ends at the first
// commit where §3's match for (successor, closed) holds or cannot be
// proven. busy marks the evaluation in progress, which a nested question
// never needs (see before).
type walkState struct {
	successor, closed string
	list              walkList
	evals             []evaluation
	busy              bool
}

// evaluation is §3's match for a walk's (successor, closed) at one commit.
type evaluation struct {
	state   evalState
	witness string // evalUnproven: why
}

type evalState int

const (
	evalAbsent   evalState = iota // the successor is not in the commit's tree
	evalFails                     // the match fails
	evalHolds                     // the match holds: the acceptance point
	evalUnproven                  // the match cannot be proven here
)

// ended reports whether the walk has reached its point or an unproven
// commit.
func (w *walkState) ended() bool {
	n := len(w.evals)
	return n > 0 && (w.evals[n-1].state == evalHolds || w.evals[n-1].state == evalUnproven)
}

func newEngine(src walkSource) *engine {
	return &engine{src: src, lists: map[string]walkList{}, walks: map[string]*walkState{},
		recs: map[string]recordsAt{}, dates: map[string]Fact{}, answers: map[string]Establishment{}}
}

// establishment answers whether successor's supersession of object is in
// force on the default branch (Establisher): in force at any walked
// commit.
func (e *engine) establishment(ctx context.Context, successor string, object artifact.Ref) Establishment {
	return e.before(ctx, successor, object, math.MaxInt)
}

// before answers whether successor's supersession of object is in force
// at a walked commit strictly before position bound; every answer is
// before the head. The acceptance point of successor for object's closed
// spec T is the earliest walked commit at which successor is present and
// §3's match for (successor, T) holds (SI-275); object is established
// there only when its own edge is one of the match's new replacements,
// and then since that commit's committer date. The match at a commit
// reads condition 5 as of that commit: another successor's supersession
// in force strictly before it (SI-272 as amended), so a supersession in
// force is never unseated by a later one (design §4) and every nested
// question asks about an earlier commit than the one that asks it. Two
// successors whose matches for one object first hold at the same commit
// are each acceptance unproven, since neither is already superseded by
// the other and the rulings decide no order between them. No commit
// matching before bound is not in force, with the reason from the latest
// evaluated commit (not accepted when none before bound holds
// successor); an evaluation that cannot be proven (a record that does not
// decode, SI-274(6); an unreadable commit; another successor's unproven
// establishment, SI-274(3)) is acceptance unproven, since the point could
// be that commit.
func (e *engine) before(ctx context.Context, successor string, object artifact.Ref, bound int) Establishment {
	w := e.walkOf(ctx, successor, object.Name)
	switch w.list.fact.State {
	case FactAbsent:
		return Establishment{Reason: ReasonEstablisherNotAccepted}
	case FactUnproven:
		return Establishment{Reason: ReasonAcceptanceUnproven, Detail: w.list.fact.Witness}
	}
	if !e.advance(ctx, w, bound) {
		return Establishment{Reason: ReasonAcceptanceUnproven, Detail: fmt.Sprintf("objsupersede: spec/%s's walk was re-entered at a commit it is evaluating", successor)}
	}
	last := -1
	for i, ev := range w.evals {
		if w.list.pos[i] >= bound {
			break
		}
		switch ev.state {
		case evalUnproven:
			return Establishment{Reason: ReasonAcceptanceUnproven, Detail: ev.witness}
		case evalHolds:
			return e.answer(ctx, w, i, object)
		case evalFails:
			last = i
		}
	}
	switch {
	case last >= 0:
		return e.answer(ctx, w, last, object)
	case bound == math.MaxInt:
		// The walk's first commit holds successor, so this is reached only
		// if no commit's tree held it after all.
		return Establishment{Reason: ReasonAcceptanceUnproven, Detail: fmt.Sprintf("spec/%s is not in its acceptance commit's tree", successor)}
	}
	return Establishment{Reason: ReasonEstablisherNotAccepted}
}

// walkOf returns the (successor, closed) walk, created on first use.
func (e *engine) walkOf(ctx context.Context, successor, closed string) *walkState {
	key := successor + "\x00" + closed
	w, ok := e.walks[key]
	if !ok {
		w = &walkState{successor: successor, closed: closed, list: e.list(ctx, successor, closed)}
		e.walks[key] = w
	}
	return w
}

// list returns the (spec, closed) walk list with its commits' positions,
// read once; a list that does not start at its first commit or whose
// positions do not increase is unproven.
func (e *engine) list(ctx context.Context, spec, closed string) walkList {
	key := spec + "\x00" + closed
	if l, ok := e.lists[key]; ok {
		return l
	}
	commits, f := e.src.walk(ctx, spec, closed)
	l := walkList{commits: commits, fact: e.known(f, fmt.Sprintf("spec/%s's acceptance", spec))}
	if l.fact.State == FactProven {
		if len(commits) == 0 || commits[0] != l.fact.Commit {
			l.fact = unproven(fmt.Sprintf("spec/%s's walk does not start at the commit that first holds it", spec))
		}
		for i := 0; l.fact.State == FactProven && i < len(commits); i++ {
			p, err := e.src.position(ctx, commits[i])
			switch {
			case err != nil:
				l.fact = unproven(err.Error())
			case i > 0 && p <= l.pos[i-1]:
				l.fact = unproven(fmt.Sprintf("spec/%s's walk is not in first-parent order at %s", spec, commits[i]))
			}
			l.pos = append(l.pos, p)
		}
	}
	e.lists[key] = l
	return l
}

// advance evaluates w's commits before position bound, in order, until
// the walk ends. A nested question about a walk that is evaluating a
// commit always asks about an earlier one (before), so it never needs
// that walk to advance; if it did, advance fails closed.
func (e *engine) advance(ctx context.Context, w *walkState, bound int) bool {
	for !w.ended() && len(w.evals) < len(w.list.commits) && w.list.pos[len(w.evals)] < bound {
		if w.busy {
			e.defect(fmt.Errorf("objsupersede: spec/%s's walk for spec/%s was re-entered at %s", w.successor, w.closed, w.list.commits[len(w.evals)]))
			return false
		}
		w.busy = true
		ev := e.evaluate(ctx, w, len(w.evals))
		w.busy = false
		w.evals = append(w.evals, ev)
	}
	return true
}

// evaluate is w's match at its commit i, reading condition 5 as of it. A
// commit whose records cannot be read or decoded is unproven, its witness
// naming the commit (SI-274(3)), since it may lie before the point
// (SI-281(4)).
func (e *engine) evaluate(ctx context.Context, w *walkState, i int) evaluation {
	commit := w.list.commits[i]
	r := e.records(ctx, commit)
	switch {
	case r.err != nil:
		return evaluation{state: evalUnproven, witness: fmt.Sprintf("the records at commit %s cannot be read: %v", shortCommit(commit), r.err)}
	case len(r.recs.Failures) > 0:
		return evaluation{state: evalUnproven, witness: fmt.Sprintf("records do not decode at commit %s: %s", shortCommit(commit), strings.Join(r.recs.Failures, "; "))}
	}
	s := r.recs.Specs[w.successor]
	if s == nil {
		return evaluation{state: evalAbsent} // the successor left the branch here
	}
	switch st, witness := r.recs.stMatch(ctx, s, w.closed, e.asOf(w.list.pos[i])); st {
	case matchHolds:
		return evaluation{state: evalHolds}
	case matchUnproven:
		return evaluation{state: evalUnproven, witness: witness}
	}
	return evaluation{state: evalFails}
}

// asOf is condition 5's establisher at the commit at position pos:
// another successor's supersession in force strictly before it.
func (e *engine) asOf(pos int) Establisher { return asOf{e: e, bound: pos} }

// asOf answers establishments as of a walked commit (engine.before).
type asOf struct {
	e     *engine
	bound int
}

// Establishment implements Establisher.
func (a asOf) Establishment(ctx context.Context, successor string, object artifact.Ref) Establishment {
	return a.e.before(ctx, successor, object, a.bound)
}

// answer is object's establishment from w's commit i, memoized: in force
// since i's date when the match holds at i, object's own edge is a new
// replacement there, and no other successor ties it at i; otherwise not
// in force with object's reason at i, naming i so the reason never reads
// as a statement about the head (SI-281), or acceptance unproven.
func (e *engine) answer(ctx context.Context, w *walkState, i int, object artifact.Ref) Establishment {
	commit := w.list.commits[i]
	key := w.successor + "\x00" + object.String() + "\x00" + commit
	if a, ok := e.answers[key]; ok {
		return a
	}
	recs := e.records(ctx, commit).recs
	reason, detail := recs.inForceAt(ctx, w.successor, object, e.asOf(w.list.pos[i]))
	var a Establishment
	switch {
	case w.evals[i].state != evalHolds && reason != ReasonEstablisherNotInForce:
		// The match failed definitely at i, so object's answer there must
		// be a definite failure; anything else is this package's defect,
		// reported rather than passed.
		a = Establishment{Reason: ReasonAcceptanceUnproven, Detail: fmt.Sprintf("objsupersede: spec/%s's evaluation of %s at %s is inconsistent", w.successor, object, commit)}
	case reason == ReasonEstablisherNotInForce:
		a = Establishment{Reason: reason, Detail: fmt.Sprintf("as of commit %s, %s", shortCommit(commit), detail)}
	case reason != "":
		a = Establishment{Reason: reason, Detail: detail}
	default:
		a = e.dateOrTie(ctx, w, i, object)
	}
	e.answers[key] = a
	return a
}

// dateOrTie is object's in-force answer at w's point i: acceptance
// unproven when another successor's match for object also first holds at
// i (a tie, before's doc) or cannot be proven there, and otherwise in
// force since i's committer date.
func (e *engine) dateOrTie(ctx context.Context, w *walkState, i int, object artifact.Ref) Establishment {
	commit, pos := w.list.commits[i], w.list.pos[i]
	recs := e.records(ctx, commit).recs
	for _, x := range recs.rivals(object, w.successor) {
		xw := e.walkOf(ctx, x, object.Name)
		if xw.list.fact.State == FactUnproven {
			return Establishment{Reason: ReasonAcceptanceUnproven, Detail: fmt.Sprintf("spec/%s's establishment: %s", x, xw.list.fact.Witness)}
		}
		j := indexOf(xw.list.commits, commit)
		if j < 0 {
			continue // x's walk skips the commit: its match cannot first hold there
		}
		if !e.advance(ctx, xw, pos+1) {
			return Establishment{Reason: ReasonAcceptanceUnproven, Detail: fmt.Sprintf("objsupersede: spec/%s's walk was re-entered at a commit it is evaluating", x)}
		}
		if j >= len(xw.evals) {
			continue // x's walk ended before the commit
		}
		switch xw.evals[j].state {
		case evalUnproven:
			return Establishment{Reason: ReasonAcceptanceUnproven, Detail: fmt.Sprintf("spec/%s's establishment: %s", x, xw.evals[j].witness)}
		case evalHolds:
			if r, _ := recs.inForceAt(ctx, x, object, e.asOf(pos)); r == "" {
				return Establishment{Reason: ReasonAcceptanceUnproven, Detail: fmt.Sprintf("spec/%s and spec/%s first match for %s at the same commit %s, so neither takes effect before the other", w.successor, x, object, commit)}
			}
		}
	}
	d := e.date(ctx, commit)
	if d.State != FactProven {
		return Establishment{Reason: ReasonAcceptanceUnproven, Detail: d.Witness}
	}
	return Establishment{Commit: commit, Date: d.Date}
}

// records returns commit's records, read once.
func (e *engine) records(ctx context.Context, commit string) recordsAt {
	r, ok := e.recs[commit]
	if !ok {
		r.recs, r.err = e.src.recordsAt(ctx, commit)
		e.recs[commit] = r
	}
	return r
}

// date returns commit's committer date, read once.
func (e *engine) date(ctx context.Context, commit string) Fact {
	d, ok := e.dates[commit]
	if !ok {
		d = e.known(e.src.dated(ctx, commit), "commit "+commit+"'s date")
		e.dates[commit] = d
	}
	return d
}

// known holds a history answer to FactState's three values. Any other
// state is recorded as the engine's error and reads as unproven, never as
// a state it does not name.
func (e *engine) known(f Fact, what string) Fact {
	switch f.State {
	case FactProven, FactAbsent, FactUnproven:
		return f
	}
	err := fmt.Errorf("objsupersede: the history answered %s with unknown state %q", what, f.State)
	e.defect(err)
	return unproven(err.Error())
}

// defect records the engine's first error.
func (e *engine) defect(err error) {
	if e.err == nil {
		e.err = err
	}
}

// matchState is §3's match for (S, T) at one commit.
type matchState int

const (
	matchFails matchState = iota
	matchHolds
	matchUnproven
)

// stEdge is one edge of a successor to an object of a closed spec as
// §3's match for (S, T) reads it (SI-275): a carried candidate there
// (SI-273), which the match excludes, or a new replacement and its
// evaluation.
type stEdge struct {
	ref     artifact.Ref
	carried bool
	r       Result
}

// stEdges evaluates, in decision and link order, every fragment
// `supersedes` edge of s to an object of closed spec t. est answers
// condition 5 (SI-272 as amended).
func (recs *Records) stEdges(ctx context.Context, s *Spec, t string, est Establisher) []stEdge {
	chain := recs.chain(s.Name)
	var out []stEdge
	for _, d := range s.FM.Decisions {
		for _, l := range d.Links {
			ref, ok := fragmentEdge(l)
			if !ok || ref.Name != t {
				continue
			}
			if recs.carriedCandidate(chain, d.ID, ref) {
				out = append(out, stEdge{ref: ref, carried: true})
				continue
			}
			out = append(out, stEdge{ref: ref, r: recs.evaluateEdge(ctx, s, d.ID, l.Ref, ref, false, est)})
		}
	}
	return out
}

// gapsOver returns s's completeness results for fragments of closed spec
// t; a result whose edge does not parse counts, failing closed.
func (recs *Records) gapsOver(s *Spec, t string) []Result {
	var out []Result
	for _, r := range recs.completeness(s) {
		if ref, err := artifact.ParseRef(r.Edge); err != nil || ref.Name == t {
			out = append(out, r)
		}
	}
	return out
}

// unprovenEdge reports whether r is an edge left unresolved only because
// its history could not be proven (SI-274(3)).
func unprovenEdge(r Result) bool { return r.Reason == ReasonAcceptanceUnproven }

// stMatch is §3's whole match for successor s and closed spec t on recs
// (SI-275): every edge of s to an object of t that is not a carried
// candidate resolves new, and there is at least one, since §3's match is
// for a successor that issues replacements of t; and completeness for s
// over t holds. It fails when any of these fails definitely, is unproven
// (with the witness) when none does but an edge's history is unproven,
// and holds otherwise.
func (recs *Records) stMatch(ctx context.Context, s *Spec, t string, est Establisher) (matchState, string) {
	issued, witness := 0, ""
	for _, e := range recs.stEdges(ctx, s, t, est) {
		switch {
		case e.carried:
		case e.r.Outcome == ResolvedNew:
			issued++
		case unprovenEdge(e.r):
			if witness == "" {
				witness = e.r.Detail
			}
		default:
			return matchFails, ""
		}
	}
	switch {
	case len(recs.gapsOver(s, t)) > 0 || (issued == 0 && witness == ""):
		return matchFails, ""
	case witness != "":
		return matchUnproven, witness
	}
	return matchHolds, ""
}

// inForceAt is object's answer on recs, one tree's records: "" when §3's
// match for successor and object's closed spec T holds and object's own
// edge is one of its new replacements (SI-275). Otherwise it is "not in
// force" with the first definite failure, in order: an edge (object's own
// edge carried from a predecessor, or a new edge that does not resolve),
// no new edge to object, then completeness over T; or, when nothing fails
// definitely but an edge's history is unproven, "acceptance unproven"
// with its witness. Records that do not decode are acceptance unproven
// (SI-274(6)). est answers condition 5.
func (recs *Records) inForceAt(ctx context.Context, successor string, object artifact.Ref, est Establisher) (Reason, string) {
	if len(recs.Failures) > 0 {
		return ReasonAcceptanceUnproven, "records do not decode at the acceptance commit: " + strings.Join(recs.Failures, "; ")
	}
	s := recs.Specs[successor]
	if s == nil {
		return ReasonAcceptanceUnproven, fmt.Sprintf("spec/%s is not in its acceptance commit's tree", successor)
	}
	own, witness := false, ""
	for _, e := range recs.stEdges(ctx, s, object.Name, est) {
		switch {
		case e.carried:
			if e.ref == object {
				return ReasonEstablisherNotInForce, fmt.Sprintf("spec/%s carries its edge to %s from its predecessor; it issues no new replacement", successor, object)
			}
			continue
		case e.r.Outcome != ResolvedNew && !unprovenEdge(e.r):
			return ReasonEstablisherNotInForce, textOrError(e.r)
		case unprovenEdge(e.r) && witness == "":
			witness = e.r.Detail
		}
		own = own || e.ref == object
	}
	if !own {
		return ReasonEstablisherNotInForce, fmt.Sprintf("spec/%s carries no edge to %s", successor, object)
	}
	if gaps := recs.gapsOver(s, object.Name); len(gaps) > 0 {
		return ReasonEstablisherNotInForce, textOrError(gaps[0])
	}
	if witness != "" {
		return ReasonAcceptanceUnproven, witness
	}
	return "", ""
}

// shortCommit is how a reason names the commit it was evaluated at
// (SI-281): a full commit id's 12-hex prefix, the stamp lines' short form
// (internal/specdoc); a shorter name is returned whole.
func shortCommit(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

// textOrError renders a result this package built; a rendering error is
// reported inline, never dropped.
func textOrError(r Result) string {
	text, err := r.Text()
	if err != nil {
		return err.Error()
	}
	return text
}
