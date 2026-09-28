package objsupersede

import (
	"context"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// fakeEst answers Establishment from a map keyed by successor name; a
// successor it does not name is not accepted.
type fakeEst map[string]Establishment

func (f fakeEst) Establishment(_ context.Context, successor string, _ artifact.Ref) Establishment {
	if e, ok := f[successor]; ok {
		return e
	}
	return Establishment{Reason: ReasonEstablisherNotAccepted}
}

var inForce = fakeEst{
	"successor":       {Commit: "c1", Date: "2024-02-15"},
	"story-successor": {Commit: "c2", Date: "2024-02-16"},
}

// want is the part of a Result a case pins.
type want struct {
	outcome                  Outcome
	reason                   Reason
	other, conflict          string
	predecessor, detailStart string // detailStart: Detail's required prefix ("" pins an empty Detail)
}

func check(t *testing.T, r Result, w want) {
	t.Helper()
	detailOK := r.Detail == w.detailStart || (w.detailStart != "" && strings.HasPrefix(r.Detail, w.detailStart))
	if r.Outcome != w.outcome || r.Reason != w.reason || r.Other != w.other || r.Conflict != w.conflict || r.Predecessor != w.predecessor || !detailOK {
		t.Fatalf("got %s/%s other=%q conflict=%q predecessor=%q detail=%q (%+v)\nwant %s/%s other=%q conflict=%q predecessor=%q detail starting %q",
			r.Outcome, r.Reason, r.Other, r.Conflict, r.Predecessor, r.Detail, r, w.outcome, w.reason, w.other, w.conflict, w.predecessor, w.detailStart)
	}
}

func evaluate(t *testing.T, recs *Records, spec string) []Result {
	t.Helper()
	res, err := Evaluate(context.Background(), recs, spec, inForce)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	return res
}

func result(t *testing.T, res []Result, decision string) Result {
	t.Helper()
	for _, r := range res {
		if r.Decision == decision {
			return r
		}
	}
	t.Fatalf("no result for decision %q in %+v", decision, res)
	return Result{}
}

func edgeOf(recs *Records, spec, decision string) *artifact.Link {
	for i := range recs.Specs[spec].FM.Decisions {
		if d := &recs.Specs[spec].FM.Decisions[i]; d.ID == decision {
			return &d.Links[0]
		}
	}
	return nil
}

const (
	cFeature = "successor-closed-feature"
	cStory   = "successor-closed-story"
)

// TestEvaluate_Conditions walks design §5's ordered conditions (plus SI-271's
// pin and SI-274's additions) for a new replacement, one failing record set
// at a time, and pins that the first failing condition wins.
func TestEvaluate_Conditions(t *testing.T) {
	tests := []struct {
		name     string
		layers   []string
		mutate   func(*Records)
		spec, dc string
		want     want
	}{
		{"records match: decision target", []string{"successor", "conflict-feature", "conflict-story"}, nil, "successor", "dc-1", want{ResolvedNew, "", "", cFeature, "", ""}},
		{"records match: criterion target", []string{"successor", "conflict-feature", "conflict-story"}, nil, "successor", "dc-2", want{ResolvedNew, "", "", cStory, "", ""}},
		{"pinned edge", []string{"successor", "conflict-feature", "conflict-story"}, func(r *Records) {
			edgeOf(r, "successor", "dc-1").Ref = "spec/closed-feature@0a1b2c3#dc-1"
		}, "successor", "dc-1", want{Unresolved, ReasonPinned, "", "", "", ""}},
		{"pinned wins over a missing target", []string{"successor"}, func(r *Records) {
			edgeOf(r, "successor", "dc-1").Ref = "spec/nowhere@0a1b2c3#dc-1"
		}, "successor", "dc-1", want{Unresolved, ReasonPinned, "", "", "", ""}},
		{"undecodable record", []string{"successor", "conflict-feature", "conflict-story"}, func(r *Records) {
			r.Failures = []string{".verdi/conflicts/x.md: broken"}
		}, "successor", "dc-1", want{Unresolved, ReasonRecordsUndecodable, "", "", "", ".verdi/conflicts/x.md: broken"}},
		{"undecodable wins over a missing target (SI-274(6))", []string{"successor", "conflict-feature", "conflict-story"}, func(r *Records) {
			delete(r.Specs, "closed-feature")
			r.Failures = []string{".verdi/specs/archive/closed-feature/spec.md: broken"}
		}, "successor", "dc-1", want{Unresolved, ReasonRecordsUndecodable, "", "", "", ".verdi/specs/archive/closed-feature/spec.md: broken"}},
		{"pinned wins over undecodable (SI-271)", []string{"successor", "conflict-feature", "conflict-story"}, func(r *Records) {
			edgeOf(r, "successor", "dc-1").Ref = "spec/closed-feature@0a1b2c3#dc-1"
			r.Failures = []string{".verdi/conflicts/x.md: broken"}
		}, "successor", "dc-1", want{Unresolved, ReasonPinned, "", "", "", ""}},
		{"target missing", []string{"successor", "conflict-feature", "conflict-story"}, func(r *Records) {
			delete(r.Specs, "closed-feature")
		}, "successor", "dc-1", want{Unresolved, ReasonTargetMissing, "", "", "", ""}},
		{"target not closed", []string{"successor-not-closed", "conflict-not-closed", "conflict-story"}, nil, "successor", "dc-1", want{Unresolved, ReasonTargetNotClosed, "", "", "", ""}},
		{"target not closed wins over no conflict", []string{"successor-not-closed"}, nil, "successor", "dc-1", want{Unresolved, ReasonTargetNotClosed, "", "", "", ""}},
		{"an active-zone status: closed target is not closed (SI-277)", []string{"successor-not-closed", "conflict-not-closed"}, func(r *Records) {
			r.Specs["other-feature"].FM.Status = "closed"
		}, "successor", "dc-1", want{Unresolved, ReasonTargetNotClosed, "", "", "", ""}},
		{"object not declared", []string{"successor-undeclared", "conflict-feature-undeclared", "conflict-story"}, nil, "successor", "dc-1", want{Unresolved, ReasonObjectNotDeclared, "", "", "", ""}},
		{"object not declared wins over no conflict", []string{"successor-undeclared"}, nil, "successor", "dc-1", want{Unresolved, ReasonObjectNotDeclared, "", "", "", ""}},
		{"object is a constraint", []string{"successor-constraint", "conflict-feature-constraint", "conflict-story"}, nil, "successor", "dc-1", want{Unresolved, ReasonObjectNotTarget, "", "", "", ""}},
		{"already superseded by another successor", []string{"prior", "successor", "conflict-feature", "conflict-story"}, nil, "successor", "dc-1", want{Unresolved, ReasonAlreadySuperseded, "prior-successor", "prior-successor-closed-feature", "", ""}},
		{"already superseded wins over resolved_by another spec", []string{"prior", "successor", "conflict-story"}, nil, "successor", "dc-1", want{Unresolved, ReasonAlreadySuperseded, "prior-successor", "prior-successor-closed-feature", "", ""}},
		{"unrelated reuse of an established conflict", []string{"successor", "conflict-feature", "conflict-story", "unrelated"}, nil, "unrelated", "dc-1", want{Unresolved, ReasonAlreadySuperseded, "successor", cFeature, "", ""}},
		{"no conflict", []string{"successor", "conflict-story"}, nil, "successor", "dc-1", want{Unresolved, ReasonNoConflict, "", "", "", ""}},
		{"a pinned challenge never matches", []string{"successor", "conflict-feature", "conflict-story"}, func(r *Records) {
			r.Conflicts[0].FM.Links[0].Ref = "spec/closed-feature@0a1b2c3#dc-1"
		}, "successor", "dc-1", want{Unresolved, ReasonNoConflict, "", "", "", ""}},
		{"open conflict", []string{"successor", "conflict-feature-open", "conflict-story"}, nil, "successor", "dc-1", want{Unresolved, ReasonConflictNotSuperseded, "", cFeature, "", ""}},
		{"dismissed conflict", []string{"successor", "conflict-feature-dismissed", "conflict-story"}, nil, "successor", "dc-1", want{Unresolved, ReasonConflictNotSuperseded, "", cFeature, "", ""}},
		{"resolved_by names another spec that carries no edge", []string{"successor", "conflict-feature-other", "conflict-story"}, nil, "successor", "dc-1", want{Unresolved, ReasonResolvedByOther, "other-feature", cFeature, "", ""}},
		{"more than one conflict for one closed spec and successor", []string{"successor", "conflict-feature", "conflict-story"}, func(r *Records) {
			dup := *r.Conflicts[0]
			dup.Name = "successor-closed-feature-again"
			r.Conflicts = append(r.Conflicts, &dup)
		}, "successor", "dc-1", want{Unresolved, ReasonMultipleConflicts, "successor", cFeature, "", ""}},
		{"more than one conflict wins over spans (SI-274(1) before (2))", []string{"successor", "conflict-feature-spans"}, func(r *Records) {
			dup := *r.Conflicts[0]
			dup.Name = "successor-closed-feature-again"
			r.Conflicts = append(r.Conflicts, &dup)
		}, "successor", "dc-1", want{Unresolved, ReasonMultipleConflicts, "successor", cFeature, "", ""}},
		{"conflict spans two specs (decision)", []string{"successor", "conflict-feature-spans"}, nil, "successor", "dc-1", want{Unresolved, ReasonConflictSpansSpecs, "", cFeature, "", ""}},
		{"conflict spans two specs (criterion)", []string{"successor", "conflict-feature-spans"}, nil, "successor", "dc-2", want{Unresolved, ReasonConflictSpansSpecs, "", cFeature, "", ""}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recs := mustRead(t, layerTree(t, tc.layers...))
			if tc.mutate != nil {
				tc.mutate(recs)
			}
			check(t, result(t, evaluate(t, recs, tc.spec), tc.dc), tc.want)
		})
	}
}

