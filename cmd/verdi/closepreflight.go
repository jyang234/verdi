// verdi close --preflight <ref> (spec/close-preflight; closure-ergonomics
// ac-1/dc-5/ADJ-23): a mode of the existing close verb, never a new one
// (dc-1) — rehearses every condition a real `verdi close <ref>` would
// refuse on, for a story or a feature spec alike (dc-3), through the
// IDENTICAL evaluation functions close itself calls (dc-2: runClosureGate
// for a story, runFeatureClosureGate for a feature, closuregate.go /
// closuregatefeature.go, both consumed completely unchanged) — and stops
// there. Both functions are already pure with respect to the store
// (nothing on disk changes until AFTER they return ok=true; the mutating
// tail close.go/closefeature.go run afterward is never reached from here),
// so --preflight structurally cannot cut a branch, freeze anything, write
// a file, or publish a rollup, in any of its three outcomes (ready / unmet
// / operational error).
//
// Where the gate's own per-condition breakdown is already itemized enough
// — spec-stale's own-text finding id(s), pending-supersession's MR/object
// ids, stub-reconciliation's unreconciled slugs, the implementing-stories
// condition's still-open refs — runClosureGate/runFeatureClosureGate's own
// printed lines are surfaced completely unchanged (this file adds nothing
// to them). Where it is not — an AC's own evidenced/pending/violated/
// no-signal status collapses to one coarse line with no path attached
// (closuregate.go:91, closuregatefeature.go:148) — unmetACDetail (this
// file) renders the missing per-kind detail straight from the same fold
// primitives (evidence.Current, evidence.RecordsForAC,
// evidence.LoadAttestationState, evidence.ExcludedCommitDirs) the shared
// internal/wallbadge empty-slot compute already establishes as the
// correct, non-duplicative way for an external fold consumer to get this
// detail (dc-4) — never a re-derived verdict: which AC is unmet, and its
// overall status, is read ONLY from the evidence.StoryResult/
// evidence.FeatureResult the shared gate functions' own callees
// (foldStoryEvidence/foldFeature) already compute, recomputed here the
// same deterministic, side-effect-free way close.go's own runClose already
// recomputes it a second time for the rollup payload (foldStory).
package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

// preflightGuardDisclosureText is dc-1's added clause: whenever --preflight
// runs outside a detected CI environment without --force-local, it
// discloses — once, informationally, never as a failing condition — that a
// real close run under those same conditions would separately refuse at
// the CI-only publish guard, regardless of the gate verdict above. This
// closes the gap this story's own design-mode judge sweep found
// (judged-dcj-2): without it, a "ready" preflight run locally without
// --force-local could be followed by a real close's surprise refusal at a
// guard --preflight itself never needs to hit.
const preflightGuardDisclosureText = `outside a detected CI environment and without --force-local, a real "verdi close" run on this ref would separately refuse at the CI-only publish guard (04 §Semantics: "PublishRollup runs in CI only"), regardless of the gate verdict above; this --preflight run itself never reaches that guard`

// closePublishGuardRefuses is the ONE boolean evaluation both cmdClose's
// own CI-only publish-guard refusal and --preflight's guard disclosure
// (dc-1) call — factored out so the two print sites can never independently
// drift apart (dc-1's own second judge-fix wording: "one predicate, two
// print sites, not two predicates").
func closePublishGuardRefuses(forceLocal bool) bool {
	return !lint.ReadCIEnv().InCI && !forceLocal
}

