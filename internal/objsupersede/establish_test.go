package objsupersede

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// fakeWalk is a walkSource over in-memory commits "c1", "c2", ..., in
// first-parent order: lists maps a successor to its walk for every closed
// spec (the first commit holds it; none: never on the branch), pairs
// overrides it for one "successor closed" pair (SI-281's per-(S, T) walk
// set), facts overrides a successor's walk fact, at maps a commit to its
// records (absent or nil: unreadable), and days to its date (absent:
// unproven). calls counts every query.
type fakeWalk struct {
	lists map[string][]string
	pairs map[string][]string
	facts map[string]Fact
	at    map[string]*Records
	days  map[string]string
	calls map[string]int
}

func (f *fakeWalk) position(_ context.Context, commit string) (int, error) {
	var n int
	if _, err := fmt.Sscanf(commit, "c%d", &n); err != nil {
		return 0, fmt.Errorf("no position for %s", commit)
	}
	return n, nil
}

func (f *fakeWalk) walk(_ context.Context, spec, closed string) ([]string, Fact) {
	f.calls["walk "+spec+" "+closed]++
	l, ok := f.pairs[spec+" "+closed]
	if !ok {
		l = f.lists[spec]
	}
	if fact, ok := f.facts[spec]; ok {
		return l, fact
	}
	if len(l) == 0 {
		return nil, Fact{State: FactAbsent}
	}
	return l, Fact{State: FactProven, Commit: l[0]}
}

func (f *fakeWalk) recordsAt(_ context.Context, commit string) (*Records, error) {
	f.calls["records "+commit]++
	if r := f.at[commit]; r != nil {
		return r, nil
	}
	return nil, errors.New("blob missing at " + commit)
}

func (f *fakeWalk) dated(_ context.Context, commit string) Fact {
	f.calls["dated "+commit]++
	if d, ok := f.days[commit]; ok {
		return Fact{State: FactProven, Commit: commit, Date: d}
	}
	return unproven("no committer date for " + commit)
}

// walkOver is a fakeWalk in which s1 lands at c1 and walks c1, c2, ...,
// commit ci holding recs[i-1] and dated 2024-02-1<i>.
func walkOver(recs ...*Records) *fakeWalk {
	f := &fakeWalk{lists: map[string][]string{}, pairs: map[string][]string{}, facts: map[string]Fact{}, at: map[string]*Records{}, days: map[string]string{}, calls: map[string]int{}}
	for i, r := range recs {
		c := string(rune('1' + i))
		f.lists["s1"] = append(f.lists["s1"], "c"+c)
		f.at["c"+c] = r
		f.days["c"+c] = "2024-02-1" + c
	}
	return f
}

