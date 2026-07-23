// verdi close <jira:STORY-KEY | spec/name> [--force-local] (05 §CLI;
// 03 §Closure ritual; spec/close-verb ac-1, ac-2, ac-3, dc-3): drives a
// merged verdi STORY to a true, archived closure on authoritative
// (source: ci) evidence alone, then publishes its rollup to the configured
// tracker. Flips `close` from I-23's phase-0 stub ("not implemented (out
// of v0 scope)", exit 2) to a real handler (dispatch.go).
//
// Feature closure (03's other half: every feature AC evidenced including
// the outcome floor, plus stub reconciliation passed, plus every
// implementing story closed) was OUT OF spec/close-verb's SCOPE — a clear,
// honest "not yet" for a feature-class target rather than a silent no-op
// or a lie about the verb surface (I-23's own precedent) — and is now
// completed in closefeature.go/closuregatefeature.go (runClose below
// delegates to runCloseFeature the moment the resolved spec is
// class: feature).
//
// The story ritual, in order:
//  1. Resolve the story (internal/storyresolve.Resolve — a scheme-prefixed
//     story ref or a spec/<name> ref, I-30's strict two-form contract).
//  2. Evaluate the closure gate (runClosureGate, closuregate.go, CONSUMED
//     UNCHANGED): eligible (every AC evidenced/waived, folding ONLY
//     source: ci records — internal/evidence.Fold's existing authoritative
//     filter, co-1), no unresolved spec-stale flag, no unresolved
//     pending-supersession flag. A forge is best-effort (buildForgeBestEffort,
//     gate_threads.go): unavailable degrades to a disclosed-unproven
//     pending-supersession condition, never a silent pass, exactly as
//     closuregate_test.go already proves for `verdi gate`.
//  3. Only once the gate holds: cut a closure branch (close/<name>),
//     freeze the alignment report in place (runAlignForSpec, align.go,
//     CONSUMED via extraction — same Generate/write logic `verdi align
//     --freeze` uses, without depending on the feature/<name> build-branch
//     naming convention), build and canonjson-digest rollup.json, and move
//     the whole target spec directory to specs/archive/<name>/
//     (store.ArchiveMove). Every archive contains spec.md, the frozen
//     deviation-report.md, and rollup.json; a grandfathered board.json
//     moves with the directory only when already present.
//  4. Commit only the target spec's active-zone deletion and archive-zone
//     tree on the closure branch. A pre-existing staged path is refused
//     before any mutation; unrelated unstaged and untracked work survives
//     outside this commit.
//  5. Publish the rollup to the configured tracker (ac-2) — the round-6
//     hermetic fake provider by default (spec/close-verb dc-2), a real
//     Jira adapter by a pure config change.
//  6. Print the push/open-MR instruction. dc-3: no CreateMR is added to the
//     forge port — the phase-7 precedent that verbs stop at the branch;
//     opening the MR is the human's (or `glab`/`gh`'s) act.
//
// PublishRollup "runs in CI only" (04 §Semantics) is enforced the same way
// `rollup --publish` enforces it (I-32): cmdClose refuses outside a
// detected CI environment unless --force-local overrides it, printing the
// same disclosed, non-authoritative warning — 03's "Author ... runs verdi
// close" is satisfied either by a human running it inside a manually
// triggered CI job, or locally with --force-local for testing only.
//
// --preflight (spec/close-preflight; closure-ergonomics dc-5/ADJ-23) is a
// mode-selecting switch on this same verb, not a new one: it rehearses
// steps 1-2 above (resolve, evaluate the closure gate) through the
// IDENTICAL runClosureGate/runFeatureClosureGate functions and stops
// there, dispatched in cmdClose BEFORE the CI-only/--force-local guard
// below — that guard exists solely to protect step 5's publish call,
// which --preflight never reaches. See closepreflight.go/
// closepreflightfeature.go for the full implementation.
//
// --prepare (closure-session ergonomics, ledger L-N15(1)) is the resumable
// operator entry point for both classes — a MODE of this verb, never a new
// one, on close-preflight dc-1's ruling (a bare mode-selecting switch; no
// 05 §CLI inventory change), and dispatched before the CI-only publish
// guard on that same reasoning since it never reaches a publish. Given an
// explicit ref, it refreshes an absent or
// stale living alignment report, then stops at every undispositioned finding
// for human judgment. Re-running it at the same HEAD preserves that report
// byte-for-byte instead of regenerating it. Once every finding is
// dispositioned, it enters the identical preflight path above and reports
// MECHANICAL WORK REQUIRED, READY WITH DISCLOSURES, or READY. Preparation
// never freezes, archives, commits, publishes, or chooses a disposition.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/provider"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
	"github.com/jyang234/verdi/internal/upstream"
)

// closeDeps bundles close's injectable dependencies (mirroring rollupDeps/
// syncDeps/designDeps) so runClose can be driven hermetically in tests
// (CLAUDE.md: no network, no exec in any test); cmdClose wires the real
// ones. Forge may be nil (no forge configured/reachable) — runClosureGate
// already handles that via disclosure, never a silent pass.
type closeDeps struct {
	Runner        upstream.Runner
	JudgeCmd      []string
	JudgeRequired bool
	// JudgeTimeout mirrors verdi.yaml's align.judge_timeout_seconds (D6-21);
	// threaded through to the freeze-time runAlignForSpec call below exactly
	// like align.go's own alignDeps.JudgeTimeout.
	JudgeTimeout time.Duration
	Forge        forge.Forge
	Registry     provider.StoryProvider
	// Model is the store's resolved operating model (store.Open's config
	// bottleneck) — display vocabulary for the gate lines and ritual prose
	// this verb prints (L-M13(1)). nil (every pre-existing test literal)
	// falls back to bare ids.
	Model *model.Model
}

