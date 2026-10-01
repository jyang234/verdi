package reach

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
)

// Function values (ledger SI-318), and values that carry one (ledger
// SI-319, SI-320: an interface a named function type with methods
// implements, a type parameter constrained to a function type, never the
// empty interface; Program.carriesFunc). The classes ledger SI-321
// discloses fall outside these rules and are neither followed nor failed
// closed until BL-136: values held by dependency code, values passed
// through a type parameter constrained by any, an embedded function type,
// a method-value handler's bound receiver, and (suspected) a store through
// a pointer to a function-typed field; see the package doc. The graph is
// use-based: a function
// value is an edge from the code that names it. That code is in a verb's
// reach whenever the activation that produced the value is part of the
// verb's execution: an uncaptured parameter's value was named by a caller,
// which is in reach; an uncaptured local's by its own function; a call's
// result by its callee; a package-level variable's by its initializer (its
// node). A value named outside the verb's execution reaches it only
// through state that outlives the activation, and each such shape is
// handled explicitly:
//   - a named function-typed (or function-carrying interface) struct
//     field: every read resolves to every value the module stores in it
//     (fields.go); an embedded function type is not followed (SI-321);
//   - a function value captured from an enclosing function: a call through
//     it resolves through the flow, and fails closed when the flow cannot
//     follow it; any other use of it (passed on as an argument, assigned,
//     returned, stored, read element-wise) fails closed;
//   - a registration's wrapper argument: a root of the route (routes.go);
//   - a channel receive yielding a function value, a type assertion to a
//     function type, a dereference of a pointer to a function value not
//     loaded from a package-level variable, and a read of a field-held
//     container of function values: fail closed.
// Failing closed means Reach returns an error naming the site for every
// entry whose reach holds it, never an entry that reaches nothing.

// unfollowed is one function value in a node's code the analysis cannot
// follow: where it is, the expression, why, and (for a capture the flow
// lost) where the flow lost it.
type unfollowed struct {
	pos  token.Pos
	expr string
	why  string
	at   token.Pos
}

// funcSpan is one declared function or function literal's extent.
type funcSpan struct{ pos, end token.Pos }

// indexSpans records every function's extent, per file, so innermost can
// tell which function a position belongs to.
func (g *Graph) indexSpans() {
	for _, pkg := range g.prog.packages {
		for _, f := range pkg.Files {
			tf := g.prog.Fset.File(f.Pos())
			ast.Inspect(f, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.FuncDecl:
					g.spans[tf] = append(g.spans[tf], funcSpan{x.Pos(), x.End()})
				case *ast.FuncLit:
					g.spans[tf] = append(g.spans[tf], funcSpan{x.Pos(), x.End()})
				}
				return true
			})
		}
	}
}

// innermost returns the extent of the innermost function holding pos.
func (g *Graph) innermost(pos token.Pos) funcSpan {
	best := funcSpan{}
	for _, sp := range g.spans[g.prog.Fset.File(pos)] {
		if sp.pos <= pos && pos < sp.end && (best.end == 0 || sp.end-sp.pos < best.end-best.pos) {
			best = sp
		}
	}
	return best
}

// captured reports whether v, read at pos, is a variable of an enclosing
// function rather than of the function the read is in.
func (g *Graph) captured(v *types.Var, pos token.Pos) bool {
	if v.IsField() || v.Pkg() == nil || v.Parent() == v.Pkg().Scope() || !g.inModule(v.Pkg()) {
		return false
	}
	return g.innermost(v.Pos()) != g.innermost(pos)
}

func (g *Graph) fail(from int, e ast.Expr, why string, at token.Pos) {
	g.nodes[from].unfollowed = append(g.nodes[from].unfollowed, unfollowed{pos: e.Pos(), expr: types.ExprString(e), why: why, at: at})
}

// benignBuiltins are the builtins whose arguments are not taken as values:
// a container passed to them gives up no element.
func benignBuiltins() map[string]bool {
	return map[string]bool{"len": true, "cap": true, "delete": true, "clear": true}
}

