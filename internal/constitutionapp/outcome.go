package constitutionapp

import "fmt"

// Classification is the shared application-level 0/1/2 outcome vocabulary
// for every constitutionapp operation (root CLAUDE.md: "every verb exits 0
// (clean) / 1 (verdict) / 2 (operational)"). Modeled on
// internal/designapp.Classification/Error so every application-core package
// in this repo speaks the same shape.
type Classification string

const (
	ClassificationClean       Classification = "clean"
	ClassificationVerdict     Classification = "verdict"
	ClassificationOperational Classification = "operational"
)

// Error is the shared typed failure for every constitutionapp operation.
// Code is a short machine-stable diagnostic (this package's own closed
// vocabulary) and Detail is the human-readable cause; Cause, when set, is
// the wrapped underlying error for %w-based inspection.
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
// clean exit (mirrors internal/designapp.Error.ExitCode).
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

// Failure is the ONE typed, versioned projection of an application failure
// for transports that carry no exit-code channel — the MCP tool surface.
// Cause is deliberately absent: it is an in-process %w handle for Go
// callers, not wire content.
type Failure struct {
	Schema         string         `json:"schema"`
	Classification Classification `json:"classification"`
	Code           string         `json:"code"`
	Detail         string         `json:"detail"`
}

// Failure projects this error into the wire envelope. A nil receiver still
// fails closed as operational rather than materializing a
// classification-free value.
func (e *Error) Failure() Failure {
	if e == nil {
		return NewFailure(ClassificationOperational, "result-invalid", "an unspecified application failure was reported without a diagnostic")
	}
	classification := e.Classification
	switch classification {
	case ClassificationVerdict, ClassificationOperational:
	default:
		classification = ClassificationOperational
	}
	return NewFailure(classification, e.Code, e.Detail)
}

// NewFailure builds the typed envelope directly, for an adapter-detected
// application failure that never had an *Error to carry it.
func NewFailure(classification Classification, code, detail string) Failure {
	return Failure{Schema: FailureSchema, Classification: classification, Code: code, Detail: detail}
}

func inputInvalid(code, detail string) *Error {
	return &Error{Classification: ClassificationVerdict, Code: code, Detail: detail}
}

func verdict(code, detail string) *Error {
	return &Error{Classification: ClassificationVerdict, Code: code, Detail: detail}
}

func verdictWithCause(code, detail string, cause error) *Error {
	return &Error{Classification: ClassificationVerdict, Code: code, Detail: detail, Cause: cause}
}

func operational(code, detail string, cause error) *Error {
	return &Error{Classification: ClassificationOperational, Code: code, Detail: detail, Cause: cause}
}
