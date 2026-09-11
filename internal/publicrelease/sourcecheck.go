package publicrelease

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

const sourceVerdiBaseline = "8ab423fefa14f6cee3070ca96754928daf28c062"
const sourceATCBaseline = "b1f9e995a62aaa2f2ea78248f9b784fd3720817d"
const sourceRolloutSHA = "9b987b57bdf99d4491035d753a5ebd201fb0727d3c940a7b1ed56845c2a45501"
const sourceWitnessSHA = "d293818aefb2950418e1f84d13240b83dac539f5a385b548b2ce73e99eea14a4"

// SourceCheck is a source-free inventory row. It records inspected file hashes
// and symbol/order facts, never private source text or a claimed test outcome.
type SourceCheck struct {
	AC    string            `json:"ac"`
	Name  string            `json:"name"`
	Files map[string]string `json:"files"`
	Facts []string          `json:"facts"`
}
type sourceUnit struct {
	path  string
	raw   []byte
	set   *token.FileSet
	file  *ast.File
	funcs map[string]*ast.FuncDecl
}
type sourceTree map[string]*sourceUnit

func sourceHash(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func sourceText(set *token.FileSet, n ast.Node) string {
	var b bytes.Buffer
	if err := format.Node(&b, set, n); err != nil {
		return ""
	}
	return b.String()
}
func sourceReceiver(f *ast.FuncDecl) string {
	if f.Recv == nil {
		return ""
	}
	t := f.Recv.List[0].Type
	if p, ok := t.(*ast.StarExpr); ok {
		t = p.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name + "."
	}
	return ""
}
func parseSourceUnit(path string, raw []byte) (*sourceUnit, error) {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, raw, 0)
	if err != nil {
		return nil, fmt.Errorf("parse source %s: %w", path, err)
	}
	u := &sourceUnit{path: path, raw: raw, set: set, file: file, funcs: map[string]*ast.FuncDecl{}}
	for _, d := range file.Decls {
		if f, ok := d.(*ast.FuncDecl); ok {
			u.funcs[sourceReceiver(f)+f.Name.Name] = f
		}
	}
	return u, nil
}
func readSourceTree(ctx context.Context, root string) (sourceTree, error) {
	tree := sourceTree{}
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if err = ctx.Err(); err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			u, err := parseSourceUnit(filepath.ToSlash(rel), raw)
			if err == nil {
				tree[u.path] = u
			}
			return err
		})
		if err != nil {
			return nil, err
		}
	}
	return tree, nil
}
func (t sourceTree) unit(path string) (*sourceUnit, error) {
	u := t[path]
	if u == nil {
		return nil, fmt.Errorf("source inventory missing %s", path)
	}
	return u, nil
}
func sourceFunction(u *sourceUnit, name string) (*ast.FuncDecl, error) {
	f := u.funcs[name]
	if f == nil || f.Body == nil {
		return nil, fmt.Errorf("source inventory missing %s:%s", u.path, name)
	}
	return f, nil
}
func sourceCalls(f *ast.FuncDecl) []*ast.CallExpr {
	var calls []*ast.CallExpr
	ast.Inspect(f.Body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		if c, ok := n.(*ast.CallExpr); ok {
			calls = append(calls, c)
		}
		return true
	})
	return calls
}
func sourceCallName(c *ast.CallExpr) string {
	switch f := c.Fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}
func sourceReturns(b *ast.BlockStmt) bool {
	for _, s := range b.List {
		if _, ok := s.(*ast.ReturnStmt); ok {
			return true
		}
	}
	return false
}
func sourceErrGuard(s ast.Stmt, errorName string) bool {
	v, ok := s.(*ast.IfStmt)
	if !ok {
		return false
	}
	b, ok := v.Cond.(*ast.BinaryExpr)
	if !ok || b.Op != token.NEQ {
		return false
	}
	id, ok := b.Y.(*ast.Ident)
	left, lok := b.X.(*ast.Ident)
	return ok && lok && errorName != "" && left.Name == errorName && id.Name == "nil" && sourceReturns(v.Body)
}

