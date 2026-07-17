// verdi attest <story-ref> <ac-id> (05 §CLI; spec/attest-helper ac-1..ac-4;
// spec/closure-ergonomics ac-2): given a (story, AC) pair, scaffolds a
// correctly-slugged, correctly-placed attestation skeleton at the exact
// path internal/evidence's fold reads (I-6/I-31) — frontmatter complete
// except for the claim, which is left in an explicit, machine-recognizable
// UNAUTHORED state (evidence.UnauthoredAttestationMarker). Refuses outright
// (exit 1, verdict) when the pair does not exist or an attestation already
// sits at the path; every other failure is operational (exit 2). Writes
// exactly one file to the working tree and commits nothing (dc-2, co-2):
// an attestation is authored once, in place, before its first commit — not
// a multi-commit design surface the way a draft spec is.
//
// Kept in its own file per the lint.go/sync.go/matrix.go/dex.go convention,
// so dispatch.go's diff for wiring this verb in stays a one-line change.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// cmdAttest is `verdi attest`'s entry point, invoked by dispatch.go. Its
// own argument-shape validation runs before any store root is resolved
// (mirroring matrix.go/design.go's own usage-first posture), so a bare or
// malformed invocation fails fast and identically regardless of cwd.
func cmdAttest(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "usage: verdi attest <story-ref> <ac-id>")
		return 2
	}
	storyRefArg, acID := args[0], args[1]

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "attest:", err)
		return 2
	}

	return runAttest(context.Background(), root, storyRefArg, acID, stdout, stderr)
}

// runAttest is the testable core: given an already-resolved store root,
// run the whole scaffold ritual and return the exit code (co-2: 0 clean /
// 1 verdict / 2 operational, dc-5's exact mapping).
func runAttest(ctx context.Context, root, storyRefArg, acID string, stdout, stderr io.Writer) int {
	spec, refusal := classifyPair(root, storyRefArg, acID)
	if refusal != "" {
		fmt.Fprintln(stderr, "attest:", refusal)
		return 1
	}

	storySlug := store.RefSlug(spec.Story)

	// The pre-check (ac-2's "check"): a nice, specific error on the common
	// case. The atomic O_CREATE|O_EXCL open below (ac-2's "write") is the
	// actual race-safety backstop for a file appearing in between — dc-2's
	// "never overwrite a human record" made mechanically race-safe (I-12).
	exists, err := attestationAlreadyExists(root, storySlug, acID)
	if err != nil {
		fmt.Fprintln(stderr, "attest:", err)
		return 2
	}
	if exists {
		fmt.Fprintf(stderr, "attest: an attestation already exists at %s — nothing written\n", attestationPath(root, storySlug, acID))
		return 1
	}

	head, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "attest:", err)
		return 2
	}

	content := evidence.RenderAttestationScaffold(evidence.AttestationScaffold{
		StorySlug:   storySlug,
		ACID:        acID,
		StoryRefArg: storyRefArg,
		VerifiesRef: spec.ID,
		Owners:      spec.Owners,
		Frozen:      artifact.Frozen{At: time.Now().UTC().Format("2006-01-02"), Commit: head},
	})

	// Self-validate the exact bytes before ever touching disk (AC-4;
	// CLAUDE.md: "never fake success") — the same pre-write posture
	// `design start`/stub-instantiate/obligation-graduate all wear.
	fm, _, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		fmt.Fprintln(stderr, "attest: internal error: scaffold failed self-validation:", err)
		return 2
	}
	if _, err := artifact.DecodeAttestation(fm); err != nil {
		fmt.Fprintln(stderr, "attest: internal error: scaffold failed self-validation:", err)
		return 2
	}

	path := attestationPath(root, storySlug, acID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintln(stderr, "attest:", err)
		return 2
	}

	// Atomic create-only write (I-12's own O_CREATE|O_EXCL idiom, mirroring
	// internal/filelock.Acquire): a file appearing at path between the
	// check above and this Open is caught by the OS, never silently
	// overwritten.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			fmt.Fprintf(stderr, "attest: an attestation already exists at %s — nothing written\n", path)
			return 1
		}
		fmt.Fprintln(stderr, "attest:", err)
		return 2
	}
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		fmt.Fprintln(stderr, "attest:", err)
		return 2
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		fmt.Fprintln(stderr, "attest:", err)
		return 2
	}

	fmt.Fprintf(stdout, "attest: scaffolded %s\n", path)
	fmt.Fprintln(stdout, "attest: unauthored — replace the marker with your own first-person claim before this AC can fold as evidenced")
	return 0
}

