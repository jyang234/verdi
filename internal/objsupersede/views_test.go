package objsupersede

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
)

// fakeHist is a viewHistory over in-memory facts. A spec in accepted is
// accepted at commit "acc-<spec>" on that date, and that commit's records
// are at[spec] (nil: unreadable), else tree; unproven maps a spec to its
// acceptance witness; facts overrides a spec's acceptance fact as given;
// closed overrides a closed spec's fact (default: proven on 2024-01-10).
// calls counts every query.
type fakeHist struct {
	tree     *Records
	accepted map[string]string
	unproven map[string]string
	facts    map[string]Fact
	at       map[string]*Records
	closed   map[string]Fact
	calls    map[string]int
}

func (f *fakeHist) Acceptance(_ context.Context, spec string) Fact {
	f.calls["acceptance "+spec]++
	if fact, ok := f.facts[spec]; ok {
		return fact
	}
	if w, ok := f.unproven[spec]; ok {
		return Fact{State: FactUnproven, Witness: w}
	}
	if d, ok := f.accepted[spec]; ok {
		return Fact{State: FactProven, Commit: "acc-" + spec, Date: d}
	}
	return Fact{State: FactAbsent}
}

func (f *fakeHist) Closed(_ context.Context, t *Spec) Fact {
	f.calls["closed "+t.Name]++
	if c, ok := f.closed[t.Name]; ok {
		return c
	}
	return Fact{State: FactProven, Commit: "close-" + t.Name, Date: "2024-01-10"}
}

func (f *fakeHist) recordsAt(_ context.Context, commit string) (*Records, error) {
	f.calls["records "+commit]++
	if r, ok := f.at[strings.TrimPrefix(commit, "acc-")]; ok {
		if r == nil {
			return nil, errors.New("blob missing")
		}
		return r, nil
	}
	return f.tree, nil
}

// hist is a fakeHist over tree with s accepted on the given dates.
func hist(tree *Records, accepted ...string) *fakeHist {
	f := &fakeHist{tree: tree, accepted: map[string]string{}, unproven: map[string]string{}, facts: map[string]Fact{}, at: map[string]*Records{}, closed: map[string]Fact{}, calls: map[string]int{}}
	for i := 0; i+1 < len(accepted); i += 2 {
		f.accepted[accepted[i]] = accepted[i+1]
	}
	return f
}

func index(t *testing.T, h *fakeHist) *Index {
	t.Helper()
	x, err := newIndex(context.Background(), h.tree, h)
	if err != nil {
		t.Fatalf("newIndex: %v", err)
	}
	return x
}

// lines joins a view's rendered lines, failing t on a rendering error.
func lines(t *testing.T, v interface{ Lines() ([]string, error) }) string {
	t.Helper()
	ls, err := v.Lines()
	if err != nil {
		t.Fatalf("lines: %v", err)
	}
	return strings.Join(ls, " | ")
}

const (
	vo    = "spec/t#dc-1"
	vGov  = "governed spec/t's completed work (closed 2024-01-10) | "
	vBy   = "superseded since 2024-02-15 by spec/s1#dc-1"
	vDay1 = "2024-02-15"
)