// sourceGuarded follows the containing statement blocks, not just token order.
// A validation parked in a false branch or closure is not an admission anchor.
func sourceGuarded(f *ast.FuncDecl, c *ast.CallExpr) bool {
	valid := false
	dead := false
	errorName := ""
	ast.Inspect(f.Body, func(n ast.Node) bool {
		if a, ok := n.(*ast.AssignStmt); ok && a.Pos() <= c.Pos() && a.End() >= c.End() && len(a.Lhs) > 0 {
			if id, ok := a.Lhs[len(a.Lhs)-1].(*ast.Ident); ok && id.Name != "_" {
				errorName = id.Name
			}
		}
		return true
	})
	ast.Inspect(f.Body, func(n ast.Node) bool {
		if n == nil {
			return true
		}
		if n.Pos() > c.Pos() || n.End() < c.End() {
			return false
		}
		if v, ok := n.(*ast.IfStmt); ok {
			if id, ok := v.Cond.(*ast.Ident); ok && id.Name == "false" {
				dead = true
			}
			if v.Init != nil && v.Init.Pos() <= c.Pos() && v.Init.End() >= c.End() && sourceErrGuard(v, errorName) {
				valid = true
			}
		}
		if b, ok := n.(*ast.BlockStmt); ok {
			for i, s := range b.List {
				if s.Pos() <= c.Pos() && s.End() >= c.End() && i+1 < len(b.List) && sourceErrGuard(b.List[i+1], errorName) {
					valid = true
				}
			}
		}
		return true
	})
	return valid && !dead
}
func sourceOrdered(u *sourceUnit, name string, order, guarded []string) error {
	f, err := sourceFunction(u, name)
	if err != nil {
		return err
	}
	// Operation 23 has a separate claims result. Its successful return does
	// not belong to the 22-owner admission path being ordered here.
	if name == "Controller.answer" {
		copyFunc := *f
		copyBody := *f.Body
		copyBody.List = nil
		for _, stmt := range f.Body.List {
			if branch, ok := stmt.(*ast.IfStmt); ok && sourceText(u.set, branch.Cond) == "owner == OwnerClaims" {
				continue
			}
			copyBody.List = append(copyBody.List, stmt)
		}
		copyFunc.Body = &copyBody
		f = &copyFunc
	}
	calls := sourceCalls(f)
	last := token.NoPos
	for _, want := range order {
		var found *ast.CallExpr
		for _, c := range calls {
			if sourceCallName(c) == want {
				found = c
				break
			}
		}
		if found == nil || found.Pos() <= last {
			return fmt.Errorf("source order missing %s:%s:%s", u.path, name, want)
		}
		last = found.Pos()
		for _, g := range guarded {
			if g == want && !sourceGuarded(f, found) {
				return fmt.Errorf("source validation unguarded %s:%s:%s", u.path, name, want)
			}
		}
	}
	return nil
}
func sourceConsumers(tree sourceTree, want map[string]string) error {
	found := map[string]string{}
	references := 0
	for path, u := range tree {
		declarations := map[token.Pos]bool{}
		for _, f := range u.funcs {
			declarations[f.Name.Pos()] = true
		}
		ast.Inspect(u.file, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == "openSealedController" && !declarations[id.Pos()] {
				references++
			}
			return true
		})
		for name, f := range u.funcs {
			for _, c := range sourceCalls(f) {
				if sourceCallName(c) == "openSealedController" {
					key := path + ":" + name
					if found[key] != "" {
						return fmt.Errorf("duplicate FD3 consumer %s", key)
					}
					found[key] = "present"
				}
			}
		}
	}
	expected := map[string]string{}
	for p, n := range want {
		expected[p+":"+n] = "present"
	}
	if references != len(found) || !reflect.DeepEqual(found, expected) {
		return fmt.Errorf("FD3 consumer inventory differs: got %d want %d", len(found), len(expected))
	}
	return nil
}
func sourceNoTranslations(tree sourceTree) error {
	for path, u := range tree {
		if path == "internal/verdiproto/owner_client.go" {
			continue
		}
		var bad string
		ast.Inspect(u.file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Ident:
				if x.Name == "DecodeOwner" || x.Name == "EncodeOwner" || x.Name == "OpenOwnerBridge" || x.Name == "newOwnerBridge" || x.Name == "OwnerBridge" {
					bad = x.Name
				}
			}
			return bad == ""
		})
		if bad != "" {
			return fmt.Errorf("production translation reference %s:%s", path, bad)
		}
	}
	return nil
}
func sourceDeclarations(u *sourceUnit) map[string]string {
	out := map[string]string{}
	for _, d := range u.file.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok == token.IMPORT {
			continue
		}
		for _, s := range g.Specs {
			switch x := s.(type) {
			case *ast.TypeSpec:
				out["type:"+x.Name.Name] = sourceText(u.set, x)
			case *ast.ValueSpec:
				for _, n := range x.Names {
					out[g.Tok.String()+":"+n.Name] = sourceText(u.set, x)
				}
			}
		}
	}
	return out
}
func sourceSameDeclarations(before, after *sourceUnit) error {
	if !reflect.DeepEqual(sourceDeclarations(before), sourceDeclarations(after)) {
		return fmt.Errorf("durable declarations changed %s", after.path)
	}
	return nil
}
func sourceSameFunction(before, after *sourceUnit, name string) error {
	a, err := sourceFunction(before, name)
	if err != nil {
		return err
	}
	b, err := sourceFunction(after, name)
	if err != nil {
		return err
	}
	if sourceText(before.set, a) != sourceText(after.set, b) {
		return fmt.Errorf("durable identity/write function changed %s:%s", after.path, name)
	}
	return nil
}
func sourceLimitExpression(e ast.Expr, ceiling string) bool {
	b, ok := e.(*ast.BinaryExpr)
	if !ok || b.Op != token.SUB {
		return false
	}
	id, ok := b.X.(*ast.Ident)
	if !ok || id.Name != ceiling {
		return false
	}
	c, ok := b.Y.(*ast.CallExpr)
	if !ok || sourceCallName(c) != "len" || len(c.Args) != 1 {
		return false
	}
	frame, ok := c.Args[0].(*ast.Ident)
	return ok && frame.Name == "frame"
}
func sourceLengthOf(e ast.Expr, name string) bool {
	c, ok := e.(*ast.CallExpr)
	if !ok || sourceCallName(c) != "len" || len(c.Args) != 1 {
		return false
	}
	id, ok := c.Args[0].(*ast.Ident)
	return ok && name != "" && id.Name == name
}
func sourceBoundedRead(u *sourceUnit, name, ceiling string) error {
	f, err := sourceFunction(u, name)
	if err != nil {
		return err
	}
	var read, appendAt, bound token.Pos
	unbounded := false
	reads := 0
	chunkName := ""
	ast.Inspect(f.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if len(x.Rhs) == 1 && len(x.Lhs) > 0 {
				if c, ok := x.Rhs[0].(*ast.CallExpr); ok && sourceCallName(c) == "ReadSlice" {
					if id, ok := x.Lhs[0].(*ast.Ident); ok {
						chunkName = id.Name
					}
				}
			}
		case *ast.CallExpr:
			switch sourceCallName(x) {
			case "ReadBytes", "ReadString", "ReadAll":
				unbounded = true
				return false
			case "ReadSlice":
				read = x.Pos()
				reads++
			case "append":
				appendAt = x.Pos()
			}
		case *ast.IfStmt:
			b, ok := x.Cond.(*ast.BinaryExpr)
			if ok && b.Op == token.GEQ && sourceLengthOf(b.X, chunkName) && sourceLimitExpression(b.Y, ceiling) && sourceReturns(x.Body) {
				bound = x.Pos()
			}
		}
		return true
	})
	if unbounded || reads != 1 || read == token.NoPos || bound <= read || appendAt <= bound {
		return fmt.Errorf("incremental exclusive read bound missing %s:%s", u.path, name)
	}
	return nil
}
func sourceGitUnit(ctx context.Context, dir, commit, path string) (*sourceUnit, error) {
	cmd := exec.CommandContext(ctx, "git", "show", commit+":"+path)
	cmd.Dir = dir
	b, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("fixed baseline %s:%s unavailable: %w", commit, path, err)
	}
	return parseSourceUnit(path, b)
}
func sourceRow(ac, name, prefix string, tree sourceTree, paths []string, facts ...string) (SourceCheck, error) {
	r := SourceCheck{AC: ac, Name: name, Files: map[string]string{}, Facts: facts}
	for _, p := range paths {
		u, err := tree.unit(p)
		if err != nil {
			return r, err
		}
		r.Files[prefix+":"+p] = sourceHash(u.raw)
	}
	return r, nil
}
func sourceDependencyCheck(ctx context.Context, root, forbidden string) error {
	cmd := exec.CommandContext(ctx, "go", "list", "-deps", "-f", "{{.ImportPath}}", "./cmd/...")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off")
	b, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("package dependency inventory: %w", err)
	}
	count := 0
	for _, path := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if path == "" || strings.ContainsAny(path, " \t\r") {
			return fmt.Errorf("invalid dependency import-path line")
		}
		count++
		if path == forbidden || strings.HasPrefix(path, forbidden+"/") || strings.Contains(path, "/verdi-go") {
			return fmt.Errorf("excluded package dependency %s", path)
		}
	}
	if count == 0 {
		return fmt.Errorf("empty dependency inventory")
	}
	return nil
}

