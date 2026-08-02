// verdi accept <spec-ref|diagram-ref> (05 §CLI): retired from the
// ordinary human workflow (docs/superpowers/specs/2026-08-01-merge-
// signals-spec-acceptance-design.md, "Command behavior"). Specification
// acceptance is now merge-signaled: merging the reviewed specification
// pull request into the configured default branch accepts the exact
// landed revision (internal/specstate's Git-derived projection) — no
// separate `verdi accept` flip, freeze stamp, obligation scaffold,
// staging, or commit is required or performed.
//
// During this compatibility window, invoking `verdi accept` against a
// spec/<name> ref prints a deprecation notice explaining the
// merge-signaled ritual and performs NO filesystem, index, branch, or
// commit mutation whatsoever — an informational message, never an
// acceptance claim, so it always exits 0 for any well-formed spec ref,
// regardless of the target's class or lifecycle state (there is no more
// precondition left to fail: `verdi spec state` — the read-only Git-
// derived projection, Task 5 — is where eligibility is actually
// determined now, not this command). A malformed argument count, or an
// arg that does not even parse as a spec/diagram ref, is still an
// operational error (exit 2), unchanged from before this retirement.
//
// A diagram/<name> ref is unaffected: diagram acceptance
// (acceptdiagram.go) is a separate, still-mutating ritual this design
// does not retire (spec/proposal-artifact ac-3/dc-2 — a diagram carries
// no ACs or stubs to match against, so it was never part of the
// duplicated specification-acceptance ceremony this task addresses).
//
// Kept in its own file per the lint.go/sync.go/matrix.go/dex.go
// convention.
package main

import (
	"context"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/store"
)

// acceptRetiredNotice is the exact compatibility-window message every
// `verdi accept` invocation against a spec ref prints (docs/superpowers/
// specs/2026-08-01-merge-signals-spec-acceptance-design.md, "Command
// behavior"): merging the reviewed specification pull request into the
// configured default branch is what accepts the exact revision — never
// this command. A fixed, byte-exact protocol string (the task brief's own
// exact-text requirement, asserted verbatim by cmd/verdi/accept_test.go):
// never routed through model.DisplayVerb like an ordinary verdict line —
// doing so would make a vocab-renamed store's compatibility notice no
// longer contain this literal, defeating the one thing a fixed
// compatibility message is for.
// vocab:identity — fixed compatibility-window protocol text, never a display surface
const acceptRetiredNotice = "accept is retired: merge the reviewed specification pull request into the configured default branch to accept this exact revision"

// cmdAccept is `verdi accept`'s entry point, invoked by dispatch.go.
func cmdAccept(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		// vocab:identity — CLI usage/verb-name grammar (identity)
		fmt.Fprintln(stderr, "accept: usage: verdi accept <spec-ref|diagram-ref> (e.g. spec/stale-decline, diagram/loansvc-target-topology)")
		return 2
	}
	specArg := args[0]

	ctx := context.Background()
	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}
	return runAccept(ctx, root, specArg, stdout, stderr)
}

// runAccept is the testable core. For a diagram/<name> ref it dispatches
// unchanged to runAcceptDiagram (a separate, still-mutating ritual this
// retirement does not touch). For a spec/<name> ref it performs the sole
// remaining behavior: print acceptRetiredNotice and return 0 — no read of
// the spec's own status, class, or content, no git operation of any kind.
// A malformed or non-spec/diagram ref is still an operational error
// (exit 2, unchanged from before this retirement).
func runAccept(ctx context.Context, root, specArg string, stdout, stderr io.Writer) int {
	ref, err := artifact.ParseRef(specArg)
	if err != nil || ref.Pinned() || (ref.Kind != artifact.KindSpec && ref.Kind != artifact.KindDiagram) {
		fmt.Fprintf(stderr, "accept: %q is not a spec or diagram ref (want spec/<name> or diagram/<name>, e.g. spec/stale-decline)\n", specArg)
		return 2
	}

	// spec/proposal-artifact ac-3/dc-2: a diagram/... ref still dispatches
	// to its own separate ritual — this retirement is scoped to
	// specification acceptance only.
	if ref.Kind == artifact.KindDiagram {
		return runAcceptDiagram(ctx, root, ref, stdout, stderr)
	}

	fmt.Fprintf(stdout, "accept: %s: %s\n", ref.String(), acceptRetiredNotice)
	return 0
}
