package specimport

import "errors"

// Sentinel errors carrying the contract's operational error codes
// (spec-import-contract.md, "Errors and browser behavior"). Every error
// this package's exported functions return wraps exactly one of these with
// %w, so a later task (CLI exit-code mapping, HTTP status mapping) can
// classify a failure with errors.Is without parsing message text. This
// shape is a Task 1 public-interface residual: the contract names the
// operational codes but not a Go error type, and this is the smallest
// reversible reading (see the lane report's semantic-decisions section).
//
// invalid-model, identity-unavailable, authority-invalid and every
// completed-refusal code (unresolved, invalid-candidate, dirty-context,
// stale-preview, target-exists, policy-forbidden, actor-forbidden,
// provenance-mismatch) belong to later tasks' services and are never
// produced by this package.
var (
	// ErrInvalidRequest is the contract's "invalid-request" code: the
	// request JSON or struct violates the closed shape, an enum, a
	// required field, a count/size limit, or a structural mapping rule
	// (duplicate target, evidence-only mapping to a non-existent target,
	// and similar request-shape defects Normalize can only see after
	// looking at content).
	ErrInvalidRequest = errors.New("specimport: invalid request")

	// ErrInvalidSource is the contract's "invalid-source" code: a selected
	// source's bytes, path or line selection cannot be honored — non-UTF-8
	// content, an out-of-range or malformed line range, an unclosed
	// leading frontmatter delimiter, or an unsafe file path/component.
	ErrInvalidSource = errors.New("specimport: invalid source")

	// ErrUnsupportedFormat is the contract's "unsupported-format" code:
	// Format is not one of the four closed values, or the named versioned
	// f13-reference-v1 profile refuses to bind because its pinned
	// precondition (the primary's selected SHA-256) is not met.
	ErrUnsupportedFormat = errors.New("specimport: unsupported format")

	// ErrIOFailure is the contract's "io-failure" code: a filesystem error
	// or context cancellation while ReadSource reads a source.
	ErrIOFailure = errors.New("specimport: io failure")
)

// Finding codes Normalize produces, from the contract's closed vocabulary
// (spec-import-contract.md, "Errors and browser behavior"). The remaining
// closed codes (invalid-candidate, existing-corpus-finding, statements-
// deferred, current-spec-changed, import-record-missing) are produced by
// later tasks' candidate/preview/apply logic, never by this package.
const (
	FindingMissingStatement     = "missing-statement"
	FindingEmptyField           = "empty-field"
	FindingAmbiguousField       = "ambiguous-field"
	FindingMultipleTargets      = "multiple-targets"
	FindingUnsupportedStructure = "unsupported-structure"
	FindingMissingEvidence      = "missing-evidence"
	FindingUnresolvedCoverage   = "unresolved-coverage"
	FindingSourceIDRequiresMap  = "source-id-requires-mapping"
)