// SourceChecks inspects the declared production paths and fixed durable baseline.
// It is deliberately not whole-program analysis: runtime tests and the accepted
// Task4 witness remain required independent evidence for each release producer.
func SourceChecks(ctx context.Context, verdiDir, atcDir string) ([]SourceCheck, error) {
	v, err := readSourceTree(ctx, verdiDir)
	if err != nil {
		return nil, err
	}
	a, err := readSourceTree(ctx, atcDir)
	if err != nil {
		return nil, err
	}
	var rows []SourceCheck
	bindings := map[string]string{}
	add := func(ac, name, prefix string, tree sourceTree, paths []string, facts ...string) error {
		r, e := sourceRow(ac, name, prefix, tree, paths, facts...)
		if e == nil {
			rows = append(rows, r)
		}
		return e
	}
	if err = sourceRegistries(v, a); err != nil {
		return nil, err
	}
	if err = add("ac-1", "exact-operation-and-schema-registry", "verdi", v, []string{"internal/sealedexec/controller_schema.go", "internal/contextowner/schema.go", "internal/contextowner/identity.go", "internal/sealedexec/owner_bridge.go"}, "AST literal registries contain the exact ordered 23 controller and 22 owner operations; schema constants and exclusive 32 MiB bounds checked in both endpoints"); err != nil {
		return nil, err
	}
	for _, path := range []string{"internal/verdiproto/controller.go", "internal/verdiproto/owner_schema.go"} {
		rows[len(rows)-1].Files["atc:"+path] = sourceHash(a[path].raw)
	}
	consumers := map[string]string{"cmd/verdi/context_execution.go": "cmdContextExecution", "cmd/verdi/context_mcp.go": "cmdContextMCP", "cmd/verdi/context_receipt.go": "cmdContextReceiptVerify"}
	if err = sourceConsumers(v, consumers); err != nil {
		return nil, err
	}
	if err = sourceNoTranslations(a); err != nil {
		return nil, err
	}
	if err = add("ac-1", "closed-fd3-consumers", "verdi", v, []string{"cmd/verdi/context_execution.go", "cmd/verdi/context_mcp.go", "cmd/verdi/context_receipt.go"}, "Three declared FD3 consumer functions; no missing/additional direct opener call sites"); err != nil {
		return nil, err
	}
	if err = add("ac-1", "local-production-owner-routing", "atc", a, []string{"cmd/vatc/assembly.go", "internal/executor/owners.go", "internal/executor/ownerimpl.go", "internal/verdiproto/controller.go", "internal/verdiproto/owner_client.go"}, "Production AST references exclude bridge constructors/types/methods outside isolated compatibility owner_client.go"); err != nil {
		return nil, err
	}
	orderRules := []struct {
		tree          sourceTree
		path, fn      string
		order, guards []string
	}{
		{v, "internal/sealedexec/controller_client.go", "ControllerClient.invoke", []string{"EncodeControllerCall", "NewCall", "writeControllerFrame", "readControllerReply", "DecodeControllerResult", "NewReply"}, []string{"EncodeControllerCall", "NewCall", "DecodeControllerResult", "NewReply"}},
		{v, "internal/sealedexec/controller_owner_conversion.go", "encodePublicControllerCallPayload", []string{"controllerCallToOwner", "RequestArm"}, []string{"controllerCallToOwner"}},
		{a, "internal/verdiproto/controller.go", "Controller.answer", []string{"NewOwnerCall", "Observe", "Resolve", "NewOwnerReply", "succeed"}, []string{"NewOwnerCall", "NewOwnerReply"}},
		{a, "internal/verdiproto/controller.go", "Controller.Serve", []string{"readControllerFrame", "answer", "Write"}, nil},
		{a, "internal/executor/ownerimpl.go", "ownerGroup.Resolve", []string{"NewOwnerCall", "answer", "OwnerResultPayload"}, []string{"NewOwnerCall", "answer", "OwnerResultPayload"}},
		{a, "cmd/vatc/assembly.go", "assemble", []string{"Open", "Negotiate", "openDatabase", "bindClaimListener"}, []string{"Open", "Negotiate"}},
		{a, "cmd/vatc/assembly.go", "runController.Assemble", []string{"NewProductionOwners", "Controller", "OpenSealed"}, []string{"NewProductionOwners", "Controller", "OpenSealed"}},
		{a, "cmd/vatc/assembly.go", "assembleRun", []string{"assemble"}, []string{"assemble"}},
		{v, "internal/sealedexec/mcpcompile.go", "validateChildCompileRequest", []string{"validateDomainSnapshot"}, []string{"validateDomainSnapshot"}},
		{v, "internal/sealedexec/detail.go", "DetailProcessor.Resolve", []string{"canonicalDomainSegment"}, []string{"canonicalDomainSegment"}},
	}
	for _, r := range orderRules {
		u, e := r.tree.unit(r.path)
		if e != nil {
			return nil, e
		}
		if e = sourceOrdered(u, r.fn, r.order, r.guards); e != nil {
			return nil, e
		}
	}
	assemblyUnit, e := a.unit("cmd/vatc/assembly.go")
	if e != nil {
		return nil, e
	}
	for _, r := range []struct{ fn, call string }{{"assemble", "Negotiate"}, {"assembleRun", "assemble"}} {
		if e = sourceUnconditionalCall(assemblyUnit, r.fn, r.call); e != nil {
			return nil, e
		}
	}
	for _, r := range []struct {
		tree            sourceTree
		path, fn, limit string
	}{{v, "internal/sealedexec/controller_public.go", "readControllerReply", "OwnerFrameCeiling"}, {a, "internal/verdiproto/controller.go", "readControllerFrame", "MaxControllerFrameBytes"}} {
		u, e := r.tree.unit(r.path)
		if e != nil {
			return nil, e
		}
		if e = sourceBoundedRead(u, r.fn, r.limit); e != nil {
			return nil, e
		}
	}
	if err = add("ac-2", "verdi-admission-before-wire", "verdi", v, []string{"internal/sealedexec/controller_client.go", "internal/sealedexec/controller_public.go", "internal/sealedexec/controller_owner_conversion.go"}, "Guarded request validation precedes write; bounded reply read precedes decode and local relation validation"); err != nil {
		return nil, err
	}
	if err = add("ac-2", "atc-admission-before-observation", "atc", a, []string{"internal/verdiproto/controller.go", "internal/executor/ownerimpl.go"}, "Guarded NewOwnerCall before Observe and owner action; NewOwnerReply before success; incremental exclusive bound before append"); err != nil {
		return nil, err
	}
	for _, group := range []struct {
		tree                 sourceTree
		root, commit, prefix string
		dirs, files          []string
	}{
		{v, verdiDir, sourceVerdiBaseline, "verdi", []string{"internal/artifact/", "internal/store/", "internal/contextreceipt/", "internal/contextevent/"}, []string{"internal/sealedexec/codec.go", "internal/sealedexec/control_codec.go", "internal/sealedexec/schema.go", "internal/sealedexec/control_schema.go"}},
		{a, atcDir, sourceATCBaseline, "atc", []string{"internal/persistence/", "internal/recorder/", "internal/ledger/", "internal/board/", "internal/envelope/", "internal/protocol/"}, []string{"internal/executor/stores.go", "internal/executor/ownerstate.go", "internal/executor/codec.go", "internal/executor/schema.go", "internal/executor/continuity.go", "internal/executor/recovery.go", "internal/executor/executor.go"}},
	} {
		r, e := sourceFixedStorage(ctx, group.root, group.commit, group.prefix, group.tree, group.dirs, group.files)
		if e != nil {
			return nil, e
		}
		rows = append(rows, r)
	}
	durable := []struct {
		tree                 sourceTree
		root, commit, prefix string
		paths                []string
	}{{v, verdiDir, sourceVerdiBaseline, "verdi", []string{"internal/sealedexec/schema.go", "internal/sealedexec/control_schema.go", "internal/contextreceipt/schema.go", "internal/contextevent/schema.go"}}, {a, atcDir, sourceATCBaseline, "atc", []string{"internal/executor/schema.go", "internal/protocol/identity.go", "internal/envelope/schema.go"}}}
	for _, group := range durable {
		for _, p := range group.paths {
			before, e := sourceGitUnit(ctx, group.root, group.commit, p)
			if e != nil {
				return nil, e
			}
			bindings[group.prefix+"-baseline:"+group.commit+":"+p] = sourceHash(before.raw)
			after, e := group.tree.unit(p)
			if e != nil {
				return nil, e
			}
			if e = sourceSameDeclarations(before, after); e != nil {
				return nil, e
			}
		}
		if err = add("ac-3", "fixed-durable-declarations-"+group.prefix, group.prefix, group.tree, group.paths, "Type/constant/variable declaration ASTs equal fixed baseline "+group.commit); err != nil {
			return nil, err
		}
	}
	fixedFunctions := []struct {
		tree               sourceTree
		root, commit, path string
		names              []string
	}{
		{v, verdiDir, sourceVerdiBaseline, "internal/sealedexec/codec.go", []string{"ExecutionWorkspaceRequestDigest", "continuityDigest", "digestBytes"}},
		{v, verdiDir, sourceVerdiBaseline, "internal/contextreceipt/codec.go", []string{"receiptDigest", "verifyRequestDigest"}},
		{v, verdiDir, sourceVerdiBaseline, "internal/contextevent/codec.go", []string{"eventDigest", "EventPrefixDigest"}},
		{a, atcDir, sourceATCBaseline, "internal/executor/ownerimpl.go", []string{"ownerRuntime.receiptPosition", "ownerRuntime.commitControl"}},
		{a, atcDir, sourceATCBaseline, "internal/executor/recovery.go", []string{"Supervisor.reenter", "PayloadDigest", "sameLease"}},
	}
	for _, r := range fixedFunctions {
		before, e := sourceGitUnit(ctx, r.root, r.commit, r.path)
		if e != nil {
			return nil, e
		}
		prefix := "verdi"
		if r.root == atcDir {
			prefix = "atc"
		}
		bindings[prefix+"-baseline:"+r.commit+":"+r.path] = sourceHash(before.raw)
		after, e := r.tree.unit(r.path)
		if e != nil {
			return nil, e
		}
		bindings[prefix+":"+r.path] = sourceHash(after.raw)
		for _, name := range r.names {
			if e = sourceSameFunction(before, after, name); e != nil {
				return nil, e
			}
		}
	}
	if err = sourceRecoveryInventory(atcDir, a); err != nil {
		return nil, err
	}
	if err = add("ac-3", "separate-receipt-writes-and-five-cuts", "atc", a, []string{"internal/executor/ownerimpl.go", "internal/executor/executor.go", "internal/executor/recovery.go"}, "Fixed-baseline receiptPosition and commitControl preserve separate writes; five overlay cuts bind actual receipt/candidate statements; recovery classifier retained"); err != nil {
		return nil, err
	}
	rollout := filepath.Join(verdiDir, "docs/handoffs/public-execution-contract-rollout.md")
	raw, err := os.ReadFile(rollout)
	if err != nil {
		return nil, err
	}
	if sourceHash(raw) != sourceRolloutSHA {
		return nil, fmt.Errorf("approved rollout input changed")
	}
	if err = add("ac-4", "every-run-negotiation-before-effects", "atc", a, []string{"cmd/vatc/assembly.go", "internal/verdiproto/contract.go"}, "assembleRun invokes assemble; configured client negotiation is guarded before openDatabase; approved full rollout input SHA256 "+sourceRolloutSHA); err != nil {
		return nil, err
	}
	rows[len(rows)-1].Files["verdi:docs/handoffs/public-execution-contract-rollout.md"] = sourceHash(raw)
	raw, err = os.ReadFile(filepath.Join(verdiDir, "internal/sealedexec/testdata/public-consolidation/checks.json"))
	if err != nil {
		return nil, err
	}
	if sourceHash(raw) != sourceWitnessSHA {
		return nil, fmt.Errorf("accepted Task4 witness changed")
	}
	if err = sourceNoPrivateOwnerWire(v); err != nil {
		return nil, err
	}
	if err = sourceDependencyCheck(ctx, verdiDir, "github.com/jyang234/verdi-atc"); err != nil {
		return nil, err
	}
	if err = sourceDependencyCheck(ctx, atcDir, "github.com/jyang234/verdi"); err != nil {
		return nil, err
	}
	if err = add("ac-5", "sole-owner-wire-and-retained-domain", "verdi", v, []string{"internal/sealedexec/controller_codec.go", "internal/sealedexec/controller_domain_conversion.go", "internal/sealedexec/controller_owner_conversion.go", "internal/sealedexec/detail.go", "internal/sealedexec/mcpcompile.go", "internal/contextowner/fragments.go"}, "Closed framing/claims-only private wire declaration inventory; actual retained nontransport domain validator calls; go list dependency exclusion in both repositories; accepted Task4 witness SHA256 "+sourceWitnessSHA); err != nil {
		return nil, err
	}
	for _, r := range []struct{ prefix, root, path string }{
		{"verdi", verdiDir, "go.mod"}, {"verdi", verdiDir, "go.sum"}, {"atc", atcDir, "go.mod"}, {"atc", atcDir, "go.sum"},
		{"atc", atcDir, "cmd/vatc/boundary_recovery_instrumentation_test.go"},
		{"verdi", verdiDir, "internal/sealedexec/testdata/public-consolidation/checks.json"},
	} {
		b, e := os.ReadFile(filepath.Join(r.root, r.path))
		if e != nil {
			return nil, e
		}
		bindings[r.prefix+":"+r.path] = sourceHash(b)
	}
	// The shared bindings row covers inspected fixed-baseline identity bodies,
	// dependency inputs and the executable cut placement input without source text.
	rows = append(rows, SourceCheck{AC: "ac-3", Name: "fixed-baseline-and-replay-input-bindings", Files: bindings, Facts: []string{"Fixed baseline source and candidate identity function bodies inspected; recovery overlay source anchors inspected"}})
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].AC != rows[j].AC {
			return rows[i].AC < rows[j].AC
		}
		return rows[i].Name < rows[j].Name
	})
	return rows, nil
}

