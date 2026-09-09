// verdi context project [--root DIR] (local-operator disposition design
// 2026-09-05, §2.4; ledger SI-183; plan Task 4): a thin CLI wrapper over
// the read-only internal/instructionprojection.Generate. Library-only
// generation forced a throwaway program in every fixture regeneration to
// date (design §2.4); this verb removes that.
//
// It writes exactly what Generate itself writes — every adapter's managed
// projection files and one manifest per adapter — and then reports,
// deterministically (sorted by repo-relative path, across every adapter),
// each written path with its sha256 content digest, one
// "<digest>  <path>" line per file, so a caller can diff two runs' stdout
// byte-for-byte. Human prose goes to stderr only, on every invocation —
// including --help/-h, which this verb does not accept and refuses like
// any other flag-shape error, exit 2 — so stdout carries nothing but
// those lines on success, and is otherwise always empty, except the one
// case this command cannot prevent: a destination writer that itself
// returns a short byte count with a nil error has already accepted those
// bytes before the failure is detected (see the short-write handling
// below) — a contract violation on the writer's part, not this command's.
//
// Kept in its own file per the lint.go/sync.go/matrix.go/dex.go/journey.go
// convention (context.go's own doc comment), so dispatch.go's — here,
// context.go's own switch's — diff for wiring this verb in stays a
// one-line change.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyauthority"
	"github.com/jyang234/verdi/internal/store"
)

// contextProjectUsage is this verb's own invocation grammar: printed for
// --help/-h, and appended after every flag-shape diagnostic below —
// mirroring context_resolve.go/cmdContextContract's own "always show the
// one-line grammar" posture, which fits a verb with exactly one optional
// flag better than context_compile.go/context_conflict.go's per-error,
// no-banner style (reserved for their own multi-flag grammars).
//
// vocab:identity — CLI usage/flag grammar (identity)
const contextProjectUsage = "usage: verdi context project [--root DIR]"

// cmdContextProject implements `verdi context project [--root DIR]`.
//
// --help/-h are NOT accepted flags: no verb in this binary recognizes
// them, and every sibling verb's own flag-shape diagnostics go to
// stderr, never stdout. Both fall straight through to the ordinary
// flag-shape handling below like any other malformed invocation —
// --help as an unknown flag, -h (which does not start with "--") as an
// unexpected positional argument — so stdout is never used to answer a
// help request; there is no such requestable state.
//
// Exit-class conventions (mirroring context_conflict.go's own mapping,
// CLAUDE.md's 0/1/2 contract): a flag-shape error, an unusable root (no
// ancestor .verdi/verdi.yaml, or an explicit --root that does not itself
// carry one), or any I/O failure while writing or reporting is
// operational (2). A policy/constitution refusal Generate reports by name
// — policyauthority.ErrNotAdopted ("no constitution") or
// instructionprojection.ErrOverlappingManagedPath ("overlapping managed
// paths") — is the verdict 1, matching this design's own two named
// examples. policyauthority.ErrIncompleteAdoption (a half-adopted store:
// .verdi/policy/ exists but constitution.md does not) is deliberately
// NOT folded into that verdict class: cmd/verdi/context_conflict.go's own
// provider path only ever treats policyauthority.ErrNotAdopted as a
// "not adopted" verdict (internal/policyconflict.Service checks
// errors.Is(err, policyauthority.ErrNotAdopted) alone, never
// ErrIncompleteAdoption), and the design text names only "no
// constitution", not this distinct broken-adoption state — so it falls
// through to the operational default, consistent with that sibling verb.
// Every other Generate failure is likewise operational, but Generate's
// own preflight (internal/instructionprojection's
// checkProjectionPathsSafe) only ever refuses a PRE-EXISTING symlinked
// projections directory or managed file — genuinely before any adapter
// is written, across every adapter at once. A managed path already
// occupied by something Generate cannot overwrite (a plain directory,
// say), a Render failure, or a permission/other I/O failure is different:
// Generate writes adapter-by-adapter in sorted-id order
// (internal/instructionprojection/generate.go's own write loop) and only
// fails once it reaches the offending adapter, so every EARLIER
// adapter's managed files and manifest may already be on disk — Generate
// returns no partial-result value to say so, only the error. This
// command cannot tell from the error alone which of these two shapes it
// is looking at, so on every OPERATIONAL Generate failure (never the two
// proven write-free verdict paths above, which are caught before any
// write is attempted) it discloses the conservative truth: the store may
// already be partially projected, and re-running this verb is always
// safe, because Generate is deterministic and idempotent (a clean file
// or manifest byte-matches what a second run would write anyway). Every
// error already names the offending repo-relative path and component;
// this command only needs to relay it.
func cmdContextProject(args []string, stdout, stderr io.Writer) int {
	rootFlag, hasRoot, rest, err := extractContextProjectFlags(args)
	if err != nil {
		return contextProjectFlagError(stderr, err.Error())
	}
	if len(rest) != 0 {
		return contextProjectFlagError(stderr, "unexpected positional argument(s): "+strings.Join(rest, " "))
	}
	if hasRoot && rootFlag == "" {
		return contextProjectFlagError(stderr, "--root requires a value")
	}

	var root string
	if hasRoot {
		root, err = store.RootAt(rootFlag)
		if err != nil {
			// store.RootAt's own message talks about "an explicit --store
			// override" — accurate for ITS doc comment's hypothetical
			// caller, but this verb's own flag is --root, and relaying
			// "--store" verbatim would name a flag that does not exist
			// here. Rewrite the flag name in place rather than editing
			// internal/store (shared infrastructure other, differently-
			// named callers may use truthfully); every other word in the
			// message (the exact path, the manifest filename, "no
			// ancestor search") stays accurate and is worth keeping.
			err = errors.New(strings.ReplaceAll(err.Error(), "--store", "--root"))
		}
	} else {
		root, err = store.FindRoot(".")
	}
	if err != nil {
		fmt.Fprintln(stderr, "context project:", err)
		return 2
	}

	result, err := instructionprojection.Generate(root)
	if err != nil {
		printContextCommandDiagnostic(stderr, "project", root, err)
		if errors.Is(err, policyauthority.ErrNotAdopted) || errors.Is(err, instructionprojection.ErrOverlappingManagedPath) {
			return 1
		}
		fmt.Fprintln(stderr, "context project: the store may be partially projected by this failed run; re-running this verb is safe (Generate is idempotent)")
		return 2
	}

	out := contextProjectFormatResult(result)
	written, werr := stdout.Write(out)
	if werr == nil && written != len(out) {
		// A writer that returns a short count with a nil error breaks its
		// own io.Writer contract, but a destination behind a pipe, a
		// filter, or a third-party wrapper can still do it (mirroring
		// context_resolve.go's own short-write posture) — never wrap a
		// nil error into the diagnostic.
		werr = fmt.Errorf("short write: wrote %d of %d bytes", written, len(out))
	}
	if werr != nil {
		printContextCommandDiagnostic(stderr, "project", root, fmt.Errorf("writing output: %w", werr))
		return 2
	}
	return 0
}

