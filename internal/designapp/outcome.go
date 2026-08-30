package designapp

import (
	"fmt"

	"github.com/jyang234/verdi/internal/draftmutation"
)

// Classification is the shared application-level 0/1/2 outcome vocabulary
// for designapp's five non-mutation operations (root CLAUDE.md: "every
// verb exits 0 (clean) / 1 (verdict) / 2 (operational)"). MutateDraft does
// not use this type: it returns draftmutation's own closed
// Response/*draftmutation.Error union unchanged, since AC-1's mutation
// contract already owns that classification and this package must not
// reinterpret it a second time (doc.go). Modeled on
// internal/experimentapp's Outcome/Classification precedent so every
// application-core package in this repo speaks the same shape.
type Classification string

const (
	ClassificationClean       Classification = "clean"
	ClassificationVerdict     Classification = "verdict"
	ClassificationOperational Classification = "operational"
)

// Error is the shared typed failure for get_board, get_design_context,
// get_design_capabilities, get_design_provenance, and
// prepare_design_review. Code is a short machine-stable diagnostic (this
// package's own closed vocabulary, distinct from draftmutation.Code —
// these are read/derive operations, not mutation refusals) and Detail is
// the human-readable cause; Cause, when set, is the wrapped underlying
// error for %w-based inspection.
type Error struct {
	Classification Classification
	Code           string
	Detail         string
	Cause          error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Detail, e.Cause)
	}
	if e.Detail == "" {
		return e.Code
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Detail)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// ExitCode returns the fixed CLI projection. An unknown internal
// Classification value fails closed as operational rather than becoming a
// clean exit (mirrors internal/experimentapp.Outcome.ExitCode).
func (e *Error) ExitCode() int {
	if e == nil {
		return 0
	}
	switch e.Classification {
	case ClassificationVerdict:
		return 1
	default:
		return 2
	}
}

// Failure is the ONE typed, versioned projection of an application
// failure for transports that carry no exit-code channel — the MCP tool
// surface (internal/mcpserve/designappbridge.go). The CLI distinguishes a
// refusal from a breakage by exiting 1 or 2; an MCP caller reads
// Classification instead, so the same distinction survives both adapters
// (CO-9's adapter conformance names "error classifications" as a
// conformance object in its own right). Code and Detail are the same two
// strings the CLI prints, so the two transports' diagnostics stay
// comparable field-for-field.
//
// Cause is deliberately absent: it is an in-process %w handle for Go
// callers, not wire content, and its text can carry local filesystem
// paths that would make two conformant adapters' output differ for
// reasons that are not about the failure at all.
type Failure struct {
	Schema         string         `json:"schema"`
	Classification Classification `json:"classification"`
	Code           string         `json:"code"`
	Detail         string         `json:"detail"`
}

// Failure projects this error into the wire envelope. A nil receiver is
// still a Failure — an adapter that reached this call has already decided
// something failed — and fails closed as operational rather than
// materializing a classification-free (and therefore silently favorable)
// value.
func (e *Error) Failure() Failure {
	if e == nil {
		return NewFailure(ClassificationOperational, "result-invalid", "an unspecified application failure was reported without a diagnostic")
	}
	classification := e.Classification
	switch classification {
	case ClassificationVerdict, ClassificationOperational:
	default:
		// Fails closed exactly as ExitCode does: an unknown internal value
		// never becomes the favorable answer — and neither does
		// ClassificationClean, which no constructor here produces and which
		// on a value that already failed would be a contradiction, not a
		// pass.
		classification = ClassificationOperational
	}
	return NewFailure(classification, e.Code, e.Detail)
}

// NewFailure builds the typed envelope directly, for an adapter-detected
// application failure that never had an *Error to carry it (mcpserve's
// invalid-response-union guard).
func NewFailure(classification Classification, code, detail string) Failure {
	return Failure{Schema: FailureSchema, Classification: classification, Code: code, Detail: detail}
}

// MutationFailure projects mutate_draft's own diagnostic union into the
// same envelope, taking its classification from draftmutation.Verdict()
// rather than re-deriving one (AC-1: the mutation contract owns that
// judgment and this package must not interpret it a second time).
func MutationFailure(err *draftmutation.Error) Failure {
	return translateDraftmutationError(err).Failure()
}

func inputInvalid(code, detail string) *Error {
	return &Error{Classification: ClassificationVerdict, Code: code, Detail: detail}
}

func operational(code, detail string, cause error) *Error {
	return &Error{Classification: ClassificationOperational, Code: code, Detail: detail, Cause: cause}
}

func notFound(code, detail string) *Error {
	return &Error{Classification: ClassificationVerdict, Code: code, Detail: detail}
}

// translateDraftmutationError projects a *draftmutation.Error (returned by
// a composed read, such as ResolvePolicyGrant/AuthorizeState) into
// designapp's own Error, preserving its verdict/operational
// classification exactly (draftmutation.Error.Verdict()) rather than
// re-deriving it. The wrapped error keeps draftmutation's own code string
// as this package's Code so a caller can still recognize the exact
// upstream refusal.
func translateDraftmutationError(err *draftmutation.Error) *Error {
	if err == nil {
		return nil
	}
	classification := ClassificationOperational
	if err.Verdict() {
		classification = ClassificationVerdict
	}
	return &Error{Classification: classification, Code: string(err.Code), Detail: err.Error(), Cause: err}
}
