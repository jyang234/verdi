// verdi design start --supersedes spec/<name> --name <new> (05 §CLI,
// I-129/ac-11, spec/uat-round-1; ratified by the owner 2026-09-17, option
// (a)): scaffolds a superseding successor that carries an accepted
// predecessor feature's objects and stubs verbatim, a `supersedes` link,
// and a `supersession:` block classifying every predecessor object
// `carried` — so the untouched scaffold lints clean under VL-015 and an
// author reclassifies rather than reconstructs (02 §Kind registry: "the
// spec is never amended after acceptance: supersession is the only forward
// path"). The one shared operation this verb and the board's later Revise
// action (W3-C, ac-11's board half) both call is internal/supersede's
// Resolve (the predecessor guard) and Compose (the pure byte composition);
// this file supplies the CLI-specific plumbing around them — flag
// grammar, checkout switching (dc-2, retained and disclosed exactly like
// this verb's plain --kind/--name path), and the commit.
//
// Kept in its own file per the accept.go/acceptobligation.go and
// designfromstub.go convention: a related but distinct entry point for the
// same verb, not tangled into design.go's --kind/--name flag flow.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/supersede"
	"github.com/jyang234/verdi/internal/upstream"
)

// extractSupersedeFlags pulls "--supersedes", "--name", and "--kind" out of
// args in whatever position they appear (mirroring extractFlags' own "any
// flag, either form" philosophy, minus the "-x" short spellings extractFlags
// also accepts — --supersedes/--from-stub have never had one, and neither
// does this new form), returning every value, every incompatible flag
// token encountered (--from-stub/--problem/--outcome/--defer-statements —
// design.go's other statement-sourcing/from-stub flags, all mutually
// exclusive with --supersedes since a superseding successor's content is
// wholly determined by its predecessor, never authored or deferred), and
// every remaining (unrecognized) positional argument.
func extractSupersedeFlags(args []string) (supersedesRef, kind, name string, incompatible, rest []string, err error) {
	take := func(label string, dst *string, i int) (consumed int, err error) {
		if *dst != "" {
			return 0, fmt.Errorf("--%s given more than once", label)
		}
		if i+1 >= len(args) {
			return 0, fmt.Errorf("--%s requires a value", label)
		}
		*dst = args[i+1]
		return 1, nil
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--supersedes":
			n, e := take("supersedes", &supersedesRef, i)
			if e != nil {
				return "", "", "", nil, nil, e
			}
			i += n
		case "--name":
			n, e := take("name", &name, i)
			if e != nil {
				return "", "", "", nil, nil, e
			}
			i += n
		case "--kind":
			n, e := take("kind", &kind, i)
			if e != nil {
				return "", "", "", nil, nil, e
			}
			i += n
		case "--from-stub", "--problem", "--outcome", "--defer-statements":
			incompatible = append(incompatible, a)
		default:
			rest = append(rest, a)
		}
	}
	return supersedesRef, kind, name, incompatible, rest, nil
}

