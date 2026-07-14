// Package wtmanager is spec/worktree-manager's backend seam: lazily
// cutting, reusing, and reclaiming a managed git worktree per local
// design branch, entirely under a checkout's data zone.
//
// EnsureWorktree(ctx, root, branch) is the lazy, synchronous, idempotent
// entry point spec/workbench-directory's draft-boards story consumes to
// serve a draft's own working tree without ever touching the serving
// checkout's own branch, index, or working tree (dc-1). GC(ctx, root,
// defaultBranchRef) is the managed-worktree reclamation slice behind
// `verdi gc` (dc-3/dc-4/dc-5): merged-or-locally-deleted, clean, unlocked
// worktrees are reaped; everything else is disclosed and kept.
//
// Layout (co-1, dc-1): a design branch design/<name> maps to
// .verdi/data/worktrees/<name>/, guarded by a sibling lockfile at
// .verdi/data/worktrees/<name>.lock using the exact algorithm
// internal/filelock already implements for the per-checkout writer lock
// (dc-2) — held only for the duration of the one git-worktree-mutating
// call (add or remove) that needs it, never for the worktree's idle
// lifetime in between.
package wtmanager