// valueChecks applies the rules above to node n of from's code. A parent
// is visited before its children, so it marks the child expressions whose
// use is benign (a call's callee, a nil comparison's operand, a benign
// builtin's argument, an assignment's target, a range expression whose
// elements are not taken) before the children are judged.
func (g *Graph) valueChecks(from int, pkg *Package, n ast.Node) {
	switch x := n.(type) {
	case *ast.CallExpr:
		g.calleeValue(from, pkg, x)
	case *ast.BinaryExpr:
		if x.Op == token.EQL || x.Op == token.NEQ {
			g.allowed[unparen(x.X)] = true
			g.allowed[unparen(x.Y)] = true
		}
	case *ast.AssignStmt:
		for _, lhs := range x.Lhs {
			lhs = unparen(lhs)
			g.allowed[lhs] = true
			if ix, ok := lhs.(*ast.IndexExpr); ok {
				g.allowed[unparen(ix.X)] = true // a store into the container, not a read of it
			}
		}
	case *ast.RangeStmt:
		g.rangeValues(from, pkg, x)
	case *ast.Ident:
		g.capturedUse(from, pkg, x)
	case *ast.SelectorExpr:
		g.fieldContainer(from, pkg, x)
	case *ast.UnaryExpr:
		if x.Op == token.ARROW {
			if ch, ok := pkg.Info.TypeOf(x.X).Underlying().(*types.Chan); ok && g.prog.carriesFunc(ch.Elem()) {
				g.fail(from, x, "receives a function value from a channel, which code outside this reach may have sent", x.Pos())
			}
		}
	case *ast.TypeAssertExpr:
		if x.Type != nil && g.prog.carriesFunc(pkg.Info.TypeOf(x.Type)) {
			g.fail(from, x, "asserts an interface value to a function type, a value code outside this reach may have stored", x.Pos())
		}
	case *ast.TypeSwitchStmt:
		for _, st := range x.Body.List {
			for _, te := range st.(*ast.CaseClause).List {
				if tv := pkg.Info.Types[te]; tv.IsType() && g.prog.carriesFunc(tv.Type) {
					g.fail(from, te, "switches an interface value to a function type, a value code outside this reach may have stored", te.Pos())
				}
			}
		}
	case *ast.StarExpr:
		if tv := pkg.Info.Types[x]; !tv.IsType() && g.prog.carriesFunc(tv.Type) && !g.packageLoaded(pkg, x.X, map[types.Object]bool{}) {
			g.fail(from, x, "dereferences a pointer to a function value that was not loaded from a package-level variable", x.Pos())
		}
	}
}

// calleeValue handles a call. A benign builtin's arguments are benign
// uses. A call through a function value captured from an enclosing
// function, or a method called on a captured value that holds one (an
// interface a function type implements, or a named function type: the
// method calls the function, ledger SI-319), resolves through the flow,
// its values becoming edges (an unfollowable one fails closed). Any other
// callee is covered: a field by the field flow, a call's result by its
// callee, an uncaptured variable by the activation that produced it, and
// an element, receive, assertion, or dereference by its own rule.
func (g *Graph) calleeValue(from int, pkg *Package, call *ast.CallExpr) {
	if tv, ok := pkg.Info.Types[call.Fun]; ok && tv.IsBuiltin() {
		if benignBuiltins()[types.ExprString(unparen(call.Fun))] {
			for _, a := range call.Args {
				g.allowed[unparen(a)] = true
			}
		}
		return
	}
	var id *ast.Ident
	if callee, dynamic := dynamicCallee(pkg, call); dynamic {
		id, _ = callee.(*ast.Ident)
	} else if sel, ok := unparen(call.Fun).(*ast.SelectorExpr); ok {
		if s, ok := pkg.Info.Selections[sel]; ok && s.Kind() == types.MethodVal {
			id, _ = unparen(sel.X).(*ast.Ident)
		}
	}
	if id == nil {
		return
	}
	v, ok := pkg.Info.Uses[id].(*types.Var)
	if !ok || !g.prog.carriesFunc(v.Type()) || !g.captured(v, id.Pos()) {
		return
	}
	g.allowed[id] = true
	r := &resolver{p: g.prog, bound: g.bound, seen: map[any]bool{}, out: newFuncValues()}
	r.variable(v, id.Pos())
	for _, nid := range g.valueIDs(r.out) {
		g.edge(from, nid)
	}
	if r.out.opaque {
		g.fail(from, id, "calls a function value captured from an enclosing function that the flow cannot follow", r.out.opaqueAt)
	}
}

// capturedUse fails closed on a use of a captured function value (or a
// container of them) other than calling it: passed on as an argument,
// assigned, returned, stored, or read element-wise, the value leaves the
// shapes the analysis follows.
func (g *Graph) capturedUse(from int, pkg *Package, id *ast.Ident) {
	v, ok := pkg.Info.Uses[id].(*types.Var)
	if !ok || g.allowed[id] || !g.prog.carriesFunc(v.Type()) || !g.captured(v, id.Pos()) {
		return
	}
	g.fail(from, id, "uses a function value captured from an enclosing function other than by calling it (passed on, stored, or read element-wise)", id.Pos())
}

