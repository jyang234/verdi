package draftmutation_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/store"
)

const draftMutationImport = "github.com/jyang234/verdi/internal/draftmutation"

// TestLaterWorkbenchAdapterDoesNotImportDraftMutation guards the delivery
// units still ahead: internal/workbench must keep NO direct dependency on
// this package until its own Task 2 unit lands (Wave 6 authority design
// §6.1 — "Task 2 must atomically rewire every board mutation to designapp
// and delete the splice path in the same unit"), and the two CLI server
// entrypoints (cmd/verdi/mcp.go, cmd/verdi/serve.go) must keep routing
// draft mutation through their adapters rather than reaching into this
// package themselves.
//
// Exactly ONE guarded path legitimately fell away in Wave 6 Task 1:
// internal/mcpserve, whose tool_mutate_draft.go now imports draftmutation
// directly for the exact wire-schema types (Request/Actor/
// NewDelegatedAgent) AC-1's mutation contract fixes. That package alone is
// out of scope now; mcp.go and serve.go still hold the boundary and stay
// guarded, so a future regression that pulls draftmutation into either
// entrypoint still fails here (the gate grows, never shrinks — CLAUDE.md).
func TestLaterWorkbenchAdapterDoesNotImportDraftMutation(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..")
	targets := []string{
		filepath.Join(repositoryRoot, "internal", "workbench"),
		filepath.Join(repositoryRoot, "cmd", "verdi", "mcp.go"),
		filepath.Join(repositoryRoot, "cmd", "verdi", "serve.go"),
	}
	parsed := 0
	for _, target := range targets {
		info, err := os.Stat(target)
		if err != nil {
			t.Fatal(err)
		}
		paths := []string{target}
		if info.IsDir() {
			paths = nil
			err = filepath.WalkDir(target, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if !entry.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
					paths = append(paths, path)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		for _, path := range paths {
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}
			parsed++
			for _, imported := range file.Imports {
				value, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					t.Fatalf("unquoting import in %s: %v", path, err)
				}
				if value == draftMutationImport {
					t.Fatalf("later adapter %s imports draftmutation before its delivery unit", path)
				}
			}
		}
	}
	if parsed == 0 {
		t.Fatal("adapter import witness parsed no production Go files")
	}
}

// TestNewUnauthenticatedHumanHasNoNonTestProductionCaller structurally
// proves clause E of Wave 6 Task 1A's bounded contract: today (before
// Task 2's workbench handler exists) there is exactly ZERO non-test
// production caller of draftmutation.NewUnauthenticatedHuman anywhere in
// the repository — CLI, MCP, internal/designapp, and internal/workbench
// alike. Combined with the constructor itself taking no request-derived
// argument at all (policy.go), this also proves no request decoder can
// mint the actor: there is no code path, let alone a data-driven one, that
// even calls it outside this package's own definition and tests.
func TestNewUnauthenticatedHumanHasNoNonTestProductionCaller(t *testing.T) {
	parsed, references, err := productionReferencesToConstructor(filepath.Join("..", ".."), unauthenticatedHumanConstructor)
	if err != nil {
		t.Fatal(err)
	}
	if parsed == 0 {
		t.Fatal("caller-inventory witness parsed no production Go files")
	}
	if len(references) != 0 {
		t.Fatalf("%s referenced outside its own declaration in non-test production code: %v", unauthenticatedHumanConstructor, references)
	}
}

const unauthenticatedHumanConstructor = "NewUnauthenticatedHuman"

