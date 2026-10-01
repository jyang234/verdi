package reach

import (
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"sort"
	"strings"
)

// GitDirWriters returns the module functions, outside the packages in
// exclude, that write under a repository's git directory without going
// through git: a function that hands a filesystem mutation in package os
// (a function, os.CopyFS, or a mutating method of an *os.Root, whose
// receiver counts as its path) a path derived from the git directory.
//
// A path is derived from the git directory when it flows from a call to
// one of locators, from a string constant naming the git directory
// (".git", ".git/...", or one of git's --git-dir/--git-common-dir/
// --absolute-git-dir flags), through local variables, path functions,
// returns, and the parameters of other module functions. Each function
// gets a summary: whether its results carry such a path, which of its
// parameters reach its results, and which reach a filesystem mutation.
// The writer is the function where a git-directory path meets a mutation,
// or meets a parameter of another function that the summary says it
// writes; a function that writes only what it is handed is not one, so a
// generic file writer is never flagged for one caller's argument.
//
// A walk of a git-directory path (filepath.Walk, filepath.WalkDir,
// fs.WalkDir) hands its callback paths under it: a literal callback's path
// parameter carries the walk root's label, and a named callback that
// writes its path parameter makes the walk a write.
//
// Not tracked: paths stored in struct fields and read in another function,
// paths passed through channels or interface calls, and parameters of
// function literals other than a walk callback's path.
func GitDirWriters(prog *Program, locators []*types.Func, exclude []string) ([]Writer, error) {
	if len(locators) == 0 {
		return nil, errors.New("reach: GitDirWriters needs at least one locator")
	}
	loc := map[*types.Func]bool{}
	for _, l := range locators {
		if _, ok := prog.decls[l.Origin()]; !ok {
			return nil, fmt.Errorf("reach: locator %s is not a module function", l.FullName())
		}
		loc[l.Origin()] = true
	}
	excluded := map[string]bool{}
	for _, e := range exclude {
		excluded[e] = true
	}
	t := &taint{prog: prog, locators: loc, sums: map[*types.Func]*summary{}}
	var fns []*types.Func
	for fn, d := range prog.decls {
		if excluded[d.pkg.Path] || d.decl.Body == nil {
			continue
		}
		fns = append(fns, fn)
		t.sums[fn] = &summary{}
	}
	sort.Slice(fns, func(i, j int) bool { return fns[i].Pos() < fns[j].Pos() })
	for changed := true; changed; {
		changed = false
		for _, fn := range fns {
			if t.analyze(fn) {
				changed = true
			}
		}
	}
	var out []Writer
	for _, fn := range fns {
		if sum := t.sums[fn]; sum.srcWrite {
			out = append(out, Writer{Func: fn, At: prog.Fset.Position(sum.at).String()})
		}
	}
	return out, nil
}

// Writer is one function that writes under a git directory, with the
// position of the call where a git-directory path meets the write.
type Writer struct {
	Func *types.Func
	At   string
}

// Label bits: srcBit marks a git-directory path; recvBit and paramBit(i)
// mark a value derived from the receiver or the i-th parameter.
const (
	srcBit   uint64 = 1
	recvBit  uint64 = 1 << 1
	maxParam        = 61
)

func paramBit(i int) uint64 {
	if i >= maxParam {
		i = maxParam - 1
	}
	return 1 << (2 + uint(i))
}

type summary struct {
	srcRet     bool
	paramRet   uint64 // receiver/parameter bits that reach a result
	paramWrite uint64 // receiver/parameter bits that reach a mutation
	srcWrite   bool
	at         token.Pos // the first call where a git-directory path is written
}

type taint struct {
	prog     *Program
	locators map[*types.Func]bool
	sums     map[*types.Func]*summary
}