// closeAddPaths and closeCreateCommit are the closure ritual's two post-
// archive-move git write ops as package-level seams, so a test can force the
// exact AddPaths/CreateCommit failure each ritual's recovery path must
// survive. This mirrors `verdi accept`'s already-proven pattern verbatim
// (accept.go's acceptAddPaths/acceptCreateCommit, spec/obligation-seam ac-3)
// rather than inventing a second injection style: a real `git add`/`git
// commit` cannot be made to fail deterministically in a clean hermetic fixture
// repo, and closeDeps is reserved for real runtime dependencies (a runner, a
// forge, a provider registry), never for pure fault injection. Production is
// gitx's own; tests override and restore via defer.
var (
	closeAddPaths     = gitx.AddPaths
	closeCreateCommit = gitx.CreateCommit
)

// closeExpiryResumeHint is close's freeze-align resume guidance for a
// bounded-wait expiry (finding
// judged-close-inherits-aligns-resume-instructions-verbatim): a close caller
// exposes no --wait flag and cannot resume by re-running align — the close
// aborted at exit 2 and only re-running close completes the freeze and
// archive. So close's ResumeHint names `verdi close`, in no flag language,
// rather than inheriting align's own alignExpiryResumeHint ("re-run align …
// optionally with a longer --wait") verbatim.
// vocab:identity — CLI invocation grammar ("verdi close", identity)
const closeExpiryResumeHint = "Re-run verdi close once the judge window allows to complete the freeze and archive"

// freezeAlignDeps builds the alignDeps for close's freeze-align step — the
// single construction both runClose (story, this file) and runCloseFeature
// (feature, closefeature.go) call, so the two can never drift (CLAUDE.md: no
// copy-paste across call sites; the two literals were byte-identical). It
// carries close's judge configuration (from closeDeps, cmdClose's manifest
// resolution) plus the once-resolved model digest each caller passes.
//
// Wait is set (spec/judge-ergonomics ac-3, finding
// judged-close-cannot-reach-inherited-wait): close's internal freeze-align
// inherits align's bounded-wait contract from the same runAlignForSpec hook
// `verdi align --wait` uses, rather than the contract being latent-only for
// close. The bound is the judge's own configured ceiling
// (deps.JudgeTimeout — duration identical to today), and a judge that does
// not complete within it surfaces the honest exit-2-with-report-path expiry
// instead of hanging past a caller's patience or degrading into a synthetic
// judge-absence finding frozen straight into the archive. This is the
// "future story" alignDeps.Wait's own comment deferred close's opt-in to;
// this is that story. Every non-timeout judge failure is unchanged — it
// still degrades and is still caught by D6-24's preserve-don't-clobber rule
// (keepGenuineOnJudgeFailure, align.go); only the TIMEOUT shape changes.
//
// ResumeHint is set to closeExpiryResumeHint so that, on a --wait expiry, the
// shared align engine's guidance speaks close's verb rather than align's flag
// language inherited verbatim (finding
// judged-close-inherits-aligns-resume-instructions-verbatim).
func freezeAlignDeps(deps closeDeps, modelDigest string) alignDeps {
	return alignDeps{
		Runner:        deps.Runner,
		JudgeCmd:      deps.JudgeCmd,
		JudgeRequired: deps.JudgeRequired,
		JudgeTimeout:  deps.JudgeTimeout,
		ModelDigest:   modelDigest,
		Wait:          true,
		ResumeHint:    closeExpiryResumeHint,
	}
}

// unwindClosureBranchCut reverses close's just-made close/<name> branch cut on
// a freeze-align failure, so the resume hint's promised retry (closeExpiryResumeHint,
// "Re-run verdi close …") is real rather than blocked by the verb's own
// residue (finding judged-close-resume-hint-names-a-path-close-itself-refuses).
// Both runClose (this file) and runCloseFeature (closefeature.go) cut
// close/<name> at cutPoint BEFORE the shared freeze step that can fail, and
// gitx.CheckoutNewBranch deliberately refuses a name that already exists
// (branch.go's no-clobber posture) — so a freeze failure that left the branch
// behind made the very next `verdi close` abort at the cut, exactly the path
// the hint promised. This is the single implementation both callers use (the
// freezeAlignDeps precedent — no per-verb reimplementation to drift).
//
// It is called on the two post-cut failure paths that committed and staged
// NOTHING: the freeze-setup / freeze-align failure (where the freeze wrote
// nothing — every runAlignForSpec non-zero return leaves the report on disk
// untouched) and the staging failure (where `git add` recorded no index entry).
// close/<name> therefore still points exactly at cutPoint, and it returns to
// originalBranch (or, for a close run from a detached HEAD, the cut commit
// itself) via the board-guard-free gitx.CheckoutExisting, since the target is
// that same commit and nothing is lost. It is deliberately NOT called on the
// commit failure, where the closure paths ARE staged and deleting the branch
// would strand that index (reportStagedClosureCommitFailure).
//
// It NEVER discards committed work: it deletes only after proving close/<name>
// still points at cutPoint (no commit beyond the cut). If anything was somehow
// committed there, or the switch-back/delete cannot be completed, it leaves the
// branch in place and says so on stderr rather than force-removing it — the
// caller's exit code is already the freeze failure's, and this is best-effort
// cleanup whose every giving-up branch is disclosed (constitution 2/10: silence
// is never a pass), never silent.
func unwindClosureBranchCut(ctx context.Context, root, originalBranch, closureBranch, cutPoint string, stderr io.Writer) {
	tip, err := gitx.RevParse(ctx, root, closureBranch)
	if err != nil {
		fmt.Fprintf(stderr, "close: left %s in place: could not inspect it to unwind the branch cut (%v); switch back and delete it before retrying\n", closureBranch, err)
		return
	}
	if tip != cutPoint {
		fmt.Fprintf(stderr, "close: left %s in place: it carries commit(s) beyond its cut point %s and nothing was discarded; remove it manually if unneeded before retrying\n", closureBranch, cutPoint)
		return
	}
	restore := originalBranch
	if restore == "" {
		// close ran from a detached HEAD (CurrentBranch is "" there, not an
		// error): return to the cut commit itself rather than a branch name.
		restore = cutPoint
	}
	if err := gitx.CheckoutExisting(ctx, root, restore); err != nil {
		fmt.Fprintf(stderr, "close: left %s in place: could not switch back to %s to unwind the branch cut (%v); remove it manually before retrying\n", closureBranch, restore, err)
		return
	}
	if err := gitx.DeleteBranch(ctx, root, closureBranch); err != nil {
		fmt.Fprintf(stderr, "close: switched back to %s but left %s in place: %v; remove it manually before retrying\n", restore, closureBranch, err)
		return
	}
}

