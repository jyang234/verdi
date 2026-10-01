package reach

import (
	"go/ast"
	"go/token"
	"go/types"
)

// Function values: the flow resolver.
//
// The call graph is use-based: a function is an edge from the code that
// names it. A function value named by one piece of code and called by
// another is therefore an edge from the code that named it. The flow
// resolves an expression holding a function value to every function value
// that can reach it, following stores into function-typed fields,
// assignments to local variables, the arguments a declared function's
// direct callers bind to its parameters, conversions, and the returns of
// the function (declared or literal) a call reaches. A dependency's
// function, or a call to one, reaches module code only through the
// function values it was handed. A value the flow cannot follow (an index
// expression on anything but a package-level variable, a type assertion,
// a range variable, a parameter of a function that escapes as a value or
// of a function literal, ...) makes the result opaque. Three readers use
// it (ledger SI-318): a read of a function-typed field, a call through a
// function value captured from an enclosing function (valuecalls.go), and
// a registration's wrapper argument (routes.go); each fails closed on an
// opaque result where its rule says so.

// flowSite is one value: expr evaluated in pkg, the idx-th result when
// expr is a multi-value call (-1 otherwise), or, when obj is set, the
// values of that variable. A site with neither is a value the flow cannot
// follow.
type flowSite struct {
	pkg  *Package
	expr ast.Expr
	obj  *types.Var
	idx  int
	pos  token.Pos
}

// paramSlot locates a parameter of a declared function.
type paramSlot struct {
	fn       *types.Func
	index    int
	variadic bool
}

// flowCall is one direct call of a declared function.
type flowCall struct {
	pkg  *Package
	call *ast.CallExpr
}

// fieldFlow is the module-wide record the resolution reads.
type fieldFlow struct {
	stores  map[*types.Var][]flowSite    // function-typed field (origin) -> values stored in it
	locals  map[*types.Var][]flowSite    // non-package variable -> values assigned to it
	params  map[*types.Var]paramSlot     // parameter of a declared function -> its slot
	calls   map[*types.Func][]flowCall   // declared function (origin) -> its direct calls
	escapes map[*types.Func]bool         // declared function used other than as a direct callee
	returns map[*types.Func][][]flowSite // declared function (origin) -> per return statement, its results
	bodies  map[*types.Func]bool         // declared functions with a body

	litReturns map[*ast.FuncLit][][]flowSite // function literal -> per return statement, its results
	litPkg     map[*ast.FuncLit]*Package     // function literal -> the package declaring it
}

// isFuncField reports whether v is a struct field of function type.
func isFuncField(v *types.Var) bool {
	if v == nil || !v.IsField() {
		return false
	}
	_, ok := v.Type().Underlying().(*types.Signature)
	return ok
}

// isFlowField reports whether v is a struct field the field flow
// follows: one of function type, or of an interface type a named function
// type with methods implements (ledger SI-319).
func (p *Program) isFlowField(v *types.Var) bool {
	if isFuncField(v) {
		return true
	}
	if v == nil || !v.IsField() {
		return false
	}
	iface, ok := v.Type().Underlying().(*types.Interface)
	return ok && p.ifaceCarries(iface)
}

// flowFieldOf returns the field a selector reads when the field flow
// follows it, or nil.
func (p *Program) flowFieldOf(pkg *Package, sel *ast.SelectorExpr) *types.Var {
	s, ok := pkg.Info.Selections[sel]
	if !ok || s.Kind() != types.FieldVal {
		return nil
	}
	v, _ := s.Obj().(*types.Var)
	if !p.isFlowField(v) {
		return nil
	}
	return v.Origin()
}

