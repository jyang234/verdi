package objsupersede

// The acceptance walk: the one computation of whether an establishing
// successor's supersession of an object is in force, and since when
// (SI-270 as amended after the whole-wave review, F-4; SI-274(3); SI-275).
// History runs it for align and the gate, and the views' memo for the
// docs site and the board, so neither re-derives it.

import (
	"context"
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// walkSource is the default-branch history the acceptance walk reads.
// History implements it.
type walkSource interface {
	// walk lists, oldest first, the first-parent commits of the default
	// branch that change spec's spec.md in either zone or the conflicts
	// directory, from the first that holds spec (SI-270 as amended). Its
	// fact is FactProven, naming that first commit, when the branch holds
	// spec; FactAbsent when it never did; and FactUnproven, with the
	// witness, when that cannot be read.
	walk(ctx context.Context, spec string) ([]string, Fact)
	// recordsAt reads the records of commit's tree.
	recordsAt(ctx context.Context, commit string) (*Records, error)
	// dated is commit's committer date as a proven fact, or the witness.
	dated(ctx context.Context, commit string) Fact
}

// engine answers establishments over one default-branch head. It
// memoizes each walk list per spec, each (successor, closed spec) walk
// (SI-270's point is per (S_k, T)), each commit's records, and each
// commit's date, so one engine reads each at most once. It is not safe for
// concurrent use; History serializes its callers.
type engine struct {
	src   walkSource
	lists map[string]walkList
	walks map[string]*walkState
	recs  map[string]recordsAt
	dates map[string]Fact
	err   error // the first history answer outside FactState
}

// walkList is one spec's walk: its commits and their fact (walkSource).
type walkList struct {
	commits []string
	fact    Fact
}

// recordsAt is one commit's records, or the error reading them.
type recordsAt struct {
	recs *Records
	err  error
}

// walkState is one (successor, closed spec) walk, advanced lazily: the
// commits before next are evaluated, and it stops at the first commit
// where §3's match for (successor, closed) holds (held) or cannot be
// proven (stopped).
type walkState struct {
	successor, closed string
	list              walkList
	next              int    // list.commits[:next] are evaluated
	held              int    // the first commit where the match holds, or -1
	stopped           string // the witness of an unproven evaluation, or ""
	last              int    // the latest evaluated commit holding successor, or -1
}

func newEngine(src walkSource) *engine {
	return &engine{src: src, lists: map[string]walkList{}, walks: map[string]*walkState{}, recs: map[string]recordsAt{}, dates: map[string]Fact{}}
}

// establishment answers whether successor's supersession of object is in
// force (Establisher). The acceptance point of successor for object's
// closed spec T is the earliest walked commit at which successor is
// present and §3's match for (successor, T) holds (SI-275); object is
// established there only when its own edge is one of the match's new
// replacements, and then since that commit's committer date. No commit
// matching is not in force, with the reason from the latest evaluated
// commit; an evaluation that cannot be proven (a record that does not
// decode, SI-274(6); an unreadable commit) stops the walk as acceptance
// unproven, since the point could be that commit. A spec never on the
// default branch is not accepted.
func (e *engine) establishment(ctx context.Context, successor string, object artifact.Ref) Establishment {
	w := e.walkOf(ctx, successor, object.Name)
	switch w.list.fact.State {
	case FactAbsent:
		return Establishment{Reason: ReasonEstablisherNotAccepted}
	case FactUnproven:
		return Establishment{Reason: ReasonAcceptanceUnproven, Detail: w.list.fact.Witness}
	}
	e.advance(ctx, w)
	switch {
	case w.stopped != "":
		return Establishment{Reason: ReasonAcceptanceUnproven, Detail: w.stopped}
	case w.held >= 0:
		return e.at(ctx, w, w.held, object)
	case w.last >= 0:
		return e.at(ctx, w, w.last, object)
	}
	// The walk's first commit holds successor, so it is evaluated: this is
	// reached only if every evaluated commit lacked it.
	return Establishment{Reason: ReasonAcceptanceUnproven, Detail: fmt.Sprintf("spec/%s is not in its acceptance commit's tree", successor)}
}

// walkOf returns the (successor, closed) walk, created on first use.
func (e *engine) walkOf(ctx context.Context, successor, closed string) *walkState {
	key := successor + "\x00" + closed
	w, ok := e.walks[key]
	if !ok {
		w = &walkState{successor: successor, closed: closed, list: e.list(ctx, successor), held: -1, last: -1}
		e.walks[key] = w
	}
	return w
}

// list returns spec's walk list, read once.
func (e *engine) list(ctx context.Context, spec string) walkList {
	l, ok := e.lists[spec]
	if !ok {
		commits, f := e.src.walk(ctx, spec)
		l = walkList{commits: commits, fact: e.known(f, fmt.Sprintf("spec/%s's acceptance", spec))}
		if l.fact.State == FactProven && (len(commits) == 0 || commits[0] != l.fact.Commit) {
			l.fact = unproven(fmt.Sprintf("spec/%s's walk does not start at the commit that first holds it", spec))
		}
		e.lists[spec] = l
	}
	return l
}

// advance evaluates w's commits in order until the match holds or cannot
// be proven, or the commits run out.
func (e *engine) advance(ctx context.Context, w *walkState) {
	for w.held < 0 && w.stopped == "" && w.next < len(w.list.commits) {
		i := w.next
		w.next++
		r := e.records(ctx, w.list.commits[i])
		switch {
		case r.err != nil:
			w.stopped = r.err.Error()
			continue
		case len(r.recs.Failures) > 0:
			w.stopped = "records do not decode at the acceptance commit: " + strings.Join(r.recs.Failures, "; ")
			continue
		}
		s := r.recs.Specs[w.successor]
		if s == nil {
			continue // the successor left the branch here; nothing to evaluate
		}
		switch st, witness := r.recs.stMatch(ctx, s, w.closed, nil); st {
		case matchHolds:
			w.held = i
		case matchUnproven:
			w.stopped = witness
		default:
			w.last = i
		}
	}
}

// at answers object's establishment from w's commit i: in force since its
// date when i is where the match holds and object's own edge is a new
// replacement there, and otherwise not in force with object's reason at i.
func (e *engine) at(ctx context.Context, w *walkState, i int, object artifact.Ref) Establishment {
	commit := w.list.commits[i]
	recs := e.records(ctx, commit).recs
	reason, detail := recs.inForceAt(ctx, w.successor, object, nil)
	switch {
	case reason == "" && i == w.held:
		d := e.date(ctx, commit)
		if d.State != FactProven {
			return Establishment{Reason: ReasonAcceptanceUnproven, Detail: d.Witness}
		}
		return Establishment{Commit: commit, Date: d.Date}
	case reason == "" || (i != w.held && reason != ReasonEstablisherNotInForce):
		// The match failed definitely at i, so object's answer there must
		// be a definite failure; anything else is this package's defect,
		// reported rather than passed.
		return Establishment{Reason: ReasonAcceptanceUnproven, Detail: fmt.Sprintf("objsupersede: spec/%s's evaluation of %s at %s is inconsistent", w.successor, object, commit)}
	}
	return Establishment{Reason: reason, Detail: detail}
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
	if e.err == nil {
		e.err = err
	}
	return unproven(err.Error())
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
// condition 5 (SI-272).
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

// textOrError renders a result this package built; a rendering error is
// reported inline, never dropped.
func textOrError(r Result) string {
	text, err := r.Text()
	if err != nil {
		return err.Error()
	}
	return text
}