// cmdClose is `verdi close`'s entry point, invoked by dispatch.go.
func cmdClose(args []string, stdout, stderr io.Writer) int {
	forceLocal := false
	preflight := false
	prepare := false
	var storyArg string
	for _, a := range args {
		switch a {
		case "--force-local":
			forceLocal = true
			continue
		case "--preflight":
			preflight = true
			continue
		case "--prepare":
			prepare = true
			continue
		}
		if storyArg != "" {
			fmt.Fprintf(stderr, "close: unexpected extra argument %q\n", a)
			return 2
		}
		storyArg = a
	}
	if prepare && preflight {
		fmt.Fprintln(stderr, "close: --prepare and --preflight are mutually exclusive")
		return 2
	}
	if storyArg == "" {
		// vocab:identity — CLI usage/verb-name grammar (identity)
		fmt.Fprintln(stderr, `close: usage: verdi close <jira:STORY-KEY | spec/name> [--force-local]
              verdi close --preflight <jira:STORY-KEY | spec/name> [--force-local]
              verdi close --prepare <jira:STORY-KEY | spec/name> [--force-local]`)
		return 2
	}

	// --preflight is dispatched BEFORE the CI-only/--force-local publish
	// guard below, not conditioned by it (dc-1): that guard exists solely
	// to gate the publish step (04 §Semantics), and --preflight never
	// reaches a publish call at all (ac-2) — subjecting it to the same
	// refusal would make the verb's only side-effect-free, anywhere-
	// runnable mode unusable from a plain local checkout without an
	// unrelated escape hatch.
	if preflight {
		ctx := context.Background()
		root, err := store.FindRoot(".")
		if err != nil {
			fmt.Fprintln(stderr, "close:", err)
			return 2
		}
		// store.Open (not the bare loadManifest delegate): the rehearsed
		// closure-gate lines resolve display vocabulary through
		// Config.Model (L-M13(1)) — one open yields both halves.
		cfg, err := store.Open(root)
		if err != nil {
			fmt.Fprintln(stderr, "close:", err)
			return 2
		}
		return runPreflight(ctx, root, storyArg, cfg.Manifest, cfg.Model, buildForgeBestEffort(ctx, root), forceLocal, stdout, stderr)
	}
	// 04 §Semantics: "PublishRollup runs in CI only" — close calls it
	// directly (ac-2), so the same CI-only discipline `rollup --publish`
	// already enforces (I-32) applies here, mirrored exactly.
	if !prepare {
		inCI := lint.ReadCIEnv().InCI
		if closePublishGuardRefuses(forceLocal) {
			fmt.Fprintln(stderr, "close: refusing to publish outside CI (04 §Semantics: \"PublishRollup runs in CI only\"); pass --force-local to run anyway for local testing only")
			return 2
		}
		if !inCI {
			fmt.Fprintln(stderr, "close: --force-local: running outside CI; this escape hatch exists for local testing only and its publish is NON-AUTHORITATIVE (04 §Semantics: PublishRollup runs in CI only)")
		}
	}

	ctx := context.Background()
	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}
	// store.Open (not the bare loadManifest delegate): close's gate lines
	// and feature-ritual prose resolve display vocabulary through
	// Config.Model (L-M13(1)) — one open yields both halves.
	cfg, err := store.Open(root)
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}
	manifest := cfg.Manifest

	var runner upstream.Runner
	if manifest.Toolchain != nil {
		runner = upstream.RealRunner{Module: manifest.Toolchain.Module, Commit: manifest.Toolchain.Commit, Dir: root}
	}
	var judgeCmd []string
	judgeRequired := false
	var judgeTimeout time.Duration
	if manifest.Align != nil {
		judgeCmd = manifest.Align.JudgeCmd
		judgeRequired = manifest.Align.JudgeRequired
		if manifest.Align.JudgeTimeoutSeconds > 0 {
			judgeTimeout = time.Duration(manifest.Align.JudgeTimeoutSeconds) * time.Second
		}
	}

	deps := closeDeps{
		Runner:        runner,
		JudgeCmd:      judgeCmd,
		JudgeRequired: judgeRequired,
		JudgeTimeout:  judgeTimeout,
		Forge:         buildForgeBestEffort(ctx, root),
		Registry:      buildProviderRegistry(manifest),
		Model:         cfg.Model,
	}
	if prepare {
		return runPrepare(ctx, root, storyArg, manifest, deps, forceLocal, stdout, stderr)
	}
	return runClose(ctx, root, storyArg, manifest, deps, stdout, stderr)
}

