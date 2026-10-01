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

// Entry discovery. Every function here derives entries from the code that
// dispatches them, never from a list kept beside it:
//
//   - CLIEntries follows a dispatcher's []string parameter. args[0] (or a
//     variable holding it) is the level's key; an `if key == "lit"` arm
//     (possibly guarded with &&, or widened with ||) or a `case "lit":`
//     clause of a switch on the key is the verb prefix+lit. Inside an arm,
//     a call handing args[1:] to a function starts the next level, named
//     after the arm. A key handed to a function keeps the level, so a
//     dispatch completed in a helper is still found.
//   - SwitchEntries finds the one switch statement of a package whose string
//     cases are exactly an inventory's names (the MCP tool switch).
//   - RouteEntries reads a package's http.ServeMux registrations; a handler
//     that switches on one of its route's wildcards (r.PathValue("action"))
//     contributes one entry per case, named by the route with the wildcard
//     replaced. A registration whose pattern or handler cannot be resolved
//     statically is an error, never a silently skipped route.

// CLIEntries derives the verbs the dispatcher funcName in pkgPath defines.
func CLIEntries(prog *Program, pkgPath, funcName, surface string) ([]Entry, error) {
	fn, err := lookupFunc(prog, pkgPath, funcName)
	if err != nil {
		return nil, err
	}
	params := fn.Type().(*types.Signature).Params()
	if params.Len() == 0 || !isStringSlice(params.At(0).Type()) {
		return nil, fmt.Errorf("reach: %s.%s has no []string first parameter to dispatch on", pkgPath, funcName)
	}
	s := newKeyScan(prog, true)
	s.scanFunc(fn, scanCtx{parent: PreDispatch, args: map[types.Object]bool{params.At(0): true}, keys: map[types.Object]string{}})
	if len(s.arms) == 0 {
		return nil, fmt.Errorf("reach: %s.%s dispatches no verb", pkgPath, funcName)
	}
	return s.entries(surface), nil
}

// SwitchEntries returns one entry per name, rooted at its clause of the one
// switch statement in pkgPath whose string cases are exactly names.
func SwitchEntries(prog *Program, pkgPath, surface string, names []string) ([]Entry, error) {
	pkg := prog.Package(pkgPath)
	if pkg == nil {
		return nil, fmt.Errorf("reach: no module package %s", pkgPath)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("reach: SwitchEntries needs names")
	}
	want := map[string]bool{}
	for _, n := range names {
		if want[n] {
			return nil, fmt.Errorf("reach: duplicate inventory name %q", n)
		}
		want[n] = true
	}
	var matches []map[string]*ast.CaseClause
	for _, f := range pkg.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			sw, ok := n.(*ast.SwitchStmt)
			if !ok || sw.Tag == nil {
				return true
			}
			clauses := map[string]*ast.CaseClause{}
			for _, st := range sw.Body.List {
				cc := st.(*ast.CaseClause)
				lits, ok := stringConsts(pkg, cc.List)
				if !ok {
					return true
				}
				for _, l := range lits {
					clauses[l] = cc
				}
			}
			if len(clauses) != len(want) {
				return true
			}
			for n := range want {
				if clauses[n] == nil {
					return true
				}
			}
			matches = append(matches, clauses)
			return true
		})
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("reach: %d switch statements in %s have exactly the inventory's %d names as cases, want 1", len(matches), pkgPath, len(names))
	}
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	out := make([]Entry, 0, len(sorted))
	for _, n := range sorted {
		out = append(out, Entry{Surface: surface, Name: n, Roots: []Root{{Arm: matches[0][n]}}})
	}
	return out, nil
}

// StringKeyedMap returns the keys of the map literal initializing the
// package-level variable name in pkgPath, each with its constant value
// written exactly.
func StringKeyedMap(prog *Program, pkgPath, name string) (map[string]string, error) {
	pkg := prog.Package(pkgPath)
	if pkg == nil {
		return nil, fmt.Errorf("reach: no module package %s", pkgPath)
	}
	v, ok := pkg.Types.Scope().Lookup(name).(*types.Var)
	if !ok {
		return nil, fmt.Errorf("reach: %s declares no package-level variable %s", pkgPath, name)
	}
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, id := range vs.Names {
					if pkg.Info.Defs[id] != v || i >= len(vs.Values) {
						continue
					}
					return constMapLiteral(pkg, vs.Values[i])
				}
			}
		}
	}
	return nil, fmt.Errorf("reach: %s.%s has no initializer", pkgPath, name)
}

