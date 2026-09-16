package designapp

import (
	"errors"
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

// TestError_ErrorStringSinglePrefixed pins Error()'s rendering against a
// second, latent doubling an Opus review found on top of ac-5's Detail
// fix (spec/uat-round-1): the "%s: %s: %v" Cause branch stringifies Cause
// with %v, and for a translateDraftmutationError result Cause is the
// exact *draftmutation.Error whose own Error() is already "Code: Detail"
// — so the WHOLE message repeated, e.g. "policy-forbidden: project has
// not adopted policy authority: policy-forbidden: project has not
// adopted policy authority". Nothing stringifies *designapp.Error in
// production today (this was latent), but Error() must still render
// single-prefixed. A Cause that is NOT that exact redundant shape keeps
// its original three-part "code: detail: cause" rendering unchanged.
func TestError_ErrorStringSinglePrefixed(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "translated policy-forbidden",
			err:  translateDraftmutationError(draftmutation.NewError(draftmutation.CodePolicyForbidden, draftmutation.Identity{}, "project has not adopted policy authority")),
			want: "policy-forbidden: project has not adopted policy authority",
		},
		{
			name: "translated io-failure",
			err:  translateDraftmutationError(draftmutation.NewError(draftmutation.CodeIOFailure, draftmutation.Identity{}, "disk gone")),
			want: "io-failure: disk gone",
		},
		{
			name: "generic cause keeps its three-part rendering",
			err:  operational("io-failure", "reading spec", errors.New("disk gone")),
			want: "io-failure: reading spec: disk gone",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Fatalf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestError_UnwrapKeepsCauseForDraftmutationTranslation proves the
// single-prefix fix above only changes Error()'s rendering: Cause stays
// set, so errors.Is/As chaining through a translated draftmutation error
// is untouched.
func TestError_UnwrapKeepsCauseForDraftmutationTranslation(t *testing.T) {
	inner := draftmutation.NewError(draftmutation.CodePolicyForbidden, draftmutation.Identity{}, "project has not adopted policy authority")
	if !errors.Is(translateDraftmutationError(inner), inner) {
		t.Fatal("errors.Is must still find the wrapped draftmutation.Error through Cause")
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