func sourceNoPrivateOwnerWire(tree sourceTree) error {
	for path, u := range tree {
		if strings.HasPrefix(path, "internal/contextowner/") {
			for _, imp := range u.file.Imports {
				p, e := strconv.Unquote(imp.Path.Value)
				if e != nil {
					return e
				}
				if strings.HasSuffix(p, "/internal/sealedexec") {
					return fmt.Errorf("public owner imports private execution domain %s", path)
				}
			}
		}
	}
	allowed := map[string]bool{"controllerContract": true, "controllerCallWire": true, "controllerResultWire": true, "controllerResultPayloadWire": true, "controllerErrorWire": true, "claimMCPQueryWire": true, "claimMCPRegistrationWire": true}
	for path, u := range tree {
		if !strings.HasPrefix(path, "internal/sealedexec/controller_") {
			continue
		}
		for name := range u.funcs {
			if name == "encodeControllerCallPayload" || name == "decodeControllerCallPayload" || name == "encodeControllerSuccessPayload" || name == "decodeControllerSuccessPayload" {
				return fmt.Errorf("parallel private operation codec %s:%s", path, name)
			}
		}
		var bad bool
		allowedStructs := map[*ast.StructType]bool{}
		ast.Inspect(u.file, func(n ast.Node) bool {
			if t, ok := n.(*ast.TypeSpec); ok && allowed[t.Name.Name] {
				if st, ok := t.Type.(*ast.StructType); ok {
					allowedStructs[st] = true
				}
			}
			return true
		})
		ast.Inspect(u.file, func(n ast.Node) bool {
			s, ok := n.(*ast.StructType)
			if !ok {
				return true
			}
			for _, f := range s.Fields.List {
				if f.Tag != nil && strings.Contains(f.Tag.Value, "json:") && !allowedStructs[s] && (path != "internal/sealedexec/controller_claims.go" || !sourceClaimsWrapper(u, s)) {
					bad = true
				}
			}
			return true
		})
		if bad {
			return fmt.Errorf("parallel serialized controller type in %s", path)
		}
	}
	return nil
}

