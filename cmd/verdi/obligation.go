// verdi obligation author <story-ref> <ac-id> <kind> (spec/obligation-seam
// ac-5, spec/creation-surfaces#ac-4, ledger L-N8 §12 addendum): the
// design-branch, PRE-FREEZE authoring/regeneration surface for an
// evidence-obligation artifact, sharing the identical shared renderer seam
// (internal/evidence.RenderObligation / WriteObligationFile, O-5)
// accept's freeze-moment backstop (acceptobligation.go) calls. Unlike the
// board's own sticky-graduate action (create-only, refuses on any existing
// file — internal/workbench/obligationauthor.go) and unlike accept's own
// backstop (skip-not-overwrite, an honest disclosed placeholder), this
// verb creates OR regenerates: given a declared (story, ac) pair and a
// known evidence kind, it always writes an unauthored scaffold — UNLESS
// the target is already frozen by a merge to main, in which case it
// refuses outright (exit 2), naming the path. "Frozen" is decided the
// same way VL-010 scopes immutability (internal/lint/vl010.go): reachable
// from merge-base(HEAD, default branch), never merely "exists on the
// current branch" — a frozen obligation is superseded through the normal
// ladder like any other frozen artifact, never refined in place.
//
// Kept in its own file per the lint.go/sync.go/matrix.go/dex.go/attest.go
// convention, so dispatch.go's diff for wiring this verb in stays a
// one-line change.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// obligationFrozenProbeBase resolves the commit `verdi obligation author`'s
// frozen check probes an obligation against, with the three-way discrimination
// ac-5 needs (judged-frozen-check-fail-open) — drawn HERE at the call site
// rather than inherited from lint.BuildContext, which deliberately collapses a
// merge-base failure into an empty DiffBase for its own consumers' contracts
// (this fix must not change that seam's behavior):
//
//   - the default branch cannot be resolved at all -> ("", false, nil): the
//     disclosed hermetic "can't prove frozen, proceed" posture (§Ac 5), for
//     the no-configured-remote layouts every verb on this seam tolerates.
//   - the default branch resolves but merge-base(HEAD, it) fails OPERATIONALLY
//     -> ("", true, err): the base is unknowable because git failed, NOT a
//     proven absence — the caller must refuse rather than guess and overwrite
//     what a merge to main may have frozen (the round-1 fix already refuses on
//     a Show/ls-tree error AFTER a base resolved; this closes the sibling gap
//     one seam up, at base COMPUTATION).
//   - the default branch resolves and a merge-base is found -> (base, false,
//     nil); or there is genuinely no common ancestor -> ("", false, nil), a
//     clean git exit-1 negative (nothing is reachable from a base that does
//     not exist, so proceed, never refuse). Both take the ordinary path.
//
// A package var so a test can inject the operational-merge-base-failure case a
// clean fixture repo cannot deterministically produce; production resolves the
// default branch through the shared internal/specstate machinery every other
// Git-derived-state consumer in this package now routes through (Task 5) —
// specstate.ResolveDefaultBranch, not the bare-name lint.ResolveDefaultBranch
// compatibility wrapper — so the merge-base computation below always runs
// against a git-RESOLVABLE ref (Branch.Ref: a local branch name when one is
// checked out, otherwise "origin/<name>"), never a bare name that silently
// fails to resolve on a fresh checkout carrying only an origin/<name>
// remote-tracking ref and no local branch of that name (the same gap
// specstate.Branch's two-field shape exists to close, gate.go's condition 1
// closed identically). The hermetic "no default branch at all" case stays
// byte-identical; this discriminates only the merge-base COMPUTATION once a
// branch resolves.
//
// fix-round-1 finding 3: specstate.ResolveDefaultBranch's single boolean
// collapses TWO different "not ok" shapes that this probe's own three-way
// discrimination must NOT conflate — judged-frozen-check-fail-open exists
// precisely to keep them apart:
//
//   - no default-branch NAME resolves at all (no CI_DEFAULT_BRANCH, no
//     configured origin/HEAD, no unambiguous local origin/main or
//     origin/master) — genuinely no signal to act on; the disclosed
//     hermetic "can't prove frozen, proceed" posture (§Ac 5), unchanged.
//   - a NAME resolves (e.g. CI_DEFAULT_BRANCH=main is configured) but
//     specstate's own further check — a local branch of that name, else
//     an origin/<name> remote-tracking ref — finds NEITHER (main
//     configured but never fetched, the shape a shallow/partial clone
//     leaves behind): this is NOT "no signal at all"; a default branch IS
//     configured, so silently falling through to the hermetic proceed
//     posture would be the exact fail-open gap this seam exists to close.
//     Refuses operationally instead, naming the branch and the fetch
//     remedy — mirroring the pre-Task-5 behavior, where the bare name was
//     handed straight to gitx.MergeBaseCommit and git's own "unknown
//     revision" error surfaced as this same operational refusal.
//
// The name-only half is re-derived here from exactly the two
// ref-resolution-INDEPENDENT sources specstate.ResolveDefaultBranch's own
// name resolution consults first (defaultbranch.go's
// resolveDefaultBranchName, unexported — this fix's own scope forbids
// widening specstate's public API to reach it directly, so the two
// cheap, already-exported primitives it is built from are re-read here
// instead): the CI_DEFAULT_BRANCH environment variable, then
// gitx.DefaultBranch (the configured remote's origin/HEAD symbolic ref).
// The THIRD source, the D6-6 local origin/main-or-master fallback, is
// deliberately NOT re-checked here: that fallback only ever names a
// branch AFTER confirming its own origin/<name> remote-tracking ref
// exists (gitx.HasRemoteTrackingBranch), so whenever it would find a
// name, specstate.ResolveDefaultBranch's ref-resolution succeeds too and
// this whole branch is unreachable — the fallback's name and ref
// resolution are inseparable, unlike the first two sources.
var obligationFrozenProbeBase = func(ctx context.Context, root string) (base string, operationalFailure bool, err error) {
	branch, ok := specstate.ResolveDefaultBranch(ctx, root)
	if !ok {
		name := os.Getenv("CI_DEFAULT_BRANCH")
		if name == "" {
			if b, derr := gitx.DefaultBranch(ctx, root); derr == nil && b != "" {
				name = b
			}
		}
		if name == "" {
			// No default-branch signal at all: the disclosed hermetic
			// "can't prove frozen, proceed" posture (§Ac 5), byte-identical
			// to before.
			return "", false, nil
		}
		// A name resolved but neither a local branch nor an origin/<name>
		// remote-tracking ref exists for it — refuse rather than silently
		// proceed as though nothing were configured.
		return "", true, fmt.Errorf("default branch %q is configured but not git-resolvable (no local branch and no origin/%s remote-tracking ref) — fetch it (e.g. `git fetch origin %s`) before authoring/regenerating an obligation", name, name, name)
	}
	base, found, err := gitx.MergeBaseCommit(ctx, root, "HEAD", branch.Ref)
	if err != nil {
		return "", true, err
	}
	if !found {
		return "", false, nil
	}
	return base, false, nil
}