// osMutations are the package os functions that change the filesystem at
// a path argument, and the *os.Root methods that change it beneath the
// root (the receiver, opened on a path, counts as one of their paths).
func osMutations() map[string]bool {
	return map[string]bool{
		"os.Chmod": true, "os.Chown": true, "os.Chtimes": true, "os.CopyFS": true,
		"os.Create": true, "os.CreateTemp": true, "os.Lchown": true, "os.Link": true,
		"os.Mkdir": true, "os.MkdirAll": true, "os.MkdirTemp": true, "os.OpenFile": true,
		"os.Remove": true, "os.RemoveAll": true, "os.Rename": true, "os.Symlink": true,
		"os.Truncate": true, "os.WriteFile": true,

		"(*os.Root).Chmod": true, "(*os.Root).Chown": true, "(*os.Root).Chtimes": true,
		"(*os.Root).Create": true, "(*os.Root).Lchown": true, "(*os.Root).Link": true,
		"(*os.Root).Mkdir": true, "(*os.Root).MkdirAll": true, "(*os.Root).OpenFile": true,
		"(*os.Root).Remove": true, "(*os.Root).RemoveAll": true, "(*os.Root).Rename": true,
		"(*os.Root).Symlink": true, "(*os.Root).WriteFile": true,
	}
}

// walkRoots maps each directory-walk function to the index of its root
// argument; its callback is its last argument.
func walkRoots() map[string]int {
	return map[string]int{"path/filepath.Walk": 0, "path/filepath.WalkDir": 0, "io/fs.WalkDir": 1}
}

// isGitDirString reports whether a string constant names the git directory.
func isGitDirString(s string) bool {
	switch s {
	case ".git", "--git-dir", "--git-common-dir", "--absolute-git-dir":
		return true
	}
	return strings.HasPrefix(s, ".git/")
}

// fnState is one function's labels during its analysis.
type fnState struct {
	t       *taint
	pkg     *Package
	lab     map[types.Object]uint64
	ret     uint64
	write   uint64
	at      token.Pos
	changed bool
	muts    map[string]bool
	walks   map[string]int
}

// analyze recomputes fn's summary and reports whether it grew.
func (t *taint) analyze(fn *types.Func) bool {
	d := t.prog.decls[fn]
	st := &fnState{t: t, pkg: d.pkg, lab: map[types.Object]uint64{}, muts: osMutations(), walks: walkRoots()}
	sig := fn.Type().(*types.Signature)
	if sig.Recv() != nil {
		st.lab[sig.Recv()] = recvBit
	}
	for i := 0; i < sig.Params().Len(); i++ {
		st.lab[sig.Params().At(i)] = paramBit(i)
	}
	for st.changed = true; st.changed; {
		st.changed = false
		st.walk(d.decl.Body, false)
	}
	old := *t.sums[fn]
	next := summary{
		srcRet:     old.srcRet || st.ret&srcBit != 0,
		paramRet:   old.paramRet | st.ret&^srcBit,
		paramWrite: old.paramWrite | st.write&^srcBit,
		srcWrite:   old.srcWrite || st.write&srcBit != 0,
		at:         old.at,
	}
	if next.at == token.NoPos {
		next.at = st.at
	}
	*t.sums[fn] = next
	return next != old
}

func (st *fnState) set(obj types.Object, l uint64) {
	if obj == nil || l == 0 || !carriesPath(obj.Type()) {
		return
	}
	if st.lab[obj]|l != st.lab[obj] {
		st.lab[obj] |= l
		st.changed = true
	}
}

// walk applies every statement under n; inLit is true inside a function
// literal, whose returns are not the function's.
func (st *fnState) walk(n ast.Node, inLit bool) {
	ast.Inspect(n, func(x ast.Node) bool {
		switch s := x.(type) {
		case *ast.FuncLit:
			if x != n {
				st.walk(s.Body, true)
				return false
			}
		case *ast.AssignStmt:
			switch {
			case len(s.Lhs) == len(s.Rhs):
				for i := range s.Lhs {
					st.set(st.target(s.Lhs[i]), st.label(s.Rhs[i]))
				}
			case len(s.Rhs) == 1:
				l := st.label(s.Rhs[0])
				for _, lhs := range s.Lhs {
					st.set(st.target(lhs), l)
				}
			}
		case *ast.ValueSpec:
			switch {
			case len(s.Names) == len(s.Values):
				for i := range s.Names {
					st.set(st.pkg.Info.Defs[s.Names[i]], st.label(s.Values[i]))
				}
			case len(s.Values) == 1:
				l := st.label(s.Values[0])
				for _, name := range s.Names {
					st.set(st.pkg.Info.Defs[name], l)
				}
			}
		case *ast.RangeStmt:
			l := st.label(s.X)
			if s.Key != nil {
				st.set(st.target(s.Key), l)
			}
			if s.Value != nil {
				st.set(st.target(s.Value), l)
			}
		case *ast.ReturnStmt:
			if !inLit {
				for _, r := range s.Results {
					st.ret |= st.label(r)
				}
			}
		case *ast.CallExpr:
			l := st.sinkLabel(s) | st.walkLabel(s)
			if l&srcBit != 0 && st.at == token.NoPos {
				st.at = s.Pos()
			}
			st.write |= l
		}
		return true
	})
}

