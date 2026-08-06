// Package policyintegration holds this delivery unit's cross-package
// AC-1 integration proofs. It exports nothing: every file here is a
// test that drives internal/policyartifact, internal/policyauthority,
// internal/humanartifact, and internal/instructionprojection together,
// through their own public entry points only, exactly the way a real
// caller (a future CLI verb, the workbench, or an adapter) would chain
// them.
//
// The four packages each carry their own happy-path and negative-path
// unit coverage; this package does not repeat it. What it proves instead
// is the seam BETWEEN them: that a store built by hand from committed-
// artifact-shaped fixtures survives Load, Resolve, scaffold-render-
// write, Generate, and Verify as one coherent chain (AC-1's storage,
// renderer, resolution, and projection stages); that adoption stays
// opt-in and reversible across all four packages' own entry points
// (DC-15 — no legacy store, and no incompletely adopted store, ever
// yields a value claiming constitution-backed authority anywhere in this
// surface); that template and policy evolution are prospective, never
// retroactive (CO-5 — a previously rendered artifact and a previously
// generated projection keep their own recorded identity and bytes until
// something re-renders or re-generates them); and that the whole chain
// is deterministic under textually different but semantically identical
// source ordering (CO-3, restated at the cross-package boundary).
//
// This unit's browser-facing surface is none: the four packages'
// exported surface is Go-only (no workbench page, no board, no dex
// output) — the CLAUDE.md frontend-dispatch exception does not apply
// here for lack of any UI to dispatch. The CLI behavioral paths this
// chain will eventually back (a lint-admission gate over an adopted
// store, a harness model/instruction check) are covered inside
// internal/policyauthority and internal/instructionprojection's own
// test suites already; this package does not add a second CLI-shaped
// proof for them.
package policyintegration
