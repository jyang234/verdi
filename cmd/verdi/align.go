// verdi align [--freeze] [--wait[=seconds]] (05 §CLI, PLAN.md Phase 8):
// generates/refreshes deviation-report.md for the current build branch's
// spec, inferred from the checked-out branch name (feature/<name>, cut by
// `verdi feature start` — see internal/storyresolve.ResolveBuildSpec; align
// takes no story/spec argument, matching 05 §CLI's table). --freeze
// produces the closure edition (a Frozen stamp at the build head).
//
// spec/judge-ergonomics (L-N1, X-8): every judge-backed run of this verb
// prints the report path as stdout's FIRST line before the judge subprocess
// ever runs (ac-1, runAlignForSpec below) and writes the report through the
// internal/atomicfile seam at completion, so a reader polling that path
// observes either nothing yet or the finished report, never a partial one.
// --wait[=seconds] (ac-2) bounds how long this run waits on the judge: a
// bare --wait reuses the already-resolved JudgeTimeout (manifest-configured
// or internal/align's own default) as ac-2's "sane default" bound, rather
// than inventing a second, possibly-conflicting timeout knob; --wait=N sets
// that bound to N seconds outright. Either form makes a judge that does not
// complete within the bound exit 2 (an operational timeout, never a
// verdict) instead of today's default graceful degrade to a synthetic
// absence finding — the path is already on stdout by the time this can
// happen. The contract lives once in runAlignForSpec (ac-3), the exact
// function close.go's freeze-align calls, so close inherits it without a
// second implementation; --wait itself is out of scope for the
// design-branch decision-conflict mode and --diagram-sweep (rejected
// explicitly, never silently ignored).
//
// verdi align --diagram-sweep <diagram-ref> (spec/judged-sweep ac-1, dc-1)
// is a THIRD, wholly on-demand mode: unlike the build-branch and
// design-branch modes, it takes its own diagram-ref argument, infers
// nothing from the checked-out branch, is never required by any gate, and
// writes a disposable sibling report — see aligndiagramsweep.go.
//
// Exit contract (CLAUDE.md 0/1/2, PLAN.md Phase 8's exit criteria):
// 0 report written; 1 align.judge_required is true and no judge produced a
// judged section; 2 every other operational failure, including a --wait
// expiry and bad usage.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
	"github.com/jyang234/verdi/internal/upstream"
)

// alignDeps is cmdAlign's injectable dependency set — the same seam
// feature.go's syncDeps establishes, so tests can supply an
// upstream.FakeRunner and a fake judge command instead of the real,
// network-needing toolchain/judge (CLAUDE.md: "no network in any test").
type alignDeps struct {
	Runner        upstream.Runner
	JudgeCmd      []string
	JudgeRequired bool
	// JudgeTimeout mirrors verdi.yaml's align.judge_timeout_seconds (D6-21);
	// zero leaves internal/align's own DefaultJudgeTimeout fallback
	// unchanged (align.Input.JudgeTimeout's zero-value contract).
	JudgeTimeout time.Duration
	// Wait mirrors align.Input.Wait (spec/judge-ergonomics ac-2): true when
	// --wait was passed (cmdAlign) or a caller (close.go, in a future story)
	// opts a freeze-align call into bounded-wait semantics. cmdAlign folds
	// --wait=N's explicit bound into JudgeTimeout above BEFORE calling
	// runAlign, so this field alone is enough for runAlignForSpec to thread
	// both pieces of ac-2's contract through to align.Input.
	Wait bool
	// ModelDigest is the resolved operating model's canonical-JSON sha256
	// digest (model.Model.Digest(), spec/model-digest ledger L-M5),
	// resolved once in cmdAlign via store.Open and threaded into every
	// align.Input/DecisionConflictInput/DiagramSweepInput this package
	// builds — the report.go/decision_report.go/diagram_report.go mint
	// sites never re-derive it themselves.
	ModelDigest string
}

