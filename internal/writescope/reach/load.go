package reach

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Target is the build configuration a module is analyzed under. Fixing it
// makes the analysis independent of the host that runs it; a caller that
// needs every platform-specific file loads once per target.
type Target struct {
	GOOS   string
	GOARCH string
}

// String renders the target as GOOS/GOARCH.
func (t Target) String() string { return t.GOOS + "/" + t.GOARCH }

// Targets returns the release platforms verdi is built for: the CI job's
// Linux and the operators' macOS. Every platform-specific file of the
// module is built for at least one of them.
func Targets() []Target {
	return []Target{{GOOS: "linux", GOARCH: "amd64"}, {GOOS: "darwin", GOARCH: "arm64"}}
}

// Program is one module's packages, parsed from source and type-checked
// with the standard library's go/parser and go/types. Only the module's
// own packages are kept, with full syntax and type information; every
// dependency is type-checked for its declarations alone.
type Program struct {
	Fset   *token.FileSet
	Module string
	Target Target

	packages []*Package
	byPath   map[string]*Package
	decls    map[*types.Func]funcDecl
	byName   map[string]*types.Func
	objects  map[string]types.Object // interface methods and package-level variables, by ObjectName
	inputs   []string

	// named, ifaces, and generics are the module's named types, sorted by
	// position: non-generic concrete types (the candidates for interface
	// dispatch), interfaces, and generic types with methods. flow is the
	// module's function-value record (fields.go).
	named    []*types.Named
	ifaces   []*types.Named
	generics []*types.Named
	flow     *fieldFlow

	// funcTypes are the named function types with methods anywhere in the
	// loaded type universe (the module and every package it imports), the
	// types through which an interface value can hold a function (ledger
	// SI-319); carries memoizes which interfaces one of them implements.
	funcTypes []*types.Named
	carries   map[*types.Interface]bool
}

// Inputs returns the directories and module files Load read in-process
// beyond the parsed sources: every module package's directory and the
// module's go.mod and go.sum (each that exists).
func (p *Program) Inputs() []string { return p.inputs }

// funcDecl is where a module function is declared.
type funcDecl struct {
	pkg  *Package
	decl *ast.FuncDecl
}

// Package is one type-checked package of the module.
type Package struct {
	Path  string
	Files []*ast.File
	Types *types.Package
	Info  *types.Info
}

// Packages returns the module's packages, sorted by import path.
func (p *Program) Packages() []*Package { return p.packages }

// Package returns the module package with import path path, or nil.
func (p *Program) Package(path string) *Package { return p.byPath[path] }

// listedPackage is the subset of `go list -json` this loader reads.
type listedPackage struct {
	ImportPath string
	Dir        string
	GoFiles    []string
	ImportMap  map[string]string
	Standard   bool
	// Module is go list's module object, whose field set grows with the go
	// command's version; only its Path and Main members are read (module).
	Module     map[string]json.RawMessage
	Error      *listedError
	DepsErrors []*listedError
}

// module returns the package's module path and whether it is the main
// module ("" and false for the standard library).
func (lp *listedPackage) module() (path string, main bool, err error) {
	if lp.Module == nil {
		return "", false, nil
	}
	if raw, ok := lp.Module["Path"]; ok {
		if err := json.Unmarshal(raw, &path); err != nil {
			return "", false, fmt.Errorf("reach: go list: package %s: module path: %w", lp.ImportPath, err)
		}
	}
	if raw, ok := lp.Module["Main"]; ok {
		if err := json.Unmarshal(raw, &main); err != nil {
			return "", false, fmt.Errorf("reach: go list: package %s: module main flag: %w", lp.ImportPath, err)
		}
	}
	return path, main, nil
}

type listedError struct {
	Err string
}

const listFields = "ImportPath,Dir,GoFiles,ImportMap,Standard,Module,Error,DepsErrors"

