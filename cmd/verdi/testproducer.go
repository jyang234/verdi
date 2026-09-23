// The per-test CI evidence producer (SI-228, plan R-CM-2; ledger SI-228,
// SI-229): 03 §Declarations and binding's coarse-by-default rule carries
// one sanctioned exception — an elaborated evidence obligation naming a
// single top-level Go test as its producer (`go-test:<package path
// relative to the module root>:<TopLevelTestName>`) is matched per test,
// not per suite. This file is that exception's one CI-side implementation:
// invoked only from `sync --produce` (sync.go's runProduce), after the
// coarse self-hosted producer (selfevidence.go), it walks every obligation
// on disk, selects the ones this CI job is authoritative for (SI-229's
// authoritative_source.ref == provenance.job_name), runs `go test -json
// -count=1 -run '^(...)$' ./<pkg>` once per named package through an
// injectable seam, and emits one record per selected obligation carrying
// that obligation's own kind and acceptance-criterion id.
//
// HONESTY. A renamed or removed test must surface as a missing producer, a
// closure blocker, never a silent pass — so a named test absent from the
// run emits no record, only a disclosure, and withdraws any earlier record
// for its producer at the same commit (SI-238; see produceGoTestEvidence's
// REPEAT PRODUCTION). But a malformed producer ref, or an obligation
// elsewhere in the store this reader cannot even decode, must never turn
// this shared CI step into an operational failure for every OTHER story's
// obligations (03 §Declarations and binding's own
// framing: "never as a silent pass" is about false positives, not about
// making one story's authoring mistake block CI for everyone else) — both
// are disclosed and skipped, exactly like an absent test. So is a named
// package that does not build or that the go command cannot load (renamed,
// removed, or broken): its named tests did not run. Only a runner that
// produces no output at all, or a go test -json stream the shared reader
// (internal/gotestjson) rejects — malformed, out of sequence, or ending
// before its package's own terminal event — is an operational error (exit
// 2): at that point nothing read from this invocation can be trusted.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/gotestjson"
	"github.com/jyang234/verdi/internal/store"
)

// --- Grammar (contract 1) ---------------------------------------------

// goTestProducerRef is a producer ref's parsed (package, top-level test)
// pair.
type goTestProducerRef struct {
	Package string
	Test    string
}

// parseGoTestProducerRef strictly parses a producer ref against the SI-228
// grammar `go-test:<package path relative to the module root>:<top-level
// TestName>`, returning why a ref does not match. The package segment is one
// or more canonical import-path elements (checkGoTestPackagePath); the test
// segment is one Go identifier that is a test function name (isGoTestName),
// so it can name neither a subtest, an example, a benchmark, nor a fuzz
// target, and can never carry a regexp metacharacter into `-run`.
func parseGoTestProducerRef(ref string) (goTestProducerRef, error) {
	rest, ok := strings.CutPrefix(ref, "go-test:")
	if !ok {
		return goTestProducerRef{}, errors.New(`the scheme is not "go-test:"`)
	}
	pkg, test, ok := strings.Cut(rest, ":")
	if !ok {
		return goTestProducerRef{}, errors.New("there is no test segment")
	}
	if strings.Contains(test, ":") {
		return goTestProducerRef{}, errors.New("there is more than one test segment")
	}
	if err := checkGoTestPackagePath(pkg); err != nil {
		return goTestProducerRef{}, err
	}
	if !isGoTestName(test) {
		return goTestProducerRef{}, fmt.Errorf("the test segment %q is not a top-level Go test function name", test)
	}
	return goTestProducerRef{Package: pkg, Test: test}, nil
}

// checkGoTestPackagePath accepts a package path relative to the module root
// made of canonical import-path elements: no empty element (so no leading,
// trailing, or doubled slash), no element beginning or ending with a dot (so
// no ".", "..", or "..." and no hidden directory), and only the characters
// Go permits in an import-path element (golang.org/x/mod/module's
// importPathOK and checkElem, vendored in Go 1.25.5 at
// cmd/vendor/golang.org/x/mod/module/module.go; the leading-dot rule is
// stricter than import paths require). Such a path is always path.Clean's
// fixed point and never escapes the module root.
func checkGoTestPackagePath(pkg string) error {
	if pkg == "" {
		return errors.New("the package path is empty")
	}
	for _, elem := range strings.Split(pkg, "/") {
		if elem == "" {
			return fmt.Errorf("the package path %q has an empty element", pkg)
		}
		if elem[0] == '.' || elem[len(elem)-1] == '.' {
			return fmt.Errorf("the package path element %q begins or ends with a dot", elem)
		}
		for _, r := range elem {
			if !importPathElementRune(r) {
				return fmt.Errorf("the package path element %q has the character %q, which is not allowed in a Go import path", elem, r)
			}
		}
	}
	return nil
}