// walkLabel handles a directory walk: it labels a literal callback's path
// parameter with the walk root's bits, and returns the root's bits as
// written when a named module callback writes its path parameter.
func (st *fnState) walkLabel(call *ast.CallExpr) uint64 {
	var id *ast.Ident
	switch f := calleeExpr(call).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	default:
		return 0
	}
	fn, ok := st.pkg.Info.Uses[id].(*types.Func)
	if !ok {
		return 0
	}
	at, ok := st.walks[fn.FullName()]
	if !ok || len(call.Args) != at+2 {
		return 0
	}
	l := st.label(call.Args[at])
	if l == 0 {
		return 0
	}
	cb := unparen(call.Args[len(call.Args)-1])
	if lit, ok := cb.(*ast.FuncLit); ok {
		if ps := lit.Type.Params; ps != nil && len(ps.List) > 0 && len(ps.List[0].Names) > 0 {
			st.set(st.pkg.Info.Defs[ps.List[0].Names[0]], l)
		}
		return 0
	}
	if callee := st.t.prog.funcValue(st.pkg, cb); callee != nil {
		if sum, ok := st.t.sums[callee]; ok && sum.paramWrite&paramBit(0) != 0 {
			return l
		}
	}
	return 0
}

// target returns the variable an assignment to e taints: the root variable
// of a field, index, or dereference.
func (st *fnState) target(e ast.Expr) types.Object {
	for {
		switch x := unparen(e).(type) {
		case *ast.Ident:
			if obj := st.pkg.Info.Defs[x]; obj != nil {
				return obj
			}
			return st.pkg.Info.Uses[x]
		case *ast.SelectorExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		default:
			return nil
		}
	}
}

// label returns the bits e carries. Only a value that can hold a path
// carries any: a boolean, a number, an error, or another interface value
// never does, so a status check on a git-directory path taints nothing.
func (st *fnState) label(e ast.Expr) uint64 {
	if e == nil {
		return 0
	}
	tv := st.pkg.Info.Types[e]
	if v := tv.Value; v != nil {
		if v.Kind() == constant.String && isGitDirString(constant.StringVal(v)) {
			return srcBit
		}
		return 0
	}
	if _, tuple := tv.Type.(*types.Tuple); tv.Type != nil && !tuple && !carriesPath(tv.Type) {
		return 0
	}
	switch x := e.(type) {
	case *ast.Ident:
		return st.lab[st.pkg.Info.Uses[x]]
	case *ast.ParenExpr:
		return st.label(x.X)
	case *ast.StarExpr:
		return st.label(x.X)
	case *ast.UnaryExpr:
		return st.label(x.X)
	case *ast.SelectorExpr:
		if _, ok := st.pkg.Info.Selections[x]; ok {
			return st.label(x.X)
		}
		return 0
	case *ast.IndexExpr:
		return st.label(x.X)
	case *ast.SliceExpr:
		return st.label(x.X)
	case *ast.BinaryExpr:
		return st.label(x.X) | st.label(x.Y)
	case *ast.TypeAssertExpr:
		return st.label(x.X)
	case *ast.KeyValueExpr:
		return st.label(x.Value)
	case *ast.CompositeLit:
		var l uint64
		for _, elt := range x.Elts {
			l |= st.label(elt)
		}
		return l
	case *ast.CallExpr:
		return st.callLabel(x)
	}
	return 0
}

