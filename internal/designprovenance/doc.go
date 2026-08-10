// Package designprovenance defines ASD's committed, append-only,
// non-authoritative design-provenance sidecar. Its records preserve exact
// mutation, policy, attribution, context-unavailability, and semantic-change
// identities without becoming acceptance evidence or normal build context.
package designprovenance

const (
	// Schema is the sidecar entry schema.
	Schema = "verdi.design-provenance/v1"
	// ExclusionReason is the fixed Context Integrity classifier reason.
	ExclusionReason = "design-provenance-sidecar"
	// ContextUnavailableReason is the honest v1 context identity until the
	// context compiler is delivered.
	ContextUnavailableReason = "unavailable-before-context-compiler"
	// MaxExcerptScalars is the per-excerpt Unicode scalar bound.
	MaxExcerptScalars = 600
	// MaxExcerptsPerTarget is the per-target excerpt count bound.
	MaxExcerptsPerTarget = 3
)
