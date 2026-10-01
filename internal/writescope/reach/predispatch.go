package reach

import (
	"fmt"
	"go/ast"
	"go/types"
	"sort"
)

// PreDispatch names the pseudo-entry for the code a binary runs for every
// verb outside any verb's arm (ledger SI-314 (3)): no declaration can own
// a mutation it reaches, so the witness requires that it reach none.
const PreDispatch = "(pre-dispatch)"

// PreDispatchEntry returns the pseudo-entry for the binary whose dispatcher
// is funcName in pkgPath: rooted at the package's main function (when it
// declares one), the dispatcher itself (its arms are the CLI verbs, cut as
// its descendants), and, in every module package the binary links, each
// init function and each package-level variable whose initializer runs
// code. An initializer that only names or builds a value runs nothing at
// initialization; a verb that uses the variable reaches what it names.
func PreDispatchEntry(prog *Program, pkgPath, funcName, surface string) (Entry, error) {
	fn, err := lookupFunc(prog, pkgPath, funcName)
	if err != nil {
		return Entry{}, err
	}
	roots := []Root{{Func: fn}}
	if m, ok := prog.Package(pkgPath).Types.Scope().Lookup("main").(*types.Func); ok && m != fn {
		roots = append(roots, Root{Func: m})
	}
	for _, pkg := range prog.linked(pkgPath) {
		for _, f := range pkg.Files {
			for _, d := range f.Decls {
				switch x := d.(type) {
				case *ast.FuncDecl:
					if x.Recv != nil || x.Name.Name != "init" {
						continue
					}
					obj, ok := pkg.Info.Defs[x.Name].(*types.Func)
					if !ok {
						return Entry{}, fmt.Errorf("reach: %s: an init function has no type information", prog.Fset.Position(x.Pos()))
					}
					roots = append(roots, Root{Func: obj})
				case *ast.GenDecl:
					for _, spec := range x.Specs {
						vs, ok := spec.(*ast.ValueSpec)
						if !ok || !runsCode(pkg, vs.Values) {
							continue
						}
						for _, id := range vs.Names {
							if v, ok := pkg.Info.Defs[id].(*types.Var); ok {
								roots = append(roots, Root{Var: v})
							}
						}
					}
				}
			}
		}
	}
	return Entry{Surface: surface, Name: PreDispatch, Roots: roots}, nil
}

// linked returns pkgPath and every module package it imports, directly or
// not, sorted by import path: the packages whose initialization runs in
// that package's binary.
func (p *Program) linked(pkgPath string) []*Package {
	seen := map[string]bool{}
	var walk func(path string)
	walk = func(path string) {
		pkg := p.byPath[path]
		if pkg == nil || seen[path] {
			return
		}
		seen[path] = true
		for _, imp := range pkg.Types.Imports() {
			walk(imp.Path())
		}
	}
	walk(pkgPath)
	out := make([]*Package, 0, len(seen))
	for path := range seen {
		out = append(out, p.byPath[path])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// runsCode reports whether evaluating values calls a function: a call that
// is neither a conversion nor a builtin, outside any function literal.
func runsCode(pkg *Package, values []ast.Expr) bool {
	runs := false
	for _, v := range values {
		ast.Inspect(v, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncLit:
				return false
			case *ast.CallExpr:
				tv := pkg.Info.Types[x.Fun]
				if !tv.IsType() && !tv.IsBuiltin() {
					runs = true
				}
			}
			return !runs
		})
	}
	return runs
}
