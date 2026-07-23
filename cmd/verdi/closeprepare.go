package main

import (
	"context"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

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

// prepareRegenerationSource is the disclosure source id for preparation's
// one destructive step (internal/disclosure's producer-id convention: the
// verb and the condition, never a new taxonomy).
const prepareRegenerationSource = "close:prepare-regeneration"

// discloseRegeneratedDispositions names every human disposition a refresh is
// about to regenerate over, BEFORE the refresh runs.
//
// Preparation only refreshes a report that does not cover HEAD, and
// regeneration re-derives findings from scratch: align.PreserveDispositions
// carries a disposition forward only where the regenerated finding's
// (kind, id, text) content hash matches exactly, and ReconcileJudged
// re-offers a non-matching prior judged ruling as a candidate a human must
// confirm rather than as a carried disposition. A finding whose text
// drifted — or which this run does not re-derive at all — therefore loses
// its disposition and its human-authored note.
//
// That behavior is the design's ("retry safety is scoped to the same
// repository state") and is deliberately NOT changed here: this function
// adds no branch, refuses nothing, and returns nothing. What it removes is
// the silence. Three-valued honesty admits proven, violated-with-witness,
// and disclosed-as-unproven — never a silent discard of human judgment —
// so the loss is rendered through the shared disclosure seam, in the one
// vocabulary every other disclosure in this binary speaks, rather than as a
// local fmt.Sprintf that could drift from it.
func discloseRegeneratedDispositions(report *artifact.DeviationFrontmatter, specRef artifact.Ref, head string, stdout io.Writer) {
	if report == nil {
		return
	}
	for _, finding := range report.Findings {
		if !finding.Dispositioned() {
			continue
		}
		fmt.Fprintln(stdout, disclosure.Render(disclosure.New(
			prepareRegenerationSource,
			specRef.String(),
			regeneratedDispositionText(finding, head),
		)))
	}
}

// regeneratedDispositionText is one disclosure's human-readable half: which
// finding, what human state it holds, and the exact rule that decides
// whether that state survives the refresh — stated per kind, because judged
// and computed findings do not follow the same carry rule.
func regeneratedDispositionText(finding artifact.Finding, head string) string {
	note := ""
	if finding.Note != "" {
		note = " and a human-authored note"
	}
	carry := "a disposition survives only where the regenerated finding's kind, id and text are identical"
	if finding.Kind == artifact.FindingJudged {
		carry = "a judged ruling survives only where the regenerated finding's kind, id and text are identical, and is otherwise re-offered as a candidate a human must reaffirm"
	}
	return fmt.Sprintf(
		"%s (%s) holds the human disposition %q%s; the refresh about to run for HEAD %s re-derives every finding, and %s — so this one may not survive it",
		finding.ID, finding.Kind, finding.Disposition, note, head, carry,
	)
}

// reportFrozenState reports the one report state preparation can neither
// refresh nor hand to the gate: an ALREADY-FROZEN living deviation report.
// It returns 1 (a verdict — preparation cannot proceed) once it has named
// the state, or 0 when report is absent or living, leaving every other path
// exactly as it was.
//
// The state is reachable and was previously invisible in BOTH directions. A
// frozen report covering HEAD passes the freshness check (it covers HEAD)
// and the gate's disposition condition (which does not inspect the frozen
// stamp), so preparation fell through to READY and printed a next command
// that structurally cannot succeed: close's freeze step refuses an
// already-frozen report. A frozen report that does NOT cover HEAD hit that
// same refusal one layer down, inside the align engine, surfacing an
// unframed `align:` line with no diagnosis and no next step.
//
// It is rendered under MECHANICAL WORK REQUIRED rather than a new summary
// word: the design's state table admits exactly that for non-judgment
// blocked states in the first iteration ("the authoritative condition text
// remains visible and no decision is automated"), and the machine boundary
// the design cares about — JUDGMENT REQUIRED versus everything else — is
// preserved. Inventing a seventh state name would be spec text this code
// has no authority to write.
//
// Nothing is unfrozen and nothing is repaired: a frozen report is immutable
// (align.go's own refusal says so), and choosing between restoring a living
// report and completing an interrupted archive move is human work.
func reportFrozenState(report *artifact.DeviationFrontmatter, specName string, stdout io.Writer) int {
	if report == nil || report.Frozen == nil {
		return 0
	}
	reportRelPath := store.DeviationReportRelPath(store.ZoneActive, specName)
	archiveRelPath := store.SpecDirRelPath(store.ZoneArchive, specName)
	fmt.Fprintf(stdout, "close: --prepare: MECHANICAL WORK REQUIRED (%s is already frozen at %s, commit %s; a frozen alignment report is immutable)\n", reportRelPath, report.Frozen.At, report.Frozen.Commit)
	fmt.Fprintln(stdout, "close: --prepare: preparation refreshes only a LIVING report and never unfreezes one; the closure ritual is equally blocked here, because its own freeze step refuses an already-frozen report.")
	fmt.Fprintf(stdout, "close: --prepare: two paths reach this state — a freeze that ran before the archive move without that move completing, or an explicit align --freeze — and preparation cannot tell them apart. Inspect %s: if it already holds this spec the closure completed and the active copy is residue; if it does not, deciding whether to restore a living report or finish the archive move is human work.\n", archiveRelPath)
	return 1
}

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
//
// The states it derives, in the order they are decided: an already-frozen
// report (reportFrozenState — neither refreshable nor closeable), an absent
// or stale report (refreshed through the shared align engine, after
// discloseRegeneratedDispositions names what that refresh may cost), a
// current report with undispositioned findings (human judgment, printed as
// exact disposition commands), and otherwise the authoritative preflight.
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
	if rc := reportFrozenState(report, specRef.Name, stdout); rc != 0 {
		return rc
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
		discloseRegeneratedDispositions(report, specRef, head, stdout)
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
	// The echoed ref is quoted for the same reason every word of the
	// disposition templates above is: this is a line an operator copies into
	// a shell, and correctness must not rest on an argument-grammar analysis
	// holding forever. That analysis is currently narrow but NOT empty:
	// storyresolve accepts only artifact's scheme:key story refs and
	// kebab-case spec refs, whose characters are all shell-safe — except
	// that ParseRef also admits a fragment ref (spec/<name>#<object-id>) and
	// Resolve ignores the fragment when loading the spec, so
	// `spec/close-fixture#ac-1` resolves and reaches this line. Under zsh
	// with extended_glob set, a bare '#' is a pattern operator and the
	// pasted command dies with "no matches found" before reaching the verb.
	// vocab:identity — CLI next-command verb-name grammar (identity)
	fmt.Fprintf(stdout, "close: --prepare: next command: verdi close %s%s\n", shellQuoteWord(storyArg), forceArg)
	return 0
}