// runClose is the testable core: given an already-resolved root, a
// story/spec argument, the decoded manifest, and injected deps, run the
// whole closure ritual and return the exit code (CLAUDE.md: 0 clean,
// 1 the closure gate did not hold, 2 operational error).
func runClose(ctx context.Context, root, storyArg string, manifest *store.Manifest, deps closeDeps, stdout, stderr io.Writer) int {
	if err := requireCleanIndex(ctx, root); err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}

	spec, err := storyresolve.Resolve(root, storyArg)
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}
	if spec.Class == artifact.ClassFeature {
		return runCloseFeature(ctx, root, spec, manifest, deps, stdout, stderr)
	}

	head, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}
	defaultBranchRef := lint.ResolveDefaultBranch(ctx, root)

	// The closure gate (co-1: authoritative evidence only — runClosureGate
	// folds via internal/evidence.Fold with Preview false, exactly as
	// `verdi gate`/`verdi rollup` do; CONSUMED UNCHANGED).
	ok, err := runClosureGate(ctx, root, spec, deps.Forge, defaultBranchRef, manifest, deps.Model, head, stdout)
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}
	if !ok {
		fmt.Fprintln(stdout, "close: FAIL (closure gate not satisfied; see conditions above)")
		return 1
	}

	// Recompute the fold for the rollup payload: the closure gate above
	// already proved eligibility; this call additionally needs the full
	// per-AC breakdown Rollup.Criteria and the publish payload carry.
	fold, err := foldStory(ctx, root, spec, head)
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}

	// Name every attestation/waiver the gate just folded on that HEAD does not
	// carry identically. This changes no verdict — it discloses the one class
	// of fold input that neither the index guard nor the exact staging can
	// account for, because both live outside the closure paths this ritual
	// commits (see closeUncommittedRecordSource). It runs BEFORE the branch
	// cut, so the disclosure survives any later operational failure.
	if err := discloseUncommittedFoldRecords(ctx, root, head, storyFoldRecordPaths(store.RefSlug(spec.Story), fold.ACs), stdout); err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}

	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		fmt.Fprintln(stderr, "close: internal error: resolved spec has an invalid id:", err)
		return 2
	}

	// The branch to return to if the freeze fails after the cut below
	// (finding judged-close-resume-hint-names-a-path-close-itself-refuses).
	// "" for a detached-HEAD close is not an error — unwindClosureBranchCut
	// returns to the cut commit itself in that case.
	originalBranch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}
	closureBranch := "close/" + specRef.Name
	if err := gitx.CheckoutNewBranch(ctx, root, closureBranch); err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}

	// Freeze the alignment report in place, still under specs/active/ (the
	// same generate-freeze-write logic `verdi align --freeze` uses,
	// align.go's runAlignForSpec) — which, on the regenerate fallback path
	// (no living report / stale covers / an undispositioned finding),
	// mints a fresh Provenance and needs a resolved model digest exactly
	// like `verdi align` itself does (spec/model-digest ledger L-M5).
	//
	// Both post-cut, pre-commit failure points below UNWIND the branch cut
	// before exiting (finding
	// judged-close-resume-hint-names-a-path-close-itself-refuses): the freeze
	// wrote nothing, so close/<name> still points at the cut and returning to
	// originalBranch loses nothing — leaving the resume hint's promised
	// `verdi close` retry able to complete rather than dying at the next cut's
	// no-clobber refusal.
	modelDigest, err := resolveModelDigest(root)
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
		unwindClosureBranchCut(ctx, root, originalBranch, closureBranch, head, stderr)
		return 2
	}
	alignD := freezeAlignDeps(deps, modelDigest)
	if rc := runAlignForSpec(ctx, root, spec, head, true, alignD, stdout, stderr); rc != 0 {
		fmt.Fprintln(stderr, "close: freezing the alignment report failed (see above)")
		unwindClosureBranchCut(ctx, root, originalBranch, closureBranch, head, stderr)
		return rc
	}

	if err := writeRollup(root, specRef, spec, head, fold); err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}

	// Flip the spec's status accepted-pending-build → closed as part of the
	// archive step (02 §Kind registry: story/feature specs transition
	// "… → closed(archive)"). Done in the active-zone spec.md BEFORE
	// ArchiveMove renames the whole target spec directory in one shot:
	// spec.md moves with its sole status-line change and every other present
	// file moves byte-identically — VL-010's round-6 status-only archive-flip
	// exception (D6-11), not the pure-rename one, is what admits the move.
	if err := flipSpecStatusToClosed(root, specRef.Name); err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}

	if err := store.ArchiveMove(root, specRef.Name); err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}

	// Staging and committing are the ritual's last two failure points, and
	// each leaves a different state (see the two report helpers): a staging
	// failure staged nothing, so the branch cut is unwound exactly as the
	// freeze-align failure path unwinds it; a commit failure left the closure
	// paths staged, so the branch is kept on purpose.
	if err := stageClosureSpec(ctx, root, specRef.Name); err != nil {
		fmt.Fprintln(stderr, "close:", err)
		reportUncommittedArchiveMove(specRef.Name, stderr)
		unwindClosureBranchCut(ctx, root, originalBranch, closureBranch, head, stderr)
		return 2
	}
	commitMsg := fmt.Sprintf("close: archive %s (%s)", specRef.String(), spec.Story)
	closeCommit, err := closeCreateCommit(ctx, root, commitMsg)
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
		reportStagedClosureCommitFailure(specRef.Name, closureBranch, commitMsg, stderr)
		return 2
	}

	// ac-2: publish the rollup to the configured tracker — the round-6
	// hermetic fake provider by default (dc-2), reaching the real publish
	// step exactly as `rollup --publish` does.
	pubRoll := provider.Rollup{
		Story:    provider.StoryRef(spec.Story),
		Ref:      specRef.String(),
		Commit:   head,
		Criteria: mapCriteria(fold.ACs),
		Eligible: fold.Eligible,
	}
	if err := deps.Registry.PublishRollup(ctx, pubRoll); err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}

	fmt.Fprintf(stdout, "close: archived %s to specs/archive/%s/ on branch %s (commit %s)\n", specRef.String(), specRef.Name, closureBranch, closeCommit)
	fmt.Fprintf(stdout, "close: rollup published to %s (eligible=%t)\n", spec.Story, fold.Eligible)
	fmt.Fprintln(stdout, "close: this verb stops at the branch (dc-3) — push it and open the closure MR/PR yourself:")
	fmt.Fprintf(stdout, "  git push -u origin %s\n", closureBranch)
	return 0
}

