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
	"github.com/jyang234/verdi/internal/designinterview"
	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/provider"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/upstream"
	"golang.org/x/term"
)

// runDesignVerb dispatches `verdi design <subcommand>`. There is exactly
// one subcommand, `start` (05 §CLI); anything else is a usage error.
func runDesignVerb(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "start" {
		// vocab:identity — CLI usage/flag grammar (--kind enum values, identity)
		fmt.Fprintln(stderr, "usage: verdi design start [<ref>] --kind feature|story --name <name> (or: verdi design start --from-stub <feature> <stub>)")
		return 2
	}
	return cmdDesignStart(args[1:], stdout, stderr)
}

// extractFlags pulls "--name"/"-name", "--kind"/"-kind", "--problem"/
// "-problem", and "--outcome"/"-outcome" (each as a separate value token
// or "=value"), plus the boolean "--defer-statements" (spec/cli-creation
// ac-1, ledger L-N7), out of args in whatever position they appear,
// returning every value and every remaining (positional) argument in
// order. This hand-rolled parse exists for the same reason
// extractNameFlag did pre-round-four: flag.FlagSet stops consuming flags at
// the first non-flag token, so it cannot parse
// "verdi design start <ref> --kind feature --name <name>" — the exact
// ordering 05 §CLI's own example uses, a positional ref leading two
// trailing flags — without also accepting every flag-first permutation.
// This loop accepts every flag, in either form, in any position, relative
// to each other and to the positional ref. --owners is deliberately
// ABSENT from this grammar (spec/cli-creation ac-4, I-10/X-4) —
// TestDesignGo_NoOwnersFlag pins its absence from this whole file's
// source, not merely from this function.
func extractFlags(args []string) (kind, name, problem, outcome string, deferStatements bool, rest []string, err error) {
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
				return "", "", "", "", false, nil, e
			}
			i += n
		case strings.HasPrefix(a, "--name=") || strings.HasPrefix(a, "-name="):
			if name != "" {
				return "", "", "", "", false, nil, fmt.Errorf("--name given more than once")
			}
			_, name, _ = strings.Cut(a, "=")
		case a == "--kind" || a == "-kind":
			n, e := take("kind", &kind, i, a)
			if e != nil {
				return "", "", "", "", false, nil, e
			}
			i += n
		case strings.HasPrefix(a, "--kind=") || strings.HasPrefix(a, "-kind="):
			if kind != "" {
				return "", "", "", "", false, nil, fmt.Errorf("--kind given more than once")
			}
			_, kind, _ = strings.Cut(a, "=")
		case a == "--problem" || a == "-problem":
			n, e := take("problem", &problem, i, a)
			if e != nil {
				return "", "", "", "", false, nil, e
			}
			i += n
		case strings.HasPrefix(a, "--problem=") || strings.HasPrefix(a, "-problem="):
			if problem != "" {
				return "", "", "", "", false, nil, fmt.Errorf("--problem given more than once")
			}
			_, problem, _ = strings.Cut(a, "=")
		case a == "--outcome" || a == "-outcome":
			n, e := take("outcome", &outcome, i, a)
			if e != nil {
				return "", "", "", "", false, nil, e
			}
			i += n
		case strings.HasPrefix(a, "--outcome=") || strings.HasPrefix(a, "-outcome="):
			if outcome != "" {
				return "", "", "", "", false, nil, fmt.Errorf("--outcome given more than once")
			}
			_, outcome, _ = strings.Cut(a, "=")
		case a == "--defer-statements" || a == "-defer-statements":
			if deferStatements {
				return "", "", "", "", false, nil, fmt.Errorf("--defer-statements given more than once")
			}
			deferStatements = true
		default:
			rest = append(rest, a)
		}
	}
	return kind, name, problem, outcome, deferStatements, rest, nil
}

