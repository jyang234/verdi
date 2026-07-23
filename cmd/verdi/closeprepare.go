package main

import (
	"context"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

// runPrepare derives the next closure-session state for an explicit story or
// feature ref. It may refresh only the target's living deviation report; all
// judgment remains a human-authored disposition and final closure remains a
// separate invocation of the existing close ritual.
//
// Ledger L-N15(1). State is DERIVED, never persisted — a closure-session
// file was rejected as a second source of truth for HEAD, report freshness,
// findings, and gate state. This composes the existing engines only
// (storyresolve.Resolve, runAlignForSpec(freeze=false), runPreflight): no
// second align engine, no second gate, and no new pass path — final
// readiness stays the existing closure gate's verdict. Preserving a current
// undispositioned report byte-for-byte is what keeps X-16's forcing
// function intact across retries.
// prepareExpiryResumeHint is preparation's own bounded-wait resume guidance
// (alignDeps.ResumeHint), naming the exact command that resumes THIS run.
//
// The field is documented as the calling verb's own vocabulary, and neither
// existing hint is preparation's: align's speaks a --wait flag preparation
// exposes no way to pass, and close's ("Re-run verdi close … to complete
// the freeze and archive") would send an operator who ran --prepare
// precisely so as NOT to freeze or archive yet into the real ritual
// instead. The ref goes through shellQuoteWord for the same reason the
// disposition templates do — it is a line the operator copies.
func prepareExpiryResumeHint(storyArg string) string {
	// vocab:identity — CLI invocation grammar ("verdi close --prepare", identity)
	return "Re-run verdi close --prepare " + shellQuoteWord(storyArg) + " once the judge window allows"
}

// prepareAlignDeps builds the alignDeps preparation hands the shared align
// engine. It is freezeAlignDeps' single construction (close.go) with
// exactly one field overridden, never a second literal: close introduced
// that helper so its two callers "can never drift", and preparation writes
// the very report close later freezes, so it must inherit the same judge
// contract rather than restate a subset of it.
//
// Wait is the field that makes this load-bearing (spec/judge-ergonomics
// ac-3). Without it, a judge that outruns its ceiling does not fail: it
// degrades to align's synthetic "judged coverage absent" finding, which
// preparation would write into the living report and present as ordinary
// JUDGMENT REQUIRED work. Once a human dispositioned that synthetic
// finding, close's freeze-in-place branch (align.go) would stamp the judge
// failure into the archive verbatim, without ever re-running the judge —
// exactly what freezeAlignDeps' Wait prevents for close's own freeze.
//
// ResumeHint is the one documented per-caller field (see the alignDeps
// field's own doc comment, and finding judged-close-inherits-aligns-resume-
// instructions-verbatim, which exists because inheriting another verb's
// resume language verbatim misdirects the operator). Overriding it is a
// deliberate, tested divergence — TestRunPrepare_JudgeTimeoutResumeHint
// SpeaksPreparation pins it — not a silent omission; every other field,
// including any field added to alignDeps later, comes from freezeAlignDeps.
func prepareAlignDeps(deps closeDeps, modelDigest, storyArg string) alignDeps {
	alignD := freezeAlignDeps(deps, modelDigest)
	alignD.ResumeHint = prepareExpiryResumeHint(storyArg)
	return alignD
}

func runPrepare(ctx context.Context, root, storyArg string, manifest *store.Manifest, deps closeDeps, forceLocal bool, stdout, stderr io.Writer) int {
	spec, err := storyresolve.Resolve(root, storyArg)
	if err != nil {
		fmt.Fprintln(stderr, "close: --prepare:", err)
		return 2
	}
	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		fmt.Fprintln(stderr, "close: --prepare: internal error: resolved spec has an invalid id:", err)
		return 2
	}
	head, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "close: --prepare:", err)
		return 2
	}
	reportPath := store.DeviationReportPath(root, store.ZoneActive, specRef.Name)
	report, _, err := loadExistingReport(reportPath)
	if err != nil {
		fmt.Fprintln(stderr, "close: --prepare:", err)
		return 2
	}

	if report == nil || report.Covers != head {
		freshness := "absent"
		if report != nil {
			freshness = fmt.Sprintf("stale (covers %s)", report.Covers)
		}
		modelDigest, err := resolveModelDigest(root)
		if err != nil {
			fmt.Fprintln(stderr, "close: --prepare:", err)
			return 2
		}
		if rc := runAlignForSpec(ctx, root, spec, head, false, prepareAlignDeps(deps, modelDigest, storyArg), stdout, stderr); rc != 0 {
			return rc
		}
		fmt.Fprintf(stdout, "close: --prepare: ALIGNMENT REQUIRED (living report was %s for HEAD %s; the existing align engine refreshed it)\n", freshness, head)

		report, _, err = loadExistingReport(reportPath)
		if err != nil {
			fmt.Fprintln(stderr, "close: --prepare:", err)
			return 2
		}
		if report == nil {
			fmt.Fprintf(stderr, "close: --prepare: align returned success but %s is absent\n", reportPath)
			return 2
		}
	}

	undispositioned := make([]artifact.Finding, 0, len(report.Findings))
	for _, finding := range report.Findings {
		if !finding.Dispositioned() {
			undispositioned = append(undispositioned, finding)
		}
	}
	if len(undispositioned) > 0 {
		fmt.Fprintf(stdout, "close: --prepare: JUDGMENT REQUIRED (%d undispositioned finding(s) in %s)\n", len(undispositioned), store.DeviationReportRelPath(store.ZoneActive, specRef.Name))
		for _, finding := range undispositioned {
			fmt.Fprintf(
				stdout,
				"verdi disposition --rationale %s -- %s %s %s\n",
				shellQuoteWord("<human-authored rationale>"),
				shellQuoteWord(specRef.String()),
				shellQuoteWord(finding.ID),
				shellQuoteWord("<human-authored-disposition:fixed|accepted-deviation>"),
			)
		}
		return 1
	}

	rc := runPreflight(ctx, root, storyArg, manifest, deps.Model, deps.Forge, forceLocal, stdout, stderr)
	if rc == 1 {
		fmt.Fprintln(stdout, "close: --prepare: MECHANICAL WORK REQUIRED (closure preflight is NOT READY; see its diagnostics above)")
		return 1
	}
	if rc != 0 {
		return rc
	}

	forceArg := ""
	if forceLocal {
		forceArg = " --force-local"
	}
	// vocab:identity — CLI next-command verb-name grammar (identity)
	fmt.Fprintf(stdout, "close: --prepare: next command: verdi close %s%s\n", storyArg, forceArg)
	return 0
}
