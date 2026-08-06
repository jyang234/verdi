// Package policyauthority is the constitution store's one loader and one
// effective-policy resolver (spec/context-integrity-v2 DC-23/owner ruling
// OD-5: "policy storage, inheritance, effective-policy resolution, and
// policy identity and digest are owned exclusively here ... no
// feature-local fallback, competing hierarchy, or second policy
// interpretation is permitted"). Every feature-specific governance
// configuration — including AI-assisted design-assistance policy and
// comparative-experiment policy — reaches this system only as a typed
// policyartifact.Payload; there is no untyped or feature-local path
// around Load and Resolve.
//
// Load walks a repository's .verdi/policy/ constitution store, strict-
// decodes every artifact through internal/policyartifact's sealed
// decoders, and cross-validates the store as a whole (selected profile,
// claim-subject registration, scope-environment registration, overlay
// refinement targets, exemption witness freshness, approval roles, and
// payload-kind uniqueness). Its result, *Store, is sealed the same way
// internal/policyartifact and internal/governanceprincipal seal their own
// decoded values: only Load's own output satisfies Resolve's gate, so a
// hand-built or zero-value Store fails closed rather than silently
// resolving.
//
// Resolve computes the one canonical *EffectivePolicy from a loaded
// Store: every base policy claim plus the scope-bounded refinements its
// applicable overlays prove under DC-3's narrow-only discipline
// (specificity alone never changes authority — an overlay may narrow a
// declared-overridable claim only in the direction that removes
// optionality, never widens it, and only inside the overlay's own scope),
// with exemptions carried as recorded facts, never evaluated here. A
// refinement is recorded against the scope that bounds it rather than
// flattened into the base operand, and refinements at distinct scopes are
// never combined: that would need scope-comparison semantics this unit
// does not own. Its result is itself sealed against post-Resolve
// mutation; Digest() verifies the seal before returning the canonical
// content address.
//
// Nothing in this package exposes a way to compute an effective value
// from raw policyartifact values without going through Load first: the
// unexported Store marker is the structural proof that ASD, CSE, or any
// other feature payload can register a typed Payload kind (via
// policyartifact.RegisterPayloadKind) but can never install a second
// resolver alongside this one.
package policyauthority
