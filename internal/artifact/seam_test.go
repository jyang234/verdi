package artifact

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// moduleRoot resolves the directory containing this module's go.mod via
// `go env GOMOD`. Both seam tests run `go list ./...` from HERE — a test
// binary's cwd is its own package directory (internal/artifact), so an
// unanchored `./...` would only ever inspect internal/artifact's subtree
// and the module-wide guard would pass vacuously (the exact defect that
// let internal/lint import yaml.v3 unnoticed). A guard that silently
// passes is worse than no guard.
func moduleRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMOD").CombinedOutput()
	if err != nil {
		t.Fatalf("go env GOMOD: %v\n%s", err, out)
	}
	gomod := strings.TrimSpace(string(out))
	if gomod == "" || gomod == "/dev/null" || gomod == "NUL" {
		t.Fatalf("go env GOMOD = %q: not inside a module", gomod)
	}
	return filepath.Dir(gomod)
}

// TestYAMLImportSeam proves CLAUDE.md's "single import seam": across the
// whole module, only internal/artifact (and its own test binary) imports
// gopkg.in/yaml.v3. If any other package starts importing it directly,
// this test fails loudly rather than letting decode logic fork silently
// across packages.
func TestYAMLImportSeam(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool not on PATH")
	}

	// Which packages in this module import yaml.v3? Anchored at the module
	// root so `./...` genuinely means module-wide (see moduleRoot).
	cmd := exec.Command("go", "list", "-f", "{{.ImportPath}}: {{join .Imports \",\"}}", "./...")
	cmd.Dir = moduleRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -f: %v\n%s", err, out)
	}

	const yamlPkg = "gopkg.in/yaml.v3"
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		pkgPath, imports, ok := strings.Cut(line, ": ")
		if !ok {
			t.Fatalf("unexpected go list output line: %q", line)
		}
		if !strings.Contains(imports, yamlPkg) {
			continue
		}
		if !strings.HasSuffix(pkgPath, "/internal/artifact") && pkgPath != "internal/artifact" {
			t.Fatalf("package %q imports %s directly; only internal/artifact may (CLAUDE.md single import seam)", pkgPath, yamlPkg)
		}
	}
}

// TestYAMLImportSeam_TestFiles additionally checks _test.go files (go list
// -f {{.Imports}} covers only non-test files), since a stray import in a
// test file would still violate the seam's intent.
func TestYAMLImportSeam_TestFiles(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool not on PATH")
	}

	cmd := exec.Command("go", "list", "-f", "{{.ImportPath}} {{join .TestImports \",\"}} {{join .XTestImports \",\"}}", "./...")
	cmd.Dir = moduleRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -f: %v\n%s", err, out)
	}

	const yamlPkg = "gopkg.in/yaml.v3"
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, " ", 2)
		pkgPath := fields[0]
		rest := ""
		if len(fields) > 1 {
			rest = fields[1]
		}
		if !strings.Contains(rest, yamlPkg) {
			continue
		}
		if !strings.HasSuffix(pkgPath, "/internal/artifact") && pkgPath != "internal/artifact" {
			t.Fatalf("package %q imports %s in a test file; only internal/artifact may (CLAUDE.md single import seam)", pkgPath, yamlPkg)
		}
	}
}