// requireCleanIndex refuses to begin either close ritual unless the index is
// EMPTY relative to HEAD. Close creates a commit, so inheriting index entries
// would make that commit claim changes the ritual did not produce. It requires
// nothing to be UNSTAGED — unrelated unstaged and untracked work is legal and
// intentionally survives closure — which is why it is named for the condition
// its own refusal text states rather than for the working tree.
//
// D6-33 applied to close (ledger L-N15(2)): the same AddAll-swept-the-whole-
// working-tree defect `verdi accept` was fixed for. The check runs before
// every mutation — branch cut, align freeze, rollup, status flip, archive
// move — because a later ordinary commit would carry inherited index entries
// regardless of how narrowly the ritual itself stages.
func requireCleanIndex(ctx context.Context, root string) error {
	paths, err := gitx.StagedPaths(ctx, root)
	if err != nil {
		return fmt.Errorf("checking the pre-ritual index: %w", err)
	}
	if len(paths) == 0 {
		return nil
	}
	if name := closureResidueName(storeRelativeStagedPaths(ctx, root, paths)); name != "" {
		return closureResidueRefusal(ctx, root, name, paths)
	}
	return fmt.Errorf("refusing to run with pre-existing staged paths %q; commit or unstage them before running the ritual", paths)
}

// storeRelativeStagedPaths re-bases gitx.StagedPaths' answers onto the store
// root, which is the base closureResidueName's zone prefixes are written in.
//
// The two bases differ exactly when the store root sits below the git root
// (store.FindRoot walks up to the nearest .verdi), and StagedPaths answers in
// REPOSITORY-root-relative paths on purpose — that is the property that makes
// it immune to diff.relative. Without this, an interrupted close's own index
// reads as foreign work in that layout and the operator is told to "commit or
// unstage" their half-finished archive: precisely the advice the residue
// refusal exists to replace.
//
// It returns nil — an index no name can be derived from, so the generic
// refusal stands — when any staged path lies OUTSIDE the store root, or when
// the prefix cannot be resolved at all. Both are the safe direction: verdi
// never claims an index it cannot prove it owns, and the refusal itself is
// already decided either way (the same posture closureResidueRefusal takes
// with CurrentBranch's error). Only the ownership question is asked in the
// store's vocabulary; both refusals still name paths exactly as git named
// them.
func storeRelativeStagedPaths(ctx context.Context, root string, paths []string) []string {
	prefix, err := gitx.RepoPrefix(ctx, root)
	if err != nil {
		return nil
	}
	if prefix == "" {
		return paths
	}
	out := make([]string, len(paths))
	for i, p := range paths {
		rest, inStore := strings.CutPrefix(p, prefix)
		if !inStore {
			return nil
		}
		out[i] = rest
	}
	return out
}

// closureResidueName returns the spec name an index full of closure residue
// belongs to, or "" when the staged set is not that shape.
//
// Ownership is derived from the INDEX, never from the caller's target
// argument: an interrupted close has already moved the spec out of the active
// zone, so a retry's own ref no longer resolves and the guard would never see
// a name to compare against. The index still carries the answer.
//
// The shape it recognises is every staged path under ONE spec's active or
// archive closure directory, with BOTH zones represented. The trailing
// separator keeps a prefix-sharing sibling ("close-fixture-two") from reading
// as residue of "close-fixture", and one foreign path collapses the answer to
// "" so verdi never claims an index it does not wholly own.
//
// Both zones is the load-bearing requirement, and it is a safety rule, not a
// tidiness one. The refusal this feeds ends in "deleting the leftover
// specs/archive/<name> directory", so the recognizer must not fire unless that
// archive tree is provably THIS run's own creation. The staged active-zone
// deletion is that proof: it exists only because the spec directory was
// tracked at HEAD and this ritual moved it away. Without it the index is
// byte-for-byte an ordinary staged edit inside an already-closed, COMMITTED
// archived spec — resolving a merge or rebase conflict there reaches it with
// no misbehaviour at all — and the advice would delete committed content from
// the operator's worktree.
//
// The cost is disclosed rather than hidden: an interrupted close of a spec
// that was NEVER COMMITTED stages the archive tree alone (stageClosureSpec
// omits an untracked active zone), and that residue now falls through to the
// generic refusal instead of the tailored one. It is still REFUSED — no new
// pass path, no silence — and losing tailored guidance is the strictly safe
// direction of that trade. A pure function of the index is also deliberate:
// this guard's only job is to refuse safely, so it acquires no way to fail.
func closureResidueName(paths []string) string {
	const activeRoot = ".verdi/specs/active/"
	const archiveRoot = ".verdi/specs/archive/"

	name := ""
	sawActive, sawArchive := false, false
	for _, p := range paths {
		rest, inArchive := strings.CutPrefix(p, archiveRoot)
		if !inArchive {
			var inActive bool
			if rest, inActive = strings.CutPrefix(p, activeRoot); !inActive {
				return ""
			}
		}
		specName, _, hasChild := strings.Cut(rest, "/")
		if !hasChild || specName == "" {
			return ""
		}
		if name == "" {
			name = specName
		} else if specName != name {
			return ""
		}
		sawArchive = sawArchive || inArchive
		sawActive = sawActive || !inArchive
	}
	if !sawActive || !sawArchive {
		return ""
	}
	return name
}