func sourceRecoveryInventory(root string, tree sourceTree) error {
	u, err := tree.unit("internal/executor/ownerimpl.go")
	if err != nil {
		return err
	}
	if err = sourceOrdered(u, "ownerRuntime.appendReceipt", []string{"receiptPosition", "Marshal", "commitControl", "advanceRecorder"}, []string{"receiptPosition", "commitControl"}); err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(root, "cmd/vatc/boundary_recovery_instrumentation_test.go"))
	if err != nil {
		return err
	}
	return sourceRecoveryCuts(raw, tree)
}

func sourceRecoveryCuts(raw []byte, tree sourceTree) error {
	hooks, err := parseSourceUnit("boundary_recovery_instrumentation_test.go", raw)
	if err != nil {
		return err
	}
	cuts := []string{"before-receipt-event", "after-receipt-event", "after-receipt-control", "before-candidate-ledger", "after-candidate-ledger"}
	var declared []string
	for _, d := range hooks.file.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, s := range g.Specs {
			v, ok := s.(*ast.ValueSpec)
			if !ok || len(v.Names) != 1 || v.Names[0].Name != "recoveryCuts" || len(v.Values) != 1 {
				continue
			}
			a, ok := v.Values[0].(*ast.CompositeLit)
			if !ok {
				continue
			}
			for _, e := range a.Elts {
				b, ok := e.(*ast.BasicLit)
				if !ok {
					return fmt.Errorf("nonliteral recovery cut")
				}
				value, e := strconv.Unquote(b.Value)
				if e != nil {
					return e
				}
				declared = append(declared, value)
			}
		}
	}
	if !reflect.DeepEqual(cuts, declared) {
		return fmt.Errorf("five-cut inventory changed")
	}
	expected := map[string]struct{ path, fn string }{"before-receipt-event": {"internal/executor/ownerimpl.go", "ownerRuntime.receiptPosition"}, "after-receipt-event": {"internal/executor/ownerimpl.go", "ownerRuntime.appendReceipt"}, "after-receipt-control": {"internal/executor/ownerimpl.go", "ownerRuntime.appendReceipt"}, "before-candidate-ledger": {"internal/executor/executor.go", "Supervisor.terminal"}, "after-candidate-ledger": {"internal/executor/executor.go", "Supervisor.terminal"}}
	found := map[string]bool{}
	var problem error
	ast.Inspect(hooks.file, func(n ast.Node) bool {
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		key, ok := kv.Key.(*ast.BasicLit)
		if !ok {
			return true
		}
		path, e := strconv.Unquote(key.Value)
		if e != nil || tree[path] == nil {
			return true
		}
		pairs, ok := kv.Value.(*ast.CompositeLit)
		if !ok {
			return true
		}
		for _, element := range pairs.Elts {
			pair, ok := element.(*ast.CompositeLit)
			if !ok || len(pair.Elts) != 2 {
				continue
			}
			left, lok := pair.Elts[0].(*ast.BasicLit)
			right, rok := pair.Elts[1].(*ast.BasicLit)
			if !lok || !rok {
				continue
			}
			old, e := strconv.Unquote(left.Value)
			if e != nil {
				problem = e
				continue
			}
			replacement, e := strconv.Unquote(right.Value)
			if e != nil {
				problem = e
				continue
			}
			for cut, want := range expected {
				hook := "\ttask3Boundary(" + strconv.Quote(cut) + ")\n"
				if !strings.Contains(replacement, hook) {
					continue
				}
				if found[cut] || path != want.path || replacement != hook+old {
					problem = fmt.Errorf("recovery cut placement changed %s", cut)
					continue
				}
				source := tree[path]
				f, e := sourceFunction(source, want.fn)
				if e != nil {
					problem = e
					continue
				}
				body := string(source.raw[source.set.Position(f.Pos()).Offset:source.set.Position(f.End()).Offset])
				if strings.Count(body, old) != 1 {
					problem = fmt.Errorf("recovery cut anchor missing/ambiguous %s", cut)
					continue
				}
				found[cut] = true
			}
		}
		return true
	})
	if problem != nil {
		return problem
	}
	if len(found) != len(cuts) {
		return fmt.Errorf("recovery cut placement inventory incomplete")
	}
	return nil
}

