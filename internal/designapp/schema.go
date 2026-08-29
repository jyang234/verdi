package designapp

import "github.com/jyang234/verdi/internal/draftmutation"

// The ONE place every ASD result envelope's version constant is declared
// (CO-2: "The mutation request and result, project policy, sidecar
// entries, capability responses, and every typed operation use versioned
// strict schemas"). Each of AC-8's six operations returns exactly one
// envelope carrying exactly one of these constants in its own `schema`
// field, so a caller can branch on the version before reading anything
// else — and a future field addition or removal is a visible /v2, never a
// silent shape change.
//
// The names mirror the operation names and follow this repository's
// existing `verdi.<name>/v1` convention (internal/draftmutation/schema.go,
// internal/designprovenance/doc.go, internal/experiment/result.go, ...).
//
//	get_board              -> BoardResultSchema
//	get_design_context     -> ContextResultSchema
//	get_design_capabilities-> CapabilitiesResultSchema
//	mutate_draft           -> MutationResultSchema (draftmutation's own)
//	get_design_provenance  -> ProvenanceResultSchema
//	prepare_design_review  -> ReviewResultSchema
const (
	// BoardResultSchema versions get_board's envelope. The embedded board
	// projection itself is workbench's own unchanged shape (board.go: the
	// projection is never re-derived here); this constant versions the ASD
	// envelope around it, not that projection.
	BoardResultSchema = "verdi.design-board/v1"

	// ContextResultSchema versions get_design_context's bounded AC-5
	// content envelope.
	ContextResultSchema = "verdi.design-context/v1"

	// CapabilitiesResultSchema versions get_design_capabilities' AC-3
	// capability-discovery envelope.
	CapabilitiesResultSchema = "verdi.design-capabilities/v1"

	// ProvenanceResultSchema versions get_design_provenance's envelope.
	// It is deliberately DISTINCT from designprovenance.Schema
	// ("verdi.design-provenance/v1"), which versions one sidecar ENTRY:
	// the entries this envelope carries keep their own per-entry schema
	// field untouched, so the two versions move independently.
	ProvenanceResultSchema = "verdi.design-provenance-result/v1"

	// ReviewResultSchema versions prepare_design_review's AC-6 semantic
	// review packet envelope.
	ReviewResultSchema = "verdi.design-review/v1"
)

// MutationResultSchema is mutate_draft's envelope version. It is
// draftmutation's own constant, not a new one: MutateDraft is a
// pass-through that returns draftmutation's closed Response/*Error union
// byte-identically (mutate.go), and minting a second version constant for
// the same bytes would be exactly the parallel interpretation AC-1
// forbids. Declared here so all six operations' versions are readable in
// one place.
const MutationResultSchema = draftmutation.ResultSchema