// obligationVerbUsage is the shared usage line for both `verdi obligation`
// subcommands.
const obligationVerbUsage = "usage: verdi obligation <author <story-ref> <ac-id> <kind> | scaffold <story-ref>>"

// runObligationVerb dispatches `verdi obligation <subcommand>`: `author`
// (spec/obligation-seam ac-5, pre-freeze single-pair authoring/
// regeneration) and `scaffold` (Task 7, docs/superpowers/specs/2026-08-01-
// merge-signals-spec-acceptance-design.md — the pre-review, idempotent,
// batch-creation surface that replaces accept's retired freeze-moment
// backstop). Anything else is a usage error.
func runObligationVerb(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, obligationVerbUsage)
		return 2
	}
	switch args[0] {
	case "author":
		return cmdObligationAuthor(args[1:], stdout, stderr)
	case "scaffold":
		return cmdObligationScaffold(args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, obligationVerbUsage)
		return 2
	}
}

// cmdObligationAuthor is `verdi obligation author`'s real entry point: it
// checks the argument shape, resolves the store root, computes the
// merge-base diff base the frozen check needs (mirroring how
// internal/lint/context.go's own BuildContext is the CLI's one seam for
// this), and delegates to runObligationAuthor.
func cmdObligationAuthor(args []string, stdout, stderr io.Writer) int {
	if len(args) != 3 {
		fmt.Fprintln(stderr, "usage: verdi obligation author <story-ref> <ac-id> <kind>")
		return 2
	}
	storyRefArg, acID, kindArg := args[0], args[1], args[2]

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "obligation author:", err)
		return 2
	}

	ctx := context.Background()
	base, operationalFailure, probeErr := obligationFrozenProbeBase(ctx, root)
	if operationalFailure {
		// judged-frozen-check-fail-open: the default branch resolved but the
		// merge-base computation failed operationally, so we cannot prove the
		// target is unfrozen. Refuse rather than guess — never regenerate over
		// what a merge to main may have frozen. (A default branch that cannot
		// be resolved at all is a different case: base is "" and we proceed,
		// the disclosed hermetic posture.)
		fmt.Fprintf(stderr, "obligation author: cannot determine whether the target obligation is already frozen (the merge-base with the default branch failed): %v\n", probeErr)
		return 2
	}
	return runObligationAuthor(ctx, root, storyRefArg, acID, kindArg, base, stdout, stderr)
}

