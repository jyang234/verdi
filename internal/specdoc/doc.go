// internal/specdoc/doc.go
// Package specdoc renders a spec as a human-readable document from its
// decoded objects, its body, a stamp, and facts supplied by the caller.
//
// The package computes nothing it can receive: criteria coverage and
// open-question claims come from the spec's own stubs (FactsFromSpec),
// evidence state from the matrix projection (WithMatrix), readiness from a
// snapshot the caller owns. It never reads the store, never runs git, and
// never writes anything. A rendered document is a projection of the
// objects and carries a stamp saying so; it is never authority
// (spec/spec-documents co-2).
package specdoc