// TestEvaluate_Completeness pins §5's completeness result for the issuing
// successor, and its absence for a later revision.
func TestEvaluate_Completeness(t *testing.T) {
	tests := []struct {
		name, spec string
		layers     []string
		mutate     func(*Records)
		want       []string // Edge of each completeness result, in order
	}{
		{"every challenged fragment has an edge", "successor", []string{"successor", "conflict-feature", "conflict-story"}, nil, nil},
		{"challenged criterion with no edge", "successor", []string{"successor", "conflict-feature-unmatched", "conflict-story"}, nil, []string{"spec/closed-feature#ac-1"}},
		{"pinned challenge is unmatched", "successor", []string{"successor", "conflict-feature", "conflict-story"}, func(r *Records) {
			r.Conflicts[0].FM.Links[0].Ref = "spec/closed-feature@0a1b2c3#dc-1"
		}, []string{"spec/closed-feature@0a1b2c3#dc-1"}},
		{"undecodable records leave completeness unproven", "successor", []string{"successor", "conflict-feature", "conflict-story"}, func(r *Records) {
			r.Failures = []string{"x: broken"}
		}, []string{""}},
		{"a later revision is not the issuing successor", "successor-v2", []string{"successor", "conflict-feature-unmatched", "conflict-story", "successor-v2"}, nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recs := mustRead(t, layerTree(t, tc.layers...))
			if tc.mutate != nil {
				tc.mutate(recs)
			}
			var got []string
			for _, r := range evaluate(t, recs, tc.spec) {
				if r.Decision == "" {
					if r.Outcome != Unresolved || (r.Reason != ReasonUnmatchedChallenge && r.Reason != ReasonRecordsUndecodable) {
						t.Fatalf("completeness result %+v", r)
					}
					got = append(got, r.Edge)
				}
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("completeness %q, want %q", got, tc.want)
			}
		})
	}
}