// Load lists patterns (and every dependency) in the module at dir for
// target with `go list`, then parses and type-checks each package from
// source. The go command runs with the proxy off, a read-only module graph,
// no workspace, and cgo disabled, so it never reaches the network: every
// dependency must already be in the module cache.
func Load(ctx context.Context, dir string, target Target, patterns ...string) (*Program, error) {
	if len(patterns) == 0 {
		return nil, errors.New("reach: Load needs at least one package pattern")
	}
	if target.GOOS == "" || target.GOARCH == "" {
		return nil, fmt.Errorf("reach: Load needs a complete target, got %q", target)
	}
	listed, err := goList(ctx, dir, target, patterns)
	if err != nil {
		return nil, err
	}
	l := &loader{
		target:   target,
		fset:     token.NewFileSet(),
		listed:   map[string]*listedPackage{},
		checked:  map[string]*types.Package{},
		module:   map[string]*Package{},
		inModule: map[string]bool{},
	}
	for _, lp := range listed {
		l.listed[lp.ImportPath] = lp
		modPath, main, err := lp.module()
		if err != nil {
			return nil, err
		}
		if main {
			l.inModule[lp.ImportPath] = true
			if l.modulePath == "" {
				l.modulePath = modPath
			}
		}
	}
	if l.modulePath == "" {
		return nil, fmt.Errorf("reach: no package of the main module matched %v in %s", patterns, dir)
	}
	inputs, err := readInputs(listed, l.inModule)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(listed))
	for _, lp := range listed {
		paths = append(paths, lp.ImportPath)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if _, err := l.check(path); err != nil {
			return nil, err
		}
	}
	prog := &Program{Fset: l.fset, Module: l.modulePath, Target: target, byPath: l.module, decls: map[*types.Func]funcDecl{}, byName: map[string]*types.Func{}, objects: map[string]types.Object{}, inputs: inputs}
	for _, path := range paths {
		pkg, ok := l.module[path]
		if !ok {
			continue
		}
		prog.packages = append(prog.packages, pkg)
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			switch obj := scope.Lookup(name).(type) {
			case *types.Var:
				prog.objects[prog.ObjectName(obj)] = obj
			case *types.TypeName:
				if iface, ok := obj.Type().Underlying().(*types.Interface); ok && !obj.IsAlias() {
					for i := 0; i < iface.NumExplicitMethods(); i++ {
						m := iface.ExplicitMethod(i)
						prog.objects[prog.ObjectName(m)] = m
					}
				}
			}
		}
		for _, f := range pkg.Files {
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok {
					if fn, ok := pkg.Info.Defs[fd.Name].(*types.Func); ok {
						prog.decls[fn] = funcDecl{pkg: pkg, decl: fd}
						prog.byName[prog.FuncName(fn)] = fn
					}
				}
			}
		}
	}
	prog.collectTypes()
	prog.collectFuncTypes(l.checked)
	prog.flow = newFieldFlow(prog)
	return prog, nil
}

// collectTypes sorts the module's named types into the candidates for
// interface dispatch, the interfaces, and the generic types with methods.
func (p *Program) collectTypes() {
	for _, pkg := range p.packages {
		for _, obj := range pkg.Info.Defs {
			tn, ok := obj.(*types.TypeName)
			if !ok || tn.IsAlias() {
				continue
			}
			named, ok := tn.Type().(*types.Named)
			if !ok {
				continue
			}
			switch {
			case types.IsInterface(named):
				p.ifaces = append(p.ifaces, named)
			case named.TypeParams().Len() > 0:
				// A generic type's full method set, promoted methods
				// included, can complete an interface (the tripwire).
				if types.NewMethodSet(types.NewPointer(named)).Len() > 0 {
					p.generics = append(p.generics, named)
				}
			default:
				p.named = append(p.named, named)
			}
		}
	}
	for _, list := range [][]*types.Named{p.named, p.ifaces, p.generics} {
		sort.Slice(list, func(i, j int) bool { return list[i].Obj().Pos() < list[j].Obj().Pos() })
	}
}

// collectFuncTypes records every named function type with a non-empty
// method set (its own or its pointer's) in the type universe: each
// type-checked package's package-level types, and the module's local
// ones. Sorted by package path, then position.
func (p *Program) collectFuncTypes(checked map[string]*types.Package) {
	p.carries = map[*types.Interface]bool{}
	seen := map[*types.Named]bool{}
	add := func(tn *types.TypeName) {
		if tn == nil || tn.IsAlias() {
			return
		}
		named, ok := tn.Type().(*types.Named)
		if !ok || seen[named] {
			return
		}
		if _, isFunc := named.Underlying().(*types.Signature); !isFunc {
			return
		}
		if types.NewMethodSet(types.NewPointer(named)).Len() == 0 {
			return
		}
		seen[named] = true
		p.funcTypes = append(p.funcTypes, named)
	}
	for _, pkg := range checked {
		if pkg == nil {
			continue
		}
		scope := pkg.Scope()
		for _, name := range scope.Names() {
			tn, _ := scope.Lookup(name).(*types.TypeName)
			add(tn)
		}
	}
	for _, pkg := range p.packages {
		for _, obj := range pkg.Info.Defs {
			tn, _ := obj.(*types.TypeName)
			add(tn)
		}
	}
	sort.Slice(p.funcTypes, func(i, j int) bool {
		a, b := p.funcTypes[i].Obj(), p.funcTypes[j].Obj()
		if a.Pkg().Path() != b.Pkg().Path() {
			return a.Pkg().Path() < b.Pkg().Path()
		}
		return a.Pos() < b.Pos()
	})
}