// fieldContainer fails closed on a read of a field-held container of
// function values (its elements were stored by code the field flow does
// not follow), except a benign use: a benign builtin's argument, a nil
// comparison, an assignment's target, a range that takes no element.
func (g *Graph) fieldContainer(from int, pkg *Package, sel *ast.SelectorExpr) {
	if g.allowed[sel] || g.stores[sel] {
		return
	}
	s, ok := pkg.Info.Selections[sel]
	if !ok || s.Kind() != types.FieldVal {
		return
	}
	field, _ := s.Obj().(*types.Var)
	if g.prog.isFlowField(field) || !g.prog.carriesFunc(field.Type()) {
		return // a function-typed or function-carrying interface field resolves through the field flow
	}
	g.fail(from, sel, "reads a field-held container of function values, whose elements code outside this reach may have stored", sel.Pos())
}

// rangeValues handles a range statement: ranging over a channel of
// function values receives them (fail closed); a range that takes no
// element of a container is a benign use of it.
func (g *Graph) rangeValues(from int, pkg *Package, rs *ast.RangeStmt) {
	t := pkg.Info.TypeOf(rs.X)
	if t == nil {
		return
	}
	if ch, ok := t.Underlying().(*types.Chan); ok {
		if g.prog.carriesFunc(ch.Elem()) && rs.Key != nil {
			g.fail(from, rs.X, "receives function values from a channel, which code outside this reach may have sent", rs.X.Pos())
		}
		g.allowed[unparen(rs.X)] = true
		return
	}
	if rs.Value == nil {
		g.allowed[unparen(rs.X)] = true
	}
}

// packageLoaded reports whether pointer expression e was loaded from a
// package-level variable of the module: the variable itself, its address,
// a method called on it (atomic.Pointer's Load), or a local every
// assignment of which is one of those. Such a value was stored by
// initialization code, which the pre-dispatch entry covers.
func (g *Graph) packageLoaded(pkg *Package, e ast.Expr, seen map[types.Object]bool) bool {
	switch x := unparen(e).(type) {
	case *ast.Ident:
		v, ok := pkg.Info.Uses[x].(*types.Var)
		if !ok || seen[v] {
			return false
		}
		seen[v] = true
		if v.Pkg() != nil && v.Parent() == v.Pkg().Scope() {
			return g.inModule(v.Pkg())
		}
		if _, isParam := g.prog.flow.params[v]; isParam {
			return false
		}
		sites := g.prog.flow.locals[v]
		if len(sites) == 0 {
			return false
		}
		for _, s := range sites {
			if s.expr == nil || s.idx >= 0 || !g.packageLoaded(s.pkg, s.expr, seen) {
				return false
			}
		}
		return true
	case *ast.SelectorExpr:
		if _, isSel := pkg.Info.Selections[x]; isSel {
			return false // a field
		}
		v, ok := pkg.Info.Uses[x.Sel].(*types.Var)
		return ok && v.Pkg() != nil && v.Parent() == v.Pkg().Scope() && g.inModule(v.Pkg())
	case *ast.UnaryExpr:
		return x.Op == token.AND && g.packageLoaded(pkg, x.X, seen)
	case *ast.CallExpr:
		sel, ok := unparen(x.Fun).(*ast.SelectorExpr)
		if !ok {
			return false
		}
		if s, isSel := pkg.Info.Selections[sel]; !isSel || s.Kind() != types.MethodVal {
			return false
		}
		return g.packageLoaded(pkg, sel.X, seen)
	}
	return false
}

// checkUnfollowed fails closed on the first function value in n's code
// the analysis cannot follow, naming its site.
func (g *Graph) checkUnfollowed(entry Entry, n *node) error {
	if len(n.unfollowed) == 0 {
		return nil
	}
	u := n.unfollowed[0]
	return fmt.Errorf("reach: entry %s %q: %s: %s %s (at %s); fail closed",
		entry.Surface, entry.Name, g.prog.Fset.Position(u.pos), u.expr, u.why, g.prog.Fset.Position(u.at))
}

// fieldValues resolves a function-typed field, once per graph.
func (g *Graph) fieldValues(field *types.Var) *funcValues {
	if v, ok := g.fieldVals[field]; ok {
		return v
	}
	v := g.prog.resolveField(field, g.bound)
	g.fieldVals[field] = v
	return v
}

// valueIDs returns the nodes of what v holds: an interface method stands
// for its own node and every implementation's.
func (g *Graph) valueIDs(v *funcValues) []int {
	ids := map[int]bool{}
	for fn := range v.funcs {
		if id, ok := g.byFunc[fn.Origin()]; ok {
			ids[id] = true
		}
		if isAbstract(fn) {
			for _, impl := range g.implementations(fn) {
				if id, ok := g.byFunc[impl.Origin()]; ok {
					ids[id] = true
				}
			}
		}
	}
	for lit := range v.lits {
		if id, ok := g.byLit[lit]; ok {
			ids[id] = true
		}
	}
	for vr := range v.vars {
		if id, ok := g.byVar[vr]; ok {
			ids[id] = true
		}
	}
	return sortedKeys(ids)
}