// newFieldFlow records every store into a function-typed field, every
// assignment to a local variable, every direct call of a declared
// function, and every declared function's returns.
func newFieldFlow(prog *Program) *fieldFlow {
	ff := &fieldFlow{
		stores:  map[*types.Var][]flowSite{},
		locals:  map[*types.Var][]flowSite{},
		params:  map[*types.Var]paramSlot{},
		calls:   map[*types.Func][]flowCall{},
		escapes: map[*types.Func]bool{},
		returns: map[*types.Func][][]flowSite{},
		bodies:  map[*types.Func]bool{},

		litReturns: map[*ast.FuncLit][][]flowSite{},
		litPkg:     map[*ast.FuncLit]*Package{},
	}
	for fn, d := range prog.decls {
		sig := fn.Type().(*types.Signature)
		for i := 0; i < sig.Params().Len(); i++ {
			ff.params[sig.Params().At(i)] = paramSlot{fn: fn, index: i, variadic: sig.Variadic() && i == sig.Params().Len()-1}
		}
		if d.decl.Body != nil {
			ff.bodies[fn] = true
			ff.returns[fn] = returnSites(d.pkg, d.decl.Body, sig)
		}
	}
	for _, pkg := range prog.packages {
		callees := map[ast.Expr]bool{}
		sels := map[*ast.Ident]bool{}
		for _, f := range pkg.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.SelectorExpr:
					sels[x.Sel] = true
				case *ast.CallExpr:
					callees[calleeExpr(x)] = true
					if fn := prog.staticCallee(pkg, x); fn != nil {
						ff.calls[fn] = append(ff.calls[fn], flowCall{pkg: pkg, call: x})
					}
				case *ast.FuncLit:
					ff.litPkg[x] = pkg
					if sig, ok := pkg.Info.TypeOf(x).(*types.Signature); ok {
						ff.litReturns[x] = returnSites(pkg, x.Body, sig)
					}
				case *ast.CompositeLit:
					ff.recordComposite(prog, pkg, x)
				case *ast.AssignStmt:
					ff.recordAssign(prog, pkg, x)
				case *ast.ValueSpec:
					ff.recordValueSpec(pkg, x)
				case *ast.RangeStmt:
					for _, e := range []ast.Expr{x.Key, x.Value} {
						if v := localVar(pkg, e); v != nil {
							ff.locals[v] = append(ff.locals[v], flowSite{pkg: pkg, pos: e.Pos()})
						}
					}
				}
				return true
			})
		}
		// A declared function named anywhere but as a direct callee escapes:
		// it may be called with arguments no direct call site shows.
		for _, f := range pkg.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				var id *ast.Ident
				switch x := n.(type) {
				case *ast.SelectorExpr:
					if callees[x] {
						return true
					}
					id = x.Sel
				case *ast.Ident:
					if sels[x] || callees[x] {
						return true
					}
					id = x
				default:
					return true
				}
				if fn, ok := pkg.Info.Uses[id].(*types.Func); ok {
					if _, declared := prog.decls[fn.Origin()]; declared {
						ff.escapes[fn.Origin()] = true
					}
				}
				return true
			})
		}
	}
	return ff
}

// calleeExpr returns the expression a call names its function by, with
// parentheses and generic instantiation removed.
func calleeExpr(call *ast.CallExpr) ast.Expr {
	fun := unparen(call.Fun)
	switch ix := fun.(type) {
	case *ast.IndexExpr:
		return unparen(ix.X)
	case *ast.IndexListExpr:
		return unparen(ix.X)
	}
	return fun
}

// returnSites collects, per return statement of body (not inside a nested
// literal), the value of each result.
func returnSites(pkg *Package, body *ast.BlockStmt, sig *types.Signature) [][]flowSite {
	var out [][]flowSite
	n := sig.Results().Len()
	ast.Inspect(body, func(node ast.Node) bool {
		switch r := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			sites := make([]flowSite, n)
			switch {
			case len(r.Results) == n:
				for i, e := range r.Results {
					sites[i] = flowSite{pkg: pkg, expr: e, idx: -1, pos: e.Pos()}
				}
			case len(r.Results) == 1 && n > 1:
				for i := range sites {
					sites[i] = flowSite{pkg: pkg, expr: r.Results[0], idx: i, pos: r.Pos()}
				}
			case len(r.Results) == 0:
				// A bare return yields the named results' variables.
				for i := range sites {
					sites[i] = flowSite{pkg: pkg, obj: sig.Results().At(i), idx: -1, pos: r.Pos()}
				}
			}
			out = append(out, sites)
		}
		return true
	})
	return out
}