// runObligationAuthor is the testable core: given an already-resolved
// store root and diffBase (the caller's own merge-base(HEAD, default
// branch) computation — "" when it cannot be established, matching every
// other git-aware seam's "can't prove it, don't guess" posture, I-14),
// run the whole author/regenerate ritual and return the exit code
// (CLAUDE.md: 0 clean / 1 verdict / 2 operational). An empty diffBase
// means frozen-ness can never be proven, so this verb proceeds as create/
// regenerate rather than refusing — the same disclosed reading
// VL-010 itself wears when DiffBase is unknown.
func runObligationAuthor(ctx context.Context, root, storyRefArg, acID, kindArg, diffBase string, stdout, stderr io.Writer) int {
	kind := artifact.EvidenceKind(kindArg)
	switch kind {
	case artifact.EvidenceStatic, artifact.EvidenceBehavioral, artifact.EvidenceRuntime, artifact.EvidenceAttestation:
	default:
		fmt.Fprintf(stderr, "obligation author: %q is not a known evidence kind (one of static, behavioral, runtime, attestation); fail closed\n", kindArg)
		return 2
	}

	cfg, err := store.Open(root)
	if err != nil {
		fmt.Fprintln(stderr, "obligation author:", err)
		return 2
	}

	// classifyPair (attest.go) is the identical (story-ref, ac-id)
	// resolution `verdi attest` already uses: it resolves storyRefArg via
	// resolveBuildTarget (buildstart.go's own class:story fallback layered
	// on storyresolve.Resolve), confirms the resolved spec is class:
	// story, and confirms it declares acID — refused in attest's own
	// plain-language dc-5 terms, reused here rather than re-implemented.
	spec, refusal, opErr := classifyPair(root, storyRefArg, acID, cfg.Model)
	if opErr != nil {
		fmt.Fprintln(stderr, "obligation author:", opErr)
		return 2
	}
	if refusal != "" {
		fmt.Fprintln(stderr, "obligation author:", refusal)
		return 1
	}

	var declaredKinds []artifact.EvidenceKind
	for _, ac := range spec.AcceptanceCriteria {
		if ac.ID == acID {
			declaredKinds = ac.Evidence
			break
		}
	}
	if !evidenceKindDeclared(declaredKinds, kind) {
		fmt.Fprintf(stderr, "obligation author: %s does not declare %s evidence for %s (declared: %v)\n", spec.ID, kind, acID, declaredKinds)
		return 1
	}

	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		fmt.Fprintf(stderr, "obligation author: internal error: resolved spec has an invalid id: %v\n", err)
		return 2
	}
	specName := specRef.Name

	// The frozen check (spec/obligation-seam ac-5): reachable from
	// merge-base(HEAD, default branch) — the same predicate VL-010 scopes
	// immutability with — never merely "exists on the current branch".
	// gitx.PathExistsAt against an empty store-relative path (relPath, built
	// via store.ObligationPath("", ...) exactly as AttestationPath's own
	// empty-root display-form convention documents) draws the three-way a
	// bare gitx.Show cannot (judged-frozen-check-fail-open): the obligation
	// is committed at diffBase (frozen: refuse), genuinely absent at a
	// resolvable diffBase (not frozen: proceed), or the probe itself failed
	// operationally. On that third case we must NEVER guess "not frozen" and
	// silently regenerate content a merge to main may have frozen — VL-010
	// itself surfaces its own git errors rather than passing on them, so this
	// verb refuses (exit 2) naming the failure. (The already-approved
	// diffBase=="" posture — frozen-ness unprovable because no default branch
	// resolved at all — is upstream of here and unchanged: this discriminates
	// only Show errors after a base resolved.)
	relPath := store.ObligationPath("", specName, acID, string(kind))
	absPath := store.ObligationPath(root, specName, acID, string(kind))
	if diffBase != "" {
		frozen, existsErr := gitx.PathExistsAt(ctx, root, diffBase, relPath)
		if existsErr != nil {
			fmt.Fprintf(stderr, "obligation author: cannot determine whether %s is already frozen (the git probe against the merge-base failed): %v\n", relPath, existsErr)
			return 2
		}
		if frozen {
			// Deliberately avoids the "superseded" status word: this
			// sentence describes the general amendment-ladder mechanism,
			// never prints a spec's own status: value, so no display-chain
			// routing applies here — reworded rather than routed or
			// vocab:identity-marked, since a hardcoded vocabulary word
			// would read wrong for a store that renamed it
			// (TestVocabProseWitness, L-M13a's enumeration rule).
			fmt.Fprintf(stderr, "obligation author: %s is already frozen (reachable from the merge-base with the default branch) — a frozen obligation is replaced through the normal amendment ladder, never refined in place\n", relPath)
			return 2
		}
	}

	head, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "obligation author:", err)
		return 2
	}
	at, err := gitx.CommitDateOnly(ctx, root, head)
	if err != nil {
		fmt.Fprintln(stderr, "obligation author:", err)
		return 2
	}

	id := fmt.Sprintf("obligation/%s--%s--%s", specName, acID, kind)
	verifiesRef := "spec/" + specName
	title := fmt.Sprintf("unauthored obligation scaffold: %s %s %s", storyRefArg, acID, kind)
	content := evidence.RenderObligation(evidence.ObligationInput{
		ID:          id,
		Title:       title,
		ForKind:     kind,
		VerifiesRef: verifiesRef,
		Body:        renderObligationAuthorScaffoldBody(storyRefArg, acID, kind),
		Owners:      spec.Owners, // copied verbatim from the resolved story spec (attest.go's own dc-2 precedent)
		Frozen:      artifact.NewFrozen(at, head),
	})

	if err := evidence.WriteObligationFile(absPath, content); err != nil {
		fmt.Fprintln(stderr, "obligation author:", err)
		return 2
	}

	fmt.Fprintf(stdout, "obligation author: scaffolded %s\n", relPath)
	fmt.Fprintln(stdout, "obligation author: unauthored — replace the marker with a first-person statement of what this evidence must specifically show before this obligation is considered authored")
	return 0
}

