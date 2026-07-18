// verdi design start [<ref>] --kind feature|story --name <name> (05 §CLI,
// R4-I-1/R4-I-6): cuts the design branch, scaffolds specs/active/<name>/ as
// a draft spec of the chosen class, resolves a story ref's title via the
// provider registry (degrading to the raw ref on any resolution failure —
// 04 §Semantics) when a story ref is given, commits the scaffold, and
// best-effort regenerates the impacted-service baseline (baseline.go).
// Kept in its own file per the lint.go/sync.go/matrix.go/dex.go convention,
// so dispatch.go's diff for wiring this verb in stays a one-line change.
//
// --kind selects the two-scope spec class (02 §Kind registry: feature spec
// vs. story spec); ref optionality follows the class exactly as 05 §CLI
// states it: "--kind feature takes an OPTIONAL tracker ref (features may
// carry no story: at all); --kind story REQUIRES the scheme-prefixed story
// ref". A feature's ref, when given, is an epic/objective tracker ref (02
// §Kind registry's own worked example, `okr:LOAN-Q3`) — the SAME
// scheme-configured-check design start already ran pre-round-four, just no
// longer mandatory.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/provider"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/upstream"
)

// runDesignVerb dispatches `verdi design <subcommand>`. There is exactly
// one subcommand, `start` (05 §CLI); anything else is a usage error.
func runDesignVerb(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "start" {
		fmt.Fprintln(stderr, "usage: verdi design start [<ref>] --kind feature|story --name <name>")
		return 2
	}
	return cmdDesignStart(args[1:], stdout, stderr)
}

// extractFlags pulls "--name"/"-name" and "--kind"/"-kind" (each as a
// separate value token or "=value") out of args in whatever position they
// appear, returning both values and every remaining (positional) argument
// in order. This hand-rolled parse exists for the same reason
// extractNameFlag did pre-round-four: flag.FlagSet stops consuming flags at
// the first non-flag token, so it cannot parse
// "verdi design start <ref> --kind feature --name <name>" — the exact
// ordering 05 §CLI's own example uses, a positional ref leading two
// trailing flags — without also accepting every flag-first permutation.
// This loop accepts both flags, in either form, in any position, relative
// to each other and to the positional ref.
func extractFlags(args []string) (kind, name string, rest []string, err error) {
	take := func(label string, dst *string, i int, a string) (consumed int, err error) {
		if strings.HasPrefix(a, "--"+label+"=") || strings.HasPrefix(a, "-"+label+"=") {
			if *dst != "" {
				return 0, fmt.Errorf("--%s given more than once", label)
			}
			_, *dst, _ = strings.Cut(a, "=")
			return 0, nil
		}
		if i+1 >= len(args) {
			return 0, fmt.Errorf("--%s requires a value", label)
		}
		if *dst != "" {
			return 0, fmt.Errorf("--%s given more than once", label)
		}
		*dst = args[i+1]
		return 1, nil
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--name" || a == "-name":
			n, e := take("name", &name, i, a)
			if e != nil {
				return "", "", nil, e
			}
			i += n
		case strings.HasPrefix(a, "--name=") || strings.HasPrefix(a, "-name="):
			if name != "" {
				return "", "", nil, fmt.Errorf("--name given more than once")
			}
			_, name, _ = strings.Cut(a, "=")
		case a == "--kind" || a == "-kind":
			n, e := take("kind", &kind, i, a)
			if e != nil {
				return "", "", nil, e
			}
			i += n
		case strings.HasPrefix(a, "--kind=") || strings.HasPrefix(a, "-kind="):
			if kind != "" {
				return "", "", nil, fmt.Errorf("--kind given more than once")
			}
			_, kind, _ = strings.Cut(a, "=")
		default:
			rest = append(rest, a)
		}
	}
	return kind, name, rest, nil
}

