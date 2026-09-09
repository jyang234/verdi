package mcpserve

import (
	"encoding/json"
	"testing"

	"github.com/jyang234/verdi/internal/designapp"
	"github.com/jyang234/verdi/internal/draftmutation"
)

// TestToolErrorForDesignAppPreservesClassification is the classification
// probe: two application failures that agree on code and detail but
// disagree on 0/1/2 classification must NOT serialize identically. The CLI
// half of the same pair is distinguishable by exit code (1 vs 2); an MCP
// caller has no exit-code channel, so the classification has to ride in a
// machine-readable field of the tool result itself, or the adapter has
// erased a distinction the application core made.
func TestToolErrorForDesignAppPreservesClassification(t *testing.T) {
	verdict := &designapp.Error{Classification: designapp.ClassificationVerdict, Code: "same-code", Detail: "same detail"}
	operational := &designapp.Error{Classification: designapp.ClassificationOperational, Code: "same-code", Detail: "same detail"}

	verdictResult := toolErrorForDesignApp(verdict)
	operationalResult := toolErrorForDesignApp(operational)

	if !isToolError(verdictResult) || !isToolError(operationalResult) {
		t.Fatal("both application failures must render as tool errors (isError), never protocol errors")
	}
	verdictText := toolResultText(t, verdictResult)
	operationalText := toolResultText(t, operationalResult)
	if verdictText == operationalText {
		t.Fatalf("verdict and operational failures serialize identically: %s", verdictText)
	}

	decode := func(text string) designapp.Failure {
		t.Helper()
		var failure designapp.Failure
		if err := json.Unmarshal([]byte(text), &failure); err != nil {
			t.Fatalf("decoding typed failure %q: %v", text, err)
		}
		return failure
	}
	got := decode(verdictText)
	if got.Schema != designapp.FailureSchema {
		t.Fatalf("schema = %q, want %q", got.Schema, designapp.FailureSchema)
	}
	if got.Classification != designapp.ClassificationVerdict || got.Code != "same-code" || got.Detail != "same detail" {
		t.Fatalf("verdict failure = %+v", got)
	}
	if got := decode(operationalText); got.Classification != designapp.ClassificationOperational {
		t.Fatalf("operational failure = %+v", got)
	}
}

// TestMutationFailureClassification is the same probe for mutate_draft's
// own diagnostic union, whose classification comes from draftmutation's
// Verdict() rather than designapp's Classification.
func TestMutationFailureClassification(t *testing.T) {
	for _, tc := range []struct {
		name string
		code draftmutation.Code
		want designapp.Classification
	}{
		{name: "verdict", code: draftmutation.CodeStateForbidden, want: designapp.ClassificationVerdict},
		{name: "operational", code: draftmutation.CodeIOFailure, want: designapp.ClassificationOperational},
	} {
		t.Run(tc.name, func(t *testing.T) {
			failure := designapp.MutationFailure(draftmutation.NewError(tc.code, draftmutation.Identity{}, "same detail"))
			if failure.Schema != designapp.FailureSchema {
				t.Fatalf("schema = %q, want %q", failure.Schema, designapp.FailureSchema)
			}
			if failure.Classification != tc.want {
				t.Fatalf("classification = %q, want %q", failure.Classification, tc.want)
			}
			if failure.Code != string(tc.code) {
				t.Fatalf("code = %q, want %q", failure.Code, tc.code)
			}
		})
	}
}
