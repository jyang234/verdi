package main

// BL-148 (and BL-113): every repository the harness creates switches off
// git's background housekeeping, as internal/fixturegit does (D6-31), so
// no detached `git gc --auto` can still be writing into a store while
// whatever created it removes it.

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// requireNoBackgroundMaintenance fails t unless the repository at dir (a
// working tree or a bare repository) has both of D6-31's background-
// maintenance settings off in its own config.
func requireNoBackgroundMaintenance(t *testing.T, dir string) {
	t.Helper()
	for _, key := range []string{"gc.autoDetach", "maintenance.auto"} {
		got, err := gitOutput(t.Context(), dir, "config", "--local", "--get", key)
		if err != nil || got != "false" {
			t.Errorf("%s: %s = %q (%v), want false (BL-148: no detached background gc may outlive the store's creator)", dir, key, got, err)
		}
	}
}

// TestInitRepo_DisablesBackgroundMaintenance: a working tree and a bare
// repository alike start on main with both settings off.
func TestInitRepo_DisablesBackgroundMaintenance(t *testing.T) {
	for _, tc := range []struct {
		name string
		bare bool
	}{
		{name: "working tree"},
		{name: "bare", bare: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			dir := filepath.Join(t.TempDir(), "repo")
			if err := initRepo(ctx, dir, tc.bare); err != nil {
				t.Fatalf("initRepo: %v", err)
			}
			requireNoBackgroundMaintenance(t, dir)
			if head, err := gitOutput(ctx, dir, "symbolic-ref", "HEAD"); err != nil || head != "refs/heads/main" {
				t.Errorf("HEAD = %q (%v), want refs/heads/main", head, err)
			}
			bare, err := gitOutput(ctx, dir, "rev-parse", "--is-bare-repository")
			if err != nil || bare != strconv.FormatBool(tc.bare) {
				t.Errorf("--is-bare-repository = %q (%v), want %v", bare, err, tc.bare)
			}
		})
	}
}

// TestInitRepo_Negative: a directory that cannot be created, and a
// cancelled context, are errors — never a half-initialized store.
func TestInitRepo_Negative(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "a-file")
	if err := os.WriteFile(blocker, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := initRepo(t.Context(), filepath.Join(blocker, "repo"), false); err == nil {
		t.Error("initRepo beneath a regular file: want an error, got nil")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := initRepo(ctx, filepath.Join(t.TempDir(), "repo"), true); !errors.Is(err, context.Canceled) {
		t.Errorf("initRepo under a cancelled context = %v, want context.Canceled", err)
	}
}

// TestHarnessStores_DisableBackgroundMaintenance: each sibling store the
// harness provisions for its own serve, and its bare origin where it has
// one, has both settings off. The shared store is proven by
// TestProvisionSharedStore_AfterMainRunsOnMainBeforeBranches.
func TestHarnessStores_DisableBackgroundMaintenance(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name      string
		provision func(t *testing.T) (string, error)
		origin    bool
	}{
		{name: "empty glance", origin: true, provision: func(*testing.T) (string, error) { return provisionEmptyStore(ctx) }},
		{name: "unproven board", provision: func(*testing.T) (string, error) { return provisionUnprovenStore(ctx, testModuleRoot) }},
		{name: "vocabulary", origin: true, provision: func(*testing.T) (string, error) { return provisionVocabStore(ctx, testModuleRoot) }},
		{name: "spec import", origin: true, provision: func(*testing.T) (string, error) { return provisionSpecImportStore(ctx, testModuleRoot) }},
		{name: "index dates", origin: true, provision: func(t *testing.T) (string, error) { return provisionIndexDatesStore(ctx, t.TempDir()) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, err := tc.provision(t)
			if err != nil {
				t.Fatalf("provisioning: %v", err)
			}
			if !strings.HasPrefix(root, os.TempDir()) || filepath.Base(root) != "store" {
				t.Fatalf("store root %q is not a store/ beneath the temp dir", root)
			}
			t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(root)) })
			requireNoBackgroundMaintenance(t, root)
			if tc.origin {
				requireNoBackgroundMaintenance(t, filepath.Join(filepath.Dir(root), "origin.git"))
			}
		})
	}
}

// TestHarnessRepoInits_GoThroughInitRepo is the structural half: no
// production file of the harness runs `git init` through the git seam
// except initRepo itself, so a store added later cannot skip the
// settings.
func TestHarnessRepoInits_GoThroughInitRepo(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	scanned := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		scanned++
		for _, site := range rawRepoInits(f) {
			t.Errorf("%s: runs git init outside initRepo; call initRepo so the store gets BL-148's settings", fset.Position(site))
		}
	}
	if scanned == 0 {
		t.Fatal("no production file was scanned: the witness would be vacuous")
	}
}

// rawRepoInits returns the position of every runGit or gitOutput call in f
// whose git subcommand (its first git argument) is the literal "init",
// outside the body of initRepo.
func rawRepoInits(f *ast.File) []token.Pos {
	// gitArgsAt is where each seam function's git arguments start: after
	// runGit's ctx, dir and extraEnv, and after gitOutput's ctx and dir.
	gitArgsAt := map[string]int{"runGit": 3, "gitOutput": 2}
	var out []token.Pos
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "initRepo" {
			continue
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := call.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			at, seam := gitArgsAt[id.Name]
			if !seam || len(call.Args) <= at {
				return true
			}
			if lit, ok := call.Args[at].(*ast.BasicLit); ok && lit.Kind == token.STRING && lit.Value == `"init"` {
				out = append(out, call.Pos())
			}
			return true
		})
	}
	return out
}

// TestRawRepoInits: the scanner flags a raw init through either seam
// function, and passes initRepo's own body, an "init" that is not the
// subcommand, and a call outside the seam.
func TestRawRepoInits(t *testing.T) {
	const src = `package main
func initRepo() { runGit(nil, "", nil, "init") }
func a() { runGit(nil, "", nil, "init", "--bare") }
func b() { _, _ = gitOutput(nil, "", "init") }
func c() { runGit(nil, "", nil, "commit", "-m", "init") }
func d() { other(nil, "", nil, "init") }
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "src.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, pos := range rawRepoInits(f) {
		lines = append(lines, strconv.Itoa(fset.Position(pos).Line))
	}
	if got := strings.Join(lines, ","); got != "3,4" {
		t.Errorf("rawRepoInits flagged lines %q, want \"3,4\"", got)
	}
}