// evidenceKindDeclared reports whether kind appears in declared.
func evidenceKindDeclared(declared []artifact.EvidenceKind, kind artifact.EvidenceKind) bool {
	for _, k := range declared {
		if k == kind {
			return true
		}
	}
	return false
}

// obligationAuthorScaffoldBody is the fixed instructional prose every
// `verdi obligation author` scaffold carries — the unauthored marker, then
// prose naming the (story-ref, ac-id, kind) triple and the regenerate
// contract, mirroring evidence.attestationScaffoldBody's own shape
// (internal/evidence/attestations.go) for the CLI's second
// scaffold-and-mark-unauthored verb. The three %s verbs take
// (storyRefArg, acID, kind); the trailing three take the same triple again
// for the literal re-run command.
const obligationAuthorScaffoldBody = "%s\n" +
	"This obligation was scaffolded by `verdi obligation author` for %s's\n" +
	"%s evidence on %s and has not been authored. Replace this entire\n" +
	"paragraph, and delete the marker comment above, with your own\n" +
	"statement of what that evidence must specifically show before this\n" +
	"acceptance criterion can rely on it. Re-running\n" +
	"`verdi obligation author %s %s %s` before this file is frozen by a\n" +
	"merge to main regenerates this scaffold from scratch, discarding any\n" +
	"authoring done in the meantime — the design branch is the safety net\n" +
	"(git diff/checkout), not this verb.\n"

