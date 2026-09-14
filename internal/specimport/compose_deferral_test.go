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
		switch f.Target {
		case "problem":
			sawProblemDeferred = true
			if !strings.Contains(f.Message, "First line.") {
				t.Fatalf("problem deferral disclosure does not retain the displaced source text: %+v", f)
			}
		case "outcome":
			sawOutcomeDeferred = true
			if !strings.Contains(f.Message, "Users get value.") {
				t.Fatalf("outcome deferral disclosure does not retain the displaced source text: %+v", f)
			}
		}
	}
	if !sawProblemDeferred || !sawOutcomeDeferred {
		t.Fatalf("want one statements-deferred disclosure per statement, got: %+v", findings)
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
