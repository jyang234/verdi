// Package policyartifact implements the artifact grammar of the
// constitution store under .verdi/policy/ (spec/context-integrity-v2
// AC-1, DC-3, DC-4, DC-20, DC-23, DC-24; store-layout §Directory layout's
// policy/ entry, whose internal grammar SI-6 assigns to the
// policy-authority unit): strict decoding, semantic validation,
// normalization, canonical encoding, and digesting for the policy,
// policy-overlay, policy-exemption, and policy-constitution artifact
// kinds, plus the storage record for governance-profile artifacts whose
// schema and decoding the governance-principal kernel owns (OD-1, DC-20 —
// this package stores, identifies, and digests profiles; it never decodes
// or interprets their rule content itself).
//
// Every artifact decodes through the single internal/artifact strict seam
// (KnownFields, dialect rejection of anchors/aliases/tags) and fails
// closed on unknown schemas, fields, kinds, enums, constraint families,
// operators, scope dimensions, and payload kinds (co-2). Canonical
// encoding and digesting go through internal/canonjson (co-3): decoded
// values are normalized (semantic sets sorted, content-tiebroken) so
// identical semantic inputs produce byte-identical canonical JSON and
// digests regardless of source ordering.
//
// Decoded artifact values are sealed against post-decode mutation and
// hand construction the same way internal/governanceprincipal seals
// profiles (SI-21's forgery posture): each Decode* mints an unexported
// canonical-content seal, and Digest() and every authority consumer
// verify it before use. A hand-built or mutated value is an operational
// error, never silently accepted authority.
//
// LIFECYCLE CONTRACT — the constitution kinds are STATUSLESS. There is
// deliberately no status field in any of their frontmatter: an artifact
// committed under .verdi/policy/ on the default branch IS active
// authority, its lifecycle state derived from git presence exactly as
// the store's ratified statusless direction derives spec state (VL-015's
// merge-signaled supersession; the attestation kinds' existence-is-the-
// record; DC-14: Git is the constitution's durable layer, never hidden
// mutable state an authorable enum could flip). The effective-policy
// resolver and the workbench inherit this operand: every loaded
// artifact is live BY CONTRACT, and supersession or retirement is a
// later-wave governance flow over git history (DC-15), never a
// frontmatter edit.
package policyartifact