// TestEngine_Establishment pins the acceptance walk (SI-270 as amended
// after the whole-wave review, F-4; SI-275): the point is the earliest
// walked commit at which §3's match for (S_k, T) holds, per (S_k, T); no
// match is not in force with the latest evaluated commit's reason; an
// unproven evaluation before the point is acceptance unproven.
func TestEngine_Establishment(t *testing.T) {
	const p = "spec/t#ac-1"
	full := func() *Records {
		return mRecs([]*Conflict{mConflict("c1", "s1", vo)}, mSpec("s1", nil, mDec("dc-1", vo)))
	}
	specOnly := func() *Records { return mRecs(nil, mSpec("s1", nil, mDec("dc-1", vo))) }
	undecodable := func() *Records {
		r := full()
		r.Failures = []string{".verdi/specs/active/x/spec.md: broken"}
		return r
	}
	open := func() *Records {
		r := full()
		r.Conflicts[0].FM.Status = "open"
		return r
	}
	notClosed := func() *Records {
		r := full()
		r.Specs["t"].Archived = false
		return r
	}
	// v2 carries dc-1 from s1 (c1 names s1) and issues dc-3 to t#ac-1 (c2
	// names v2).
	carries := func() *Records {
		return mRecs([]*Conflict{mConflict("c1", "s1", vo), mConflict("c2", "v2", p)},
			mSpec("s1", nil, mDec("dc-1", vo)), mCarries(mSpec("v2", []string{"s1"}, mDec("dc-1", vo), mDec("dc-3", p)), "dc-1"))
	}
	inForceAt := func(c, day string) Establishment { return Establishment{Commit: c, Date: day} }
	notIn := func(detail string) Establishment {
		return Establishment{Reason: ReasonEstablisherNotInForce, Detail: detail}
	}
	unprovenBy := func(detail string) Establishment {
		return Establishment{Reason: ReasonAcceptanceUnproven, Detail: detail}
	}
	tests := []struct {
		name      string
		f         func() *fakeWalk
		successor string
		object    string
		want      Establishment
	}{
		{"the match holds where the spec lands", func() *fakeWalk { return walkOver(full()) }, "s1", vo, inForceAt("c1", "2024-02-11")},
		{"a landing without a merge commit: the conflict comes a commit later", func() *fakeWalk { return walkOver(specOnly(), full()) }, "s1", vo, inForceAt("c2", "2024-02-12")},
		{"a commit where the spec issues no replacement of T does not hold", func() *fakeWalk {
			return walkOver(mRecs(nil, mSpec("s1", nil, mDec("dc-1"))), full())
		}, "s1", vo, inForceAt("c2", "2024-02-12")},
		{"per (S_k, T): the object's edge added after the point establishes nothing", func() *fakeWalk {
			return walkOver(mRecs([]*Conflict{mConflict("c1", "s1", p)}, mSpec("s1", nil, mDec("dc-2", p))),
				mRecs([]*Conflict{mConflict("c1", "s1", p, vo)}, mSpec("s1", nil, mDec("dc-2", p), mDec("dc-1", vo))))
		}, "s1", vo, notIn("as of commit c1, spec/s1 carries no edge to spec/t#dc-1")},
		{"per (S_k, T): the object whose edge the point holds", func() *fakeWalk {
			return walkOver(mRecs([]*Conflict{mConflict("c1", "s1", p)}, mSpec("s1", nil, mDec("dc-2", p))),
				mRecs([]*Conflict{mConflict("c1", "s1", p, vo)}, mSpec("s1", nil, mDec("dc-2", p), mDec("dc-1", vo))))
		}, "s1", p, inForceAt("c1", "2024-02-11")},
		{"the walk for (S, T) reaches T's archive commit (SI-281's walk set)", func() *fakeWalk {
			f := walkOver(notClosed(), notClosed(), full())
			f.lists["s1"] = []string{"c1", "c2"} // S's own paths and the conflicts only
			f.pairs["s1 t"] = []string{"c1", "c3"}
			return f
		}, "s1", vo, inForceAt("c3", "2024-02-13")},
		{"never matches: the latest evaluated commit's reason", func() *fakeWalk { return walkOver(specOnly(), open()) }, "s1", vo,
			notIn("as of commit c2, the conflict conflict/c1 is not superseded")},
		{"the successor leaves the branch: the latest commit holding it gives the reason", func() *fakeWalk {
			return walkOver(specOnly(), mRecs(nil))
		}, "s1", vo, notIn("as of commit c1, no conflict challenges spec/t#dc-1")},
		{"a record that does not decode before the match holds (SI-274(6))", func() *fakeWalk { return walkOver(undecodable(), full()) }, "s1", vo,
			unprovenBy("records do not decode at commit c1: .verdi/specs/active/x/spec.md: broken")},
		{"an unreadable commit before the match holds", func() *fakeWalk { return walkOver(nil, full()) }, "s1", vo, unprovenBy("the records at commit c1 cannot be read: blob missing at c1")},
		{"a commit after the point is never read", func() *fakeWalk { return walkOver(full(), nil) }, "s1", vo, inForceAt("c1", "2024-02-11")},
		{"the point's date is unproven", func() *fakeWalk {
			f := walkOver(full())
			delete(f.days, "c1")
			return f
		}, "s1", vo, unprovenBy("no committer date for c1")},
		{"the object's own edge is carried at the point", func() *fakeWalk {
			f := walkOver(carries())
			f.lists["v2"] = f.lists["s1"]
			return f
		}, "v2", vo, notIn("as of commit c1, spec/v2 carries its edge to spec/t#dc-1 from its predecessor; it issues no new replacement")},
		{"never on the default branch", func() *fakeWalk { return walkOver() }, "s1", vo, Establishment{Reason: ReasonEstablisherNotAccepted}},
		{"the walk is unproven", func() *fakeWalk {
			f := walkOver(full())
			f.facts["s1"] = unproven("shallow history")
			return f
		}, "s1", vo, unprovenBy("shallow history")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := artifact.ParseRef(tc.object)
			if err != nil {
				t.Fatal(err)
			}
			if got := newEngine(tc.f()).establishment(context.Background(), tc.successor, ref); got != tc.want {
				t.Fatalf("got  %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

// TestEngine_Memo pins the engine's cost: the objects of one closed spec
// share one (S_k, T) walk, so each walk list, walked commit's records,
// and date is read once, and a commit past the point is never read.
func TestEngine_Memo(t *testing.T) {
	const p = "spec/t#ac-1"
	recs := mRecs([]*Conflict{mConflict("c1", "s1", vo, p)}, mSpec("s1", nil, mDec("dc-1", vo), mDec("dc-2", p)))
	f := walkOver(mRecs(nil, mSpec("s1", nil, mDec("dc-1", vo), mDec("dc-2", p))), recs, recs)
	e := newEngine(f)
	for _, o := range []artifact.Ref{obj("t", "dc-1"), obj("t", "ac-1"), obj("t", "dc-1")} {
		if got := e.establishment(context.Background(), "s1", o); got != (Establishment{Commit: "c2", Date: "2024-02-12"}) {
			t.Fatalf("%s: %+v", o, got)
		}
	}
	for key, n := range f.calls {
		if n != 1 {
			t.Errorf("%s read %d times, want once", key, n)
		}
	}
	if f.calls["records c3"] != 0 || f.calls["walk s1 t"] != 1 || f.calls["dated c2"] != 1 {
		t.Errorf("calls %v: want s1's walk and c2's date read once, and c3 never", f.calls)
	}
}

// storeOf is a fakeWalk over commits c1..cn holding recs, with the given
// walk lists.
func storeOf(lists map[string][]string, recs ...*Records) *fakeWalk {
	f := walkOver(recs...)
	f.lists = lists
	return f
}

// TestEngine_ConditionFiveAsOfCommit pins condition 5 inside the walk
// (SI-272 as amended, read at the evaluated commit): a rival refuses only
// when its supersession was in force strictly before that commit, so one
// in force is never unseated by a later one (design §4) and one never in
// force refuses nothing (F-1); a rival whose establishment is unproven
// there leaves the commit unproven (SI-274(3)); two matches first holding
// at one commit are each unproven, since neither is already superseded
// by the other; and a question nested inside a walk reads only the
// commits that walk has evaluated, so the walk terminates.
func TestEngine_ConditionFiveAsOfCommit(t *testing.T) {
	const p = "spec/t#ac-1"
	complete := func(name string) (*Spec, *Conflict) {
		return mSpec(name, nil, mDec("dc-1", vo)), mConflict("c"+name, name, vo)
	}
	// gapped: name's conflict also challenges t#ac-1, which it has no
	// edge to, so its match never holds.
	gapped := func(name string) (*Spec, *Conflict) {
		return mSpec(name, nil, mDec("dc-1", vo)), mConflict("c"+name, name, vo, p)
	}
	both := func(sx, ss *Spec, cx, cs *Conflict) *Records { return mRecs([]*Conflict{cs, cx}, sx, ss) }
	undecodable := func() *Records {
		sx, cx := complete("x")
		r := mRecs([]*Conflict{cx}, sx)
		r.Failures = []string{"x: broken"}
		return r
	}
	sxC, cxC := complete("x")
	ssC, csC := complete("s")
	sxG, cxG := gapped("x")
	ssG, csG := gapped("s")
	ssFixed := mSpec("s", nil, mDec("dc-1", vo), mDec("dc-2", p))
	sxFixed := mSpec("x", nil, mDec("dc-1", vo), mDec("dc-2", p))
	notIn := func(detail string) Establishment {
		return Establishment{Reason: ReasonEstablisherNotInForce, Detail: detail}
	}
	unprovenBy := func(detail string) Establishment {
		return Establishment{Reason: ReasonAcceptanceUnproven, Detail: detail}
	}
	tieAt := func(a, b, commit string) Establishment {
		return unprovenBy("spec/" + a + " and spec/" + b + " first match for spec/t#dc-1 at the same commit " + commit + ", so neither takes effect before the other")
	}
	tie := func(a, b string) Establishment { return tieAt(a, b, "c1") }
	// notClosed is both successors' complete records before t is closed.
	notClosed := func() *Records {
		r := both(sxC, ssC, cxC, csC)
		r.Specs["t"].Archived = false
		return r
	}
	tests := []struct {
		name  string
		f     func() *fakeWalk
		wantS Establishment
		wantX Establishment
	}{
		{"a rival in force before the commit refuses", func() *fakeWalk {
			return storeOf(map[string][]string{"x": {"c1", "c2"}, "s": {"c2"}}, mRecs([]*Conflict{cxC}, sxC), both(sxC, ssC, cxC, csC))
		}, notIn("as of commit c2, the object spec/t#dc-1 is already superseded by spec/x (conflict/cx)"), Establishment{Commit: "c1", Date: "2024-02-11"}},
		{"a supersession in force is never unseated by a later one", func() *fakeWalk {
			return storeOf(map[string][]string{"x": {"c1", "c2"}, "s": {"c1", "c2"}}, both(sxG, ssC, cxG, csC), both(sxFixed, ssC, cxG, csC))
		}, Establishment{Commit: "c1", Date: "2024-02-11"}, notIn("as of commit c2, the object spec/t#dc-1 is already superseded by spec/s (conflict/cs)")},
		{"two matches first holding at one commit are each unproven", func() *fakeWalk {
			return storeOf(map[string][]string{"x": {"c1"}, "s": {"c1"}}, both(sxC, ssC, cxC, csC))
		}, tie("s", "x"), tie("x", "s")},
		{"the tie check reads the rival's own (X, T) walk: both first match at T's archive commit", func() *fakeWalk {
			f := storeOf(map[string][]string{"x": {"c1"}, "s": {"c1"}}, notClosed(), both(sxC, ssC, cxC, csC))
			f.pairs["s t"], f.pairs["x t"] = []string{"c1", "c2"}, []string{"c1", "c2"}
			return f
		}, tieAt("s", "x", "c2"), tieAt("x", "s", "c2")},
		// The tie check reads a rival's answer for the object, not only its
		// match (lane L6 review M-5, mutant tie-always): x's match holds at
		// c2 through dc-3, but its edge to the object is carried from p1,
		// whose own match never holds (cp also challenges t#ac-1), so x is
		// never in force for it and s is not tied.
		{"a rival whose match holds but whose edge is carried is no tie", func() *fakeWalk {
			p1, cp := mSpec("p1", nil, mDec("dc-1", vo)), mConflict("cp", "p1", vo, p)
			x := mCarries(mSpec("x", []string{"p1"}, mDec("dc-1", vo), mDec("dc-3", p)), "dc-1")
			return storeOf(map[string][]string{"p1": {"c1", "c2"}, "s": {"c2"}, "x": {"c2"}},
				mRecs([]*Conflict{cp}, p1), mRecs([]*Conflict{cp, csC, mConflict("cx", "x", vo, p)}, p1, ssC, x))
		}, Establishment{Commit: "c2", Date: "2024-02-12"},
			notIn("as of commit c2, spec/x carries its edge to spec/t#dc-1 from its predecessor; it issues no new replacement")},
		// A rival whose match is unproven at the point leaves the tie
		// undecided (lane L6 review M-5, mutant tie-unproven-continue): x
		// lands with s at c2, and x's other edge's rival y is unproven
		// there, since y's commit c1 cannot be read.
		{"a rival unproven at the point leaves the successor unproven", func() *fakeWalk {
			x := mSpec("x", nil, mDec("dc-1", vo), mDec("dc-3", p))
			y := mSpec("y", nil, mDec("dc-1", p))
			return storeOf(map[string][]string{"y": {"c1", "c2"}, "s": {"c2"}, "x": {"c2"}},
				nil, mRecs([]*Conflict{csC, mConflict("cx", "x", vo, p), mConflict("cy", "y", p)}, ssC, x, y))
		}, unprovenBy("spec/x's establishment: spec/y's establishment: the records at commit c1 cannot be read: blob missing at c1"),
			unprovenBy("spec/y's establishment: the records at commit c1 cannot be read: blob missing at c1")},
		{"a rival's establishment unproven before the commit", func() *fakeWalk {
			return storeOf(map[string][]string{"x": {"c1", "c2"}, "s": {"c2"}}, undecodable(), both(sxC, ssC, cxC, csC))
		}, unprovenBy("spec/x's establishment: records do not decode at commit c1: x: broken"), unprovenBy("records do not decode at commit c1: x: broken")},
		{"a successor never in force refuses no later one (F-1)", func() *fakeWalk {
			return storeOf(map[string][]string{"x": {"c1", "c2"}, "s": {"c2"}}, mRecs([]*Conflict{cxG}, sxG), both(sxG, ssC, cxG, csC))
		}, Establishment{Commit: "c2", Date: "2024-02-12"}, notIn("as of commit c2, conflict/cx challenges spec/t#ac-1, but spec/x carries no matching edge")},
		{"a question nested in a walk reads only its evaluated commits", func() *fakeWalk {
			return storeOf(map[string][]string{"x": {"c1", "c2"}, "s": {"c1", "c2"}}, both(sxG, ssG, cxG, csG), both(sxG, ssFixed, cxG, csG))
		}, Establishment{Commit: "c2", Date: "2024-02-12"}, notIn("as of commit c2, conflict/cx challenges spec/t#ac-1, but spec/x carries no matching edge")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			for _, order := range [][]string{{"s", "x"}, {"x", "s"}} {
				e := newEngine(tc.f())
				got := map[string]Establishment{}
				for _, name := range order {
					got[name] = e.establishment(ctx, name, obj("t", "dc-1"))
				}
				if got["s"] != tc.wantS || got["x"] != tc.wantX || e.err != nil {
					t.Fatalf("asked %v:\n s   %+v\n want %+v\n x   %+v\n want %+v\n err %v", order, got["s"], tc.wantS, got["x"], tc.wantX, e.err)
				}
			}
		})
	}
}

// TestEngine_FailsClosed pins the walk's defensive answers: every one is
// acceptance unproven, never a guessed answer. A commit with no position
// is a history fact (the order could not be read); a walk that breaks the
// walk source's contract (not starting where the spec lands, not in
// first-parent order), a walk re-entered at a commit it is evaluating, or
// an evaluation inconsistent with its answer is this package's defect,
// also recorded as the engine's error, which History.Err and NewIndex
// surface as an operational error (lane L6 review M-4).
func TestEngine_FailsClosed(t *testing.T) {
	ctx := context.Background()
	full := func() *Records {
		return mRecs([]*Conflict{mConflict("c1", "s1", vo)}, mSpec("s1", nil, mDec("dc-1", vo)))
	}
	for _, tc := range []struct {
		name   string
		list   []string
		first  string // the walk's fact's commit; "" is the list's first
		want   string
		defect bool
	}{
		{"out of first-parent order", []string{"c2", "c1"}, "", "spec/s1's walk for spec/t is not in first-parent order at c1", true},
		{"a walk that does not start where the spec lands", []string{"c1", "c2"}, "c2", "spec/s1's walk for spec/t does not start at the commit that first holds it", true},
		{"a commit with no position", []string{"c1", "cx"}, "", "no position for cx", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := storeOf(map[string][]string{"s1": tc.list}, full(), full())
			if tc.first != "" {
				f.facts["s1"] = Fact{State: FactProven, Commit: tc.first}
			}
			e := newEngine(f)
			got := e.establishment(ctx, "s1", obj("t", "dc-1"))
			if got.Reason != ReasonAcceptanceUnproven || !strings.HasSuffix(got.Detail, tc.want) || (e.err != nil) != tc.defect {
				t.Fatalf("got %+v, engine error %v; want acceptance unproven ending %q, defect recorded %v", got, e.err, tc.want, tc.defect)
			}
		})
	}
	t.Run("an evaluation inconsistent with its answer", func(t *testing.T) {
		e := newEngine(walkOver(full()))
		w := e.walkOf(ctx, "s1", "t")
		w.evals = []evaluation{{state: evalFails}} // the records at c1 match: a failed evaluation there is inconsistent
		got := e.answer(ctx, w, 0, obj("t", "dc-1"))
		if got.Reason != ReasonAcceptanceUnproven || !strings.Contains(got.Detail, "inconsistent") || e.err == nil {
			t.Fatalf("got %+v, engine error %v; want acceptance unproven and the defect recorded", got, e.err)
		}
	})
	t.Run("a walk re-entered at the commit it is evaluating", func(t *testing.T) {
		e := newEngine(walkOver(full()))
		e.walkOf(ctx, "s1", "t").busy = true
		got := e.establishment(ctx, "s1", obj("t", "dc-1"))
		if got.Reason != ReasonAcceptanceUnproven || e.err == nil {
			t.Fatalf("got %+v, engine error %v; want acceptance unproven and the defect recorded", got, e.err)
		}
	})
	t.Run("a rival's walk re-entered by the tie check", func(t *testing.T) {
		recs := mRecs([]*Conflict{mConflict("cs", "s", vo), mConflict("cx", "x", vo)}, mSpec("s", nil, mDec("dc-1", vo)), mSpec("x", nil, mDec("dc-1", vo)))
		e := newEngine(storeOf(map[string][]string{"s": {"c1"}, "x": {"c1"}}, recs))
		e.walkOf(ctx, "x", "t").busy = true
		got := e.establishment(ctx, "s", obj("t", "dc-1"))
		if got != (Establishment{Reason: ReasonAcceptanceUnproven, Detail: "objsupersede: spec/x's walk was re-entered at a commit it is evaluating"}) || e.err == nil {
			t.Fatalf("got %+v, engine error %v; want acceptance unproven and the defect recorded", got, e.err)
		}
	})
}

// TestShortCommit pins how a reason names the commit it was evaluated at
// (SI-281): a full commit id's 12-hex prefix, and a shorter name whole.
func TestShortCommit(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"72718a09e3c8adf897ba25c11b8e2f67149f9e0b", "72718a09e3c8"},
		{"72718a09e3c8", "72718a09e3c8"},
		{"c1", "c1"},
		{"", ""},
	} {
		if got := shortCommit(tc.in); got != tc.want {
			t.Errorf("shortCommit(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