// TestEvaluate_Carried pins SI-265/SI-273: a carried candidate through a
// feature manifest's buckets or a story's same id, condition 9 naming the
// broken step, new replacements on a revision, and SI-274(3)'s
// establishment reasons.
func TestEvaluate_Carried(t *testing.T) {
	chain := []string{"successor", "conflict-feature", "conflict-story", "successor-v2"}
	bucket := func(to string) func(*Records) {
		return func(r *Records) {
			s := r.Specs["successor-v2"].FM.Supersession
			s.Carried = []string{"ac-1", "dc-2"}
			note := []artifact.SupersessionNote{{ID: "dc-1", Note: "n"}}
			switch to {
			case "amended":
				s.Amended = note
			case "amended_advisory":
				s.AmendedAdvisory = note
			case "added":
				s.Added = []string{"dc-1"}
			}
		}
	}
	tests := []struct {
		name     string
		layers   []string
		mutate   func(*Records)
		est      fakeEst
		spec, dc string
		want     want
	}{
		{"carried bucket", chain, nil, inForce, "successor-v2", "dc-1", want{ResolvedCarried, "", "successor", cFeature, "", ""}},
		{"amended bucket", chain, bucket("amended"), inForce, "successor-v2", "dc-1", want{ResolvedCarried, "", "successor", cFeature, "", ""}},
		{"amended_advisory bucket", chain, bucket("amended_advisory"), inForce, "successor-v2", "dc-1", want{ResolvedCarried, "", "successor", cFeature, "", ""}},
		{"classified added: condition 9", chain, bucket("added"), inForce, "successor-v2", "dc-1", want{Unresolved, ReasonCarryMismatch, "successor-v2", cFeature, "successor", ""}},
		{"two steps back", append(chain, "successor-v3"), nil, inForce, "successor-v3", "dc-1", want{ResolvedCarried, "", "successor", cFeature, "", ""}},
		{"broken earlier step: condition 9 names it", append(chain, "successor-v3"), bucket("added"), inForce, "successor-v3", "dc-1", want{Unresolved, ReasonCarryMismatch, "successor-v2", cFeature, "successor", ""}},
		{"predecessor lacked the edge: new, already superseded", []string{"successor", "conflict-feature", "conflict-story", "successor-v2-drops", "successor-v3"}, nil, inForce, "successor-v3", "dc-1", want{Unresolved, ReasonAlreadySuperseded, "successor", cFeature, "", ""}},
		{"added decision on a revision: already superseded", chain, func(r *Records) {
			fm := r.Specs["successor-v2"].FM
			fm.Decisions = append(fm.Decisions, artifact.Decision{ID: "dc-3", Links: []artifact.Link{{Type: artifact.LinkSupersedes, Ref: "spec/closed-feature#dc-1"}}})
		}, inForce, "successor-v2", "dc-3", want{Unresolved, ReasonAlreadySuperseded, "successor", cFeature, "", ""}},
		{"story revision keeps the id", []string{"story-chain"}, nil, inForce, "story-successor-v2", "dc-1", want{ResolvedCarried, "", "story-successor", "story-successor-closed-feature", "", ""}},
		{"story revision renames the decision: new", []string{"story-chain", "story-successor-v2-renamed"}, nil, inForce, "story-successor-v2", "dc-2", want{Unresolved, ReasonAlreadySuperseded, "story-successor", "story-successor-closed-feature", "", ""}},
		{"establishing successor not accepted", chain, nil, fakeEst{}, "successor-v2", "dc-1", want{Unresolved, ReasonEstablisherNotAccepted, "successor", cFeature, "", ""}},
		{"establishing successor not in force", chain, nil, fakeEst{"successor": {Reason: ReasonEstablisherNotInForce, Detail: "no conflict challenges spec/closed-feature#dc-1"}}, "successor-v2", "dc-1", want{Unresolved, ReasonEstablisherNotInForce, "successor", cFeature, "", "no conflict challenges spec/closed-feature#dc-1"}},
		{"acceptance unproven", chain, nil, fakeEst{"successor": {Reason: ReasonAcceptanceUnproven, Detail: "shallow history"}}, "successor-v2", "dc-1", want{Unresolved, ReasonAcceptanceUnproven, "successor", cFeature, "", "shallow history"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recs := mustRead(t, layerTree(t, tc.layers...))
			if tc.mutate != nil {
				tc.mutate(recs)
			}
			res, err := Evaluate(context.Background(), recs, tc.spec, tc.est)
			if err != nil {
				t.Fatal(err)
			}
			r := result(t, res, tc.dc)
			check(t, r, tc.want)
			if r.Outcome == ResolvedCarried && r.Since != tc.est[tc.want.other].Date {
				t.Fatalf("since %q, want the establishing successor's date", r.Since)
			}
		})
	}
}

