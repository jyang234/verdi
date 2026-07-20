// Package reclaim implements spec/gc-reclaim's sweep: `verdi gc
// --reclaim-unmanaged`'s eligibility predicate (ac-1) and its execution
// engine (ac-2), consuming *internal/residue.Result directly rather than
// re-deriving any of its facts from git independently (dc-1) — the audit's
// report and gc's plan are the SAME computation, internal/residue.Scan,
// read by two different verbs, never computed twice.
//
// Compute (predicate.go) is a pure function: *residue.Result plus the
// invoking checkout's own already-resolved root and current branch in,
// a Plan out — no gitx calls of any kind. Apply (execute.go) is the only
// place in this package that mutates anything: per eligible Plan item, it
// removes the worktree (if any, via the existing gitx.WorktreeRemove, never
// --force) and then deletes the branch (via the new gitx.DeleteMergedBranch,
// dc-3) — each backed by git's own independent refusal as a second guard
// beyond the plan's own facts, disclosed per item, with the sweep
// continuing to the next item on any single item's refusal.
//
// cmd/verdi/gc.go is the sole consumer: it resolves root, the default
// branch ref, and the invoking checkout's identity, calls
// internal/residue.Scan itself (mirroring cmd/verdi/audit.go's own call
// site), then Compute and (on --apply) Apply, rendering every Row this
// package returns. This package owns no CLI parsing, no exit codes, and no
// disclosure-line wording beyond each Row's own Line() — cmd/verdi/gc.go's
// scope-disclosure constants live there, not here (ac-3).
package reclaim
