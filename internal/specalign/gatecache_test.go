// Gate-cache honesty guard (ADJ-68, th-1). internal/showcasealign and
// internal/specalign both build the cmd/verdi binary in a subprocess
// (TestMain: `go build ./cmd/verdi`) and exec it. That subprocess build is
// invisible to the test binary's own buildID, so `go test` (even with -race,
// which does NOT defeat result caching) serves a STALE cached PASS after a
// cmd/verdi behavior change — empirically reproduced: a real exit-code change
// gave `ok (cached)` without -count=1 and FAIL with it. The Makefile therefore
// forces -count=1 (the documented cache bypass) for exactly the packages named
// in its CROSS_BINARY_PKGS list, keeping honest caching for the
// provably-not-blind majority.
//
// This file is that fix's decay guard: it fails if a test package OUTSIDE
// cmd/verdi builds+execs the cmd/verdi binary but is missing from
// CROSS_BINARY_PKGS — so the next cross-binary suite to land cannot silently
// reopen the vector. It lives in specalign (the repo self-audit home, cf.
// repo_hygiene_test.go) deliberately: specalign is itself a cross-binary
// cluster and so runs under -count=1, meaning this guard is never served stale
// even though it reads the Makefile and walks the tree at runtime (inputs the
// go test cache does not track). In-package cmd/verdi exec tests are NOT blind
// — their buildID covers cmd/verdi's own sources, so a cmd/verdi change
// invalidates their cache — and are deliberately excluded here and absent from
// CROSS_BINARY_PKGS.
package specalign

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// mentionsCmdVerdiPackage reports whether a string-literal value names the
// cmd/verdi *package directory* as a `go build`/`go run` target — "cmd/verdi",
// "./cmd/verdi", or any path ending "/cmd/verdi". A literal naming a FILE under
// the package (e.g. "cmd/verdi/matrix_test.go") is NOT a package target and
// does not count.
func mentionsCmdVerdiPackage(val string) bool {
	seg := strings.TrimPrefix(val, "./")
	return seg == "cmd/verdi" || strings.HasSuffix(seg, "/cmd/verdi")
}

// fileBuildsCmdVerdiBinary reports whether the Go source src builds (or runs)
// the cmd/verdi package binary in a subprocess — the cache-blind pattern
// (ADJ-68). It parses src and looks at STRING LITERALS only, requiring both a
// cmd/verdi package-target literal (mentionsCmdVerdiPackage) and a "build" or
// "run" go-subcommand literal. Working from literals means comment mentions of
// cmd/verdi (internal/lint's `// go run ./cmd/verdi lint`) and file-path
// strings (showcasealign's `goE2E("cmd/verdi/matrix_test.go")`) do not match,
// while the documented `exec.Command("go", "build", ..., "./cmd/verdi")` idiom
// — even with the target held in a variable that is itself a literal — does.
func fileBuildsCmdVerdiBinary(src string) (bool, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "src.go", src, 0)
	if err != nil {
		return false, err
	}
	var hasPkgTarget, hasBuildVerb bool
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		val, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		if mentionsCmdVerdiPackage(val) {
			hasPkgTarget = true
		}
		if val == "build" || val == "run" {
			hasBuildVerb = true
		}
		return true
	})
	return hasPkgTarget && hasBuildVerb, nil
}

