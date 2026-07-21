// verdi gc (spec/worktree-manager ac-5): flips dispatch.go's `"gc": 0,
// // out of v0 (PLAN.md §5)` phase-0 stub to a real, implemented verb —
// the same incremental flip `close` already made for its own verb in
// round 6 (dispatch.go's `"close": 14`). That story implements ONLY the
// managed-worktree reclamation slice (internal/wtmanager.GC, dc-1..4):
// verdi-store-layout's "Garbage collection" section also ratifies
// derived-cache pruning and layout/tree-hash-cache pruning, and NEITHER
// is touched here (dc-5) — cmdGc's own output says so, verbatim, on
// every run, so a human never infers full gc coverage from a partial
// implementation.
//
// --reclaim-unmanaged [--apply] (spec/gc-reclaim) is a SECOND,
// mutually-exclusive mode on this same verb, not a new one (spec/
// gc-reclaim dc-1): bare `verdi gc` (unchanged, above) runs only the
// existing managed-worktree slice; `verdi gc --reclaim-unmanaged` (with or
// without --apply) runs only the new unmanaged slice (internal/reclaim) —
// never both in one invocation, so a run's own mutating-or-not character
// never depends on which flag a reader noticed (ac-3's disclosure contract
// exists to foreclose exactly that). --reclaim-unmanaged alone prints the
// plan and touches nothing; --apply (valid only alongside
// --reclaim-unmanaged) executes it. See runGcReclaimUnmanaged below.
package main

import (
	"context"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/reclaim"
	"github.com/jyang234/verdi/internal/residue"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/wtmanager"
)

// gcScopeDisclosureManaged is dc-5's original mandatory scope line, GROWN
// (ac-3's own static obligation: "grown, not replaced"), never merely
// implied by omission — printed on every PLAIN `verdi gc` run: the
// existing managed-worktrees-only disclosure, now additionally naming
// --reclaim-unmanaged as available but NOT run this invocation (spec/
// gc-reclaim ac-3, dc-1's mutual-exclusivity made observable).
const gcScopeDisclosureManaged = "gc: scope — this run reclaims managed worktrees only (spec/worktree-manager); derived-cache pruning and layout/tree-hash-cache pruning (verdi-store-layout's other ratified gc bullets) are OUT OF SCOPE and were NOT run; unmanaged branch/worktree reclamation is available via --reclaim-unmanaged but was NOT run this invocation"

// gcScopeDisclosureUnmanaged is ac-3's MIRRORED scope line, printed on
// every `--reclaim-unmanaged` run (dry-run or --apply alike): names
// managed-worktree reclamation as available but not run this invocation,
// alongside the same still-out-of-scope derived-cache/layout-cache bullets
// spec/residue-reclamation co-1 leaves untouched either way.
const gcScopeDisclosureUnmanaged = "gc: scope — this run reclaims unmanaged branches/worktrees only (spec/gc-reclaim); managed-worktree reclamation (spec/worktree-manager) is available via a plain `verdi gc` but was NOT run this invocation; derived-cache pruning and layout/tree-hash-cache pruning (verdi-store-layout's other ratified gc bullets) remain OUT OF SCOPE and were NOT run"

// cmdGc is `verdi gc`'s real entry point, invoked by dispatch.go.
func cmdGc(args []string, stdout, stderr io.Writer) int {
	reclaimUnmanaged, apply, err := parseGcArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "gc:", err)
		return 2
	}

	ctx := context.Background()
	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "gc:", err)
		return 2
	}
	defaultBranchRef := lint.ResolveDefaultBranch(ctx, root)

	if reclaimUnmanaged {
		return runGcReclaimUnmanaged(ctx, root, defaultBranchRef, apply, stdout, stderr)
	}

	results, err := wtmanager.GC(ctx, root, defaultBranchRef)
	if err != nil {
		fmt.Fprintln(stderr, "gc:", err)
		return 2
	}

	for _, r := range results {
		fmt.Fprintln(stdout, r.Line())
	}
	fmt.Fprintln(stdout, gcScopeDisclosureManaged)
	return 0
}

