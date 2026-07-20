// verdi audit (05 §CLI, R4-I-10): audits ADR exemptions (03 §Exemption
// audit — per-ADR active-exemption count against
// verdi.yaml's audit.exempts_conflict_threshold, auto-filing a conflict
// record at threshold), mid-build deviations (03 §The amendment ladder's
// spec-stale flag, V1-P3's internal/evidence.SpecStale, against
// audit.deviations_stale_threshold), and — spec/closure-hygiene, a third,
// additive section (dc-1) — closure hygiene: every active-zone spec whose
// declared status contradicts git reality, every stranded close/<name>
// branch, and a read-only survey of merged-but-undeleted branches and
// worktrees (internal/residue.Scan). Both thresholds are tunable
// (verdi.yaml's audit: block, decoded by internal/store) and default to 3
// when absent (internal/decisionsweep.DefaultExemptsConflictThreshold,
// internal/evidence.DefaultDeviationsStaleThreshold).
//
// Exit contract (CLAUDE.md 0/1/2): 0 clean (nothing flagged, nothing
// newly filed beyond a routine run); 1 verdict — an ADR crossed the
// exemption threshold this run (a new conflict was just filed), a story
// is spec-stale, a spec's active-zone status contradicts git reality
// (spec/closure-hygiene AC-1 pattern (a)), or a close/<name> branch is
// ritual-incomplete (AC-2); 2 operational error. Auditing itself is never
// destructive beyond the auto-file (deterministic, idempotent —
// internal/decisionsweep) and internal/residue's own read-only scan
// (spec/closure-hygiene co-1) so a report-only run (nothing crossed a
// threshold, no contradiction found) still exits 0 even though `audit`
// "found" pre-existing, already-filed exemptions or an ordinary
// merged-but-undeleted branch.
package main

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/decisionsweep"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/residue"
	"github.com/jyang234/verdi/internal/store"
)

// cmdAudit is `verdi audit`'s entry point, invoked by dispatch.go.
func cmdAudit(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "audit: usage: verdi audit (no arguments)")
		return 2
	}

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "audit:", err)
		return 2
	}
	// store.Open (not the bare loadManifest delegate): audit's empty-sweep
	// line resolves display vocabulary through Config.Model (L-M13(1)) —
	// one open yields both halves.
	cfg, err := store.Open(root)
	if err != nil {
		fmt.Fprintln(stderr, "audit:", err)
		return 2
	}
	manifest := cfg.Manifest

	exemptsThreshold, deviationsThreshold := 0, 0
	if manifest.Audit != nil {
		exemptsThreshold = manifest.Audit.ExemptsConflictThreshold
		deviationsThreshold = manifest.Audit.DeviationsStaleThreshold
	}

	ctx := context.Background()
	// lint.ResolveDefaultBranch: the same "which branch is the default"
	// seam every other verb that needs it shares (gate, close, gc, ...) —
	// internal/residue.Scan takes it caller-resolved (mirroring
	// internal/wtmanager.GC's own convention) rather than re-resolving it
	// itself.
	defaultBranchRef := lint.ResolveDefaultBranch(ctx, root)

	return runAudit(ctx, root, exemptsThreshold, deviationsThreshold, defaultBranchRef, cfg.Model, stdout, stderr)
}

// runAudit is the testable core: given an already-resolved root,
// thresholds, the resolved default branch ref, and the resolved display
// model (nil-safe: bare-id fallback), runs internal/decisionsweep.Audit
// and internal/residue.Scan and reports both.
func runAudit(ctx context.Context, root string, exemptsThreshold, deviationsThreshold int, defaultBranchRef string, mdl *model.Model, stdout, stderr io.Writer) int {
	result, err := decisionsweep.Audit(root, exemptsThreshold, deviationsThreshold)
	if err != nil {
		fmt.Fprintln(stderr, "audit:", err)
		return 2
	}

	flagged := false

	fmt.Fprintln(stdout, "== Exemption audit ==")
	if len(result.Exemptions) == 0 {
		fmt.Fprintln(stdout, "(no active exemptions)")
	}
	for _, e := range result.Exemptions {
		fmt.Fprintf(stdout, "%s: %d active exemption(s)\n", e.ADRRef, e.Count())
		for _, s := range e.Sources {
			fmt.Fprintf(stdout, "  - %s#%s: %s\n", s.SpecRef, s.DecisionID, s.Reason)
		}
	}
	filed := append([]string(nil), result.Filed...)
	sort.Strings(filed)
	for _, path := range filed {
		fmt.Fprintf(stdout, "FILED: %s\n", path)
		flagged = true
	}

	fmt.Fprintln(stdout, "== Spec-stale audit ==")
	if len(result.SpecStale) == 0 {
		// The class plural is display prose and resolves (L-M13(1)); the
		// per-row "accepted-deviations" label below is the deviation
		// ledger's own count key, not a lifecycle state — identity.
		fmt.Fprintf(stdout, "(no %s with a deviation report to audit)\n", mdl.DisplayClassPlural("story"))
	}
	for _, e := range result.SpecStale {
		status := "ok"
		if e.Result.Flagged {
			status = "SPEC-STALE"
			flagged = true
		}
		fmt.Fprintf(stdout, "%s: %s (accepted-deviations: %d)\n", e.StoryRef, status, e.Result.AcceptedDeviationCount)
	}

	// == Closure hygiene audit == (spec/closure-hygiene dc-1: a third,
	// independent pass appended to the same run — co-2: the two sections
	// above, and their own exit-code contributions, are byte-for-byte
	// unchanged by this addition).
	if rc := runClosureHygieneSection(ctx, root, defaultBranchRef, mdl, stdout, stderr, &flagged); rc != 0 {
		return rc
	}

	if flagged {
		fmt.Fprintln(stdout, "audit: FLAGGED")
		return 1
	}
	fmt.Fprintln(stdout, "audit: CLEAN")
	return 0
}