// importPathElementRune is x/mod's importPathOK: ASCII letters and digits,
// and - . _ ~ +.
func importPathElementRune(r rune) bool {
	return r == '-' || r == '.' || r == '_' || r == '~' || r == '+' ||
		'0' <= r && r <= '9' || 'A' <= r && r <= 'Z' || 'a' <= r && r <= 'z'
}

// isGoTestName reports whether name is a Go identifier (go/token) that go
// test runs as a test function: "Test", or "Test" followed by a rune that is
// not lower-case (cmd/go/internal/load's isTest, Go 1.25.5).
func isGoTestName(name string) bool {
	if !token.IsIdentifier(name) || !strings.HasPrefix(name, "Test") {
		return false
	}
	if len(name) == len("Test") {
		return true
	}
	r, _ := utf8.DecodeRuneInString(name[len("Test"):])
	return !unicode.IsLower(r)
}

// nestedModuleDir reports the first directory on pkg's path below root that
// holds its own go.mod: a package there belongs to another module, never to
// the module root the producer ref's package path is relative to.
func nestedModuleDir(root, pkg string) (string, bool) {
	elems := strings.Split(pkg, "/")
	for i := range elems {
		dir := strings.Join(elems[:i+1], "/")
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(dir), "go.mod")); err == nil {
			return dir, true
		}
	}
	return "", false
}

// --- Discovery (contract 2, part 1: find every candidate) --------------

// testProducerCandidate is one elaborated, test-kind, ci-job-sourced
// obligation found anywhere in the store — before this job's own
// authoritative_source match (selectGoTestObligations) or producer-ref
// grammar check (parseGoTestProducerRef) have been applied.
type testProducerCandidate struct {
	// SpecName is the obligation's own owning story spec (the id's
	// story-slug segment, spec/obligation-artifact DC-2) — where the
	// emitted record is ultimately written, derived/<RefSlug(spec/
	// SpecName)>/<commit>/verdicts.json.
	SpecName string
	ACID     string
	Kind     artifact.EvidenceKind
	// ProducerRef is the obligation's declared quality.producer.ref —
	// unparsed; grammar validity is checked separately (selection, not
	// discovery), so a malformed ref still reaches selection where its
	// disclosure can name the obligation ID it came from.
	ProducerRef string
	// JobRef is the obligation's declared quality.authoritative_source.ref
	// (SI-229): compared against the running CI job's own declared name.
	JobRef string
	// ObligationID is the decoded obligation's own id (e.g.
	// "obligation/stale-decline--ac-1--behavioral"), used only to name the
	// obligation in a disclosure.
	ObligationID string
}

// testProducerObligationUnreadableSource is the disclosure source for an
// obligation file this discovery walk could not read or decode — a
// pre-existing authoring fault elsewhere in the store, never this job's
// concern to repair, and never allowed to fail sync --produce for every
// other story (see this file's package doc).
const testProducerObligationUnreadableSource = "sync:go-test-producer-obligation-unreadable"

func testProducerObligationUnreadableDisclosure(root, path string, err error) disclosure.Disclosure {
	rel, relErr := filepath.Rel(root, path)
	if relErr != nil {
		rel = path
	}
	text := fmt.Sprintf("could not be read or decoded as an obligation (%v); no per-test evidence was considered for it", err)
	return disclosure.New(testProducerObligationUnreadableSource, filepath.ToSlash(rel), text)
}

// testProducerObligationMisfiledSource is the disclosure source for a
// decodable obligation file that is not at the convention path its own id
// names (store.ObligationPath), which is the only path the matcher reads.
const testProducerObligationMisfiledSource = "sync:go-test-producer-obligation-misfiled"

