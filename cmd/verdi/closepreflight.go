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
	// expose to its caller. The per-kind missing/violated/unauthored detail
	// is read STRAIGHT from that result's own per-kind evaluation
	// (evidence.ACResult.Kinds), never re-derived over a differently-filtered
	// record set (ADJ-56).
	result, err := foldStoryEvidence(ctx, root, spec, head, false)
	if err != nil {
		return false, fmt.Errorf("close: --preflight: %w", err)
	}
	derivedRel, excluded, err := preflightDerivedContext(ctx, root, spec.ID, head)
	if err != nil {
		return false, fmt.Errorf("close: --preflight: %w", err)
	}
	printACDetail(stdout, unmetStoryACDetail(result.ACs, store.RefSlug(spec.Story), derivedRel, excluded))

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
	path := store.DeviationReportRelPath(store.ZoneActive, specRef.Name)
	fmt.Fprintf(stdout, "close: --preflight: deviation-report.md: %s\n", path)
	return nil
}

// preflightDerivedContext resolves the two pure-presentation facts a
// missing-evidence disclosure names alongside the fold's OWN evaluation: the
// store-relative derived-tree root the fold reads records from
// (.verdi/data/derived/<ref-slug>/, foldload.go's own join) and any commit
// directory found on disk but excluded as a non-ancestor (dc-4's "for free"
// stale rendering — the identical ancestry check LoadRecords already performs
// per entry, evidence.ExcludedCommitDirs). Neither changes any verdict; both
// are diagnostics the fold already computed and the coarse gate line
// discards.
func preflightDerivedContext(ctx context.Context, root, specID, head string) (derivedRel string, excluded []string, err error) {
	derivedRoot := store.DerivedSpecDir(root, store.RefSlug(specID))
	excluded, err = evidence.ExcludedCommitDirs(ctx, root, derivedRoot, head)
	if err != nil {
		return "", nil, err
	}
	derivedRel = store.DerivedSpecRelDir(store.RefSlug(specID)) + "/"
	return derivedRel, excluded, nil
}

// unmetStoryACDetail renders dc-4's per-declared-kind missing/violated/
// unauthored detail for every unmet story AC, consuming the fold's OWN
// per-kind evaluation (evidence.ACResult.Kinds) rather than re-deriving a
// second, differently-filtered per-kind status (ADJ-56: the detail layer
// obeys dc-2's one-enumeration principle exactly as the verdict layer does).
// Which AC is unmet, whether each declared kind is satisfied under the
// authoritative-source fold, and which kind carries a failing witness are all
// READ from the caller's already-computed StoryResult — this function only
// maps those evaluated slots to paths and human sentences. slug is the
// story-scope attestation slug (store.RefSlug(spec.Story), dc-6);
// derivedRel/excluded are the caller's already-resolved presentation facts.
func unmetStoryACDetail(acs []evidence.ACResult, slug, derivedRel string, excluded []string) []string {
	var lines []string
	for _, ac := range acs {
		if ac.Status == evidence.StatusEvidenced || ac.Status == evidence.StatusWaived {
			continue // a met AC (evidenced or waived) has nothing to disclose.
		}
		for _, k := range ac.Kinds {
			if line := renderStoryKindGap(ac.ID, k, slug, derivedRel, excluded); line != "" {
				lines = append(lines, line)
			}
		}
	}
	return lines
}

// renderStoryKindGap maps one evaluated per-kind slot to its disclosure line
// (or "" when the kind is satisfied and needs none). It reads only the fold's
// own KindResult — Satisfied, the attestation state, and any failing witness
// — so a source:local pass the authoritative fold discounts reads here as
// unsatisfied too (ADJ-56 finding 1), and a violated kind is named as a
// violation, never as merely-absent (finding 3).
func renderStoryKindGap(acID string, k evidence.KindResult, slug, derivedRel string, excluded []string) string {
	if k.Kind == artifact.EvidenceAttestation {
		if k.Satisfied {
			return ""
		}
		path := filepath.ToSlash(evidence.AttestationPath("", slug, acID))
		if k.Attestation == evidence.AttestationUnauthored {
			return fmt.Sprintf("%s attestation: a scaffold is present at %s but the claim is unauthored (sentinel present); author it", acID, path)
		}
		return fmt.Sprintf("%s attestation: no file at %s; scaffold it with `verdi attest`", acID, path)
	}

	// A violated kind (a current FAILING record) is named as a violation with
	// its witness — never flattened into the coarse "no current passing
	// record" line, which misdescribes a failing witness as an absent one
	// (ADJ-56 finding 3). This wins even when a passing record of the same
	// kind coexists (a distinct producer), so the disclosure never goes silent
	// on a violated AC.
	if k.Violating != nil {
		return fmt.Sprintf("%s %s: current record FAILED (witness %q); fix or supersede it — derived-tree root probed: %s", acID, k.Kind, k.Violating.Witness, derivedRel)
	}
	if k.Satisfied {
		return ""
	}
	line := fmt.Sprintf("%s %s: no current passing record; derived-tree root probed: %s", acID, k.Kind, derivedRel)
	if len(excluded) > 0 {
		line += fmt.Sprintf(" (found but excluded as non-ancestor: %v)", excluded)
	}
	return line
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