// TestIndex_ObjectViews pins the object view (design §4, §6; SI-261,
// SI-265, SI-270, SI-274(5), (6), (8)): superseded only when one
// establishing successor is in force, its carrying state from the head of
// its revision chain, and unproven rather than guessed.
func TestIndex_ObjectViews(t *testing.T) {
	s1 := func(edges ...string) *Spec { return mSpec("s1", nil, mDec("dc-1", edges...)) }
	carries := func(name, pred string, edges ...string) *Spec {
		return mCarries(mSpec(name, []string{pred}, mDec("dc-1", edges...)), "dc-1")
	}
	c1 := func() *Conflict { return mConflict("c1", "s1", vo) }
	one := func() *Records { return mRecs([]*Conflict{c1()}, s1(vo)) }
	superseded := ObjectView{Object: vo, State: ObjectSuperseded, By: "spec/s1#dc-1", Conflict: "conflict/c1", Since: vDay1, Closed: "2024-01-10"}
	with := func(v ObjectView, f func(*ObjectView)) ObjectView { f(&v); return v }
	tests := []struct {
		name  string
		h     func() *fakeHist
		spec  string // "" means t#dc-1
		id    string
		want  ObjectView
		lines string
	}{
		{"no record names the object", func() *fakeHist { return hist(mRecs(nil), "s1", vDay1) }, "", "",
			ObjectView{Object: vo, State: ObjectNotSuperseded}, ""},
		{"in force", func() *fakeHist { return hist(one(), "s1", vDay1) }, "", "", superseded, vGov + vBy},
		{"a criterion in force", func() *fakeHist {
			return hist(mRecs([]*Conflict{mConflict("c1", "s1", "spec/t#ac-1")}, mSpec("s1", nil, mDec("dc-2", "spec/t#ac-1"))), "s1", vDay1)
		}, "t", "ac-1", ObjectView{Object: "spec/t#ac-1", State: ObjectSuperseded, By: "spec/s1#dc-2", Conflict: "conflict/c1", Since: vDay1, Closed: "2024-01-10"},
			"governed spec/t's completed work (closed 2024-01-10) | superseded since 2024-02-15 by spec/s1#dc-2"},
		{"proposed: the successor is not accepted", func() *fakeHist { return hist(one()) }, "", "", ObjectView{Object: vo, State: ObjectNotSuperseded}, ""},
		{"accepted, but a sibling edge had no conflict: the records do not match", func() *fakeHist {
			return hist(mRecs([]*Conflict{c1()}, mSpec("s1", nil, mDec("dc-1", vo), mDec("dc-2", "spec/t#ac-1"))), "s1", vDay1)
		}, "", "", ObjectView{Object: vo, State: ObjectNotSuperseded}, ""},
		{"an open conflict names no successor", func() *fakeHist {
			r := one()
			r.Conflicts[0].FM.Status = "open"
			return hist(r, "s1", vDay1)
		}, "", "", ObjectView{Object: vo, State: ObjectNotSuperseded}, ""},
		{"a pinned challenge names no successor", func() *fakeHist {
			return hist(mRecs([]*Conflict{mConflict("c1", "s1", "spec/t@0a1b2c3#dc-1")}, s1(vo)), "s1", vDay1)
		}, "", "", ObjectView{Object: vo, State: ObjectNotSuperseded}, ""},
		{"an object of a spec that is not closed", func() *fakeHist {
			r := one()
			r.Specs["t"].Archived = false
			return hist(r, "s1", vDay1)
		}, "", "", ObjectView{Object: vo, State: ObjectNotSuperseded}, ""},
		{"acceptance unproven", func() *fakeHist {
			h := hist(one())
			h.unproven["s1"] = "shallow history"
			return h
		}, "", "", ObjectView{Object: vo, State: ObjectUnproven, Witness: "spec/s1: acceptance unproven: shallow history"},
			"supersession unproven: spec/s1: acceptance unproven: shallow history"},
		{"the acceptance commit is unreadable", func() *fakeHist {
			h := hist(one(), "s1", vDay1)
			h.at["s1"] = nil
			return h
		}, "", "", ObjectView{Object: vo, State: ObjectUnproven, Witness: "spec/s1: acceptance unproven: blob missing"}, "supersession unproven: spec/s1: acceptance unproven: blob missing"},
		{"two successors unproven: the first is named", func() *fakeHist {
			h := hist(mRecs([]*Conflict{c1(), mConflict("cx", "x", vo)}, s1(vo), mSpec("x", nil, mDec("dc-1", vo))))
			h.unproven["s1"], h.unproven["x"] = "witness s1", "witness x"
			return h
		}, "", "", ObjectView{Object: vo, State: ObjectUnproven, Witness: "spec/s1: acceptance unproven: witness s1"},
			"supersession unproven: spec/s1: acceptance unproven: witness s1"},
		{"two successors unproven: the first in sorted order, whatever the conflicts' order", func() *fakeHist {
			h := hist(mRecs([]*Conflict{mConflict("c0", "x", vo), c1()}, s1(vo), mSpec("x", nil, mDec("dc-1", vo))))
			h.unproven["s1"], h.unproven["x"] = "witness s1", "witness x"
			return h
		}, "", "", ObjectView{Object: vo, State: ObjectUnproven, Witness: "spec/s1: acceptance unproven: witness s1"},
			"supersession unproven: spec/s1: acceptance unproven: witness s1"},
		{"a proposed rival: the successor in force is still named", func() *fakeHist {
			h := hist(mRecs([]*Conflict{c1(), mConflict("cx", "x", vo)}, s1(vo), mSpec("x", nil, mDec("dc-1", vo))), "s1", vDay1)
			h.at["s1"] = one()
			return h
		}, "", "", superseded, vGov + vBy},
		{"a later duplicate conflict: the conflict as it stood at acceptance (SI-279)", func() *fakeHist {
			h := hist(mRecs([]*Conflict{mConflict("c0", "s1", vo), c1()}, s1(vo)), "s1", vDay1)
			h.at["s1"] = one()
			return h
		}, "", "", superseded, vGov + vBy},
		{"two successors in force (SI-274(8))", func() *fakeHist {
			x, cx := mSpec("x", nil, mDec("dc-1", vo)), mConflict("cx", "x", vo)
			h := hist(mRecs([]*Conflict{c1(), cx}, s1(vo), x), "s1", vDay1, "x", "2024-03-01")
			h.at["s1"], h.at["x"] = mRecs([]*Conflict{c1()}, s1(vo)), mRecs([]*Conflict{cx}, x)
			return h
		}, "", "", ObjectView{Object: vo, State: ObjectUnproven, Witness: "more than one successor's supersession is in force: spec/s1, spec/x"},
			"supersession unproven: more than one successor's supersession is in force: spec/s1, spec/x"},
		{"undecodable records (SI-274(6))", func() *fakeHist {
			r := one()
			r.Failures = []string{".verdi/conflicts/x.md: broken"}
			return hist(r, "s1", vDay1)
		}, "", "", ObjectView{Object: vo, State: ObjectUnproven, Witness: "records do not decode: .verdi/conflicts/x.md: broken"},
			"supersession unproven: records do not decode: .verdi/conflicts/x.md: broken"},
		{"undecodable records leave an open spec's object alone", func() *fakeHist {
			r := one()
			r.Failures = []string{".verdi/conflicts/x.md: broken"}
			return hist(r, "s1", vDay1)
		}, "s1", "dc-1", ObjectView{Object: "spec/s1#dc-1", State: ObjectNotSuperseded}, ""},
		{"carried by the head of the chain", func() *fakeHist {
			return hist(mRecs([]*Conflict{c1()}, s1(vo), carries("s2", "s1", vo)), "s1", vDay1, "s2", "2024-03-15")
		}, "", "", with(superseded, func(v *ObjectView) { v.Carry, v.Revision = CarryCarried, "spec/s2" }), vGov + vBy + " | carried by spec/s2"},
		{"carried through two revisions", func() *fakeHist {
			return hist(mRecs([]*Conflict{c1()}, s1(vo), carries("s2", "s1", vo), carries("s3", "s2", vo)), "s1", vDay1, "s2", "2024-03-15", "s3", "2024-04-15")
		}, "", "", with(superseded, func(v *ObjectView) { v.Carry, v.Revision = CarryCarried, "spec/s3" }), vGov + vBy + " | carried by spec/s3"},
		{"the head dropped the edge: still superseded", func() *fakeHist {
			return hist(mRecs([]*Conflict{c1()}, s1(vo), carries("s2", "s1")), "s1", vDay1, "s2", "2024-03-15")
		}, "", "", with(superseded, func(v *ObjectView) { v.Carry, v.Revision = CarryDropped, "spec/s2" }),
			vGov + vBy + " | no longer carried by the current revision (spec/s2)"},
		{"the head's edge is a new replacement, not a carry", func() *fakeHist {
			s2 := mSpec("s2", []string{"s1"}, mDec("dc-1"), mDec("dc-9", vo))
			s2.FM.Supersession = &artifact.Supersession{Carried: []string{"dc-1"}, Added: []string{"dc-9"}}
			return hist(mRecs([]*Conflict{c1()}, s1(vo), s2), "s1", vDay1, "s2", "2024-03-15")
		}, "", "", with(superseded, func(v *ObjectView) { v.Carry, v.Revision = CarryDropped, "spec/s2" }),
			vGov + vBy + " | no longer carried by the current revision (spec/s2)"},
		{"the establishing successor edited in place dropped its edge", func() *fakeHist {
			h := hist(mRecs([]*Conflict{c1()}, s1()), "s1", vDay1)
			h.at["s1"] = one()
			return h
		}, "", "", with(superseded, func(v *ObjectView) { v.Carry, v.Revision = CarryDropped, "spec/s1" }),
			vGov + vBy + " | no longer carried by the current revision (spec/s1)"},
		{"the chain branches (SI-274(5))", func() *fakeHist {
			return hist(mRecs([]*Conflict{c1()}, s1(vo), carries("b", "s1", vo), carries("a", "s1", vo), carries("a2", "a", vo)),
				"s1", vDay1, "a", "2024-03-15", "b", "2024-03-15", "a2", "2024-04-15")
		}, "", "", with(superseded, func(v *ObjectView) {
			v.Carry, v.Heads, v.Witness = CarryUnproven, []string{"spec/a2", "spec/b"}, "the revision chain of spec/s1 branches: spec/a2, spec/b"
		}), vGov + vBy + " | carrying unproven: the revision chain of spec/s1 branches: spec/a2, spec/b"},
		{"the head is not accepted", func() *fakeHist {
			return hist(mRecs([]*Conflict{c1()}, s1(vo), carries("s2", "s1", vo)), "s1", vDay1)
		}, "", "", with(superseded, func(v *ObjectView) {
			v.Carry, v.Revision, v.Witness = CarryUnproven, "spec/s2", "spec/s2 is not accepted"
		}), vGov + vBy + " | carrying unproven: spec/s2 is not accepted"},
		{"the head's acceptance is unproven", func() *fakeHist {
			h := hist(mRecs([]*Conflict{c1()}, s1(vo), carries("s2", "s1", vo)), "s1", vDay1)
			h.unproven["s2"] = "shallow history"
			return h
		}, "", "", with(superseded, func(v *ObjectView) {
			v.Carry, v.Revision, v.Witness = CarryUnproven, "spec/s2", "acceptance unproven: shallow history"
		}), vGov + vBy + " | carrying unproven: acceptance unproven: shallow history"},
		{"the establishing successor is not in the evaluated tree", func() *fakeHist {
			h := hist(mRecs([]*Conflict{c1()}), "s1", vDay1)
			h.at["s1"] = one()
			return h
		}, "", "", with(superseded, func(v *ObjectView) {
			v.Carry, v.Revision, v.Witness = CarryUnproven, "spec/s1", "spec/s1 is not in the evaluated tree"
		}), vGov + vBy + " | carrying unproven: spec/s1 is not in the evaluated tree"},
		{"the closed date is unproven: never invented", func() *fakeHist {
			h := hist(one(), "s1", vDay1)
			h.closed["t"] = Fact{State: FactUnproven, Witness: "path exists, but not '.verdi/x'"}
			return h
		}, "", "", with(superseded, func(v *ObjectView) { v.Closed, v.ClosedWitness = "", "path exists, but not '.verdi/x'" }),
			"governed spec/t's completed work (closed date unproven: path exists, but not '.verdi/x') | " + vBy},
		{"no closed baseline", func() *fakeHist {
			h := hist(one(), "s1", vDay1)
			h.closed["t"] = Fact{State: FactAbsent}
			return h
		}, "", "", with(superseded, func(v *ObjectView) {
			v.Closed, v.ClosedWitness = "", "no closed baseline of spec/t on the default branch"
		}),
			"governed spec/t's completed work (closed date unproven: no closed baseline of spec/t on the default branch) | " + vBy},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spec, id := tc.spec, tc.id
			if spec == "" {
				spec, id = "t", "dc-1"
			}
			got := index(t, tc.h()).Object(spec, id)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got  %+v\nwant %+v", got, tc.want)
			}
			if l := lines(t, got); l != tc.lines {
				t.Fatalf("lines %q\nwant  %q", l, tc.lines)
			}
		})
	}
}