// designDeps bundles design start's injectable dependencies (mirroring
// syncDeps) so runDesignStart can be driven hermetically in tests
// (CLAUDE.md: no network, no exec in any test); cmdDesignStart wires the
// real ones. Runner is nil when verdi.yaml carries no toolchain: block —
// baseline.go's regenerateBaseline reads that as "skip gracefully",
// never as an error.
//
// Problem/Outcome/DeferStatements/Stdin/IsTTY are spec/cli-creation
// ac-1/ac-2's statement-sourcing inputs (ledger L-N7): Problem and
// Outcome carry --problem/--outcome's values ("" when not given);
// DeferStatements carries --defer-statements; Stdin/IsTTY are the
// interview's injectable input source and the real-TTY predicate,
// mirroring cmd/verdi/init.go's own stdin/isTTY injection for the init
// wizard. The zero value of all five — no statement flags, no TTY — is
// deliberately the REFUSAL case (runDesignStart: "cannot interview,
// statements required"), so every pre-existing test that constructs a
// designDeps without opting into one of the three statement-sourcing
// modes now sets DeferStatements: true explicitly, preserving its
// original TODO-placeholder scaffold (a disclosed migration, spec/
// cli-creation's own build: this verb's default behavior changed by
// construction, never silently).
type designDeps struct {
	Provider        provider.StoryProvider
	Runner          upstream.Runner
	GoTest          goTestRunner
	Problem         string
	Outcome         string
	DeferStatements bool
	Stdin           io.Reader
	IsTTY           bool
}

// cmdDesignStart is `verdi design start`'s real entry point: it parses
// --kind/--name and the optional positional ref, resolves the store root
// and manifest, wires the real provider registry from verdi.yaml's
// providers: map (buildProviderRegistry — the same construction rollup/sync
// use, so a configured jira ref resolves or degrades for the true reason per
// 04 §Semantics) and runner, and delegates to runDesignStart.
func cmdDesignStart(args []string, stdout, stderr io.Writer) int {
	// --from-stub is a wholly distinct invocation shape (spec/cli-creation
	// ac-3, ledger L-N7): <feature> <stub>, never --kind/--name/the
	// statement flags below — dispatched to its own file
	// (designfromstub.go) before extractFlags' --kind/--name grammar ever
	// sees these tokens.
	if len(args) > 0 && args[0] == "--from-stub" {
		return cmdDesignStartFromStub(args[1:], stdout, stderr)
	}

	kindArg, name, problem, outcome, deferStatements, rest, err := extractFlags(args)
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
		// vocab:identity — CLI usage/flag grammar (--kind enum values, identity)
		fmt.Fprintln(stderr, "design start: --kind is required (feature|story, 05 §CLI)")
		return 2
	default:
		// vocab:identity — CLI usage/flag grammar (--kind enum values, identity)
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
		// vocab:identity — CLI usage/flag grammar (--kind enum values, identity)
		fmt.Fprintln(stderr, "design start: --kind story requires a scheme-prefixed story ref (e.g. jira:LOAN-1482)")
		return 2
	default:
		// vocab:identity — CLI usage/flag grammar (--kind enum values, identity)
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
	deps := designDeps{
		Provider: reg, Runner: runner, GoTest: realGoTestRunner{},
		Problem: problem, Outcome: outcome, DeferStatements: deferStatements,
		Stdin: os.Stdin, IsTTY: isDesignAssumeTTY(),
	}

	return runDesignStart(ctx, root, kind, storyRef, name, manifest, cfg.Model, deps, stdout, stderr)
}

