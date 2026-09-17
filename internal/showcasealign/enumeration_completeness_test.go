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

// prePhaseSpellings maps every verb dispatch.go's run() dispatches BEFORE
// the `verbPhase[verb]` lookup to the EXACT set of literal tokens it may be
// dispatched on. It is the ONE list this file and cliVerbs
// (coverage_test.go) share: this test proves run() dispatches exactly these
// pre-phase and nothing else, and cliVerbs hand-appends exactly these keys
// to the phase>0 verbs it reads out of verbPhase — so the two can never
// drift apart, and every pre-phase verb is enumerated by the
// showcase-coverage gate rather than escaping it.
//
// A FLAG SPELLING IS THE SAME CAPABILITY, NOT A SECOND ONE: "--help"/"-h"
// are "help" and "--version" is "version" (spec/uat-round-1 ac-1/ac-2 name
// both spellings of each), so the enumeration gains one entry per
// capability, never one per spelling. Adding a spelling to a verb here is a
// deliberate edit, not an accident — the arm's literal set must match this
// map exactly or the arm is not blessed.
//
// Adding a KEY here is the load-bearing decision: it declares a new
// dispatched-but-not-phase-numbered verb, which cliVerbs then enumerates
// and showcaseCoverage must map to real showcase-backed evidence. That is
// the whole point — a pre-phase verb cannot be added quietly.
var prePhaseSpellings = map[string][]string{
	"lint":    {"lint"},
	"help":    {"help", "--help", "-h"},
	"version": {"version", "--version"},
}

// prePhaseVerbs returns prePhaseSpellings' keys in sorted order (a stable
// enumeration, independent of map iteration order).
func prePhaseVerbs() []string {
	verbs := make([]string, 0, len(prePhaseSpellings))
	for v := range prePhaseSpellings {
		verbs = append(verbs, v)
	}
	sort.Strings(verbs)
	return verbs
}

