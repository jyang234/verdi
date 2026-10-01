package reach

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"
)

// Root names where an entry's code starts: a declared function or method,
// a function literal, a dispatch arm (an *ast.CaseClause, or the body
// *ast.BlockStmt of an if-arm), or a package-level variable whose
// initializer runs at initialization. Exactly one field is set.
type Root struct {
	Func *types.Func
	Lit  *ast.FuncLit
	Arm  ast.Node
	Var  *types.Var
}

// Entry is one verb: the surface it is reached from, its name there, and
// the roots its code starts at.
type Entry struct {
	Surface string
	Name    string
	Roots   []Root
	// Parent names the entry on the same surface whose dispatch derives
	// this one: a CLI subcommand's verb (PreDispatch for a top-level verb),
	// a route action's route. Empty when nothing on the surface dispatches
	// it. Traversal stops at the roots of an entry's own descendants, whose
	// mutations are theirs (ledger SI-314 (2)).
	Parent string
	// Site is the code that dispatches the entry from a host: the function
	// or function literal holding a route's ServeMux registration (a route
	// action carries its route's) or the MCP tool switch. Zero for a verb no
	// host serves. A verb of another surface whose code reaches the site
	// serves the entry. An entry with a site (its own or an ancestor's)
	// fails closed, in Reach, on a read of a function-typed field the
	// analysis cannot resolve: its collaborators were built by code outside
	// its own reach.
	Site Root
	// Bound names the route table's function-typed fields whose row values
	// Roots already holds, because the registration handed the row itself
	// to the handler it calls (internal/workbench's /b/{branch} mount).
	Bound []*types.Var
}

// Hit is one target an entry reaches, with one call path from a root of
// the entry to the target (node names, root first).
type Hit struct {
	Func *types.Func
	Path []string
}

// Graph is the module's call graph. Its nodes are the module's declared
// functions and methods, its function literals, its package-level
// variables that have an initializer, and the dispatch arms of the entries
// it was built for. An edge from a node to a function exists when the
// node's code uses the function at all: a call, a function value, a method
// value or expression, or a reference to a package-level variable whose
// initializer uses it. A function literal is an edge from the code that
// writes it, and a dispatch arm an edge from the code that contains it.
// Interface methods resolve to every module type that implements the
// interface (class-hierarchy analysis). A module type converted to an
// interface declared outside the module adds edges to the methods that
// interface names, because only the standard library (or another
// dependency) can call them.
type Graph struct {
	prog  *Program
	nodes []*node

	byFunc map[*types.Func]int
	byLit  map[*ast.FuncLit]int
	byArm  map[ast.Node]int
	byVar  map[*types.Var]int

	named []*types.Named
	impls map[*types.Func][]*types.Func

	// flow and fieldVals resolve reads of function-typed struct fields
	// (fields.go); bound is every route-table field some entry binds, whose
	// reads resolve per entry instead; stores is every selector an
	// assignment writes, which is not a read.
	flow      *fieldFlow
	fieldVals map[*types.Var]fieldValue
	bound     map[*types.Var]bool
	stores    map[ast.Expr]bool

	// entries are the entries the graph was built for, by surface and
	// name; children lists each entry's direct descendants (its Parent's
	// inverse); roots are each entry's root nodes.
	entries  []Entry
	byEntry  map[entryKey]int
	children map[int][]int
	roots    [][]int
}

type entryKey struct{ surface, name string }

type node struct {
	name  string
	fn    *types.Func
	out   map[int]bool
	reads []fieldRead
}

// fieldRead is one read of a function-typed struct field in a node's code.
type fieldRead struct {
	field *types.Var
	pos   token.Pos
}

