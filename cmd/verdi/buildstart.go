// verdi build start <story-spec | story-ref> (05 §CLI, R4-I-6): the
// post-acceptance build ritual, replacing v0's `feature start` now that
// the unit of build is the STORY (03 §Lifecycle: the feature-first
// cascade, step 3 "Build"). Locates the story's spec (I-30 strict forms,
// reusing internal/storyresolve), REFUSES (exit 1) unless its status is
// accepted-pending-build (03 §Gates condition 1's local half) and unless
// no unresolved rung-4 cascade-stale/invalidated verdict blocks it
// (cascadecheck.go), cuts the build branch feature/<name> (the git-branch
// naming convention is kept unchanged from v0's `feature start` —
// storyresolve.ResolveBuildSpec and gate.go's condition 1 both already
// depend on it; renaming the branch prefix is a separate, unforced change
// this phase does not make), and best-effort refreshes the baseline
// (baseline.go). Kept in its own file per the lint.go/sync.go/matrix.go/
// dex.go convention.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
	"github.com/jyang234/verdi/internal/upstream"
)

// specStateResolver is the internal/specstate.Projector consumption surface
// this package's activation/eligibility decisions need — Resolve for a
// single candidate (build start, gate) and ResolveMany for a batch (feature
// archive's cross-level ruling gather, feature matrix's implementing-story
// classification: one Git-derived-state resolution per call, not one per
// candidate, so a batch consumer never triggers an O(specs²) scan). 04
// §port pattern: define the interface at the consumer, accept interfaces,
// return structs — the same shape internal/lint's own SpecStateResolver
// already established for its rules, widened here by the one extra method
// this package's batch consumers need. Production always wires the real,
// git-backed specstate.NewProjector() (every cmd* constructor in this
// package); the interface exists so a test can substitute a fake driving a
// specstate.Result shape real git cannot practically reconstruct (mirroring
// internal/lint's own rationale for the identical seam).
type specStateResolver interface {
	Resolve(ctx context.Context, root string, candidate specstate.Candidate) (specstate.Result, error)
	ResolveMany(ctx context.Context, root string, candidates []specstate.Candidate) ([]specstate.Result, error)
}

// unresolvableDefaultBranchMessage restores the D6-6 legible refusal
// (fix-round-1 finding 4) for the specific Unproven cause every operator
// hits most often — the default branch could not be resolved AT ALL — by
// independently re-checking specstate.ResolveDefaultBranch and, if it
// fails, naming every source the resolution chain tries (CI_DEFAULT_BRANCH,
// configured git remote HEAD, the unambiguous local origin/main-or-master
// fallback) plus the `git remote set-head` remedy, exactly as the
// pre-Task-5 gate.go message did. Returns "" when the default branch DOES
// resolve — meaning this Unproven verdict has some OTHER cause (an
// incomplete successor-corpus scan, or an unprovable first-parent landing),
// which the caller reports via specstate's own per-candidate disclosures
// instead; this message is never a substitute for those, only an addition
// for the one cause that has a concrete, actionable remedy.
func unresolvableDefaultBranchMessage(ctx context.Context, root string) string {
	if _, ok := specstate.ResolveDefaultBranch(ctx, root); ok {
		return ""
	}
	// D6-6: name every source resolveDefaultBranchName tries — not just the
	// two GitLab-CI-centric ones — plus the remedy, since this is exactly
	// the message a fresh GitHub checkout hits (GitHub Actions sets no
	// CI_DEFAULT_BRANCH, and actions/checkout never runs `git remote
	// set-head`, so origin/HEAD is unconfigured too).
	return "cannot determine the default branch (no CI_DEFAULT_BRANCH, no configured git remote HEAD, and no single unambiguous local origin/main or origin/master ref) — failing closed; run `git remote set-head origin <branch>` to configure it"
}

// runBuildVerb dispatches `verdi build <subcommand>`. There is exactly one
// subcommand, `start` (05 §CLI); anything else is a usage error.
func runBuildVerb(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "start" {
		// vocab:identity — CLI usage grammar (identity arg placeholders)
		fmt.Fprintln(stderr, "usage: verdi build start <story-spec | story-ref>")
		return 2
	}
	return cmdBuildStart(args[1:], stdout, stderr)
}

