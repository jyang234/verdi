---
id: obligation/gc-reclaim--ac-3--static
kind: obligation
title: "The scope-disclosure line and per-item templates are literal constants, and internal/residue plus verdi audit's report sections are byte-unchanged from this story's own merge base"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/gc-reclaim" }
frozen: { at: 2026-07-20, commit: 1dcb8f67e8d20b5930de9aa7966bac935d16d3b8 }
---
# The scope-disclosure line and per-item templates are literal constants, and internal/residue plus verdi audit's report sections are byte-unchanged from this story's own merge base

The static evidence must show `cmd/verdi/gc.go`'s scope-disclosure text is
grown, not replaced, into two literal, inspectable constants — one printed
by a plain `verdi gc` run naming the unmanaged slice as available-but-not-
run-this-invocation, one printed by a `--reclaim-unmanaged` run naming the
managed slice as not-run-this-invocation alongside the still-out-of-scope
derived-cache and layout/tree-hash-cache bullets (`spec/residue-
reclamation` co-1) — never a single shared string whose wording silently
depends on which flag happened to be set.

It must show every reclaimed, kept, and partial-outcome line (dc-4) is
produced by one of a small, named set of line-template functions, mirroring
`spec/worktree-manager`'s own `Result.Line()` shape, rather than ad hoc
`fmt.Sprintf` calls scattered through the execution path.

It must show a diff of `internal/residue`'s package, all three existing
`== ... audit ==` report sections in `cmd/verdi/audit.go`, and
`internal/wtmanager`'s existing managed-worktree GC logic, taken against
this story's own merge base, is empty (co-2) — the only files this story
touches are the new `internal/reclaim` package, the additive
`gitx.DeleteMergedBranch` primitive, and `cmd/verdi/gc.go`'s own flag
dispatch plus its scope-disclosure constants.
