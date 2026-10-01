package reach

import (
	"fmt"
	"go/types"
	"sort"
)

// ExportedFuncs returns the names (FuncName form) of pkgPath's exported
// functions and of the exported methods of its exported types, sorted.
// An exported method of an unexported type is not part of the package's
// API and is left out.
func ExportedFuncs(prog *Program, pkgPath string) ([]string, error) {
	pkg := prog.Package(pkgPath)
	if pkg == nil {
		return nil, fmt.Errorf("reach: no module package %s", pkgPath)
	}
	var out []string
	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		switch obj := scope.Lookup(name).(type) {
		case *types.Func:
			if obj.Exported() {
				out = append(out, prog.FuncName(obj))
			}
		case *types.TypeName:
			if !obj.Exported() || obj.IsAlias() {
				continue
			}
			named, ok := obj.Type().(*types.Named)
			if !ok {
				continue
			}
			for i := 0; i < named.NumMethods(); i++ {
				if m := named.Method(i); m.Exported() {
					out = append(out, prog.FuncName(m))
				}
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// FuncByName returns the module function or method whose FuncName is
// name, or nil when the module declares none.
func (p *Program) FuncByName(name string) *types.Func { return p.byName[name] }