// parseGcArgs parses verdi gc's own two flags (spec/gc-reclaim dc-1):
// --reclaim-unmanaged and --apply, the latter valid only alongside the
// former (a bare --apply is a usage error, never silently ignored or
// silently implying --reclaim-unmanaged). Any other argument — an
// unrecognized flag or a stray positional one — is refused by name,
// mirroring cmdGc's own pre-existing "unexpected argument(s)" wording so
// an unrecognized argument reads identically whichever mode a caller
// meant.
func parseGcArgs(args []string) (reclaimUnmanaged, apply bool, err error) {
	for _, a := range args {
		switch a {
		case "--reclaim-unmanaged":
			reclaimUnmanaged = true
			continue
		case "--apply":
			apply = true
			continue
		}
		return false, false, fmt.Errorf("unexpected argument(s) %v", args)
	}
	if apply && !reclaimUnmanaged {
		return false, false, fmt.Errorf("--apply is only valid together with --reclaim-unmanaged")
	}
	return reclaimUnmanaged, apply, nil
}

// runGcReclaimUnmanaged is `--reclaim-unmanaged`'s testable core (mirrors
// audit.go's own cmdAudit/runAudit split — runAudit itself calls
// residue.Scan directly, precisely the "one computation, internal/residue.
// Scan, read by two different verbs, never computed twice" precedent
// spec/gc-reclaim dc-1 draws on). cmdGc resolves root and defaultBranchRef
// from real process state only; this function does the rest, directly
// testable against a fixturegit root with an explicit defaultBranchRef, no
// built binary required.
//
// An unresolvable default branch refuses the WHOLE run before computing
// any plan, dry-run and --apply alike (spec/gc-reclaim ac-2: "asserting
// nothing rather than a plan it cannot compute") — a stricter posture than
// internal/wtmanager's own managed-worktree GC, deliberately: this mode
// reaches worktrees and branches it did not itself create.
func runGcReclaimUnmanaged(ctx context.Context, root, defaultBranchRef string, apply bool, stdout, stderr io.Writer) int {
	res, err := residue.Scan(ctx, root, defaultBranchRef)
	if err != nil {
		fmt.Fprintln(stderr, "gc:", err)
		return 2
	}
	if !res.DefaultBranchResolved {
		fmt.Fprintln(stderr, "gc: --reclaim-unmanaged: default branch could not be resolved; refusing to compute a plan rather than assert one it cannot compute")
		return 2
	}

	// The invoking checkout's own identity (spec/gc-reclaim dc-2), resolved
	// ONCE here from facts this verb already computes: root itself (store.
	// FindRoot, above, unchanged) against worktree rows' Path, and the
	// current branch (gitx.CurrentBranch — "" for a detached invoking HEAD,
	// which then matches no branch-only row) against branch-only rows'
	// names. Never re-derived inside internal/reclaim (dc-1).
	invokingBranch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		fmt.Fprintln(stderr, "gc:", err)
		return 2
	}

	// defaultBranchRef — already resolved above (lint.ResolveDefaultBranch)
	// and handed to residue.Scan — is threaded on to Compute as well, so the
	// predicate keeps any worktree checked out ON the default branch rather
	// than reclaiming it (R4-I-84). Same value, still resolved exactly once;
	// residue.Scan's own DefaultBranchResolved guard above guarantees it is
	// non-empty here.
	plan := reclaim.Compute(res, root, invokingBranch, defaultBranchRef)

	rows := plan.DryRunRows()
	if apply {
		rows = reclaim.Apply(ctx, root, plan)
	}
	for _, row := range rows {
		fmt.Fprintln(stdout, row.Line())
	}
	fmt.Fprintln(stdout, gcScopeDisclosureUnmanaged)
	return 0
}
