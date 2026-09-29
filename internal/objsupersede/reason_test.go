package objsupersede

import (
	"strings"
	"testing"
)

// TestResultText pins one canonical rendering per outcome and reason: design
// §5's two resolved texts verbatim, and SI-274's quoted reason wordings.
func TestResultText(t *testing.T) {
	base := Result{Spec: "s2", Decision: "dc-1", Edge: "spec/t#dc-1", Conflict: "c", Other: "x"}
	with := func(o Outcome, r Reason, edit func(*Result)) Result {
		res := base
		res.Outcome, res.Reason = o, r
		if edit != nil {
			edit(&res)
		}
		return res
	}
	tests := []struct {
		res  Result
		want string
	}{
		{with(ResolvedNew, "", nil), "records match; takes effect when spec/s2 is accepted"},
		{with(ResolvedCarried, "", func(r *Result) { r.Since = "2024-02-15" }), "carries the replacement established by spec/x (conflict/c, since 2024-02-15)"},
		{with(Unresolved, ReasonPinned, func(r *Result) { r.Edge = "spec/t@0a1b2c3#dc-1" }), "the edge spec/t@0a1b2c3#dc-1 is pinned; a closed spec's object is superseded only by an unpinned ref"},
		{with(Unresolved, ReasonRecordsUndecodable, func(r *Result) { r.Detail = "a.md: bad" }), "records do not decode: a.md: bad"},
		{with(Unresolved, ReasonTargetMissing, nil), "the target spec spec/t is missing"},
		{with(Unresolved, ReasonTargetNotClosed, nil), "the target spec spec/t is not closed"},
		{with(Unresolved, ReasonObjectNotDeclared, nil), "the object spec/t#dc-1 is not declared"},
		{with(Unresolved, ReasonObjectNotTarget, nil), "the object spec/t#dc-1 is not an acceptance criterion or a decision"},
		{with(Unresolved, ReasonAlreadySuperseded, nil), "the object spec/t#dc-1 is already superseded by spec/x (conflict/c)"},
		{with(Unresolved, ReasonNoConflict, nil), "no conflict challenges spec/t#dc-1"},
		{with(Unresolved, ReasonConflictNotSuperseded, nil), "the conflict conflict/c is not superseded"},
		{with(Unresolved, ReasonResolvedByOther, nil), "the conflict conflict/c's resolved_by names another spec, spec/x"},
		{with(Unresolved, ReasonResolvedByOther, func(r *Result) { r.Other = "" }), "the conflict conflict/c's resolved_by names no spec"},
		{with(Unresolved, ReasonMultipleConflicts, nil), "more than one superseded conflict names spec/x for spec/t"},
		{with(Unresolved, ReasonConflictSpansSpecs, nil), "the conflict challenges objects of more than one spec"},
		{with(Unresolved, ReasonCarryMismatch, func(r *Result) { r.Predecessor = "s1" }), "a carried decision's classification or edge does not match its predecessor (spec/x from spec/s1)"},
		{with(Unresolved, ReasonUnmatchedChallenge, func(r *Result) { r.Decision = "" }), "conflict/c challenges spec/t#dc-1, but spec/s2 carries no matching edge"},
		{with(Unresolved, ReasonEstablisherNotAccepted, nil), "spec/x is not accepted"},
		{with(Unresolved, ReasonEstablisherNotInForce, func(r *Result) { r.Detail = "no conflict challenges spec/t#dc-1" }), "spec/x's supersession was not in force at its acceptance: no conflict challenges spec/t#dc-1"},
		{with(Unresolved, ReasonAcceptanceUnproven, func(r *Result) { r.Detail = "shallow history" }), "acceptance unproven: shallow history"},
	}
	seen := map[Reason]bool{}
	for _, tc := range tests {
		got, err := tc.res.Text()
		if err != nil || got != tc.want {
			t.Errorf("%s/%s: got %q, %v; want %q", tc.res.Outcome, tc.res.Reason, got, err, tc.want)
		}
		seen[tc.res.Reason] = true
	}
	for _, r := range reasons {
		if !seen[r] {
			t.Errorf("reason %q has no pinned rendering", r)
		}
	}
}

func TestResultText_FailsClosed(t *testing.T) {
	tests := []Result{
		{Outcome: "maybe"},
		{Outcome: Unresolved, Reason: "mystery"},
		{Outcome: Unresolved},
		{Outcome: ResolvedNew, Reason: ReasonNoConflict},
		{Outcome: ResolvedCarried, Other: "x", Conflict: "c"},
		// review b M-4: an empty name, an empty witness, or a malformed date
		// is refused, never rendered.
		{Outcome: ResolvedNew},
		{Outcome: ResolvedCarried, Since: "2024-01-01"},
		{Outcome: ResolvedCarried, Other: "x", Since: "2024-01-01"},
		{Outcome: ResolvedCarried, Other: "x", Conflict: "c", Since: "not-a-date"},
		{Outcome: ResolvedCarried, Other: "x", Conflict: "c", Since: "2024-1-1"},
		{Outcome: Unresolved, Reason: ReasonAcceptanceUnproven},
		{Outcome: Unresolved, Reason: ReasonRecordsUndecodable},
		{Outcome: Unresolved, Reason: ReasonEstablisherNotInForce, Other: "s"},
		{Outcome: Unresolved, Reason: ReasonEstablisherNotInForce, Detail: "d"},
		{Outcome: Unresolved, Reason: ReasonEstablisherNotAccepted},
		{Outcome: Unresolved, Reason: ReasonCarryMismatch},
		{Outcome: Unresolved, Reason: ReasonCarryMismatch, Other: "s2"},
		{Outcome: Unresolved, Reason: ReasonAlreadySuperseded, Edge: "spec/t#dc-1", Other: "x"},
		{Outcome: Unresolved, Reason: ReasonMultipleConflicts, Edge: "spec/t#dc-1"},
		{Outcome: Unresolved, Reason: ReasonNoConflict},
		{Outcome: Unresolved, Reason: ReasonPinned},
		{Outcome: Unresolved, Reason: ReasonConflictNotSuperseded},
		{Outcome: Unresolved, Reason: ReasonResolvedByOther},
		{Outcome: Unresolved, Reason: ReasonUnmatchedChallenge, Edge: "spec/t#dc-1", Conflict: "c"},
	}
	for _, r := range tests {
		if got, err := r.Text(); err == nil {
			t.Errorf("%+v rendered %q", r, got)
		}
	}
}

func TestParseReason(t *testing.T) {
	for _, r := range reasons {
		if got, err := ParseReason(string(r)); err != nil || got != r {
			t.Errorf("ParseReason(%q) = %q, %v", r, got, err)
		}
	}
	for _, s := range []string{"", "mystery", strings.ToUpper(string(ReasonPinned))} {
		if _, err := ParseReason(s); err == nil {
			t.Errorf("ParseReason(%q) accepted an unknown reason", s)
		}
	}
}
