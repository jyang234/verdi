package disclosureview

import (
	"context"
	"crypto/sha256"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
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

// TestKey_CoversLintFileReads pins every I/O call site in the enumeration's
// closure (the functions cachekey.go's inventory names). A new or removed
// call fails here until the key is reviewed against it and the table is
// updated.
func TestKey_CoversLintFileReads(t *testing.T) {
	tests := []struct {
		name  string
		dir   string
		files []string // nil: every non-test file
		funcs []string // nil: every function
		want  map[string]int
	}{
		{name: "internal/lint", dir: filepath.Join("..", "lint"), want: map[string]int{
			"candidate.go specstate.NewProjector":       1,
			"candidate.go store.Open":                   1,
			"candidate.go store.SpecRelPath":            1,
			"cienv.go os.Getenv":                        5,
			"cienv.go specstate.ResolveDefaultBranch":   1,
			"context.go gitx.CurrentBranch":             1,
			"context.go gitx.MergeBase":                 1,
			"context.go specstate.ResolveDefaultBranch": 1,
			"engine.go specstate.NewProjector":          1,
			"engine.go store.Open":                      1,
			"snapshot.go os.IsNotExist":                 7,
			"snapshot.go os.ReadDir":                    2,
			"snapshot.go os.ReadFile":                   5,
			"snapshot.go store.DecodeManifest":          1,
			"snapshot.go store.DiscoverServices":        1,
			"vl003.go filepath.Abs":                     1,
			"vl003.go gitx.ReachableFromHEAD":           1,
			"vl004.go os.ReadFile":                      1,
			"vl009.go gitx.ReachableFromHEAD":           1,
			"vl010.go gitx.DiffNameStatus":              1,
			"vl010.go gitx.Show":                        3,
			"vl013.go gitx.LsFiles":                     1,
			"vl015.go gitx.Show":                        1,
			"vl015.go os.ReadFile":                      1,
			"vl016.go gitx.DiffNameStatus":              1,
			"vl017.go os.IsNotExist":                    1,
			"vl017.go os.ReadDir":                       1,
			"vl017.go os.ReadFile":                      1,
			"vl017.go os.Stat":                          1,
			"vl019.go storyresolve.LoadSpec":            1,
			"vl022.go store.RefSlug":                    1,
			"vl022.go storyresolve.LoadSpec":            1,
			"walk.go filepath.WalkDir":                  1,
			"walk.go os.ReadDir":                        1,
			"walk.go os.ReadFile":                       1,
		}},
		{name: "internal/specstate", dir: filepath.Join("..", "specstate"), want: map[string]int{
			"defaultbranch.go gitx.DefaultBranch":           1,
			"defaultbranch.go gitx.HasLocalBranch":          1,
			"defaultbranch.go gitx.HasRemoteTrackingBranch": 3,
			"defaultbranch.go os.Getenv":                    1,
			"resolve.go gitx.BlobAt":                        1,
			"resolve.go gitx.FirstParentBlobLanding":        1,
			"resolve.go gitx.LsTree":                        1,
			"resolve.go gitx.RevParse":                      1,
			"resolve.go gitx.Show":                          1,
		}},
		{name: "internal/store discovery and open", dir: filepath.Join("..", "store"), files: []string{"discovery.go", "open.go"}, want: map[string]int{
			"discovery.go filepath.Abs":     1,
			"discovery.go filepath.WalkDir": 1,
			"discovery.go os.ReadFile":      2,
			"discovery.go os.Stat":          1,
			"open.go os.IsNotExist":         1,
			"open.go os.ReadFile":           2,
		}},
		{name: "storyresolve.LoadSpec", dir: filepath.Join("..", "storyresolve"), files: []string{"resolve.go"}, funcs: []string{"LoadSpec"}, want: map[string]int{
			"resolve.go os.IsNotExist":  1,
			"resolve.go os.ReadFile":    1,
			"resolve.go store.SpecPath": 1,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ioCallSites(t, tt.dir, tt.files, tt.funcs)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("I/O call sites changed — review the cache key's input inventory (cachekey.go) against them, then update this table.\n got %v\nwant %v", got, tt.want)
			}
		})
	}
}

// ioCallSites counts calls to I/O-capable package functions — os, os/exec,
// io/fs, io/ioutil, filepath's walking and resolving functions, and the
// git, spec-state, store and story-resolution packages — by file.
func ioCallSites(t *testing.T, dir string, files, funcs []string) map[string]int {
	t.Helper()
	ioPkgs := map[string]bool{"os": true, "exec": true, "ioutil": true, "fs": true, "gitx": true, "specstate": true, "store": true, "storyresolve": true, "filepath": true}
	filepathIO := map[string]bool{"Walk": true, "WalkDir": true, "Glob": true, "EvalSymlinks": true, "Abs": true}
	if files == nil {
		matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range matches {
			if !strings.HasSuffix(m, "_test.go") {
				files = append(files, filepath.Base(m))
			}
		}
	}
	wantFunc := map[string]bool{}
	for _, f := range funcs {
		wantFunc[f] = true
	}
	counts := map[string]int{}
	for _, name := range files {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && len(wantFunc) > 0 && !wantFunc[fn.Name.Name] {
				continue
			} else if !ok && len(wantFunc) > 0 {
				continue
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, ok := sel.X.(*ast.Ident)
				if !ok || !ioPkgs[pkg.Name] || (pkg.Name == "filepath" && !filepathIO[sel.Sel.Name]) {
					return true
				}
				counts[name+" "+pkg.Name+"."+sel.Sel.Name]++
				return true
			})
		}
	}
	return counts
}