// cmdAlign is `verdi align`'s entry point, invoked by dispatch.go: resolves
// the store root and manifest, wires the real upstream.Runner and
// verdi.yaml's align: block, then delegates to runAlign.
func cmdAlign(args []string, stdout, stderr io.Writer) int {
	freeze := false
	wait := false
	var waitBound time.Duration
	diagramRef := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--wait" {
			wait = true
			continue
		}
		if secStr, ok := strings.CutPrefix(a, "--wait="); ok {
			secs, err := strconv.Atoi(secStr)
			if err != nil || secs <= 0 {
				fmt.Fprintf(stderr, "align: --wait=%s: must be a positive whole number of seconds\n", secStr)
				return 2
			}
			wait = true
			waitBound = time.Duration(secs) * time.Second
			continue
		}
		switch a {
		case "--freeze":
			freeze = true
		case "--diagram-sweep":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "align: --diagram-sweep requires a <diagram-ref> argument")
				return 2
			}
			i++
			diagramRef = args[i]
		default:
			fmt.Fprintf(stderr, "align: unexpected argument %q; usage: verdi align [--freeze] [--wait[=seconds]] | verdi align --diagram-sweep <diagram-ref>\n", a)
			return 2
		}
	}
	if wait && diagramRef != "" {
		fmt.Fprintln(stderr, "align: --wait and --diagram-sweep are mutually exclusive (the sweep mode is advisory/non-exhaustive and out of scope for spec/judge-ergonomics)")
		return 2
	}

	ctx := context.Background()
	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}
	cfg, err := store.Open(root)
	if err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}
	manifest := cfg.Manifest
	modelDigest, err := cfg.Model.Digest()
	if err != nil {
		fmt.Fprintln(stderr, "align: computing model digest:", err)
		return 2
	}

	deps := alignDeps{ModelDigest: modelDigest}
	if manifest.Toolchain != nil {
		deps.Runner = upstream.RealRunner{Module: manifest.Toolchain.Module, Commit: manifest.Toolchain.Commit, Dir: root}
	}
	if manifest.Align != nil {
		deps.JudgeCmd = manifest.Align.JudgeCmd
		deps.JudgeRequired = manifest.Align.JudgeRequired
		if manifest.Align.JudgeTimeoutSeconds > 0 {
			deps.JudgeTimeout = time.Duration(manifest.Align.JudgeTimeoutSeconds) * time.Second
		}
	}
	if wait {
		deps.Wait = true
		if waitBound > 0 {
			deps.JudgeTimeout = waitBound
		}
		// A bare --wait (waitBound == 0) deliberately leaves JudgeTimeout as
		// whatever was just resolved above — manifest's
		// align.judge_timeout_seconds, or internal/align's own
		// DefaultJudgeTimeout if unconfigured — as ac-2's "sane default"
		// bound: the operator's own already-established judge ceiling,
		// rather than a second, possibly-conflicting timeout invented here.
	}

	// Diagram-sweep mode (spec/judged-sweep ac-1, dc-1): a THIRD, wholly
	// on-demand mode of this verb, dispatched before branch resolution since
	// it takes its own <diagram-ref> argument and never infers anything from
	// the checked-out branch — see aligndiagramsweep.go.
	if diagramRef != "" {
		if freeze {
			fmt.Fprintln(stderr, "align: --freeze and --diagram-sweep are mutually exclusive (a sweep report is never frozen, spec/judged-sweep dc-3)")
			return 2
		}
		return runDiagramSweepAlign(ctx, root, diagramRef, deps, stdout, stderr)
	}

	return runAlign(ctx, root, freeze, deps, stdout, stderr)
}

// runAlign is the testable core: given an already-resolved root and
// injected deps, resolve the build-head spec (from the current branch,
// feature/<name> — align takes no story/spec argument), run
// internal/align.Generate, and write deviation-report.md into the spec's
// directory.
func runAlign(ctx context.Context, root string, freeze bool, deps alignDeps, stdout, stderr io.Writer) int {
	branch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}
	// Design-branch mode (03 §Decision-conflict gate; 05 §CLI: "on a
	// design branch, grows a decision-conflict-report mode") — the two
	// modes share this one command; see align_design.go.
	if strings.HasPrefix(branch, "design/") {
		if deps.Wait {
			fmt.Fprintln(stderr, "align: --wait is not supported on a design branch (decision-conflict mode is out of scope for spec/judge-ergonomics); re-run without --wait")
			return 2
		}
		return runDesignAlign(ctx, root, freeze, deps, stdout, stderr)
	}
	spec, err := storyresolve.ResolveBuildSpec(root, branch)
	if err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}
	covers, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}
	return runAlignForSpec(ctx, root, spec, covers, freeze, deps, stdout, stderr)
}