// renderObligationAuthorScaffoldBody renders the body evidence.RenderObligation
// wraps in frontmatter.
func renderObligationAuthorScaffoldBody(storyRefArg, acID string, kind artifact.EvidenceKind) string {
	return fmt.Sprintf(obligationAuthorScaffoldBody, evidence.UnauthoredObligationMarker, storyRefArg, kind, acID, storyRefArg, acID, kind)
}

// cmdObligationScaffold is `verdi obligation scaffold <story-ref>`'s real
// entry point (Task 7, docs/superpowers/specs/2026-08-01-merge-signals-
// spec-acceptance-design.md "Command behavior": "Obligation scaffolding
// that is mechanically derivable from declared acceptance criteria moves
// into proposal validation or an idempotent generation step before
// review"). Resolves the store root/manifest/model and wires the real
// specstate.Projector before delegating to runObligationScaffold.
func cmdObligationScaffold(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: verdi obligation scaffold <story-ref>")
		return 2
	}
	storyRefArg := args[0]

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "obligation scaffold:", err)
		return 2
	}
	cfg, err := store.Open(root)
	if err != nil {
		fmt.Fprintln(stderr, "obligation scaffold:", err)
		return 2
	}

	ctx := context.Background()
	return runObligationScaffold(ctx, root, storyRefArg, specstate.NewProjector(), cfg.Model, stdout, stderr)
}

// runObligationScaffold is the testable core: given an already-resolved
// store root and a specStateResolver (production: specstate.NewProjector();
// tests: a fake, mirroring buildstart.go's own seam), resolve storyRefArg to
// a story-class spec (reusing buildstart.go's resolveBuildTarget — the same
// story-ref/spec-ref resolution `verdi build start` already established,
// I-41's binding pointer: resolve acceptance through the specstate
// projector, never raw status or a merge-base approximation), refuse
// (exit 1, a verdict failure) if that spec's Git-derived effective state is
// anything other than Proposed (spec/obligation-seam's whole point is
// PRE-REVIEW preparation — an already-accepted, superseded, or closed spec
// has nothing left to prepare; a post-merge write here would be exactly the
// deterministic-duplicate-bookkeeping ceremony the design's audit rule
// prohibits), refuse (exit 2, operational) if that state cannot be proven,
// and otherwise scaffold every missing declared (ac, kind) obligation,
// reporting each pair as either newly created or already present so a
// second run's "zero created" result is legible rather than silent.
func runObligationScaffold(ctx context.Context, root, storyRefArg string, resolver specStateResolver, mdl *model.Model, stdout, stderr io.Writer) int {
	spec, err := resolveBuildTarget(root, storyRefArg, mdl)
	if err != nil {
		fmt.Fprintln(stderr, "obligation scaffold:", err)
		return 2
	}
	if spec.Class != artifact.ClassStory {
		// Display resolution (L-M13(1)): the class word and its agreeing
		// article resolve; spec.ID stays identity.
		storyWord := mdl.DisplayClass("story")
		fmt.Fprintf(stderr, "obligation scaffold: %s is not %s spec; obligations are declared only on story specs (dc-3)\n", spec.ID, model.Indefinite(storyWord))
		return 2
	}

	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		fmt.Fprintln(stderr, "obligation scaffold: internal error: resolved spec has an invalid id:", err)
		return 2
	}
	specName := specRef.Name

	relPath := store.ActiveSpecRelPath(specName)
	content, err := os.ReadFile(store.ActiveSpecPath(root, specName))
	if err != nil {
		fmt.Fprintln(stderr, "obligation scaffold:", err)
		return 2
	}
	result, err := resolver.Resolve(ctx, root, specstate.Candidate{Path: relPath, Content: content})
	if err != nil {
		fmt.Fprintln(stderr, "obligation scaffold:", err)
		return 2
	}
	switch result.State {
	case specstate.Proposed:
		// proceed: still under review — pre-merge preparation is exactly
		// this verb's job.
	case specstate.Unproven:
		fmt.Fprintf(stderr, "obligation scaffold: %s cannot be proven still-proposed: %s\n", spec.ID, strings.Join(result.Disclosures, "; "))
		return 2
	default: // AcceptedPendingBuild, Superseded, Closed
		fmt.Fprintf(stderr, "obligation scaffold: refused: %s is already %s; obligation scaffolding is pre-review preparation only, never a post-merge mutation (I-41)\n", spec.ID, result.State)
		return 1
	}

	// Enumerate every declared (ac, kind) pair up front, in declaration
	// order, so the report below can distinguish "created this run" from
	// "already present" even for a pair scaffoldMissingObligations itself
	// silently skips (O-3/O-3b).
	type pair struct {
		acID string
		kind artifact.EvidenceKind
	}
	var pairs []pair
	for _, ac := range spec.AcceptanceCriteria {
		for _, kind := range ac.Evidence {
			pairs = append(pairs, pair{ac.ID, kind})
		}
	}

	created, err := scaffoldMissingObligations(ctx, root, specName, spec, operatorOwner())
	if err != nil {
		fmt.Fprintln(stderr, "obligation scaffold:", err)
		return 2
	}
	createdSet := make(map[string]bool, len(created))
	for _, p := range created {
		createdSet[p] = true
	}
	for _, p := range pairs {
		path := store.ObligationPath(root, specName, p.acID, string(p.kind))
		if createdSet[path] {
			fmt.Fprintf(stdout, "obligation scaffold: %s %s: created %s\n", p.acID, p.kind, path)
		} else {
			fmt.Fprintf(stdout, "obligation scaffold: %s %s: already present %s\n", p.acID, p.kind, path)
		}
	}
	return 0
}

