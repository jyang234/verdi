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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
	"github.com/jyang234/verdi/internal/upstream"
)

// runBuildVerb dispatches `verdi build <subcommand>`. There is exactly one
// subcommand, `start` (05 §CLI); anything else is a usage error.
func runBuildVerb(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "start" {
		fmt.Fprintln(stderr, "usage: verdi build start <story-spec | story-ref>")
		return 2
	}
	return cmdBuildStart(args[1:], stdout, stderr)
}

// cmdBuildStart is `verdi build start`'s real entry point: it parses the
// single positional argument, resolves the store root and manifest, and
// wires the real runner before delegating to runBuildStart.
func cmdBuildStart(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
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
	manifest, err := loadManifest(root)
	if err != nil {
		fmt.Fprintln(stderr, "build start:", err)
		return 2
	}

	var runner upstream.Runner
	if manifest.Toolchain != nil {
		runner = upstream.RealRunner{Module: manifest.Toolchain.Module, Commit: manifest.Toolchain.Commit, Dir: root}
	}
	deps := syncDeps{Runner: runner, GoTest: realGoTestRunner{}, Stdout: stdout, Stderr: stderr}

	return runBuildStart(ctx, root, storyArg, deps, stdout, stderr)
}

// runBuildStart is the testable core: given an already-resolved root and
// injected deps, run the whole build-start ritual and return the exit
// code. It refuses (exit 1, a verdict failure per CLAUDE.md's 0/1/2
// contract — a business precondition, not an operational problem) before
// touching git at all when the resolved spec is not accepted-pending-build
// or carries an unresolved rung-4 cascade block, so a refused build start
// leaves the repo exactly as it found it. A resolved ref that is a
// round-four birds-eye feature spec (class: feature, carrying problem/
// outcome — matrix.go's own two-conjunct discriminator) is an operational
// error (exit 2): a feature spec has no code of its own to build against —
// only its implementing stories do.
func runBuildStart(ctx context.Context, root, storyArg string, deps syncDeps, stdout, stderr io.Writer) int {
	spec, err := resolveBuildTarget(root, storyArg)
	if err != nil {
		fmt.Fprintln(stderr, "build start:", err)
		return 2
	}
	if spec.Class == artifact.ClassFeature && spec.Problem != nil {
		fmt.Fprintf(stderr, "build start: %s is a feature spec (birds-eye, outcome-level); build start operates on a story spec that implements it, not the feature itself\n", spec.ID)
		return 2
	}
	// A superseded spec is never re-buildable (D-12): report the successor
	// found via the incoming supersedes chain so the operator is pointed at
	// the spec they should build instead, rather than the generic
	// wrong-status message below.
	if spec.Status == "superseded" {
		if s, ferr := findSupersedingSpec(root, spec.ID); ferr == nil && s != nil {
			fmt.Fprintf(stderr, "build start: refused: %s is superseded by %s; build the successor, not the superseded predecessor (03 §The amendment ladder)\n", spec.ID, s.ID)
		} else {
			fmt.Fprintf(stderr, "build start: refused: %s is superseded; a superseded spec is never re-buildable (03 §The amendment ladder)\n", spec.ID)
		}
		return 1
	}
	if spec.Status != "accepted-pending-build" {
		fmt.Fprintf(stderr, "build start: %s status is %q, not accepted-pending-build; a build may only reference an accepted spec (03 §Gates)\n", spec.ID, spec.Status)
		return 1
	}

	if ok, reason, cerr := checkCascadeReaffirmation(root, spec); cerr != nil {
		fmt.Fprintln(stderr, "build start:", cerr)
		return 2
	} else if !ok {
		fmt.Fprintf(stderr, "build start: refused: %s\n", reason)
		return 1
	}

	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		fmt.Fprintln(stderr, "build start: internal error: resolved spec has an invalid id:", err)
		return 2
	}
	branch := "feature/" + specRef.Name

	commit, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "build start:", err)
		return 2
	}
	if err := gitx.CheckoutNewBranch(ctx, root, branch); err != nil {
		fmt.Fprintln(stderr, "build start:", err)
		return 2
	}

	regenerateBaseline(ctx, root, commit, spec, deps, "build start", stderr)

	fmt.Fprintf(stdout, "build start: created branch %s from %s (status: accepted-pending-build)\n", branch, spec.ID)
	return 0
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
// unchanged; (2) only if that fails with "no active feature spec has
// story" (i.e. the arg parsed as a valid scheme-prefixed story ref but
// matched no FEATURE), also scan specs/active for a class: story spec
// whose own story: field equals storyArg.
func resolveBuildTarget(root, storyArg string) (*artifact.SpecFrontmatter, error) {
	spec, err := storyresolve.Resolve(root, storyArg)
	if err == nil {
		return spec, nil
	}
	if !strings.Contains(err.Error(), "no active feature spec has story") {
		return nil, err
	}

	dir := filepath.Join(root, ".verdi", "specs", "active")
	entries, rerr := os.ReadDir(dir)
	if rerr != nil {
		// A listing failure here is operational, not the not-found verdict
		// the outer err carries (ADJ-51 finding 1): surface it as such so a
		// caller keying exit discipline on the type (verdi attest) does not
		// mistake a broken store for a missing (story, AC) pair.
		return nil, &storyresolve.OperationalError{Err: fmt.Errorf("listing %s: %w", dir, rerr)}
	}

	var matches []*artifact.SpecFrontmatter
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate, lerr := storyresolve.LoadActiveSpec(root, e.Name())
		if lerr != nil {
			// Same posture as matchStoryRef's own scan (ADJ-51 finding 1): a
			// dir under active/ that cannot be loaded mid-scan is store
			// corruption, operational — never a "(story, AC) does not exist"
			// verdict, and never a stray dir masking a reachable pair.
			return nil, &storyresolve.OperationalError{Err: lerr}
		}
		if candidate.Class == artifact.ClassStory && candidate.Story == storyArg {
			matches = append(matches, candidate)
		}
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
		return nil, fmt.Errorf("story ref %q matches more than one active story spec: %s", storyArg, strings.Join(names, ", "))
	}
}