// classifyPair resolves storyRefArg and acID against root's active store,
// returning the resolved story spec on success or a non-empty, human-
// readable refusal reason on any of AC-2's three "(story, AC) pair does
// not exist" shapes: the story-ref does not resolve, the resolved spec is
// not class: story (dc-5's scope boundary — a feature is reachable via the
// same argument form but names no STORY to attest an AC against), or the
// resolved story does not declare acID. Every one of these is the SAME
// verdict outcome (dc-5: even an unresolvable ref is grouped under exit 1
// here, a disclosed divergence from matrix's own exit-2 posture for the
// identical resolution failure) — never operational — so this function
// never itself distinguishes them by exit code; the caller applies exit 1
// uniformly to any non-empty refusal.
//
// Resolution reuses resolveBuildTarget (buildstart.go), NOT
// storyresolve.Resolve directly: storyresolve.Resolve's own scheme-
// prefixed-story-ref path (matchStoryRef) is deliberately, permanently
// feature-class-only (its own doc comment — unchanged even by round four,
// which gave stories their own story: field too), so a bare `jira:LOAN-1482`
// argument can NEVER resolve to a class: story spec through it alone, only
// to a class: feature spec that happens to share the ref. `verdi build
// start` solved this identical problem already: resolveBuildTarget layers
// a class: story fallback ON TOP of storyresolve.Resolve rather than
// widening that shared function (which backs matrix/rollup/MCP tools whose
// corpora can legitimately have a feature's OPTIONAL epic ref and a
// story's REQUIRED own ref collide on the same tracker key, e.g. this
// module's own stale-decline/borrower-update-api both carrying
// jira:LOAN-1482 — widening the shared resolver would silently change what
// those other verbs find). Reusing that same helper here — rather than
// duplicating its fallback scan — is the CLAUDE.md "no copy-paste" rule
// applied within one package.
func classifyPair(root, storyRefArg, acID string) (spec *artifact.SpecFrontmatter, refusal string) {
	spec, err := resolveBuildTarget(root, storyRefArg)
	if err != nil {
		return nil, err.Error()
	}
	if spec.Class != artifact.ClassStory {
		return nil, fmt.Sprintf("%s resolves to a %s-class spec, not a story — verdi attest scaffolds STORY attestations only (dc-5)", storyRefArg, spec.Class)
	}
	for _, ac := range spec.AcceptanceCriteria {
		if ac.ID == acID {
			return spec, ""
		}
	}
	return nil, fmt.Sprintf("%s does not declare acceptance criterion %q", spec.ID, acID)
}

// attestationPath is the exact path internal/evidence's fold reads for
// (storySlug, acID) — attestations/<storySlug>/<acID>.md (I-6/I-31) — the
// single source both the already-exists check and the final write share.
func attestationPath(root, storySlug, acID string) string {
	return filepath.Join(root, ".verdi", "attestations", storySlug, acID+".md")
}

// attestationAlreadyExists is AC-2's other refusal predicate, checked at
// the exact fold path, exercised directly by attest_test.go's static
// register.
func attestationAlreadyExists(root, storySlug, acID string) (bool, error) {
	_, err := os.Stat(attestationPath(root, storySlug, acID))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
