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
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
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
	// The resolved operating model (store.Open's config bottleneck, L-M3):
	// attest's refusal prose resolves display class words through
	// Config.Model (L-M13(1)). An unresolvable store is operational (exit
	// 2), matching every other manifest-loading verb's posture.
	cfg, err := store.Open(root)
	if err != nil {
		fmt.Fprintln(stderr, "attest:", err)
		return 2
	}

	return runAttest(context.Background(), root, storyRefArg, acID, cfg.Model, stdout, stderr)
}

// runAttest is the testable core: given an already-resolved store root
// and the resolved display model (nil-safe: bare-id fallback), run the
// whole scaffold ritual and return the exit code (co-2: 0 clean /
// 1 verdict / 2 operational, dc-5's exact mapping).
func runAttest(ctx context.Context, root, storyRefArg, acID string, mdl *model.Model, stdout, stderr io.Writer) int {
	spec, refusal, opErr := classifyPair(root, storyRefArg, acID, mdl)
	if opErr != nil {
		fmt.Fprintln(stderr, "attest:", opErr)
		return 2
	}
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
		// Wall-clock "now" is unchanged here (L-M4 fixed obligationauthor's
		// determinism violation only): AttestationScaffold.Frozen documents
		// this as a deliberate, legally-mutable-until-first-commit
		// convenience (dc-2/ADJ-30), not the frozen-forever stamp the
		// determinism rule targets. Routed through the shared constructor
		// for structural consistency with every other Frozen mint.
		Frozen: artifact.NewFrozen(time.Now().UTC().Format("2006-01-02"), head),
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
// returning exactly one of three outcomes: the resolved story spec on
// success (refusal == "", opErr == nil); a non-empty, human-readable refusal
// reason on any of AC-2's "(story, AC) pair does not exist" VERDICT shapes
// (the story-ref does not resolve, the resolved spec is not class: story —
// including a component spec, re-worded in attest's own terms rather than
// leaking storyresolve's matrix framing, ADJ-51 finding 3 — or the resolved
// story does not declare acID); or a non-nil OPERATIONAL error (opErr) when
// resolution itself fails on machinery — a spec present but unreadable or
// undecodable, or a store listing that fails (ADJ-51 finding 1). The caller
// maps opErr to exit 2 and any non-empty refusal to exit 1: dc-5 groups even
// an unresolvable ref under the verdict (a disclosed divergence from matrix's
// own exit-2 posture for the identical resolution failure), but co-2's exit
// discipline forbids dressing a genuine operational failure as that verdict.
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
func classifyPair(root, storyRefArg, acID string, mdl *model.Model) (spec *artifact.SpecFrontmatter, refusal string, opErr error) {
	spec, err := resolveBuildTarget(root, storyRefArg, mdl)
	if err != nil {
		var oe *storyresolve.OperationalError
		if errors.As(err, &oe) {
			// A spec present-but-unreadable/undecodable, or a store listing
			// that failed, encountered WHILE resolving: operational (exit 2),
			// never a "(story, AC) does not exist" verdict (co-2, dc-5).
			return nil, "", err
		}
		var ce *storyresolve.ComponentSpecError
		if errors.As(err, &ce) {
			// A component spec has no story to attest an AC against — the
			// same verdict as any other non-story class, but re-worded in
			// attest's own terms so the shared resolver's matrix framing does
			// not leak (dc-5, ADJ-51 finding 3).
			//
			// Display resolution (L-M13(1)): the emphatic STORY speaks the
			// class — resolved and upper-cased. "component" stays bare: it
			// is a legacy class id no model can rename (vocabulary classes
			// keys ∈ declared classes ∪ {spike}, L-M13a(5)), and "(no
			// story, no acceptance criteria)" names the story:/
			// acceptance_criteria: FRONTMATTER FIELDS — identity.
			return nil, fmt.Sprintf("%s resolves to a component spec (no story, no acceptance criteria) — no %s exists to attest an AC against (spec/attest-helper dc-5)", storyRefArg, strings.ToUpper(mdl.DisplayClass("story"))), nil
		}
		return nil, err.Error(), nil
	}
	if spec.Class != artifact.ClassStory {
		// Display resolution (L-M13(1)): both class words resolve, with
		// model.Article agreeing on each; the emphatic STORY is the same
		// resolved word upper-cased. The class COMPARISON above stays on
		// the bare id.
		classWord := mdl.DisplayClass(string(spec.Class))
		storyWord := mdl.DisplayClass("story")
		return nil, fmt.Sprintf("%s resolves to %s %s-class spec, not %s %s — no %s exists to attest an AC against (spec/attest-helper dc-5)", storyRefArg,
			model.Article(classWord), classWord,
			model.Article(storyWord), storyWord, strings.ToUpper(storyWord)), nil
	}
	for _, ac := range spec.AcceptanceCriteria {
		if ac.ID == acID {
			return spec, "", nil
		}
	}
	return nil, fmt.Sprintf("%s does not declare acceptance criterion %q", spec.ID, acID), nil
}

// attestationPath is the exact path internal/evidence's fold reads for
// (storySlug, acID) — attestations/<storySlug>/<acID>.md (I-6/I-31) — the
// single source both the already-exists check and the final write share.
func attestationPath(root, storySlug, acID string) string {
	return store.AttestationPath(root, storySlug, acID)
}

// attestationAlreadyExists is AC-2's other refusal predicate, checked at
// the exact fold path, exercised directly by attest_test.go's static
// register. A regular FILE at the path is an existing (human) record —
// AC-2's verdict case. A DIRECTORY at the path is store corruption, not an
// attestation: classified operationally (an error → exit 2), exactly as the
// fold's own readers (evidence.AttestationExists/LoadAttestationState) read
// it, never as an "already exists" verdict for a human record that isn't
// there (ADJ-51 finding 5).
func attestationAlreadyExists(root, storySlug, acID string) (bool, error) {
	path := attestationPath(root, storySlug, acID)
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return false, fmt.Errorf("attestation path %s is a directory, not a file", path)
		}
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
