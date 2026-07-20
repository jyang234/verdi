// verdi close <feature-spec> (05 §CLI; 03 §The feature fold, §Stub
// reconciliation, §Closure ritual): the feature half of `verdi close`,
// deferred out of spec/close-verb's scope (close.go's doc comment) and
// completed here. runCloseFeature mirrors runClose's (close.go) story
// ritual in shape — gate, freeze, quartet, archive, commit, publish — and
// reuses everything that is class-agnostic (flipSpecStatusToClosed,
// store.ArchiveMove, rollupDigest, runAlignForSpec, closeDeps) unchanged;
// it diverges only where 03 §The feature fold and §Stub reconciliation
// actually require different mechanics:
//
//   - fold: evidence.FoldFeature over discovered implementing stories
//     (discoverImplementingStories, featurematrix.go — the SAME backlink-
//     inversion discovery `verdi matrix <feature>` uses), not evidence.Fold;
//   - gate: runFeatureClosureGate (closuregatefeature.go) additionally
//     requires stub reconciliation passed and every implementing story
//     actually closed, beyond the story gate's own conditions;
//   - quartet: rollup.json's Story field is left EMPTY when the feature
//     carries no story: tracker ref at all (R4-I-2 — spec/true-closure is
//     a real example) rather than fabricating one; board.json is the
//     grandfathered 4th quartet member (03 §Alignment report) and needs NO
//     feature-specific code at all — store.ArchiveMove renames the whole
//     spec directory verbatim, so a pre-existing board.json (frozen back
//     at accept time under the retired commit-to-design ritual) or its
//     absence both fall out for free, identically to the story path;
//   - publish: skipped entirely when the feature carries no story: ref —
//     never fabricating a tracker target that isn't there (see
//     writeFeatureRollup and the publish step below for the full
//     disclosed reasoning).
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/provider"
	"github.com/jyang234/verdi/internal/store"
)