func constMapLiteral(pkg *Package, e ast.Expr) (map[string]string, error) {
	lit, ok := unparen(e).(*ast.CompositeLit)
	if !ok {
		return nil, fmt.Errorf("reach: initializer is not a composite literal")
	}
	if _, ok := pkg.Info.TypeOf(lit).Underlying().(*types.Map); !ok {
		return nil, fmt.Errorf("reach: initializer is not a map literal")
	}
	out := map[string]string{}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return nil, fmt.Errorf("reach: map literal element is not key: value")
		}
		key := pkg.Info.Types[kv.Key].Value
		val := pkg.Info.Types[kv.Value].Value
		if key == nil || key.Kind() != constant.String || val == nil {
			return nil, fmt.Errorf("reach: map literal entry is not a constant string key with a constant value")
		}
		out[constant.StringVal(key)] = val.ExactString()
	}
	return out, nil
}

// --- key scanning ------------------------------------------------------

// keyScan collects the dispatch arms of one key.
type keyScan struct {
	prog    *Program
	cli     bool
	arms    map[string][]ast.Node
	parent  map[string]string // arm name -> the entry whose dispatch derives it
	visited map[string]bool
}

// scanCtx is one dispatch level.
type scanCtx struct {
	pkg    *Package
	parent string                  // the entry this level's arms belong to
	prefix string                  // CLI: the verb so far plus a space; routes: the pattern
	args   map[types.Object]bool   // CLI: []string values whose [0] is the key
	keys   map[types.Object]string // values holding the key -> the route wildcard it came from
	wilds  map[string]bool         // routes: wildcards whose PathValue is a key
}

func newKeyScan(prog *Program, cli bool) *keyScan {
	return &keyScan{prog: prog, cli: cli, arms: map[string][]ast.Node{}, parent: map[string]string{}, visited: map[string]bool{}}
}

func (s *keyScan) entries(surface string) []Entry {
	names := make([]string, 0, len(s.arms))
	for n := range s.arms {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]Entry, 0, len(names))
	for _, n := range names {
		var roots []Root
		for _, a := range s.arms[n] {
			roots = append(roots, Root{Arm: a})
		}
		out = append(out, Entry{Surface: surface, Name: n, Roots: roots})
	}
	return out
}

func (s *keyScan) scanFunc(fn *types.Func, ctx scanCtx) {
	d, ok := s.prog.decls[fn.Origin()]
	if !ok || d.decl.Body == nil {
		return
	}
	ctx.pkg = d.pkg
	s.scanNode(d.decl.Body, ctx)
}

func (s *keyScan) scanNode(body ast.Node, ctx scanCtx) {
	memo := fmt.Sprintf("%d|%s|%s|%s|%s", body.Pos(), ctx.parent, ctx.prefix, objList(ctx.args), keyList(ctx.keys))
	if s.visited[memo] {
		return
	}
	s.visited[memo] = true
	ctx.keys = s.collectKeyVars(body, ctx)
	s.visit(body, ctx, nil)
}

// collectKeyVars adds every variable assigned a key expression.
func (s *keyScan) collectKeyVars(body ast.Node, ctx scanCtx) map[types.Object]string {
	keys := map[types.Object]string{}
	for k, v := range ctx.keys {
		keys[k] = v
	}
	ctx.keys = keys
	for changed := true; changed; {
		changed = false
		add := func(lhs *ast.Ident, rhs ast.Expr) {
			obj := ctx.pkg.Info.Defs[lhs]
			if obj == nil {
				obj = ctx.pkg.Info.Uses[lhs]
			}
			if obj == nil {
				return
			}
			if _, done := keys[obj]; done {
				return
			}
			if w, ok := s.keyOf(ctx, rhs); ok {
				keys[obj] = w
				changed = true
			}
		}
		ast.Inspect(body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				if len(x.Lhs) == len(x.Rhs) {
					for i := range x.Lhs {
						if id, ok := x.Lhs[i].(*ast.Ident); ok {
							add(id, x.Rhs[i])
						}
					}
				}
			case *ast.ValueSpec:
				if len(x.Names) == len(x.Values) {
					for i := range x.Names {
						add(x.Names[i], x.Values[i])
					}
				}
			}
			return true
		})
	}
	return keys
}

