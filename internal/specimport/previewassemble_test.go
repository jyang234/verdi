package specimport

import "testing"

// TestAssemblePreview_NonDeferred_NoDuplicateFindings proves the
// non-deferred merge never double-reports a finding Compose's own internal
// prepareCandidateFields call already produced: composeFindings simulates
// "the same fieldFindings Compose recomputed internally, plus one genuinely
// new lint finding" — the merge must keep exactly one copy of the shared
// finding and the one new one.
func TestAssemblePreview_NonDeferred_NoDuplicateFindings(t *testing.T) {
	req := minimalRequest() // RetainUnmapped: true, so no coverage findings.
	req.Mappings = []Mapping{
		{Target: "ac-1", Evidence: []string{"static"}},
		{Target: "ac-2", Evidence: []string{"static"}},
	}
	plan, err := Normalize(req)
	if err != nil {
		t.Fatal(err)
	}
	requireNoBlocking(t, plan.Findings, nil)

	extra := Finding{Code: FindingInvalidCandidate, Target: "spec.md", Message: "boom", Blocking: true}
	// Compose's real return, for a non-deferred request, is exactly
	// prepareCandidateFields' own findings plus its own additional ones.
	_, fieldFindings := prepareCandidateFields(req, plan)
	composeFindings := append(append([]Finding{}, fieldFindings...), extra)

	fields, coverage, findings := assemblePreview(req, plan, composeFindings)

	if len(fields) != len(plan.Fields) {
		t.Fatalf("fields = %+v, want plan.Fields unchanged (non-deferred pass-through)", fields)
	}
	if len(coverage) != len(plan.Coverage) {
		t.Fatalf("coverage length = %d, want %d", len(coverage), len(plan.Coverage))
	}

	count := 0
	foundExtra := false
	for _, f := range findings {
		if f == extra {
			foundExtra = true
		}
		if f.Code == extra.Code && f.Target == extra.Target {
			count++
		}
	}
	if !foundExtra {
		t.Fatalf("findings %+v missing Compose's own extra finding", findings)
	}
	if count != 1 {
		t.Fatalf("findings %+v contains %d copies of Compose's extra finding, want exactly 1 (no duplication)", findings, count)
	}
	if len(findings) != len(fieldFindings)+1 {
		t.Fatalf("findings = %+v (%d), want fieldFindings (%d) plus exactly one Compose-only finding", findings, len(findings), len(fieldFindings))
	}
}

// TestAssemblePreview_Deferral_RecomputesCoverageNotStale proves deferral
// recomputes coverage from the candidate-ready (post-deferral) fields'
// spans rather than reusing Normalize's own pre-deferral plan.Coverage
// (Task 2 handoff: "Never reuse stale Plan.Coverage after deferral.
// Displaced spans become retained or unresolved under RetainUnmapped, with
// byte totals reasserted"). Deferring problem/outcome with retain_unmapped
// false orphans their previously-mapped bytes, which must surface as a
// FRESH blocking unresolved-coverage finding — not a second, stale copy
// alongside it.
func TestAssemblePreview_Deferral_RecomputesCoverageNotStale(t *testing.T) {
	req := minimalRequest()
	req.RetainUnmapped = false
	req.DeferStatements = true
	req.Mappings = []Mapping{
		{Target: "ac-1", Evidence: []string{"static"}},
		{Target: "ac-2", Evidence: []string{"static"}},
	}
	plan, err := Normalize(req)
	if err != nil {
		t.Fatal(err)
	}

	var planUnresolved int
	for _, c := range plan.Coverage {
		if c.SourceID == "source" {
			planUnresolved = c.UnresolvedBytes
		}
	}

	fields, coverage, findings := assemblePreview(req, plan, nil)
	if len(fields) == 0 {
		t.Fatal("assemblePreview returned no candidate-ready fields")
	}

	var recomputedUnresolved int
	found := false
	for _, c := range coverage {
		if c.SourceID == "source" {
			recomputedUnresolved = c.UnresolvedBytes
			found = true
		}
	}
	if !found {
		t.Fatalf("coverage %+v has no entry for source %q", coverage, "source")
	}
	if recomputedUnresolved <= planUnresolved {
		t.Fatalf("recomputed UnresolvedBytes = %d, want strictly greater than the stale plan value %d (deferred problem/outcome bytes must become unresolved)", recomputedUnresolved, planUnresolved)
	}

	count := 0
	for _, f := range findings {
		if f.Code == FindingUnresolvedCoverage && f.Target == "source" {
			count++
			if !f.Blocking {
				t.Fatalf("unresolved-coverage finding not blocking: %+v", f)
			}
		}
	}
	if count != 1 {
		t.Fatalf("findings %+v contains %d unresolved-coverage findings for source, want exactly 1 (fresh, not stale+fresh)", findings, count)
	}
}