// TestIndex_DecisionViews pins the decision view (design §6; SI-263,
// SI-274(3)): in force from its establishment, proposed before its
// spec's acceptance only while §3's whole match holds (SI-275), and
// otherwise not established with the core's reason.
func TestIndex_DecisionViews(t *testing.T) {
	s1 := func(decs ...artifact.Decision) *Spec { return mSpec("s1", nil, decs...) }
	c1 := func(challenges ...string) *Conflict { return mConflict("c1", "s1", challenges...) }
	one := func() *Records { return mRecs([]*Conflict{c1(vo)}, s1(mDec("dc-1", vo))) }
	chain := func() *Records {
		return mRecs([]*Conflict{c1(vo)}, s1(mDec("dc-1", vo)), mCarries(mSpec("s2", []string{"s1"}, mDec("dc-1", vo)), "dc-1"))
	}
	view := func(state DecisionState) DecisionView {
		return DecisionView{Decision: "spec/s1#dc-1", Edge: vo, Object: vo, State: state, Conflict: "conflict/c1"}
	}
	carried := func(state DecisionState) DecisionView {
		return DecisionView{Decision: "spec/s2#dc-1", Edge: vo, Object: vo, State: state, Carried: true, Establisher: "spec/s1", Conflict: "conflict/c1", Since: vDay1}
	}
	notEst := func(conflict, reason string) DecisionView {
		return DecisionView{Decision: "spec/s1#dc-1", Edge: vo, Object: vo, State: DecisionNotEstablished, Conflict: conflict, Reason: reason}
	}
	// inForce is s1's new replacement in force, with the conflict c1 that
	// established it at s1's acceptance commit (SI-279).
	inForce := DecisionView{Decision: "spec/s1#dc-1", Edge: vo, Object: vo, State: DecisionInForce, Conflict: "conflict/c1", Establisher: "spec/s1", Since: vDay1}
	rival := func() *Spec { return mSpec("x", nil, mDec("dc-1", vo)) }
	// inForceAt holds s1 in force at its acceptance commit, whose records
	// are one's, whatever the evaluated tree holds.
	inForceAt := func(tree *Records) *fakeHist {
		h := hist(tree, "s1", vDay1)
		h.at["s1"] = one()
		return h
	}
	const broken = ".verdi/conflicts/x.md: broken"
	undecodable := func() *Records {
		r := one()
		r.Failures = []string{broken}
		return r
	}
	const carriesLine = " | carries the replacement established by spec/s1 (conflict/c1, since 2024-02-15)"
	tests := []struct {
		name      string
		h         func() *fakeHist
		decision  string // "s1#dc-1" when ""
		want      []DecisionView
		wantLines string // the first view's lines
	}{
		{"a new replacement in force", func() *fakeHist { return hist(one(), "s1", vDay1) }, "",
			[]DecisionView{func() DecisionView { v := view(DecisionInForce); v.Establisher, v.Since = "spec/s1", vDay1; return v }()},
			"supersedes spec/t#dc-1"},
		{"proposed before acceptance", func() *fakeHist { return hist(one()) }, "", []DecisionView{view(DecisionProposed)},
			"proposed — supersedes spec/t#dc-1 when spec/s1 is accepted"},
		{"proposed, but a sibling edge has no conflict (SI-275)", func() *fakeHist {
			return hist(mRecs([]*Conflict{c1(vo)}, s1(mDec("dc-1", vo), mDec("dc-2", "spec/t#ac-1"))))
		}, "", []DecisionView{notEst("conflict/c1", "no conflict challenges spec/t#ac-1")},
			"supersession not established: no conflict challenges spec/t#ac-1"},
		{"proposed, but a challenged fragment has no edge", func() *fakeHist {
			return hist(mRecs([]*Conflict{c1(vo, "spec/t#ac-1")}, s1(mDec("dc-1", vo))))
		}, "", []DecisionView{notEst("conflict/c1", "conflict/c1 challenges spec/t#ac-1, but spec/s1 carries no matching edge")}, "supersession not established: conflict/c1 challenges spec/t#ac-1, but spec/s1 carries no matching edge"},
		{"the edge's own records do not match", func() *fakeHist { return hist(mRecs(nil, s1(mDec("dc-1", vo))), "s1", vDay1) }, "",
			[]DecisionView{notEst("", "no conflict challenges spec/t#dc-1")}, "supersession not established: no conflict challenges spec/t#dc-1"},
		{"accepted, but not in force at its acceptance", func() *fakeHist {
			h := hist(one(), "s1", vDay1)
			h.at["s1"] = one()
			h.at["s1"].Conflicts[0].FM.Status = "open"
			return h
		}, "", []DecisionView{notEst("conflict/c1", "spec/s1's supersession was not in force at its acceptance: the conflict conflict/c1 is not superseded")},
			"supersession not established: spec/s1's supersession was not in force at its acceptance: the conflict conflict/c1 is not superseded"},
		{"acceptance unproven", func() *fakeHist {
			h := hist(one())
			h.unproven["s1"] = "shallow history"
			return h
		}, "", []DecisionView{notEst("conflict/c1", "acceptance unproven: shallow history")}, "supersession not established: acceptance unproven: shallow history"},
		{"a pinned edge", func() *fakeHist {
			return hist(mRecs([]*Conflict{c1(vo)}, s1(mDec("dc-1", "spec/t@0a1b2c3#dc-1"))), "s1", vDay1)
		}, "", []DecisionView{{Decision: "spec/s1#dc-1", Edge: "spec/t@0a1b2c3#dc-1", Object: vo, State: DecisionNotEstablished,
			Reason: "the edge spec/t@0a1b2c3#dc-1 is pinned; a closed spec's object is superseded only by an unpinned ref"}},
			"supersession not established: the edge spec/t@0a1b2c3#dc-1 is pinned; a closed spec's object is superseded only by an unpinned ref"},
		{"a carried decision on an accepted revision", func() *fakeHist { return hist(chain(), "s1", vDay1, "s2", "2024-03-15") }, "s2#dc-1",
			[]DecisionView{carried(DecisionInForce)}, "supersedes spec/t#dc-1" + carriesLine},
		{"a carried decision on a revision not yet accepted", func() *fakeHist { return hist(chain(), "s1", vDay1) }, "s2#dc-1",
			[]DecisionView{carried(DecisionProposed)}, "proposed — supersedes spec/t#dc-1 when spec/s2 is accepted" + carriesLine},
		{"a carried decision whose revision's acceptance is unproven", func() *fakeHist {
			h := hist(chain(), "s1", vDay1)
			h.unproven["s2"] = "shallow history"
			return h
		}, "s2#dc-1", []DecisionView{{Decision: "spec/s2#dc-1", Edge: vo, Object: vo, State: DecisionNotEstablished, Conflict: "conflict/c1", Reason: "acceptance unproven: shallow history"}},
			"supersession not established: acceptance unproven: shallow history"},
		{"a carry whose establishing successor is not accepted", func() *fakeHist { return hist(chain()) }, "s2#dc-1",
			[]DecisionView{{Decision: "spec/s2#dc-1", Edge: vo, Object: vo, State: DecisionNotEstablished, Conflict: "conflict/c1", Reason: "spec/s1 is not accepted"}},
			"supersession not established: spec/s1 is not accepted"},
		{"two edges on one decision, in link order", func() *fakeHist {
			return hist(mRecs([]*Conflict{c1(vo), mConflict("cu", "s1", "spec/u#dc-1")}, s1(mDec("dc-1", vo, "spec/u#dc-1"))))
		}, "", []DecisionView{view(DecisionProposed), {Decision: "spec/s1#dc-1", Edge: "spec/u#dc-1", Object: "spec/u#dc-1", State: DecisionProposed, Conflict: "conflict/cu"}},
			"proposed — supersedes spec/t#dc-1 when spec/s1 is accepted"},
		{"a decision with no fragment edge", func() *fakeHist { return hist(one()) }, "s1#dc-9", nil, ""},
		// A supersession in force is permanent (design §4, SI-261): a
		// later record in the evaluated tree never unseats the establishing
		// successor's decision, nor names a rival as its superseder.
		{"in force: a proposed rival never unseats it", func() *fakeHist {
			return inForceAt(mRecs([]*Conflict{c1(vo), mConflict("cx", "x", vo)}, s1(mDec("dc-1", vo)), rival()))
		}, "", []DecisionView{inForce}, "supersedes spec/t#dc-1"},
		{"in force: a later duplicate conflict (SI-274(1)) never unseats it; its conflict as at acceptance", func() *fakeHist {
			return inForceAt(mRecs([]*Conflict{mConflict("c0", "s1", vo), c1(vo)}, s1(mDec("dc-1", vo))))
		}, "", []DecisionView{inForce}, "supersedes spec/t#dc-1"},
		{"in force: another successor also in force (SI-274(8))", func() *fakeHist {
			cx := mConflict("cx", "x", vo)
			h := hist(mRecs([]*Conflict{c1(vo), cx}, s1(mDec("dc-1", vo)), rival()), "s1", vDay1, "x", "2024-03-01")
			h.at["s1"], h.at["x"] = one(), mRecs([]*Conflict{cx}, rival())
			return h
		}, "", []DecisionView{inForce}, "supersedes spec/t#dc-1"},
		{"not in force at its acceptance: a later duplicate conflict reads not established", func() *fakeHist {
			h := inForceAt(mRecs([]*Conflict{mConflict("c0", "s1", vo), c1(vo)}, s1(mDec("dc-1", vo))))
			h.at["s1"].Conflicts[0].FM.Status = "open"
			return h
		}, "", []DecisionView{notEst("conflict/c0", "more than one superseded conflict names spec/s1 for spec/t")},
			"supersession not established: more than one superseded conflict names spec/s1 for spec/t"},
		{"not in force at its acceptance: a rival reads not established", func() *fakeHist {
			h := inForceAt(mRecs([]*Conflict{c1(vo), mConflict("cx", "x", vo)}, s1(mDec("dc-1", vo)), rival()))
			h.at["s1"].Conflicts[0].FM.Status = "open"
			return h
		}, "", []DecisionView{notEst("conflict/cx", "the object spec/t#dc-1 is already superseded by spec/x (conflict/cx)")},
			"supersession not established: the object spec/t#dc-1 is already superseded by spec/x (conflict/cx)"},
		{"undecodable records (SI-274(6)), though in force at acceptance", func() *fakeHist { return inForceAt(undecodable()) }, "",
			[]DecisionView{notEst("", "records do not decode: "+broken)}, "supersession not established: records do not decode: " + broken},
		{"undecodable records (SI-274(6)) before acceptance", func() *fakeHist { return hist(undecodable()) }, "",
			[]DecisionView{notEst("", "records do not decode: "+broken)}, "supersession not established: records do not decode: " + broken},
		{"a pinned edge, though its spec is in force at acceptance (SI-271)", func() *fakeHist {
			return inForceAt(mRecs([]*Conflict{c1(vo)}, s1(mDec("dc-1", "spec/t@0a1b2c3#dc-1"))))
		}, "", []DecisionView{{Decision: "spec/s1#dc-1", Edge: "spec/t@0a1b2c3#dc-1", Object: vo, State: DecisionNotEstablished,
			Reason: "the edge spec/t@0a1b2c3#dc-1 is pinned; a closed spec's object is superseded only by an unpinned ref"}},
			"supersession not established: the edge spec/t@0a1b2c3#dc-1 is pinned; a closed spec's object is superseded only by an unpinned ref"},
		{"a carried candidate takes the carried path, though its spec is in force at acceptance (SI-273)", func() *fakeHist {
			s2, c2 := mCarries(mSpec("s2", []string{"s1"}, mDec("dc-1", vo)), "dc-1"), mConflict("c2", "s2", vo)
			h := hist(mRecs([]*Conflict{c1(vo), c2}, s1(mDec("dc-1", vo)), s2), "s2", "2024-03-15")
			h.at["s2"] = mRecs([]*Conflict{c2}, s1(mDec("dc-1", vo)), s2)
			return h
		}, "s2#dc-1", []DecisionView{{Decision: "spec/s2#dc-1", Edge: vo, Object: vo, State: DecisionNotEstablished, Conflict: "conflict/c1", Reason: "spec/s1 is not accepted"}},
			"supersession not established: spec/s1 is not accepted"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decision := tc.decision
			if decision == "" {
				decision = "s1#dc-1"
			}
			spec, id, _ := strings.Cut(decision, "#")
			got := index(t, tc.h()).Decisions(spec, id)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got  %+v\nwant %+v", got, tc.want)
			}
			if len(got) > 0 {
				if l := lines(t, got[0]); l != tc.wantLines {
					t.Fatalf("lines %q\nwant  %q", l, tc.wantLines)
				}
			}
		})
	}
}