// runCloseFeature is runClose's (close.go) feature-class counterpart,
// invoked once storyresolve.Resolve returns a spec.Class ==
// artifact.ClassFeature. Same exit-code convention as runClose: 0 clean,
// 1 the closure gate did not hold, 2 operational error.
func runCloseFeature(ctx context.Context, root string, spec *artifact.SpecFrontmatter, manifest *store.Manifest, deps closeDeps, stdout, stderr io.Writer) int {
	head, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}
	defaultBranchRef := lint.ResolveDefaultBranch(ctx, root)

	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		fmt.Fprintln(stderr, "close: internal error: resolved spec has an invalid id:", err)
		return 2
	}

	// Discover implementing stories the same way `verdi matrix <feature>`
	// does (featurematrix.go) — the index's computed backlink inversion,
	// story-folded once each.
	ix, err := index.Build(root)
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}
	stories, storiesByAC, _, err := discoverImplementingStories(ctx, root, head, ix, specRef.Name, spec)
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}

	fold, err := foldFeature(ctx, root, spec, specRef, head, storiesByAC)
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}
	reconciliation, err := reconcileFeatureStubs(spec, stories)
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}

	ok, err := runFeatureClosureGate(ctx, root, spec, fold, reconciliation, stories, deps.Forge, defaultBranchRef, manifest, deps.Model, head, stdout)
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}
	if !ok {
		// The class word is display prose and resolves (L-M13(1)).
		fmt.Fprintf(stdout, "close: FAIL (%s closure gate not satisfied; see conditions above)\n", deps.Model.DisplayClass("feature"))
		return 1
	}

	closureBranch := "close/" + specRef.Name
	if err := gitx.CheckoutNewBranch(ctx, root, closureBranch); err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}

	// Freeze the alignment report in place — the SAME generate-freeze-write
	// logic `verdi align --freeze` uses (close.go's runAlignForSpec,
	// consumed unchanged). A round-four feature is never built directly
	// (03 §Lifecycle: the feature-first cascade — stories are the unit of
	// build), so it ordinarily carries no impacts: at all and this call
	// degenerates to freezing an empty-computed-section report; a
	// grandfathered v0 feature that DOES carry impacts: is handled exactly
	// as align.go already handles any spec. The regenerate fallback path
	// mints a fresh Provenance, so this needs a resolved model digest
	// exactly like close.go's own runClose (spec/model-digest ledger L-M5).
	modelDigest, err := resolveModelDigest(root)
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}
	alignD := alignDeps{Runner: deps.Runner, JudgeCmd: deps.JudgeCmd, JudgeRequired: deps.JudgeRequired, JudgeTimeout: deps.JudgeTimeout, ModelDigest: modelDigest}
	if rc := runAlignForSpec(ctx, root, spec, head, true, alignD, stdout, stderr); rc != 0 {
		fmt.Fprintln(stderr, "close: freezing the alignment report failed (see above)")
		return rc
	}

	if err := writeFeatureRollup(root, specRef, spec, head, fold); err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}

	// Flip status accepted-pending-build -> closed as part of the archive
	// step, then move the whole directory — byte-for-byte identical to the
	// story path (02 §Kind registry's "... -> closed(archive)" transition
	// applies to both spec classes alike; flipSpecStatusToClosed and
	// store.ArchiveMove are both already class-agnostic, consumed
	// unchanged). board.json, the grandfathered 4th quartet member (03
	// §Alignment report), needs no special handling here: if the active
	// spec directory already carries one (a pre-R4 artifact frozen back at
	// accept time under the retired commit-to-design ritual), ArchiveMove's
	// bare directory rename carries it along for free; if absent — the
	// common case for any round-four spec, since `verdi board commit` is
	// retired (05 §Workbench) — there is simply nothing to move, no error.
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
	commitMsg := fmt.Sprintf("close: archive %s", specRef.String())
	closeCommit, err := gitx.CreateCommit(ctx, root, commitMsg)
	if err != nil {
		fmt.Fprintln(stderr, "close:", err)
		return 2
	}

	// Publish: only when the feature carries a story: tracker ref at all.
	// R4-I-2 made story: OPTIONAL on the feature class (an epic/objective
	// ref, never a per-story binding) — spec/true-closure, this very
	// repo's own in-flight feature, carries none. 04 §Semantics's
	// PublishRollup needs a concrete StoryRef to key its idempotent
	// (Story, Commit) write on; a feature with no tracker story has
	// nowhere honest to publish to. The smallest reversible option
	// (invention-ledger candidate, disclosed here and in the phase
	// report): still write and archive the rollup.json quartet member
	// UNCONDITIONALLY (the durable, git-tracked record of the fold — never
	// skipped), but skip the tracker publish call itself when Story == "",
	// rather than fabricating a ref that isn't there. When the feature
	// does carry a story: ref, publish exactly as the story path does.
	if spec.Story != "" {
		pubRoll := provider.Rollup{
			Story:    provider.StoryRef(spec.Story),
			Ref:      specRef.String(),
			Commit:   head,
			Criteria: mapFeatureCriteria(fold.ACs),
			Eligible: featureAllEvidenced(fold),
		}
		if err := deps.Registry.PublishRollup(ctx, pubRoll); err != nil {
			fmt.Fprintln(stderr, "close:", err)
			return 2
		}
		fmt.Fprintf(stdout, "close: rollup published to %s (eligible=%t)\n", spec.Story, pubRoll.Eligible)
	} else {
		// The leading class word is display prose and resolves (L-M13(1));
		// "story:" names the FRONTMATTER FIELD (trailing colon) — identity,
		// bare — and rollup.json is a filename.
		fmt.Fprintf(stdout, "close: %s carries no story: tracker ref (R4-I-2); rollup.json is archived but not published to any tracker\n", deps.Model.DisplayClass("feature"))
	}

	fmt.Fprintf(stdout, "close: archived %s to specs/archive/%s/ on branch %s (commit %s)\n", specRef.String(), specRef.Name, closureBranch, closeCommit)
	fmt.Fprintln(stdout, "close: this verb stops at the branch (dc-3) — push it and open the closure MR/PR yourself:")
	fmt.Fprintf(stdout, "  git push -u origin %s\n", closureBranch)
	return 0
}

// foldFeature loads spec's own authoritative (source: ci) evidence —
// outcome-level records bound directly to its own AC ids, the outcome
// floor's automated-record path (03 §The feature fold) — and folds it
// together with the already-discovered, already-story-folded implementing
// stories via evidence.FoldFeature. Mirrors close.go's foldStory shape.
func foldFeature(ctx context.Context, root string, spec *artifact.SpecFrontmatter, specRef artifact.Ref, head string, storiesByAC map[string][]evidence.ImplementingStory) (evidence.FeatureResult, error) {
	derivedRoot := store.DerivedSpecDir(root, store.RefSlug(spec.ID))
	records, err := evidence.LoadRecords(ctx, root, derivedRoot, head)
	if err != nil {
		return evidence.FeatureResult{}, fmt.Errorf("loading feature evidence records: %w", err)
	}
	// Preview stays false — closure folds ONLY source: ci evidence, the
	// same co-1 authoritative-only posture the story path enforces.
	result, err := evidence.FoldFeature(evidence.FeatureInput{
		Spec:        spec,
		Stories:     storiesByAC,
		Records:     records,
		Preview:     false,
		StoreRoot:   root,
		FeatureSlug: specRef.Name,
	})
	if err != nil {
		return evidence.FeatureResult{}, fmt.Errorf("folding feature evidence: %w", err)
	}
	return result, nil
}