func testProducerObligationMisfiledDisclosure(root, path, conventionPath string) disclosure.Disclosure {
	rel, relErr := filepath.Rel(root, path)
	if relErr != nil {
		rel = path
	}
	wantRel, relErr := filepath.Rel(root, conventionPath)
	if relErr != nil {
		wantRel = conventionPath
	}
	text := fmt.Sprintf("is not at %s, the convention path its id names and the only path the matcher reads; no per-test evidence was considered for it", filepath.ToSlash(wantRel))
	return disclosure.New(testProducerObligationMisfiledSource, filepath.ToSlash(rel), text)
}

// discoverTestProducerObligations walks every spec directory under
// .verdi/obligations/ and returns every elaborated obligation whose
// declared producer kind is "test" and whose authoritative source kind is
// "ci-job" (03 §Declarations: checker and authenticated-human producers
// have no per-test emitter; an unresolved-design-debt or legacy-
// unelaborated obligation is never a candidate). A missing .verdi/
// obligations/ tree is the ordinary "no obligations authored yet" case
// (evidence.Obligations's own absence posture), not an error. An
// individual obligation file that cannot be read or decoded, or that is not
// at the convention path its own id names, is disclosed and skipped, never
// fails the whole walk (see package doc).
func discoverTestProducerObligations(root string) ([]testProducerCandidate, []disclosure.Disclosure, error) {
	obligationsRoot := filepath.Join(root, ".verdi", "obligations")
	specDirs, err := os.ReadDir(obligationsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("go-test producer: reading %s: %w", obligationsRoot, err)
	}
	sort.Slice(specDirs, func(i, j int) bool { return specDirs[i].Name() < specDirs[j].Name() })

	var candidates []testProducerCandidate
	var discl []disclosure.Disclosure
	for _, sd := range specDirs {
		if !sd.IsDir() {
			continue
		}
		dir := filepath.Join(obligationsRoot, sd.Name())
		files, err := os.ReadDir(dir)
		if err != nil {
			return nil, nil, fmt.Errorf("go-test producer: reading %s: %w", dir, err)
		}
		sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
				continue
			}
			path := filepath.Join(dir, f.Name())
			raw, err := os.ReadFile(path)
			if err != nil {
				discl = append(discl, testProducerObligationUnreadableDisclosure(root, path, err))
				continue
			}
			fm, _, err := artifact.SplitFrontmatter(raw)
			if err != nil {
				discl = append(discl, testProducerObligationUnreadableDisclosure(root, path, err))
				continue
			}
			decoded, err := artifact.DecodeObligation(fm)
			if err != nil {
				discl = append(discl, testProducerObligationUnreadableDisclosure(root, path, err))
				continue
			}
			if decoded.Quality == nil || decoded.Quality.State != artifact.ObligationQualityElaborated {
				continue
			}
			if decoded.Quality.Producer.Kind != artifact.ObligationProducerTest {
				continue
			}
			if decoded.Quality.AuthoritativeSource.Kind != artifact.ObligationSourceCIJob {
				continue
			}
			ref, err := artifact.ParseRef(decoded.ID)
			if err != nil {
				discl = append(discl, testProducerObligationUnreadableDisclosure(root, path, err))
				continue
			}
			storySlug, acID, kind, ok := artifact.SplitObligationName(ref.Name)
			if !ok {
				discl = append(discl, testProducerObligationUnreadableDisclosure(root, path, fmt.Errorf("id %q does not split into <story-slug>--<ac-id>--<for-kind>", decoded.ID)))
				continue
			}
			// The matcher reads an obligation only at its convention path
			// (evidence.AssessObligation); a copy anywhere else is not the
			// obligation it assesses, so it produces nothing here either.
			if conventionPath := store.ObligationPath(root, storySlug, acID, kind); path != conventionPath {
				discl = append(discl, testProducerObligationMisfiledDisclosure(root, path, conventionPath))
				continue
			}
			candidates = append(candidates, testProducerCandidate{
				SpecName:     storySlug,
				ACID:         acID,
				Kind:         decoded.ForKind,
				ProducerRef:  decoded.Quality.Producer.Ref,
				JobRef:       decoded.Quality.AuthoritativeSource.Ref,
				ObligationID: decoded.ID,
			})
		}
	}
	return candidates, discl, nil
}

// --- Selection (contract 2, part 2: this job's own candidates) ---------

// selectedGoTestObligation is one candidate this CI job is authoritative
// for, with its producer ref already grammar-parsed into (package, test).
type selectedGoTestObligation struct {
	testProducerCandidate
	Package string
	Test    string
}

