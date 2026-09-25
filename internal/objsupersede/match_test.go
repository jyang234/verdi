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
	outcome        Outcome
	reason         Reason
	other, conflit string
}

func check(t *testing.T, r Result, w want) {
	t.Helper()
	if r.Outcome != w.outcome || r.Reason != w.reason || r.Other != w.other || r.Conflict != w.conflit {
		t.Fatalf("got %s/%s other=%q conflict=%q (%+v), want %s/%s other=%q conflict=%q",
			r.Outcome, r.Reason, r.Other, r.Conflict, r, w.outcome, w.reason, w.other, w.conflit)
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
		{"records match: decision target", []string{"successor", "conflict-feature", "conflict-story"}, nil, "successor", "dc-1", want{ResolvedNew, "", "", cFeature}},
		{"records match: criterion target", []string{"successor", "conflict-feature", "conflict-story"}, nil, "successor", "dc-2", want{ResolvedNew, "", "", cStory}},
		{"pinned edge", []string{"successor", "conflict-feature", "conflict-story"}, func(r *Records) {
			edgeOf(r, "successor", "dc-1").Ref = "spec/closed-feature@0a1b2c3#dc-1"
		}, "successor", "dc-1", want{Unresolved, ReasonPinned, "", ""}},
		{"pinned wins over a missing target", []string{"successor"}, func(r *Records) {
			edgeOf(r, "successor", "dc-1").Ref = "spec/nowhere@0a1b2c3#dc-1"
		}, "successor", "dc-1", want{Unresolved, ReasonPinned, "", ""}},
		{"undecodable record", []string{"successor", "conflict-feature", "conflict-story"}, func(r *Records) {
			r.Failures = []string{".verdi/conflicts/x.md: broken"}
		}, "successor", "dc-1", want{Unresolved, ReasonRecordsUndecodable, "", ""}},
		{"target missing", []string{"successor", "conflict-feature", "conflict-story"}, func(r *Records) {
			delete(r.Specs, "closed-feature")
		}, "successor", "dc-1", want{Unresolved, ReasonTargetMissing, "", ""}},
		{"target not closed", []string{"successor-not-closed", "conflict-not-closed", "conflict-story"}, nil, "successor", "dc-1", want{Unresolved, ReasonTargetNotClosed, "", ""}},
		{"target not closed wins over no conflict", []string{"successor-not-closed"}, nil, "successor", "dc-1", want{Unresolved, ReasonTargetNotClosed, "", ""}},
		{"legacy status closed counts as closed", []string{"successor-not-closed", "conflict-not-closed"}, func(r *Records) {
			r.Specs["other-feature"].FM.Status = "closed"
		}, "successor", "dc-1", want{ResolvedNew, "", "", "successor-other-feature"}},
		{"object not declared", []string{"successor-undeclared", "conflict-feature-undeclared", "conflict-story"}, nil, "successor", "dc-1", want{Unresolved, ReasonObjectNotDeclared, "", ""}},
		{"object not declared wins over no conflict", []string{"successor-undeclared"}, nil, "successor", "dc-1", want{Unresolved, ReasonObjectNotDeclared, "", ""}},
		{"object is a constraint", []string{"successor-constraint", "conflict-feature-constraint", "conflict-story"}, nil, "successor", "dc-1", want{Unresolved, ReasonObjectNotTarget, "", ""}},
		{"already superseded by another successor", []string{"prior", "successor", "conflict-feature", "conflict-story"}, nil, "successor", "dc-1", want{Unresolved, ReasonAlreadySuperseded, "prior-successor", "prior-successor-closed-feature"}},
		{"already superseded wins over resolved_by another spec", []string{"prior", "successor", "conflict-story"}, nil, "successor", "dc-1", want{Unresolved, ReasonAlreadySuperseded, "prior-successor", "prior-successor-closed-feature"}},
		{"unrelated reuse of an established conflict", []string{"successor", "conflict-feature", "conflict-story", "unrelated"}, nil, "unrelated", "dc-1", want{Unresolved, ReasonAlreadySuperseded, "successor", cFeature}},
		{"no conflict", []string{"successor", "conflict-story"}, nil, "successor", "dc-1", want{Unresolved, ReasonNoConflict, "", ""}},
		{"a pinned challenge never matches", []string{"successor", "conflict-feature", "conflict-story"}, func(r *Records) {
			r.Conflicts[0].FM.Links[0].Ref = "spec/closed-feature@0a1b2c3#dc-1"
		}, "successor", "dc-1", want{Unresolved, ReasonNoConflict, "", ""}},
		{"open conflict", []string{"successor", "conflict-feature-open", "conflict-story"}, nil, "successor", "dc-1", want{Unresolved, ReasonConflictNotSuperseded, "", cFeature}},
		{"dismissed conflict", []string{"successor", "conflict-feature-dismissed", "conflict-story"}, nil, "successor", "dc-1", want{Unresolved, ReasonConflictNotSuperseded, "", cFeature}},
		{"resolved_by names another spec that carries no edge", []string{"successor", "conflict-feature-other", "conflict-story"}, nil, "successor", "dc-1", want{Unresolved, ReasonResolvedByOther, "other-feature", cFeature}},
		{"more than one conflict for one closed spec and successor", []string{"successor", "conflict-feature", "conflict-story"}, func(r *Records) {
			dup := *r.Conflicts[0]
			dup.Name = "successor-closed-feature-again"
			r.Conflicts = append(r.Conflicts, &dup)
		}, "successor", "dc-1", want{Unresolved, ReasonMultipleConflicts, "successor", cFeature}},
		{"conflict spans two specs (decision)", []string{"successor", "conflict-feature-spans"}, nil, "successor", "dc-1", want{Unresolved, ReasonConflictSpansSpecs, "", cFeature}},
		{"conflict spans two specs (criterion)", []string{"successor", "conflict-feature-spans"}, nil, "successor", "dc-2", want{Unresolved, ReasonConflictSpansSpecs, "", cFeature}},
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
		{"carried bucket", chain, nil, inForce, "successor-v2", "dc-1", want{ResolvedCarried, "", "successor", cFeature}},
		{"amended bucket", chain, bucket("amended"), inForce, "successor-v2", "dc-1", want{ResolvedCarried, "", "successor", cFeature}},
		{"amended_advisory bucket", chain, bucket("amended_advisory"), inForce, "successor-v2", "dc-1", want{ResolvedCarried, "", "successor", cFeature}},
		{"classified added: condition 9", chain, bucket("added"), inForce, "successor-v2", "dc-1", want{Unresolved, ReasonCarryMismatch, "successor-v2", cFeature}},
		{"two steps back", append(chain, "successor-v3"), nil, inForce, "successor-v3", "dc-1", want{ResolvedCarried, "", "successor", cFeature}},
		{"broken earlier step: condition 9 names it", append(chain, "successor-v3"), bucket("added"), inForce, "successor-v3", "dc-1", want{Unresolved, ReasonCarryMismatch, "successor-v2", cFeature}},
		{"predecessor lacked the edge: new, already superseded", []string{"successor", "conflict-feature", "conflict-story", "successor-v2-drops", "successor-v3"}, nil, inForce, "successor-v3", "dc-1", want{Unresolved, ReasonAlreadySuperseded, "successor", cFeature}},
		{"added decision on a revision: already superseded", chain, func(r *Records) {
			fm := r.Specs["successor-v2"].FM
			fm.Decisions = append(fm.Decisions, artifact.Decision{ID: "dc-3", Links: []artifact.Link{{Type: artifact.LinkSupersedes, Ref: "spec/closed-feature#dc-1"}}})
		}, inForce, "successor-v2", "dc-3", want{Unresolved, ReasonAlreadySuperseded, "successor", cFeature}},
		{"story revision keeps the id", []string{"story-chain"}, nil, inForce, "story-successor-v2", "dc-1", want{ResolvedCarried, "", "story-successor", "story-successor-closed-feature"}},
		{"story revision renames the decision: new", []string{"story-chain", "story-successor-v2-renamed"}, nil, inForce, "story-successor-v2", "dc-2", want{Unresolved, ReasonAlreadySuperseded, "story-successor", "story-successor-closed-feature"}},
		{"establishing successor not accepted", chain, nil, fakeEst{}, "successor-v2", "dc-1", want{Unresolved, ReasonEstablisherNotAccepted, "successor", cFeature}},
		{"establishing successor not in force", chain, nil, fakeEst{"successor": {Reason: ReasonEstablisherNotInForce, Detail: "no conflict challenges spec/closed-feature#dc-1"}}, "successor-v2", "dc-1", want{Unresolved, ReasonEstablisherNotInForce, "successor", cFeature}},
		{"acceptance unproven", chain, nil, fakeEst{"successor": {Reason: ReasonAcceptanceUnproven, Detail: "shallow history"}}, "successor-v2", "dc-1", want{Unresolved, ReasonAcceptanceUnproven, "successor", cFeature}},
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
