// verdi spec state SPEC_REF (Task 5, merge-signaled spec acceptance): the
// read-only surface over internal/specstate's Git-derived effective-state
// projection — the SAME resolver every activation/eligibility decision in
// this package (build start, gate, obligation author, feature archive,
// feature matrix) routes through, exposed directly so an operator or a
// script can inspect a spec's effective state without inferring it from a
// verdict side-effect. Read-only by construction: it resolves the ref with
// the existing store path helpers, reads the spec's own bytes once, asks
// the projector, and prints the result — no status flip, no stamp write,
// no staging, no commit, no forge call.
//
// Kept in its own file per the lint.go/sync.go/matrix.go/dex.go/attest.go/
// obligation.go convention.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// runSpecVerb dispatches `verdi spec <subcommand>`. There is exactly one
// subcommand, `state` — anything else is a usage error.
func runSpecVerb(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "state" {
		// vocab:identity — CLI usage grammar (identity arg placeholders)
		fmt.Fprintln(stderr, "usage: verdi spec state <spec-ref>")
		return 2
	}
	return cmdSpecState(args[1:], stdout, stderr)
}

// cmdSpecState is `verdi spec state`'s entry point: it validates the
// single positional spec-ref argument, resolves the store root, reads the
// spec's own bytes from whichever zone (active, then archive) currently
// holds it, asks the shared specstate.Projector for its effective state,
// and emits the result as one line of this store's canonical JSON
// (internal/canonjson — sorted keys, deterministic). Every known
// specstate.State — including Unproven, itself a fully-proven verdict
// about provability, not a failure to read anything — exits 0; only an
// argument, read, or Git-plumbing failure exits 2 (CLAUDE.md's 0/1/2
// contract: this verb makes no lifecycle CLAIM of its own, so it has no
// verdict of its own to fail — exit 1 is never reachable here).
func cmdSpecState(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		// vocab:identity — CLI usage grammar (identity arg placeholders)
		fmt.Fprintln(stderr, "spec state: usage: verdi spec state <spec-ref>")
		return 2
	}
	ref, err := artifact.ParseRef(args[0])
	if err != nil || ref.Kind != artifact.KindSpec || ref.Fragment() {
		fmt.Fprintf(stderr, "spec state: %q is not a valid spec/<name> ref\n", args[0])
		return 2
	}

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "spec state:", err)
		return 2
	}

	relPath, content, err := readSpecBytesEitherZone(root, ref.Name)
	if err != nil {
		fmt.Fprintln(stderr, "spec state:", err)
		return 2
	}

	result, err := specstate.NewProjector().Resolve(context.Background(), root, specstate.Candidate{Path: relPath, Content: content})
	if err != nil {
		fmt.Fprintln(stderr, "spec state:", err)
		return 2
	}

	data, err := canonjson.Marshal(result)
	if err != nil {
		fmt.Fprintln(stderr, "spec state:", err)
		return 2
	}
	if _, err := stdout.Write(data); err != nil {
		fmt.Fprintln(stderr, "spec state:", err)
		return 2
	}
	return 0
}

// readSpecBytesEitherZone reads name's spec.md from specs/active/, then
// specs/archive/ (active preferred — storyresolve.LoadSpec's own zone
// order, generalized here to any spec class, not only stories), returning
// its store-relative path (the specstate.Candidate.Path a Result's own
// Baseline.Path echoes back) and raw bytes. Neither zone having it is a
// plain error naming both paths tried.
func readSpecBytesEitherZone(root, name string) (relPath string, content []byte, err error) {
	activePath := store.ActiveSpecPath(root, name)
	data, rerr := os.ReadFile(activePath)
	if rerr == nil {
		return store.ActiveSpecRelPath(name), data, nil
	}
	if !os.IsNotExist(rerr) {
		return "", nil, fmt.Errorf("reading %s: %w", activePath, rerr)
	}

	archivePath := store.ArchiveSpecPath(root, name)
	data, rerr = os.ReadFile(archivePath)
	if rerr != nil {
		if os.IsNotExist(rerr) {
			return "", nil, fmt.Errorf("spec/%s not found at %s or %s", name, activePath, archivePath)
		}
		return "", nil, fmt.Errorf("reading %s: %w", archivePath, rerr)
	}
	return store.SpecRelPath(store.ZoneArchive, name), data, nil
}