// TestIndex_Batch pins item 3's cost and purity: one index answers every
// view of a tree, each acceptance, acceptance-commit read, and closed date
// queried at most once; building never mutates the records, and two
// builds are equal.
func TestIndex_Batch(t *testing.T) {
	build := func() (*Records, *fakeHist) {
		o2 := "spec/t#ac-1"
		recs := mRecs([]*Conflict{mConflict("c1", "s1", vo, o2)},
			mSpec("s1", nil, mDec("dc-1", vo), mDec("dc-2", o2)),
			mCarries(mSpec("s2", []string{"s1"}, mDec("dc-1", vo), mDec("dc-2", o2)), "dc-1", "dc-2"),
			mCarries(mSpec("s3", []string{"s2"}, mDec("dc-1", vo), mDec("dc-2", o2)), "dc-1", "dc-2"))
		return recs, hist(recs, "s1", vDay1, "s2", "2024-03-15", "s3", "2024-04-15")
	}
	recs, h := build()
	x, err := newIndex(context.Background(), recs, h)
	if err != nil {
		t.Fatal(err)
	}
	for k, n := range h.calls {
		if n != 1 {
			t.Errorf("%s queried %d times, want once", k, n)
		}
	}
	for _, key := range []string{"acceptance s1", "records acc-s1", "closed t"} {
		if h.calls[key] != 1 {
			t.Errorf("%s never queried", key)
		}
	}
	fresh, h2 := build()
	if !reflect.DeepEqual(recs, fresh) {
		t.Fatal("building the index mutated the records")
	}
	y, err := newIndex(context.Background(), fresh, h2)
	if err != nil || !reflect.DeepEqual(x, y) {
		t.Fatalf("two builds differ: %v", err)
	}
	if got := x.Object("t", "ac-1"); got.Carry != CarryCarried || got.Revision != "spec/s3" || got.By != "spec/s1#dc-2" {
		t.Fatalf("t#ac-1: %+v", got)
	}
	d := x.Decisions("s3", "dc-1")
	d[0].State = DecisionNotEstablished
	if x.Decisions("s3", "dc-1")[0].State != DecisionInForce {
		t.Fatal("a caller's edit reached the index's decision views")
	}
	branched := &Index{objects: map[string]ObjectView{vo: {Object: vo, Heads: []string{"spec/a"}}}}
	branched.Object("t", "dc-1").Heads[0] = "spec/z"
	if branched.Object("t", "dc-1").Heads[0] != "spec/a" {
		t.Fatal("a caller's edit reached the index's object views")
	}
	if _, err := newIndex(context.Background(), nil, h); err == nil {
		t.Fatal("nil records built an index")
	}
}