// reconcileFeatureStubs runs 03 §Stub reconciliation's bidirectional
// completeness check over spec's acceptance-time stub list, given the
// same implementing stories discoverImplementingStories already found.
// No withdrawal-declaration source exists yet (evidence.StubWithdrawal's
// doc comment: no committed artifact schema for it exists) — this is the
// same honest, nothing-invented posture cmd/verdi/featurematrix.go's own
// call already takes, not a narrowing specific to close.
func reconcileFeatureStubs(spec *artifact.SpecFrontmatter, stories []implementingStoryEdges) (evidence.StubReconciliation, error) {
	stubStories := make([]evidence.StubStory, len(stories))
	for i, s := range stories {
		stubStories[i] = evidence.StubStory{SpecRef: s.SpecRef, ACIDs: s.ACIDs, Closed: s.Closed}
	}
	return evidence.ReconcileStubs(evidence.StubReconcileInput{Spec: spec, Stories: stubStories})
}

// featureAllEvidenced reports whether every one of fold's feature ACs
// folded to evidenced — the same condition checkFeatureFoldEligible
// (closuregatefeature.go) gates on, recomputed here (rather than plumbed
// through as a bool) so writeFeatureRollup and the publish step each stay
// simple functions of a FeatureResult alone.
func featureAllEvidenced(fold evidence.FeatureResult) bool {
	for _, ac := range fold.ACs {
		if ac.Status != evidence.StatusEvidenced {
			return false
		}
	}
	return true
}

// mapFeatureCriteria maps the feature fold's per-AC results onto the
// story-provider port's CriterionStatus shape — mirrors close.go's
// mapCriteria for evidence.FeatureACResult instead of evidence.ACResult.
func mapFeatureCriteria(acs []evidence.FeatureACResult) []provider.CriterionStatus {
	out := make([]provider.CriterionStatus, len(acs))
	for i, ac := range acs {
		out[i] = provider.CriterionStatus{
			ID:      ac.ID,
			Text:    ac.Text,
			Status:  string(ac.Status),
			Summary: ac.Summary,
		}
	}
	return out
}

// mapFeatureRollupCriteria maps the feature fold's per-AC results onto
// rollup.json's own RollupCriterion shape — mirrors close.go's
// mapRollupCriteria for evidence.FeatureACResult instead of
// evidence.ACResult. evidence.Status's string constants are the identical
// spelling artifact.CriterionStatus's constants use (both packages spell
// "evidenced"/"violated"/"pending"/"no-signal"/"waived" verbatim), so the
// cast is exact; a feature fold never actually produces "waived" (03 §The
// feature fold: "there is no waived status at the feature level"), but the
// cast is total regardless, matching mapRollupCriteria's own posture.
func mapFeatureRollupCriteria(acs []evidence.FeatureACResult) []artifact.RollupCriterion {
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

// writeFeatureRollup builds, self-validates, and writes rollup.json into
// specs/active/<name>/ (still under the active zone — store.ArchiveMove
// moves it, along with the rest of the quartet, immediately afterward) —
// mirrors close.go's writeRollup for a evidence.FeatureResult instead of a
// evidence.StoryResult. spec.Story may be "" (R4-I-2: a feature's
// story: tracker ref is optional) — artifact.Rollup.Validate() accepts an
// empty Story exactly for this reason; the rollup is still a complete,
// self-contained, digest-verifiable record of the fold even when there is
// nowhere to publish it.
func writeFeatureRollup(root string, specRef artifact.Ref, spec *artifact.SpecFrontmatter, head string, fold evidence.FeatureResult) error {
	roll := artifact.Rollup{
		Schema:   "verdi.rollup/v1",
		Story:    spec.Story,
		Ref:      specRef.String(),
		Commit:   head,
		Criteria: mapFeatureRollupCriteria(fold.ACs),
		Eligible: featureAllEvidenced(fold),
	}
	digest, err := rollupDigest(roll)
	if err != nil {
		return err
	}
	roll.Digest = digest

	// Self-validate before writing anything to disk (CLAUDE.md: "never fake
	// success") — mirrors writeRollup's own self-check.
	if err := roll.Validate(); err != nil {
		return fmt.Errorf("close: internal error: built feature rollup.json failed self-validation: %w", err)
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
