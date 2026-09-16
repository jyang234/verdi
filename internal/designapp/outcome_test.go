package designapp

import (
	"testing"

	"github.com/jyang234/verdi/internal/draftmutation"
)

// TestErrorFailure covers the typed wire projection's happy path and every
// negative path that could turn a failure into a silently favorable
// answer: a nil diagnostic, an internally impossible "clean" failure, and
// an unknown classification value. All three fail closed as operational,
// exactly as ExitCode does.
func TestErrorFailure(t *testing.T) {
	for _, tc := range []struct {
		name               string
		err                *Error
		wantClassification Classification
		wantCode           string
	}{
		{
			name:               "verdict",
			err:                &Error{Classification: ClassificationVerdict, Code: "spec-not-found", Detail: "no such active spec"},
			wantClassification: ClassificationVerdict,
			wantCode:           "spec-not-found",
		},
		{
			name:               "operational",
			err:                &Error{Classification: ClassificationOperational, Code: "io-failure", Detail: "reading spec"},
			wantClassification: ClassificationOperational,
			wantCode:           "io-failure",
		},
		{
			name:               "nil diagnostic fails closed",
			err:                nil,
			wantClassification: ClassificationOperational,
			wantCode:           "result-invalid",
		},
		{
			name:               "clean is not a failure classification",
			err:                &Error{Classification: ClassificationClean, Code: "io-failure", Detail: "reading spec"},
			wantClassification: ClassificationOperational,
			wantCode:           "io-failure",
		},
		{
			name:               "unknown classification fails closed",
			err:                &Error{Classification: Classification("wat"), Code: "io-failure", Detail: "reading spec"},
			wantClassification: ClassificationOperational,
			wantCode:           "io-failure",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.err.Failure()
			if got.Schema != FailureSchema {
				t.Fatalf("Schema = %q, want %q", got.Schema, FailureSchema)
			}
			if got.Classification != tc.wantClassification {
				t.Fatalf("Classification = %q, want %q", got.Classification, tc.wantClassification)
			}
			if got.Code != tc.wantCode {
				t.Fatalf("Code = %q, want %q", got.Code, tc.wantCode)
			}
			if got.Detail == "" {
				t.Fatal("Detail must never be empty: a failure with no account of itself is a silent one")
			}
		})
	}
}

// TestMutationFailure proves mutate_draft's diagnostic union keeps
// draftmutation's OWN verdict/operational judgment (AC-1: this package
// never re-derives it), including the nil case.
func TestMutationFailure(t *testing.T) {
	for _, tc := range []struct {
		name               string
		err                *draftmutation.Error
		wantClassification Classification
		wantCode           string
		wantDetail         string
	}{
		{
			name:               "verdict code",
			err:                draftmutation.NewError(draftmutation.CodePolicyForbidden, draftmutation.Identity{}, "policy forbids"),
			wantClassification: ClassificationVerdict,
			wantCode:           string(draftmutation.CodePolicyForbidden),
			wantDetail:         "policy forbids",
		},
		{
			name:               "operational code",
			err:                draftmutation.NewError(draftmutation.CodeIOFailure, draftmutation.Identity{}, "disk gone"),
			wantClassification: ClassificationOperational,
			wantCode:           string(draftmutation.CodeIOFailure),
			wantDetail:         "disk gone",
		},
		{
			name:               "nil diagnostic fails closed",
			err:                nil,
			wantClassification: ClassificationOperational,
			wantCode:           "result-invalid",
			wantDetail:         "an unspecified application failure was reported without a diagnostic",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := MutationFailure(tc.err)
			if got.Schema != FailureSchema {
				t.Fatalf("Schema = %q, want %q", got.Schema, FailureSchema)
			}
			if got.Classification != tc.wantClassification {
				t.Fatalf("Classification = %q, want %q", got.Classification, tc.wantClassification)
			}
			if got.Code != tc.wantCode {
				t.Fatalf("Code = %q, want %q", got.Code, tc.wantCode)
			}
			// ac-5 (spec/uat-round-1): Detail is the BARE draftmutation
			// detail, never draftmutation's own Error() form ("<code>:
			// <detail>") — Code already carries the code, so re-embedding
			// it in Detail doubles the prefix wherever a caller (the CLI,
			// the workbench board) renders "<code>: <detail>" together.
			if got.Detail != tc.wantDetail {
				t.Fatalf("Detail = %q, want %q (must not re-embed Code)", got.Detail, tc.wantDetail)
			}
		})
	}
}

// TestNewFailure covers the adapter-constructed envelope directly.
func TestNewFailure(t *testing.T) {
	got := NewFailure(ClassificationOperational, "result-invalid", "an invalid response union")
	if got != (Failure{
		Schema: FailureSchema, Classification: ClassificationOperational,
		Code: "result-invalid", Detail: "an invalid response union",
	}) {
		t.Fatalf("NewFailure = %+v", got)
	}
}
