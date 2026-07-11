package artifact

import (
	"os/exec"
	"strings"
	"testing"
)

// TestYAMLImportSeam proves CLAUDE.md's "single import seam": across the
// whole module, only internal/artifact (and its own test binary) imports
// gopkg.in/yaml.v3. If any other package starts importing it directly,
// this test fails loudly rather than letting decode logic fork silently
// across packages.
func TestYAMLImportSeam(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool not on PATH")
	}

	out, err := exec.Command("go", "list", "-deps", "./...").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps ./...: %v\n%s", err, out)
	}
	_ = out // full dep list isn't per-package; use the importers form below.

	// go list -deps lists a flattened set with no per-importer attribution,
	// so ask directly: which packages in this module import yaml.v3?
	out, err = exec.Command("go", "list", "-f", "{{.ImportPath}}: {{join .Imports \",\"}}", "./...").CombinedOutput()
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
	out, err := exec.Command("go", "list", "-f", "{{.ImportPath}} {{join .TestImports \",\"}} {{join .XTestImports \",\"}}", "./...").CombinedOutput()
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