// localVar returns the non-package variable e names, or nil.
func localVar(pkg *Package, e ast.Expr) *types.Var {
	id, ok := e.(*ast.Ident)
	if !ok || id == nil || id.Name == "_" {
		return nil
	}
	obj := pkg.Info.Defs[id]
	if obj == nil {
		obj = pkg.Info.Uses[id]
	}
	v, ok := obj.(*types.Var)
	if !ok || v.IsField() || v.Pkg() == nil || v.Parent() == v.Pkg().Scope() {
		return nil
	}
	return v
}

func (ff *fieldFlow) recordComposite(prog *Program, pkg *Package, lit *ast.CompositeLit) {
	t := pkg.Info.TypeOf(lit)
	if t == nil {
		return
	}
	if p, ok := t.Underlying().(*types.Pointer); ok {
		t = p.Elem()
	}
	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		return
	}
	for i, elt := range lit.Elts {
		var field *types.Var
		val := elt
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			key, ok := kv.Key.(*ast.Ident)
			if !ok {
				continue
			}
			field, _ = pkg.Info.Uses[key].(*types.Var)
			val = kv.Value
		} else if i < st.NumFields() {
			field = st.Field(i)
		}
		if prog.isFlowField(field) {
			ff.stores[field.Origin()] = append(ff.stores[field.Origin()], flowSite{pkg: pkg, expr: val, idx: -1, pos: val.Pos()})
		}
	}
}

func (ff *fieldFlow) recordAssign(prog *Program, pkg *Package, as *ast.AssignStmt) {
	if as.Tok != token.ASSIGN && as.Tok != token.DEFINE {
		return
	}
	for i, lhs := range as.Lhs {
		var site flowSite
		switch {
		case len(as.Lhs) == len(as.Rhs):
			site = flowSite{pkg: pkg, expr: as.Rhs[i], idx: -1, pos: as.Rhs[i].Pos()}
		case len(as.Rhs) == 1:
			site = flowSite{pkg: pkg, expr: as.Rhs[0], idx: i, pos: as.Rhs[0].Pos()}
		default:
			continue
		}
		if sel, ok := unparen(lhs).(*ast.SelectorExpr); ok {
			if field := prog.flowFieldOf(pkg, sel); field != nil {
				ff.stores[field] = append(ff.stores[field], site)
			}
			continue
		}
		if v := localVar(pkg, unparen(lhs)); v != nil {
			ff.locals[v] = append(ff.locals[v], site)
		}
	}
}

func (ff *fieldFlow) recordValueSpec(pkg *Package, vs *ast.ValueSpec) {
	for i, name := range vs.Names {
		v := localVar(pkg, name)
		if v == nil {
			continue
		}
		switch {
		case len(vs.Values) == len(vs.Names):
			ff.locals[v] = append(ff.locals[v], flowSite{pkg: pkg, expr: vs.Values[i], idx: -1, pos: vs.Values[i].Pos()})
		case len(vs.Values) == 1:
			ff.locals[v] = append(ff.locals[v], flowSite{pkg: pkg, expr: vs.Values[0], idx: i, pos: vs.Values[0].Pos()})
		}
	}
}

// funcValues is what a function-valued expression may hold: module
// functions and methods (an interface method stands for every module
// implementation), function literals, and package-level variables (whose
// node carries their initializer). needs names route-table fields whose
// values only an entry that binds them knows (Entry.Bound); opaque marks a
// value the flow cannot follow, first met at opaqueAt.
type funcValues struct {
	funcs    map[*types.Func]bool
	lits     map[*ast.FuncLit]bool
	vars     map[*types.Var]bool
	needs    map[*types.Var]bool
	opaque   bool
	opaqueAt token.Pos
}

func newFuncValues() *funcValues {
	return &funcValues{funcs: map[*types.Func]bool{}, lits: map[*ast.FuncLit]bool{}, vars: map[*types.Var]bool{}, needs: map[*types.Var]bool{}}
}

func (v *funcValues) fail(pos token.Pos) {
	if !v.opaque {
		v.opaque, v.opaqueAt = true, pos
	}
}