// runAlignForSpec is runAlign's spec-taking core, factored out (round 6,
// spec/close-verb ac-1) so a caller that has ALREADY resolved its own spec
// by a means other than the feature/<name> build-branch convention —
// `verdi close` resolves the story via internal/storyresolve.Resolve, a
// story or spec-ref argument, never a branch name — can run the exact same
// generate-freeze-write logic runAlign uses for the frozen closure report,
// rather than duplicating it (CLAUDE.md: no copy-paste across call sites).
// runAlign itself is unchanged in behavior: it still resolves branch ->
// spec -> covers first, then delegates here.
func runAlignForSpec(ctx context.Context, root string, spec *artifact.SpecFrontmatter, covers string, freeze bool, deps alignDeps, stdout, stderr io.Writer) int {
	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		fmt.Fprintln(stderr, "align: internal error: resolved spec has an invalid id:", err)
		return 2
	}
	reportPath := store.DeviationReportPath(root, store.ZoneActive, specRef.Name)

	// spec/judge-ergonomics ac-1: the report path is stdout's FIRST line,
	// printed before anything else below runs — in particular, before
	// align.Generate ever invokes the judge subprocess (whichever branch
	// below is ultimately taken: freeze-in-place, regenerate, or an early
	// refusal). A caller — human or agent — always has a filesystem
	// location to watch without parsing anything else this verb prints.
	fmt.Fprintln(stdout, reportPath)

	existingReport, existingBody, err := loadExistingReport(reportPath)
	if err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}
	if existingReport != nil && existingReport.Frozen != nil {
		fmt.Fprintf(stderr, "align: %s is already frozen (at %s, commit %s); a frozen alignment report is immutable\n", reportPath, existingReport.Frozen.At, existingReport.Frozen.Commit)
		return 1
	}
	var existingFindings []artifact.Finding
	if existingReport != nil {
		existingFindings = existingReport.Findings
	}

	in := align.Input{
		Root:             root,
		Runner:           deps.Runner,
		Spec:             spec,
		Covers:           covers,
		JudgeCmd:         deps.JudgeCmd,
		JudgeRequired:    deps.JudgeRequired,
		JudgeTimeout:     deps.JudgeTimeout,
		ExistingFindings: existingFindings,
		ModelDigest:      deps.ModelDigest,
		Wait:             deps.Wait,
	}
	if freeze {
		frozenAt, err := gitx.CommitDateOnly(ctx, root, covers)
		if err != nil {
			fmt.Fprintln(stderr, "align:", err)
			return 2
		}

		// Freeze-in-place — the faithful freeze `verdi close` needs. When a
		// living report already covers this exact freeze commit and every
		// finding is dispositioned (the fresh, fully-dispositioned state the
		// merge gate required before merge, 03 §Gates condition 3), stamp it
		// VERBATIM (align.FreezeInPlace) rather than regenerating. Regenerating
		// re-runs the non-reproducible judge (03 §Alignment report), whose fresh
		// content-hash finding identities PreserveDispositions cannot match —
		// silently erasing every human disposition (the D6-21-exposed bug).
		// Any other state — no living report, stale covers, or an
		// undispositioned finding — falls through to the regenerate path below,
		// unchanged.
		if existingReport != nil && existingReport.Covers == covers && artifact.AllDispositioned(existingReport.Findings) {
			report, err := align.FreezeInPlace(existingReport, string(existingBody), frozenAt)
			if err != nil {
				fmt.Fprintln(stderr, "align:", err)
				return 2
			}
			// spec/judge-ergonomics ac-1: the atomicfile seam (temp-then-
			// rename), not a raw os.WriteFile — a reader polling reportPath
			// observes either the prior content or the complete new content,
			// never a partial write.
			if err := atomicfile.Write(reportPath, report.Markdown, 0o644); err != nil {
				fmt.Fprintln(stderr, "align:", err)
				return 2
			}
			fmt.Fprintf(stdout, "align: froze %s in place (covers %s, %d findings, dispositions preserved)\n", reportPath, report.Frontmatter.Covers, len(report.Frontmatter.Findings))
			fmt.Fprintf(stdout, "align: frozen at %s\n", report.Frontmatter.Frozen.At)
			return 0
		}

		in.Freeze = true
		in.FrozenAt = frozenAt
	}

	report, err := align.Generate(ctx, in)
	if err != nil {
		var reqAbsent *align.ErrJudgeRequiredAbsent
		if errors.As(err, &reqAbsent) {
			fmt.Fprintln(stderr, "align:", err)
			return 1
		}
		// spec/judge-ergonomics ac-2: an operational timeout, not a
		// verdict — exit 2, never exit 1's ErrJudgeRequiredAbsent path.
		// Nothing is written here: Generate returned no Report, so
		// reportPath (already on stdout's first line above) still names
		// whatever was there before this run — nothing genuine to lose.
		var waitExpired *align.ErrJudgeWaitExpired
		if errors.As(err, &waitExpired) {
			fmt.Fprintln(stderr, "align:", err)
			fmt.Fprintf(stderr, "align: no report was written this run; %s is unchanged — re-run, optionally with a longer --wait, or check it later\n", reportPath)
			return 2
		}
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}

	// D6-24: never let a regeneration whose judge failed to produce a
	// genuine result replace a report that already carries one on disk. See
	// keepGenuineOnJudgeFailure's doc comment for the full rule.
	var existingJudgeIntegrity *artifact.JudgeIntegrity
	if existingReport != nil {
		existingJudgeIntegrity = existingReport.JudgeIntegrity
	}
	if keepGenuineOnJudgeFailure(existingJudgeIntegrity, report.Frontmatter.JudgeIntegrity) {
		fmt.Fprintf(stderr, "align: %s\n", absenceFindingText(report.Frontmatter.Findings, align.AbsenceFindingID))
		fmt.Fprintf(stderr, "align: %s already carries a genuine judged exchange from a completed judge run; PRESERVED byte-for-byte rather than overwritten with this run's synthetic judge-failure edition (D6-24)\n", reportPath)
		return 2
	}

	// spec/judge-ergonomics ac-1: atomicfile.Write, not os.WriteFile — see
	// the freeze-in-place write above for the identical rationale.
	if err := atomicfile.Write(reportPath, report.Markdown, 0o644); err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}

	fmt.Fprintf(stdout, "align: wrote %s (covers %s, %d findings)\n", reportPath, report.Frontmatter.Covers, len(report.Frontmatter.Findings))
	if freeze {
		fmt.Fprintf(stdout, "align: frozen at %s\n", report.Frontmatter.Frozen.At)
	}
	return 0
}

