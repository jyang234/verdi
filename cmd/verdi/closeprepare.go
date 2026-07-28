package main

import (
	"context"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/align"
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
// about to regenerate over, BEFORE the refresh runs, and returns how many
// disclosures it printed so preparation's own summary can count them.
//
// Counting is not bookkeeping. Printing alone reproduced, one layer up,
// exactly the defect this function was written to close: the lines went out
// through the shared seam and the readiness summary — which counts only the
// closure gate's conditions and preflight's own sources — still printed bare
// READY, a word the design's state table reserves for a gate "fully satisfied
// with no disclosures". A run that says a human-authored note is about to be
// destroyed can never end in that word.
//
// Preparation only refreshes a report that does not cover HEAD, and
// regeneration re-derives findings from scratch: align.PreserveDispositions
// carries a computed disposition forward only where the regenerated finding's
// (kind, id, text) content hash matches exactly, and nothing persists an
// unmatched computed prior — so a computed finding whose text drifted, or
// which this run does not re-derive at all, loses its disposition and its
// human-authored note outright. A judged prior is not destroyed (ReconcileJudged
// persists it into not-resurfaced: and re-offers a recurrence as a candidate),
// but its settled ruling stops applying until a human reaffirms it; each kind
// is disclosed as what it actually is, per regeneratedDispositionText.
//
// That behavior is the design's ("retry safety is scoped to the same
// repository state") and is deliberately NOT changed here: this function
// adds no branch and refuses nothing; it returns only a count. What it removes
// is the silence. Three-valued honesty admits proven, violated-with-witness,
// and disclosed-as-unproven — never a silent discard of human judgment —
// so the cost is rendered through the shared disclosure seam, in the one
// vocabulary every other disclosure in this binary speaks, rather than as a
// local fmt.Sprintf that could drift from it.
func discloseRegeneratedDispositions(report *artifact.DeviationFrontmatter, specRef artifact.Ref, head string, stdout io.Writer) int {
	if report == nil {
		return 0
	}
	printed := 0
	emit := func(text string) {
		fmt.Fprintln(stdout, disclosure.Render(disclosure.New(
			prepareRegenerationSource,
			specRef.String(),
			text,
		)))
		printed++
	}
	for _, finding := range report.Findings {
		if !finding.Dispositioned() {
			continue
		}
		emit(regeneratedDispositionText(finding, head))
	}
	for _, finding := range report.NotResurfaced {
		if !finding.Dispositioned() || finding.Kind == artifact.FindingJudged {
			continue
		}
		emit(discardedNotResurfacedText(finding, head))
	}
	return printed
}

// discardedNotResurfacedText is the disclosure for a dispositioned NON-JUDGED
// entry sitting in not-resurfaced: — a human ruling the refresh discards
// outright, with nothing anywhere to recover it from.
//
// internal/align's ReconcileJudged filters priors to judged twice over (once
// indexing them, once rebuilding the section), so such an entry is neither
// carried into findings: nor persisted back into not-resurfaced:. It simply
// stops existing.
//
// Nothing in the tool produces a non-judged not-resurfaced entry, so reaching
// this state needs a hand-edited or externally-written report — which is why
// the check belongs here rather than being assumed away.
// artifact.DeviationFrontmatter.Validate accepts the shape, and the sibling
// reader over this same working-tree file holds exactly this posture on
// purpose: disposition.go's findNotResurfacedIndex scopes itself to judged as
// "belt-and-suspenders ... but this verb operates over a working-tree file a
// human can hand-edit and must not rely on that invariant holding by
// construction". Preparation reads the same file and owes the same standard —
// and unlike that verb, preparation is the one that destroys it.
//
// A JUDGED entry there is deliberately silent: it is persisted unchanged, so
// there is no cost to disclose and every reason not to dilute the two arms
// beside it that name real ones.
func discardedNotResurfacedText(finding artifact.Finding, head string) string {
	note := ""
	if finding.Note != "" {
		note = " and a human-authored note"
	}
	return fmt.Sprintf(
		"%s (%s) sits in not-resurfaced: holding the human disposition %q%s; the refresh about to run for HEAD %s rebuilds that section from JUDGED priors alone — the reaffirmation machinery is judged-only — so a %s entry there is neither carried nor persisted and this one is discarded outright",
		finding.ID, finding.Kind, finding.Disposition, note, head, finding.Kind,
	)
}

// regeneratedDispositionText is one disclosure's human-readable half: which
// finding, what human state it holds, and the exact outcome the refresh has
// for it — stated per kind, because judged and computed findings do not follow
// the same rule and do not suffer the same cost.
//
// The judged arm once warned that a ruling "may not survive" the refresh. It
// cannot fail to survive: internal/align's ReconcileJudged persists EVERY
// unmatched dispositioned judged prior, so a judged ruling in findings: is
// carried on an exact content match and otherwise lands in not-resurfaced:
// with its note intact, from where a recurrence at the same id is re-offered
// as a candidate. Naming that as a possible loss made the judged arm fire
// unconditionally for a loss that never happens, which is the exact way to
// teach an operator to ignore the computed arm beside it — where the loss is
// real (identity.go's PreserveDispositions is content-hash-exact and nothing
// persists an unmatched computed prior anywhere).
//
// What the judged arm discloses instead is the cost that IS real: a settled
// ruling stops applying until a human reaffirms it.
func regeneratedDispositionText(finding artifact.Finding, head string) string {
	note := ""
	if finding.Note != "" {
		note = " and a human-authored note"
	}
	outcome := "a disposition survives only where the regenerated finding's kind, id and text are identical — so this one may not survive it"
	if finding.Kind == artifact.FindingJudged {
		outcome = "a judged ruling is carried only where the regenerated finding's kind, id and text are identical, and otherwise moves to not-resurfaced: with its note intact, from where a recurrence at the same id is re-offered as a candidate — so this one is not discarded, but the settled ruling stops applying until a human reaffirms it"
	}
	return fmt.Sprintf(
		"%s (%s) holds the human disposition %q%s; the refresh about to run for HEAD %s re-derives every finding, and %s",
		finding.ID, finding.Kind, finding.Disposition, note, head, outcome,
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

// reloadRefreshedReport re-reads the living report the align engine has just
// reported refreshing, and enforces that engine's post-condition: a refresh
// that returned success left a decodable report at reportPath. It returns
// (report, 0) on success and (nil, 2) once it has named the failure — an
// operational one either way, since the engine, not the operator, broke its
// contract.
//
// It is a function rather than two inline branches because nothing executes
// between align's atomic write and this read, which makes both failures
// unreachable through runPrepare by construction and therefore untestable
// there. They are still real post-conditions of a shared engine this file
// does not own — silently treating a vanished report as "no findings" would
// walk straight into the gate — so the check lives where a test can drive it
// (TestReloadRefreshedReport).
func reloadRefreshedReport(reportPath string, stderr io.Writer) (*artifact.DeviationFrontmatter, int) {
	report, _, err := loadExistingReport(reportPath)
	if err != nil {
		fmt.Fprintln(stderr, "close: --prepare:", err)
		return nil, 2
	}
	if report == nil {
		fmt.Fprintf(stderr, "close: --prepare: align returned success but %s is absent\n", reportPath)
		return nil, 2
	}
	return report, 0
}

// printJudgmentWork renders one undispositioned finding as the work it
// actually is: an exact `verdi disposition` command for a genuine finding, and
// a plain diagnosis for align's synthetic judged-coverage-absent stand-in,
// which is not human work at all.
func printJudgmentWork(stdout io.Writer, finding artifact.Finding, specRef artifact.Ref, storyArg string) {
	if finding.ID == align.AbsenceFindingID {
		printJudgeAbsence(stdout, finding, storyArg)
		return
	}
	fmt.Fprintf(
		stdout,
		"verdi disposition --rationale %s -- %s %s %s\n",
		shellQuoteWord("<human-authored rationale>"),
		shellQuoteWord(specRef.String()),
		shellQuoteWord(finding.ID),
		shellQuoteWord("<human-authored-disposition:fixed|accepted-deviation>"),
	)
}

// printJudgeAbsence names align's synthetic judged-coverage-absent finding as
// the MACHINE failure it is, in place of the disposition template preparation
// prints for real findings.
//
// A judge that crashed or was never configured does not fail the run: RunJudged
// degrades to this one synthetic finding (judged.go's absent-result contract),
// which preparation writes into the living report and used to present as
// ordinary JUDGMENT REQUIRED work, complete with a copy-paste template offering
// `fixed | accepted-deviation`. Once dispositioned, closure gate condition 4
// passes and close's freeze-in-place branch stamps the judge failure into the
// archive verbatim without ever re-running the judge — the exact harm
// prepareAlignDeps' own doc comment describes. Its Wait closes only the TIMEOUT
// shape; crash and not-configured reach here, and stderr says nothing, so the
// synthetic finding's own text is the only place the failure detail exists.
//
// Withholding the template is the whole fix, and it withholds only the PASTE.
// Accepting absent judged coverage remains a legal, governed human ruling (03
// §Alignment report: "skipping the judge is never free, always visible to the
// reviewer, and countable in audit") — it simply has to be authored
// deliberately rather than pasted from a line the tool suggested. Nothing about
// the verdict, the state, or the exit class moves: this is still JUDGMENT
// REQUIRED, still exit 1, and preparation still chooses no disposition.
func printJudgeAbsence(stdout io.Writer, finding artifact.Finding, storyArg string) {
	fmt.Fprintf(stdout, "close: --prepare: %s is NOT a semantic reading a human can judge — it is align's synthetic stand-in for a judge that produced none: %q\n", finding.ID, finding.Text)
	// vocab:identity — CLI invocation grammar ("verdi close --prepare", identity)
	fmt.Fprintf(stdout, "close: --prepare: no disposition template is printed for it. Dispositioning it satisfies the closure gate's disposition condition, and close then freezes this report in place — stamping the judge failure into the archive verbatim, without ever re-running the judge. Repair the judge (align.judge_cmd in verdi.yaml) and re-run verdi close --prepare %s; accepting absent judged coverage instead is a deliberate human ruling to author by hand.\n", shellQuoteWord(storyArg))
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
	// Every disclosure preparation itself makes, so the readiness summary
	// runPreflight prints below can count them: a run that disclosed must never
	// reach the design's bare READY.
	prelude := preflightPrelude{}

	// Rehearsed FIRST, before preparation resolves anything and before its one
	// destructive write — the real ritual's own order (requireCleanIndex is
	// runClose's first act, close.go).
	//
	// Preparation cannot delegate this to runPreflight the way it delegates the
	// gate: it reaches preflight only after refreshing a stale report and only
	// when nothing is undispositioned, so in BOTH of its own stopping states an
	// operator with a dirty index was never told the real close refuses before
	// it evaluates a single condition. Rehearsing here also makes the canonical
	// interrupted close (archive move done, commit failed) diagnosable through
	// --prepare, whose resolve otherwise fails first on a spec that is already
	// out of the active zone.
	//
	// It stays a DISCLOSURE and changes no verdict: preparation writes only the
	// living deviation report, which needs no clean index and clears none.
	indexGuardDisclosures := disclosePreflightIndexGuard(ctx, root, stdout)
	prelude.Disclosures += indexGuardDisclosures
	prelude.IndexGuardRehearsed = true

	spec, err := storyresolve.Resolve(root, storyArg)
	if err != nil {
		fmt.Fprintln(stderr, "close: --prepare:", err)
		return 2
	}
	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		// Unreachable by construction, and deliberately kept: storyresolve
		// only ever returns a spec decoded through artifact.DecodeSpec, whose
		// validateBase has already run ParseRef over this exact id. No
		// hermetic test can reach this branch (there is no seam between the
		// two calls to inject through) — it guards against that decode
		// contract changing out from under this file, exactly as
		// runAlignForSpec's identical guard does.
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
		prelude.Disclosures += discloseRegeneratedDispositions(report, specRef, head, stdout)
		if rc := runAlignForSpec(ctx, root, spec, head, false, prepareAlignDeps(deps, modelDigest, storyArg), stdout, stderr); rc != 0 {
			return rc
		}
		fmt.Fprintf(stdout, "close: --prepare: ALIGNMENT REQUIRED (living report was %s for HEAD %s; the existing align engine refreshed it)\n", freshness, head)

		refreshed, rc := reloadRefreshedReport(reportPath, stderr)
		if rc != 0 {
			return rc
		}
		report = refreshed
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
			printJudgmentWork(stdout, finding, specRef, storyArg)
		}
		return 1
	}

	rc := runPreflightWithPrelude(ctx, root, storyArg, manifest, deps.Model, deps.Forge, forceLocal, prelude, stdout, stderr)
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
	// A run that rehearsed the index guard and got a refusal has just proved
	// that the command below exits 2, so calling it NEXT with nothing attached
	// is the same three-valued dishonesty this file exists to remove.
	//
	// The command is KEPT and qualified rather than suppressed. The exit class
	// stays 0 and belongs there: the closure gate holds, and a dirty index is
	// operator state, not a verdict about the spec — which is exactly why the
	// rehearsal is a disclosure and not a condition. Deleting the echo would
	// smuggle that verdict change in through presentation, since an exit-0 run
	// naming no next step reads as "nothing to do". Nor is the command wrong:
	// after a `git commit` or `git restore` it succeeds unchanged. What was
	// dishonest was the word NEXT while something must happen first — so this
	// names the ordering instead of hiding the command, and the design's own
	// READY rows keep both halves they ask for (the real close command, and
	// what the human must weigh before running it).
	if indexGuardDisclosures > 0 {
		// vocab:identity — CLI invocation grammar ("verdi close", identity)
		fmt.Fprintln(stdout, `close: --prepare: the closure gate holds, but the index-guard rehearsal above refuses the command below: a real "verdi close" exits 2 there, before it evaluates any closure condition. Clear the index first (the rehearsal names how), then run it.`)
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