// runPreflight is `--preflight`'s testable dispatch core: resolves storyArg
// exactly as runClose does (storyresolve.Resolve, the identical I-30
// two-form contract), then routes to the story or feature preflight gate by
// the resolved spec's Class — the same dispatch runClose itself performs
// (close.go's "if spec.Class == artifact.ClassFeature", dc-3). Exit
// discipline mirrors runClose exactly: 0 the gate holds (ready to close), 1
// the gate does not (a verdict), 2 a genuine operational failure (dc-5) —
// never for a merely-absent artifact, which is always a verdict.
func runPreflight(ctx context.Context, root, storyArg string, manifest *store.Manifest, f forge.Forge, forceLocal bool, stdout, stderr io.Writer) int {
	spec, err := storyresolve.Resolve(root, storyArg)
	if err != nil {
		fmt.Fprintln(stderr, "close: --preflight:", err)
		return 2
	}

	fmt.Fprintf(stdout, "close: --preflight %s (dry run: rehearses the closure gate only; nothing on disk changes and nothing is published)\n", storyArg)

	head, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}
	defaultBranchRef := lint.ResolveDefaultBranch(ctx, root)

	var ok bool
	if spec.Class == artifact.ClassFeature {
		ok, err = runFeaturePreflightGate(ctx, root, spec, manifest, f, defaultBranchRef, head, stdout)
	} else {
		ok, err = runStoryPreflightGate(ctx, root, spec, manifest, f, defaultBranchRef, head, stdout)
	}
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}

	if closePublishGuardRefuses(forceLocal) {
		fmt.Fprintln(stdout, disclosure.Render(disclosure.New("close:preflight-publish-guard", "", preflightGuardDisclosureText)))
	}

	if !ok {
		fmt.Fprintln(stdout, "close: --preflight: NOT READY (closure gate would refuse a real close; see conditions above)")
		return 1
	}
	fmt.Fprintln(stdout, "close: --preflight: READY (closure gate holds; a real close would proceed to archive)")
	return 0
}

// runStoryPreflightGate runs the SAME evaluation function a real story-
// class `verdi close` calls first (runClosureGate, closuregate.go — dc-2),
// printing its unchanged PASS/FAIL/disclosed lines, then enriches
// condition 1's coarse eligibility line with the exact missing-evidence-
// kind/path detail ac-1 requires (dc-4) and condition 2's spec-stale line
// with the deviation-report.md path ac-1 additionally requires (neither of
// which the shared gate's own Reason strings carry today).
func runStoryPreflightGate(ctx context.Context, root string, spec *artifact.SpecFrontmatter, manifest *store.Manifest, f forge.Forge, defaultBranchRef, head string, stdout io.Writer) (bool, error) {
	ok, err := runClosureGate(ctx, root, spec, f, defaultBranchRef, manifest, head, stdout)
	if err != nil {
		return false, err
	}

	// dc-2: recompute the SAME fold a second time (deterministic, pure,
	// side-effect-free — the identical pattern close.go's own runClose
	// already uses for its rollup payload) to get the full per-AC
	// evidence.StoryResult the gate's own coarse condition-1 line does not
	// expose to its caller.
	result, err := foldStoryEvidence(ctx, root, spec, head, false)
	if err != nil {
		return false, fmt.Errorf("close: --preflight: %w", err)
	}
	unmet := make(map[string]bool, len(result.ACs))
	for _, ac := range result.ACs {
		if ac.Status != evidence.StatusEvidenced && ac.Status != evidence.StatusWaived {
			unmet[ac.ID] = true
		}
	}
	storySlug := store.RefSlug(spec.Story)
	lines, err := unmetACDetail(ctx, root, spec.ID, spec.AcceptanceCriteria, unmet, storySlug, head)
	if err != nil {
		return false, fmt.Errorf("close: --preflight: %w", err)
	}
	printACDetail(stdout, lines)

	if err := printSpecStalePathIfFailing(stdout, root, spec, manifest); err != nil {
		return false, fmt.Errorf("close: --preflight: %w", err)
	}

	return ok, nil
}

// printSpecStalePathIfFailing recomputes checkSpecStaleCondition (the
// identical function runClosureGate/runFeatureClosureGate's own condition
// already called moments ago) to learn whether it failed, and if so prints
// the deviation-report.md path ac-1 requires but the condition's own
// Reason string does not carry — never re-deciding the condition, only
// reading its already-computed OK field a second, deterministic time.
func printSpecStalePathIfFailing(stdout io.Writer, root string, spec *artifact.SpecFrontmatter, manifest *store.Manifest) error {
	specStale, err := checkSpecStaleCondition(root, spec, manifest)
	if err != nil {
		return err
	}
	if specStale.OK {
		return nil
	}
	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		return fmt.Errorf("internal error: resolved spec has an invalid id: %w", err)
	}
	path := filepath.ToSlash(filepath.Join(".verdi", "specs", "active", specRef.Name, "deviation-report.md"))
	fmt.Fprintf(stdout, "close: --preflight: deviation-report.md: %s\n", path)
	return nil
}