// cmdBuildStart is `verdi build start`'s real entry point: it parses the
// single positional argument, resolves the store root and manifest, and
// wires the real runner before delegating to runBuildStart.
func cmdBuildStart(args []string, stdout, stderr io.Writer) int {
	requestPath, rest, err := extractConflictRequestFlag(args)
	if err != nil {
		fmt.Fprintln(stderr, "build start:", err)
		return 2
	}
	args = rest
	if len(args) != 1 {
		// vocab:identity — CLI usage grammar (identity arg placeholders)
		fmt.Fprintln(stderr, "build start: usage: verdi build start <story-spec | story-ref>")
		return 2
	}
	storyArg := args[0]

	ctx := context.Background()
	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "build start:", err)
		return 2
	}
	// store.Open (not the bare loadManifest delegate): build start's
	// verdict lines resolve display vocabulary through Config.Model
	// (spec/vocabulary-surfaces ac-1) — the same single open that already
	// loads the manifest, threaded down via deps.
	cfg, err := store.Open(root)
	if err != nil {
		fmt.Fprintln(stderr, "build start:", err)
		return 2
	}
	manifest := cfg.Manifest

	var runner upstream.Runner
	if manifest.Toolchain != nil {
		runner = upstream.RealRunner{Module: manifest.Toolchain.Module, Commit: manifest.Toolchain.Commit, Dir: root}
	}
	deps := syncDeps{Runner: runner, GoTest: realGoTestRunner{}, Stdout: stdout, Stderr: stderr, Model: cfg.Model}

	return runBuildStartWithConflict(ctx, root, storyArg, specstate.NewProjector(), deps, requestPath, localLifecycleConflictProvider{root: root}, stdout, stderr)
}

// runBuildStart is the testable core: given an already-resolved root, a
// specStateResolver (production: specstate.NewProjector(); tests: real,
// fixturegit-backed, or a fake for shapes real git cannot practically
// reconstruct), and injected deps, run the whole build-start ritual and
// return the exit code. It refuses (exit 1, a verdict failure per
// CLAUDE.md's 0/1/2 contract — a business precondition, not an operational
// problem) before touching git at all when the resolved spec's Git-derived
// effective state is not AcceptedPendingBuild, or carries an unresolved
// rung-4 cascade block, so a refused build start leaves the repo exactly as
// it found it. An effective state that cannot be proven (specstate.Unproven
// — no default branch resolvable, or ancestry/supersession proof
// incomplete) is operational (exit 2): build start cannot honestly decide
// the precondition, so it must not guess either way. A resolved ref that is
// a round-four birds-eye feature spec (class: feature, carrying problem/
// outcome — matrix.go's own two-conjunct discriminator) is likewise an
// operational error (exit 2): a feature spec has no code of its own to
// build against — only its implementing stories do.
func runBuildStart(ctx context.Context, root, storyArg string, resolver specStateResolver, deps syncDeps, stdout, stderr io.Writer) int {
	return runBuildStartWithConflict(ctx, root, storyArg, resolver, deps, "", localLifecycleConflictProvider{root: root}, stdout, stderr)
}

