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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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

// cmdContext dispatches `verdi context <subcommand>`. The namespace exposes
// the read-only "compile" and "conflict" inspection surfaces; any other
// subcommand — or none — is a usage error (CLAUDE.md's operational exit code
// 2). This argument-shape check
// runs before any store root is resolved, so a bare `verdi context` is
// hermetic and safe against a live checkout (mirrors journey.go/model.go's
// own usage-first posture).
func cmdContext(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, contextCompileUsage)
		return 2
	}
	switch args[0] {
	case "compile":
		return cmdContextCompile(args[1:], stdin, stdout, stderr)
	case "conflict":
		return cmdContextConflict(args[1:], stdin, stdout, stderr)
	case "execution":
		return cmdContextExecution(args[1:], stdin, stdout, stderr)
	case "mcp":
		return cmdContextMCP(args[1:], stdin, stdout, stderr)
	case "receipt":
		return cmdContextReceipt(args[1:], stdin, stdout, stderr)
	default:
		fmt.Fprintln(stderr, contextCompileUsage)
		return 2
	}
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
	// Symmetric with --request's own emptiness check: `--out=` (or an
	// explicitly empty value) names no destination at all, so it is a flag
	// shape the parser rejects — never an invocation that compiles first
	// and only discovers it has nowhere to write at write time.
	if hasOut && outArg == "" {
		fmt.Fprintln(stderr, "context compile: --out requires a value")
		return 2
	}
	// Belt-and-braces against the lexical/kernel `..` split (see
	// canonicalOutPath): a destination spelled with a `..` element is
	// rejected outright at flag-shape time, hermetically, before any store
	// root is resolved. The caller can always respell the same destination
	// without `..`, so this over-refusal costs nothing and removes a whole
	// class of guard-versus-write divergence.
	if hasOut && hasDotDotElement(outArg) {
		fmt.Fprintln(stderr, "context compile:", errContextOutDotDot)
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

	// outCanon is the ONE destination string this command ever uses once a
	// root exists: every guard below approves it, and the write at the end
	// receives exactly it. Canonicalizing once and reusing the result is
	// what makes "the guard approved this write" a true statement rather
	// than a statement about a different, merely similar, path.
	var outCanon string
	if hasOut {
		outCanon, err = canonicalOutPath(root, outArg)
		if err != nil {
			printContextDiagnostic(stderr, root, err)
			return 2
		}
		if err := validateContextOutputStoreZone(root, outCanon, requestArg); err != nil {
			printContextDiagnostic(stderr, root, err)
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
		printContextDiagnostic(stderr, root, fmt.Errorf("reading request: %w", err))
		return 2
	}

	request, err := contextcompile.DecodeRequest(data)
	if err != nil {
		printContextDiagnostic(stderr, root, err)
		return contextExitCode(err)
	}

	result, err := contextcompile.NewCompiler().Compile(context.Background(), root, request)
	if err != nil {
		printContextDiagnostic(stderr, root, err)
		return contextExitCode(err)
	}

	if !hasOut {
		if _, err := stdout.Write(result.ManifestBytes); err != nil {
			printContextDiagnostic(stderr, root, fmt.Errorf("writing manifest to stdout: %w", err))
			return 2
		}
		return 0
	}

	if err := validateContextOutputProjectionPaths(root, outCanon, result.ManagedProjectionPaths); err != nil {
		printContextDiagnostic(stderr, root, err)
		return 2
	}
	if err := atomicfile.Write(outCanon, result.ManifestBytes, 0o644); err != nil {
		printContextDiagnostic(stderr, root, fmt.Errorf("writing manifest: %w", err))
		return 2
	}
	return 0
}

// contextCheckoutToken is the stable, relative stand-in every absolute
// spelling of the resolved store root is replaced by before a diagnostic
// reaches stderr. It is a fixed literal, so a redacted diagnostic is
// byte-identical across machines, users and checkout locations — which is
// the "deterministic diagnostics" half of the Wave-3 plan Task 8 Step 1
// bullet, not merely the "without absolute checkout paths" half.
const contextCheckoutToken = "<checkout>"

// printContextDiagnostic writes one "context compile: <message>" line to
// stderr with every absolute spelling of root redacted.
//
// The redaction lives HERE, at the CLI boundary, and deliberately not in
// the seams that produce the paths. internal/gitx formats every failure as
// "gitx: git %s (dir %s): %w: %s" and internal/policyauthority wraps
// *os.PathError from os.ReadFile; both are shared packages whose other
// consumers (journey, repositoryfacts, lint, the projection verifier) have
// their own diagnostic contracts, and quieting them globally would trade a
// leak in one verb for lost debuggability everywhere else. This verb's
// stderr contract is this verb's to enforce.
func printContextDiagnostic(stderr io.Writer, root string, err error) {
	printContextCommandDiagnostic(stderr, "compile", root, err)
}

// printContextCommandDiagnostic is the context namespace's one redaction and
// diagnostic framing seam. Both inspection subcommands reach it so neither
// can leak a checkout path while formatting a package or process error.
func printContextCommandDiagnostic(stderr io.Writer, subcommand, root string, err error) {
	fmt.Fprintf(stderr, "context %s: %s\n", subcommand, redactCheckoutRoot(root, err.Error()))
}

// redactCheckoutRoot replaces every absolute spelling of root in msg with
// contextCheckoutToken. Both the root as resolved and its EvalSymlinks form
// are redacted: a checkout reached through a symlinked ancestor (macOS's
// /var -> /private/var being the everyday case) has two absolute spellings,
// and different seams report different ones — os.ReadFile echoes the path
// it was handed, while an exec'd child may report the kernel-resolved one.
//
// Longer spellings are replaced first because one variant can contain
// another as a substring ("/private/var/x" contains "/var/x"); replacing
// the shorter one first would leave a mangled "/private<checkout>" rather
// than a clean token. Replacement is unconditional and order-fixed, so the
// result is deterministic for a given message.
func redactCheckoutRoot(root, msg string) string {
	if root == "" {
		return msg
	}
	variants := []string{filepath.Clean(root)}
	if abs, err := filepath.Abs(root); err == nil {
		variants = append(variants, abs)
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		variants = append(variants, resolved)
	}
	// The process environment can retain the caller's symlink spelling of
	// the current checkout even when filepath.Abs/Getwd return the kernel-
	// resolved spelling (notably macOS /var versus /private/var). Include it
	// only after proving it names the same directory, so an unrelated PWD
	// value can never redact arbitrary diagnostic text.
	if pwd := os.Getenv("PWD"); filepath.IsAbs(pwd) {
		rootInfo, rootErr := os.Stat(root)
		pwdInfo, pwdErr := os.Stat(pwd)
		if rootErr == nil && pwdErr == nil && os.SameFile(rootInfo, pwdInfo) {
			variants = append(variants, filepath.Clean(pwd))
		}
	}
	// Also recover top-level symlink aliases for the resolved root. exec
	// diagnostics on macOS commonly re-spell /private/var/... as /var/...
	// even when both root and PWD are already resolved. This small bounded
	// scan proves the alias through EvalSymlinks before admitting it.
	if entries, err := os.ReadDir(string(filepath.Separator)); err == nil {
		cleanRoot := filepath.Clean(root)
		for _, entry := range entries {
			if entry.Type()&os.ModeSymlink == 0 {
				continue
			}
			aliasBase := filepath.Join(string(filepath.Separator), entry.Name())
			resolved, err := filepath.EvalSymlinks(aliasBase)
			if err != nil {
				continue
			}
			rel, err := filepath.Rel(resolved, cleanRoot)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				continue
			}
			variants = append(variants, filepath.Clean(filepath.Join(aliasBase, rel)))
		}
	}
	sort.Slice(variants, func(i, j int) bool { return len(variants[i]) > len(variants[j]) })

	seen := map[string]bool{}
	for _, v := range variants {
		// Never redact "/" or a relative spelling: the first would shred
		// every path in the message, the second is not a checkout leak.
		if seen[v] || v == "" || v == string(filepath.Separator) || !filepath.IsAbs(v) {
			continue
		}
		seen[v] = true
		msg = strings.ReplaceAll(msg, v, contextCheckoutToken)
	}
	return msg
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

// sameFileArg reports whether a and b name the same filesystem object,
// compared through canonicalGuardPath (absolute, symlink-resolved) rather
// than by cleaned spelling alone — catching a literal duplicate, two
// spellings of one path (`foo.json` vs `./foo.json`), a case-variant
// spelling on a case-insensitive filesystem, and a symlink whose target is
// the other argument. It falls back to a literal comparison when the cwd
// itself cannot be resolved.
func sameFileArg(a, b string) bool {
	aCanon, aErr := canonicalGuardPath(a)
	bCanon, bErr := canonicalGuardPath(b)
	if aErr != nil || bErr != nil {
		return a == b
	}
	if pathsEqualAliasSafe(aCanon, bCanon) {
		return true
	}
	aInfo, aStatErr := os.Stat(aCanon)
	bInfo, bStatErr := os.Stat(bCanon)
	return aStatErr == nil && bStatErr == nil && os.SameFile(aInfo, bInfo)
}

// validateContextOutputStoreZone rejects an --out destination that falls
// inside root's .git/ or .verdi/ trees, or that names the exact input
// request file (Wave-3 plan Task 8 Step 2) — checked before any request
// is even read, so an unsafe --out never triggers a compile at all. The
// comparison is over canonical (symlink-resolved) spellings, so a
// symlinked parent or a case-variant spelling of a reserved path cannot
// smuggle a write into it. outCanon is canonicalOutPath's result — the
// very string the eventual write receives, never the caller's raw spelling.
func validateContextOutputStoreZone(root, outCanon, request string) error {
	forbidden := []string{
		filepath.Join(root, ".git"),
		filepath.Join(root, ".verdi"),
	}
	if request != "-" {
		forbidden = append(forbidden, request)
	}
	return checkNotWithinAny(outCanon, forbidden)
}

// validateContextOutputProjectionPaths rejects an --out destination that
// collides with ANY adapter's managed instruction-projection file in the
// resolved constitution — not only the REQUESTED adapter's own (Wave-3
// plan Task 8 Step 2: "one of the managed projection paths"; the plan's
// "never writes ... managed projection files" constraint names the whole
// managed surface, not a per-request subset of it). Checked only after a
// successful Compile, since the managed path set is only known then.
// managedProjectionPaths is Result.ManagedProjectionPaths — the full,
// cross-adapter set stage 5 of Compile already resolves the constitution
// to compute, so this guard never triggers a second authority load.
// outCanon is the same canonicalOutPath result the store-zone guard
// already approved and the write is about to receive.
func validateContextOutputProjectionPaths(root, outCanon string, managedProjectionPaths []string) error {
	forbidden := make([]string, 0, len(managedProjectionPaths))
	for _, rel := range managedProjectionPaths {
		forbidden = append(forbidden, filepath.Join(root, filepath.FromSlash(rel)))
	}
	return checkNotWithinAny(outCanon, forbidden)
}

// errContextOutReserved is the guard's one deterministic, path-free
// refusal (CLAUDE.md/the authority design: stderr diagnostics must never
// carry an absolute checkout path). Every reserved-destination match — by
// canonical spelling, by case-folded spelling, or by filesystem identity —
// reports exactly this message.
var errContextOutReserved = errors.New("--out must not target a reserved store path, a managed projection file, or the input request file")

// errContextOutDotDot is the deterministic, path-free diagnostic for an
// --out spelling that carries a ".." element. See canonicalOutPath for why
// such a spelling is refused outright rather than resolved.
var errContextOutDotDot = errors.New(`--out must not contain a ".." path element`)

// errContextOutSymlink is the deterministic, path-free diagnostic for an
// existing symlink anywhere in the output path. Output paths are authority
// boundaries, so resolving a symlink to an otherwise-safe target is not an
// acceptable substitute for proving that the caller's path is symlink-free.
var errContextOutSymlink = errors.New("--out must not contain a symlink path component")

// hasDotDotElement reports whether p contains a ".." PATH ELEMENT under
// either separator convention. It is element-wise, never a substring test:
// a file honestly named "..notes.json" or "a..b" carries no traversal and
// stays allowed.
func hasDotDotElement(p string) bool {
	for _, seg := range strings.FieldsFunc(p, func(r rune) bool {
		return r == '/' || r == filepath.Separator
	}) {
		if seg == ".." {
			return true
		}
	}
	return false
}

// canonicalOutPath returns the single absolute, alias-resolved destination
// string the command uses for BOTH the reserved-path guards and the write
// itself.
//
// The single-string discipline exists because filepath.Clean collapses a
// ".." element LEXICALLY, before any symlink is resolved, while the kernel
// resolves each symlink component FIRST and only then applies "..". With
// `a` a symlink to `.verdi/sub`, the spelling `a/../out.json` cleans to
// `./out.json` (which the guard happily approves) but names `.verdi/
// out.json` to the kernel (which authority design §11 forbids writing).
// Handing the write the guard's own canonical string closes that gap by
// construction: there is no second path to disagree with the approved one.
// hasDotDotElement rejects such spellings earlier still, so this function's
// input is already ".."-free; the discipline is kept structurally anyway so
// no future spelling can reintroduce the divergence.
func canonicalOutPath(root, p string) (string, error) {
	if err := rejectContextOutputSymlinks(root, p); err != nil {
		return "", err
	}
	canon, err := canonicalGuardPath(p)
	if err != nil {
		return "", fmt.Errorf("resolving --out: %w", err)
	}
	return canon, nil
}

// rejectContextOutputSymlinks Lstats every existing caller-selected component
// of p. Only lexical containment under the established checkout root or its
// explicitly resolved spelling creates a trusted anchor. Every other path is
// inspected from the filesystem root, so a caller-created alias to the checkout
// or one of its ancestors cannot become authority merely through filesystem
// identity. Inspection stops at the first nonexistent component so a new leaf
// or parent remains valid for atomicfile.Write to create. Lstat is deliberate:
// Stat/EvalSymlinks would follow the entry and erase the fact that the caller
// selected a symlinked output path.
func rejectContextOutputSymlinks(root, p string) error {
	abs, err := filepath.Abs(p)
	if err != nil {
		return fmt.Errorf("resolving --out: %w", err)
	}
	abs = normalizeContextSystemAlias(filepath.Clean(abs))

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolving checkout root: %w", err)
	}
	rootAbs = normalizeContextSystemAlias(filepath.Clean(rootAbs))
	rootCanonical, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return fmt.Errorf("resolving checkout root: %w", err)
	}
	rootCanonical = normalizeContextSystemAlias(filepath.Clean(rootCanonical))

	anchor := ""
	for _, trusted := range []string{rootAbs, rootCanonical} {
		if (abs == trusted || isWithinDir(abs, trusted)) && len(trusted) > len(anchor) {
			anchor = trusted
		}
	}

	components := make([]string, 0, 8)
	if anchor != "" {
		for cur := abs; cur != anchor; cur = filepath.Dir(cur) {
			components = append(components, cur)
		}
	} else {
		for cur := abs; ; {
			components = append(components, cur)
			parent := filepath.Dir(cur)
			if parent == cur {
				break
			}
			cur = parent
		}
	}
	for i := len(components) - 1; i >= 0; i-- {
		info, statErr := os.Lstat(components[i])
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("checking --out path components: %w", statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errContextOutSymlink
		}
	}
	return nil
}

