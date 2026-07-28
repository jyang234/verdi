---
id: obligation/obligation-seam--ac-4--static
kind: obligation
title: "a source-text witness proves cmd/verdi carries no second obligation-render or self-validate implementation"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/obligation-seam" }
frozen: { at: 2026-07-21, commit: af0edd77237b6c52cffda3bc344c020ff5fad58e }
---
# a source-text witness proves cmd/verdi carries no second obligation-render or self-validate implementation

The static evidence must show a source-text witness test, in the style of
`internal/workbench/obligationauthor_test.go`'s existing
`TestObligationAuthor_AtomicWrite_NoDirectCreateTemp` (which greps
`obligationauthor.go`'s own source for a banned construct), that reads
`cmd/verdi/accept.go` (or wherever the backstop's scaffolding helper
lands) and `cmd/verdi/obligation.go` and asserts neither file's source
text contains a hand-rolled obligation frontmatter render (no literal
`"id: %s\n"` / `"for_kind: %s\n"`-shaped `fmt.Fprintf` sequence building
obligation YAML by hand, no second `artifact.DecodeObligation`-preceded
self-validate block duplicating the shared seam's own) — proving by
mutation-resistant source inspection, not merely by today's call graph,
that a future edit cannot silently reintroduce a second render path
without this witness turning red. The witness must fail if run against a
deliberately reverted, pre-extraction copy of these files (a copy that
still hand-renders), proving it actually discriminates rather than
passing vacuously.