// designDeps bundles design start's injectable dependencies (mirroring
// syncDeps) so runDesignStart can be driven hermetically in tests
// (CLAUDE.md: no network, no exec in any test); cmdDesignStart wires the
// real ones. Runner is nil when verdi.yaml carries no toolchain: block —
// baseline.go's regenerateBaseline reads that as "skip gracefully",
// never as an error.
type designDeps struct {
	Provider provider.StoryProvider
	Runner   upstream.Runner
	GoTest   goTestRunner
}

// cmdDesignStart is `verdi design start`'s real entry point: it parses
// --kind/--name and the optional positional ref, resolves the store root
// and manifest, wires the real provider registry from verdi.yaml's
// providers: map (buildProviderRegistry — the same construction rollup/sync
// use, so a configured jira ref resolves or degrades for the true reason per
// 04 §Semantics) and runner, and delegates to runDesignStart.
func cmdDesignStart(args []string, stdout, stderr io.Writer) int {
	kindArg, name, rest, err := extractFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, "design start:", err)
		return 2
	}
	if name == "" {
		fmt.Fprintln(stderr, "design start: --name is required (I-10: no magic, no tracker-derived naming)")
		return 2
	}

	var kind artifact.SpecClass
	switch kindArg {
	case "feature":
		kind = artifact.ClassFeature
	case "story":
		kind = artifact.ClassStory
	case "":
		fmt.Fprintln(stderr, "design start: --kind is required (feature|story, 05 §CLI)")
		return 2
	default:
		fmt.Fprintf(stderr, "design start: --kind %q is not feature or story\n", kindArg)
		return 2
	}

	var storyRef string
	switch {
	case len(rest) == 1:
		storyRef = rest[0]
	case len(rest) == 0 && kind == artifact.ClassFeature:
		// A feature's tracker ref is optional (05 §CLI: "features may
		// carry no story: at all") — nothing more to parse.
	case len(rest) == 0 && kind == artifact.ClassStory:
		fmt.Fprintln(stderr, "design start: --kind story requires a scheme-prefixed story ref (e.g. jira:LOAN-1482)")
		return 2
	default:
		fmt.Fprintln(stderr, "design start: usage: verdi design start [<ref>] --kind feature|story --name <name>")
		return 2
	}

	ctx := context.Background()
	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "design start:", err)
		return 2
	}
	cfg, err := store.Open(root)
	if err != nil {
		fmt.Fprintln(stderr, "design start:", err)
		return 2
	}
	manifest := cfg.Manifest

	// Wire the real story-provider registry from verdi.yaml's providers:
	// map — the same construction rollup/sync use (buildProviderRegistry,
	// rollup.go) — so a CONFIGURED scheme (jira) actually attempts
	// resolution and degrades for the true reason (NotFound/Unavailable) per
	// 04 §Semantics, rather than always degrading with ErrUnknownScheme (the
	// D-3 defect: the old empty registry made a well-formed, configured ref
	// and a truly-unconfigured one indistinguishable from design start's
	// point of view). An unconfigured scheme is still rejected earlier, in
	// runDesignStart, via manifest.ConfiguredStorySchemes (the VL-005-shaped
	// error) — it never reaches this registry.
	reg := buildProviderRegistry(manifest)

	var runner upstream.Runner
	if manifest.Toolchain != nil {
		runner = upstream.RealRunner{Module: manifest.Toolchain.Module, Commit: manifest.Toolchain.Commit, Dir: root}
	}
	deps := designDeps{Provider: reg, Runner: runner, GoTest: realGoTestRunner{}}

	return runDesignStart(ctx, root, kind, storyRef, name, manifest, cfg.Model, deps, stdout, stderr)
}