// normalizeContextSystemAlias canonicalizes macOS's system-owned
// /var -> /private/var alias before output-component inspection. This narrow,
// verified alias preserves checkout and temp-path spellings supplied by the OS
// without admitting caller-created aliases as trusted roots.
func normalizeContextSystemAlias(p string) string {
	if filepath.Separator != '/' {
		return p
	}
	varRoot := filepath.Clean("/var")
	privateVarRoot := filepath.Clean("/private/var")
	resolved, err := filepath.EvalSymlinks(varRoot)
	if err != nil || filepath.Clean(resolved) != privateVarRoot {
		return p
	}
	if p != varRoot && !isWithinDir(p, varRoot) {
		return p
	}
	rel, err := filepath.Rel(varRoot, p)
	if err != nil {
		return p
	}
	return filepath.Clean(filepath.Join(privateVarRoot, rel))
}

// checkNotWithinAny returns errContextOutReserved when the canonical
// outCanon equals, or falls inside, any directory or file in forbidden.
// Each forbidden entry is canonicalized the same way, and the comparison
// fails closed on ANY of three signals: exact canonical equality/
// containment, case-folded equality/containment (a case-insensitive
// filesystem resolves `.VERDI` and `.verdi` to one directory), or
// filesystem identity via os.SameFile against outCanon or any of its
// existing ancestors (which catches aliases no textual comparison can see,
// including hardlinked or otherwise duplicated directory entries).
func checkNotWithinAny(outCanon string, forbidden []string) error {
	for _, f := range forbidden {
		fCanon, err := canonicalGuardPath(f)
		if err != nil {
			// A reserved path that cannot be resolved cannot be
			// excluded either; refuse rather than let --out through.
			return errContextOutReserved
		}
		if pathEqualOrWithin(outCanon, fCanon) || sameFileAsSelfOrAncestor(outCanon, fCanon) {
			return errContextOutReserved
		}
	}
	return nil
}