func TestEvaluate_Negative(t *testing.T) {
	recs := mustRead(t, layerTree(t, "successor"))
	ctx := context.Background()
	if _, err := Evaluate(ctx, recs, "nowhere", inForce); err == nil {
		t.Error("a spec absent from the records evaluated")
	}
	if _, err := Evaluate(ctx, recs, "successor", nil); err == nil {
		t.Error("a nil establisher was accepted")
	}
	if _, err := Evaluate(ctx, nil, "successor", inForce); err == nil {
		t.Error("nil records were accepted")
	}
	res, err := Evaluate(ctx, mustRead(t, layerTree(t)), "other-feature", inForce)
	if err != nil || len(res) != 0 {
		t.Errorf("a spec with no fragment supersedes edge: %v, %v", res, err)
	}
}

// In-memory records for chains the committed fixture does not hold.

func mSpec(name string, preds []string, decs ...artifact.Decision) *Spec {
	fm := &artifact.SpecFrontmatter{Class: artifact.ClassFeature}
	fm.ID = "spec/" + name
	for _, p := range preds {
		fm.Links = append(fm.Links, artifact.Link{Type: artifact.LinkSupersedes, Ref: "spec/" + p})
	}
	fm.Decisions = decs
	return &Spec{Name: name, FM: fm}
}