// ifaceCarries reports whether a value of interface type iface carries a
// function value the analysis must follow (ledger SI-319, SI-320). The
// empty interface (no methods, every type in its type set) does not: a
// value of that type reaches a call in module code only through a type
// assertion or type switch, which fails closed whenever its target type
// carries a function, and in dependency code only through reflection or
// the dependency's own assertion, the boundaries the package doc
// discloses. A method-less constraint with type terms carries one when a
// term's underlying type does (a ~func type parameter is callable). Any
// other interface carries one when a named function type with methods
// implements it (a generic one judged by method names and arities, as the
// tripwire is): a method of that type calls the function with no
// assertion.
func (p *Program) ifaceCarries(iface *types.Interface) bool {
	if v, ok := p.carries[iface]; ok {
		return v
	}
	p.carries[iface] = false // a cycle through embedded constraints carries nothing more
	if iface.Empty() {
		return false
	}
	if iface.NumMethods() == 0 {
		carries := p.termsCarry(iface)
		p.carries[iface] = carries
		return carries
	}
	carries := false
	for _, ft := range p.funcTypes {
		if ft.TypeParams().Len() > 0 {
			carries = shapeCovers(ft, iface)
		} else {
			carries = types.Implements(ft, iface) || types.Implements(types.NewPointer(ft), iface)
		}
		if carries {
			break
		}
	}
	p.carries[iface] = carries
	return carries
}

// termsCarry reports whether a method-less constraint's type terms (its
// embedded unions and constraints) include a type that carries a
// function value.
func (p *Program) termsCarry(iface *types.Interface) bool {
	for i := 0; i < iface.NumEmbeddeds(); i++ {
		switch e := iface.EmbeddedType(i).(type) {
		case *types.Union:
			for j := 0; j < e.Len(); j++ {
				if p.carriesFunc(e.Term(j).Type()) {
					return true
				}
			}
		default:
			if p.carriesFunc(e) {
				return true
			}
		}
	}
	return false
}

// shapeCovers reports whether a generic type's method set has a method of
// every name and arity iface names.
func shapeCovers(gen *types.Named, iface *types.Interface) bool {
	have := map[string]bool{}
	mset := types.NewMethodSet(types.NewPointer(gen))
	for i := 0; i < mset.Len(); i++ {
		if fn, ok := mset.At(i).Obj().(*types.Func); ok {
			have[methodShape(fn)] = true
		}
	}
	for i := 0; i < iface.NumMethods(); i++ {
		if !have[methodShape(iface.Method(i))] {
			return false
		}
	}
	return true
}

// carriesFunc reports whether a value of type t is a function value or can
// hold one where the analysis must follow it (ledger SI-319, SI-320): a
// function type; an interface a named function type with methods
// implements (a method of that type calls the function with no
// assertion); a type parameter whose constraint has a function type among
// its terms; or a pointer, slice, array, map, or channel of any of these.
// The empty interface is excluded (see ifaceCarries): a function stored in
// it surfaces only through an assertion, which fails closed on its own. A
// struct's function-typed and function-carrying interface fields resolve
// through the field flow instead.
func (p *Program) carriesFunc(t types.Type) bool {
	for depth := 0; t != nil && depth < 16; depth++ {
		switch u := t.Underlying().(type) {
		case *types.Signature:
			return true
		case *types.Interface:
			return p.ifaceCarries(u)
		case *types.Pointer:
			t = u.Elem()
		case *types.Slice:
			t = u.Elem()
		case *types.Array:
			t = u.Elem()
		case *types.Map:
			t = u.Elem()
		case *types.Chan:
			t = u.Elem()
		default:
			return false
		}
	}
	return false
}

// implementationsOf returns every module method an interface method call
// to m may dispatch to (class-hierarchy analysis over the module's
// non-generic named types).
func (p *Program) implementationsOf(m *types.Func) []*types.Func {
	sig := m.Type().(*types.Signature)
	iface, _ := sig.Recv().Type().Underlying().(*types.Interface)
	var out []*types.Func
	if iface == nil {
		return nil
	}
	for _, named := range p.named {
		for _, t := range []types.Type{named, types.NewPointer(named)} {
			if !types.Implements(t, iface) {
				continue
			}
			obj, _, _ := types.LookupFieldOrMethod(t, true, m.Pkg(), m.Name())
			if fn, ok := obj.(*types.Func); ok && fn.Pkg() != nil && p.byPath[fn.Pkg().Path()] != nil {
				out = append(out, fn.Origin())
			}
			break
		}
	}
	return out
}