// resolve returns what e, evaluated in pkg, may hold; reads of the fields
// in bound are recorded as needs rather than followed.
func (p *Program) resolve(pkg *Package, e ast.Expr, bound map[*types.Var]bool) *funcValues {
	r := &resolver{p: p, bound: bound, seen: map[any]bool{}, out: newFuncValues()}
	r.expr(pkg, e, -1)
	return r.out
}

// resolveField returns what a function-typed field may hold.
func (p *Program) resolveField(f *types.Var, bound map[*types.Var]bool) *funcValues {
	r := &resolver{p: p, bound: bound, seen: map[any]bool{}, out: newFuncValues()}
	r.field(f)
	return r.out
}

// resolver follows one expression's values to the functions they name.
type resolver struct {
	p     *Program
	bound map[*types.Var]bool
	seen  map[any]bool
	out   *funcValues
}

// returnKey is one result of one declared function or literal.
type returnKey struct {
	fn  any
	idx int
}

func (r *resolver) field(f *types.Var) {
	if r.bound[f] {
		r.out.needs[f] = true
		return
	}
	if r.seen[f] {
		return
	}
	r.seen[f] = true
	for _, s := range r.p.flow.stores[f] {
		r.site(s)
	}
}

func (r *resolver) site(s flowSite) {
	switch {
	case s.obj != nil:
		r.variable(s.obj, s.pos)
	case s.expr != nil:
		r.expr(s.pkg, s.expr, s.idx)
	default:
		r.out.fail(s.pos)
	}
}

func (r *resolver) expr(pkg *Package, e ast.Expr, idx int) {
	if t := pkg.Info.TypeOf(e); t != nil && !r.p.carriesFunc(t) {
		if _, isTuple := t.(*types.Tuple); !isTuple {
			// A value that cannot hold a function (a struct behind an
			// interface, nil, a constant): class-hierarchy analysis
			// dispatches its methods; there is nothing to follow.
			return
		}
	}
	switch x := unparen(e).(type) {
	case *ast.FuncLit:
		r.out.lits[x] = true
		return
	case *ast.Ident:
		r.object(pkg.Info.Uses[x], x.Pos())
		return
	case *ast.SelectorExpr:
		sel, ok := pkg.Info.Selections[x]
		if !ok {
			// A qualified identifier: another package's function or variable.
			r.object(pkg.Info.Uses[x.Sel], x.Pos())
			return
		}
		switch sel.Kind() {
		case types.FieldVal:
			if f := r.p.flowFieldOf(pkg, x); f != nil {
				r.field(f)
				return
			}
		case types.MethodVal, types.MethodExpr:
			if fn, ok := sel.Obj().(*types.Func); ok {
				r.function(fn, x.Pos())
				return
			}
		}
	case *ast.IndexExpr:
		if r.packageContainer(pkg, x.X) {
			return
		}
		if fn := genericFunc(pkg, x.X); fn != nil {
			r.function(fn, x.Pos())
			return
		}
	case *ast.IndexListExpr:
		if fn := genericFunc(pkg, x.X); fn != nil {
			r.function(fn, x.Pos())
			return
		}
	case *ast.CallExpr:
		r.result(pkg, x, idx)
		return
	}
	r.out.fail(e.Pos())
}

// packageContainer resolves an element of a package-level variable (a map
// or slice of functions) to that variable: its node carries the
// initializer's elements, and the ground rules forbid changing it later
// outside initialization, which the pre-dispatch entry covers.
func (r *resolver) packageContainer(pkg *Package, x ast.Expr) bool {
	var id *ast.Ident
	switch c := unparen(x).(type) {
	case *ast.Ident:
		id = c
	case *ast.SelectorExpr:
		if _, isSel := pkg.Info.Selections[c]; isSel {
			return false
		}
		id = c.Sel
	default:
		return false
	}
	v, ok := pkg.Info.Uses[id].(*types.Var)
	if !ok || v.Pkg() == nil || v.Parent() != v.Pkg().Scope() || r.p.byPath[v.Pkg().Path()] == nil {
		return false
	}
	r.out.vars[v] = true
	return true
}