// visit walks n for arms and for calls that carry the key or the args
// onward. arms names the arms n sits inside at this level.
func (s *keyScan) visit(n ast.Node, ctx scanCtx, arms []string) {
	ast.Inspect(n, func(x ast.Node) bool {
		switch st := x.(type) {
		case *ast.SwitchStmt:
			w, isKey := "", false
			if st.Tag != nil {
				w, isKey = s.keyOf(ctx, st.Tag)
			}
			if !isKey {
				return true
			}
			if st.Init != nil {
				s.visit(st.Init, ctx, arms)
			}
			for _, cs := range st.Body.List {
				cc := cs.(*ast.CaseClause)
				lits, ok := stringConsts(ctx.pkg, cc.List)
				if !ok || len(lits) == 0 {
					for _, b := range cc.Body {
						s.visit(b, ctx, arms)
					}
					continue
				}
				names := s.armNames(ctx, w, lits)
				for _, name := range names {
					s.arms[name] = appendNode(s.arms[name], cc)
					s.parent[name] = ctx.parent
				}
				for _, b := range cc.Body {
					s.visit(b, ctx, names)
				}
			}
			return false
		case *ast.IfStmt:
			if !s.cli {
				return true
			}
			w, lits := s.keyLits(ctx, st.Cond)
			if len(lits) == 0 {
				return true
			}
			if st.Init != nil {
				s.visit(st.Init, ctx, arms)
			}
			s.visit(st.Cond, ctx, arms)
			names := s.armNames(ctx, w, lits)
			for _, name := range names {
				s.arms[name] = appendNode(s.arms[name], st.Body)
				s.parent[name] = ctx.parent
			}
			s.visit(st.Body, ctx, names)
			if st.Else != nil {
				s.visit(st.Else, ctx, arms)
			}
			return false
		case *ast.CallExpr:
			s.followCall(ctx, st, arms)
		}
		return true
	})
}

// followCall continues the scan into a statically called module function
// that receives the key, the args, or (inside an arm) the next level's args.
func (s *keyScan) followCall(ctx scanCtx, call *ast.CallExpr, arms []string) {
	callee := s.prog.staticCallee(ctx.pkg, call)
	if callee == nil {
		return
	}
	params := callee.Type().(*types.Signature).Params()
	if params.Len() == 0 {
		return
	}
	for i, arg := range call.Args {
		pi := i
		if pi >= params.Len() {
			pi = params.Len() - 1
		}
		param := params.At(pi)
		if w, ok := s.keyOf(ctx, arg); ok {
			s.scanFunc(callee, scanCtx{parent: ctx.parent, prefix: ctx.prefix, args: map[types.Object]bool{}, keys: map[types.Object]string{param: w}, wilds: ctx.wilds})
			continue
		}
		if !s.cli {
			continue
		}
		if id, ok := unparen(arg).(*ast.Ident); ok && ctx.args[ctx.pkg.Info.Uses[id]] {
			s.scanFunc(callee, scanCtx{parent: ctx.parent, prefix: ctx.prefix, args: map[types.Object]bool{param: true}, keys: map[types.Object]string{}})
			continue
		}
		if s.isRestOfArgs(ctx, arg) {
			for _, a := range arms {
				s.scanFunc(callee, scanCtx{parent: a, prefix: a + " ", args: map[types.Object]bool{param: true}, keys: map[types.Object]string{}})
			}
		}
	}
}

// armNames names the arms one clause or if-arm selects.
func (s *keyScan) armNames(ctx scanCtx, wildcard string, lits []string) []string {
	var out []string
	for _, l := range lits {
		if l == "" {
			continue
		}
		if s.cli {
			out = append(out, ctx.prefix+l)
		} else {
			out = append(out, strings.Replace(ctx.prefix, "{"+wildcard+"}", l, 1))
		}
	}
	return out
}