func runBuildStartWithConflict(ctx context.Context, root, storyArg string, resolver specStateResolver, deps syncDeps, requestPath string, provider policyconflict.VerdictProvider, stdout, stderr io.Writer) int {
	spec, err := resolveBuildTarget(root, storyArg, deps.Model)
	if err != nil {
		fmt.Fprintln(stderr, "build start:", err)
		return 2
	}
	if spec.Class == artifact.ClassFeature && spec.Problem != nil {
		// Display resolution (L-M13(1),
		// judged-cli-refusal-prose-class-state-words-still-bare): both
		// class words and their agreeing articles resolve; spec.ID stays
		// identity.
		featureWord := deps.Model.DisplayClass("feature")
		storyWord := deps.Model.DisplayClass("story")
		fmt.Fprintf(stderr, "build start: %s is %s spec (birds-eye, outcome-level); build start operates on %s spec that implements it, not the %s itself\n",
			spec.ID, model.Indefinite(featureWord),
			model.Indefinite(storyWord), featureWord)
		return 2
	}

	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		fmt.Fprintln(stderr, "build start: internal error: resolved spec has an invalid id:", err)
		return 2
	}

	// The acceptance precondition (03 §Gates condition 1's local half) is
	// now Git-derived (internal/specstate), never the persisted status
	// field alone: read the resolved spec's own bytes once (the exact
	// content resolveBuildTarget's storyresolve.Resolve/LoadActiveSpec
	// chain already decoded from), and ask the shared projector what Git
	// says about them — a statusless spec that has landed on the default
	// branch is accepted-pending-build exactly like an explicitly-flagged
	// one (Task 4's compatibility reading), and a spec whose bytes have
	// merely been hand-edited locally to CLAIM acceptance without actually
	// landing is caught by the exact-byte comparison (RelationDiverged),
	// same "never trust the working tree" property the old direct
	// status-field read only achieved for the STATUS FIELD specifically.
	relPath := store.ActiveSpecRelPath(specRef.Name)
	content, err := os.ReadFile(store.ActiveSpecPath(root, specRef.Name))
	if err != nil {
		fmt.Fprintln(stderr, "build start:", err)
		return 2
	}
	result, err := resolver.Resolve(ctx, root, specstate.Candidate{Path: relPath, Content: content})
	if err != nil {
		fmt.Fprintln(stderr, "build start:", err)
		return 2
	}
	switch result.State {
	case specstate.AcceptedPendingBuild:
		// proceed
	case specstate.Superseded:
		// A superseded spec is never re-buildable (D-12): report the
		// successor found via the incoming supersedes chain so the
		// operator is pointed at the spec they should build instead,
		// rather than the generic wrong-state message below. The state
		// WORD resolves through the model (L-M13(1)); the comparison
		// above and s.ID stay bare ids.
		supersededWord := deps.Model.DisplayState(string(spec.Class), "superseded")
		if s, ferr := findSupersedingSpec(root, spec.ID); ferr == nil && s != nil {
			fmt.Fprintf(stderr, "build start: refused: %s is %s by %s; build the successor, not the %s predecessor (03 §The amendment ladder)\n", spec.ID, supersededWord, s.ID, supersededWord)
		} else {
			fmt.Fprintf(stderr, "build start: refused: %s is %s; %s spec is never re-buildable (03 §The amendment ladder)\n", spec.ID, supersededWord, model.Indefinite(supersededWord))
		}
		return 1
	case specstate.Closed:
		closedWord := deps.Model.DisplayState(string(spec.Class), "closed")
		fmt.Fprintf(stderr, "build start: refused: %s is already %s; a build may only reference an accepted spec pending build (03 §Gates)\n", spec.ID, closedWord)
		return 1
	case specstate.Proposed:
		// vocab:identity — the fixed refusal phrase a caller may key on
		// ("proposal has not landed"); the class/state words around it
		// still resolve (L-M13(1)).
		fmt.Fprintf(stderr, "build start: refused: %s's proposal has not landed on the default branch yet (not yet %s); a build may only reference an accepted spec (03 §Gates)\n", spec.ID,
			deps.Model.DisplayState(string(spec.Class), "accepted-pending-build"))
		return 1
	default: // specstate.Unproven
		if msg := unresolvableDefaultBranchMessage(ctx, root); msg != "" {
			fmt.Fprintf(stderr, "build start: %s\n", msg)
			return 2
		}
		fmt.Fprintf(stderr, "build start: %s cannot be proven accepted: %s\n", spec.ID, strings.Join(result.Disclosures, "; "))
		return 2
	}

	if ok, reason, cerr := checkCascadeReaffirmation(root, spec); cerr != nil {
		fmt.Fprintln(stderr, "build start:", cerr)
		return 2
	} else if !ok {
		fmt.Fprintf(stderr, "build start: refused: %s\n", reason)
		return 1
	}

	// Obligation-quality is the final pre-effect build precondition. It runs
	// after acceptance and cascade proof, but before RevParse, branch creation,
	// or baseline work. Feature ACs remain exempt: obligations are story-only.
	if spec.Class == artifact.ClassStory {
		debts, qerr := buildObligationQualityDebts(ctx, root, specRef.Name, spec)
		if qerr != nil {
			fmt.Fprintln(stderr, "build start: obligation quality:", qerr)
			return 2
		}
		if len(debts) > 0 {
			parts := make([]string, len(debts))
			for i, debt := range debts {
				parts[i] = fmt.Sprintf("%s/%s: %s (witness %s)", debt.acID, debt.kind, debt.assessment.StructuralState, debt.assessment.WitnessPath)
				if debt.assessment.Reason != "" {
					parts[i] += " reason=" + string(debt.assessment.Reason)
				}
			}
			fmt.Fprintf(stderr, "build start: refused: obligation quality unresolved: %s\n", strings.Join(parts, "; "))
			return 1
		}
	}

	currentBranch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		fmt.Fprintln(stderr, "build start:", err)
		return 2
	}
	commit, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "build start:", err)
		return 2
	}
	conflict, err := runConflictGate(ctx, root, conflictGateInput{
		RequestPath: requestPath,
		Phase:       contextcompile.PhaseBuild,
		Spec:        spec.ID,
		Branch:      currentBranch,
		Head:        commit,
	}, provider)
	if err != nil {
		fmt.Fprintln(stderr, "build start:", err)
		return 2
	}
	if conflict.Adopted {
		renderConflictSummary(stdout, conflict.Result)
		if conflict.Result.Report.Verdict != policyconflict.VerdictPass {
			return 1
		}
	}

	branch := "feature/" + specRef.Name

	if err := gitx.CheckoutNewBranch(ctx, root, branch); err != nil {
		fmt.Fprintln(stderr, "build start:", err)
		return 2
	}

	regenerateBaseline(ctx, root, commit, spec, deps, "build start", stderr)

	fmt.Fprintf(stdout, "build start: created branch %s from %s (status: %s)\n", branch, spec.ID,
		deps.Model.DisplayState(string(spec.Class), "accepted-pending-build"))
	return 0
}