// runDesignStart is the testable core: given an already-resolved root and
// injected deps, run the whole design-start ritual and return the exit
// code. It never partially applies the ritual on failure: a validation
// failure before the branch is cut leaves the repo untouched; baseline
// regeneration failures after the scaffold is committed are disclosed but
// non-fatal (baseline.go), since the baseline is advisory, not the point
// of this verb. storyRef is "" iff kind is ClassFeature and no ref was
// given (05 §CLI's documented optionality) — validated by the caller
// (cmdDesignStart), re-asserted here defensively since this function is
// also driven directly by tests. mdl is the store's already-resolved
// operating model (store.Open's Config.Model): the class switch below
// reads its own scaffold template off mdl.Classes[kind].Template rather
// than a hardcoded filename (spec/scaffold-templates ac-1 cont.).
func runDesignStart(ctx context.Context, root string, kind artifact.SpecClass, storyRef, name string, manifest *store.Manifest, mdl *model.Model, deps designDeps, stdout, stderr io.Writer) int {
	if kind != artifact.ClassFeature && kind != artifact.ClassStory {
		fmt.Fprintf(stderr, "design start: internal error: kind %q is neither feature nor story\n", kind)
		return 2
	}
	if kind == artifact.ClassStory && storyRef == "" {
		fmt.Fprintln(stderr, "design start: --kind story requires a scheme-prefixed story ref (e.g. jira:LOAN-1482)")
		return 2
	}

	specRef, err := artifact.ParseRef("spec/" + name)
	if err != nil {
		fmt.Fprintf(stderr, "design start: --name %q is not a valid spec name: %v\n", name, err)
		return 2
	}

	if storyRef != "" {
		scheme, _, err := provider.ParseStoryRef(provider.StoryRef(storyRef))
		if err != nil {
			fmt.Fprintf(stderr, "design start: story ref %q: %v\n", storyRef, err)
			return 2
		}
		if schemes := manifest.ConfiguredStorySchemes(); !schemes[scheme] {
			fmt.Fprintf(stderr, "design start: story ref %q uses scheme %q, which verdi.yaml's providers: block does not configure\n", storyRef, scheme)
			return 2
		}
	}

	specDir := store.ActiveSpecDir(root, name)
	if _, statErr := os.Stat(specDir); statErr == nil {
		fmt.Fprintf(stderr, "design start: %s already exists\n", specDir)
		return 2
	}

	branch := "design/" + name
	if err := gitx.CheckoutNewBranch(ctx, root, branch); err != nil {
		fmt.Fprintln(stderr, "design start:", err)
		return 2
	}

	var title string
	if storyRef != "" {
		title = resolveStoryTitle(ctx, deps.Provider, storyRef, stderr)
	} else {
		title = designscaffold.HumanizeName(name)
	}

	// The scaffold template is no longer a Go switch on class name: both
	// design start and the workbench's stub-instantiate action resolve it
	// through the same seam, reading Class.Template off the store's
	// already-resolved model (spec/scaffold-templates ac-1 cont.) — a
	// store override at .verdi/templates/<name>.md wins over the embedded
	// canonical default when present (LoadTemplate).
	class, ok := mdl.Classes[string(kind)]
	if !ok {
		fmt.Fprintf(stderr, "design start: internal error: resolved model has no %q class\n", kind)
		return 2
	}
	tmpl, err := designscaffold.LoadTemplate(root, class.Template)
	if err != nil {
		fmt.Fprintln(stderr, "design start:", err)
		return 2
	}

	var content string
	if kind == artifact.ClassFeature {
		content, err = designscaffold.Feature(tmpl, specRef.String(), storyRef, title)
	} else {
		// design start's own placeholder edge: an implements edge to a
		// not-yet-real feature/AC pair, since 05 §CLI names no --feature
		// flag (nothing about which feature a story belongs to is knowable
		// at scaffold time) — the design-branch edit is where a human or
		// agent replaces it with a real edge into the accepted feature this
		// story implements.
		links := []designscaffold.StoryLink{{Type: artifact.LinkImplements, Ref: "spec/todo-replace-feature-name#ac-1"}}
		content, err = designscaffold.Story(tmpl, specRef.String(), storyRef, title, false, links)
	}
	if err != nil {
		fmt.Fprintf(stderr, "design start: rendering template %s for class %s: %v\n", class.Template, kind, err)
		return 2
	}

	// Self-validate before writing anything to disk (CLAUDE.md: "never
	// fake success") — a scaffold that cannot round-trip through the same
	// strict decode/validate every other verb uses is an internal bug,
	// not a user-facing state.
	fm, _, splitErr := artifact.SplitFrontmatter([]byte(content))
	if splitErr != nil {
		fmt.Fprintln(stderr, "design start: internal error: scaffold failed self-validation:", splitErr)
		return 2
	}
	spec, decodeErr := artifact.DecodeSpec(fm)
	if decodeErr != nil {
		fmt.Fprintln(stderr, "design start: internal error: scaffold failed self-validation:", decodeErr)
		return 2
	}

	if err := os.MkdirAll(specDir, 0o755); err != nil {
		fmt.Fprintln(stderr, "design start:", err)
		return 2
	}
	// internal/atomicfile.Write (MkdirAll + CreateTemp + fsync +
	// Rename-into-place), never a plain os.WriteFile (truncate-then-write,
	// no fsync) — the same crash-durability primitive every other corpus
	// write in this repo now shares (CLEANUP-BEFORE #1).
	if err := atomicfile.Write(filepath.Join(specDir, "spec.md"), []byte(content), 0o644); err != nil {
		fmt.Fprintln(stderr, "design start:", err)
		return 2
	}

	if err := gitx.AddAll(ctx, root); err != nil {
		fmt.Fprintln(stderr, "design start:", err)
		return 2
	}
	// L-M13(1) classification: commit subjects are history — identity; the
	// kind here is the --kind flag's enum VALUE, bare.
	msg := fmt.Sprintf("design start: scaffold %s (%s spec, no tracker ref)", specRef.String(), kind)
	if storyRef != "" {
		msg = fmt.Sprintf("design start: scaffold %s (%s spec, story %s)", specRef.String(), kind, storyRef)
	}
	headCommit, err := gitx.CreateCommit(ctx, root, msg)
	if err != nil {
		fmt.Fprintln(stderr, "design start:", err)
		return 2
	}

	regenerateBaseline(ctx, root, headCommit, spec, syncDeps{Runner: deps.Runner, GoTest: deps.GoTest, Stdout: stdout, Stderr: stderr}, "design start", stderr)

	fmt.Fprintf(stdout, "design start: created branch %s\n", branch)
	// L-M13(1) classification: "(kind: %s, status: draft)" ECHOES the
	// scaffold's literal frontmatter — the --kind enum value and the
	// status: field value just written to disk — identity, bare (unlike
	// accept's transition VERDICT, which resolves display state words).
	fmt.Fprintf(stdout, "design start: scaffolded %s (kind: %s, status: draft)\n", specRef.String(), kind)
	fmt.Fprintf(stdout, "design start: board: http://%s/board/spec/%s (run `verdi serve` from this checkout)\n", defaultWorkbenchAddr, name)
	return 0
}

// resolveStoryTitle resolves storyRef's title through prov, degrading to
// the raw ref on any failure (04 §Semantics: "On failure, degrade to
// displaying the raw ref; never block rendering") — NotFound, Unavailable,
// or (the common case today, no real adapter registered yet) ErrUnknownScheme
// all take the same honest, disclosed path.
func resolveStoryTitle(ctx context.Context, prov provider.StoryProvider, storyRef string, stderr io.Writer) string {
	if prov == nil {
		return storyRef
	}
	story, err := prov.Resolve(ctx, provider.StoryRef(storyRef))
	if err != nil {
		fmt.Fprintf(stderr, "design start: story title resolution degraded to the raw ref %q: %v\n", storyRef, err)
		return storyRef
	}
	if story.Title == "" {
		return storyRef
	}
	return story.Title
}

// The scaffold-rendering core (feature/story markdown content,
// HumanizeName) has moved to internal/designscaffold (CLAUDE.md: two
// consumers — this verb and the workbench's stub-instantiate board
// action — share one internal/ home; cmd stays thin).
