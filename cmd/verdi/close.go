// verdi close <jira:STORY-KEY | spec/name | feature spec/name> (05 §CLI;
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
//     the whole quartet to specs/archive/<name>/ (store.ArchiveMove, a
//     pure rename — VL-010's sole legal exception on an otherwise-frozen
//     spec.md).
//  4. Commit the quartet + the archive rename on the closure branch.
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
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
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
func freezeAlignDeps(deps closeDeps, modelDigest string) alignDeps {
	return alignDeps{
		Runner:        deps.Runner,
		JudgeCmd:      deps.JudgeCmd,
		JudgeRequired: deps.JudgeRequired,
		JudgeTimeout:  deps.JudgeTimeout,
		ModelDigest:   modelDigest,
		Wait:          true,
	}
}

// cmdClose is `verdi close`'s entry point, invoked by dispatch.go.
func cmdClose(args []string, stdout, stderr io.Writer) int {
	forceLocal := false
	preflight := false
	var storyArg string
	for _, a := range args {
		switch a {
		case "--force-local":
			forceLocal = true
			continue
		case "--preflight":
			preflight = true
			continue
		}
		if storyArg != "" {
			fmt.Fprintf(stderr, "close: unexpected extra argument %q\n", a)
			return 2
		}
		storyArg = a
	}
	if storyArg == "" {
		fmt.Fprintln(stderr, "close: usage: verdi close <jira:STORY-KEY | spec/name> [--force-local] [--preflight]")
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
	inCI := lint.ReadCIEnv().InCI
	if closePublishGuardRefuses(forceLocal) {
		fmt.Fprintln(stderr, "close: refusing to publish outside CI (04 §Semantics: \"PublishRollup runs in CI only\"); pass --force-local to run anyway for local testing only")
		return 2
	}
	if !inCI {
		fmt.Fprintln(stderr, "close: --force-local: running outside CI; this escape hatch exists for local testing only and its publish is NON-AUTHORITATIVE (04 §Semantics: PublishRollup runs in CI only)")
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
	return runClose(ctx, root, storyArg, manifest, deps, stdout, stderr)
}

// runClose is the testable core: given an already-resolved root, a
// story/spec argument, the decoded manifest, and injected deps, run the
// whole closure ritual and return the exit code (CLAUDE.md: 0 clean,
// 1 the closure gate did not hold, 2 operational error).
func runClose(ctx context.Context, root, storyArg string, manifest *store.Manifest, deps closeDeps, stdout, stderr io.Writer) int {
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

	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		fmt.Fprintln(stderr, "close: internal error: resolved spec has an invalid id:", err)
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
	modelDigest, err := resolveModelDigest(root)
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}
	alignD := freezeAlignDeps(deps, modelDigest)
	if rc := runAlignForSpec(ctx, root, spec, head, true, alignD, stdout, stderr); rc != 0 {
		fmt.Fprintln(stderr, "close: freezing the alignment report failed (see above)")
		return rc
	}

	if err := writeRollup(root, specRef, spec, head, fold); err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}

	// Flip the spec's status accepted-pending-build → closed as part of the
	// archive step (02 §Kind registry: story/feature specs transition
	// "… → closed(archive)"). Done in the active-zone spec.md BEFORE
	// ArchiveMove renames the directory, so the whole quartet moves in one
	// shot: the spec.md moves with its sole status-line change and everything
	// else byte-identical — VL-010's round-6 status-only archive-flip
	// exception (D6-11), not the pure-rename one, is what admits the move.
	if err := flipSpecStatusToClosed(root, specRef.Name); err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}

	if err := store.ArchiveMove(root, specRef.Name); err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}

	if err := gitx.AddAll(ctx, root); err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}
	commitMsg := fmt.Sprintf("close: archive %s (%s)", specRef.String(), spec.Story)
	closeCommit, err := gitx.CreateCommit(ctx, root, commitMsg)
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
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
// moves it, along with the rest of the quartet, immediately afterward).
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
