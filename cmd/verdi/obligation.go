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

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/store"
)

// runObligationVerb dispatches `verdi obligation <subcommand>`. There is
// exactly one subcommand, `author` — anything else is a usage error.
func runObligationVerb(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "author" {
		fmt.Fprintln(stderr, "usage: verdi obligation author <story-ref> <ac-id> <kind>")
		return 2
	}
	return cmdObligationAuthor(args[1:], stdout, stderr)
}

// cmdObligationAuthor is `verdi obligation author`'s real entry point: it
// checks the argument shape, resolves the store root, computes the
// merge-base diff base the frozen check needs (mirroring how
// internal/lint/context.go's own BuildContext is the CLI's one seam for
// this — acceptlint.go's lintQuartetOrRefuse uses the identical call), and
// delegates to runObligationAuthor.
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
	lctx := lint.BuildContext(ctx, root)
	return runObligationAuthor(ctx, root, storyRefArg, acID, kindArg, lctx.DiffBase, stdout, stderr)
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
	// gitx.Show against an empty store-relative path (relPath, built via
	// store.ObligationPath("", ...) exactly as AttestationPath's own
	// empty-root display-form convention documents) either finds the
	// obligation committed at diffBase (frozen: refuse) or does not
	// (proceed to create/regenerate).
	relPath := store.ObligationPath("", specName, acID, string(kind))
	absPath := store.ObligationPath(root, specName, acID, string(kind))
	if diffBase != "" {
		if _, showErr := gitx.Show(ctx, root, diffBase, relPath); showErr == nil {
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