// goTestProducerMalformedRefSource is the disclosure source for an
// elaborated test-producer obligation whose producer ref fails the SI-228
// grammar, or names a package inside a nested module (contract 1: "never an
// operational error that breaks CI for every story").
const goTestProducerMalformedRefSource = "sync:go-test-producer-malformed-ref"

func goTestProducerMalformedRefDisclosure(c testProducerCandidate, reason error) disclosure.Disclosure {
	text := fmt.Sprintf("declares producer %q, which does not match the go-test:<package>:<TopLevelTestName> grammar (%v); no evidence record was emitted for it", c.ProducerRef, reason)
	return disclosure.New(goTestProducerMalformedRefSource, c.ObligationID, text)
}

// goTestProducerRuntimeKindSource is the disclosure source for a runtime-kind
// obligation naming a test producer: runtime evidence is post-deploy (03
// §Evidence kinds), so this pre-merge job emits no per-test record for it.
const goTestProducerRuntimeKindSource = "sync:go-test-producer-runtime-kind"

func goTestProducerRuntimeKindDisclosure(c testProducerCandidate) disclosure.Disclosure {
	text := fmt.Sprintf("declares producer %q for a runtime-kind obligation; runtime evidence exists only after deployment (03 §Evidence kinds), so this CI job emitted no record for it", c.ProducerRef)
	return disclosure.New(goTestProducerRuntimeKindSource, c.ObligationID, text)
}

// selectGoTestObligations narrows candidates to exactly this CI job's own
// authoritative obligations (SI-229: authoritative_source.ref ==
// jobName), excludes a runtime-kind obligation (03 §Evidence kinds: runtime
// evidence is post-deploy), then grammar-parses each survivor's producer ref
// and rejects a package path that crosses into a nested module under root.
// jobName == "" (not running in a named CI job at all) naturally selects
// nothing, since an elaborated obligation's authoritative_source.ref is
// always non-blank (internal/artifact/obligation.go's Validate). A rejected
// ref is disclosed on its own obligation and excluded, never returned as an
// error, and never reaches the `-run` expression another obligation's test
// shares. The returned slice is sorted (package, test, spec, ac) for
// deterministic execution grouping and emission order.
func selectGoTestObligations(root string, candidates []testProducerCandidate, jobName string) ([]selectedGoTestObligation, []disclosure.Disclosure) {
	var selected []selectedGoTestObligation
	var discl []disclosure.Disclosure
	for _, c := range candidates {
		if jobName == "" || c.JobRef != jobName {
			continue
		}
		if c.Kind == artifact.EvidenceRuntime {
			discl = append(discl, goTestProducerRuntimeKindDisclosure(c))
			continue
		}
		parsed, err := parseGoTestProducerRef(c.ProducerRef)
		if err == nil {
			if dir, nested := nestedModuleDir(root, parsed.Package); nested {
				err = fmt.Errorf("the package path enters the nested module at %s", dir)
			}
		}
		if err != nil {
			discl = append(discl, goTestProducerMalformedRefDisclosure(c, err))
			continue
		}
		selected = append(selected, selectedGoTestObligation{testProducerCandidate: c, Package: parsed.Package, Test: parsed.Test})
	}
	sort.Slice(selected, func(i, j int) bool {
		a, b := selected[i], selected[j]
		if a.Package != b.Package {
			return a.Package < b.Package
		}
		if a.Test != b.Test {
			return a.Test < b.Test
		}
		if a.SpecName != b.SpecName {
			return a.SpecName < b.SpecName
		}
		return a.ACID < b.ACID
	})
	return selected, discl
}

// --- Execution (contract 3) ---------------------------------------------

// namedGoTestRunner abstracts one `go test -json -count=1 -run <pattern>
// <pkgArg>` invocation, restricted to an explicit package and run pattern —
// the per-test producer's own execution seam, distinct from goTestRunner's
// whole-module `./...` run in sync_regen.go (CLAUDE.md: no exec in any
// test; hermetic fakes only in unit tests, the real runner only in
// production and in testproducer_integration_test.go).
type namedGoTestRunner interface {
	// RunNamedGoTest runs `go test -json -count=1 -run <runPattern>
	// <pkgArg>` with dir as the module root and returns its stdout. A
	// failing or skipped test, or a package that fails to build, is not an
	// error here — the stream reports it; only a truly broken invocation is.
	RunNamedGoTest(ctx context.Context, dir, pkgArg, runPattern string) ([]byte, error)
}

