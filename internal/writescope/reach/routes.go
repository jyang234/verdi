package reach

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"sort"
	"strings"
)

// Route discovery: RouteEntries reads a package's http.ServeMux
// registrations, statically evaluating each pattern and handler (through a
// route table when the registration ranges over one), and scans each
// handler for the actions it dispatches on a route wildcard. A
// registration that hands a route table's row to the handler it calls
// (internal/workbench's /b/{branch} mount: bb.dispatch(rt)) also roots the
// route at the row's handler values, binds the row's function-typed
// fields to them, and splits them on the route's wildcards like any other
// handler, so the actions the row's handler dispatches are derived under
// that route too.

// RouteEntries derives the routes pkgPath registers on an http.ServeMux,
// and the actions their handlers dispatch on a path wildcard.
func RouteEntries(prog *Program, pkgPath, surface string) ([]Entry, error) {
	pkg := prog.Package(pkgPath)
	if pkg == nil {
		return nil, fmt.Errorf("reach: no module package %s", pkgPath)
	}
	type route struct {
		pattern string
		roots   []Root
		bound   []*types.Var
		site    Root
		pkg     *Package
	}
	var routes []route
	for _, f := range pkg.Files {
		var ranges []*ast.RangeStmt
		ast.Inspect(f, func(n ast.Node) bool {
			if r, ok := n.(*ast.RangeStmt); ok {
				ranges = append(ranges, r)
			}
			return true
		})
		var ferr error
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || ferr != nil || !isServeMuxRegistration(pkg, call) {
				return true
			}
			if len(call.Args) != 2 {
				ferr = fmt.Errorf("reach: %s: a ServeMux registration takes two arguments", prog.Fset.Position(call.Pos()))
				return false
			}
			envs, err := prog.rowEnvs(pkg, call, ranges)
			if err != nil {
				ferr = err
				return false
			}
			site, err := enclosingFunc(pkg, f, call)
			if err != nil {
				ferr = err
				return false
			}
			for _, env := range envs {
				pattern, err := prog.evalString(pkg, call.Args[0], env)
				if err != nil {
					ferr = err
					return false
				}
				roots, bound, err := prog.evalHandler(pkg, call.Args[1], env)
				if err != nil {
					ferr = err
					return false
				}
				routes = append(routes, route{pattern: pattern, roots: roots, bound: bound, site: site, pkg: pkg})
			}
			return true
		})
		if ferr != nil {
			return nil, ferr
		}
	}
	if len(routes) == 0 {
		return nil, fmt.Errorf("reach: %s registers no route", pkgPath)
	}
	seen := map[string]bool{}
	sites := map[string]Root{}
	var out []Entry
	s := newKeyScan(prog, false)
	for _, rt := range routes {
		if seen[rt.pattern] {
			return nil, fmt.Errorf("reach: %s registers route %q twice", pkgPath, rt.pattern)
		}
		seen[rt.pattern] = true
		sites[rt.pattern] = rt.site
		out = append(out, Entry{Surface: surface, Name: rt.pattern, Roots: rt.roots, Site: rt.site, Bound: rt.bound})
		ctx := scanCtx{parent: rt.pattern, prefix: rt.pattern, keys: map[types.Object]string{}, wilds: wildcards(rt.pattern)}
		for _, r := range rt.roots {
			switch {
			case r.Func != nil:
				s.scanFunc(r.Func, ctx)
			case r.Lit != nil:
				ctx.pkg = rt.pkg
				if lp := prog.flow.litPkg[r.Lit]; lp != nil {
					ctx.pkg = lp
				}
				s.scanNode(r.Lit.Body, ctx)
			}
		}
	}
	for _, e := range s.entries(surface) {
		if seen[e.Name] {
			return nil, fmt.Errorf("reach: action %q collides with a registered route", e.Name)
		}
		e.Site = sites[s.parent[e.Name]]
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// --- static evaluation of route registrations ---------------------------

// rowEnv binds a range variable to one row of a route table.
type rowEnv struct {
	rangeVar types.Object
	row      map[string]ast.Expr
	pkg      *Package
}

// rowEnvs returns one environment per table row when the registration sits
// in a range over a route-table function and uses its variable, else one
// empty environment.
func (p *Program) rowEnvs(pkg *Package, call *ast.CallExpr, ranges []*ast.RangeStmt) ([]*rowEnv, error) {
	var inner *ast.RangeStmt
	for _, r := range ranges {
		if r.Pos() <= call.Pos() && call.End() <= r.End() {
			if inner == nil || r.Pos() > inner.Pos() {
				inner = r
			}
		}
	}
	if inner == nil {
		return []*rowEnv{nil}, nil
	}
	val, ok := inner.Value.(*ast.Ident)
	if !ok {
		return []*rowEnv{nil}, nil
	}
	rv := pkg.Info.Defs[val]
	uses := false
	ast.Inspect(call, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && pkg.Info.Uses[id] == rv {
			uses = true
		}
		return true
	})
	if !uses {
		return []*rowEnv{nil}, nil
	}
	tc, ok := unparen(inner.X).(*ast.CallExpr)
	if !ok {
		return nil, fmt.Errorf("reach: %s: a route registration ranges over something other than a route-table function", p.Fset.Position(inner.Pos()))
	}
	fn := p.staticCallee(pkg, tc)
	if fn == nil {
		return nil, fmt.Errorf("reach: %s: the route table is not a module function", p.Fset.Position(inner.Pos()))
	}
	d := p.decls[fn]
	var lit *ast.CompositeLit
	returns := 0
	ast.Inspect(d.decl.Body, func(n ast.Node) bool {
		if r, ok := n.(*ast.ReturnStmt); ok {
			returns++
			if len(r.Results) == 1 {
				lit, _ = unparen(r.Results[0]).(*ast.CompositeLit)
			}
		}
		return true
	})
	if returns != 1 || lit == nil {
		return nil, fmt.Errorf("reach: route table %s must return one composite literal", p.FuncName(fn))
	}
	var envs []*rowEnv
	for _, elt := range lit.Elts {
		cl, ok := elt.(*ast.CompositeLit)
		if !ok {
			return nil, fmt.Errorf("reach: %s: route table row is not a composite literal", p.Fset.Position(elt.Pos()))
		}
		row := map[string]ast.Expr{}
		for _, f := range cl.Elts {
			kv, ok := f.(*ast.KeyValueExpr)
			key, kok := kvKey(kv)
			if !ok || !kok {
				return nil, fmt.Errorf("reach: %s: route table rows must name their fields", p.Fset.Position(f.Pos()))
			}
			row[key] = kv.Value
		}
		envs = append(envs, &rowEnv{rangeVar: rv, row: row, pkg: d.pkg})
	}
	return envs, nil
}