// canonicalGuardPath returns p's alias-resolved absolute spelling: the
// absolute cleaned path with filepath.EvalSymlinks applied to its DEEPEST
// EXISTING ancestor and the not-yet-existing remainder re-appended (an
// --out destination usually does not exist yet, so EvalSymlinks cannot be
// applied to the whole path). The only error returned is a failure to make
// p absolute — an unreadable cwd; a failing EvalSymlinks (for example an
// unreadable ancestor) degrades to the unresolved absolute spelling, which
// the caller still compares textually and by filesystem identity.
func canonicalGuardPath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)

	remainder := ""
	for cur := abs; ; {
		if _, statErr := os.Stat(cur); statErr == nil {
			resolved, evalErr := filepath.EvalSymlinks(cur)
			if evalErr != nil {
				return abs, nil
			}
			return filepath.Clean(filepath.Join(resolved, remainder)), nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs, nil
		}
		remainder = filepath.Join(filepath.Base(cur), remainder)
		cur = parent
	}
}

// pathsEqualAliasSafe reports whether two canonical paths name the same
// destination, treating a pure case difference as a match: the guard must
// fail closed on a filesystem that resolves both spellings to one file,
// and refusing an oddly-cased --out elsewhere is a safe, deterministic
// over-refusal the caller resolves by renaming.
func pathsEqualAliasSafe(a, b string) bool {
	return a == b || strings.EqualFold(a, b)
}

// pathEqualOrWithin reports whether path equals dir or falls inside it,
// under both exact and case-folded comparison.
func pathEqualOrWithin(path, dir string) bool {
	if pathsEqualAliasSafe(path, dir) {
		return true
	}
	return isWithinDir(path, dir) || isWithinDir(strings.ToLower(path), strings.ToLower(dir))
}

// sameFileAsSelfOrAncestor reports whether path, or any existing ancestor
// of it, is the very filesystem object target names. An ancestor match
// means path lies inside target, however it was spelled.
func sameFileAsSelfOrAncestor(path, target string) bool {
	targetInfo, err := os.Stat(target)
	if err != nil {
		return false
	}
	for cur := path; ; {
		if info, statErr := os.Stat(cur); statErr == nil && os.SameFile(info, targetInfo) {
			return true
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return false
		}
		cur = parent
	}
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
