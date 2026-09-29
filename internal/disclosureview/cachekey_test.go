package disclosureview

import (
	"context"
	"crypto/sha256"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
)

func TestReadInputs(t *testing.T) {
	tests := []struct {
		name  string
		store func(t *testing.T) string
		// wantErr is false for a store whose key must be computable,
		// deterministic and stable across two readings.
		wantErr bool
	}{
		{"an unchanged git-backed store", func(t *testing.T) string { return newCacheFixture(t).root }, false},
		{"a store root given as a relative path", func(t *testing.T) string {
			root := newCacheFixture(t).root
			t.Chdir(filepath.Dir(root))
			return filepath.Base(root)
		}, false},
		{"not a git repository", buildFixtureStore, true},
		{".verdi is a symbolic link", func(t *testing.T) string {
			fx := newCacheFixture(t)
			moved := filepath.Join(t.TempDir(), "verdi")
			if err := os.Rename(filepath.Join(fx.root, ".verdi"), moved); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(moved, filepath.Join(fx.root, ".verdi")); err != nil {
				t.Fatal(err)
			}
			return fx.root
		}, true},
		{"a dangling symbolic link under .verdi", func(t *testing.T) string {
			fx := newCacheFixture(t)
			if err := os.Symlink(filepath.Join(t.TempDir(), "gone.md"), filepath.Join(fx.root, ".verdi", "adr-link.md")); err != nil {
				t.Fatal(err)
			}
			return fx.root
		}, true},
		{"an unreadable directory in the service walk", func(t *testing.T) string {
			fx := newCacheFixture(t)
			locked := filepath.Join(fx.root, "locked")
			if err := os.Mkdir(locked, 0o000); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
			if _, err := os.ReadDir(locked); err == nil {
				t.Skip("running with permissions that read a mode-000 directory")
			}
			return fx.root
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := tt.store(t)
			first, err := readInputs(context.Background(), root)
			if tt.wantErr {
				if !errors.Is(err, errUncomputable) {
					t.Fatalf("readInputs error = %v, want errUncomputable", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("readInputs: %v", err)
			}
			second, err := readInputs(context.Background(), root)
			if err != nil {
				t.Fatalf("second readInputs: %v", err)
			}
			if !first.same(second) {
				t.Fatalf("two readings of an unchanged store differ:\n%+v\n%+v", first, second)
			}
			if first.key == "" || first.stamps == "" || first.newest.IsZero() {
				t.Fatalf("incomplete reading: %+v", first)
			}
		})
	}
}

func TestInputs_Same(t *testing.T) {
	base := inputs{key: "k", stamps: "s"}
	tests := []struct {
		name  string
		other inputs
		want  bool
	}{
		{"same contents, nothing written", inputs{key: "k", stamps: "s"}, true},
		{"different contents", inputs{key: "k2", stamps: "s"}, false},
		{"same contents, something written", inputs{key: "k", stamps: "s2"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := base.same(tt.other); got != tt.want {
				t.Fatalf("same = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWriteFramed_SeparatesFields(t *testing.T) {
	digest := func(parts ...string) [32]byte {
		h := sha256.New()
		for _, p := range parts {
			writeFramed(h, []byte(p))
		}
		var out [32]byte
		copy(out[:], h.Sum(nil))
		return out
	}
	tests := []struct {
		name string
		a, b []string
	}{
		{"a byte moved across the boundary", []string{"ab", "c"}, []string{"a", "bc"}},
		{"an empty field", []string{"abc", ""}, []string{"abc"}},
		{"the same bytes in one field or two", []string{"a\x00b"}, []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if digest(tt.a...) == digest(tt.b...) {
				t.Fatalf("%q and %q frame to the same digest", tt.a, tt.b)
			}
		})
	}
}

func TestIsFanoutDir(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"00", true}, {"ab", true}, {"f9", true},
		{"pack", false}, {"info", false}, {"a", false}, {"abc", false}, {"AB", false}, {"g0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isFanoutDir(tt.name); got != tt.want {
				t.Fatalf("isFanoutDir(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// TestServiceWalk_MatchesStoreDiscovery pins the key's copy of service
// discovery's unexported walk rules to store/discovery.go's own: the
// directory names it skips, the bindings file name and the OpenAPI
// candidates. If discovery stops skipping a directory or starts reading
// another companion, this fails until the key reads it too.
func TestServiceWalk_MatchesStoreDiscovery(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join("..", "store", "discovery.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse store/discovery.go: %v", err)
	}
	values := map[string][]string{}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
				continue
			}
			values[vs.Names[0].Name] = stringLiterals(t, vs.Values[0])
		}
	}

	var skips []string
	for name := range serviceWalkSkipDirs {
		skips = append(skips, name)
	}
	sort.Strings(skips)
	storeSkips := append([]string(nil), values["skipDirNames"]...)
	sort.Strings(storeSkips)

	var companions []string
	companions = append(companions, values["bindingsFile"]...)
	for _, name := range values["openAPICandidates"] {
		companions = append(companions, "api/"+name)
	}

	checks := []struct {
		name      string
		got, want []string
	}{
		{"skipped directory names", skips, storeSkips},
		{"service marker file", []string{".flowmap.yaml"}, values["flowmapFile"]},
		{"bindings and OpenAPI companions", serviceCompanions[1:], companions},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if len(c.want) == 0 {
				t.Fatal("found nothing in store/discovery.go — the scan is broken")
			}
			if !reflect.DeepEqual(c.got, c.want) {
				t.Fatalf("key reads %q, store/discovery.go has %q", c.got, c.want)
			}
		})
	}
}

// stringLiterals returns every string literal in e, in source order.
func stringLiterals(t *testing.T, e ast.Expr) []string {
	t.Helper()
	var out []string
	ast.Inspect(e, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			t.Fatalf("unquote %s: %v", lit.Value, err)
		}
		out = append(out, s)
		return true
	})
	return out
}

