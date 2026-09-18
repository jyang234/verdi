// ValidateSuccessorName and its typed refusal now live in
// internal/specname (moved there by the UAT-030/031/032 fix round): the
// predicate widened to cover the plain `design start` path and the
// board's create action, neither of which supersedes anything, so
// validating a bare name's shape and uniqueness no longer belongs to THIS
// package's own concern (CLAUDE.md: "one package = one concern ... split
// before a package accumulates a second concern").
//
// This file is a thin re-export so the two callers that ARE
// supersede-flavored — `design start --supersedes`
// (cmd/verdi/designsupersede.go) and the board's Revise action
// (internal/workbench/boardspecapi.go actionRevise) — keep reading
// supersede.ValidateSuccessorName / supersede.NameError / supersede.ReasonXxx
// exactly as before, with no import-path or call-site rename forced on
// them; the two callers with no supersede semantics of their own (the
// plain `design start` path, the board's create action) import
// internal/specname directly instead. See internal/specname/validate.go
// for the implementation and its own exhaustive tests — this package's own
// validate_test.go only proves the re-export wires correctly.
//
// Note for a reader of resolve.go's own package doc comment ("Resolve's
// own reasons are below; ValidateSuccessorName's are in validate.go"):
// that sentence is still true in spirit (this file is still where they are
// FOUND, from this package), but the two Reason types are no longer the
// same Go type — ResolveError.Reason stays this package's own
// supersede.Reason (resolve.go, untouched by this move); NameError.Reason
// is specname.Reason, aliased in below under different constant names so
// the two never collide.
package supersede

import "github.com/jyang234/verdi/internal/specname"

// NameError is specname.NameError, re-exported under this package's own
// established name (see the package doc comment above).
type NameError = specname.NameError

// Reason values specname.ValidateSuccessorName's NameError.Reason can
// carry — re-exported so a caller comparing against
// supersede.ReasonSuccessorExists (etc.) needs no second import. Each
// constant's own static type is specname.Reason (inherited from the
// right-hand side), distinct from this package's OWN Reason type
// (resolve.go, ResolveError's predecessor-side reasons) — the two never
// collide because they are different identifiers.
const (
	ReasonInvalidName     = specname.ReasonInvalidName
	ReasonSuccessorExists = specname.ReasonSuccessorExists
	ReasonArchivedExists  = specname.ReasonArchivedExists
	ReasonExistsOnBase    = specname.ReasonExistsOnBase
)

// ValidateSuccessorName is specname.ValidateSuccessorName, re-exported
// (see the package doc comment above). ctx is required (BlobAt's base-ref
// probe, run only when baseRef != ""); baseRef is the ref the caller's new
// branch will actually be cut from, or "" to skip that third check
// (UAT-031).
var ValidateSuccessorName = specname.ValidateSuccessorName