// productionReferencesToConstructor AST-walks every non-test .go file under
// root and returns every reference to constructorName that is not inside
// the constructor's own declaring FuncDecl.
//
// It matches any *ast.Ident or *ast.SelectorExpr naming the constructor,
// not only a *ast.CallExpr: `handler := draftmutation.NewUnauthenticatedHuman`
// hands the minting capability to a data-driven dispatch table just as
// completely as calling it does, and a CallExpr-only matcher would report
// that escape as zero callers.
//
// The exclusion is the declaring FuncDecl alone — not the whole file that
// declares it — so a second production caller added elsewhere in
// policy.go (or anywhere else in package draftmutation) stays visible to
// the witness. Test files remain excluded: the constructor is exercised by
// this package's own tests by design.
func productionReferencesToConstructor(root, constructorName string) (parsed int, references []string, err error) {
	walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fileSet := token.NewFileSet()
		file, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		parsed++
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.FuncDecl:
				// Skip the constructor's own declaration (name and body)
				// and nothing else.
				return node.Recv != nil || node.Name == nil || node.Name.Name != constructorName
			case *ast.SelectorExpr:
				if node.Sel != nil && node.Sel.Name == constructorName {
					references = append(references, fileSet.Position(node.Pos()).String())
					return false
				}
			case *ast.Ident:
				if node.Name == constructorName {
					references = append(references, fileSet.Position(node.Pos()).String())
				}
			}
			return true
		})
		return nil
	})
	return parsed, references, walkErr
}

// TestConstructorReferenceWitnessDetectsFunctionValueReferences proves the
// caller-inventory witness above actually bites: over a synthetic tree it
// must report a bare function-value reference and a qualified
// function-value reference (neither of which is a CallExpr), must report a
// production call site in the very file that declares the constructor, and
// must stay silent for the declaring FuncDecl itself and for test files.
// Without this, a matcher that quietly stopped matching would still report
// "zero callers" and read as a pass.
func TestConstructorReferenceWitnessDetectsFunctionValueReferences(t *testing.T) {
	root := t.TempDir()
	write := func(name, source string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The declaring file also holds an in-package caller: the exclusion is
	// the FuncDecl, so that caller must still be reported.
	write("policy.go", `package draftmutation

func NewUnauthenticatedHuman() (int, error) { return 0, nil }

func inPackageProductionCaller() (int, error) { return NewUnauthenticatedHuman() }
`)
	write("escape.go", `package adapter

var mintHuman = NewUnauthenticatedHuman
`)
	write("qualified.go", `package adapter

import "example.com/draftmutation"

var table = map[string]any{"human": draftmutation.NewUnauthenticatedHuman}
`)
	write("boundary_probe_test.go", `package adapter

func probe() { _ = NewUnauthenticatedHuman }
`)

	parsed, references, err := productionReferencesToConstructor(root, unauthenticatedHumanConstructor)
	if err != nil {
		t.Fatal(err)
	}
	if parsed != 3 {
		t.Fatalf("parsed = %d production files, want 3 (the test file must be skipped)", parsed)
	}
	if len(references) != 3 {
		t.Fatalf("references = %v, want exactly the in-package caller and the two function-value references", references)
	}
	for _, want := range []string{"policy.go", "escape.go", "qualified.go"} {
		found := false
		for _, reference := range references {
			if strings.Contains(reference, want) {
				found = true
			}
		}
		if !found {
			t.Fatalf("references %v missing the %s reference", references, want)
		}
	}
}

func TestDesignProvenanceExclusionIsConsumableWithoutArtifactClassification(t *testing.T) {
	identity, err := designprovenance.ResolveIdentity("spec/sample", store.ZoneActive)
	if err != nil {
		t.Fatal(err)
	}
	if identity.ExclusionReason != designprovenance.ExclusionReason {
		t.Fatalf("external exclusion identity = %+v", identity)
	}
	if designprovenance.ExclusionReason != "design-provenance-sidecar" {
		t.Fatalf("ExclusionReason = %q", designprovenance.ExclusionReason)
	}
	if kind, ok := artifact.ClassifyPath("specs/active/sample/design-provenance.jsonl"); ok || kind != "" {
		t.Fatalf("ClassifyPath classified non-authoritative sidecar as %q, ok=%v", kind, ok)
	}
	if kind, ok := artifact.ClassifyPath("specs/active/sample/spec.md"); !ok || kind != "spec" {
		t.Fatalf("ClassifyPath existing spec behavior changed to %q, ok=%v", kind, ok)
	}
}