// unmetACDetail renders dc-4's exact missing-evidence-kind/path detail for
// every AC in declared whose id is in unmet — the per-declared-kind
// breakdown neither runClosureGate's nor runFeatureClosureGate's own coarse
// Reason string carries today (dc-2). slug is the attestation-directory
// slug the caller resolves per dc-6: store.RefSlug(spec.Story) for a
// story-scope AC, or the feature spec's own Name (FeatureSlug) for a
// feature outcome-floor AC — this function takes it as given, the same
// "caller resolves, fold reduces" idiom evidence.FeatureInput.FeatureSlug's
// own doc comment already establishes.
//
// Which AC is unmet is never decided here (dc-2: unmet is the caller's
// already-computed evidence.StoryResult/evidence.FeatureResult) — only
// which of an already-known-unmet AC's OWN declared evidence kinds lacks a
// satisfying record is new rendering detail, computed from the same
// already-exported fold primitives (evidence.Current, evidence.RecordsForAC,
// evidence.LoadAttestationState, evidence.ExcludedCommitDirs)
// internal/wallbadge's own empty-slot compute already uses for exactly
// this purpose — never a copy-pasted reimplementation of the fold's own
// unexported kindStatus.
func unmetACDetail(ctx context.Context, root, specID string, declared []artifact.AcceptanceCriterion, unmet map[string]bool, slug, head string) ([]string, error) {
	if len(unmet) == 0 {
		return nil, nil
	}

	derivedRoot := filepath.Join(root, ".verdi", "data", "derived", store.RefSlug(specID))
	records, err := evidence.LoadRecords(ctx, root, derivedRoot, head)
	if err != nil {
		return nil, err
	}
	// dc-4: named "for free" alongside the derived-tree root — any commit
	// directory found on disk but excluded (non-ancestor) from the fold's
	// own current-record computation, computed via the identical ancestry
	// check LoadRecords itself already performs per entry.
	excluded, err := evidence.ExcludedCommitDirs(ctx, root, derivedRoot, head)
	if err != nil {
		return nil, err
	}
	derivedRel := filepath.ToSlash(filepath.Join(".verdi", "data", "derived", store.RefSlug(specID))) + "/"

	var lines []string
	for _, ac := range declared {
		if !unmet[ac.ID] {
			continue
		}
		current := evidence.Current(evidence.RecordsForAC(records, ac.ID))
		for _, kind := range ac.Evidence {
			if kind == artifact.EvidenceAttestation {
				state, err := evidence.LoadAttestationState(root, slug, ac.ID)
				if err != nil {
					return nil, err
				}
				if state == evidence.AttestationAuthored {
					continue
				}
				path := filepath.ToSlash(evidence.AttestationPath("", slug, ac.ID))
				switch state {
				case evidence.AttestationAbsent:
					lines = append(lines, fmt.Sprintf("%s attestation: no file at %s; scaffold it with `verdi attest`", ac.ID, path))
				case evidence.AttestationUnauthored:
					lines = append(lines, fmt.Sprintf("%s attestation: a scaffold is present at %s but the claim is unauthored (sentinel present); author it", ac.ID, path))
				}
				continue
			}

			satisfied := false
			for _, r := range current {
				if r.Kind == kind && r.Verdict == artifact.VerdictPass {
					satisfied = true
					break
				}
			}
			if satisfied {
				continue
			}

			line := fmt.Sprintf("%s %s: no current passing record; derived-tree root probed: %s", ac.ID, kind, derivedRel)
			if len(excluded) > 0 {
				line += fmt.Sprintf(" (found but excluded as non-ancestor: %v)", excluded)
			}
			lines = append(lines, line)
		}
	}
	return lines, nil
}

// printACDetail prints unmetACDetail's lines under one grep-friendly
// header — a no-op when lines is empty (every AC evidenced or waived;
// nothing to add beyond the gate's own output).
func printACDetail(stdout io.Writer, lines []string) {
	if len(lines) == 0 {
		return
	}
	fmt.Fprintln(stdout, "close: --preflight: missing-evidence detail:")
	for _, l := range lines {
		fmt.Fprintf(stdout, "close: --preflight:   %s\n", l)
	}
}