// readInputs reads, in this process, every module package's directory and
// the module's go.mod and go.sum. `go list` reads them in a child process,
// which `go test`'s result cache cannot see; reading them here records them
// as test inputs, so a file added to or removed from any analyzed package,
// or a changed module file, invalidates a cached pass. (Parsed source files
// are recorded already: the parser opens each in-process.)
func readInputs(listed []*listedPackage, inModule map[string]bool) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, lp := range listed {
		if !inModule[lp.ImportPath] {
			continue
		}
		if _, err := os.ReadDir(lp.Dir); err != nil {
			return nil, fmt.Errorf("reach: reading package directory %s: %w", lp.Dir, err)
		}
		out = append(out, lp.Dir)
		var goMod string
		if raw, ok := lp.Module["GoMod"]; ok {
			if err := json.Unmarshal(raw, &goMod); err != nil {
				return nil, fmt.Errorf("reach: go list: package %s: module file: %w", lp.ImportPath, err)
			}
		}
		if goMod == "" || seen[goMod] {
			continue
		}
		seen[goMod] = true
		for _, f := range []string{goMod, filepath.Join(filepath.Dir(goMod), "go.sum")} {
			_, err := os.ReadFile(f)
			switch {
			case err == nil:
				out = append(out, f)
			case errors.Is(err, os.ErrNotExist) && f != goMod:
			default:
				return nil, fmt.Errorf("reach: reading %s: %w", f, err)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// goList runs `go list -deps -json` hermetically and strict-decodes its
// stream of package objects.
func goList(ctx context.Context, dir string, target Target, patterns []string) ([]*listedPackage, error) {
	args := append([]string{"list", "-deps", "-json=" + listFields, "--"}, patterns...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GOFLAGS=-mod=readonly",
		"GOPROXY=off",
		"GOWORK=off",
		"CGO_ENABLED=0",
		"GOOS="+target.GOOS,
		"GOARCH="+target.GOARCH,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("reach: go list %v in %s: %w: %s", patterns, dir, err, strings.TrimSpace(stderr.String()))
	}
	dec := json.NewDecoder(&stdout)
	dec.DisallowUnknownFields()
	var out []*listedPackage
	for {
		var lp listedPackage
		err := dec.Decode(&lp)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reach: decoding go list output: %w", err)
		}
		if lp.Error != nil {
			return nil, fmt.Errorf("reach: go list: package %s: %s", lp.ImportPath, lp.Error.Err)
		}
		for _, de := range lp.DepsErrors {
			return nil, fmt.Errorf("reach: go list: package %s: dependency error: %s", lp.ImportPath, de.Err)
		}
		out = append(out, &lp)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("reach: go list %v in %s listed no packages", patterns, dir)
	}
	return out, nil
}

// loader type-checks listed packages on demand, dependencies first.
type loader struct {
	target     Target
	fset       *token.FileSet
	modulePath string
	listed     map[string]*listedPackage
	checked    map[string]*types.Package
	module     map[string]*Package
	inModule   map[string]bool
}

func (l *loader) check(path string) (*types.Package, error) {
	if path == "unsafe" {
		return types.Unsafe, nil
	}
	if pkg, ok := l.checked[path]; ok {
		if pkg == nil {
			return nil, fmt.Errorf("reach: import cycle through %s", path)
		}
		return pkg, nil
	}
	lp, ok := l.listed[path]
	if !ok {
		return nil, fmt.Errorf("reach: package %s was not listed", path)
	}
	l.checked[path] = nil // cycle guard
	inModule := l.inModule[path]
	files := make([]*ast.File, 0, len(lp.GoFiles))
	for _, name := range lp.GoFiles {
		f, err := parser.ParseFile(l.fset, filepath.Join(lp.Dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("reach: parsing %s: %w", name, err)
		}
		files = append(files, f)
	}
	conf := types.Config{
		Importer:         importerFunc(func(imp string) (*types.Package, error) { return l.importFrom(lp, imp) }),
		IgnoreFuncBodies: !inModule,
		Sizes:            types.SizesFor("gc", l.target.GOARCH),
	}
	var info *types.Info
	if inModule {
		info = &types.Info{
			Types:      map[ast.Expr]types.TypeAndValue{},
			Defs:       map[*ast.Ident]types.Object{},
			Uses:       map[*ast.Ident]types.Object{},
			Selections: map[*ast.SelectorExpr]*types.Selection{},
		}
	}
	pkg, err := conf.Check(path, l.fset, files, info)
	if err != nil {
		return nil, fmt.Errorf("reach: type-checking %s: %w", path, err)
	}
	l.checked[path] = pkg
	if inModule {
		l.module[path] = &Package{Path: path, Files: files, Types: pkg, Info: info}
	}
	return pkg, nil
}

func (l *loader) importFrom(from *listedPackage, imp string) (*types.Package, error) {
	if mapped, ok := from.ImportMap[imp]; ok {
		imp = mapped
	}
	return l.check(imp)
}

type importerFunc func(path string) (*types.Package, error)

func (f importerFunc) Import(path string) (*types.Package, error) { return f(path) }