// Build builds the graph for prog, with every root of entries as a node.
// The entries are the ones Reach may be asked about: their Parent and Site
// relations decide where each one's traversal stops, so each surface and
// name may appear once.
func Build(prog *Program, entries []Entry) (*Graph, error) {
	g := &Graph{
		prog:   prog,
		byFunc: map[*types.Func]int{},
		byLit:  map[*ast.FuncLit]int{},
		byArm:  map[ast.Node]int{},
		byVar:  map[*types.Var]int{},
		impls:  map[*types.Func][]*types.Func{},

		byEntry:  map[entryKey]int{},
		children: map[int][]int{},

		fieldVals: map[*types.Var]fieldValue{},
		bound:     map[*types.Var]bool{},
		stores:    map[ast.Expr]bool{},
	}
	for i, e := range entries {
		if len(e.Roots) == 0 {
			return nil, fmt.Errorf("reach: entry %s %q has no roots", e.Surface, e.Name)
		}
		key := entryKey{e.Surface, e.Name}
		if _, dup := g.byEntry[key]; dup {
			return nil, fmt.Errorf("reach: entry %s %q is given twice", e.Surface, e.Name)
		}
		g.byEntry[key] = i
		for _, r := range e.Roots {
			if r.Arm != nil {
				switch r.Arm.(type) {
				case *ast.CaseClause, *ast.BlockStmt:
				default:
					return nil, fmt.Errorf("reach: entry %s %q: an arm root must be a case clause or an if-arm body, got %T", e.Surface, e.Name, r.Arm)
				}
				if _, ok := g.byArm[r.Arm]; !ok {
					g.byArm[r.Arm] = g.add(&node{name: "arm@" + g.position(r.Arm.Pos())})
				}
			}
		}
	}
	for _, e := range entries {
		for _, f := range e.Bound {
			g.bound[f.Origin()] = true
		}
	}
	g.declare()
	g.flow = newFieldFlow(prog)
	g.connect()
	g.entries = entries
	for i, e := range entries {
		ids, err := g.rootIDs(e)
		if err != nil {
			return nil, err
		}
		g.roots = append(g.roots, ids)
		if e.Parent == "" {
			continue
		}
		if p, ok := g.byEntry[entryKey{e.Surface, e.Parent}]; ok && p != i {
			g.children[p] = append(g.children[p], i)
		}
	}
	for i := range entries {
		if _, err := g.siteNode(i); err != nil {
			return nil, err
		}
	}
	return g, nil
}

// Reach returns the targets entry reaches, sorted by name, each with one
// shortest path. Traversal stops only at the roots of entries whose
// mutations are their own (ledger SI-314 (2)): the entry's descendants (a
// dispatcher's subcommands, a route's actions), and the entries of another
// surface it serves, which are those whose site its code reaches (serve's
// workbench routes; mcp's tools). Every other call, into another verb's
// code included, is traversed, so a verb that delegates reaches what its
// delegate reaches. The entry must be one the graph was built for; an
// entry the graph does not know is an error, never one that reaches
// nothing.
func (g *Graph) Reach(entry Entry, targets map[*types.Func]bool) ([]Hit, error) {
	idx, ok := g.byEntry[entryKey{entry.Surface, entry.Name}]
	if !ok {
		if _, err := g.rootIDs(entry); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("reach: entry %s %q is not one the graph was built for", entry.Surface, entry.Name)
	}
	entry = g.entries[idx]
	cut := map[int]bool{}
	for _, d := range g.descendants(idx) {
		g.cutAt(cut, d)
	}
	visited, _, err := g.traverse(idx, cut, nil, nil)
	if err != nil {
		return nil, err
	}
	for j, e := range g.entries {
		if e.Surface == entry.Surface {
			continue
		}
		site, _ := g.siteNode(j)
		if site < 0 || !visited[site] {
			continue
		}
		g.cutAt(cut, j)
		for _, d := range g.descendants(j) {
			g.cutAt(cut, d)
		}
	}
	_, hits, err := g.traverse(idx, cut, targets, g.readChecker(idx))
	return hits, err
}

