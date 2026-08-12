// verdi context compile --request <path|-> [--out <path>] (Context Integrity
// Wave-3, docs/superpowers/specs/2026-08-11-context-compiler-authority-
// design.md §2, ledger SI-78..SI-87): the first built-binary inspection
// surface over the read-only internal/contextcompile.Compiler. It decodes
// a caller-supplied `verdi.context-compile-request/v1` document (from a
// file or, with `-`, from stdin), compiles it against the checkout's
// trusted in-process ports, and returns exactly one canonical
// `verdi.context-manifest/v1` document — to stdout when --out is absent,
// or to exactly one caller-selected file when --out is present (in which
// case stdout stays empty). Data items and rendered projection bytes are
// never written to disk by this verb; only the manifest, and only to the
// one destination the caller named.
//
// Registered at verb phase 23 (authority design §2). This command accepts
// no actor, evidence, persistence, or execution flags — its actor posture
// is always explicitly unproven (authority design §2: "The CLI supplies no
// principal-resolution port in v1").
//
// Kept in its own file per the lint.go/sync.go/matrix.go/dex.go/journey.go
// convention, so dispatch.go's diff for wiring this verb in stays a
// one-line change.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/store"
)

// contextCompileUsage is the exact invocation grammar the authority design
// (§2) fixes.
//
// vocab:identity — CLI usage/flag grammar (identity)
const contextCompileUsage = "usage: verdi context compile --request <path|-> [--out <path>]"

// cmdContext dispatches `verdi context <subcommand>`. v0 has exactly one
// subcommand, "compile"; any other subcommand — or none — is a usage
// error (CLAUDE.md's operational exit code 2). This argument-shape check
// runs before any store root is resolved, so a bare `verdi context` is
// hermetic and safe against a live checkout (mirrors journey.go/model.go's
// own usage-first posture).
func cmdContext(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "compile" {
		fmt.Fprintln(stderr, contextCompileUsage)
		return 2
	}
	return cmdContextCompile(args[1:], stdin, stdout, stderr)
}

// cmdContextCompile implements `verdi context compile`. Every flag-shape
// check (missing/unknown subcommand already handled by cmdContext,
// unknown/missing/duplicate flags, extra positional arguments, and
// --request equal to --out) runs before the store root is resolved, so
// those failures are hermetic and identical regardless of cwd.
func cmdContextCompile(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	requestArg, hasRequest, outArg, hasOut, rest, err := extractContextCompileFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, "context compile:", err)
		return 2
	}
	if len(rest) != 0 {
		fmt.Fprintln(stderr, "context compile: unexpected positional argument(s):", strings.Join(rest, " "))
		return 2
	}
	if !hasRequest || requestArg == "" {
		fmt.Fprintln(stderr, "context compile: --request is required")
		return 2
	}
	if hasOut && requestArg != "-" && sameFileArg(requestArg, outArg) {
		fmt.Fprintln(stderr, "context compile: --request and --out must not name the same path")
		return 2
	}

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "context compile:", err)
		return 2
	}

	if hasOut {
		if err := validateContextOutputStoreZone(root, outArg, requestArg); err != nil {
			fmt.Fprintln(stderr, "context compile:", err)
			return 2
		}
	}

	var data []byte
	if requestArg == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(requestArg)
	}
	if err != nil {
		fmt.Fprintln(stderr, "context compile: reading request:", err)
		return 2
	}

	request, err := contextcompile.DecodeRequest(data)
	if err != nil {
		fmt.Fprintln(stderr, "context compile:", err)
		return contextExitCode(err)
	}

	result, err := contextcompile.NewCompiler().Compile(context.Background(), root, request)
	if err != nil {
		fmt.Fprintln(stderr, "context compile:", err)
		return contextExitCode(err)
	}

	if !hasOut {
		if _, err := stdout.Write(result.ManifestBytes); err != nil {
			fmt.Fprintln(stderr, "context compile: writing manifest to stdout:", err)
			return 2
		}
		return 0
	}

	if err := validateContextOutputProjectionPaths(root, outArg, result.ProjectionFiles); err != nil {
		fmt.Fprintln(stderr, "context compile:", err)
		return 2
	}
	if err := atomicfile.Write(outArg, result.ManifestBytes, 0o644); err != nil {
		fmt.Fprintln(stderr, "context compile: writing manifest:", err)
		return 2
	}
	return 0
}

// contextExitCode maps a Compile/DecodeRequest error to CLAUDE.md's 0/1/2
// contract: a closed contextcompile state refusal is a verdict failure
// (exit 1); every other error — malformed/noncanonical input, invalid
// authority, or an operational port failure — is exit 2 (authority design
// §10).
func contextExitCode(err error) int {
	if contextcompile.IsRefusal(err) {
		return 1
	}
	return 2
}

