package specimport

import "errors"

// Task 3 sentinel errors, extending errors.go's Task 1 set with the
// remaining operational and completed-refusal codes spec-import-
// contract.md's "Errors and browser behavior" names for Preview/Apply/
// ReadRecord. Every error these three return wraps exactly one sentinel
// with %w, matching errors.go's own convention.
var (
	// ErrInvalidModel is "invalid-model": the store's operating model or
	// resolved template cannot express the candidate.
	ErrInvalidModel = errors.New("specimport: invalid model")
	// ErrIdentityUnavailable is "identity-unavailable": HEAD, the current
	// branch, or another required Git identity fact could not be resolved.
	ErrIdentityUnavailable = errors.New("specimport: identity unavailable")
	// ErrAuthorityInvalid is "authority-invalid": an operational authority
	// resolution failure (not genuine non-adoption).
	ErrAuthorityInvalid = errors.New("specimport: authority invalid")
	// ErrUnresolved is "unresolved": the preview is not ready (a blocking
	// finding remains) and Apply refuses to publish.
	ErrUnresolved = errors.New("specimport: unresolved")
	// ErrDirtyContext is "dirty-context": the tracked checkout/index carries
	// uncommitted changes, or an untracked corpus/config input is present.
	ErrDirtyContext = errors.New("specimport: dirty context")
	// ErrStalePreview is "stale-preview": Apply's recomputed preview digest
	// does not match the caller's expected digest.
	ErrStalePreview = errors.New("specimport: stale preview")
	// ErrTargetExists is "target-exists": an active/archive spec identity or
	// the target design/<slug> branch already exists and does not reconcile
	// with this exact request.
	ErrTargetExists = errors.New("specimport: target exists")
	// ErrPolicyForbidden is "policy-forbidden": the resolved design_assistance
	// policy forbids this actor's write.
	ErrPolicyForbidden = errors.New("specimport: policy forbidden")
	// ErrActorForbidden is "actor-forbidden": actor is not adapter-controlled
	// or is not the actor this operation requires.
	ErrActorForbidden = errors.New("specimport: actor forbidden")
	// ErrProvenanceMismatch is "provenance-mismatch": a retry's committed
	// bytes were produced under a different engine identity, or otherwise do
	// not verify against this call's bindings.
	ErrProvenanceMismatch = errors.New("specimport: provenance mismatch")
	// ErrImportRecordMissing is "import-record-missing": ReadRecord found no
	// (or an unreadable/corrupt) import record for the requested branch/slug.
	ErrImportRecordMissing = errors.New("specimport: import record missing")
)

// Finding codes produced by Task 3's ReadRecord logic, completing
// errors.go's/compose.go's Task 1/2 closed vocabulary (spec-import-
// contract.md, "Errors and browser behavior").
const (
	FindingCurrentSpecChanged  = "current-spec-changed"
	FindingImportRecordMissing = "import-record-missing"
)