// traverse walks breadth-first from entry idx's roots, never entering a
// cut node that is not one of its own roots, and returns the nodes it
// visits and the targets among them. check, when set, vets each visited
// node.
func (g *Graph) traverse(idx int, cut map[int]bool, targets map[*types.Func]bool, check func(*node) error) (map[int]bool, []Hit, error) {
	own := map[int]bool{}
	parent := map[int]int{}
	var queue []int
	for _, id := range g.roots[idx] {
		own[id] = true
		if _, seen := parent[id]; !seen {
			parent[id] = -1
			queue = append(queue, id)
		}
	}
	var hits []Hit
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		n := g.nodes[id]
		if n.fn != nil && targets[n.fn] {
			hits = append(hits, Hit{Func: n.fn, Path: g.path(parent, id)})
		}
		if check != nil {
			if err := check(n); err != nil {
				return nil, nil, err
			}
		}
		for _, next := range sortedKeys(n.out) {
			if _, seen := parent[next]; seen {
				continue
			}
			if cut[next] && !own[next] {
				continue
			}
			parent[next] = id
			queue = append(queue, next)
		}
	}
	visited := make(map[int]bool, len(parent))
	for id := range parent {
		visited[id] = true
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Func.FullName() < hits[j].Func.FullName() })
	return visited, hits, nil
}

// cutAt adds entry i's roots to cut.
func (g *Graph) cutAt(cut map[int]bool, i int) {
	for _, id := range g.roots[i] {
		cut[id] = true
	}
}

// descendants returns every entry whose Parent chain leads to entry i.
func (g *Graph) descendants(i int) []int {
	var out []int
	seen := map[int]bool{i: true}
	stack := append([]int(nil), g.children[i]...)
	for len(stack) > 0 {
		c := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
		stack = append(stack, g.children[c]...)
	}
	sort.Ints(out)
	return out
}

// siteNode returns the node of entry i's site (its own, else its nearest
// ancestor's), or -1 when it has none.
func (g *Graph) siteNode(i int) (int, error) {
	seen := map[int]bool{}
	for i >= 0 && !seen[i] {
		seen[i] = true
		e := g.entries[i]
		if e.Site != (Root{}) {
			ids, err := g.rootIDs(Entry{Surface: e.Surface, Name: e.Name, Roots: []Root{e.Site}})
			if err != nil {
				return -1, fmt.Errorf("reach: entry %s %q: its site: %w", e.Surface, e.Name, err)
			}
			return ids[0], nil
		}
		p, ok := g.byEntry[entryKey{e.Surface, e.Parent}]
		if e.Parent == "" || !ok {
			return -1, nil
		}
		i = p
	}
	return -1, nil
}

// readChecker returns the fail-closed check for entry i's traversal: an
// entry a host dispatches (one with a site) must resolve every
// function-typed field it reads; any other entry needs none.
func (g *Graph) readChecker(i int) func(*node) error {
	if site, _ := g.siteNode(i); site < 0 {
		return nil
	}
	entry := g.entries[i]
	boundHere := map[*types.Var]bool{}
	for _, f := range entry.Bound {
		boundHere[f.Origin()] = true
	}
	return func(n *node) error { return g.checkReads(entry, n, boundHere) }
}

// checkReads fails closed on a hosted entry's read of a function-typed
// field whose values this entry cannot know: a route table's field its
// registration did not bind, or a field holding a value the field flow
// cannot follow.
func (g *Graph) checkReads(entry Entry, n *node, boundHere map[*types.Var]bool) error {
	for _, rd := range n.reads {
		name := g.fieldName(rd.field)
		switch {
		case boundHere[rd.field]:
		case g.bound[rd.field]:
			return fmt.Errorf("reach: entry %s %q: %s reads route-table field %s outside the registration that binds it, so its value is unknown here (fail closed)",
				entry.Surface, entry.Name, g.prog.Fset.Position(rd.pos), name)
		default:
			if v := g.fieldValues(rd.field); v.opaque {
				return fmt.Errorf("reach: entry %s %q: %s reads function-typed field %s, but a value stored in it (%s) cannot be followed statically (fail closed)",
					entry.Surface, entry.Name, g.prog.Fset.Position(rd.pos), name, g.prog.Fset.Position(v.opaqueAt))
			}
		}
	}
	return nil
}

func (g *Graph) fieldName(f *types.Var) string {
	return strings.TrimPrefix(f.Pkg().Path(), g.prog.Module+"/") + "." + f.Name()
}