// callLabel returns the bits a call's results carry.
func (st *fnState) callLabel(call *ast.CallExpr) uint64 {
	if tv, ok := st.pkg.Info.Types[call.Fun]; ok && tv.IsType() {
		if len(call.Args) == 1 {
			return st.label(call.Args[0])
		}
		return 0
	}
	if id, ok := unparen(call.Fun).(*ast.Ident); ok {
		if b, ok := st.pkg.Info.Uses[id].(*types.Builtin); ok {
			switch b.Name() {
			case "len", "cap":
				return 0
			}
		}
	}
	callee := st.t.prog.staticCallee(st.pkg, call)
	if callee != nil && st.t.locators[callee] {
		return srcBit
	}
	if sum, ok := st.t.sums[callee]; ok && callee != nil {
		var l uint64
		if sum.srcRet {
			l |= srcBit
		}
		st.eachBoundArg(call, func(bit uint64, arg ast.Expr) {
			if sum.paramRet&bit != 0 {
				l |= st.label(arg)
			}
		})
		return l
	}
	// A dependency, an excluded package, an interface method, or a
	// function value: its result may carry any argument it was handed.
	var l uint64
	for _, a := range call.Args {
		l |= st.label(a)
	}
	if sel, ok := unparen(call.Fun).(*ast.SelectorExpr); ok {
		if _, isSel := st.pkg.Info.Selections[sel]; isSel {
			l |= st.label(sel.X)
		}
	}
	return l
}

// sinkLabel returns the bits a call writes: every argument of a package os
// filesystem mutation, and every argument bound to a parameter (or the
// receiver) that the callee's summary writes.
func (st *fnState) sinkLabel(call *ast.CallExpr) uint64 {
	sel, ok := unparen(call.Fun).(*ast.SelectorExpr)
	if ok {
		if fn, ok := st.pkg.Info.Uses[sel.Sel].(*types.Func); ok && st.muts[fn.FullName()] {
			var l uint64
			for _, a := range call.Args {
				l |= st.label(a)
			}
			if s, isMethod := st.pkg.Info.Selections[sel]; isMethod && s.Kind() == types.MethodVal {
				l |= st.label(sel.X)
			}
			return l
		}
	}
	callee := st.t.prog.staticCallee(st.pkg, call)
	sum, ok := st.t.sums[callee]
	if callee == nil || !ok || sum.paramWrite == 0 {
		return 0
	}
	var l uint64
	st.eachBoundArg(call, func(bit uint64, arg ast.Expr) {
		if sum.paramWrite&bit != 0 {
			l |= st.label(arg)
		}
	})
	return l
}

// carriesPath reports whether a value of type t can hold a path: a string,
// or a slice, array, map, pointer, or struct that can contain one.
func carriesPath(t types.Type) bool {
	return carries(t, map[types.Type]bool{})
}

func carries(t types.Type, seen map[types.Type]bool) bool {
	if t == nil || seen[t] {
		return false
	}
	seen[t] = true
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return u.Info()&types.IsString != 0
	case *types.Slice:
		return carries(u.Elem(), seen)
	case *types.Array:
		return carries(u.Elem(), seen)
	case *types.Map:
		return carries(u.Key(), seen) || carries(u.Elem(), seen)
	case *types.Pointer:
		return carries(u.Elem(), seen)
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if carries(u.Field(i).Type(), seen) {
				return true
			}
		}
	}
	return false
}

// eachBoundArg calls f with the callee-side bit of the receiver and of each
// argument of call.
func (st *fnState) eachBoundArg(call *ast.CallExpr, f func(bit uint64, arg ast.Expr)) {
	if sel, ok := unparen(call.Fun).(*ast.SelectorExpr); ok {
		if s, ok := st.pkg.Info.Selections[sel]; ok && s.Kind() == types.MethodVal {
			f(recvBit, sel.X)
		}
	}
	callee := st.t.prog.staticCallee(st.pkg, call)
	n := callee.Type().(*types.Signature).Params().Len()
	for i, a := range call.Args {
		pi := i
		if n > 0 && pi >= n {
			pi = n - 1
		}
		f(paramBit(pi), a)
	}
}
