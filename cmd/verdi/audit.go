// verdi audit (05 §CLI, R4-I-10): audits ADR exemptions (03 §Exemption
// audit — per-ADR active-exemption count against
// verdi.yaml's audit.exempts_conflict_threshold, auto-filing a conflict
// record at threshold) and mid-build deviations (03 §The amendment
// ladder's spec-stale flag, V1-P3's internal/evidence.SpecStale, against
// audit.deviations_stale_threshold). Both thresholds are tunable
// (verdi.yaml's audit: block, decoded by internal/store) and default to 3
// when absent (internal/decisionsweep.DefaultExemptsConflictThreshold,
// internal/evidence.DefaultDeviationsStaleThreshold).
//
// Exit contract (CLAUDE.md 0/1/2): 0 clean (nothing flagged, nothing
// newly filed beyond a routine run); 1 verdict — an ADR crossed the
// exemption threshold this run (a new conflict was just filed) or a story
// is spec-stale; 2 operational error. Auditing itself is never destructive
// beyond the auto-file (deterministic, idempotent — internal/decisionsweep)
// so a report-only run (nothing crossed a threshold) still exits 0 even
// though `audit` "found" pre-existing, already-filed exemptions.
package main

import (
	"fmt"
	"io"
	"sort"

	"github.com/jyang234/verdi/internal/decisionsweep"
	"github.com/jyang234/verdi/internal/model"
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

	return runAudit(root, exemptsThreshold, deviationsThreshold, cfg.Model, stdout, stderr)
}

// runAudit is the testable core: given an already-resolved root,
// thresholds, and the resolved display model (nil-safe: bare-id
// fallback), runs internal/decisionsweep.Audit and reports its findings.
func runAudit(root string, exemptsThreshold, deviationsThreshold int, mdl *model.Model, stdout, stderr io.Writer) int {
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

	if flagged {
		fmt.Fprintln(stdout, "audit: FLAGGED")
		return 1
	}
	fmt.Fprintln(stdout, "audit: CLEAN")
	return 0
}