// genericFunc returns the generic module function e instantiates, or nil.
func genericFunc(pkg *Package, e ast.Expr) *types.Func {
	var id *ast.Ident
	switch f := unparen(e).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	default:
		return nil
	}
	fn, ok := pkg.Info.Uses[id].(*types.Func)
	if !ok {
		return nil
	}
	return fn
}

// result follows the idx-th result of call to what the function it
// reaches returns.
func (r *resolver) result(pkg *Package, call *ast.CallExpr, idx int) {
	if tv, ok := pkg.Info.Types[call.Fun]; ok && tv.IsType() {
		if len(call.Args) == 1 {
			r.expr(pkg, call.Args[0], -1)
			return
		}
		r.out.fail(call.Pos())
		return
	}
	if fn := r.p.staticCallee(pkg, call); fn != nil {
		r.returns(fn, idx, call.Pos())
		return
	}
	if dependencyCall(r.p, pkg, call) {
		// A dependency's function returns module code only if it was
		// handed some: follow the function values among its arguments.
		for _, a := range call.Args {
			if isFuncTyped(pkg, a) {
				r.expr(pkg, a, -1)
			}
		}
		return
	}
	callee, dynamic := dynamicCallee(pkg, call)
	if !dynamic {
		// A method called through an interface: what each implementation
		// returns.
		if fn := abstractCallee(pkg, call); fn != nil {
			for _, impl := range r.p.implementationsOf(fn) {
				r.returns(impl, idx, call.Pos())
			}
			return
		}
		r.out.fail(call.Pos())
		return
	}
	// A call through a function value: what each value it may hold
	// returns.
	inner := &resolver{p: r.p, bound: r.bound, seen: r.seen, out: newFuncValues()}
	inner.expr(pkg, callee, -1)
	if inner.out.opaque {
		r.out.fail(inner.out.opaqueAt)
	}
	for f := range inner.out.needs {
		r.out.needs[f] = true // what a bound field's values return is reached from the entry's roots
	}
	for fn := range inner.out.funcs {
		if isAbstract(fn) {
			for _, impl := range r.p.implementationsOf(fn) {
				r.returns(impl, idx, call.Pos())
			}
			continue
		}
		r.returns(fn, idx, call.Pos())
	}
	for lit := range inner.out.lits {
		r.litResult(lit, idx, call.Pos())
	}
	if len(inner.out.vars) > 0 {
		r.out.fail(call.Pos()) // a call of a function a package-level variable holds, for its result
	}
}

func isFuncTyped(pkg *Package, e ast.Expr) bool {
	t := pkg.Info.TypeOf(e)
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Signature)
	return ok
}

// dependencyCall reports whether call statically calls a function declared
// outside the module.
func dependencyCall(p *Program, pkg *Package, call *ast.CallExpr) bool {
	var id *ast.Ident
	switch f := calleeExpr(call).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	default:
		return false
	}
	fn, ok := pkg.Info.Uses[id].(*types.Func)
	return ok && !isAbstract(fn) && p.byPath[fn.Pkg().Path()] == nil
}

// abstractCallee returns the interface method call calls, or nil.
func abstractCallee(pkg *Package, call *ast.CallExpr) *types.Func {
	sel, ok := unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	fn, ok := pkg.Info.Uses[sel.Sel].(*types.Func)
	if !ok || !isAbstract(fn) {
		return nil
	}
	return fn
}

// dynamicCallee returns the expression a call obtains its function from
// when that function is a value rather than a declared function or method:
// a variable, a struct field, a call's result, an element, an assertion.
// Conversions, builtins, declared functions and methods (an interface's
// included: class-hierarchy analysis resolves those), generic
// instantiations, and immediately called literals are not dynamic.
func dynamicCallee(pkg *Package, call *ast.CallExpr) (ast.Expr, bool) {
	if tv, ok := pkg.Info.Types[call.Fun]; ok && (tv.IsType() || tv.IsBuiltin()) {
		return nil, false
	}
	f := unparen(call.Fun)
	switch ix := f.(type) {
	case *ast.IndexExpr:
		if genericFunc(pkg, ix.X) != nil {
			return nil, false
		}
	case *ast.IndexListExpr:
		return nil, false
	}
	switch x := f.(type) {
	case *ast.FuncLit:
		return nil, false
	case *ast.Ident:
		if _, isFunc := pkg.Info.Uses[x].(*types.Func); isFunc {
			return nil, false
		}
	case *ast.SelectorExpr:
		if sel, ok := pkg.Info.Selections[x]; ok {
			if sel.Kind() != types.FieldVal {
				return nil, false
			}
		} else if _, isFunc := pkg.Info.Uses[x.Sel].(*types.Func); isFunc {
			return nil, false
		}
	}
	return f, true
}