// cmdDesignStartSupersede is `verdi design start --supersedes`'s real entry
// point: parses its own flag grammar, validates the --supersedes ref shape
// and the --kind/incompatible-flag constraints, resolves the store root and
// manifest (for the same toolchain-derived runner baseline regeneration
// already uses, buildProviderRegistry's own sibling wiring in
// cmdDesignStart), and delegates to runDesignStartSupersede.
func cmdDesignStartSupersede(args []string, stdout, stderr io.Writer) int {
	supersedesRef, kindArg, name, incompatible, rest, err := extractSupersedeFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, "design start --supersedes:", err)
		return 2
	}
	if len(incompatible) > 0 {
		fmt.Fprintf(stderr, "design start --supersedes: %s cannot be combined with --supersedes (a superseding successor's content is wholly determined by its predecessor)\n", strings.Join(incompatible, ", "))
		return 2
	}
	if len(rest) > 0 {
		fmt.Fprintf(stderr, "design start --supersedes: unrecognized argument %q\n", rest[0])
		return 2
	}
	if name == "" {
		fmt.Fprintln(stderr, "design start --supersedes: --name is required (I-10: no magic, no tracker-derived naming)")
		return 2
	}
	switch kindArg {
	case "", "feature":
		// Supersession is feature-only (02 §Kind registry: story and
		// component classes refuse it) — --kind defaults to feature, and an
		// EXPLICIT --kind feature is accepted too (never an error to name
		// the only legal value).
	default:
		// vocab:identity — CLI usage/flag grammar (--kind's only legal value, identity: mirrors design.go's own --kind %q is not feature or story diagnostic)
		fmt.Fprintf(stderr, "design start --supersedes: --kind %q is not feature (supersession is feature-only, 02 §Kind registry)\n", kindArg)
		return 2
	}

	predRef, err := artifact.ParseRef(supersedesRef)
	if err != nil || predRef.Kind != artifact.KindSpec || predRef.Fragment() || predRef.Pinned() {
		if err == nil {
			err = fmt.Errorf("must be a spec/<name> ref, not a fragment or a pinned ref")
		}
		fmt.Fprintf(stderr, "design start --supersedes: %q is not a valid spec/<name> ref: %v\n", supersedesRef, err)
		return 2
	}

	ctx := context.Background()
	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "design start --supersedes:", err)
		return 2
	}
	cfg, err := store.Open(root)
	if err != nil {
		fmt.Fprintln(stderr, "design start --supersedes:", err)
		return 2
	}

	// The same toolchain-derived runner cmdDesignStart itself wires
	// (design.go) — best-effort baseline regeneration needs it, but a
	// store with no toolchain: block still succeeds (regenerateBaseline
	// treats a nil Runner as "skip gracefully", never an error).
	var runner upstream.Runner
	if cfg.Manifest.Toolchain != nil {
		runner = upstream.RealRunner{Module: cfg.Manifest.Toolchain.Module, Commit: cfg.Manifest.Toolchain.Commit, Dir: root}
	}

	return runDesignStartSupersede(ctx, root, predRef.Name, name, cfg.Model, runner, realGoTestRunner{}, stdout, stderr)
}