// TestIndex_UnknownFact pins that a history answer outside FactState's
// three values fails the index closed with an error, never a panic or a
// guessed state (lane L3c review F-4).
func TestIndex_UnknownFact(t *testing.T) {
	one := func() *Records {
		return mRecs([]*Conflict{mConflict("c1", "s1", vo)}, mSpec("s1", nil, mDec("dc-1", vo)))
	}
	tests := []struct {
		name string
		h    func() *fakeHist
		want string
	}{
		{"a zero acceptance fact", func() *fakeHist {
			h := hist(one())
			h.facts["s1"] = Fact{}
			return h
		}, `spec/s1's acceptance with unknown state ""`},
		{"an unknown acceptance state", func() *fakeHist {
			h := hist(one())
			h.facts["s1"] = Fact{State: "maybe", Commit: "acc-s1", Date: vDay1}
			return h
		}, `spec/s1's acceptance with unknown state "maybe"`},
		{"an unknown closed state", func() *fakeHist {
			h := hist(one(), "s1", vDay1)
			h.closed["t"] = Fact{State: "maybe", Commit: "close-t", Date: "2024-01-10"}
			return h
		}, `spec/t's closed date with unknown state "maybe"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := tc.h()
			x, err := newIndex(context.Background(), h.tree, h)
			if err == nil || x != nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, %v; want an error containing %q", x, err, tc.want)
			}
		})
	}
}

// TestObjectView_LinesFailClosed pins that a malformed view is an error,
// never a rendering.
func TestObjectView_LinesFailClosed(t *testing.T) {
	ok := ObjectView{Object: vo, State: ObjectSuperseded, By: "spec/s1#dc-1", Conflict: "conflict/c1", Since: vDay1, Closed: "2024-01-10"}
	tests := []struct {
		name string
		f    func(*ObjectView)
	}{
		{"unknown state", func(v *ObjectView) { v.State = "gone" }},
		{"unproven without a witness", func(v *ObjectView) { v.State = ObjectUnproven }},
		{"no deciding decision", func(v *ObjectView) { v.By = "" }},
		{"no conflict", func(v *ObjectView) { v.Conflict = "" }},
		{"a malformed since date", func(v *ObjectView) { v.Since = "15 Feb 2024" }},
		{"no closed date and no witness", func(v *ObjectView) { v.Closed = "" }},
		{"a malformed closed date", func(v *ObjectView) { v.Closed = "2024-1-10" }},
		{"an unparseable object", func(v *ObjectView) { v.Object = "not a ref" }},
		{"unknown carry", func(v *ObjectView) { v.Carry = "sideways" }},
		{"carried without a revision", func(v *ObjectView) { v.Carry = CarryCarried }},
		{"dropped without a revision", func(v *ObjectView) { v.Carry = CarryDropped }},
		{"carry unproven without a witness", func(v *ObjectView) { v.Carry = CarryUnproven }},
	}
	if _, err := ok.Lines(); err != nil {
		t.Fatalf("the well-formed view: %v", err)
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := ok
			tc.f(&v)
			if ls, err := v.Lines(); err == nil {
				t.Fatalf("rendered %q", ls)
			}
		})
	}
}

// TestDecisionView_LinesFailClosed pins the same for decision views.
func TestDecisionView_LinesFailClosed(t *testing.T) {
	ok := DecisionView{Decision: "spec/s2#dc-1", Edge: vo, Object: vo, State: DecisionInForce, Carried: true, Establisher: "spec/s1", Conflict: "conflict/c1", Since: vDay1}
	tests := []struct {
		name string
		f    func(*DecisionView)
	}{
		{"unknown state", func(v *DecisionView) { v.State = "gone" }},
		{"not established without a reason", func(v *DecisionView) { v.State = DecisionNotEstablished }},
		{"no object", func(v *DecisionView) { v.Object = "" }},
		{"an unparseable decision", func(v *DecisionView) { v.State, v.Decision = DecisionProposed, "dc-1" }},
		{"carried without its establisher", func(v *DecisionView) { v.Establisher = "" }},
		{"carried without its conflict", func(v *DecisionView) { v.Conflict = "" }},
		{"carried with a malformed date", func(v *DecisionView) { v.Since = "" }},
	}
	if _, err := ok.Lines(); err != nil {
		t.Fatalf("the well-formed view: %v", err)
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := ok
			tc.f(&v)
			if ls, err := v.Lines(); err == nil {
				t.Fatalf("rendered %q", ls)
			}
		})
	}
}

// TestIndex_Scenarios drives the views over built scenario stores with the
// real history: design §8's surfaces after acceptance, carried through a
// revision, dropped by one, not yet accepted, and not established.
func TestIndex_Scenarios(t *testing.T) {
	hermetic(t)
	ctx := context.Background()
	const (
		govF = "governed spec/closed-feature's completed work (closed 2024-01-10) | "
		govS = "governed spec/closed-story's completed work (closed 2024-01-10) | "
		byF  = "superseded since 2024-02-15 by spec/successor#dc-1"
		byS  = "superseded since 2024-02-15 by spec/successor#dc-2"
	)
	type look struct{ object, decision, want string } // object "T#o" or decision "S#dc-x"
	tests := []struct {
		name, scenario, branch string
		looks                  []look
	}{
		{"after acceptance", "accepted", "", []look{
			{"closed-feature#dc-1", "", govF + byF},
			{"closed-story#ac-1", "", govS + byS},
			{"closed-feature#ac-1", "", ""},
			{"", "successor#dc-1", "supersedes spec/closed-feature#dc-1"},
			{"", "successor#dc-2", "supersedes spec/closed-story#ac-1"},
		}},
		{"carried through two revisions", "chain", "", []look{
			{"closed-feature#dc-1", "", govF + byF + " | carried by spec/successor-v3"},
			{"closed-story#ac-1", "", govS + byS + " | carried by spec/successor-v3"},
			{"", "successor-v3#dc-2", "supersedes spec/closed-story#ac-1 | carries the replacement established by spec/successor (conflict/successor-closed-story, since 2024-02-15)"},
		}},
		{"a revision drops the edge", "chain-drop", "", []look{
			{"closed-feature#dc-1", "", govF + byF + " | no longer carried by the current revision (spec/successor-v2)"},
			{"closed-story#ac-1", "", govS + byS + " | carried by spec/successor-v2"},
		}},
		{"not yet accepted: the default branch shows nothing", "proposed", "main", []look{
			{"closed-feature#dc-1", "", ""},
			{"closed-story#ac-1", "", ""},
			{"", "successor#dc-1", ""},
		}},
		{"not yet accepted: the design branch shows it proposed", "proposed", "", []look{
			{"closed-feature#dc-1", "", ""},
			{"", "successor#dc-1", "proposed — supersedes spec/closed-feature#dc-1 when spec/successor is accepted"},
			{"", "successor#dc-2", "proposed — supersedes spec/closed-story#ac-1 when spec/successor is accepted"},
		}},
		{"not established on the design branch", "no-conflict", "", []look{
			{"", "successor#dc-1", "supersession not established: no conflict challenges spec/closed-feature#dc-1"},
			{"", "successor#dc-2", "proposed — supersedes spec/closed-story#ac-1 when spec/successor is accepted"},
		}},
		{"not established after acceptance: no supersession on the object", "chain-not-in-force", "main", []look{
			{"closed-feature#dc-1", "", ""},
			{"closed-feature#ac-1", "", ""},
			{"closed-story#ac-1", "", govS + byS},
			{"", "successor#dc-1", "supersession not established: spec/successor's supersession was not in force at its acceptance: no conflict challenges spec/closed-feature#ac-1"},
			{"", "successor#dc-3", "supersession not established: no conflict challenges spec/closed-feature#ac-1"},
		}},
		{"a proposed rival on the design branch never unseats the successor in force", "already-superseded", "", []look{
			{"closed-feature#dc-1", "", govF + "superseded since 2024-01-15 by spec/prior-successor#dc-1"},
			{"", "prior-successor#dc-1", "supersedes spec/closed-feature#dc-1"},
			{"", "successor#dc-1", "supersession not established: the object spec/closed-feature#dc-1 is already superseded by spec/prior-successor (conflict/prior-successor-closed-feature)"},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := scenario.Build(t, tc.scenario).Dir
			if tc.branch != "" {
				gitIn(t, dir, "checkout", "-q", tc.branch)
			}
			x, err := NewIndex(ctx, mustRead(t, WorkTree{Root: dir}), NewHistory(ctx, dir))
			if err != nil {
				t.Fatal(err)
			}
			for _, l := range tc.looks {
				var got string
				if spec, id, ok := strings.Cut(l.object, "#"); ok {
					got = lines(t, x.Object(spec, id))
				} else {
					spec, id, _ := strings.Cut(l.decision, "#")
					var all []string
					for _, v := range x.Decisions(spec, id) {
						all = append(all, lines(t, v))
						agrees(t, x, v)
					}
					got = strings.Join(all, " || ")
				}
				if got != l.want {
					t.Errorf("%s%s:\n got %q\nwant %q", l.object, l.decision, got, l.want)
				}
			}
		})
	}
}

// agrees fails t unless a new replacement's in-force decision view agrees
// with its object's view in the same index (lane L3c review F-1): the
// object is superseded by the decision's spec, through the same conflict,
// since the same date.
func agrees(t *testing.T, x *Index, v DecisionView) {
	t.Helper()
	if v.State != DecisionInForce || v.Carried {
		return
	}
	ref, err := artifact.ParseRef(v.Object)
	if err != nil {
		t.Fatalf("object %q: %v", v.Object, err)
	}
	o := x.Object(ref.Name, ref.Object)
	if o.State != ObjectSuperseded || !strings.HasPrefix(o.By, v.Establisher+"#") || o.Conflict != v.Conflict || o.Since != v.Since {
		t.Errorf("%s in force disagrees with its object's view:\n decision %+v\n object   %+v", v.Decision, v, o)
	}
}

// TestIndex_BelowGitRoot pins the L3 re-review's disclosure: below the git
// root History.Closed is unproven, so the object view shows the closed
// date as unproven and never invents one, while the supersession itself
// is proven.
func TestIndex_BelowGitRoot(t *testing.T) {
	hermetic(t)
	ctx := context.Background()
	root := underSubdir(t, scenario.Build(t, "accepted"))
	x, err := NewIndex(ctx, mustRead(t, WorkTree{Root: root}), NewHistory(ctx, root))
	if err != nil {
		t.Fatal(err)
	}
	v := x.Object("closed-feature", "dc-1")
	ls, err := v.Lines()
	if err != nil || v.State != ObjectSuperseded || v.Closed != "" || v.ClosedWitness == "" || len(ls) != 2 ||
		!strings.HasPrefix(ls[0], "governed spec/closed-feature's completed work (closed date unproven: ") ||
		ls[1] != "superseded since 2024-02-15 by spec/successor#dc-1" {
		t.Fatalf("got %+v, %q, %v", v, ls, err)
	}
}