func mDec(id string, edges ...string) artifact.Decision {
	d := artifact.Decision{ID: id}
	for _, e := range edges {
		d.Links = append(d.Links, artifact.Link{Type: artifact.LinkSupersedes, Ref: e})
	}
	return d
}

func mCarries(s *Spec, ids ...string) *Spec {
	s.FM.Supersession = &artifact.Supersession{Carried: ids}
	return s
}

func mConflict(name, resolvedBy string, challenges ...string) *Conflict {
	fm := &artifact.ConflictFrontmatter{Status: "superseded", ResolvedBy: "spec/" + resolvedBy}
	fm.ID = "conflict/" + name
	for _, c := range challenges {
		fm.Links = append(fm.Links, artifact.Link{Type: artifact.LinkChallenges, Ref: c})
	}
	return &Conflict{Name: name, FM: fm}
}

// mClosed is a closed feature declaring ac-1, dc-1, and dc-2.
func mClosed(name string) *Spec {
	t := mSpec(name, nil, mDec("dc-1"), mDec("dc-2"))
	t.Archived = true
	t.FM.AcceptanceCriteria = []artifact.AcceptanceCriterion{{ID: "ac-1"}}
	return t
}

// mRecs holds the closed specs t and u, plus specs and conflicts.
func mRecs(conflicts []*Conflict, specs ...*Spec) *Records {
	r := &Records{Specs: map[string]*Spec{"t": mClosed("t"), "u": mClosed("u")}, Conflicts: conflicts}
	for _, s := range specs {
		r.Specs[s.Name] = s
	}
	return r
}