// runDesignStartSupersede is the testable core: given an already-resolved
// root, predecessor/successor bare names, the store's resolved operating
// model, and injected baseline-regen dependencies, run the whole
// supersede-scaffold ritual and return the exit code. Mirrors
// runDesignStart's own preparation-boundary shape (design.go: resolve/
// validate everything before the first Git mutation) — predName's guard
// (supersede.Resolve) runs in the CURRENT checkout, before base resolution
// or the checkout switch, exactly like supersede.Resolve's own doc comment
// states it must.
func runDesignStartSupersede(ctx context.Context, root, predName, newName string, mdl *model.Model, runner upstream.Runner, goTest goTestRunner, stdout, stderr io.Writer) int {
	// The successor-side preconditions live in internal/supersede beside
	// the predecessor guard (ValidateSuccessorName), so the board's Revise
	// action reuses the identical checks rather than re-implementing them;
	// this verb keeps its OWN wording for each refusal — the operator typed
	// `--name`, and only the CLI knows that.
	newRef, err := supersede.ValidateSuccessorName(root, newName)
	if err != nil {
		var nerr *supersede.NameError
		switch {
		case errors.As(err, &nerr) && nerr.Reason == supersede.ReasonSuccessorExists:
			fmt.Fprintf(stderr, "design start --supersedes: %s already exists\n", nerr.Path)
		case errors.As(err, &nerr) && nerr.Reason == supersede.ReasonInvalidName:
			fmt.Fprintf(stderr, "design start --supersedes: --name %q is not a valid spec name: %v\n", newName, errors.Unwrap(nerr))
		default:
			fmt.Fprintln(stderr, "design start --supersedes:", err)
		}
		return 2
	}
	specDir := store.ActiveSpecDir(root, newName)

	pred, err := supersede.Resolve(ctx, root, predName, mdl)
	if err != nil {
		fmt.Fprintln(stderr, "design start --supersedes:", err)
		return 2
	}

	// Compose is pure — it needs nothing but the predecessor's bytes — so it
	// belongs on the READ-ONLY side of the preparation boundary, above every
	// Git mutation. Composing after the checkout switch stranded an operator
	// on an empty design/<new> branch they never asked to be on whenever
	// composition failed; a refusal here now leaves the repository exactly
	// as it was. Compose's error already classifies itself, so it is relayed
	// verbatim rather than prefixed a second time.
	composed, err := supersede.Compose(supersede.ComposeInput{
		PredecessorName: predName,
		PredecessorRaw:  pred.Raw,
		SuccessorName:   newName,
	})
	if err != nil {
		fmt.Fprintln(stderr, "design start --supersedes:", err)
		return 2
	}

	// Preparation boundary (mirroring runDesignStart, design.go's own R1/
	// SI-198 comment): everything above is read-only validation. Only now
	// does this verb touch Git — base resolution (dc-7) then the checkout
	// switch (dc-2), both the identical shared helpers runDesignStart's
	// own --kind/--name path uses, so the two paths' disclosure wording can
	// never drift apart.
	baseRef, ok := resolveDesignStartBase(ctx, root, stdout, stderr)
	if !ok {
		return 2
	}

	branch := "design/" + newName
	if !checkoutNewDesignBranch(ctx, root, branch, baseRef, stdout, stderr) {
		return 2
	}

	if err := os.MkdirAll(specDir, 0o755); err != nil {
		fmt.Fprintln(stderr, "design start --supersedes:", err)
		return 2
	}
	// internal/atomicfile.Write (MkdirAll + CreateTemp + fsync +
	// Rename-into-place), matching runDesignStart's own write — never a
	// plain os.WriteFile.
	if err := atomicfile.Write(filepath.Join(specDir, "spec.md"), composed.Content, 0o644); err != nil {
		fmt.Fprintln(stderr, "design start --supersedes:", err)
		return 2
	}

	if err := gitx.AddAll(ctx, root); err != nil {
		fmt.Fprintln(stderr, "design start --supersedes:", err)
		return 2
	}
	msg := fmt.Sprintf("design start: supersede spec/%s as spec/%s", predName, newName)
	headCommit, err := gitx.CreateCommit(ctx, root, msg)
	if err != nil {
		fmt.Fprintln(stderr, "design start --supersedes:", err)
		return 2
	}

	// Compose already self-validated (SplitFrontmatter + DecodeSpec +
	// CheckClass) before returning, so this re-decode cannot fail in
	// practice — guarded anyway rather than trusted blindly, matching this
	// module's "never fake success" posture: regenerateBaseline still needs
	// a *artifact.SpecFrontmatter, and a nil one would panic deep inside it
	// rather than surface a legible error here.
	outFM, _, splitErr := artifact.SplitFrontmatter(composed.Content)
	if splitErr != nil {
		fmt.Fprintln(stderr, "design start --supersedes: internal error: composed successor failed re-validation:", splitErr)
		return 2
	}
	spec, decodeErr := artifact.DecodeSpec(outFM)
	if decodeErr != nil {
		fmt.Fprintln(stderr, "design start --supersedes: internal error: composed successor failed re-validation:", decodeErr)
		return 2
	}

	regenerateBaseline(ctx, root, headCommit, spec, syncDeps{Runner: runner, GoTest: goTest, Stdout: stdout, Stderr: stderr}, "design start", stderr)

	fmt.Fprintf(stdout, "design start: created branch %s\n", branch)
	// vocab:identity — echo of the scaffolded frontmatter's kind; state wording names the Git-derived lifecycle vocabulary (mirrors runDesignStart's own identical line)
	fmt.Fprintf(stdout, "design start: scaffolded %s (kind: %s, state: proposed (derived until merge))\n", newRef.String(), artifact.ClassFeature)
	fmt.Fprintf(stdout, "design start: board: http://%s/board/spec/%s (run `verdi serve` from this checkout)\n", defaultWorkbenchAddr, newName)
	fmt.Fprintf(stdout, "design start: supersedes spec/%s: %d objects carried, 0 amended, 0 removed\n", predName, len(composed.CarriedIDs))
	return 0
}