// closureResidueRefusal is the refusal for an index carrying nothing but one
// spec's own closure paths: an earlier run of this very ritual staged them and
// then failed to commit (a failing pre-commit hook, commit.gpgsign with no
// key, an unset user.email are the reachable causes).
//
// It still REFUSES — the guard opens no new pass path, and a second ritual
// over a half-finished archive is exactly what must not happen. What changes
// is the guidance: the generic "commit or unstage them" text is addressed to
// an operator's own staged work, and following it means unstaging your own
// half-finished archive with no way to tell ritual residue from real work.
// Here the state is known exactly, so the recovery is named exactly.
func closureResidueRefusal(ctx context.Context, root, name string, paths []string) error {
	active := store.SpecDirRelPath(store.ZoneActive, name)
	archive := store.SpecDirRelPath(store.ZoneArchive, name)
	where := ""
	// CurrentBranch's error is deliberately swallowed: this is refusal prose
	// enrichment, and the refusal itself is already decided. A detached HEAD
	// returns ("", nil) and simply adds no clause.
	if branch, err := gitx.CurrentBranch(ctx, root); err == nil && branch == "close/"+name {
		where = fmt.Sprintf(" You are on %s, where that run left off.", branch)
	}
	return fmt.Errorf("refusing to run: the index already carries spec/%s's OWN closure paths %q — an interrupted run of this ritual staged them and never committed.%s "+
		"Complete it with `git commit`, or abandon it by restoring the active zone (git restore --source=HEAD --staged --worktree -- %s), "+
		"unstaging the archive zone (git restore --staged -- %s), and deleting the leftover %s directory. "+
		"Unstaging alone is not enough: the spec directory is already moved on disk",
		name, paths, where, active, archive, archive)
}

// stageClosureSpec stages exactly the target spec's active-zone deletion and
// archive-zone tree. Story and feature closure share this one path assembler
// so neither ritual can widen its commit ownership independently.
//
// The guarantee is deliberately stated as "only the target spec's paths",
// NOT "only files the ritual itself wrote" (ledger L-N15(2)): an uncommitted
// edit inside the target spec directory still rides the archive tree — the
// same property that lets the living deviation report be frozen in place.
//
// The active-zone pathspec is included only when the spec directory is
// actually tracked. `git add` is FATAL (rc 128) and stages NOTHING when ANY
// pathspec matches neither the working tree nor the index — a failure mode
// gitx.AddAll never had. After the archive move the active-zone directory is
// gone from the working tree, so that pathspec survives on its index entries
// alone, which exist exactly when the directory is tracked at HEAD: this runs
// on close/<name> at its cut point, and requireCleanIndex already proved the
// index equals HEAD before the ritual mutated anything. A spec that was never
// committed therefore has no deletion to record, and asking git to record one
// anyway would abort the whole staging step and lose the archive tree with it.
func stageClosureSpec(ctx context.Context, root, name string) error {
	active := store.SpecDirRelPath(store.ZoneActive, name)
	archive := store.SpecDirRelPath(store.ZoneArchive, name)

	tracked, err := gitx.PathExistsAt(ctx, root, "HEAD", active)
	if err != nil {
		return fmt.Errorf("staging closure paths for spec/%s: %w", name, err)
	}
	var paths []string
	if tracked {
		paths = append(paths, active)
	}
	paths = append(paths, archive)

	if err := closeAddPaths(ctx, root, paths...); err != nil {
		return fmt.Errorf("staging closure paths for spec/%s: %w", name, err)
	}
	return nil
}

// reportUncommittedArchiveMove discloses the checkout state a staging failure
// leaves behind — one implementation for both rituals (the freezeAlignDeps
// precedent: no per-verb copy to drift). The caller unwinds the branch cut,
// but the archive move itself is NOT rolled back — transactional rollback of
// every post-branch-cut operational failure is explicitly out of the closure-
// session design's scope — so silence here would leave the operator to
// discover a moved spec directory on their own (constitution 2/10: silence is
// never a pass).
func reportUncommittedArchiveMove(name string, stderr io.Writer) {
	active := store.SpecDirRelPath(store.ZoneActive, name)
	archive := store.SpecDirRelPath(store.ZoneArchive, name)
	fmt.Fprintf(stderr, "close: nothing was committed, but this run's archive move is already on disk and UNCOMMITTED: %s is gone and %s exists. Restore the checkout before retrying: git restore --source=HEAD --staged --worktree -- %s, then delete the leftover %s directory\n", active, archive, active, archive)
}

// reportStagedClosureCommitFailure discloses the state a commit failure leaves
// behind — the shared implementation for both rituals, mirroring
// reportUncommittedArchiveMove.
//
// Unlike the staging failure, this path deliberately does NOT unwind the
// branch cut: the closure paths are already staged, and deleting close/<name>
// would strand that index on whatever branch the unwind returned to. Leaving
// the branch is also the more recoverable state — a single `git commit`
// completes what the ritual started — and the next `verdi close` recognises
// exactly this residue rather than blaming the operator for it
// (closureResidueRefusal).
func reportStagedClosureCommitFailure(name, closureBranch, commitMsg string, stderr io.Writer) {
	active := store.SpecDirRelPath(store.ZoneActive, name)
	archive := store.SpecDirRelPath(store.ZoneArchive, name)
	fmt.Fprintf(stderr, "close: the closure paths are STAGED on %s and nothing was committed; the branch and the index are left in place on purpose, because deleting either would strand this staged work. Complete it with: git commit -m %q — or abandon it by restoring the active zone (git restore --source=HEAD --staged --worktree -- %s), unstaging the archive zone (git restore --staged -- %s), and deleting the leftover %s directory\n", closureBranch, commitMsg, active, archive, archive)
}

// closeUncommittedRecordSource is the disclosure source id for a human record
// the closure fold consumed out of the working tree that HEAD does not carry
// identically, and uncommittedFoldRecordText is its explanation.
//
// Attestations and waivers live OUTSIDE both closure paths
// (.verdi/attestations/<storySlug>/<acID>.md, .verdi/waivers/...), and the
// fold reads them from the working tree with plain os.Stat/os.ReadFile — no
// git, no reachability check. With close committing only the target spec's own
// two paths, an operator can author one, never `git add` it, and have the
// index guard pass (untracked is not staged), the gate PASS on that file, and
// the closure commit and archive contain no trace of it. Under the old
// AddAll-the-working-tree staging it was swept in.
//
// The closure-session design DELIBERATELY refuses to absorb such records
// ("human records must already be committed in the HEAD they attest to"), and
// changing gate semantics is explicitly out of scope — so close neither
// absorbs nor refuses. What was missing is any word about a stated
// precondition nothing enforces. close-preflight dc-1 met this identical shape
// ("a mode can report ready while a real close would refuse") and recorded
// "closed, not disclaimed": a runtime disclosure computed by the same
// predicate, never prose in a design document. In CI the working tree equals
// HEAD, so this cannot arise there; it bites the documented --force-local
// local-close route.
const (
	closeUncommittedRecordSource = "close:uncommitted-fold-record"
	// vocab:identity — names the .verdi store paths and the `git add` verb (identity)
	uncommittedFoldRecordText = "the closure fold read this human record from the working tree, but HEAD does not carry it identically; close commits only the target spec's own active and archive paths, so this record enters neither the closure commit nor the archive — commit it in the HEAD it attests to (03 §Attestations and waivers) if it belongs to this closure's record"
)