// TestEvaluate_Establisher pins SI-276 (S_k is the one named chain member
// in force at its acceptance, chosen before condition 9), both edge halves
// of condition 9 past step 0, and the Establisher contract (review b M-4).
func TestEvaluate_Establisher(t *testing.T) {
	const o = "spec/t#dc-1"
	day := func(d string) Establishment { return Establishment{Commit: "c", Date: d} }
	notIn := Establishment{Reason: ReasonEstablisherNotInForce, Detail: "no conflict challenges spec/t#ac-1"}
	unproven := Establishment{Reason: ReasonAcceptanceUnproven, Detail: "shallow history"}
	// three: s3 carries dc-1 from s2, which carries it from s1; c1 names s1, c2 names s2.
	three := mRecs([]*Conflict{mConflict("c1", "s1", o), mConflict("c2", "s2", o)},
		mSpec("s1", nil, mDec("dc-1", o)), mCarries(mSpec("s2", []string{"s1"}, mDec("dc-1", o)), "dc-1"),
		mCarries(mSpec("s3", []string{"s2"}, mDec("dc-1", o)), "dc-1"))
	// review b I-2's witness: s1 named but carried no edge; s2 added dc-5.
	earliest := mRecs([]*Conflict{mConflict("c1", "s1", o), mConflict("c2", "s2", o)},
		mSpec("s1", nil, mDec("dc-1")), mSpec("s2", []string{"s1"}, mDec("dc-1"), mDec("dc-5", o)),
		mCarries(mSpec("s3", []string{"s2"}, mDec("dc-1"), mDec("dc-5", o)), "dc-1", "dc-5"))
	// s1 is named and in force but its dc-1 lacks the edge: step 1 breaks.
	predEdge := mRecs([]*Conflict{mConflict("c1", "s1", o)},
		mSpec("s1", nil, mDec("dc-1", "spec/t#dc-2")), mCarries(mSpec("s2", []string{"s1"}, mDec("dc-1", o)), "dc-1"),
		mCarries(mSpec("s3", []string{"s2"}, mDec("dc-1", o)), "dc-1"))
	// carry: s3 carries dc-1 from s2, which carries it from s1; c1 names s1,
	// plus extra conflicts.
	carry := func(extra ...*Conflict) *Records {
		return mRecs(append([]*Conflict{mConflict("c1", "s1", o)}, extra...),
			mSpec("s1", nil, mDec("dc-1", o)), mCarries(mSpec("s2", []string{"s1"}, mDec("dc-1", o)), "dc-1"),
			mCarries(mSpec("s3", []string{"s2"}, mDec("dc-1", o)), "dc-1"))
	}
	spans := carry()
	spans.Conflicts[0].FM.Links = append(spans.Conflicts[0].FM.Links, artifact.Link{Type: artifact.LinkChallenges, Ref: "spec/u#dc-1"})
	outsider := carry(mConflict("cx", "x", o))
	outsider.Specs["x"] = mSpec("x", nil, mDec("dc-9", o))
	// class: s2 classifies dc-1 as added, so step 1 breaks on classification.
	withClass := func(extra ...*Conflict) *Records {
		r := carry(extra...)
		r.Specs["s2"].FM.Supersession = &artifact.Supersession{Added: []string{"dc-1"}}
		return r
	}
	class := withClass()
	tests := []struct {
		name     string
		recs     *Records
		est      fakeEst
		spec, dc string
		want     want
	}{
		{"the earliest named member established nothing; the nearest is S_k", earliest, fakeEst{"s1": notIn, "s2": day("2024-05-01")}, "s3", "dc-5", want{ResolvedCarried, "", "s2", "c2", "", ""}},
		{"the earliest named member is S_k; the nearest is not in force", three, fakeEst{"s1": day("2024-02-15"), "s2": notIn}, "s3", "dc-1", want{ResolvedCarried, "", "s1", "c1", "", ""}},
		{"two named members in force: unproven, naming both", three, fakeEst{"s1": day("2024-02-15"), "s2": day("2024-03-15")}, "s3", "dc-1", want{Unresolved, ReasonAcceptanceUnproven, "", "", "", "more than one revision's supersession is in force: spec/s2, spec/s1"}},
		{"none in force: the earliest named member's reason", three, fakeEst{"s2": notIn}, "s3", "dc-1", want{Unresolved, ReasonEstablisherNotAccepted, "s1", "c1", "", ""}},
		{"none in force, one unproven: acceptance unproven", three, fakeEst{"s1": notIn, "s2": unproven}, "s3", "dc-1", want{Unresolved, ReasonAcceptanceUnproven, "s2", "c2", "", "shallow history"}},
		{"one in force, another unproven: acceptance unproven", three, fakeEst{"s1": day("2024-02-15"), "s2": unproven}, "s3", "dc-1", want{Unresolved, ReasonAcceptanceUnproven, "s2", "c2", "", "shallow history"}},
		{"no member in force wins over a broken step", class, fakeEst{"s1": notIn}, "s3", "dc-1", want{Unresolved, ReasonEstablisherNotInForce, "s1", "c1", "", "no conflict challenges spec/t#ac-1"}},
		{"condition 9: classification broken at step 1", class, fakeEst{"s1": day("2024-02-15")}, "s3", "dc-1", want{Unresolved, ReasonCarryMismatch, "s2", "c1", "s1", ""}},
		{"condition 9: predecessor edge broken at step 1", predEdge, fakeEst{"s1": day("2024-02-15")}, "s3", "dc-1", want{Unresolved, ReasonCarryMismatch, "s2", "c1", "s1", ""}},
		{"two named members unproven: the earliest is named", three, fakeEst{"s1": {Reason: ReasonAcceptanceUnproven, Detail: "witness s1"}, "s2": {Reason: ReasonAcceptanceUnproven, Detail: "witness s2"}}, "s3", "dc-1", want{Unresolved, ReasonAcceptanceUnproven, "s1", "c1", "", "witness s1"}},
		{"SI-274(1) on the carried path: S_k's second conflict for T", carry(mConflict("c1b", "s1", "spec/t#ac-1")), fakeEst{"s1": day("2024-02-15")}, "s3", "dc-1", want{Unresolved, ReasonMultipleConflicts, "s1", "c1", "", ""}},
		{"SI-274(2) on the carried path: S_k's conflict spans two specs", spans, fakeEst{"s1": day("2024-02-15")}, "s3", "dc-1", want{Unresolved, ReasonConflictSpansSpecs, "", "c1", "", ""}},
		{"SI-274(1) wins over a broken step on the carried path", withClass(mConflict("c1b", "s1", "spec/t#ac-1")), fakeEst{"s1": day("2024-02-15")}, "s3", "dc-1", want{Unresolved, ReasonMultipleConflicts, "s1", "c1", "", ""}},
		{"a named member not in force never blocks the carry (R3)", carry(mConflict("c2", "s2", o), mConflict("c2b", "s2", "spec/t#ac-1")), fakeEst{"s1": day("2024-02-15"), "s2": notIn}, "s3", "dc-1", want{ResolvedCarried, "", "s1", "c1", "", ""}},
		{"condition 5 never refuses a carried edge", outsider, fakeEst{"s1": day("2024-02-15")}, "s3", "dc-1", want{ResolvedCarried, "", "s1", "c1", "", ""}},
		{"in force without a date is unproven", three, fakeEst{"s1": {Commit: "c"}}, "s2", "dc-1", want{Unresolved, ReasonAcceptanceUnproven, "s1", "c1", "", "spec/s1 is reported in force without a YYYY-MM-DD acceptance date"}},
		{"in force with a malformed date is unproven", three, fakeEst{"s1": day("15 Feb 2024")}, "s2", "dc-1", want{Unresolved, ReasonAcceptanceUnproven, "s1", "c1", "", "spec/s1 is reported in force without a YYYY-MM-DD acceptance date"}},
		{"a foreign reason is never passed through", three, fakeEst{"s1": {Reason: ReasonNoConflict}}, "s2", "dc-1", want{Unresolved, ReasonAcceptanceUnproven, "s1", "c1", "", `the establishment of spec/s1 answered "no-conflict"`}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Evaluate(context.Background(), tc.recs, tc.spec, tc.est)
			if err != nil {
				t.Fatal(err)
			}
			r := result(t, res, tc.dc)
			check(t, r, tc.want)
			if r.Outcome == ResolvedCarried && r.Since != tc.est[r.Other].Date {
				t.Fatalf("since %q, want S_k's date %q", r.Since, tc.est[r.Other].Date)
			}
		})
	}
}

