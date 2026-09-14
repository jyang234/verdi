package specimport

import (
	"context"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designscaffold"
)

// TestCompose_DeferStatements_DisplacesResolvedStatement proves explicit
// pair deferral (request.DeferStatements) is an all-or-nothing choice: even
// though minimalRequest()'s markdown-v1 source automatically resolves real
// problem/outcome text, the composed candidate carries the shared
// designscaffold.DefaultProblem/DefaultOutcome placeholders — mirrored in
// both frontmatter and body, exactly like any other mapped field — and the
// displaced real text is never silently lost: it is named, verbatim, in a
// nonblocking statements-deferred disclosure.
func TestCompose_DeferStatements_DisplacesResolvedStatement(t *testing.T) {
	root := minimalStoreRoot(t)
	req := minimalRequest()
	req.DeferStatements = true
	req.Mappings = []Mapping{
		{Target: "ac-1", Evidence: []string{"static", "attestation"}},
		{Target: "ac-2", Evidence: []string{"static", "attestation"}},
	}

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if _, ok := fieldByTarget(plan.Fields, "problem"); !ok {
		t.Fatalf("test fixture assumption broken: want Normalize to resolve \"problem\" automatically before deferral is applied")
	}

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	requireNoBlocking(t, findings, candidate) // a deferral disclosure is nonblocking
	if candidate == nil {
		t.Fatal("Compose returned nil bytes for a candidate that only deferred its statements")
	}

	fm, body, err := artifact.SplitFrontmatter(candidate)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v\n%s", err, candidate)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v\n%s", err, candidate)
	}
	if err := spec.ResolveObjectAnchors(body); err != nil {
		t.Fatalf("ResolveObjectAnchors: %v\n%s", err, candidate)
	}
	if spec.Problem == nil || spec.Problem.Text != designscaffold.DefaultProblem {
		t.Fatalf("problem was not deferred to the shared placeholder: %+v", spec.Problem)
	}
	if spec.Outcome == nil || spec.Outcome.Text != designscaffold.DefaultOutcome {
		t.Fatalf("outcome was not deferred to the shared placeholder: %+v", spec.Outcome)
	}
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "## Problem\n\n"+designscaffold.DefaultProblem) {
		t.Fatalf("deferred problem placeholder not mirrored into its own body section:\n%s", bodyStr)
	}
	if !strings.Contains(bodyStr, "## Outcome\n\n"+designscaffold.DefaultOutcome) {
		t.Fatalf("deferred outcome placeholder not mirrored into its own body section:\n%s", bodyStr)
	}
	if strings.Contains(bodyStr, "First line.\nSecond line.") {
		t.Fatalf("the displaced real problem text was written into the candidate body instead of being deferred:\n%s", bodyStr)
	}

	var sawProblemDeferred, sawOutcomeDeferred bool
	for _, f := range findings {
		if f.Code != FindingStatementsDeferred {
			continue
		}
		requireTruthfulDeferralMessage(t, f)
		switch f.Target {
		case "problem":
			sawProblemDeferred = true
			if !strings.Contains(f.Message, "First line.") || !strings.Contains(f.Message, OriginCopiedSource) {
				t.Fatalf("problem deferral disclosure does not name the displaced value and its origin: %+v", f)
			}
		case "outcome":
			sawOutcomeDeferred = true
			if !strings.Contains(f.Message, "Users get value.") || !strings.Contains(f.Message, OriginCopiedSource) {
				t.Fatalf("outcome deferral disclosure does not name the displaced value and its origin: %+v", f)
			}
		}
	}
	if !sawProblemDeferred || !sawOutcomeDeferred {
		t.Fatalf("want one statements-deferred disclosure per statement, got: %+v", findings)
	}
}

// requireTruthfulDeferralMessage fails if a statements-deferred disclosure
// asserts a disposition Compose cannot establish. Compose holds no coverage
// witness for a displaced value — only Task 3's Preview recomputes coverage
// from the candidate-ready fields — so claiming the value "was retained", or
// calling it "source text" when its origin may be user-added, states as fact
// what nothing in the call establishes (spec-import-contract.md:
// "Placeholders are visibly incomplete and not source quotations").
func requireTruthfulDeferralMessage(t *testing.T, f Finding) {
	t.Helper()
	for _, forbidden := range []string{"retained", "source text", "recognized source"} {
		if strings.Contains(f.Message, forbidden) {
			t.Fatalf("deferral disclosure claims %q, which Compose cannot establish: %+v", forbidden, f)
		}
	}
	if f.Blocking {
		t.Fatalf("a statements-deferred disclosure must be nonblocking: %+v", f)
	}
}

