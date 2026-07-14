// Package lint is artifactlint: the VL-001..VL-019 rules (see engine.go's
// allRules for the authoritative registry) from
// 02 §Lint rules, run over a store root's committed zone plus whatever git
// and service-discovery context each rule needs. It is dependency-light Go
// — no frameworks, no services in the gate path (02 §Lint rules) — built
// entirely on internal/artifact's single decode seam, internal/store's
// manifest/discovery, and internal/gitx's plumbing helpers.
//
// Design note: VL-001 ("frontmatter present, decodes strictly against kind
// schema; the restricted dialect is enforced here") is implemented as
// exactly internal/artifact.DecodeStrict succeeding or failing — the
// syntactic half of decode (frontmatter presence, unknown fields, anchors/
// aliases/custom tags). Every other rule's semantic content check (id/path
// agreement, status legality, story shape, AC evidence, frozen/provenance
// requiredness, disposition completeness, ...) is re-implemented directly
// against the raw decoded struct by the specific VL-xxx rule 02 assigns it
// to, rather than by calling a kind's Validate(). This is what lets a
// document that would fail internal/artifact's full Decode<Kind> (defense
// in depth) still surface as its own specific rule rather than being
// swallowed by VL-001 — see testdata/violations/README.md's design note.
package lint