// extractContextCompileFlags pulls --request and --out out of args
// (mirroring board.go's extractBoardCommitFlags/design.go's
// extractFlags), rejecting a missing value, a repeated flag, or any other
// "--"-prefixed token outright rather than treating it as positional.
// Every other token is returned, in order, as rest — the exact grammar is
// `--request <path|-> [--out <path>]` with no positional arguments at
// all, so any nonempty rest is itself an error the caller reports.
func extractContextCompileFlags(args []string) (request string, hasRequest bool, out string, hasOut bool, rest []string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--request":
			if i+1 >= len(args) {
				return "", false, "", false, nil, fmt.Errorf("--request requires a value")
			}
			if hasRequest {
				return "", false, "", false, nil, fmt.Errorf("--request given more than once")
			}
			request, hasRequest = args[i+1], true
			i++
		case strings.HasPrefix(a, "--request="):
			if hasRequest {
				return "", false, "", false, nil, fmt.Errorf("--request given more than once")
			}
			_, request, _ = strings.Cut(a, "=")
			hasRequest = true
		case a == "--out":
			if i+1 >= len(args) {
				return "", false, "", false, nil, fmt.Errorf("--out requires a value")
			}
			if hasOut {
				return "", false, "", false, nil, fmt.Errorf("--out given more than once")
			}
			out, hasOut = args[i+1], true
			i++
		case strings.HasPrefix(a, "--out="):
			if hasOut {
				return "", false, "", false, nil, fmt.Errorf("--out given more than once")
			}
			_, out, _ = strings.Cut(a, "=")
			hasOut = true
		case strings.HasPrefix(a, "--"):
			return "", false, "", false, nil, fmt.Errorf("unknown flag %q", a)
		default:
			rest = append(rest, a)
		}
	}
	return request, hasRequest, out, hasOut, rest, nil
}

// sameFileArg reports whether a and b name the same filesystem path,
// resolved to their absolute cleaned forms when that resolution succeeds
// (falling back to a literal comparison otherwise, e.g. under an
// unreadable cwd) — catching both a literal duplicate and two different
// spellings of the same file (`foo.json` vs `./foo.json`).
func sameFileArg(a, b string) bool {
	aAbs, aErr := filepath.Abs(a)
	bAbs, bErr := filepath.Abs(b)
	if aErr != nil || bErr != nil {
		return a == b
	}
	return aAbs == bAbs
}

// validateContextOutputStoreZone rejects an --out destination that falls
// inside root's .git/ or .verdi/ trees, or that names the exact input
// request file (Wave-3 plan Task 8 Step 2) — checked before any request
// is even read, so an unsafe --out never triggers a compile at all.
func validateContextOutputStoreZone(root, out, request string) error {
	outAbs, err := filepath.Abs(out)
	if err != nil {
		return fmt.Errorf("resolving --out: %w", err)
	}
	outAbs = filepath.Clean(outAbs)

	forbidden := []string{
		filepath.Join(root, ".git"),
		filepath.Join(root, ".verdi"),
	}
	if request != "-" {
		if reqAbs, err := filepath.Abs(request); err == nil {
			forbidden = append(forbidden, filepath.Clean(reqAbs))
		}
	}
	return checkNotWithinAny(outAbs, forbidden)
}

// validateContextOutputProjectionPaths rejects an --out destination that
// collides with one of this compile's own managed instruction-projection
// files (Wave-3 plan Task 8 Step 2: "one of the managed projection
// paths") — checked only after a successful Compile, since the exact set
// of managed paths for THIS request's adapter is only known then.
func validateContextOutputProjectionPaths(root, out string, projectionFiles []contextcompile.ProjectionFile) error {
	outAbs, err := filepath.Abs(out)
	if err != nil {
		return fmt.Errorf("resolving --out: %w", err)
	}
	outAbs = filepath.Clean(outAbs)

	forbidden := make([]string, 0, len(projectionFiles))
	for _, pf := range projectionFiles {
		forbidden = append(forbidden, filepath.Join(root, filepath.FromSlash(pf.Path)))
	}
	return checkNotWithinAny(outAbs, forbidden)
}

// checkNotWithinAny returns a deterministic, path-free refusal (CLAUDE.md/
// the authority design: stderr diagnostics must never carry an absolute
// checkout path) when outAbs equals, or falls inside, any directory or
// file in forbidden.
func checkNotWithinAny(outAbs string, forbidden []string) error {
	for _, f := range forbidden {
		f = filepath.Clean(f)
		if outAbs == f || isWithinDir(outAbs, f) {
			return fmt.Errorf("--out must not target a reserved store path, a managed projection file, or the input request file")
		}
	}
	return nil
}

// isWithinDir reports whether path is strictly inside dir (dir treated as
// a directory boundary, not a prefix string match — "/x/yy" is not
// "within" "/x/y").
func isWithinDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