// scaffoldMissingObligations is `verdi obligation scaffold`'s own creation
// core (spec/obligation-seam O-1/O-2/O-3/O-3b/O-4/O-6, moved here from the
// retired accept-time backstop by Task 7): for a story-class spec, it
// scaffolds a stub obligation for every declared (ac, kind) pair with no
// decodable obligation of that kind yet at the EXACT convention path
// (internal/evidence.ObligationKindAt — the same convention-path predicate
// VL-020 itself applies, O-3b), stamping every stub with the CURRENT
// HEAD's own commit and committer date (resolved lazily, only once there
// is actually something to write) and owner (the caller passes
// operatorOwner(), O-6). Fix round 1 correction: the shared
// evidence.RenderObligation seam always emits a `frozen: { at, commit }`
// block — every obligation is "frozen" in the schema's existing,
// unconditional sense (DC-1: "existence is the record", the same
// convention `verdi obligation author` already uses; this task does not
// touch that seam). What changed from the retired accept-time backstop is
// WHICH commit the stamp names: never a pre-baked, not-yet-created
// acceptance commit computed ahead of a status flip that no longer
// happens, only the ordinary current-HEAD "created at" stamp — a proposal
// obligation's stamp names when it was scaffolded, not a claim that the
// content itself is immutable or reviewed. It never overwrites: a pair
// whose own convention path already holds a decodable obligation of that
// kind is skipped outright (O-3). created lists exactly the paths newly
// written this call, in declaration order, for the caller to report —
// created is returned even when err != nil, so a failure partway through
// scaffolding still reports what was written so far. Fix round 1 finding
// 2 (spec/obligation-seam ac-3's own narrowed guarantee): unlike the
// retired accept-time backstop, a failure partway through this loop does
// NOT roll back or unlink whatever was already written — the caller
// reports it via the returned created slice, but the files themselves stay
// on disk. See docs/superpowers/sdd/2026-08-01-merge-signaled-spec-
// acceptance-implementation/task-7-report.md's fix-round-1 addendum for
// the invention-ledger wording and the deliberate reasoning (this is a
// pre-review, untracked-by-git working-tree write; the design branch
// itself — git diff/checkout — is the safety net, exactly like `verdi
// obligation author`'s own existing regenerate posture, never a bespoke
// rollback mechanism duplicated for a second, weaker transaction).
//
// Coverage is keyed on the EXACT path .verdi/obligations/<spec>/<acID>--<kind>.md
// (judged-coverage-predicate-forkind-keying), never decoded for_kind scanned
// over every <acID>--*.md: a decodable obligation misfiled under ANOTHER kind's
// filename neither counts as covering the kind its filename names (the reverse
// direction — else the real convention path is left unscaffolded and VL-020
// reds the frozen story post-merge) nor is silently overwritten (the clobber
// direction). path/id agreement stays VL-011's business at lint time.
//
// Two write-side arms are deliberately stricter than VL-020's existence-only
// check and can only ever refuse where VL-020 would pass: a present-but-
// undecodable file AT a declared pair's convention path (malformed) and a
// decodable obligation occupying that path whose for_kind disagrees with the
// filename (the clobber case) both refuse rather than paper over or
// overwrite — a real, if rare, tree state this verb will not guess about.
//
// spec is the caller's already-decoded, PRE-merge spec — its own
// AcceptanceCriteria/Class fields are all this needs; it is never mutated.
func scaffoldMissingObligations(ctx context.Context, root, specName string, spec *artifact.SpecFrontmatter, owner string) (created []string, err error) {
	if spec.Class != artifact.ClassStory {
		return nil, nil // dc-3: feature (and component) ACs never carry obligations
	}
	specRef := "spec/" + specName

	var frozen artifact.Frozen
	frozenComputed := false

	for _, ac := range spec.AcceptanceCriteria {
		for _, kind := range ac.Evidence {
			path := store.ObligationPath(root, specName, ac.ID, string(kind))

			// Coverage is keyed on the EXACT convention path (VL-020's own
			// predicate) and the obligation there decoding AND declaring the
			// kind its filename names — never decoded for_kind scanned over
			// every <acID>--*.md file (judged-coverage-predicate-forkind-keying).
			forKind, present, kerr := evidence.ObligationKindAt(path)
			if kerr != nil {
				// present-but-undecodable AT the convention path (malformed):
				// refuse rather than clobber it or count it as coverage — a
				// deliberately stricter-than-VL-020 arm that can only refuse
				// where VL-020's existence-only check would pass.
				return created, fmt.Errorf("existing obligation at %s is present but does not decode; refusing to overwrite or ignore it — reconcile it by hand or via VL-011/VL-001: %w", path, kerr)
			}
			if present && forKind == kind {
				continue // O-3/O-3b: a decodable obligation of this kind already sits at its own convention path
			}

			// Not covered. The occupied-path stat guard (clobber direction):
			// if anything already occupies this exact convention path — a
			// decodable obligation whose for_kind disagrees with the
			// filename — refuse rather than clobber the hand-authored file.
			if _, statErr := os.Stat(path); statErr == nil {
				return created, fmt.Errorf("obligation already present at %s but not recognized as covering %s %s evidence — the file's own for_kind disagrees with its filename, a conflicted state to reconcile by hand or via VL-011; refusing to overwrite it", path, ac.ID, kind)
			} else if !os.IsNotExist(statErr) {
				return created, fmt.Errorf("checking obligation path %s: %w", path, statErr)
			}

			if !frozenComputed {
				head, herr := gitx.RevParse(ctx, root, "HEAD")
				if herr != nil {
					return created, fmt.Errorf("scaffolding obligations: resolving HEAD: %w", herr)
				}
				at, aerr := gitx.CommitDateOnly(ctx, root, head)
				if aerr != nil {
					return created, fmt.Errorf("scaffolding obligations: resolving HEAD's committer date: %w", aerr)
				}
				frozen = artifact.NewFrozen(at, head)
				frozenComputed = true
			}

			id := fmt.Sprintf("obligation/%s--%s--%s", specName, ac.ID, kind)
			title := fmt.Sprintf("scaffolded obligation: %s %s evidence", ac.ID, kind)
			content := evidence.RenderObligation(evidence.ObligationInput{
				ID:          id,
				Title:       title,
				ForKind:     kind,
				VerifiesRef: specRef,
				Body:        backstopObligationBody(specRef, ac.ID, kind, ac.Text),
				Owners:      []string{owner},
				Frozen:      frozen,
			})
			if werr := evidence.WriteObligationFile(path, content); werr != nil {
				return created, fmt.Errorf("scaffolding obligation for %s %s: %w", ac.ID, kind, werr)
			}
			created = append(created, path)
		}
	}
	return created, nil
}