// gitShapes records the subcommand and flags of every git invocation.
type gitShapes struct {
	mu     sync.Mutex
	shapes map[string]bool
}

func (g *gitShapes) Observe(_ string, args []string) {
	var flags []string
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
		}
	}
	sort.Strings(flags)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.shapes[strings.Join(append([]string{args[0]}, flags...), " ")] = true
}

// TestKey_CoversLintGitReads runs the enumeration under a gitx observer
// and requires every git read it makes to be one whose inputs the key
// covers. A new git read in lint's closure fails here until the key reads
// what it depends on and the shape is added below.
func TestKey_CoversLintGitReads(t *testing.T) {
	fx := newCacheFixture(t)
	// A frozen stamp exercises the reachability reads (VL-009).
	writeFile(t, filepath.Join(fx.root, ".verdi", "specs", "active", "frozen-story", "spec.md"),
		strings.Replace(strings.Replace(storySpecMD, "spec/panel-fixture", "spec/frozen-story", 1),
			"links:", "frozen: { at: 2026-01-01, commit: "+fx.heads[0]+" }\nlinks:", 1))
	gitIn(t, fx.root, "add", "-A")
	gitIn(t, fx.root, "commit", "-q", "-m", "frozen story")

	covered := map[string]string{
		"symbolic-ref --short -q":           "HEAD and origin/HEAD: rev-parse --symbolic-full-name HEAD, for-each-ref %(symref)",
		"rev-parse --git-dir":               "repository identity: rev-parse --git-dir",
		"show-ref --quiet --verify":         "ref presence: for-each-ref",
		"merge-base":                        "HEAD, refs, objects, shallow, grafts",
		"merge-base --is-ancestor":          "HEAD, refs, objects, shallow, grafts",
		"rev-parse --verify -q":             "pin resolution: refs, objects, config (core.disambiguate)",
		"rev-parse --verify":                "revision resolution: refs, objects",
		"rev-parse --is-shallow-repository": "rev-parse --is-shallow-repository, the shallow file",
		"diff --ignore-submodules=none --name-status --no-relative -M": "commits (refs, objects) and config (rename limits)",
		"show":                                 "objects",
		"ls-files":                             "gitx.LsFiles",
		"ls-tree --":                           "objects",
		"ls-tree -- --name-only -r":            "objects",
		"rev-list -- --first-parent --reverse": "refs, objects",
	}
	obs := &gitShapes{shapes: map[string]bool{}}
	ctx := gitx.WithObserver(context.Background(), obs)
	if _, err := lintDisclosures(ctx, fx.root); err != nil {
		t.Fatalf("lintDisclosures: %v", err)
	}
	if len(obs.shapes) < 5 {
		t.Fatalf("observed only %d git read shapes %v — the observer is not attached", len(obs.shapes), obs.shapes)
	}
	for shape := range obs.shapes {
		if _, ok := covered[shape]; !ok {
			t.Errorf("lint runs `git %s`, a read the cache key does not cover: extend readInputs and this table", shape)
		}
	}
}