// FuncName returns fn's full name with the module path prefix trimmed,
// e.g. "internal/gitx.AddAll" or "(*internal/x.T).M".
func (p *Program) FuncName(fn *types.Func) string {
	return strings.ReplaceAll(fn.FullName(), p.Module+"/", "")
}

func (g *Graph) path(parent map[int]int, id int) []string {
	var rev []string
	for cur := id; cur != -1; cur = parent[cur] {
		rev = append(rev, g.nodes[cur].name)
	}
	out := make([]string, len(rev))
	for i := range rev {
		out[i] = rev[len(rev)-1-i]
	}
	return out
}

func (g *Graph) rootIDs(e Entry) ([]int, error) {
	if len(e.Roots) == 0 {
		return nil, fmt.Errorf("reach: entry %s %q has no roots", e.Surface, e.Name)
	}
	var ids []int
	for _, r := range e.Roots {
		var (
			id int
			ok bool
		)
		switch {
		case r.Func != nil:
			id, ok = g.byFunc[r.Func.Origin()]
		case r.Lit != nil:
			id, ok = g.byLit[r.Lit]
		case r.Arm != nil:
			id, ok = g.byArm[r.Arm]
		case r.Var != nil:
			id, ok = g.byVar[r.Var]
		default:
			return nil, fmt.Errorf("reach: entry %s %q has an empty root", e.Surface, e.Name)
		}
		if !ok {
			return nil, fmt.Errorf("reach: entry %s %q has a root the module's source does not declare", e.Surface, e.Name)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (g *Graph) add(n *node) int {
	n.out = map[int]bool{}
	g.nodes = append(g.nodes, n)
	return len(g.nodes) - 1
}

func (g *Graph) position(pos token.Pos) string {
	p := g.prog.Fset.Position(pos)
	return fmt.Sprintf("%s:%d", filepath.Base(p.Filename), p.Line)
}

// declare creates a node for every function, literal, and initialized
// package-level variable, and collects the module's named types.
func (g *Graph) declare() {
	for _, pkg := range g.prog.packages {
		for _, f := range pkg.Files {
			for _, decl := range f.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					if fn, ok := pkg.Info.Defs[d.Name].(*types.Func); ok {
						g.byFunc[fn] = g.add(&node{name: g.prog.FuncName(fn), fn: fn})
					}
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						vs, ok := spec.(*ast.ValueSpec)
						if !ok || len(vs.Values) == 0 {
							continue
						}
						for _, id := range vs.Names {
							if v, ok := pkg.Info.Defs[id].(*types.Var); ok {
								g.byVar[v] = g.add(&node{name: "var " + strings.TrimPrefix(v.Pkg().Path(), g.prog.Module+"/") + "." + v.Name()})
							}
						}
					}
				}
			}
			ast.Inspect(f, func(n ast.Node) bool {
				if lit, ok := n.(*ast.FuncLit); ok {
					g.byLit[lit] = g.add(&node{name: "func@" + g.position(lit.Pos())})
				}
				return true
			})
		}
		for _, obj := range pkg.Info.Defs {
			tn, ok := obj.(*types.TypeName)
			if !ok || tn.IsAlias() {
				continue
			}
			named, ok := tn.Type().(*types.Named)
			if !ok || types.IsInterface(named) || named.TypeParams().Len() > 0 {
				continue
			}
			g.named = append(g.named, named)
		}
	}
	sort.Slice(g.named, func(i, j int) bool {
		return g.named[i].Obj().Pos() < g.named[j].Obj().Pos()
	})
}

// connect adds every node's outgoing edges.
func (g *Graph) connect() {
	for _, pkg := range g.prog.packages {
		for _, f := range pkg.Files {
			for _, decl := range f.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					fn, ok := pkg.Info.Defs[d.Name].(*types.Func)
					if !ok || d.Body == nil {
						continue
					}
					g.walk(pkg, g.byFunc[fn], d.Body, fn.Type().(*types.Signature))
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						vs, ok := spec.(*ast.ValueSpec)
						if !ok || len(vs.Values) == 0 {
							continue
						}
						for _, id := range vs.Names {
							v, ok := pkg.Info.Defs[id].(*types.Var)
							if !ok {
								continue
							}
							for _, val := range vs.Values {
								g.walk(pkg, g.byVar[v], val, nil)
							}
						}
					}
				}
			}
		}
	}
}

