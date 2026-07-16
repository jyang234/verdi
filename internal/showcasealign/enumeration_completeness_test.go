package showcasealign

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
)

// TestShowcaseCoverage_EnumerationIsComplete proves the CLI axis's enumeration
// is COMPLETE, not merely that its red-direction SEAM works
// (TestShowcaseCoverage_RealEnumerationDetectsGaps proves the seam). cliVerbs
// enumerates exactly verbPhase's phase>0 keys plus one hand-appended "lint"
// (coverage_test.go), because dispatch.go's run() special-cases "lint" BEFORE
// the `verbPhase[verb]` lookup and resolves every other verb THROUGH that
// lookup — so a verb can ship dispatched-but-unenumerated (invisible to the
// whole gate, this story's own silent pass) only via a SECOND pre-phase
// special-case arm, the exact shape `lint` already set as precedent.
//
// This test parses run() and asserts the set of verbs special-cased before the
// verbPhase lookup is EXACTLY {lint}. A second pre-phase `if verb == "X"` arm
// (a new capability dispatched ahead of the phase check) fails this loudly,
// naming X, until X is either added to verbPhase or the enumeration is taught
// about it. Verbs compared AFTER the lookup are out of scope on purpose: run()'s
// `if !known { usage; return }` guarantees they are already verbPhase keys, so
// they are enumerated by construction. Every unexpected AST shape fails with a
// clear "dispatch.go shape changed" message rather than silently enumerating
// nothing.
func TestShowcaseCoverage_EnumerationIsComplete(t *testing.T) {
	path := filepath.Join(verdiRepoRoot, "cmd", "verdi", "dispatch.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("EnumerationIsComplete: parsing %s: %v", path, err)
	}

	var runBody *ast.BlockStmt
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name.Name == "run" {
			runBody = fn.Body
		}
	}
	if runBody == nil {
		t.Fatalf("EnumerationIsComplete: func run not found in %s (dispatch.go shape changed)", path)
	}

	// Boundary: the `... := verbPhase[verb]` lookup. Statements before it are
	// the pre-phase region; every verb dispatched there escapes verbPhase.
	boundary := -1
	for i, stmt := range runBody.List {
		as, ok := stmt.(*ast.AssignStmt)
		if !ok {
			continue
		}
		for _, rhs := range as.Rhs {
			idx, ok := rhs.(*ast.IndexExpr)
			if !ok {
				continue
			}
			if ident, ok := idx.X.(*ast.Ident); ok && ident.Name == "verbPhase" {
				boundary = i
			}
		}
		if boundary >= 0 {
			break
		}
	}
	if boundary < 0 {
		t.Fatalf("EnumerationIsComplete: the verbPhase[verb] lookup was not found in run() (dispatch.go shape changed) — cannot locate the pre-phase region")
	}

	// Collect every `verb == "<literal>"` comparison strictly before the
	// boundary. (The `if len(args) == 0` guard compares len(args), not verb, so
	// it is naturally skipped.)
	prePhaseVerbs := map[string]bool{}
	for _, stmt := range runBody.List[:boundary] {
		ast.Inspect(stmt, func(n ast.Node) bool {
			be, ok := n.(*ast.BinaryExpr)
			if !ok || be.Op != token.EQL {
				return true
			}
			x, ok := be.X.(*ast.Ident)
			if !ok || x.Name != "verb" {
				return true
			}
			lit, ok := be.Y.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatalf("EnumerationIsComplete: unquoting pre-phase verb literal %s: %v", lit.Value, err)
			}
			prePhaseVerbs[s] = true
			return true
		})
	}

	if len(prePhaseVerbs) != 1 || !prePhaseVerbs["lint"] {
		got := make([]string, 0, len(prePhaseVerbs))
		for v := range prePhaseVerbs {
			got = append(got, v)
		}
		sort.Strings(got)
		t.Errorf("run() special-cases %v before the verbPhase lookup, want exactly [lint]; any other pre-phase verb ships dispatched-but-unenumerated (cliVerbs enumerates verbPhase's keys and hand-appends only \"lint\") — add it to verbPhase, or teach cliVerbs, before it can pass the gate invisibly", got)
	}
}