// detectCrossBinaryTestPkgs walks root for *_test.go files and returns, sorted,
// the module-relative directories (slash-separated) of every package whose
// tests build+exec the cmd/verdi binary — EXCEPT cmd/verdi itself, whose
// in-package exec tests are cache-honest (their buildID covers cmd/verdi's
// sources). vendor/.git/node_modules are skipped.
func detectCrossBinaryTestPkgs(t *testing.T, root string) []string {
	t.Helper()
	seen := map[string]bool{}
	cmdVerdiDir := filepath.Join("cmd", "verdi")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", ".git", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		pkgDir := filepath.Dir(rel)
		if pkgDir == cmdVerdiDir {
			return nil // in-package cmd/verdi tests are not cache-blind
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		builds, err := fileBuildsCmdVerdiBinary(string(data))
		if err != nil {
			// A committed _test.go that will not parse is a real problem, not
			// something to skip past — surface it.
			t.Fatalf("parsing %s for the cross-binary build pattern: %v", rel, err)
		}
		if builds {
			seen[filepath.ToSlash(pkgDir)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s for cross-binary test packages: %v", root, err)
	}
	dirs := make([]string, 0, len(seen))
	for d := range seen {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs
}

// extractCrossBinaryPkgs parses the `CROSS_BINARY_PKGS := ./a/... ./b/...`
// assignment out of the Makefile and returns each entry normalized to a
// module-relative package directory ("./internal/specalign/..." ->
// "internal/specalign"). Twin of guard_test.go's extractRequiredTests; returns
// an empty set if the variable is absent.
func extractCrossBinaryPkgs(makefile string) map[string]bool {
	const key = "CROSS_BINARY_PKGS"
	out := map[string]bool{}
	for _, line := range strings.Split(makefile, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, key) {
			continue
		}
		rest := strings.TrimSpace(trimmed[len(key):])
		rest = strings.TrimPrefix(rest, ":=")
		rest = strings.TrimPrefix(rest, "=")
		for _, tok := range strings.Fields(rest) {
			norm := strings.TrimPrefix(tok, "./")
			norm = strings.TrimSuffix(norm, "...")
			norm = strings.TrimSuffix(norm, "/")
			if norm != "" {
				out[norm] = true
			}
		}
		break
	}
	return out
}

// TestGateCacheHonesty_FileBuildsCmdVerdiBinary is the detector's own
// happy-path + negative-path unit test: the real build idiom (direct and with
// the target in a variable) and `go run` match; a comment mention, a file-path
// string, a different package, and a bare package reference with no build/run
// verb do not.
func TestGateCacheHonesty_FileBuildsCmdVerdiBinary(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "exec go build ./cmd/verdi (the real idiom)",
			src: `package p
import "os/exec"
func f() { _ = exec.Command("go", "build", "-o", "/tmp/v", "./cmd/verdi") }`,
			want: true,
		},
		{
			name: "exec go build cmd/verdi without leading dot-slash",
			src: `package p
import "os/exec"
func f() { _ = exec.Command("go", "build", "cmd/verdi") }`,
			want: true,
		},
		{
			name: "go run ./cmd/verdi",
			src: `package p
import "os/exec"
func f() { _ = exec.Command("go", "run", "./cmd/verdi") }`,
			want: true,
		},
		{
			name: "build target held in a variable literal",
			src: `package p
import "os/exec"
func f() { tgt := "./cmd/verdi"; _ = exec.Command("go", "build", "-o", "/tmp/v", tgt) }`,
			want: true,
		},
		{
			name: "comment mention only (internal/lint shape)",
			src: `package p
// This file is the "go run ./cmd/verdi lint exits 0" proof.
func f() {}`,
			want: false,
		},
		{
			name: "file path under cmd/verdi is not a package target (coverage shape)",
			src: `package p
func f() string { return "cmd/verdi/matrix_test.go" }`,
			want: false,
		},
		{
			name: "builds a different command package",
			src: `package p
import "os/exec"
func f() { _ = exec.Command("go", "build", "./cmd/other") }`,
			want: false,
		},
		{
			name: "names the package path but has no build/run verb",
			src: `package p
func f() string { return "./cmd/verdi" }`,
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := fileBuildsCmdVerdiBinary(tc.src)
			if err != nil {
				t.Fatalf("fileBuildsCmdVerdiBinary parse error: %v", err)
			}
			if got != tc.want {
				t.Errorf("fileBuildsCmdVerdiBinary = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestGateCacheHonesty_CrossBinaryPkgsListInSync is the decay guard: every test
// package this repo actually contains that builds+execs cmd/verdi from outside
// cmd/verdi MUST be named in the Makefile's CROSS_BINARY_PKGS, or `make test` /
// `make verify` would serve it a stale cached PASS after a cmd/verdi behavior
// change (ADJ-68). It also asserts the two clusters ADJ-68 proved blind are
// still detected (so a silently-broken detector is itself caught) and that the
// list is actually wired to a -count=1 invocation (so defining the variable but
// not consuming it is caught too).
func TestGateCacheHonesty_CrossBinaryPkgsListInSync(t *testing.T) {
	mkPath := filepath.Join(verdiRepoRoot, "Makefile")
	data, err := os.ReadFile(mkPath)
	if err != nil {
		t.Fatalf("reading %s: %v", mkPath, err)
	}
	makefile := string(data)

	listed := extractCrossBinaryPkgs(makefile)
	if len(listed) == 0 {
		t.Fatalf("CROSS_BINARY_PKGS not found (or empty) in %s — the ADJ-68 cache-bypass list is the load-bearing fix; its absence means the gate can serve stale cross-binary greens", mkPath)
	}

	detected := detectCrossBinaryTestPkgs(t, verdiRepoRoot)
	if len(detected) == 0 {
		t.Fatalf("detected no packages building cmd/verdi from outside cmd/verdi — the detector or repo layout changed; at minimum internal/specalign and internal/showcasealign build it")
	}

	// Floor: the two clusters ADJ-68 proved blind must always be detected, so a
	// detector that silently stops matching cannot pass this guard vacuously.
	for _, want := range []string{"internal/showcasealign", "internal/specalign"} {
		found := false
		for _, d := range detected {
			if d == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("detector no longer identifies known cross-binary cluster %s — the detection heuristic regressed (detected: %v)", want, detected)
		}
	}

	// Under-inclusion is the dangerous direction: a real cache-blind package
	// missing from the list rides stale greens.
	for _, dir := range detected {
		if !listed[dir] {
			t.Errorf("package %s builds+execs the cmd/verdi binary from outside cmd/verdi (cache-blind, ADJ-68) but is ABSENT from CROSS_BINARY_PKGS in the Makefile — add ./%s/... so `make test`/`make verify` force -count=1 for it, or it will serve stale cached PASSes after a cmd/verdi behavior change", dir, dir)
		}
	}

	// The list must actually be consumed by a cache-bypassing invocation;
	// defining it but never wiring `-count=1 $(CROSS_BINARY_PKGS)` would leave
	// `make test` blind while this guard still saw a populated variable.
	if !makefileWiresCountOne(makefile) {
		t.Errorf("CROSS_BINARY_PKGS is defined but no Makefile recipe runs `-count=1 $(CROSS_BINARY_PKGS)` — the list is not wired to the cache bypass, so `make test` can still serve stale cross-binary greens (ADJ-68)")
	}
}

// makefileWiresCountOne reports whether some Makefile line applies -count=1 to
// the CROSS_BINARY_PKGS list — the `test` target's cache-bypass re-run.
func makefileWiresCountOne(makefile string) bool {
	for _, line := range strings.Split(makefile, "\n") {
		if strings.Contains(line, "-count=1") && strings.Contains(line, "$(CROSS_BINARY_PKGS)") {
			return true
		}
	}
	return false
}