func sourceValues(units ...*sourceUnit) map[string]ast.Expr {
	values := map[string]ast.Expr{}
	for _, u := range units {
		for _, d := range u.file.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, s := range g.Specs {
				v, ok := s.(*ast.ValueSpec)
				if !ok || len(v.Names) != len(v.Values) {
					continue
				}
				for i, n := range v.Names {
					values[n.Name] = v.Values[i]
				}
			}
		}
	}
	return values
}
func sourceLiteralRegistry(values map[string]ast.Expr, name string, want []string) error {
	lit, ok := values[name].(*ast.CompositeLit)
	if !ok {
		return fmt.Errorf("missing literal registry %s", name)
	}
	var got []string
	for _, e := range lit.Elts {
		if id, ok := e.(*ast.Ident); ok {
			e = values[id.Name]
		}
		b, ok := e.(*ast.BasicLit)
		if !ok {
			return fmt.Errorf("nonliteral registry entry %s", name)
		}
		v, err := strconv.Unquote(b.Value)
		if err != nil {
			return err
		}
		got = append(got, v)
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("exact operation registry changed %s", name)
	}
	return nil
}
func sourceLiteralConstants(u *sourceUnit, want map[string]string) error {
	values := sourceValues(u)
	for name, value := range want {
		e := values[name]
		if e == nil || sourceText(u.set, e) != value {
			return fmt.Errorf("contract constant changed %s:%s", u.path, name)
		}
	}
	return nil
}
func sourceRegistries(v, a sourceTree) error {
	operations := []string{"verify-authority", "resolve-profile", "verify-conflict", "resolve-recorder", "recorder-checkpoint", "recorder-append", "store-redacted-segment", "resolve-redacted-segment", "verify-opaque-boundary", "verify-provider-session", "verify-expansion", "store-adapter-session", "next-stamp", "resolve-context", "verify-epoch", "install-expansion", "resolve-receipt-inputs", "append-receipt", "resolve-receipt-verification-authority", "persist-handback", "persist-quarantine", "persist-abort", "resolve-claim-mcp"}
	vc, err := v.unit("internal/sealedexec/controller_schema.go")
	if err != nil {
		return err
	}
	vo, err := v.unit("internal/contextowner/schema.go")
	if err != nil {
		return err
	}
	ac, err := a.unit("internal/verdiproto/controller.go")
	if err != nil {
		return err
	}
	ao, err := a.unit("internal/verdiproto/owner_schema.go")
	if err != nil {
		return err
	}
	for _, r := range []struct {
		values map[string]ast.Expr
		name   string
		want   []string
	}{{sourceValues(vc), "controllerOperations", operations}, {sourceValues(vo), "operations", operations[:22]}, {sourceValues(ac), "operations", operations}, {sourceValues(ac, ao), "ownerOperations", operations[:22]}} {
		if err := sourceLiteralRegistry(r.values, r.name, r.want); err != nil {
			return err
		}
	}
	for _, r := range []struct {
		tree sourceTree
		path string
		want map[string]string
	}{
		{v, "internal/sealedexec/controller_schema.go", map[string]string{"ControllerCallSchemaID": `"verdi.context-controller-call/v2"`, "ControllerResultSchemaID": `"verdi.context-controller-result/v2"`, "ControllerErrorSchemaID": `"verdi.context-controller-error/v1"`, "ControllerContractSchemaID": `"verdi.context-controller-contract/v2"`}},
		{v, "internal/contextowner/schema.go", map[string]string{"armSchemaPrefix": `"verdi.context-owner/"`, "requestSchemaTag": `"-request/v1"`, "resultSchemaTag": `"-result/v1"`, "installRequestSchemaTag": `"-request/v2"`}},
		{v, "internal/contextowner/identity.go", map[string]string{"legacyRequestSchemaPrefix": `"verdi.context-controller/"`, "legacyRequestSchemaTag": `"-request/v1"`, "legacyInstallRequestSchemaTag": `"-request/v2"`}},
		{v, "internal/sealedexec/owner_bridge.go", map[string]string{"OwnerFrameCeiling": "32 << 20"}},
		{a, "internal/verdiproto/controller.go", map[string]string{"SchemaControllerCall": `"verdi.context-controller-call/v2"`, "SchemaControllerResult": `"verdi.context-controller-result/v2"`, "SchemaControllerError": `"verdi.context-controller-error/v1"`, "MaxControllerFrameBytes": "32 << 20"}},
		{a, "internal/verdiproto/owner_schema.go", map[string]string{"ownerArmSchemaPrefix": `"verdi.context-owner/"`, "ownerPrivateArmSchemaPrefix": `"verdi.context-controller/"`, "ownerRequestSchemaTag": `"-request/v1"`, "ownerResultSchemaTag": `"-result/v1"`, "ownerInstallRequestSchemaTag": `"-request/v2"`}},
	} {
		u, e := r.tree.unit(r.path)
		if e != nil {
			return e
		}
		if e = sourceLiteralConstants(u, r.want); e != nil {
			return e
		}
	}
	return nil
}