// TestChain pins SI-274(9)'s walk: exactly one predecessor ref at every
// step; a missing predecessor or a cycle ends it (review b M-3).
func TestChain(t *testing.T) {
	const o = "spec/t#dc-1"
	mk := func(s2preds []string, withS1 bool) *Records {
		specs := []*Spec{mCarries(mSpec("s2", s2preds, mDec("dc-1", o)), "dc-1")}
		if withS1 {
			specs = append(specs, mSpec("s1", []string{"s2"}, mDec("dc-1", o)))
		}
		return mRecs([]*Conflict{mConflict("c1", "s1", o)}, specs...)
	}
	tests := []struct {
		name  string
		recs  *Records
		chain string
		want  want
	}{
		{"a cycle ends the walk", mk([]string{"s1"}, true), "s2,s1", want{ResolvedCarried, "", "s1", "c1", "", ""}},
		{"two predecessor refs end it", mk([]string{"s1", "t"}, true), "s2", want{Unresolved, ReasonAlreadySuperseded, "s1", "c1", "", ""}},
		{"a missing predecessor ends it", mk([]string{"s1"}, false), "s2", want{Unresolved, ReasonResolvedByOther, "s1", "c1", "", ""}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := strings.Join(tc.recs.chain("s2"), ","); got != tc.chain {
				t.Fatalf("chain %s, want %s", got, tc.chain)
			}
			res, err := Evaluate(context.Background(), tc.recs, "s2", fakeEst{"s1": {Commit: "c", Date: "2024-02-15"}})
			if err != nil {
				t.Fatal(err)
			}
			check(t, result(t, res, "dc-1"), tc.want)
		})
	}
}