// isDesignAssumeTTY reports whether the real os.Stdin is attached to an
// interactive terminal (golang.org/x/term.IsTerminal — a real ioctl
// check), for design start's flagless TTY interview (spec/cli-creation
// ac-2). VERDI_DESIGN_ASSUME_TTY=1 is a disclosed, test-only override —
// mirroring cmd/verdi/init.go's own VERDI_INIT_ASSUME_TTY precedent
// (itself mirroring serve.go's VERDI_REVIEW_FEED/VERDI_OPENMR_FEED
// canned-injection convention) — that lets a built-binary test drive the
// real interview over a scripted stdin pipe without a real terminal ever
// being attached. A DISTINCT env var from init's own, deliberately: the
// two verbs' test-injection surfaces stay independent even though
// init_test.go and design_test.go share one compiled test binary. It
// changes nothing about a real user's invocation: no production flag or
// documented surface ever sets it.
func isDesignAssumeTTY() bool {
	if os.Getenv("VERDI_DESIGN_ASSUME_TTY") == "1" {
		return true
	}
	return term.IsTerminal(int(os.Stdin.Fd()))
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
		// vocab:identity — CLI usage/flag grammar (--kind enum values, identity)
		fmt.Fprintf(stderr, "design start: internal error: kind %q is neither feature nor story\n", kind)
		return 2
	}
	if kind == artifact.ClassStory && storyRef == "" {
		// vocab:identity — CLI usage/flag grammar (--kind enum values, identity)
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
			// vocab:identity — the "story ref" FIELD-form grammar (buildstart.go twin classification)
			fmt.Fprintf(stderr, "design start: story ref %q: %v\n", storyRef, err)
			return 2
		}
		if schemes := manifest.ConfiguredStorySchemes(); !schemes[scheme] {
			// vocab:identity — the "story ref" FIELD-form grammar; scheme/provider are manifest ids
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

	// Statement sourcing (spec/cli-creation ac-1/ac-2, ledger L-N7): the
	// scaffold's problem/outcome content comes from exactly one of three
	// explicit sources — never a silent TODO placeholder by default, the
	// behavior this verb carried before this story. --problem/--outcome
	// given together source it directly (TODO-free); --defer-statements
	// keeps the old placeholders, but only with an explicit disclosure
	// line naming them deferred; otherwise, on an attached terminal, a
	// TTY interview collects it, driven from the same designscaffold.Fields
	// descriptors the board's creation form validates against (never a
	// second, hand-rolled field list); a non-interactive invocation with
	// none of the above refuses by name — "cannot interview, statements
	// required" — rather than falling back to the retired silent default.
	problemText, outcomeText := deps.Problem, deps.Outcome
	switch {
	case deps.DeferStatements && (deps.Problem != "" || deps.Outcome != ""):
		fmt.Fprintln(stderr, "design start: --defer-statements cannot be combined with --problem/--outcome (mutually exclusive statement-sourcing modes)")
		return 2
	case deps.Problem != "" && deps.Outcome != "":
		// Both given together: TODO-free, nothing more to resolve.
	case deps.Problem != "" || deps.Outcome != "":
		fmt.Fprintln(stderr, "design start: --problem and --outcome must be given together (a lone flag would leave one section templated and the other not); use --defer-statements to defer both instead")
		return 2
	case deps.DeferStatements:
		problemText, outcomeText = designscaffold.DefaultProblem, designscaffold.DefaultOutcome
		// spec/verb-surfaces ac-4: the verb word routes through DisplayVerb —
		// a cli-creation surface this feature shipped, now vocabulary-complete.
		fmt.Fprintf(stdout, "design start: --defer-statements: problem and outcome deliberately deferred as TODO placeholders — replace both before %s\n", mdl.DisplayVerb("accept"))
	default:
		if !deps.IsTTY {
			fmt.Fprintln(stderr, "design start: cannot interview (no attached terminal) and statements are required; pass --problem/--outcome, or --defer-statements to explicitly defer them")
			return 2
		}
		stdin := deps.Stdin
		if stdin == nil {
			stdin = os.Stdin
		}
		answers, ierr := designinterview.RunInterview(tmpl, stdin, stdout)
		if ierr != nil {
			fmt.Fprintln(stderr, "design start:", ierr)
			return 2
		}
		problemText, outcomeText = answers["Problem"], answers["Outcome"]
	}

	var content string
	if kind == artifact.ClassFeature {
		content, err = designscaffold.Feature(tmpl, specRef.String(), storyRef, title, problemText, outcomeText)
	} else {
		// design start's own placeholder edge: an implements edge to a
		// not-yet-real feature/AC pair, since 05 §CLI names no --feature
		// flag (nothing about which feature a story belongs to is knowable
		// at scaffold time) — the design-branch edit is where a human or
		// agent replaces it with a real edge into the accepted feature this
		// story implements.
		links := []designscaffold.StoryLink{{Type: artifact.LinkImplements, Ref: "spec/todo-replace-feature-name#ac-1"}}
		content, err = designscaffold.Story(tmpl, specRef.String(), storyRef, title, false, links, problemText, outcomeText)
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
	// K1: the decoded scaffold's OWN class must agree with the requested
	// --kind — class.Template (above) is DATA, so a misconfigured
	// model.yaml (or a hand-edited store override) can bind kind's class
	// to a DIFFERENT class's template file and still strict-decode clean,
	// as the other class. Caught here, before any write: stdout and the
	// commit message below echo the REQUESTED kind, so a silent mismatch
	// would otherwise land a spec.md whose own class: line disagrees with
	// everything design start told the caller it scaffolded.
	if err := designscaffold.CheckClass(spec, kind); err != nil {
		fmt.Fprintln(stderr, "design start: internal error: scaffold failed self-validation:", err)
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
		// vocab:identity — git commit subject (history, never display prose)
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
	// vocab:identity — echo of the scaffolded frontmatter (field names + enum values)
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
		// vocab:identity — the "story ref" FIELD-form grammar (title enrichment disclosure)
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