type buildObligationDebt struct {
	acID       string
	kind       artifact.EvidenceKind
	assessment evidence.ObligationAssessment
}

func buildObligationQualityDebts(ctx context.Context, root, specName string, spec *artifact.SpecFrontmatter) ([]buildObligationDebt, error) {
	var debts []buildObligationDebt
	for _, ac := range spec.AcceptanceCriteria {
		for _, kind := range ac.Evidence {
			assessment, err := evidence.AssessObligation(ctx, evidence.ObligationAssessmentInput{
				StoreRoot: root,
				SpecName:  specName,
				ACID:      ac.ID,
				Kind:      kind,
			})
			if err != nil {
				return nil, fmt.Errorf("%s/%s: %w", ac.ID, kind, err)
			}
			if assessment.StructuralState != evidence.ObligationElaborated {
				debts = append(debts, buildObligationDebt{acID: ac.ID, kind: kind, assessment: assessment})
			}
		}
	}
	sort.SliceStable(debts, func(i, j int) bool {
		if debts[i].acID != debts[j].acID {
			return debts[i].acID < debts[j].acID
		}
		return debts[i].kind < debts[j].kind
	})
	return debts, nil
}

// resolveBuildTarget resolves storyArg (05 §CLI: "<story-spec | story-ref>")
// to a story-grade spec for `verdi build start`, layering a class: story
// fallback ON TOP of storyresolve.Resolve rather than widening that shared
// function itself. Disclosed judgment call (see the phase report): an
// earlier version of this phase widened storyresolve's own story-ref
// matching to also consider class: story specs, but that shared function
// backs several OTHER already-shipped consumers (matrix, rollup, the
// verdict viewer, MCP tools) whose corpus can legitimately carry a class:
// feature spec's OPTIONAL epic/objective story: field and a class: story
// spec's REQUIRED own story: field with the SAME tracker-ref value (no
// reserved-uniqueness rule stops it, and this module's own examples/showcase
// does exactly that: stale-decline, class: feature, and
// borrower-update-api, class: story, both carry story: jira:LOAN-1482) —
// widening the shared resolver silently changed which spec those other
// verbs found, breaking e2e coverage unrelated to this phase. Confining
// the new story-class capability to this one verb keeps every other
// consumer's resolution behavior byte-for-byte unchanged.
//
// Resolution order: (1) storyresolve.Resolve as-is — the spec-ref form,
// and the legacy story-ref-matches-a-class:-feature-spec form, both
// unchanged; (2) only if that fails with the typed no-match outcome
// (storyresolve.UnmatchedStoryRefError: the arg parsed as a valid
// scheme-prefixed story ref but matched no FEATURE), also scan
// specs/active for a class: story spec whose own story: field equals
// storyArg. The discriminant is errors.As on the TYPE — never the message
// text, which used to be string-matched here as control flow and thereby
// pinned that user-facing prose bare by coupling (ledger L-M13a(7)).
//
// mdl resolves the ambiguity refusal's class word (L-M13(1)); nil is safe
// (bare-id fallback).
func resolveBuildTarget(root, storyArg string, mdl *model.Model) (*artifact.SpecFrontmatter, error) {
	spec, err := storyresolve.Resolve(root, storyArg)
	if err == nil {
		return spec, nil
	}
	var unmatched *storyresolve.UnmatchedStoryRefError
	if !errors.As(err, &unmatched) {
		return nil, err
	}

	// The story-class scan is storyresolve.MatchStoryClassRef's (CLAUDE.md:
	// anything two packages need lives in one shared internal/ package —
	// `verdi journey`'s own target resolution needs the same scan). Its
	// failure posture is exactly the one this scan always had: a listing
	// or mid-scan load/decode failure is operational, never the not-found
	// verdict the outer err carries (ADJ-51 finding 1), so a broken store
	// is never mistaken for a missing (story, AC) pair, and no stray dir
	// masks a reachable one.
	matches, serr := storyresolve.MatchStoryClassRef(root, storyArg)
	if serr != nil {
		return nil, serr
	}
	switch len(matches) {
	case 0:
		return nil, err // no story-class match either: surface the original error
	case 1:
		return matches[0], nil
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.ID
		}
		// "story ref" names the scheme-prefixed story: FIELD's ref form
		// (identity, like the usage line's <story-ref>); the second class
		// word speaks the spec's class — display, resolved (L-M13(1)).
		return nil, fmt.Errorf("story ref %q matches more than one active %s spec: %s", storyArg, mdl.DisplayClass("story"), strings.Join(names, ", "))
	}
}