// keepGenuineOnJudgeFailure implements D6-24's fix: an align regeneration
// must never replace a living report that carries a genuine judged
// exchange (judge_integrity present, from a completed judge run) with a
// synthetic judge-failure edition. Witnessed in round 6: a re-run whose
// judge timed out overwrote a living report carrying a genuine judge
// exchange (2 real findings + dispositions) with a synthetic
// judged-coverage-absent finding, destroying both.
//
// existingJudgeIntegrity is the prior on-disk report's JudgeIntegrity — nil
// for "no prior report" or a prior report whose own judged section was
// ALREADY synthetic; both cases have nothing genuine to lose, so today's
// plain regenerate-and-overwrite behavior is correct and deliberately left
// unprotected. newJudgeIntegrity is THIS run's freshly regenerated
// JudgeIntegrity — nil exactly when this run's judge failed, timed out, or
// was never configured (RunJudged's — judged.go — and RunDecisionSweep's —
// decision_judge.go — shared absent-result contract: every non-required
// failure degrades to exactly one synthetic absence finding and no
// JudgeIntegrity). A judge run that completes genuinely (both non-nil) is
// ordinary regeneration and is unaffected by this rule — genuine-to-genuine
// replacement, including its own known finding-identity drift, is
// explicitly out of scope for D6-24's fix (its own second half);
// PreserveDispositions/PreserveConflictDispositions (identity.go) are
// untouched.
//
// Shared by align.go's build-branch runAlignForSpec and align_design.go's
// design-branch runDesignAlign — the two callers write different report
// schemas (DeviationFrontmatter/DecisionConflictFrontmatter) but apply
// exactly this one yes/no rule, so it lives once here rather than being
// duplicated per mode (CLAUDE.md: no copy-paste across call sites).
func keepGenuineOnJudgeFailure(existingJudgeIntegrity, newJudgeIntegrity *artifact.JudgeIntegrity) bool {
	return existingJudgeIntegrity != nil && newJudgeIntegrity == nil
}

// absenceFindingText returns the synthetic absence finding's own disclosed
// text (the judge failure's stage/exit/stderr detail — judged.go's
// absenceFinding) so the keep-genuine disclosure (D6-24) can show the
// operator exactly what the judge reported, without internal/align needing
// to expose a second, parallel failure-detail API alongside Report. Every
// call site only reaches here when newJudgeIntegrity is nil, which
// RunJudged's absent-result contract guarantees means exactly one finding
// with this id is present in findings; the fallback string only guards
// against that contract changing out from under this call site unnoticed.
func absenceFindingText(findings []artifact.Finding, id string) string {
	for _, f := range findings {
		if f.ID == id {
			return f.Text
		}
	}
	return "align: internal warning: expected a synthetic judge-absence finding but found none"
}

// loadExistingReport reads and strict-decodes a prior deviation-report.md, if
// one exists — (nil, nil, nil) for a first run (no file yet). It returns the
// decoded frontmatter AND the raw body bytes: the freeze-in-place path
// (align.FreezeInPlace) reattaches the body verbatim, so a faithful freeze must
// keep it byte-for-byte rather than re-render it. A file that exists but fails
// to decode is a real, surfaced error (CLAUDE.md: "silence is never a pass" — a
// broken report on disk must never be treated as "no report").
func loadExistingReport(path string) (*artifact.DeviationFrontmatter, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("reading existing %s: %w", path, err)
	}
	fm, body, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	decoded, err := artifact.DecodeDeviation(fm)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	return decoded, body, nil
}