// realNamedGoTestRunner execs the real toolchain.
type realNamedGoTestRunner struct{}

func (realNamedGoTestRunner) RunNamedGoTest(ctx context.Context, dir, pkgArg, runPattern string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "go", "test", "-json", "-count=1", "-run", runPattern, pkgArg)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// A nonzero exit is how go test reports a failing test or an unbuildable
	// package, so runErr alone is not a failure: the stream says what ran.
	runErr := cmd.Run()
	// A run killed by its context leaves a partial stream, and os/exec
	// reports the kill as an *exec.ExitError rather than the context's
	// error (Cmd.Wait prefers the process's own error), so check it first.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, fmt.Errorf("go test -json for package %s: %w", pkgArg, ctxErr)
	}
	if stdout.Len() == 0 {
		if runErr == nil {
			runErr = errors.New("exit status 0")
		}
		return nil, fmt.Errorf("go test -json for package %s produced no output: %w (stderr: %s)", pkgArg, runErr, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// goTestRunPattern builds the `-run` expression from a package's wanted
// top-level test names: "^(A|B|...)$", each name regexp-quoted and the
// whole alternation anchored at both ends so each name matches only itself,
// sorted for a deterministic, reproducible command line. Every name already
// passed parseGoTestProducerRef, so quoting is defence in depth.
func goTestRunPattern(tests []string) string {
	quoted := make([]string, len(tests))
	for i, t := range tests {
		quoted[i] = regexp.QuoteMeta(t)
	}
	sort.Strings(quoted)
	return "^(" + strings.Join(quoted, "|") + ")$"
}

// goTestPackageArg is the go test argument for a package path relative to
// the module root: the package's directory, as internal/publicrelease's
// runner passes it.
func goTestPackageArg(pkg string) string { return "./" + pkg }

// goModulePath reports the module path root/go.mod declares, and whether
// root is a Go module root at all. The go command reports every test2json
// event's Package as the package's full import path — the module path joined
// with the package directory relative to the module root
// (cmd/go/internal/test: test2json.NewConverter(..., p.ImportPath, ...)) —
// and "the module root directory is the directory that contains the go.mod
// file" (Go Modules Reference, go.mod files). No go.mod at root is
// present == false with no error; a go.mod that cannot be read, or that does
// not declare exactly one module path, is an error.
func goModulePath(root string) (modulePath string, present bool, err error) {
	raw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("go-test producer: reading go.mod: %w", err)
	}
	modulePath, err = parseModuleDirective(raw)
	if err != nil {
		return "", true, fmt.Errorf("go-test producer: %s: %w", filepath.Join(root, "go.mod"), err)
	}
	return modulePath, true, nil
}

// parseModuleDirective extracts the one module path a go.mod declares, per
// the Go Modules Reference grammar
// `ModuleDirective = "module" ( ModulePath | "(" newline ModulePath newline ")" ) newline`,
// where `//` starts a comment and a ModulePath may be a quoted string.
func parseModuleDirective(data []byte) (string, error) {
	fields := func(line string) []string {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		return strings.Fields(line)
	}
	lines := strings.Split(string(data), "\n")
	var paths []string
	for i := 0; i < len(lines); i++ {
		f := fields(lines[i])
		if len(f) == 0 || f[0] != "module" {
			continue
		}
		if len(f) != 2 {
			return "", fmt.Errorf("malformed module directive %q", strings.TrimSpace(lines[i]))
		}
		if f[1] != "(" {
			paths = append(paths, f[1])
			continue
		}
		closed := false
		for i++; i < len(lines); i++ {
			g := fields(lines[i])
			if len(g) == 1 && g[0] == ")" {
				closed = true
				break
			}
			if len(g) > 1 {
				return "", fmt.Errorf("malformed module block line %q", strings.TrimSpace(lines[i]))
			}
			paths = append(paths, g...)
		}
		if !closed {
			return "", errors.New("unterminated module block")
		}
	}
	if len(paths) != 1 {
		return "", fmt.Errorf("want exactly one module path, found %d", len(paths))
	}
	p := paths[0]
	if strings.HasPrefix(p, `"`) || strings.HasPrefix(p, "`") {
		unquoted, err := strconv.Unquote(p)
		if err != nil {
			return "", fmt.Errorf("malformed quoted module path %s: %w", p, err)
		}
		p = unquoted
	}
	if p == "" || strings.ContainsAny(p, " \t\r\n") {
		return "", fmt.Errorf("invalid module path %q", p)
	}
	return p, nil
}

// executeGoTestProducers groups selected by package, runs exactly one `go
// test -json` per distinct package (restricted to the union of that
// package's wanted top-level test names), and reads each run's stream with
// the shared reader (internal/gotestjson) against that package's full import
// path — modulePath joined with the relative package path, as
// internal/publicrelease's runner joins them — or, when the go command
// cannot load the package, the argument it was given. Returns each package's
// Result keyed by the relative package path. A runner or read failure is
// returned as an operational error, never swallowed.
func executeGoTestProducers(ctx context.Context, root, modulePath string, runner namedGoTestRunner, selected []selectedGoTestObligation) (map[string]gotestjson.Result, error) {
	byPkg := map[string]map[string]bool{}
	var pkgOrder []string
	for _, s := range selected {
		if byPkg[s.Package] == nil {
			byPkg[s.Package] = map[string]bool{}
			pkgOrder = append(pkgOrder, s.Package)
		}
		byPkg[s.Package][s.Test] = true
	}
	sort.Strings(pkgOrder)

	results := make(map[string]gotestjson.Result, len(pkgOrder))
	for _, pkg := range pkgOrder {
		tests := make([]string, 0, len(byPkg[pkg]))
		for t := range byPkg[pkg] {
			tests = append(tests, t)
		}
		arg := goTestPackageArg(pkg)
		out, err := runner.RunNamedGoTest(ctx, root, arg, goTestRunPattern(tests))
		if err != nil {
			return nil, fmt.Errorf("go-test producer: %w", err)
		}
		res, err := gotestjson.ReadPackage(bytes.NewReader(out), gotestjson.Target{ImportPath: modulePath + "/" + pkg, Arg: arg})
		if err != nil {
			return nil, fmt.Errorf("go-test producer: %w", err)
		}
		results[pkg] = res
	}
	return results, nil
}

// --- Emission (contract 5) ----------------------------------------------

// goTestProducerAbsentSource is the disclosure source for a selected
// obligation's named test that did not run — renamed, removed, in a package
// that did not build or load, or otherwise never reaching its own terminal
// event — contract 1/4: no record, a disclosure naming the obligation, and
// the obligation reads producer-missing at fold time, never a silent pass. An
// earlier record for its producer at the same commit is withdrawn, and the
// disclosure says so (SI-238, discloseGoTestAbsence).
const goTestProducerAbsentSource = "sync:go-test-producer-absent"

func goTestProducerAbsentDisclosure(s selectedGoTestObligation) disclosure.Disclosure {
	text := fmt.Sprintf("named test %s in package %s did not run (no terminal event); no evidence record was emitted for it", s.Test, s.Package)
	return disclosure.New(goTestProducerAbsentSource, s.ObligationID, text)
}

func goTestProducerNotBuiltDisclosure(s selectedGoTestObligation, res gotestjson.Result) disclosure.Disclosure {
	cause := "the go command could not load it"
	if res.Loaded {
		cause = "it failed to build"
	}
	text := fmt.Sprintf("named test %s did not run: package %s did not build or load (%s); no evidence record was emitted for it", s.Test, s.Package, cause)
	return disclosure.New(goTestProducerAbsentSource, s.ObligationID, text)
}

func goTestProducerNoModuleDisclosure(s selectedGoTestObligation) disclosure.Disclosure {
	text := fmt.Sprintf("named test %s in package %s did not run: the store root has no go.mod, so it is not the Go module root the package path is relative to; no evidence record was emitted for it", s.Test, s.Package)
	return disclosure.New(goTestProducerAbsentSource, s.ObligationID, text)
}

// goTestAbsence is one selected obligation this run recorded nothing for, with
// the disclosure that says why. The disclosure is final only once the write
// has reported what it withdrew (discloseGoTestAbsence).
type goTestAbsence struct {
	obligation selectedGoTestObligation
	disclosed  disclosure.Disclosure
}

// discloseGoTestAbsence returns a's disclosure, adding a note naming the
// earlier records this run withdrew for a's own producer in a's own spec
// (SI-238); withdrawn is writeManagedEvidence's report, keyed by spec ref. It
// adds nothing when no such record was withdrawn.
func discloseGoTestAbsence(a goTestAbsence, withdrawn map[string][]artifact.Evidence) disclosure.Disclosure {
	var verdicts []string
	for _, r := range withdrawn[goTestSpecRef(a.obligation.SpecName)] {
		if r.Producer == a.obligation.ProducerRef {
			verdicts = append(verdicts, string(r.Verdict))
		}
	}
	var note string
	switch len(verdicts) {
	case 0:
		return a.disclosed
	case 1:
		note = fmt.Sprintf("the earlier %s record for this producer at this commit was withdrawn", verdicts[0])
	default:
		note = fmt.Sprintf("the %d earlier records for this producer at this commit (%s) were withdrawn", len(verdicts), strings.Join(verdicts, ", "))
	}
	return disclosure.New(a.disclosed.Source, a.disclosed.Scope, a.disclosed.Text+"; "+note)
}

// goTestSpecRef is the owning spec ref of an obligation's story slug: the
// "spec/" + name form writeManagedEvidence keys on (store.RefSlug) and
// evidence.AssessObligation's own wantVerifies uses.
func goTestSpecRef(specName string) string { return "spec/" + specName }

// goTestManagedSubset is one production run's managed subset (SI-238): per
// owning spec ref, the producer ref of every obligation selected for this job
// at this commit, whether or not the run records anything for it.
func goTestManagedSubset(selected []selectedGoTestObligation) map[string]map[string]bool {
	managed := map[string]map[string]bool{}
	for _, s := range selected {
		specRef := goTestSpecRef(s.SpecName)
		if managed[specRef] == nil {
			managed[specRef] = map[string]bool{}
		}
		managed[specRef][s.ProducerRef] = true
	}
	return managed
}

// verdictForOutcome maps a named test's own terminal test2json action to
// an evidence verdict (contract 5: "pass only on that test's own terminal
// pass event, fail on its fail event, abstain when it was skipped").
func verdictForOutcome(action string) (artifact.EvidenceVerdict, error) {
	switch action {
	case gotestjson.ActionPass:
		return artifact.VerdictPass, nil
	case gotestjson.ActionFail:
		return artifact.VerdictFail, nil
	case gotestjson.ActionSkip:
		return artifact.VerdictAbstain, nil
	default:
		return "", fmt.Errorf("go-test producer: unrecognized test outcome %q", action)
	}
}

// namedTestDigest hashes rec's declared content — kind, producer, the AC
// set it attests, and (unlike selfHostedDigest's always-pass records) the
// verdict itself, which genuinely varies per run — mirroring
// internal/bundle's recordDigest posture (02 §Generated artifacts and
// digests): a content-address of the upstream fact this record asserts.
func namedTestDigest(rec artifact.Evidence) (string, error) {
	keyed := struct {
		Kind        artifact.EvidenceKind    `json:"kind"`
		Producer    string                   `json:"producer"`
		EvidenceFor []string                 `json:"evidence_for"`
		Verdict     artifact.EvidenceVerdict `json:"verdict"`
	}{Kind: rec.Kind, Producer: rec.Producer, EvidenceFor: rec.EvidenceFor, Verdict: rec.Verdict}
	digest, err := canonjson.Digest(keyed)
	if err != nil {
		return "", fmt.Errorf("go-test producer: computing digest: %w", err)
	}
	return digest, nil
}

// buildGoTestRecords turns each selected obligation's outcome (its named
// test's own terminal action in its package's Result) into its own evidence
// record, grouped by owning spec ref — the shape writeManagedEvidence writes
// into the owning spec's own derived/<...>/verdicts.json. A selected
// obligation whose test did not run — its package did not build or load, or
// its test never reached a terminal event — is returned as an absence with
// its disclosure, never an error.
func buildGoTestRecords(selected []selectedGoTestObligation, results map[string]gotestjson.Result, prov artifact.EvidenceProvenance) (map[string][]artifact.Evidence, []goTestAbsence, error) {
	bySpec := map[string][]artifact.Evidence{}
	var absent []goTestAbsence
	for _, s := range selected {
		res, ok := results[s.Package]
		if !ok {
			return nil, nil, fmt.Errorf("go-test producer: no run recorded for package %s", s.Package)
		}
		if !res.Built() {
			absent = append(absent, goTestAbsence{obligation: s, disclosed: goTestProducerNotBuiltDisclosure(s, res)})
			continue
		}
		outcome, ok := res.Tests[s.Test]
		if !ok {
			absent = append(absent, goTestAbsence{obligation: s, disclosed: goTestProducerAbsentDisclosure(s)})
			continue
		}
		verdict, err := verdictForOutcome(outcome)
		if err != nil {
			return nil, nil, err
		}
		rec := artifact.Evidence{
			Schema:      "verdi.evidence/v1",
			EvidenceFor: []string{s.ACID},
			Kind:        s.Kind,
			Verdict:     verdict,
			Witness:     fmt.Sprintf("go test -json -count=1 -run ... ./%s: %s %s", s.Package, s.Test, outcome),
			Producer:    s.ProducerRef,
			Provenance:  prov,
		}
		digest, err := namedTestDigest(rec)
		if err != nil {
			return nil, nil, err
		}
		rec.Digest = digest
		// writeManagedEvidence (and its own store.RefSlug keying) expects a
		// full spec ref, not the bare story slug SplitObligationName returns.
		specRef := goTestSpecRef(s.SpecName)
		bySpec[specRef] = append(bySpec[specRef], rec)
	}
	return bySpec, absent, nil
}

// --- Orchestration --------------------------------------------------------

// produceGoTestEvidence is `sync --produce`'s per-test CI evidence
// producer (see this file's package doc). Called from runProduce
// alongside produceSelfHostedEvidence, sharing the same provenance and
// commit. jobName is the running CI job's own declared name
// (forge.CIInfo.JobName, SI-229) — empty outside a detected CI job, which
// naturally selects zero obligations (an elaborated obligation's
// authoritative_source.ref is never blank) and makes this a silent no-op,
// matching contract 2's "only inside sync --produce in CI" without a
// separate gate. Every disclosure collected along the way (a malformed,
// nested-module, or runtime-kind ref; an undecodable or misfiled
// obligation elsewhere in the store; a store root that is not a module
// root; a package that did not build or load; or a named test that never
// ran) is rendered to stdout. Only a missing runner, a runner or stream
// failure, an unreadable go.mod, an undecodable existing verdicts.json, or a
// failed write returns an error (the caller's exit 2), and every error but a
// failed write returns before anything is written.
//
// REPEAT PRODUCTION (SI-238). 03 §The fold takes the latest run's verdict per
// (kind, producer), so each run replaces its managed subset at this commit
// (goTestManagedSubset): in every spec with a selected obligation, every
// existing record whose producer is a selected obligation's producer ref is
// dropped and this run's records are added. A selected producer this run did
// not record therefore loses any earlier record at this commit, and its
// disclosure says what was withdrawn; every other record stays unchanged.
func produceGoTestEvidence(ctx context.Context, root, commit, jobName string, runner namedGoTestRunner, prov artifact.EvidenceProvenance, stdout io.Writer) error {
	candidates, discl, err := discoverTestProducerObligations(root)
	if err != nil {
		return err
	}
	selected, selectDiscl := selectGoTestObligations(root, candidates, jobName)
	discl = append(discl, selectDiscl...)

	if len(selected) > 0 {
		managed := goTestManagedSubset(selected)
		modulePath, isModuleRoot, err := goModulePath(root)
		if err != nil {
			return err
		}
		var absent []goTestAbsence
		if !isModuleRoot {
			// A producer ref's package path is relative to the module root; a
			// store root with no go.mod has none, so no named test can run.
			for _, s := range selected {
				absent = append(absent, goTestAbsence{obligation: s, disclosed: goTestProducerNoModuleDisclosure(s)})
			}
			selected = nil
		}
		if len(selected) > 0 && runner == nil {
			return errors.New("go-test producer: no test runner is configured")
		}
		results, err := executeGoTestProducers(ctx, root, modulePath, runner, selected)
		if err != nil {
			return err
		}
		bySpec, buildAbsent, err := buildGoTestRecords(selected, results, prov)
		if err != nil {
			return err
		}
		absent = append(absent, buildAbsent...)
		withdrawn, err := writeManagedEvidence(root, commit, managed, bySpec)
		if err != nil {
			return err
		}
		for _, a := range absent {
			discl = append(discl, discloseGoTestAbsence(a, withdrawn))
		}
	}

	for _, d := range discl {
		fmt.Fprintln(stdout, disclosure.Render(d))
	}
	return nil
}
