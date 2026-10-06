package specalign

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// TestGitRecorderSeamStaticContract is the producer of obligation
// gitx-recorder-seam--ac-1--static (spec/gitx-recorder-seam ac-1, dc-1,
// dc-3; spec/ritual-write-scope-v3 ac-3, dc-5, dc-8, dc-9). It proves the
// obligation's claim, one subtest per clause:
//
//   - (a) the forbidden-token list is defined once, in a package recovery
//     imports, and holds at least the seven tokens;
//   - (b) no production Go file outside internal/gitx executes git;
//   - (c) no gitx function used for design-branch creation passes
//     update-ref.
//
// Production is the verdi binary's package closure (ledger SI-359 (4)):
// cmd/verdi and every module package it imports, transitively, read from
// the non-test files' import declarations over the module's own source.
// Nothing runs `go list`, so go.mod is never touched. No build constraint
// is evaluated, so the closure is the union over every platform, a
// superset of any one target's closure: the fail-closed direction. Test
// files, test tooling the binary does not import (fixturegit, the ritual
// witness, cmd/e2eharness) and release tooling (publicrelease) are outside
// it.
//
// The scan is syntactic (go/parser, no type checking), so its shapes are
// named where they are defined: tokenListSites for (a), execSites and
// gitProgramLiterals for (b), and the update-ref spelling for (c). The
// falsifier subtests apply one mutant per clause to the parsed source and
// require a finding.
func TestGitRecorderSeamStaticContract(t *testing.T) {
	src, err := loadSeamSource(verdiRepoRoot)
	if err != nil {
		t.Fatalf("loading the verdi binary's package closure: %v", err)
	}

	t.Run("closure", func(t *testing.T) {
		for _, f := range checkSeamClosure(src) {
			t.Error(f)
		}
		t.Logf("the verdi binary's closure: %d module packages, %d non-test files", len(src.closure), len(src.files))
	})
	t.Run("a: one shared forbidden-token list", func(t *testing.T) {
		findings, def := checkForbiddenTokenList(src)
		for _, f := range findings {
			t.Error(f)
		}
		if def != nil {
			t.Logf("the forbidden-token list is defined once, at %s, holding %q; internal/recovery imports %s", def.pos, def.tokens, def.pkg)
		}
	})
	t.Run("b: git runs only through internal/gitx", func(t *testing.T) {
		findings, sites := checkGitExecution(src, dataNamedExecExceptions())
		for _, f := range findings {
			t.Error(f)
		}
		t.Logf("%d exec sites outside internal/gitx in the closure; data-named: %d", len(sites), countDataNamed(sites))
	})
	t.Run("c: no update-ref in branch creation (static)", func(t *testing.T) {
		for _, f := range checkGitxUpdateRef(src) {
			t.Error(f)
		}
	})
	t.Run("c: no update-ref in branch creation (argv)", func(t *testing.T) {
		checkBranchCreationArgv(t)
	})

	for _, m := range seamMutants() {
		t.Run("falsifier: "+m.name, func(t *testing.T) {
			got := strings.Join(m.check(m.mutate(t, src)), "\n")
			for _, want := range m.wants {
				if !strings.Contains(got, want) {
					t.Errorf("clause %s missed the mutant %q (findings:\n%s\n); want a finding mentioning %q", m.clause, m.name, got, want)
				}
			}
		})
	}
}

// seamFile is one parsed non-test Go file of a closure package.
type seamFile struct {
	pkg     string   // module-relative package directory: "internal/recovery"
	path    string   // module-relative file path: "internal/recovery/commandlog.go"
	imports []string // import paths, as written
	file    *ast.File
}

// seamSource is the verdi binary's package closure, parsed.
type seamSource struct {
	fset    *token.FileSet
	module  string
	closure []string // module-relative package directories, sorted
	files   []seamFile
}

// loadSeamSource computes the binary's closure from cmd/verdi over the
// module rooted at root and parses every non-test file of it.
func loadSeamSource(root string) (seamSource, error) {
	module, err := modulePath(filepath.Join(root, "go.mod"))
	if err != nil {
		return seamSource{}, err
	}
	src := seamSource{fset: token.NewFileSet(), module: module}
	seen := map[string]bool{}
	queue := []string{"cmd/verdi"}
	for len(queue) > 0 {
		pkg := queue[0]
		queue = queue[1:]
		if seen[pkg] {
			continue
		}
		seen[pkg] = true
		files, err := parsePackageDir(src.fset, root, pkg)
		if err != nil {
			return seamSource{}, err
		}
		for _, f := range files {
			for _, imp := range f.imports {
				if rel, ok := moduleRel(module, imp); ok && !seen[rel] {
					queue = append(queue, rel)
				}
			}
		}
		src.files = append(src.files, files...)
	}
	for pkg := range seen {
		src.closure = append(src.closure, pkg)
	}
	sort.Strings(src.closure)
	sort.Slice(src.files, func(i, j int) bool { return src.files[i].path < src.files[j].path })
	return src, nil
}