// storyFoldRecordPaths returns the store-relative attestation and waiver paths
// a STORY fold consumed to reach its verdict, sorted and deduplicated.
//
// "Consumed" means the file materially made an AC pass: a waived AC consumed
// its waiver (evidence.WaiverActive returns true only for a present, active
// one), and an AttestationAuthored kind slot consumed its attestation. An
// unauthored scaffold satisfies nothing (spec/attest-helper dc-3), so it is
// not consumed. Everything is READ from the fold's own already-computed
// result — never re-derived over a differently-filtered record set (dc-2;
// ADJ-56) — so this can never disagree with the verdict it describes.
//
// Exposed as a plain function of (slug, results) so the preflight and prepare
// rehearsal paths can call this same predicate rather than growing a second
// copy of it.
func storyFoldRecordPaths(storySlug string, acs []evidence.ACResult) []string {
	var paths []string
	for _, ac := range acs {
		if ac.Status == evidence.StatusWaived {
			paths = append(paths, filepath.ToSlash(store.WaiverPath("", storySlug, ac.ID)))
		}
		for _, k := range ac.Kinds {
			if k.Kind == artifact.EvidenceAttestation && k.Attestation == evidence.AttestationAuthored {
				paths = append(paths, filepath.ToSlash(store.AttestationPath("", storySlug, ac.ID)))
			}
		}
	}
	return sortedUnique(paths)
}

// featureFoldRecordPaths is storyFoldRecordPaths' feature-class counterpart,
// reading the fold's own outcome-floor evaluation. It names attestations only:
// there is no waived status at the feature level (03 §The feature fold's table
// names exactly four statuses; waivers are a story-level-only mechanism), and
// the attestation slug is the FEATURE's own name, not a story ref's slug
// (evidence.FoldFeature's FeatureSlug; spec/close-preflight dc-6).
func featureFoldRecordPaths(featureSlug string, acs []evidence.FeatureACResult) []string {
	var paths []string
	for _, ac := range acs {
		if ac.Floor.DeclaresAttestation && ac.Floor.Attestation == evidence.AttestationAuthored {
			paths = append(paths, filepath.ToSlash(store.AttestationPath("", featureSlug, ac.ID)))
		}
	}
	return sortedUnique(paths)
}

// sortedUnique sorts in place and drops duplicates, so one AC satisfied by
// both a waiver and an attestation names each path once.
func sortedUnique(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	sort.Strings(paths)
	out := paths[:1]
	for _, p := range paths[1:] {
		if p != out[len(out)-1] {
			out = append(out, p)
		}
	}
	return out
}

// uncommittedFoldRecordPaths returns the subset of paths whose working-tree
// content is not byte-identical to what commit carries — either absent there
// entirely (never `git add`ed) or committed and since edited.
//
// Presence is asked through gitx.PathExistsAt precisely for its three-way
// contract: a proven absence at a resolvable commit is an answer, while an
// unresolvable commit or a broken repository is an error. Content equality
// then compares git's own committed blob id against the blob id `git add`
// would store for the current bytes, so attributes and filters are applied
// exactly as a real commit would apply them. Nothing here guesses: an
// operational failure propagates rather than silently reading as "clean".
//
// All three questions resolve rel against the SAME base — the store root this
// runs with (gitx sets cmd.Dir to it) — which the `./` in the rev-parse
// argument is load-bearing for. store.FindRoot walks up to the nearest .verdi,
// so the store root can legitimately sit BELOW the git root (the layout
// internal/gitx/stagedpaths.go names and stagedpaths_test.go builds), and
// there git resolves the three forms differently by default:
// `git ls-tree -- <path>` and `git hash-object <path>` take a working-directory-
// relative path, while `<rev>:<path>` is documented to resolve a BARE path
// against the working tree's root — a different file, usually a nonexistent
// one, which exits non-zero and turned every close of a story with a committed
// attestation into an operational failure. `<rev>:./<path>` is the documented
// spelling for "relative to the current working directory", which is what the
// other two already do. The fix is deliberately local: the bare `<rev>:<path>`
// shape is pre-existing and pervasive (gitx.Show alone has ten callers), and
// re-basing all of it is a separate, much larger change than this one.
func uncommittedFoldRecordPaths(ctx context.Context, root, commit string, paths []string) ([]string, error) {
	var out []string
	for _, rel := range paths {
		present, err := gitx.PathExistsAt(ctx, root, commit, rel)
		if err != nil {
			return nil, fmt.Errorf("checking whether %s is committed at %s: %w", rel, commit, err)
		}
		if !present {
			out = append(out, rel)
			continue
		}
		committed, err := gitx.RevParse(ctx, root, commit+":./"+rel)
		if err != nil {
			return nil, fmt.Errorf("resolving %s at %s: %w", rel, commit, err)
		}
		working, err := gitx.HashObject(ctx, root, filepath.FromSlash(rel))
		if err != nil {
			return nil, fmt.Errorf("hashing the working-tree %s: %w", rel, err)
		}
		if committed != working {
			out = append(out, rel)
		}
	}
	return out, nil
}

