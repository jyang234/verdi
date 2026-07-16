package showcasealign

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"testing"
)

// TestShowcaseCoverage_EnumerationIsComplete proves the CLI axis's enumeration
// is COMPLETE, not merely that its red-direction SEAM works
// (TestShowcaseCoverage_RealEnumerationDetectsGaps proves the seam). cliVerbs
// enumerates exactly verbPhase's phase>0 keys plus one hand-appended "lint",
// because dispatch.go's run() special-cases "lint" BEFORE the `verbPhase[verb]`
// lookup and resolves every other verb THROUGH that lookup (run()'s
// `if !known { usage; return }` rejects any verb that is not a verbPhase key).
// So a verb can ship dispatched-but-unenumerated — invisible to the whole gate,
// this story's own silent pass — only via a SECOND pre-phase branch on the verb.
//
// This test DEFAULT-DENIES that: it parses run() and requires that, in the
// region before the verbPhase lookup, the ONLY statements that reference the
// `verb` identifier are its declaration (`verb := args[0]`) and the single
// blessed branch `if verb == "lint"` (whose body does not itself reference
// verb). Anything else that touches verb pre-phase — a `switch verb`, a reversed
// `"x" == verb`, a `strings.HasPrefix(verb, …)`, a helper call taking verb —
// fails here, naming the statement. This is shape-AGNOSTIC on purpose: it does
// not enumerate the evasions (which a shape-specific `verb ==` scan would miss),
// it denies every pre-phase verb reference that is not explicitly blessed. Every
// unexpected AST shape at the boundary itself fails with a clear "dispatch.go
// shape changed" message rather than silently enumerating nothing.
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

	// Default-deny every pre-phase statement that touches `verb` and is not one
	// of the two blessed forms.
	sawDecl, sawBlessedLint := false, false
	for i, stmt := range runBody.List[:boundary] {
		if !refersToIdent(stmt, "verb") {
			continue // e.g. the `if len(args) == 0` guard — never touches verb
		}
		switch {
		case isVerbDeclaration(stmt):
			sawDecl = true
		case isBlessedLintArm(stmt):
			sawBlessedLint = true
		default:
			t.Errorf("run() statement #%d before the verbPhase lookup references `verb` in a non-blessed way (%T) — the only pre-phase verb branch allowed is `if verb == \"lint\"`; anything else (a switch on verb, a reversed comparison, strings.HasPrefix(verb, …), a helper taking verb) can dispatch a verb verbPhase does not enumerate, shipping it invisible to the gate. Add that verb to verbPhase, or teach cliVerbs before it can pass the gate invisibly.", i, stmt)
		}
	}
	if !sawDecl {
		t.Errorf("run() no longer declares `verb := args[0]` before the verbPhase lookup (dispatch.go shape changed) — cannot confirm how the pre-phase region resolves the verb")
	}
	if !sawBlessedLint {
		t.Errorf("run() no longer special-cases `if verb == \"lint\"` before the verbPhase lookup; cliVerbs hand-appends \"lint\" on the premise that run() dispatches it here, so that premise is now stale — remove the append or restore the arm")
	}
}

// refersToIdent reports whether the subtree rooted at n contains an identifier
// named name.
func refersToIdent(n ast.Node, name string) bool {
	found := false
	ast.Inspect(n, func(node ast.Node) bool {
		if found {
			return false
		}
		if id, ok := node.(*ast.Ident); ok && id.Name == name {
			found = true
			return false
		}
		return true
	})
	return found
}

// isVerbDeclaration reports whether stmt is `verb := <expr>` (the pre-phase
// declaration of the verb the rest of run() dispatches on).
func isVerbDeclaration(stmt ast.Stmt) bool {
	as, ok := stmt.(*ast.AssignStmt)
	if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 1 {
		return false
	}
	id, ok := as.Lhs[0].(*ast.Ident)
	return ok && id.Name == "verb"
}

// isBlessedLintArm reports whether stmt is exactly `if verb == "lint" { … }`
// with no initializer, no else, and a body that does NOT itself reference verb —
// so the arm only TESTS verb == "lint" and dispatches lint; it cannot branch on
// verb any further. This is the one pre-phase verb branch cliVerbs' hand-appended
// "lint" is allowed to mirror.
func isBlessedLintArm(stmt ast.Stmt) bool {
	ifs, ok := stmt.(*ast.IfStmt)
	if !ok || ifs.Init != nil || ifs.Else != nil {
		return false
	}
	be, ok := ifs.Cond.(*ast.BinaryExpr)
	if !ok || be.Op != token.EQL {
		return false
	}
	x, ok := be.X.(*ast.Ident)
	if !ok || x.Name != "verb" {
		return false
	}
	y, ok := be.Y.(*ast.BasicLit)
	if !ok || y.Kind != token.STRING {
		return false
	}
	if s, err := strconv.Unquote(y.Value); err != nil || s != "lint" {
		return false
	}
	return !refersToIdent(ifs.Body, "verb")
}
