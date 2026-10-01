package reach

import (
	"fmt"
	"go/types"
	"sort"
	"strings"
)

// ExportedFuncs returns the names (ObjectName form) of everything in
// pkgPath that a caller outside the package can run code through, sorted:
// its exported functions; the exported methods of every named type it
// declares, exported or not (a caller can hold an unexported type's value
// through an exported function or interface), which covers an exported
// alias of one of its own types; the methods of its interfaces; and its
// exported package-level variables of function type. An exported alias of
// a type declared elsewhere names methods that are not the package's
// functions, so it is an error rather than a silent omission.
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
				out = append(out, prog.ObjectName(obj))
			}
		case *types.Var:
			if _, isFunc := obj.Type().Underlying().(*types.Signature); isFunc && obj.Exported() {
				out = append(out, prog.ObjectName(obj))
			}
		case *types.TypeName:
			if obj.IsAlias() {
				if err := checkAlias(pkg, obj); err != nil {
					return nil, err
				}
				continue
			}
			named, ok := obj.Type().(*types.Named)
			if !ok {
				continue
			}
			if iface, ok := named.Underlying().(*types.Interface); ok {
				for i := 0; i < iface.NumExplicitMethods(); i++ {
					if m := iface.ExplicitMethod(i); m.Exported() {
						out = append(out, prog.ObjectName(m))
					}
				}
				continue
			}
			for i := 0; i < named.NumMethods(); i++ {
				if m := named.Method(i); m.Exported() {
					out = append(out, prog.ObjectName(m))
				}
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// checkAlias accepts an exported alias only of a type the package itself
// declares (whose methods the census lists under that type) or of a type
// with no methods.
func checkAlias(pkg *Package, tn *types.TypeName) error {
	if !tn.Exported() {
		return nil
	}
	t := types.Unalias(tn.Type())
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj().Pkg() == pkg.Types {
		return nil
	}
	if named.NumMethods() == 0 {
		if _, isIface := named.Underlying().(*types.Interface); !isIface {
			return nil
		}
	}
	return fmt.Errorf("reach: %s exports %s, an alias of %s, whose methods are not this package's functions and cannot be classified here", pkg.Path, tn.Name(), named.Obj().Pkg().Path()+"."+named.Obj().Name())
}

// FuncByName returns the module function or method whose FuncName is
// name, or nil when the module declares none.
func (p *Program) FuncByName(name string) *types.Func { return p.byName[name] }

// ObjectByName returns the module object whose ObjectName is name: a
// declared function or method, an interface method, or a package-level
// variable. It returns nil when the module declares none.
func (p *Program) ObjectByName(name string) types.Object {
	if fn, ok := p.byName[name]; ok {
		return fn
	}
	if obj, ok := p.objects[name]; ok {
		return obj
	}
	return nil
}

// ObjectName returns obj's full name with the module path prefix trimmed:
// FuncName for a function or method, "pkg.Name" for a package-level
// variable.
func (p *Program) ObjectName(obj types.Object) string {
	if fn, ok := obj.(*types.Func); ok {
		return p.FuncName(fn)
	}
	if obj.Pkg() == nil {
		return obj.Name()
	}
	return strings.TrimPrefix(obj.Pkg().Path(), p.Module+"/") + "." + obj.Name()
}