// discloseUncommittedFoldRecords prints one disclosure per consumed record
// commit does not carry identically, through the shared internal/disclosure
// seam so it reads in the same vocabulary as every other disclosure. It prints
// nothing when every consumed record is clean — the CI case, and the intended
// local one.
func discloseUncommittedFoldRecords(ctx context.Context, root, commit string, consumed []string, stdout io.Writer) error {
	uncommitted, err := uncommittedFoldRecordPaths(ctx, root, commit, consumed)
	if err != nil {
		return err
	}
	for _, rel := range uncommitted {
		fmt.Fprintln(stdout, disclosure.Render(disclosure.New(closeUncommittedRecordSource, rel, uncommittedFoldRecordText)))
	}
	return nil
}

// foldStory loads spec's authoritative (source: ci) evidence and folds it,
// via the shared foldStoryEvidence prologue (foldload.go) — kept here as
// its own small wrapper since close.go needs the full evidence.StoryResult
// (not just the closure gate's bool).
func foldStory(ctx context.Context, root string, spec *artifact.SpecFrontmatter, head string) (evidence.StoryResult, error) {
	// Preview stays false — co-1: closure folds ONLY source: ci evidence,
	// never the --preview escape hatch.
	return foldStoryEvidence(ctx, root, spec, head, false)
}

// closeAcceptedStatusLineRe matches the sole `status: accepted-pending-build`
// frontmatter line the closure flip rewrites to `status: closed`. Same
// anchored, multiline shape supersede.go's acceptedStatusLineRe uses for its
// own predecessor flip — a raw, status-line-only ReplaceAll so the archived
// spec.md differs from its active original on exactly that one line, keeping
// VL-010's status-only archive-flip exception (D6-11) cleanly satisfiable.
var closeAcceptedStatusLineRe = regexp.MustCompile(`(?m)^status:\s*"?accepted-pending-build"?\s*$`)

// flipSpecStatusToClosed rewrites the active-zone spec.md's status line from
// accepted-pending-build to closed (02 §Kind registry's "… → closed(archive)"
// transition), preserving every other byte — including the `frozen:` stamp: a
// closed spec is a post-acceptance, frozen artifact, exactly as a superseded
// one is (cmd/verdi/accept.go's predecessor flip). It insists on exactly one
// matching line so a spec whose status is not the expected pre-closure value
// (already closed, or malformed) is a loud internal error, never a silent
// no-op or a double flip.
func flipSpecStatusToClosed(root, name string) error {
	specPath := store.ActiveSpecPath(root, name)
	raw, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("close: reading %s to flip status to closed: %w", specPath, err)
	}
	if n := len(closeAcceptedStatusLineRe.FindAll(raw, -1)); n != 1 {
		// vocab:identity — frontmatter status-line machinery (field + enum value)
		return fmt.Errorf("close: %s: expected exactly one status: accepted-pending-build line to flip to closed, found %d", specPath, n)
	}
	// vocab:identity — frontmatter status-line machinery (field + enum value)
	newRaw := closeAcceptedStatusLineRe.ReplaceAll(raw, []byte("status: closed"))
	if err := os.WriteFile(specPath, newRaw, 0o644); err != nil {
		return fmt.Errorf("close: writing %s after flipping status to closed: %w", specPath, err)
	}
	return nil
}

// writeRollup builds, self-validates, and writes rollup.json into
// specs/active/<name>/ (still under the active zone — store.ArchiveMove
// moves it with the rest of the target spec directory immediately afterward).
func writeRollup(root string, specRef artifact.Ref, spec *artifact.SpecFrontmatter, head string, fold evidence.StoryResult) error {
	roll := artifact.Rollup{
		Schema:   "verdi.rollup/v1",
		Story:    spec.Story,
		Ref:      specRef.String(),
		Commit:   head,
		Criteria: mapRollupCriteria(fold.ACs),
		Eligible: fold.Eligible,
	}
	digest, err := rollupDigest(roll)
	if err != nil {
		return err
	}
	roll.Digest = digest

	// Self-validate before writing anything to disk (CLAUDE.md: "never
	// fake success") — a rollup that cannot round-trip through the same
	// Validate every other consumer (rollup --publish, providertest
	// read-back) uses is an internal bug, not a user-facing state.
	if err := roll.Validate(); err != nil {
		return fmt.Errorf("close: internal error: built rollup.json failed self-validation: %w", err)
	}

	data, err := canonjson.Marshal(roll)
	if err != nil {
		return fmt.Errorf("close: marshaling rollup.json: %w", err)
	}
	path := filepath.Join(root, ".verdi", "specs", "active", specRef.Name, "rollup.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("close: writing %s: %w", path, err)
	}
	return nil
}

// mapRollupCriteria maps the fold's per-AC results onto rollup.json's own
// RollupCriterion shape (internal/artifact/rollup.go) — CriterionStatus's
// values are the identical strings evidence.Status's constants already
// spell out (mirroring mapCriteria's own cast for the provider port,
// rollup.go).
func mapRollupCriteria(acs []evidence.ACResult) []artifact.RollupCriterion {
	out := make([]artifact.RollupCriterion, len(acs))
	for i, ac := range acs {
		out[i] = artifact.RollupCriterion{
			ID:      ac.ID,
			Text:    ac.Text,
			Status:  artifact.CriterionStatus(ac.Status),
			Summary: ac.Summary,
		}
	}
	return out
}

// rollupDigest hashes r's canonical JSON with Digest itself blanked out —
// recomputable by any verifier (02 §Generated artifacts and digests):
// read rollup.json, blank its own digest field, recompute, compare. The
// hash tail itself is canonjson.Digest (spec/shared-homes ac-2).
func rollupDigest(r artifact.Rollup) (string, error) {
	r.Digest = ""
	digest, err := canonjson.Digest(r)
	if err != nil {
		return "", fmt.Errorf("close: computing rollup digest: %w", err)
	}
	return digest, nil
}