// TestKey_CoversLintFileReads pins every I/O call site in the
// enumeration's whole dependency closure (B3-RR4): every non-standard
// package `go list -deps` reports for internal/lint and internal/disclosure,
// the packages Current reaches — gitx, specstate, store, storyresolve,
// artifact, model and the rest, third-party ones included. Each package's
// every non-test file is scanned, whatever its build constraints, for
// calls into os, os/exec, os/user, io/fs, io/ioutil, syscall, net,
// net/http, crypto/rand, math/rand, filepath's walking and resolving
// functions and time's clock, and for such functions passed as values.
//
// testdata/enumeration-io-call-sites.golden is the reviewed snapshot: on
// 2026-09-29 each call site the enumeration reaches was checked against
// cachekey.go's inventory (keyed), and the others (gitx's write and
// plumbing helpers, store's archive, root and tree-hash helpers,
// storyresolve beyond LoadSpec, artifact's ULID clock) were checked as not
// reached by lint. A new or removed call site anywhere in the closure fails
// here until the key is reviewed against it and the golden is updated.
func TestKey_CoversLintFileReads(t *testing.T) {
	got := closureIOCallSites(t)
	wantBytes, err := os.ReadFile(filepath.Join("testdata", "enumeration-io-call-sites.golden"))
	if err != nil {
		t.Fatalf("reading golden: %v", err)
	}
	want := strings.Split(strings.TrimRight(string(wantBytes), "\n"), "\n")
	if len(got) < 10 {
		t.Fatalf("found only %d I/O call sites in the closure — the scan is broken:\n%s", len(got), strings.Join(got, "\n"))
	}
	if !reflect.DeepEqual(got, want) {
		gotSet := map[string]bool{}
		for _, l := range got {
			gotSet[l] = true
		}
		wantSet := map[string]bool{}
		for _, l := range want {
			wantSet[l] = true
		}
		var diff []string
		for _, l := range want {
			if !gotSet[l] {
				diff = append(diff, "- "+l)
			}
		}
		for _, l := range got {
			if !wantSet[l] {
				diff = append(diff, "+ "+l)
			}
		}
		t.Fatalf("I/O call sites in the enumeration's dependency closure changed; review cachekey.go's input inventory against them, then update the golden.\n%s\n--- full inventory ---\n%s", strings.Join(diff, "\n"), strings.Join(got, "\n"))
	}
}

// ioImports maps each I/O-capable standard import path to its default
// package name; ioOnly narrows a package to the listed functions.
var (
	ioImports = map[string]string{
		"os": "os", "os/exec": "exec", "os/user": "user", "io/fs": "fs", "io/ioutil": "ioutil",
		"path/filepath": "filepath", "syscall": "syscall", "net": "net", "net/http": "http",
		"crypto/rand": "rand", "math/rand": "rand", "math/rand/v2": "rand", "time": "time",
	}
	ioOnly = map[string]map[string]bool{
		"path/filepath": {"Walk": true, "WalkDir": true, "Glob": true, "EvalSymlinks": true, "Abs": true},
		"time":          {"Now": true, "Since": true, "Until": true},
	}
)

// closureIOCallSites returns "<import path> <file> <callee> <call|value>
// <count>" lines, sorted, for the enumeration's dependency closure.
func closureIOCallSites(t *testing.T) []string {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", "-f", "{{if not .Standard}}{{.ImportPath}}\t{{.Dir}}{{end}}",
		"github.com/jyang234/verdi/internal/lint", "github.com/jyang234/verdi/internal/disclosure")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -deps: %v\n%s", err, stderr.String())
	}
	counts := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		pkg, dir, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("go list line %q", line)
		}
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			for site, n := range fileIOCallSites(t, file) {
				counts[pkg+" "+filepath.Base(file)+" "+site] += n
			}
		}
	}
	lines := make([]string, 0, len(counts))
	for k, n := range counts {
		lines = append(lines, k+" "+strconv.Itoa(n))
	}
	sort.Strings(lines)
	return lines
}

// fileIOCallSites counts one file's I/O calls ("<path>.<Func> call") and
// I/O functions used as values ("<path>.<Func> value"), resolving package
// names through the file's own imports.
func fileIOCallSites(t *testing.T, file string) map[string]int {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	names := map[string]string{}
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		name, ok := ioImports[path]
		if !ok {
			continue
		}
		if imp.Name != nil {
			name = imp.Name.Name
		}
		names[name] = path
	}
	resolve := func(e ast.Expr) (string, bool) {
		sel, ok := e.(*ast.SelectorExpr)
		if !ok {
			return "", false
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok {
			return "", false
		}
		path, ok := names[id.Name]
		if !ok || (ioOnly[path] != nil && !ioOnly[path][sel.Sel.Name]) {
			return "", false
		}
		return path + "." + sel.Sel.Name, true
	}
	counts := map[string]int{}
	value := func(e ast.Expr) {
		sel, ok := e.(*ast.SelectorExpr)
		if !ok || !isFunctionName(sel.Sel.Name) {
			return
		}
		if site, ok := resolve(e); ok {
			counts[site+" value"]++
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			if site, ok := resolve(n.Fun); ok {
				counts[site+" call"]++
			}
			for _, a := range n.Args {
				value(a)
			}
		case *ast.AssignStmt:
			for _, r := range n.Rhs {
				value(r)
			}
		case *ast.ValueSpec:
			for _, v := range n.Values {
				value(v)
			}
		case *ast.ReturnStmt:
			for _, r := range n.Results {
				value(r)
			}
		case *ast.KeyValueExpr:
			value(n.Value)
		case *ast.CompositeLit:
			for _, el := range n.Elts {
				value(el)
			}
		}
		return true
	})
	return counts
}

// isFunctionName reports whether an exported name read as a value is taken
// to be a function: sentinel errors, modes, flags, separators and the
// standard streams are not.
func isFunctionName(name string) bool {
	for _, prefix := range []string{"Err", "Mode", "O_", "Std", "Path", "Dev", "Sig"} {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	return true
}