func (r *resolver) object(obj types.Object, pos token.Pos) {
	switch o := obj.(type) {
	case *types.Nil:
	case *types.Func:
		r.function(o, pos)
	case *types.Var:
		r.variable(o, pos)
	default:
		r.out.fail(pos)
	}
}

// function adds fn as a value: an interface method stands for every module
// implementation; a dependency's function runs no module code.
func (r *resolver) function(fn *types.Func, pos token.Pos) {
	if isAbstract(fn) {
		r.out.funcs[fn] = true
		return
	}
	if r.p.byPath[fn.Pkg().Path()] == nil {
		return
	}
	if _, ok := r.p.decls[fn.Origin()]; !ok {
		r.out.fail(pos)
		return
	}
	r.out.funcs[fn.Origin()] = true
}

func (r *resolver) variable(v *types.Var, pos token.Pos) {
	if r.seen[v] {
		return
	}
	r.seen[v] = true
	if v.Pkg() != nil && v.Parent() == v.Pkg().Scope() {
		// A package-level variable: its node carries its initializer's
		// edges (and an initializer that runs code is pre-dispatch code).
		if r.p.byPath[v.Pkg().Path()] != nil {
			r.out.vars[v] = true
			return
		}
		r.out.fail(pos)
		return
	}
	if slot, ok := r.p.flow.params[v]; ok {
		r.param(slot, pos)
		return
	}
	if sites, ok := r.p.flow.locals[v]; ok {
		for _, s := range sites {
			r.site(s)
		}
		return
	}
	// A receiver, a function literal's parameter, or a variable no
	// assignment names.
	r.out.fail(pos)
}

// param follows a declared function's parameter to the arguments its
// direct callers bind; a function that escapes as a value has callers no
// call site shows.
func (r *resolver) param(slot paramSlot, pos token.Pos) {
	if slot.variadic || r.p.flow.escapes[slot.fn] {
		r.out.fail(pos)
		return
	}
	n := slot.fn.Type().(*types.Signature).Params().Len()
	for _, c := range r.p.flow.calls[slot.fn] {
		args := c.call.Args
		if sel, ok := calleeExpr(c.call).(*ast.SelectorExpr); ok {
			if s, ok := c.pkg.Info.Selections[sel]; ok && s.Kind() == types.MethodExpr && len(args) > 0 {
				args = args[1:] // a method expression's first argument is its receiver
			}
		}
		if len(args) != n {
			r.out.fail(c.call.Pos()) // a tuple spread into the parameters
			continue
		}
		r.expr(c.pkg, args[slot.index], -1)
	}
}

// returns follows the idx-th result of a declared function to what its
// return statements yield.
func (r *resolver) returns(fn *types.Func, idx int, pos token.Pos) {
	if idx < 0 {
		idx = 0
	}
	key := returnKey{fn: fn, idx: idx}
	if r.seen[key] {
		return
	}
	r.seen[key] = true
	if !r.p.flow.bodies[fn] {
		r.out.fail(pos)
		return
	}
	r.yields(r.p.flow.returns[fn], idx, pos)
}

// litResult follows the idx-th result of a function literal.
func (r *resolver) litResult(lit *ast.FuncLit, idx int, pos token.Pos) {
	if idx < 0 {
		idx = 0
	}
	key := returnKey{fn: lit, idx: idx}
	if r.seen[key] {
		return
	}
	r.seen[key] = true
	r.yields(r.p.flow.litReturns[lit], idx, pos)
}

func (r *resolver) yields(rets [][]flowSite, idx int, pos token.Pos) {
	for _, ret := range rets {
		if idx >= len(ret) {
			r.out.fail(pos)
			continue
		}
		r.site(ret[idx])
	}
}