// TestCompose_DeferStatements_UserAddedValueIsNotASourceQuotation is the
// decisive B2 case: an explicit USER-ADDED problem Mapping (Text only, no
// SourceID) carries origin user-added and no spans at all, so no source
// recognized it and no interval can be retained on its behalf. The deferral
// disclosure must name the displacement and the origin truthfully rather
// than presenting user-typed text as a quoted, retained source selection.
func TestCompose_DeferStatements_UserAddedValueIsNotASourceQuotation(t *testing.T) {
	root := minimalStoreRoot(t)
	userText := "Borrowers cannot resubmit a rejected document."
	req := minimalRequest()
	req.DeferStatements = true
	req.Mappings = []Mapping{
		{Target: "problem", Text: &userText},
		{Target: "ac-1", Evidence: []string{"static", "attestation"}},
		{Target: "ac-2", Evidence: []string{"static", "attestation"}},
	}

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	problem, ok := fieldByTarget(plan.Fields, "problem")
	if !ok || problem.Origin != OriginUserAdded || len(problem.Spans) != 0 {
		t.Fatalf("test fixture assumption broken: want a user-added problem field with no spans, got %+v (ok=%v)", problem, ok)
	}

	_, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	var sawProblem bool
	for _, f := range findings {
		if f.Code != FindingStatementsDeferred || f.Target != "problem" {
			continue
		}
		sawProblem = true
		requireTruthfulDeferralMessage(t, f)
		if !strings.Contains(f.Message, OriginUserAdded) {
			t.Fatalf("deferral disclosure does not name the displaced value's user-added origin: %+v", f)
		}
	}
	if !sawProblem {
		t.Fatalf("want a statements-deferred disclosure for the displaced problem, got: %+v", findings)
	}
}

// TestPrepareCandidateFields_StatementsBeforeObjects pins the contract's
// declared map order ("statements, then objects in source/explicit insertion
// order") on the SHARED helper Task 3's Preview consumes — the deferral
// branch is the only one that reorders, and PreviewResult.fields lands
// inside the SHA-256-over-canonical-JSON digest domain, so the order must be
// fixed before a digest freezes it.
func TestPrepareCandidateFields_StatementsBeforeObjects(t *testing.T) {
	req := minimalRequest()
	req.DeferStatements = true
	req.Mappings = []Mapping{
		{Target: "ac-1", Evidence: []string{"static", "attestation"}},
		{Target: "ac-2", Evidence: []string{"static", "attestation"}},
	}
	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	fields, _ := prepareCandidateFields(req, plan)
	var got []string
	for _, f := range fields {
		got = append(got, f.Target)
	}
	want := []string{"problem", "outcome", "ac-1", "ac-2"}
	if len(got) != len(want) {
		t.Fatalf("candidate-ready field order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candidate-ready field order = %v, want %v", got, want)
		}
	}
}

// TestCompose_DeferStatements_NothingResolved proves deferral works
// identically when automatic recognition found nothing at all (manual-v1
// performs no recognition): the placeholder is inserted and the
// disclosure names the target without claiming any displaced text.
func TestCompose_DeferStatements_NothingResolved(t *testing.T) {
	root := minimalStoreRoot(t)
	req := Request{
		Schema:  RequestSchema,
		Target:  Target{Slug: "manual-widget", Class: "feature", Title: "Manual Widget"},
		Format:  FormatManualV1,
		Primary: "source",
		Sources: []Source{{ID: "source", Label: "manual.md", Data: []byte("irrelevant manual content\n")}},
		Mappings: []Mapping{
			{Target: "ac-1", SourceID: "source", Start: 0, End: 9, Transform: TransformIdentity, Evidence: []string{"static", "attestation"}},
		},
		DeferStatements: true,
		RetainUnmapped:  true, // irrelevant to deferral: avoid an unrelated unresolved-coverage finding for the rest of the source
	}

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	requireNoBlocking(t, findings, candidate)
	if candidate == nil {
		t.Fatal("Compose returned nil bytes for a fully deferred manual-v1 candidate")
	}
	if !strings.Contains(string(candidate), designscaffold.DefaultProblem) {
		t.Fatalf("deferred problem placeholder missing:\n%s", candidate)
	}
}