// keyOf reports whether e is the level's key, and the wildcard it came from.
func (s *keyScan) keyOf(ctx scanCtx, e ast.Expr) (string, bool) {
	switch x := unparen(e).(type) {
	case *ast.Ident:
		obj := ctx.pkg.Info.Uses[x]
		if obj == nil {
			obj = ctx.pkg.Info.Defs[x]
		}
		w, ok := ctx.keys[obj]
		return w, ok
	case *ast.IndexExpr:
		id, ok := unparen(x.X).(*ast.Ident)
		if !ok || !ctx.args[ctx.pkg.Info.Uses[id]] {
			return "", false
		}
		v := ctx.pkg.Info.Types[x.Index].Value
		return "", v != nil && v.Kind() == constant.Int && v.ExactString() == "0"
	case *ast.CallExpr:
		sel, ok := unparen(x.Fun).(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "PathValue" || len(x.Args) != 1 {
			return "", false
		}
		if t := ctx.pkg.Info.TypeOf(sel.X); t == nil || t.String() != "*net/http.Request" {
			return "", false
		}
		v := ctx.pkg.Info.Types[x.Args[0]].Value
		if v == nil || v.Kind() != constant.String || !ctx.wilds[constant.StringVal(v)] {
			return "", false
		}
		return constant.StringVal(v), true
	}
	return "", false
}

// keyLits returns the literals an if-condition compares the key to: k ==
// "lit", a disjunction of such comparisons, or one conjoined with guards.
func (s *keyScan) keyLits(ctx scanCtx, e ast.Expr) (string, []string) {
	b, ok := unparen(e).(*ast.BinaryExpr)
	if !ok {
		return "", nil
	}
	switch b.Op {
	case token.EQL:
		for _, pair := range [][2]ast.Expr{{b.X, b.Y}, {b.Y, b.X}} {
			if w, ok := s.keyOf(ctx, pair[0]); ok {
				if v := ctx.pkg.Info.Types[pair[1]].Value; v != nil && v.Kind() == constant.String {
					return w, []string{constant.StringVal(v)}
				}
			}
		}
	case token.LOR:
		wl, l := s.keyLits(ctx, b.X)
		wr, r := s.keyLits(ctx, b.Y)
		if len(l) > 0 && len(r) > 0 && wl == wr {
			return wl, append(l, r...)
		}
	case token.LAND:
		if w, l := s.keyLits(ctx, b.X); len(l) > 0 {
			return w, l
		}
		return s.keyLits(ctx, b.Y)
	}
	return "", nil
}

// isRestOfArgs reports whether e is args[1:] for an args of this level.
func (s *keyScan) isRestOfArgs(ctx scanCtx, e ast.Expr) bool {
	sl, ok := unparen(e).(*ast.SliceExpr)
	if !ok || sl.High != nil || sl.Low == nil {
		return false
	}
	id, ok := unparen(sl.X).(*ast.Ident)
	if !ok || !ctx.args[ctx.pkg.Info.Uses[id]] {
		return false
	}
	v := ctx.pkg.Info.Types[sl.Low].Value
	return v != nil && v.Kind() == constant.Int && v.ExactString() == "1"
}

// --- small helpers -------------------------------------------------------

// funcValue resolves e, used as a value, to the module function it names.
func (p *Program) funcValue(pkg *Package, e ast.Expr) *types.Func {
	var id *ast.Ident
	switch x := unparen(e).(type) {
	case *ast.Ident:
		id = x
	case *ast.SelectorExpr:
		id = x.Sel
	default:
		return nil
	}
	fn, ok := pkg.Info.Uses[id].(*types.Func)
	if !ok || isAbstract(fn) || p.byPath[fn.Pkg().Path()] == nil {
		return nil
	}
	return fn.Origin()
}

// staticCallee returns the module function call calls by name, or nil.
func (p *Program) staticCallee(pkg *Package, call *ast.CallExpr) *types.Func {
	if call == nil {
		return nil
	}
	fun := unparen(call.Fun)
	if ix, ok := fun.(*ast.IndexExpr); ok {
		fun = ix.X
	}
	return p.funcValue(pkg, fun)
}

func lookupFunc(prog *Program, pkgPath, name string) (*types.Func, error) {
	pkg := prog.Package(pkgPath)
	if pkg == nil {
		return nil, fmt.Errorf("reach: no module package %s", pkgPath)
	}
	fn, ok := pkg.Types.Scope().Lookup(name).(*types.Func)
	if !ok {
		return nil, fmt.Errorf("reach: %s declares no function %s", pkgPath, name)
	}
	return fn, nil
}

// stringConsts returns the string constants of a case list, and false when
// any expression in it is not one.
func stringConsts(pkg *Package, list []ast.Expr) ([]string, bool) {
	var out []string
	for _, e := range list {
		v := pkg.Info.Types[e].Value
		if v == nil || v.Kind() != constant.String {
			return nil, false
		}
		out = append(out, constant.StringVal(v))
	}
	return out, true
}

func isStringSlice(t types.Type) bool {
	s, ok := t.Underlying().(*types.Slice)
	if !ok {
		return false
	}
	b, ok := s.Elem().Underlying().(*types.Basic)
	return ok && b.Kind() == types.String
}

func unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

func appendNode(list []ast.Node, n ast.Node) []ast.Node {
	for _, x := range list {
		if x == n {
			return list
		}
	}
	return append(list, n)
}

func objList(m map[types.Object]bool) string {
	var out []string
	for o := range m {
		out = append(out, fmt.Sprintf("%d", o.Pos()))
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

func keyList(m map[types.Object]string) string {
	var out []string
	for o, w := range m {
		out = append(out, fmt.Sprintf("%d=%s", o.Pos(), w))
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}