func kvKey(kv *ast.KeyValueExpr) (string, bool) {
	if kv == nil {
		return "", false
	}
	id, ok := kv.Key.(*ast.Ident)
	if !ok {
		return "", false
	}
	return id.Name, true
}

// rowField returns the table row's value for e when e selects a field of
// the range variable.
func (env *rowEnv) rowField(pkg *Package, e ast.Expr) (ast.Expr, bool) {
	if env == nil {
		return nil, false
	}
	sel, ok := unparen(e).(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	id, ok := unparen(sel.X).(*ast.Ident)
	if !ok || pkg.Info.Uses[id] != env.rangeVar {
		return nil, false
	}
	v, ok := env.row[sel.Sel.Name]
	return v, ok
}

func (p *Program) evalString(pkg *Package, e ast.Expr, env *rowEnv) (string, error) {
	if v := pkg.Info.Types[e].Value; v != nil && v.Kind() == constant.String {
		return constant.StringVal(v), nil
	}
	if v, ok := env.rowField(pkg, e); ok {
		return p.evalString(env.pkg, v, nil)
	}
	if b, ok := unparen(e).(*ast.BinaryExpr); ok && b.Op == token.ADD {
		l, err := p.evalString(pkg, b.X, env)
		if err != nil {
			return "", err
		}
		r, err := p.evalString(pkg, b.Y, env)
		if err != nil {
			return "", err
		}
		return l + r, nil
	}
	return "", fmt.Errorf("reach: %s: route pattern is neither a constant nor a route-table field", p.Fset.Position(e.Pos()))
}

// evalHandler resolves a registration's handler to the roots its code
// starts at, and the route-table fields it binds.
func (p *Program) evalHandler(pkg *Package, e ast.Expr, env *rowEnv) ([]Root, []*types.Var, error) {
	switch x := unparen(e).(type) {
	case *ast.FuncLit:
		return []Root{{Lit: x}}, nil, nil
	case *ast.Ident, *ast.SelectorExpr:
		if v, ok := env.rowField(pkg, x); ok {
			return p.evalHandler(env.pkg, v, nil)
		}
		if fn := p.funcValue(pkg, x); fn != nil {
			return []Root{{Func: fn}}, nil, nil
		}
	case *ast.CallExpr:
		if v, ok := env.rowField(pkg, x.Fun); ok {
			if fn := p.funcValue(env.pkg, v); fn != nil {
				return []Root{{Func: fn}}, nil, nil
			}
		}
		if fn := p.staticCallee(pkg, x); fn != nil {
			roots, bound, err := p.rowArgs(pkg, x, env)
			if err != nil {
				return nil, nil, err
			}
			wrapped, err := p.wrapperArgs(pkg, x, env)
			if err != nil {
				return nil, nil, err
			}
			return append(append([]Root{{Func: fn}}, roots...), wrapped...), bound, nil
		}
	}
	return nil, nil, fmt.Errorf("reach: %s: route handler is not a function, method, literal, or route-table field", p.Fset.Position(e.Pos()))
}

// rowArgs returns the extra roots a handler call's arguments carry from
// its route-table row: every function-typed field's value when the row
// itself is handed over (those fields are then bound), or one field's
// value when only that field is.
func (p *Program) rowArgs(pkg *Package, call *ast.CallExpr, env *rowEnv) ([]Root, []*types.Var, error) {
	if env == nil {
		return nil, nil, nil
	}
	var roots []Root
	var bound []*types.Var
	for _, arg := range call.Args {
		a := unparen(arg)
		if u, ok := a.(*ast.UnaryExpr); ok && u.Op == token.AND {
			a = unparen(u.X)
		}
		if id, ok := a.(*ast.Ident); ok && pkg.Info.Uses[id] == env.rangeVar {
			st, ok := env.rangeVar.Type().Underlying().(*types.Struct)
			if !ok {
				return nil, nil, fmt.Errorf("reach: %s: a route-table row handed to a handler is not a struct", p.Fset.Position(arg.Pos()))
			}
			for i := 0; i < st.NumFields(); i++ {
				f := st.Field(i)
				if !isFuncField(f) {
					continue
				}
				bound = append(bound, f.Origin())
				v, ok := env.row[f.Name()]
				if !ok {
					continue // an omitted field is nil: it calls nothing
				}
				root, err := p.rowValue(env, v)
				if err != nil {
					return nil, nil, err
				}
				roots = append(roots, root)
			}
			continue
		}
		if v, ok := env.rowField(pkg, a); ok {
			if t := pkg.Info.TypeOf(a); t != nil {
				if _, isFunc := t.Underlying().(*types.Signature); isFunc {
					root, err := p.rowValue(env, v)
					if err != nil {
						return nil, nil, err
					}
					roots = append(roots, root)
				}
			}
		}
	}
	return roots, bound, nil
}

// wrapperArgs roots every function-typed argument of a registration's
// handler call that is not a route-table row's value (ledger SI-318): the
// wrapped handler is named by the registration, outside the route's own
// code, so the route starts there too. A value the flow cannot follow
// fails closed.
func (p *Program) wrapperArgs(pkg *Package, call *ast.CallExpr, env *rowEnv) ([]Root, error) {
	var out []Root
	for _, arg := range call.Args {
		if !isFuncTyped(pkg, arg) {
			continue
		}
		if _, isRow := env.rowField(pkg, arg); isRow {
			continue
		}
		v := p.resolve(pkg, arg, nil)
		if v.opaque {
			return nil, fmt.Errorf("reach: %s: a registration's wrapper argument holds a function value that cannot be followed statically (fail closed)", p.Fset.Position(arg.Pos()))
		}
		for fn := range v.funcs {
			if isAbstract(fn) {
				for _, impl := range p.implementationsOf(fn) {
					out = append(out, Root{Func: impl})
				}
				continue
			}
			out = append(out, Root{Func: fn})
		}
		for lit := range v.lits {
			out = append(out, Root{Lit: lit})
		}
		for vr := range v.vars {
			out = append(out, Root{Var: vr})
		}
	}
	return out, nil
}

// rowValue resolves one function-typed row field's value; a value that is
// not a module function, method, or literal fails closed.
func (p *Program) rowValue(env *rowEnv, v ast.Expr) (Root, error) {
	if lit, ok := unparen(v).(*ast.FuncLit); ok {
		return Root{Lit: lit}, nil
	}
	if fn := p.funcValue(env.pkg, v); fn != nil {
		return Root{Func: fn}, nil
	}
	return Root{}, fmt.Errorf("reach: %s: a route-table row's function-typed field is not a module function, method, or literal", p.Fset.Position(v.Pos()))
}

// enclosingFunc returns the declared function or function literal of f
// whose body holds n: the code a host reaches when it registers n.
func enclosingFunc(pkg *Package, f *ast.File, n ast.Node) (Root, error) {
	var out Root
	ast.Inspect(f, func(x ast.Node) bool {
		if x == nil || x.Pos() > n.Pos() || n.End() > x.End() {
			return x == nil
		}
		switch fn := x.(type) {
		case *ast.FuncDecl:
			if obj, ok := pkg.Info.Defs[fn.Name].(*types.Func); ok {
				out = Root{Func: obj}
			}
		case *ast.FuncLit:
			out = Root{Lit: fn}
		}
		return true
	})
	if out == (Root{}) {
		return Root{}, fmt.Errorf("reach: %s is not inside a function", pkg.Path)
	}
	return out, nil
}

func isServeMuxRegistration(pkg *Package, call *ast.CallExpr) bool {
	sel, ok := unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	fn, ok := pkg.Info.Uses[sel.Sel].(*types.Func)
	if !ok {
		return false
	}
	switch fn.FullName() {
	case "(*net/http.ServeMux).HandleFunc", "(*net/http.ServeMux).Handle":
		return true
	}
	return false
}

// wildcards returns the single-segment wildcards of a ServeMux pattern.
func wildcards(pattern string) map[string]bool {
	out := map[string]bool{}
	for _, seg := range strings.Split(pattern, "/") {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}")
			if name != "$" && !strings.HasSuffix(name, "...") {
				out[name] = true
			}
		}
	}
	return out
}