// walk adds the edges of the code under root to node from. sig is the
// signature whose results a return statement in that code converts to.
func (g *Graph) walk(pkg *Package, from int, root ast.Node, sig *types.Signature) {
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		if n != root {
			if id, ok := g.byArm[n]; ok {
				g.edge(from, id)
				g.walk(pkg, id, n, sig)
				return false
			}
		}
		switch x := n.(type) {
		case *ast.FuncLit:
			if n == root {
				return true
			}
			id := g.byLit[x]
			g.edge(from, id)
			litSig, _ := pkg.Info.TypeOf(x).(*types.Signature)
			g.walk(pkg, id, x.Body, litSig)
			return false
		case *ast.Ident:
			g.use(from, pkg.Info.Uses[x])
		case *ast.ReturnStmt:
			if sig != nil && sig.Results().Len() == len(x.Results) {
				for i, res := range x.Results {
					g.convert(from, pkg, res, sig.Results().At(i).Type())
				}
			}
		case *ast.SelectorExpr:
			if g.stores[x] {
				break
			}
			if field := funcFieldOf(pkg, x); field != nil {
				g.nodes[from].reads = append(g.nodes[from].reads, fieldRead{field: field, pos: x.Pos()})
				if !g.bound[field] {
					for _, id := range g.fieldValues(field).ids {
						g.edge(from, id)
					}
				}
			}
		case *ast.AssignStmt:
			for _, lhs := range x.Lhs {
				if sel, ok := unparen(lhs).(*ast.SelectorExpr); ok {
					g.stores[sel] = true
				}
			}
			if len(x.Lhs) == len(x.Rhs) {
				for i := range x.Lhs {
					g.convert(from, pkg, x.Rhs[i], pkg.Info.TypeOf(x.Lhs[i]))
				}
			}
		case *ast.ValueSpec:
			if x.Type != nil && len(x.Names) == len(x.Values) {
				for _, val := range x.Values {
					g.convert(from, pkg, val, pkg.Info.TypeOf(x.Type))
				}
			}
		case *ast.CallExpr:
			g.convertCall(from, pkg, x)
		case *ast.CompositeLit:
			g.convertComposite(from, pkg, x)
		case *ast.SendStmt:
			if ch, ok := pkg.Info.TypeOf(x.Chan).Underlying().(*types.Chan); ok {
				g.convert(from, pkg, x.Value, ch.Elem())
			}
		}
		return true
	})
}

// use adds the edge a use of obj implies.
func (g *Graph) use(from int, obj types.Object) {
	switch o := obj.(type) {
	case *types.Func:
		if !g.inModule(o.Pkg()) {
			// A method of a dependency's interface still dispatches to
			// module implementations.
			if isAbstract(o) {
				for _, impl := range g.implementations(o) {
					g.edgeFunc(from, impl)
				}
			}
			return
		}
		if isAbstract(o) {
			for _, impl := range g.implementations(o) {
				g.edgeFunc(from, impl)
			}
			return
		}
		g.edgeFunc(from, o.Origin())
	case *types.Var:
		if id, ok := g.byVar[o]; ok {
			g.edge(from, id)
		}
	}
}

func (g *Graph) edgeFunc(from int, fn *types.Func) {
	if id, ok := g.byFunc[fn.Origin()]; ok {
		g.edge(from, id)
	}
}

func (g *Graph) edge(from, to int) { g.nodes[from].out[to] = true }

// convertCall treats each argument as converted to its parameter's type.
func (g *Graph) convertCall(from int, pkg *Package, call *ast.CallExpr) {
	tv, ok := pkg.Info.Types[call.Fun]
	if !ok {
		return
	}
	if tv.IsType() {
		if len(call.Args) == 1 {
			g.convert(from, pkg, call.Args[0], tv.Type)
		}
		return
	}
	sig, ok := tv.Type.Underlying().(*types.Signature)
	if !ok {
		return
	}
	params := sig.Params()
	for i, arg := range call.Args {
		var pt types.Type
		switch {
		case sig.Variadic() && i >= params.Len()-1:
			last := params.At(params.Len() - 1).Type()
			if call.Ellipsis.IsValid() {
				pt = last
			} else if s, ok := last.(*types.Slice); ok {
				pt = s.Elem()
			}
		case i < params.Len():
			pt = params.At(i).Type()
		}
		if pt != nil {
			g.convert(from, pkg, arg, pt)
		}
	}
}

