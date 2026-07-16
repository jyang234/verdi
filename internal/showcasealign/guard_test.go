package showcasealign

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestShowcaseCoverage_GuardScriptBites is the committed red-direction proof of
// scripts/require-pass.sh — the PASS-line predicate the lint-showcase and
// showcase-coverage make guards delegate to. AC-1's behavioral leg is that
// `make verify` FAILS naming the capability; what makes verify bite rather than
// pass vacuously is that predicate, and a name-only inline guard could never
// prove its own red direction (the finding's outermost layer). By extracting it
// to a script and driving it here with canned `go test -v` transcripts, that red
// direction is a committed, hermetic test: a required name with no `--- PASS:`
// line makes the script exit 1 and name it; a suffix-renamed test does NOT
// satisfy the base name (the trailing " (" anchor); a complete transcript exits
// 0. No `go test` recursion, no network.
func TestShowcaseCoverage_GuardScriptBites(t *testing.T) {
	script := filepath.Join(verdiRepoRoot, "scripts", "require-pass.sh")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("guard script scripts/require-pass.sh not found: %v", err)
	}
	transcript := "=== RUN   TestA\n--- PASS: TestA (0.00s)\n=== RUN   TestB\n--- PASS: TestB (0.01s)\nPASS\nok  \tpkg\t0.1s\n"

	t.Run("all required present yields exit 0", func(t *testing.T) {
		code, stderr := runGuardScript(t, script, "TestA TestB", transcript)
		if code != 0 {
			t.Errorf("exit = %d, want 0 (both required tests emitted --- PASS); stderr: %s", code, stderr)
		}
	})

	t.Run("a required test with no PASS line yields exit 1 naming it", func(t *testing.T) {
		code, stderr := runGuardScript(t, script, "TestA TestGone", transcript)
		if code != 1 {
			t.Errorf("exit = %d, want 1 (TestGone never emitted a --- PASS line)", code)
		}
		if !strings.Contains(stderr, "TestGone") {
			t.Errorf("stderr = %q, want it to name the missing test TestGone", stderr)
		}
	})

	t.Run("a suffix-renamed test does not satisfy the base name (exit 1)", func(t *testing.T) {
		// Precision: "--- PASS: TestAX (" must NOT satisfy a requirement for
		// "TestA" — the trailing space+paren anchor is what makes a rename bite.
		renamed := "=== RUN   TestAX\n--- PASS: TestAX (0.00s)\nPASS\n"
		code, _ := runGuardScript(t, script, "TestA", renamed)
		if code != 1 {
			t.Errorf("exit = %d, want 1 (a --- PASS line for TestAX must not satisfy required TestA)", code)
		}
	})
}

// runGuardScript execs bash <script> <required> with stdin=transcript and
// returns its exit code and stderr.
func runGuardScript(t *testing.T, script, required, stdin string) (int, string) {
	t.Helper()
	cmd := exec.Command("bash", script, required)
	cmd.Stdin = strings.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return 0, stderr.String()
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), stderr.String()
	}
	t.Fatalf("running guard script %s: %v", script, err)
	return -1, ""
}

// TestShowcaseCoverage_RequiredListInSync closes the guard's silent
// under-inclusion: the showcase-coverage guard demands only the names listed in
// the Makefile's SHOWCASE_REQUIRED_TESTS variable, so a NEW TestShowcaseCoverage*
// test added to this package is selected and its verdict enforced by the
// unanchored `-run` pattern, yet its later DELETION is free unless someone also
// remembers to add it to that list. This test binds the two: it reads
// SHOWCASE_REQUIRED_TESTS from the committed Makefile and every top-level
// TestShowcaseCoverage* function in this package's *_test.go files, and fails if
// any such function is missing from the list — so a new coverage test cannot be
// added without also being made undeletable-without-notice.
func TestShowcaseCoverage_RequiredListInSync(t *testing.T) {
	mkPath := filepath.Join(verdiRepoRoot, "Makefile")
	data, err := os.ReadFile(mkPath)
	if err != nil {
		t.Fatalf("reading %s: %v", mkPath, err)
	}
	required := extractRequiredTests(string(data))
	if len(required) == 0 {
		t.Fatalf("SHOWCASE_REQUIRED_TESTS not found (or empty) in %s (Makefile shape changed)", mkPath)
	}

	pkgDir := filepath.Join(verdiRepoRoot, "internal", "showcasealign")
	funcs := showcaseCoverageTestFuncs(t, pkgDir)
	if len(funcs) == 0 {
		t.Fatalf("no top-level TestShowcaseCoverage* functions found in %s (shape changed?)", pkgDir)
	}

	for _, fn := range funcs {
		if !required[fn] {
			t.Errorf("test %s exists in internal/showcasealign but is absent from SHOWCASE_REQUIRED_TESTS in the Makefile — add it, or the showcase-coverage guard cannot demand it ran+passed (silent under-inclusion: its future deletion would be undetected)", fn)
		}
	}
}

// extractRequiredTests parses the `SHOWCASE_REQUIRED_TESTS := a b c` assignment
// out of the Makefile and returns the names as a set. Tolerant of surrounding
// whitespace; returns an empty set if the variable is absent.
func extractRequiredTests(makefile string) map[string]bool {
	const key = "SHOWCASE_REQUIRED_TESTS"
	out := map[string]bool{}
	for _, line := range strings.Split(makefile, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, key) {
			continue
		}
		rest := strings.TrimSpace(trimmed[len(key):])
		rest = strings.TrimPrefix(rest, ":=")
		rest = strings.TrimPrefix(rest, "=")
		for _, name := range strings.Fields(rest) {
			out[name] = true
		}
		break
	}
	return out
}

// showcaseCoverageTestFuncs returns every top-level (receiverless) test function
// named TestShowcaseCoverage* declared in pkgDir's *_test.go files.
func showcaseCoverageTestFuncs(t *testing.T, pkgDir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(pkgDir, "*_test.go"))
	if err != nil {
		t.Fatalf("globbing %s: %v", pkgDir, err)
	}
	var names []string
	fset := token.NewFileSet()
	for _, f := range files {
		parsed, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", f, err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			if strings.HasPrefix(fn.Name.Name, "TestShowcaseCoverage") {
				names = append(names, fn.Name.Name)
			}
		}
	}
	return names
}