func sourceSameUnit(before, after *sourceUnit) error {
	if sourceText(before.set, before.file) != sourceText(after.set, after.file) {
		return fmt.Errorf("durable storage source changed %s", after.path)
	}
	return nil
}
func sourceStoragePath(path string, dirs, files []string) bool {
	if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.Contains(path, "/testdata/") {
		return false
	}
	for _, p := range files {
		if path == p {
			return true
		}
	}
	for _, d := range dirs {
		if strings.HasPrefix(path, d) {
			return true
		}
	}
	return false
}
func sourceFixedStorage(ctx context.Context, root, commit, prefix string, tree sourceTree, dirs, files []string) (SourceCheck, error) {
	row := SourceCheck{AC: "ac-3", Name: "closed-durable-storage-" + prefix, Files: map[string]string{}, Facts: []string{"Full production source AST equality to fixed baseline " + commit, "Storage ownership directories: " + strings.Join(dirs, ","), "Transport/controller arm declarations excluded; owning durable execution codecs retained"}}
	cmd := exec.CommandContext(ctx, "git", "ls-tree", "-r", "--name-only", commit)
	cmd.Dir = root
	b, err := cmd.Output()
	if err != nil {
		return row, err
	}
	wanted := map[string]bool{}
	for _, path := range strings.Split(string(b), "\n") {
		if sourceStoragePath(path, dirs, files) {
			wanted[path] = true
		}
	}
	for path := range tree {
		if sourceStoragePath(path, dirs, files) && !wanted[path] {
			return row, fmt.Errorf("added durable production source %s", path)
		}
	}
	for path := range wanted {
		before, e := sourceGitUnit(ctx, root, commit, path)
		if e != nil {
			return row, e
		}
		after, e := tree.unit(path)
		if e != nil {
			return row, e
		}
		if e = sourceSameUnit(before, after); e != nil {
			return row, e
		}
		row.Files[prefix+":"+path] = sourceHash(after.raw)
		row.Files[prefix+"-baseline:"+commit+":"+path] = sourceHash(before.raw)
	}
	if len(wanted) == 0 {
		return row, fmt.Errorf("empty fixed storage inventory")
	}
	row.Facts = append(row.Facts, fmt.Sprintf("%d production source units; additions/removals refused", len(wanted)))
	return row, nil
}

