// Package designapp is the sole application consumer for the six
// AI-assisted spec design (ASD) operations AC-8 fixes:
//
//   - get_board
//   - get_design_context
//   - get_design_capabilities
//   - mutate_draft
//   - get_design_provenance
//   - prepare_design_review
//
// (Wave 6 authority design §6.1: "internal/designapp becomes the sole
// application consumer for the six ASD operations already fixed by AC-8.")
//
// Service composes the existing schema/mutation/provenance/board owners
// (internal/draftmutation, internal/designprovenance,
// internal/workbench's board projection, internal/policyauthority,
// internal/specstate, internal/align) behind consumer-owned ports (the 04
// §port pattern: interfaces live here, at the consumer, not in the owner
// packages) rather than reimplementing any of their algorithms. CLI
// (cmd/verdi/design*.go) and MCP (internal/mcpserve's five new
// tool_*.go files plus the existing get_board) adapters both route
// through these exact six methods, so one conformance suite
// (conformance_test.go) can prove they return byte-identical typed
// results for identical inputs (AC-8's adapter-conformance requirement,
// CO-9 §Adapter conformance).
//
// MutateDraft is a pass-through onto draftmutation.Service.Mutate: its
// request/response/error types are draftmutation's own closed union,
// never re-encoded or reinterpreted here (AC-1's mutation contract stays
// the one algorithm — CO-1's three-valued honesty is draftmutation's to
// keep). The other five operations are genuinely new read/derive
// surfaces; each defines its own strict, deterministic, deep-copy-safe
// request/result pair in this package and preserves the same clean
// (0) / verdict (1) / operational (2) classification via the shared
// Classification/Error types in outcome.go.
//
// Browser attribution (SI-163) is explicitly NOT implemented here: MCP's
// mutate_draft always mints a delegated-agent actor
// (draftmutation.NewDelegatedAgent), matching AC-2's "MCP actor stays
// agent-controlled" instruction. A future Task 2 workbench adapter is
// responsible for the kernel's explicit unauthenticated-human attribution
// on browser-originated draft edits; nothing in this package special-cases
// a browser caller.
package designapp