// runClosureHygieneSection renders `== Closure hygiene audit ==`: AC-1's
// two status-vs-git-reality patterns, AC-2's close/<name> classification,
// and AC-3's read-only merged-branch/worktree survey. Returns 2 (and
// leaves *flagged untouched) on an internal/residue.Scan operational
// error; otherwise always returns 0, setting *flagged per dc-3 (only
// pattern (a) and a ritual-incomplete classification contribute).
func runClosureHygieneSection(ctx context.Context, root, defaultBranchRef string, mdl *model.Model, stdout, stderr io.Writer, flagged *bool) int {
	res, err := residue.Scan(ctx, root, defaultBranchRef)
	if err != nil {
		fmt.Fprintln(stderr, "audit:", err)
		return 2
	}

	fmt.Fprintln(stdout, "== Closure hygiene audit ==")
	if !res.DefaultBranchResolved {
		fmt.Fprintln(stdout, "(default branch could not be resolved; closure hygiene checks skipped)")
		return 0
	}

	if len(res.PatternA) == 0 && len(res.PatternB) == 0 && len(res.CloseBranches) == 0 {
		fmt.Fprintln(stdout, "(no status/git-reality contradictions found)")
	}
	for _, pa := range res.PatternA {
		// pa.Class is the spec's own declared class ("feature" or "story");
		// the accepted-pending-build state word resolves through the display
		// chain (spec/vocabulary-surfaces) rather than printing the bare
		// wire value as prose.
		fmt.Fprintf(stdout, "STRANDED: spec/%s (close/%s, tip %s) — status: %s but its close branch already moved it to archive/, unmerged\n",
			pa.SpecName, pa.SpecName, pa.Tip, mdl.DisplayState(pa.Class, "accepted-pending-build"))
		*flagged = true
	}
	for _, pb := range res.PatternB {
		// Pattern (b) only ever fires for class: feature (dc-1's own static
		// obligation) — both the class word and the closed state word
		// resolve through the display chain.
		fmt.Fprintf(stdout, "STUB-COMPLETE: spec/%s — every declared stub realized (%s), %s not yet %s\n",
			pb.SpecName, strings.Join(pb.Stubs, ", "), mdl.DisplayClass("feature"), mdl.DisplayState("feature", "closed"))
	}
	for _, cb := range res.CloseBranches {
		fmt.Fprintf(stdout, "%s: %s (tip %s)\n", cb.Branch, cb.Class, cb.Tip)
		if cb.Class == residue.RitualIncomplete {
			*flagged = true
		}
	}

	if len(res.MergedBranches) == 0 {
		fmt.Fprintln(stdout, "(no merged-but-undeleted branches)")
	} else {
		fmt.Fprintf(stdout, "merged branches: %d (%s)\n", len(res.MergedBranches), strings.Join(res.MergedBranches, ", "))
	}

	if len(res.Worktrees) == 0 {
		fmt.Fprintln(stdout, "(no other registered worktrees)")
	}
	for _, wt := range res.Worktrees {
		fmt.Fprintln(stdout, renderWorktreeLine(wt))
	}

	return 0
}

// renderWorktreeLine is AC-3(b)'s one-line-per-worktree disclosure,
// naming its branch (or, for a detached HEAD, its commit alone — dc-4:
// never a guessed branch name), and its merged/clean/managed state. Where
// a per-worktree state could not be resolved (e.g. a worktree directory
// deleted without `git worktree remove`), that aspect is disclosed as
// "unresolvable" with git's own reason appended, rather than guessed
// either way (AC-3(b): "disclosed rather than guessed").
func renderWorktreeLine(wt residue.Worktree) string {
	branch := "branch " + wt.Branch
	if wt.Branch == "" {
		branch = "detached at " + wt.Commit
	}
	merged := "unmerged"
	switch {
	case wt.MergedUnresolved:
		merged = "merge state unresolvable"
	case wt.Merged:
		merged = "merged"
	}
	clean := "dirty"
	switch {
	case wt.DirtyUnresolved:
		clean = "clean state unresolvable"
	case !wt.Dirty:
		clean = "clean"
	}
	managed := "unmanaged"
	if wt.Managed {
		managed = "managed"
	}
	line := fmt.Sprintf("worktree %s: %s, %s, %s, %s", wt.Path, branch, merged, clean, managed)
	if wt.Reason != "" {
		line += " (" + wt.Reason + ")"
	}
	return line
}