// convertComposite treats each element of a composite literal as converted
// to its field or element type.
func (g *Graph) convertComposite(from int, pkg *Package, lit *ast.CompositeLit) {
	t := pkg.Info.TypeOf(lit)
	if t == nil {
		return
	}
	if p, ok := t.Underlying().(*types.Pointer); ok {
		t = p.Elem()
	}
	switch u := t.Underlying().(type) {
	case *types.Struct:
		for i, elt := range lit.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				if key, ok := kv.Key.(*ast.Ident); ok {
					for j := 0; j < u.NumFields(); j++ {
						if u.Field(j).Name() == key.Name {
							g.convert(from, pkg, kv.Value, u.Field(j).Type())
						}
					}
				}
				continue
			}
			if i < u.NumFields() {
				g.convert(from, pkg, elt, u.Field(i).Type())
			}
		}
	case *types.Slice:
		g.convertElems(from, pkg, lit, u.Elem())
	case *types.Array:
		g.convertElems(from, pkg, lit, u.Elem())
	case *types.Map:
		g.convertElems(from, pkg, lit, u.Elem())
	}
}

func (g *Graph) convertElems(from int, pkg *Package, lit *ast.CompositeLit, elem types.Type) {
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			elt = kv.Value
		}
		g.convert(from, pkg, elt, elem)
	}
}

// convert adds edges when expr, of a concrete module type, is converted to
// an interface declared outside the module: the dependency that receives
// the value may call any method that interface names.
func (g *Graph) convert(from int, pkg *Package, expr ast.Expr, target types.Type) {
	if target == nil {
		return
	}
	iface, ok := target.Underlying().(*types.Interface)
	if !ok || iface.NumMethods() == 0 {
		return
	}
	if named, ok := target.(*types.Named); ok && g.inModule(named.Obj().Pkg()) {
		return
	}
	src := pkg.Info.TypeOf(expr)
	if src == nil || types.IsInterface(src) || !g.isModuleType(src) {
		return
	}
	for i := 0; i < iface.NumMethods(); i++ {
		m := iface.Method(i)
		obj, _, _ := types.LookupFieldOrMethod(src, true, m.Pkg(), m.Name())
		if fn, ok := obj.(*types.Func); ok && g.inModule(fn.Pkg()) {
			g.edgeFunc(from, fn)
		}
	}
}

func (g *Graph) isModuleType(t types.Type) bool {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	named, ok := t.(*types.Named)
	return ok && g.inModule(named.Obj().Pkg())
}

// implementations returns every module method that an interface method
// call to m may dispatch to.
func (g *Graph) implementations(m *types.Func) []*types.Func {
	if impls, ok := g.impls[m]; ok {
		return impls
	}
	sig := m.Type().(*types.Signature)
	iface, _ := sig.Recv().Type().Underlying().(*types.Interface)
	var out []*types.Func
	if iface != nil {
		for _, named := range g.named {
			for _, t := range []types.Type{named, types.NewPointer(named)} {
				if !types.Implements(t, iface) {
					continue
				}
				obj, _, _ := types.LookupFieldOrMethod(t, true, m.Pkg(), m.Name())
				if fn, ok := obj.(*types.Func); ok && g.inModule(fn.Pkg()) {
					out = append(out, fn.Origin())
				}
				break
			}
		}
	}
	g.impls[m] = out
	return out
}

func (g *Graph) inModule(pkg *types.Package) bool {
	return pkg != nil && g.prog.byPath[pkg.Path()] != nil
}

// isAbstract reports whether fn is an interface method.
func isAbstract(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}
	return types.IsInterface(sig.Recv().Type())
}

func sortedKeys(m map[int]bool) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}
