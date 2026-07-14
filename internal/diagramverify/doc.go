// Package diagramverify turns a `class: proposal` diagram's authored
// mermaid body into a comparable, truth-checked structure
// (spec/verification-extractor, implementing spec/diagram-proposals ac-1).
//
// Four pieces, one per acceptance criterion:
//
//   - grammar.go (ac-1): Parse extracts a declared, honestly-bounded
//     mermaid-flowchart grammar subset (dc-1) into a node/edge set,
//     normalizing each node's raw mermaid id against regenerated truth's
//     shortName space (dc-2) and disclosing one whole-artifact Coverage
//     verdict (full/partial) — never a per-line annotation.
//   - truth.go (ac-2): RegenerateTruth execs the pinned flowmap CLI's
//     `graph` subcommand at the proposal's declared scope (or unscoped)
//     through the existing internal/upstream seam (dc-3) and derives the
//     shortName-keyed identity sets Compare needs.
//   - compare.go (ac-3): Compare runs the three-way structural comparison
//     (exists / proposed-new / kept-but-gone) between a proposal's and its
//     base's extracted identity sets against truth, attaching a
//     candidate — never causally-verified — witness commit to each
//     kept-but-gone result (dc-4). This is a computed, unpersisted-to-
//     artifact Go result, not diagram frontmatter schema.
//   - stale.go (ac-4): StaleBase recomputes a derived proposal's base
//     digest at current HEAD and compares it against derived_from.digest
//     (dc-5), independently of Compare's own result.
//
// No LLM anywhere in this package (co-1): every computation here is a pure
// function of pinned inputs, or a hermetically-testable exec through
// internal/upstream/gitx. This package never reimplements flowmap's own
// call-graph construction (co-2) — it only extracts, normalizes, and
// diffs identity strings flowmap's graph JSON and a proposal's mermaid
// text already carry.
package diagramverify