// contextProjectEntry is one reported line: a repo-relative path (a
// managed file or an adapter's own manifest) and its content digest.
type contextProjectEntry struct {
	Path   string
	Digest string
}

// contextProjectFormatResult renders res as sorted-by-path
// "<digest>  <path>\n" lines — every managed file AND every manifest,
// across every adapter, combined into one list and sorted purely by path
// so the output never depends on Result's own (adapter-then-file)
// incidental ordering. Zero adapters (a valid constitution declaring
// none) renders zero lines.
func contextProjectFormatResult(res *instructionprojection.Result) []byte {
	entries := make([]contextProjectEntry, 0, len(res.Adapters)*2)
	for _, a := range res.Adapters {
		for _, f := range a.Files {
			entries = append(entries, contextProjectEntry{Path: f.Path, Digest: f.Digest})
		}
		entries = append(entries, contextProjectEntry{Path: a.ManifestPath, Digest: a.ManifestDigest})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	var buf bytes.Buffer
	for _, e := range entries {
		buf.WriteString(e.Digest)
		buf.WriteString("  ")
		buf.WriteString(e.Path)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// contextProjectFlagError prints "context project: <msg>" followed by
// this verb's own usage line, and returns the operational exit code 2 —
// the one seam every flag-shape diagnostic below goes through, so the
// grammar is always shown alongside the specific reason it was violated.
func contextProjectFlagError(stderr io.Writer, msg string) int {
	fmt.Fprintln(stderr, "context project:", msg)
	fmt.Fprintln(stderr, contextProjectUsage)
	return 2
}

// extractContextProjectFlags pulls --root out of args (mirroring
// context.go's own extractContextCompileFlags), rejecting a missing
// value, a repeated flag, or any other "--"-prefixed token outright
// rather than treating it as positional. Every other token is returned,
// in order, as rest — the grammar has no positional arguments at all, so
// any nonempty rest is itself an error the caller reports.
func extractContextProjectFlags(args []string) (root string, hasRoot bool, rest []string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--root":
			if i+1 >= len(args) {
				return "", false, nil, fmt.Errorf("--root requires a value")
			}
			if hasRoot {
				return "", false, nil, fmt.Errorf("--root given more than once")
			}
			root, hasRoot = args[i+1], true
			i++
		case strings.HasPrefix(a, "--root="):
			if hasRoot {
				return "", false, nil, fmt.Errorf("--root given more than once")
			}
			_, root, _ = strings.Cut(a, "=")
			hasRoot = true
		case strings.HasPrefix(a, "--"):
			return "", false, nil, fmt.Errorf("unknown flag %q", a)
		default:
			rest = append(rest, a)
		}
	}
	return root, hasRoot, rest, nil
}