// TestInForceAt pins SI-275's in-force check on an acceptance commit's
// records: §3's whole match for (S_k, T), carried candidates excluded, o's
// own edge new, completeness over T; and SI-274(6)'s unproven decode
// failure there (review b I-1).
func TestInForceAt(t *testing.T) {
	ctx := context.Background()
	accepted := []string{"successor", "conflict-feature", "conflict-story"}
	sibling := func(ref string) func(*Records) {
		return func(r *Records) {
			fm := r.Specs["successor"].FM
			fm.Decisions = append(fm.Decisions, mDec("dc-3", ref))
		}
	}
	const o, p = "spec/t#dc-1", "spec/t#ac-1"
	// s1 supersedes an object of t and one of u; its conflict for u leaves
	// u#ac-1 without an edge.
	twoSpecs := mRecs([]*Conflict{mConflict("c1", "s1", o), mConflict("c2", "s1", "spec/u#dc-1", "spec/u#ac-1")},
		mSpec("s1", nil, mDec("dc-1", o), mDec("dc-2", "spec/u#dc-1")))
	// v2 carries dc-1 from s1 and newly issues dc-3 with its own conflict.
	carriesAndIssues := mRecs([]*Conflict{mConflict("c1", "s1", o), mConflict("c2", "v2", p)},
		mSpec("s1", nil, mDec("dc-1", o)), mCarries(mSpec("v2", []string{"s1"}, mDec("dc-1", o), mDec("dc-3", p)), "dc-1"))
	tests := []struct {
		name         string
		layers       []string
		mutate       func(*Records)
		recs         *Records
		successor    string
		object       artifact.Ref
		reason       Reason
		detailPrefix string
	}{
		{"in force", accepted, nil, nil, "successor", obj("closed-feature", "dc-1"), "", ""},
		{"a sibling edge to T without a conflict", accepted, sibling("spec/closed-feature#ac-1"), nil, "successor", obj("closed-feature", "dc-1"), ReasonEstablisherNotInForce, "no conflict challenges spec/closed-feature#ac-1"},
		{"a sibling edge to an undeclared object of T", accepted, sibling("spec/closed-feature#dc-9"), nil, "successor", obj("closed-feature", "dc-1"), ReasonEstablisherNotInForce, "the object spec/closed-feature#dc-9 is not declared"},
		{"a sibling edge to another closed spec is not T's match", accepted, sibling("spec/other-feature#dc-1"), nil, "successor", obj("closed-feature", "dc-1"), "", ""},
		{"completeness over T", []string{"successor", "conflict-feature-unmatched", "conflict-story"}, nil, nil, "successor", obj("closed-feature", "dc-1"), ReasonEstablisherNotInForce, "conflict/successor-closed-feature challenges spec/closed-feature#ac-1, but spec/successor carries no matching edge"},
		{"no edge to the object", accepted, nil, nil, "successor", obj("closed-feature", "ac-1"), ReasonEstablisherNotInForce, "spec/successor carries no edge to spec/closed-feature#ac-1"},
		{"a carried candidate is excluded", nil, nil, carriesAndIssues, "v2", obj("t", "ac-1"), "", ""},
		{"the object's own edge is carried, not new", nil, nil, carriesAndIssues, "v2", obj("t", "dc-1"), ReasonEstablisherNotInForce, "spec/v2 carries its edge to spec/t#dc-1 from its predecessor"},
		{"a decode failure is unproven, naming the path", accepted, func(r *Records) {
			r.Failures = []string{".verdi/specs/active/zz/spec.md: broken"}
		}, nil, "successor", obj("closed-feature", "dc-1"), ReasonAcceptanceUnproven, "records do not decode at the acceptance commit: .verdi/specs/active/zz/spec.md: broken"},
		{"completeness is scoped to T: a gap over another closed spec", nil, nil, twoSpecs, "s1", obj("t", "dc-1"), "", ""},
		{"completeness is scoped to T: a gap over T itself", nil, nil, twoSpecs, "s1", obj("u", "dc-1"), ReasonEstablisherNotInForce, "conflict/c2 challenges spec/u#ac-1, but spec/s1 carries no matching edge"},
		{"the successor absent from its acceptance tree", accepted, nil, nil, "nowhere", obj("closed-feature", "dc-1"), ReasonAcceptanceUnproven, "spec/nowhere is not in its acceptance commit's tree"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recs := tc.recs
			if recs == nil {
				recs = mustRead(t, layerTree(t, tc.layers...))
			}
			if tc.mutate != nil {
				tc.mutate(recs)
			}
			reason, detail := recs.inForceAt(ctx, tc.successor, tc.object, fakeEst{})
			if reason != tc.reason || !strings.HasPrefix(detail, tc.detailPrefix) || (tc.detailPrefix == "") != (detail == "") {
				t.Fatalf("got %q %q, want %q with detail starting %q", reason, detail, tc.reason, tc.detailPrefix)
			}
		})
	}
}