// TestShowcaseCoverage_EnumerationIsComplete proves the CLI axis's enumeration
// is COMPLETE, not merely that its red-direction SEAM works
// (TestShowcaseCoverage_RealEnumerationDetectsGaps proves the seam). cliVerbs
// enumerates exactly verbPhase's phase>0 keys plus prePhaseSpellings' keys,
// because dispatch.go's run() special-cases those few verbs BEFORE the
// `verbPhase[verb]` lookup and resolves every other verb THROUGH that lookup
// (run()'s `if !known { usage; return }` rejects any verb that is not a
// verbPhase key). So a verb can ship dispatched-but-unenumerated — invisible
// to the whole gate, this story's own silent pass — only via a pre-phase
// branch on the verb that prePhaseSpellings does not declare.
//
// This test DEFAULT-DENIES that: it parses run() and requires that, in the
// region before the verbPhase lookup, the ONLY statements that reference the
// `verb` identifier are its declaration (`verb := args[0]`) and one blessed
// arm per prePhaseSpellings key — an `if` with no initializer, no else, a
// body that does not itself reference verb, and a condition that tests verb
// against exactly that verb's declared literal spellings. Anything else that
// touches verb pre-phase — a `switch verb`, a reversed `"x" == verb`, a
// `strings.HasPrefix(verb, …)`, an undeclared literal, a helper call taking
// verb — fails here, naming the statement. This is shape-AGNOSTIC on purpose:
// it does not enumerate the evasions (which a shape-specific `verb ==` scan
// would miss), it denies every pre-phase verb reference that is not
// explicitly blessed. Every unexpected AST shape at the boundary itself fails
// with a clear "dispatch.go shape changed" message rather than silently
// enumerating nothing.
//
// SCOPE LIMIT, stated plainly so the paragraph above is not read as more
// than it proves: this witness denies pre-phase statements that NAME the
// `verb` identifier. A statement branching on the same value WITHOUT naming
// it — `if args[0] == "frobnicate" { … }`, or a helper taking `args` — is
// outside its model and PASSES, as does a dispatch placed between the
// verbPhase lookup and the `!known` guard rather than before the lookup.
// Both gaps pre-date this file's current shape and are logged in the UAT
// tracker for separate treatment; neither is closed here.
//
// The one helper call blessed as a condition, `isHelpToken(verb)`, is
// itself PINNED (helpTokenSpellings reads cmd/verdi/help.go's own source):
// a helper call is exactly what this witness otherwise denies, because it
// can hide an arbitrary predicate, so the helper's body must be the same
// literal-disjunction shape an inline arm would have to take, and its
// literal set must equal prePhaseSpellings["help"]. Blessing the call
// without pinning the helper would reopen the hole.
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

	// Default-deny every pre-phase statement that touches `verb` and is not
	// the declaration or one of the declared blessed arms.
	sawDecl := false
	sawArm := map[string]bool{}
	for i, stmt := range runBody.List[:boundary] {
		if !refersToIdent(stmt, "verb") {
			continue // e.g. the `if len(args) == 0` guard — never touches verb
		}
		if isVerbDeclaration(stmt) {
			sawDecl = true
			continue
		}
		verb, ok := prePhaseArmVerb(t, stmt)
		if !ok {
			t.Errorf("run() statement #%d before the verbPhase lookup references `verb` in a non-blessed way (%T) — the only pre-phase verb branches allowed are the ones prePhaseSpellings declares (%v), each an `if` on verb's declared literal spellings with no init, no else, and a body that never mentions verb; anything else (a switch on verb, a reversed comparison, strings.HasPrefix(verb, …), an undeclared literal, a helper taking verb) can dispatch a verb verbPhase does not enumerate, shipping it invisible to the gate. Add that verb to verbPhase, or declare it in prePhaseSpellings (which makes cliVerbs enumerate it and showcaseCoverage demand real evidence for it) before it can pass the gate invisibly.", i, stmt, prePhaseVerbs())
			continue
		}
		if sawArm[verb] {
			t.Errorf("run() statement #%d before the verbPhase lookup is a SECOND blessed arm for %q — one arm per declared pre-phase verb, so the dispatched token set stays exactly prePhaseSpellings[%q]", i, verb, verb)
		}
		sawArm[verb] = true
	}
	if !sawDecl {
		t.Errorf("run() no longer declares `verb := args[0]` before the verbPhase lookup (dispatch.go shape changed) — cannot confirm how the pre-phase region resolves the verb")
	}
	for _, verb := range prePhaseVerbs() {
		if !sawArm[verb] {
			t.Errorf("run() no longer special-cases %q (spellings %v) before the verbPhase lookup; cliVerbs hand-appends every prePhaseSpellings key on the premise that run() dispatches it here, so that premise is now stale — drop %q from prePhaseSpellings (and its cli:%s showcaseCoverage entry) or restore the arm", verb, prePhaseSpellings[verb], verb, verb)
		}
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

// prePhaseArmVerb reports which prePhaseSpellings verb stmt is the blessed
// pre-phase arm for, if any. A blessed arm is an `if` with no initializer,
// no else, and a body that does NOT itself reference verb — so the arm only
// TESTS verb and dispatches; it cannot branch on verb any further — whose
// condition resolves to exactly one declared verb's literal spelling set.
// Everything else returns false and is denied by the caller.
func prePhaseArmVerb(t *testing.T, stmt ast.Stmt) (string, bool) {
	t.Helper()

	ifs, ok := stmt.(*ast.IfStmt)
	if !ok || ifs.Init != nil || ifs.Else != nil {
		return "", false
	}
	if refersToIdent(ifs.Body, "verb") {
		return "", false
	}
	spellings, ok := condSpellings(t, ifs.Cond)
	if !ok {
		return "", false
	}
	for _, verb := range prePhaseVerbs() {
		if sameStringSet(spellings, prePhaseSpellings[verb]) {
			return verb, true
		}
	}
	return "", false
}

// condSpellings returns the exact set of literal tokens a blessed arm's
// condition tests `verb` against. Two condition shapes resolve: an inline
// (possibly single-term) disjunction of `verb == "<literal>"`, and the one
// pinned helper call `isHelpToken(verb)`, whose own source is read for its
// literal set. Any other condition returns false.
func condSpellings(t *testing.T, cond ast.Expr) ([]string, bool) {
	t.Helper()

	if call, ok := cond.(*ast.CallExpr); ok {
		fn, ok := call.Fun.(*ast.Ident)
		if !ok || fn.Name != "isHelpToken" || len(call.Args) != 1 {
			return nil, false
		}
		arg, ok := call.Args[0].(*ast.Ident)
		if !ok || arg.Name != "verb" {
			return nil, false
		}
		return helpTokenSpellings(t), true
	}
	return equalityLiterals(cond, "verb")
}

// equalityLiterals flattens expr as a (possibly single-term) disjunction of
// `ident == "<literal>"` comparisons and returns the literals in source
// order. It reports false for ANY other shape — a reversed comparison, a
// different operand, a non-literal right-hand side, a nested call, a `&&`,
// a negation — so only the one shape this witness blesses is ever accepted.
func equalityLiterals(expr ast.Expr, ident string) ([]string, bool) {
	be, ok := expr.(*ast.BinaryExpr)
	if !ok {
		return nil, false
	}
	if be.Op == token.LOR {
		left, ok := equalityLiterals(be.X, ident)
		if !ok {
			return nil, false
		}
		right, ok := equalityLiterals(be.Y, ident)
		if !ok {
			return nil, false
		}
		return append(left, right...), true
	}
	if be.Op != token.EQL {
		return nil, false
	}
	x, ok := be.X.(*ast.Ident)
	if !ok || x.Name != ident {
		return nil, false
	}
	y, ok := be.Y.(*ast.BasicLit)
	if !ok || y.Kind != token.STRING {
		return nil, false
	}
	lit, err := strconv.Unquote(y.Value)
	if err != nil {
		return nil, false
	}
	return []string{lit}, true
}

// helpTokenSpellings returns the EXACT token set cmd/verdi/help.go's
// isHelpToken accepts, read from that helper's own source. The top-level
// help arm in run() is a call to it, and a helper call is precisely what
// this witness otherwise denies (it can hide an arbitrary predicate) — so
// the helper is pinned to the same literal-disjunction shape an inline arm
// must take, `return <param> == "a" || <param> == "b" || …`. Any other
// shape is a hard failure: the witness cannot confirm which tokens run()
// dispatches pre-phase, which is exactly the fact it exists to prove.
func helpTokenSpellings(t *testing.T) []string {
	t.Helper()

	path := filepath.Join(verdiRepoRoot, "cmd", "verdi", "help.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("EnumerationIsComplete: parsing %s: %v", path, err)
	}

	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if ok && fd.Recv == nil && fd.Name.Name == "isHelpToken" {
			fn = fd
		}
	}
	if fn == nil {
		t.Fatalf("EnumerationIsComplete: func isHelpToken not found in %s (help.go shape changed) — run()'s blessed help arm calls it, so its token set cannot be confirmed", path)
	}
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 || len(fn.Type.Params.List[0].Names) != 1 {
		t.Fatalf("EnumerationIsComplete: isHelpToken does not take exactly one named parameter (help.go shape changed) — cannot confirm which tokens it accepts")
	}
	param := fn.Type.Params.List[0].Names[0].Name

	if fn.Body == nil || len(fn.Body.List) != 1 {
		t.Fatalf("EnumerationIsComplete: isHelpToken's body is not a single statement (help.go shape changed) — only `return %s == \"a\" || …` is pinnable; a helper that does anything else can hide an arbitrary predicate behind run()'s blessed help arm", param)
	}
	ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		t.Fatalf("EnumerationIsComplete: isHelpToken's body is not a single `return <expr>` (help.go shape changed) — cannot confirm which tokens it accepts")
	}
	spellings, ok := equalityLiterals(ret.Results[0], param)
	if !ok {
		t.Fatalf("EnumerationIsComplete: isHelpToken does not return a plain disjunction of `%s == \"<literal>\"` comparisons (help.go shape changed) — run()'s blessed help arm delegates its token set to this helper, so any richer predicate here could dispatch a verb verbPhase does not enumerate, invisible to the gate", param)
	}
	return spellings
}

// sameStringSet reports whether got and want are the same multiset of
// strings, order-independent. Multiset, not set: a repeated literal in an
// arm's condition (`verb == "help" || verb == "help" || verb == "help"`)
// must NOT satisfy a three-spelling declaration, so duplicates are counted
// rather than collapsed.
func sameStringSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	a := append([]string(nil), got...)
	b := append([]string(nil), want...)
	sort.Strings(a)
	sort.Strings(b)
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