// modulePath reads the module directive from go.mod.
func modulePath(goMod string) (string, error) {
	data, err := os.ReadFile(goMod)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`), nil
		}
	}
	return "", fmt.Errorf("%s has no module directive", goMod)
}

// moduleRel returns imp's module-relative directory when imp is one of the
// module's own packages.
func moduleRel(module, imp string) (string, bool) {
	if imp == module {
		return ".", true
	}
	rel, ok := strings.CutPrefix(imp, module+"/")
	return rel, ok
}

// parsePackageDir parses every non-test .go file directly in pkg's
// directory, whatever its build constraints. A module import with no Go
// file is an error, never an empty package.
func parsePackageDir(fset *token.FileSet, root, pkg string) ([]seamFile, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(pkg)))
	if err != nil {
		return nil, fmt.Errorf("reading package %s: %w", pkg, err)
	}
	var out []seamFile
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		rel := path.Join(pkg, name)
		f, err := parser.ParseFile(fset, filepath.Join(root, filepath.FromSlash(rel)), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", rel, err)
		}
		out = append(out, newSeamFile(pkg, rel, f))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("package %s has no non-test Go file", pkg)
	}
	return out, nil
}

func newSeamFile(pkg, rel string, f *ast.File) seamFile {
	sf := seamFile{pkg: pkg, path: rel, file: f}
	for _, imp := range f.Imports {
		if p, err := strconv.Unquote(imp.Path.Value); err == nil {
			sf.imports = append(sf.imports, p)
		}
	}
	return sf
}

// checkSeamClosure keeps the closure honest: it holds the packages each
// clause reads, and it does not follow test files' imports (cmd/verdi's
// tests import fixturegit; the binary does not).
func checkSeamClosure(src seamSource) []string {
	in := map[string]bool{}
	for _, p := range src.closure {
		in[p] = true
	}
	var findings []string
	for _, want := range []string{"cmd/verdi", "internal/gitx", "internal/recovery"} {
		if !in[want] {
			findings = append(findings, fmt.Sprintf("closure: %s is not in the verdi binary's closure", want))
		}
	}
	for _, out := range []string{"internal/fixturegit", "internal/ritualwitness", "internal/publicrelease", "cmd/e2eharness"} {
		if in[out] {
			findings = append(findings, fmt.Sprintf("closure: %s is in the verdi binary's closure; SI-359 (4) places it outside production, so a production import of it needs a fresh reading", out))
		}
	}
	return findings
}

// ---- (a) one shared forbidden-token list ----

// forbiddenTokenFloor is the floor the list must hold: ritual-write-scope-v3
// ac-3's six tokens and recovery's -f (ledger SI-224).
func forbiddenTokenFloor() []string {
	return []string{"reset", "restore", "clean", "stash", "--force", "-f", "update-ref"}
}

// tokenListSite is one syntactic list in production source that spells at
// least two distinct forbidden tokens.
type tokenListSite struct {
	pkg    string
	pos    string
	tokens []string // the distinct forbidden tokens it spells, in source order
}

// tokenListSites finds every list in the closure that spells two or more
// distinct forbidden tokens as string literals: a composite literal's
// elements (a map's keys and values alike), a switch case's expressions, or
// one const or var declaration's values. These are the shapes a token list
// is written in; a list assembled at run time from fragments is not seen
// (a disclosed limit). On the closure today the one shared definition is
// the only list spelling even two.
func tokenListSites(src seamSource) []tokenListSite {
	floor := map[string]bool{}
	for _, tok := range forbiddenTokenFloor() {
		floor[tok] = true
	}
	var sites []tokenListSite
	for _, sf := range src.files {
		ast.Inspect(sf.file, func(n ast.Node) bool {
			var exprs []ast.Expr
			switch x := n.(type) {
			case *ast.CompositeLit:
				for _, e := range x.Elts {
					if kv, ok := e.(*ast.KeyValueExpr); ok {
						exprs = append(exprs, kv.Key, kv.Value)
						continue
					}
					exprs = append(exprs, e)
				}
			case *ast.CaseClause:
				exprs = x.List
			case *ast.GenDecl:
				for _, spec := range x.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						exprs = append(exprs, vs.Values...)
					}
				}
			default:
				return true
			}
			var tokens []string
			have := map[string]bool{}
			for _, e := range exprs {
				if v, ok := stringLiteral(e); ok && floor[v] && !have[v] {
					have[v] = true
					tokens = append(tokens, v)
				}
			}
			if len(tokens) >= 2 {
				sites = append(sites, tokenListSite{pkg: sf.pkg, pos: src.position(n.Pos()), tokens: tokens})
			}
			return true
		})
	}
	return sites
}

// checkForbiddenTokenList proves clause (a): exactly one list, outside
// internal/recovery (dc-1 moved it out), holding the floor, in a package
// recovery imports. A list inside recovery is a copy whether or not
// recovery also imports the shared one. It returns the definition when
// clause (a) holds.
func checkForbiddenTokenList(src seamSource) ([]string, *tokenListSite) {
	sites := tokenListSites(src)
	var findings []string
	var shared []tokenListSite
	for _, s := range sites {
		if s.pkg == "internal/recovery" {
			findings = append(findings, fmt.Sprintf("a: internal/recovery defines its own forbidden-token list at %s %q instead of using the shared one (spec/gitx-recorder-seam dc-1)", s.pos, s.tokens))
			continue
		}
		shared = append(shared, s)
	}
	if len(sites) > 1 {
		where := make([]string, len(sites))
		for i, s := range sites {
			where[i] = fmt.Sprintf("%s %q", s.pos, s.tokens)
		}
		findings = append(findings, fmt.Sprintf("a: the forbidden-token list is defined %d times in production source, want once:\n  %s", len(sites), strings.Join(where, "\n  ")))
	}
	switch len(shared) {
	case 0:
		return append(findings, "a: no production package outside internal/recovery defines the forbidden-token list"), nil
	case 1:
	default:
		return findings, nil
	}
	def := shared[0]
	have := map[string]bool{}
	for _, tok := range def.tokens {
		have[tok] = true
	}
	for _, tok := range forbiddenTokenFloor() {
		if !have[tok] {
			findings = append(findings, fmt.Sprintf("a: the forbidden-token list at %s lacks %q (the floor is %q)", def.pos, tok, forbiddenTokenFloor()))
		}
	}
	defImport := src.module + "/" + def.pkg
	recoveryFiles, imported := 0, false
	for _, sf := range src.files {
		if sf.pkg != "internal/recovery" {
			continue
		}
		recoveryFiles++
		for _, imp := range sf.imports {
			if imp == defImport {
				imported = true
			}
		}
	}
	switch {
	case recoveryFiles == 0:
		findings = append(findings, "a: internal/recovery has no production file in the closure")
	case !imported:
		findings = append(findings, fmt.Sprintf("a: internal/recovery does not import %s, the package defining the forbidden-token list", def.pkg))
	}
	if len(findings) > 0 {
		return findings, nil
	}
	return nil, &def
}

// ---- (b) git runs only through internal/gitx ----

// dataNamedException is an exec site whose program comes from data, named
// by ledger SI-359 (4) with its reason.
type dataNamedException struct {
	path   string // module-relative file
	fn     string // enclosing function: "(Receiver).Method" or "Func"
	reason string
}

// dataNamedExecExceptions returns the closure's two exec sites whose
// program comes from data (SI-359 (4)). Each must stay exactly where it is
// named, and no third may appear.
func dataNamedExecExceptions() []dataNamedException {
	return []dataNamedException{
		{
			path:   "internal/align/judge.go",
			fn:     "(ExecJudgeRunner).RunJudge",
			reason: "runs the align judge, a `claude -p`-shaped command the store's configuration names, with the prompt on stdin",
		},
		{
			path:   "internal/execworkspace/isolation.go",
			fn:     "(Profile).Command",
			reason: "builds the launch of the isolation profile's granted program, which must be a byte-exact member of the grant's argv0 allowlist",
		},
	}
}

// execSite is one place production code starts a process.
type execSite struct {
	path    string
	fn      string
	pos     string
	program string // the constant program name; "" when it comes from data
	data    bool
	what    string // the call shape: "os/exec.CommandContext", "os/exec.Command value", ...
}

func countDataNamed(sites []execSite) int {
	n := 0
	for _, s := range sites {
		if s.data {
			n++
		}
	}
	return n
}

// isGitProgram reports whether a program name names git: "git", or a path
// whose last element is git.
func isGitProgram(v string) bool {
	if v == "git" || v == "git.exe" {
		return true
	}
	return strings.Contains(v, "/") && (path.Base(v) == "git" || path.Base(v) == "git.exe")
}

// checkGitExecution proves clause (b) over every closure file outside
// internal/gitx: no exec site runs a constant program naming git, every
// data-named exec site is a named exception, each named exception is
// where it is named, and no string literal names the git program, which
// is how a wrapper that runs its argument (publicrelease's commandBytes
// shape) would be handed git.
func checkGitExecution(src seamSource, exceptions []dataNamedException) ([]string, []execSite) {
	var findings []string
	var sites []execSite
	consts := map[string]map[string]ast.Expr{}
	for _, sf := range src.files {
		if sf.pkg == "internal/gitx" {
			continue
		}
		fileSites, reported, fileFindings := execSites(src, sf, consts)
		sites = append(sites, fileSites...)
		findings = append(findings, fileFindings...)
		for _, lit := range gitProgramLiterals(sf) {
			if !reported[lit.Pos()] {
				findings = append(findings, fmt.Sprintf("b: %s names the git program outside internal/gitx", src.position(lit.Pos())))
			}
		}
	}
	for _, s := range sites {
		if !s.data && isGitProgram(s.program) {
			findings = append(findings, fmt.Sprintf("b: %s (%s, %s) executes git outside internal/gitx", s.pos, s.fn, s.what))
		}
	}
	matched := map[int]int{}
	for _, s := range sites {
		if !s.data {
			continue
		}
		i := exceptionIndex(exceptions, s)
		if i < 0 {
			findings = append(findings, fmt.Sprintf("b: %s (%s, %s) runs a program named by data and is not a named exception (SI-359 (4) names two)", s.pos, s.fn, s.what))
			continue
		}
		matched[i]++
	}
	for i, e := range exceptions {
		if matched[i] != 1 {
			findings = append(findings, fmt.Sprintf("b: named exception %s in %s has %d data-named exec sites, want exactly 1 (moved, removed or multiplied; reason: %s)", e.fn, e.path, matched[i], e.reason))
		}
	}
	return findings, sites
}

func exceptionIndex(exceptions []dataNamedException, s execSite) int {
	for i, e := range exceptions {
		if e.path == s.path && e.fn == s.fn {
			return i
		}
	}
	return -1
}

// execSites finds one file's process starts: os/exec's Command and
// CommandContext called (program from the first or second argument) or
// used as a value (program unknown), an exec.Cmd built directly, and
// os.StartProcess and syscall's Exec, ForkExec and StartProcess. reported
// holds the positions of program literals it classified, so the
// git-literal pass does not report them twice. consts caches each
// package's constants across its files.
func execSites(src seamSource, sf seamFile, consts map[string]map[string]ast.Expr) ([]execSite, map[token.Pos]bool, []string) {
	names := map[string]string{} // local name -> import path, for the process-starting packages
	var findings []string
	for _, imp := range sf.file.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil || (p != "os/exec" && p != "os" && p != "syscall") {
			continue
		}
		name := path.Base(p)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		switch name {
		case "_":
			continue
		case ".":
			findings = append(findings, fmt.Sprintf("b: %s dot-imports %s, so its process starts cannot be audited", sf.path, p))
			continue
		}
		names[name] = p
	}
	reported := map[token.Pos]bool{}
	if len(names) == 0 {
		return nil, reported, findings
	}
	program := func(e ast.Expr, decl ast.Decl) (string, bool) {
		if consts[sf.pkg] == nil {
			consts[sf.pkg] = packageConsts(src, sf.pkg)
		}
		return constString(e, consts[sf.pkg], shadowedNames(decl), 0)
	}
	var sites []execSite
	for _, decl := range sf.file.Decls {
		fn := funcName(decl)
		calls := map[ast.Node]bool{}
		ast.Inspect(decl, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				calls[call.Fun] = true
			}
			return true
		})
		ast.Inspect(decl, func(n ast.Node) bool {
			if n == nil {
				return true
			}
			site := execSite{path: sf.path, fn: fn, pos: src.position(n.Pos())}
			switch x := n.(type) {
			case *ast.CallExpr:
				pkgPath, sel := qualified(names, x.Fun)
				prog := -1
				switch {
				case pkgPath == "os/exec" && sel == "Command":
					prog = 0
				case pkgPath == "os/exec" && sel == "CommandContext":
					prog = 1
				case pkgPath == "os" && sel == "StartProcess", pkgPath == "syscall" && (sel == "Exec" || sel == "ForkExec" || sel == "StartProcess"):
					prog = 0
				}
				if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "new" && len(x.Args) == 1 {
					if p, s := qualified(names, x.Args[0]); p == "os/exec" && s == "Cmd" {
						site.data, site.what = true, "new(os/exec.Cmd)"
						sites = append(sites, site)
					}
				}
				if prog < 0 || len(x.Args) <= prog {
					return true
				}
				site.what = pkgPath + "." + sel
				arg := x.Args[prog]
				if v, ok := program(arg, decl); ok {
					site.program = v
					reported[arg.Pos()] = true
				} else {
					site.data = true
				}
				sites = append(sites, site)
			case *ast.SelectorExpr:
				if calls[x] {
					return true
				}
				if pkgPath, sel := qualified(names, x); pkgPath == "os/exec" && (sel == "Command" || sel == "CommandContext") {
					site.data, site.what = true, "os/exec."+sel+" value"
					sites = append(sites, site)
				}
			case *ast.CompositeLit:
				if pkgPath, sel := qualified(names, x.Type); pkgPath == "os/exec" && sel == "Cmd" {
					site.data, site.what = true, "os/exec.Cmd literal"
					for _, e := range x.Elts {
						kv, ok := e.(*ast.KeyValueExpr)
						if !ok {
							continue
						}
						if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Path" {
							if v, ok := program(kv.Value, decl); ok {
								site.data, site.program = false, v
								reported[kv.Value.Pos()] = true
							}
						}
					}
					sites = append(sites, site)
				}
			}
			return true
		})
	}
	return sites, reported, findings
}

// qualified resolves pkg.Sel against names (local import name -> path).
func qualified(names map[string]string, e ast.Expr) (string, string) {
	if star, ok := e.(*ast.StarExpr); ok {
		e = star.X
	}
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return "", ""
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", ""
	}
	p, ok := names[id.Name]
	if !ok {
		return "", ""
	}
	return p, sel.Sel.Name
}

// funcName names a declaration's function: "F", "(T).M" or "(*T).M"; ""
// for a package-level declaration that is not a function.
func funcName(decl ast.Decl) string {
	fd, ok := decl.(*ast.FuncDecl)
	if !ok {
		return ""
	}
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	recv := fd.Recv.List[0].Type
	star := ""
	if s, ok := recv.(*ast.StarExpr); ok {
		star, recv = "*", s.X
	}
	switch r := recv.(type) {
	case *ast.IndexExpr:
		recv = r.X
	case *ast.IndexListExpr:
		recv = r.X
	}
	name := "?"
	if id, ok := recv.(*ast.Ident); ok {
		name = id.Name
	}
	return "(" + star + name + ")." + fd.Name.Name
}

// packageConsts maps every const name declared anywhere in pkg's files to
// its value expression. A name declared twice (in different scopes) maps
// to nil: ambiguous, so never treated as constant.
func packageConsts(src seamSource, pkg string) map[string]ast.Expr {
	consts := map[string]ast.Expr{}
	seen := map[string]bool{}
	for _, sf := range src.files {
		if sf.pkg != pkg {
			continue
		}
		ast.Inspect(sf.file, func(n ast.Node) bool {
			gd, ok := n.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				return true
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range vs.Names {
					var value ast.Expr
					if i < len(vs.Values) {
						value = vs.Values[i]
					}
					if seen[name.Name] {
						value = nil
					}
					seen[name.Name] = true
					consts[name.Name] = value
				}
			}
			return true
		})
	}
	return consts
}

// shadowedNames returns every name a declaration binds as a parameter,
// result, receiver, := target, range variable or var: such a name is data
// inside it even when a package constant shares it.
func shadowedNames(decl ast.Decl) map[string]bool {
	names := map[string]bool{}
	addFields := func(fl *ast.FieldList) {
		if fl == nil {
			return
		}
		for _, f := range fl.List {
			for _, n := range f.Names {
				names[n.Name] = true
			}
		}
	}
	ast.Inspect(decl, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			addFields(x.Recv)
			addFields(x.Type.Params)
			addFields(x.Type.Results)
		case *ast.FuncLit:
			addFields(x.Type.Params)
			addFields(x.Type.Results)
		case *ast.AssignStmt:
			if x.Tok == token.DEFINE {
				for _, l := range x.Lhs {
					if id, ok := l.(*ast.Ident); ok {
						names[id.Name] = true
					}
				}
			}
		case *ast.RangeStmt:
			for _, e := range []ast.Expr{x.Key, x.Value} {
				if id, ok := e.(*ast.Ident); ok {
					names[id.Name] = true
				}
			}
		case *ast.GenDecl:
			if x.Tok != token.VAR {
				return true
			}
			for _, spec := range x.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					for _, n := range vs.Names {
						names[n.Name] = true
					}
				}
			}
		}
		return true
	})
	return names
}

// constString evaluates a string constant expression: a string literal, a
// constant of the package (unless shadowed), a parenthesized one, or a sum
// of them. Anything else comes from data.
func constString(e ast.Expr, consts map[string]ast.Expr, shadowed map[string]bool, depth int) (string, bool) {
	if depth > 16 {
		return "", false
	}
	switch x := e.(type) {
	case *ast.BasicLit:
		return stringLiteral(x)
	case *ast.ParenExpr:
		return constString(x.X, consts, shadowed, depth+1)
	case *ast.Ident:
		value, ok := consts[x.Name]
		if !ok || value == nil || shadowed[x.Name] {
			return "", false
		}
		return constString(value, consts, nil, depth+1)
	case *ast.BinaryExpr:
		if x.Op != token.ADD {
			return "", false
		}
		l, ok := constString(x.X, consts, shadowed, depth+1)
		if !ok {
			return "", false
		}
		r, ok := constString(x.Y, consts, shadowed, depth+1)
		if !ok {
			return "", false
		}
		return l + r, true
	}
	return "", false
}

func stringLiteral(e ast.Expr) (string, bool) {
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(bl.Value)
	if err != nil {
		return "", false
	}
	return v, true
}

// gitProgramLiterals returns a file's string literals that name the git
// program, outside import paths and struct tags (a `json:"git"` tag is
// not a program name).
func gitProgramLiterals(sf seamFile) []*ast.BasicLit {
	skip := map[*ast.BasicLit]bool{}
	for _, imp := range sf.file.Imports {
		skip[imp.Path] = true
	}
	var out []*ast.BasicLit
	ast.Inspect(sf.file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Field:
			if x.Tag != nil {
				skip[x.Tag] = true
			}
		case *ast.BasicLit:
			if skip[x] {
				return true
			}
			if v, ok := stringLiteral(x); ok && isGitProgram(v) {
				out = append(out, x)
			}
		}
		return true
	})
	return out
}

// ---- (c) no update-ref in branch creation ----

// checkGitxUpdateRef proves clause (c) statically over every gitx
// function, the design-branch creators among them: no string literal in
// internal/gitx's production source spells update-ref, so no gitx argv
// can carry it as written. checkBranchCreationArgv covers an argv built
// any other way, for the functions that create a branch.
func checkGitxUpdateRef(src seamSource) []string {
	var findings []string
	files := 0
	for _, sf := range src.files {
		if sf.pkg != "internal/gitx" {
			continue
		}
		files++
		ast.Inspect(sf.file, func(n ast.Node) bool {
			if bl, ok := n.(*ast.BasicLit); ok {
				if v, ok := stringLiteral(bl); ok && v == "update-ref" {
					findings = append(findings, fmt.Sprintf("c: %s spells update-ref in internal/gitx's production source", src.position(bl.Pos())))
				}
			}
			return true
		})
	}
	if files == 0 {
		findings = append(findings, "c: internal/gitx has no production file in the closure")
	}
	return findings
}

// argvRecord records every git argv gitx runs on its context.
type argvRecord struct {
	mu   sync.Mutex
	argv [][]string
}

func (r *argvRecord) Observe(_ string, args []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.argv = append(r.argv, append([]string(nil), args...))
}

func (r *argvRecord) take() [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.argv
	r.argv = nil
	return out
}

// checkBranchCreationArgv runs each gitx function that creates a branch —
// UpdateRef (design start --from-stub, the board's stub, create and
// revise actions, spec import: parent dc-8), CheckoutNewBranchFrom
// (design start) and CheckoutNewBranch — on a fixturegit repository under
// an observer, and requires that each created its branch and that no argv
// it ran carries update-ref.
func checkBranchCreationArgv(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "a\n"}, Message: "seed"}})
	rec := &argvRecord{}
	ctx := gitx.WithObserver(context.Background(), rec)
	cases := []struct {
		name   string
		branch string
		create func() error
	}{
		{"UpdateRef", "design/static-contract-stub", func() error {
			return gitx.UpdateRef(ctx, repo.Dir, "refs/heads/design/static-contract-stub", repo.Head)
		}},
		{"CheckoutNewBranchFrom", "design/static-contract-start", func() error {
			return gitx.CheckoutNewBranchFrom(ctx, repo.Dir, "design/static-contract-start", repo.Head)
		}},
		{"CheckoutNewBranch", "close/static-contract", func() error {
			return gitx.CheckoutNewBranch(ctx, repo.Dir, "close/static-contract")
		}},
	}
	for _, tc := range cases {
		if err := tc.create(); err != nil {
			t.Fatalf("c: gitx.%s: %v", tc.name, err)
		}
		runs := rec.take()
		if len(runs) == 0 {
			t.Fatalf("c: gitx.%s ran no git command under the observer — the check would pass vacuously", tc.name)
		}
		for _, argv := range runs {
			for _, arg := range argv {
				if arg == "update-ref" {
					t.Errorf("c: gitx.%s runs `git %s`, which passes update-ref", tc.name, strings.Join(argv, " "))
				}
			}
		}
		if got, err := gitx.RevParse(context.Background(), repo.Dir, "refs/heads/"+tc.branch); err != nil || got != repo.Head {
			t.Errorf("c: after gitx.%s, refs/heads/%s = %q (%v), want %s", tc.name, tc.branch, got, err, repo.Head)
		}
		t.Logf("c: gitx.%s ran %q", tc.name, runs)
	}
}

// ---- falsifiers ----

// seamMutant is one mutation of the parsed closure that its clause must
// catch: every string in wants must appear in the clause's findings.
type seamMutant struct {
	name   string
	clause string
	mutate func(*testing.T, seamSource) seamSource
	check  func(seamSource) []string
	wants  []string
}

func checkClauseA(src seamSource) []string {
	findings, _ := checkForbiddenTokenList(src)
	return findings
}

func checkClauseB(src seamSource) []string {
	findings, _ := checkGitExecution(src, dataNamedExecExceptions())
	return findings
}

// copiedTokensFile is recovery holding its own copy of the seven tokens.
const copiedTokensFile = `package recovery
func copiedTokens() []string { return []string{"reset", "restore", "clean", "stash", "--force", "-f", "update-ref"} }
func forbidsCopied(argv []string) bool {
	for _, arg := range argv {
		for _, tok := range copiedTokens() {
			if arg == tok {
				return true
			}
		}
	}
	return false
}
`

func seamMutants() []seamMutant {
	gitforbid := func(src seamSource) string { return src.module + "/internal/gitforbid" }
	return []seamMutant{
		{
			name: "a second token list in a production package", clause: "a", check: checkClauseA,
			mutate: func(t *testing.T, src seamSource) seamSource {
				return withSeamFile(t, src, "internal/store", "forbidden.go", `package store
var legacyForbidden = []string{"reset", "restore", "clean", "stash", "--force", "-f", "update-ref"}
`)
			},
			wants: []string{"defined 2 times", "internal/store/forbidden.go:2"},
		},
		{
			name: "a second list as a switch on two tokens", clause: "a", check: checkClauseA,
			mutate: func(t *testing.T, src seamSource) seamSource {
				return withSeamFile(t, src, "internal/store", "forbidden.go", `package store
func forbidden(arg string) bool {
	switch arg {
	case "reset", "update-ref":
		return true
	}
	return false
}
`)
			},
			wants: []string{"defined 2 times", "internal/store/forbidden.go:4"},
		},
		{
			name: "recovery copies the list and stops importing gitforbid (R5AR-5, shape 1)", clause: "a", check: checkClauseA,
			mutate: func(t *testing.T, src seamSource) seamSource {
				return withSeamFile(t, withoutSeamImport(src, "internal/recovery", gitforbid(src)), "internal/recovery", "copiedtokens.go", copiedTokensFile)
			},
			wants: []string{"internal/recovery defines its own forbidden-token list at internal/recovery/copiedtokens.go:2", "defined 2 times", "internal/recovery does not import internal/gitforbid"},
		},
		{
			name: "recovery imports gitforbid but uses a local copy (R5AR-5, shape 2)", clause: "a", check: checkClauseA,
			mutate: func(t *testing.T, src seamSource) seamSource {
				return withSeamFile(t, src, "internal/recovery", "copiedtokens.go", copiedTokensFile)
			},
			wants: []string{"internal/recovery defines its own forbidden-token list at internal/recovery/copiedtokens.go:2", "defined 2 times"},
		},
		{
			name: "recovery not importing the shared package", clause: "a", check: checkClauseA,
			mutate: func(t *testing.T, src seamSource) seamSource {
				return withoutSeamImport(src, "internal/recovery", gitforbid(src))
			},
			wants: []string{"internal/recovery does not import internal/gitforbid"},
		},
		{
			name: "a token dropped from the list", clause: "a", check: checkClauseA,
			mutate: func(t *testing.T, src seamSource) seamSource {
				return withSeamFileReplaced(t, src, "internal/gitforbid/gitforbid.go", `package gitforbid
func Tokens() []string { return []string{"reset", "restore", "clean", "stash", "--force", "update-ref"} }
`)
			},
			wants: []string{`lacks "-f"`},
		},
		{
			name: "a literal exec.Command(\"git\", …) in a closure package", clause: "b", check: checkClauseB,
			mutate: func(t *testing.T, src seamSource) seamSource {
				return withSeamFile(t, src, "internal/store", "rawgit.go", `package store
import "os/exec"
func rawGit() error { return exec.Command("git", "status").Run() }
`)
			},
			wants: []string{"internal/store/rawgit.go:3 (rawGit, os/exec.Command) executes git outside internal/gitx"},
		},
		{
			name: "dc-3: the draft identity's top-level read restored to a raw exec", clause: "b", check: checkClauseB,
			mutate: func(t *testing.T, src seamSource) seamSource {
				return withSeamFile(t, src, "internal/draftmutation", "rawtoplevel.go", `package draftmutation
import (
	"context"
	osexec "os/exec"
)
const gitProgram = "git"
func rawTopLevel(ctx context.Context, start string) ([]byte, error) {
	return osexec.CommandContext(ctx, gitProgram, "-C", start, "rev-parse", "--show-toplevel").Output()
}
`)
			},
			wants: []string{"internal/draftmutation/rawtoplevel.go:8 (rawTopLevel, os/exec.CommandContext) executes git outside internal/gitx"},
		},
		{
			name: "a wrapper handed the git program", clause: "b", check: checkClauseB,
			mutate: func(t *testing.T, src seamSource) seamSource {
				return withSeamFile(t, src, "internal/store", "wrapper.go", `package store
func status(run func(string, ...string) error) error { return run("git", "status") }
`)
			},
			wants: []string{"internal/store/wrapper.go:2 names the git program outside internal/gitx"},
		},
		{
			name: "a third data-named exec site", clause: "b", check: checkClauseB,
			mutate: func(t *testing.T, src seamSource) seamSource {
				return withSeamFile(t, src, "internal/store", "runtool.go", `package store
import "os/exec"
func runTool(name string, args ...string) error { return exec.Command(name, args...).Run() }
`)
			},
			wants: []string{"internal/store/runtool.go:3 (runTool, os/exec.Command) runs a program named by data and is not a named exception"},
		},
		{
			name: "exec.CommandContext passed as a value", clause: "b", check: checkClauseB,
			mutate: func(t *testing.T, src seamSource) seamSource {
				return withSeamFile(t, src, "internal/store", "hook.go", `package store
import "os/exec"
var command = exec.CommandContext
`)
			},
			wants: []string{"internal/store/hook.go:3", "os/exec.CommandContext value"},
		},
		{
			name: "a named exception moved", clause: "b", check: checkClauseB,
			mutate: func(t *testing.T, src seamSource) seamSource {
				return withSeamFileMoved(src, "internal/align/judge.go", "internal/align/judgeexec.go")
			},
			wants: []string{"named exception (ExecJudgeRunner).RunJudge in internal/align/judge.go has 0 data-named exec sites"},
		},
		{
			name: "UpdateRef back to update-ref", clause: "c", check: checkGitxUpdateRef,
			mutate: func(t *testing.T, src seamSource) seamSource {
				return withSeamFile(t, src, "internal/gitx", "updateref.go", `package gitx
import "context"
func updateRef(ctx context.Context, dir, ref, commit string) error {
	_, err := runStdin(ctx, dir, nil, nil, "update-ref", ref, commit, "0000000000000000000000000000000000000000")
	return err
}
`)
			},
			wants: []string{"internal/gitx/updateref.go:4 spells update-ref"},
		},
	}
}

// withSeamFile returns src with one more parsed file in pkg.
func withSeamFile(t *testing.T, src seamSource, pkg, name, code string) seamSource {
	t.Helper()
	rel := path.Join(pkg, name)
	f, err := parser.ParseFile(src.fset, rel, code, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parsing mutant %s: %v", rel, err)
	}
	out := src
	out.files = append(append([]seamFile(nil), src.files...), newSeamFile(pkg, rel, f))
	return out
}

// withSeamFileReplaced returns src with the file at rel replaced by code.
func withSeamFileReplaced(t *testing.T, src seamSource, rel, code string) seamSource {
	t.Helper()
	f, err := parser.ParseFile(src.fset, rel, code, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parsing mutant %s: %v", rel, err)
	}
	out := src
	out.files = nil
	found := false
	for _, sf := range src.files {
		if sf.path == rel {
			sf, found = newSeamFile(sf.pkg, rel, f), true
		}
		out.files = append(out.files, sf)
	}
	if !found {
		t.Fatalf("mutant: %s is not in the closure", rel)
	}
	return out
}

// withoutSeamImport returns src with importPath removed from pkg's files'
// imports.
func withoutSeamImport(src seamSource, pkg, importPath string) seamSource {
	out := src
	out.files = nil
	for _, sf := range src.files {
		if sf.pkg == pkg {
			var kept []string
			for _, imp := range sf.imports {
				if imp != importPath {
					kept = append(kept, imp)
				}
			}
			sf.imports = kept
		}
		out.files = append(out.files, sf)
	}
	return out
}

// withSeamFileMoved returns src with the file at from renamed to to.
func withSeamFileMoved(src seamSource, from, to string) seamSource {
	out := src
	out.files = nil
	for _, sf := range src.files {
		if sf.path == from {
			sf.path = to
		}
		out.files = append(out.files, sf)
	}
	return out
}

// position renders a module-relative file:line.
func (s seamSource) position(p token.Pos) string {
	pos := s.fset.Position(p)
	name := filepath.ToSlash(pos.Filename)
	if rel, err := filepath.Rel(verdiRepoRoot, pos.Filename); err == nil && !strings.HasPrefix(rel, "..") {
		name = filepath.ToSlash(rel)
	}
	return fmt.Sprintf("%s:%d", name, pos.Line)
}
