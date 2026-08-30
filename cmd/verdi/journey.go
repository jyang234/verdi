// verdi journey [--json] <feature-or-story-ref> (GLG v3 AC-1,
// .verdi/specs/active/guided-lifecycle-governance-v3/spec.md): the
// read-only CLI surface over internal/journey's projection — the first
// GLG runtime verb. It resolves arg to a target spec/story
// (journey.Projector.Project's own two-form contract, I-30), gathers
// every repository and lifecycle fact AC-1 names, derives blockers,
// principals, and safe actions from the operating-model catalog, and
// prints exactly one line of this store's canonical JSON; the explicit flag
// and legacy no-flag form delegate to these same calls and are byte-identical
// (journey.Canonical — sorted keys, digest-bound). No status flip, no
// stamp write, no staging, no commit, no forge call: GLG DC-1 is explicit
// that a journey record is a projection's output, never lifecycle state
// itself, so removing this verb changes no canonical artifact.
//
// EXIT CLASSIFICATION (CLAUDE.md's 0/1/2 contract): a produced record
// ALWAYS exits 0 — the projection makes no lifecycle claim of its own
// (Project's caller never fails merely because the record it produced
// happens to carry blockers; a blocker is a disclosed FACT the record
// reports, not a verdict this verb renders), so exit 1 is unreachable by
// design, exactly like `verdi spec state` (cmd/verdi/specstate.go)
// before it. Every failure to PRODUCE a record at all — a malformed
// argument count, no resolvable store root, an unresolvable ref
// (including the typed journey.NotFoundError), a component-class
// refusal, or any underlying git/decoding failure — is operational exit
// 2, with a stderr line naming the failure. journeyErr below guarantees
// that line is "journey: "-prefixed exactly once: internal/journey's own
// errors (GatherFacts, Project, decodeTargetSpec, ...) already self-
// prefix with "journey: " (mirroring every other verb's %w chain), while
// store.FindRoot/store.Open failures do not, so a bare Fprintln(stderr,
// "journey:", err) would double the prefix on the journey-package path;
// journeyErr checks first rather than accepting that cosmetic wart.
//
// Kept in its own file per the lint.go/sync.go/specstate.go convention.
package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/jyang234/verdi/internal/journey"
	"github.com/jyang234/verdi/internal/store"
)

// cmdJourney is `verdi journey`'s entry point, invoked by dispatch.go.
// Its own argument-shape validation runs before any store root is
// resolved (mirroring attest.go/matrix.go/design.go's usage-first
// posture), so a bare or malformed invocation fails fast and
// identically regardless of cwd.
func cmdJourney(args []string, stdout, stderr io.Writer) int {
	var ref string
	switch {
	case len(args) == 1 && !strings.HasPrefix(args[0], "-"):
		ref = args[0]
	case len(args) == 2 && args[0] == "--json" && !strings.HasPrefix(args[1], "-"):
		ref = args[1]
	default:
		// vocab:identity — CLI usage grammar (identity arg placeholders)
		fmt.Fprintln(stderr, "usage: verdi journey [--json] <feature-or-story-ref>")
		return 2
	}

	root, err := store.FindRoot(".")
	if err != nil {
		journeyErr(stderr, err)
		return 2
	}

	cfg, err := store.Open(root)
	if err != nil {
		journeyErr(stderr, err)
		return 2
	}

	rec, err := journey.NewProjector().Project(context.Background(), cfg, ref)
	if err != nil {
		journeyErr(stderr, err)
		return 2
	}

	data, err := journey.Canonical(rec)
	if err != nil {
		journeyErr(stderr, err)
		return 2
	}

	if _, err := stdout.Write(data); err != nil {
		journeyErr(stderr, err)
		return 2
	}
	return 0
}

// journeyErr writes err to stderr as one line, guaranteeing the line
// starts with "journey: " exactly once: internal/journey's own errors
// already self-prefix that way (every %w chain in facts.go/project.go
// starts "journey: ..."), so this only ADDS the prefix when it is not
// already present (store.FindRoot/store.Open's own errors, which carry
// no verb prefix of their own) rather than blindly prepending it and
// doubling the prefix on the common case.
func journeyErr(stderr io.Writer, err error) {
	msg := err.Error()
	if !strings.HasPrefix(msg, "journey: ") {
		msg = "journey: " + msg
	}
	fmt.Fprintln(stderr, msg)
}