func sourceClaimsWrapper(u *sourceUnit, s *ast.StructType) bool {
	if len(s.Fields.List) != 2 {
		return false
	}
	a, b := s.Fields.List[0], s.Fields.List[1]
	if len(a.Names) != 1 || len(b.Names) != 1 || b.Names[0].Name != "Schema" || sourceText(u.set, b.Type) != "string" {
		return false
	}
	return a.Names[0].Name == "Query" && sourceText(u.set, a.Type) == "claimMCPQueryWire" || a.Names[0].Name == "Registration" && sourceText(u.set, a.Type) == "claimMCPRegistrationWire"
}

func sourceUnconditionalCall(u *sourceUnit, fn, name string) error {
	f, err := sourceFunction(u, fn)
	if err != nil {
		return err
	}
	count := 0
	for _, s := range f.Body.List {
		var node ast.Node
		switch v := s.(type) {
		case *ast.AssignStmt:
			node = v
		case *ast.ExprStmt:
			node = v
		case *ast.IfStmt:
			if v.Init != nil {
				node = v.Init
			}
		}
		if node == nil {
			continue
		}
		ast.Inspect(node, func(n ast.Node) bool {
			if _, ok := n.(*ast.FuncLit); ok {
				return false
			}
			if c, ok := n.(*ast.CallExpr); ok && (sourceCallName(c) == name || sourceText(u.set, c.Fun) == name) {
				count++
			}
			return true
		})
	}
	if count != 1 {
		return fmt.Errorf("unconditional per-run call missing/duplicated %s:%s:%s", u.path, fn, name)
	}
	return nil
}
