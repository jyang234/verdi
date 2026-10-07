package gitx

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

type recordingObserver struct{ calls [][]string }

func (r *recordingObserver) Observe(dir string, args []string) {
	r.calls = append(r.calls, append([]string{dir}, args...))
}

func TestObserver_SeesRunAndConfigValue(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "a\n"}, Message: "a"}})
	obs := &recordingObserver{}
	ctx := WithObserver(context.Background(), obs)
	if _, err := RevParse(ctx, repo.Dir, "HEAD"); err != nil {
		t.Fatal(err)
	}
	if _, err := ConfigValue(ctx, repo.Dir, "core.bare"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{repo.Dir, "rev-parse", "--verify", "HEAD"}, {repo.Dir, "config", "--local", "--get-all", "core.bare"}}
	if !reflect.DeepEqual(obs.calls, want) {
		t.Fatalf("observed %v, want %v", obs.calls, want)
	}
}

func TestObserver_AbsentIsNoop(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "a\n"}, Message: "a"}})
	if _, err := RevParse(context.Background(), repo.Dir, "HEAD"); err != nil {
		t.Fatal(err)
	}
	ctx := WithObserver(context.Background(), nil)
	if _, err := RevParse(ctx, repo.Dir, "HEAD"); err != nil {
		t.Fatal(err)
	}
}

// TestObserver_SeesPlumbing pins the third exec site, runStdin
// (plumbing.go), the one behind WriteBlob/BuildTreeWithFile/CommitTree/
// UpdateRef — R-RR3-2 amended after Task 1 review: the brief's original
// premise ("ConfigValue is the one exec site that bypasses run") was
// false. UpdateRef's argv is pinned here too: it creates the branch with
// `git branch <name> <commit>`, never update-ref (ritual-write-scope-v3
// dc-8, ledger SI-359 (5)).
func TestObserver_SeesPlumbing(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "a\n"}, Message: "a"}})
	obs := &recordingObserver{}
	ctx := WithObserver(context.Background(), obs)

	blobSHA, err := WriteBlob(ctx, repo.Dir, []byte("plumbing\n"))
	if err != nil {
		t.Fatal(err)
	}
	const ref = "refs/heads/plumbing-observed"
	if err := UpdateRef(ctx, repo.Dir, ref, repo.Head); err != nil {
		t.Fatal(err)
	}

	want := [][]string{
		{repo.Dir, "hash-object", "-w", "--stdin"},
		{repo.Dir, "branch", "plumbing-observed", repo.Head},
	}
	if !reflect.DeepEqual(obs.calls, want) {
		t.Fatalf("observed %v, want %v", obs.calls, want)
	}
	if blobSHA == "" {
		t.Fatal("WriteBlob returned an empty SHA")
	}
}

// TestObserver_BatchProcessObservedOnceAtStart pins the fourth exec site,
// the read session's batch process (startCatFileBatch), against the
// VERDI_GITLOG sink (ledger SI-359 (1), (3)). With a writable log, two
// rounds of a ref read and a blob read in one session record one
// `rev-parse --verify HEAD` (the replay runs no git) and one `cat-file
// --batch` (observed at its start, never per object name), and the
// observer sees the same two calls. With a log that cannot be opened, the
// batch start is refused before git runs, and every read in the session —
// which then takes the exec path — returns an error naming VERDI_GITLOG,
// with no git process started and no call observed.
func TestObserver_BatchProcessObservedOnceAtStart(t *testing.T) {
	for _, tt := range []struct {
		name     string
		logPath  func(t *testing.T) string
		wantFail bool
	}{
		{"a writable log", func(t *testing.T) string { return filepath.Join(t.TempDir(), "git.jsonl") }, false},
		{"a log that cannot be opened", func(t *testing.T) string { return t.TempDir() }, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := gitLogRepo(t)
			logPath := tt.logPath(t)
			t.Setenv(GitLogEnv, logPath)
			started := ""
			if tt.wantFail {
				started = fakeGitOnPath(t)
			}
			obs := &recordingObserver{}
			ctx, release := WithReadSession(WithObserver(context.Background(), obs), repo.Dir)
			defer release()

			var errs []error
			for range 2 {
				if _, err := RevParse(ctx, repo.Dir, "HEAD"); err != nil {
					errs = append(errs, err)
				}
				data, err := Show(ctx, repo.Dir, "HEAD", "a.txt")
				if err != nil {
					errs = append(errs, err)
				} else if string(data) != "a\n" {
					t.Fatalf("Show = %q, want %q", data, "a\n")
				}
			}

			if tt.wantFail {
				if len(errs) != 4 {
					t.Fatalf("%d of 4 reads failed (%v), want every read refused", len(errs), errs)
				}
				for _, err := range errs {
					if !strings.Contains(err.Error(), GitLogEnv) {
						t.Errorf("err = %v, want it to name %s", err, GitLogEnv)
					}
				}
				if b, err := startCatFileBatch(WithObserver(context.Background(), obs), repo.Dir); err == nil || b != nil || !strings.Contains(err.Error(), GitLogEnv) {
					t.Errorf("startCatFileBatch = %v, %v; want no process and an error naming %s", b, err, GitLogEnv)
				}
				if n := gitStarts(t, started); n != 0 {
					t.Errorf("git started %d time(s), want none before a failed record", n)
				}
				if len(obs.calls) != 0 {
					t.Errorf("the observer saw %v, want no call that never ran", obs.calls)
				}
				return
			}
			if len(errs) != 0 {
				t.Fatalf("reads failed: %v", errs)
			}
			argvs := [][]string{{"rev-parse", "--verify", "HEAD"}, {"cat-file", "--batch"}}
			var want []GitLogRecord
			var wantObserved [][]string
			for _, argv := range argvs {
				want = append(want, GitLogRecord{Args: argv, Dir: repo.Dir, PID: os.Getpid()})
				wantObserved = append(wantObserved, append([]string{repo.Dir}, argv...))
			}
			if got := readGitLog(t, logPath); !reflect.DeepEqual(got, want) {
				t.Fatalf("logged %+v, want %+v", got, want)
			}
			if !reflect.DeepEqual(obs.calls, wantObserved) {
				t.Fatalf("observed %v, want %v", obs.calls, wantObserved)
			}
		})
	}
}

// TestObserverCoversEveryExecSite is the structural guard the review
// required: it parses every non-test .go file in this package and asserts
// that the number of exec.Command/exec.CommandContext call sites equals
// the number of observe( call sites, so a fifth exec site cannot land
// unobserved without failing this test by construction (today: 4 and 4 —
// execGit in exec.go, ConfigValue in configvalue.go, runStdin in
// plumbing.go, and startCatFileBatch in readsession.go).
//
// It also holds every exec site to returning observe's error before git
// runs (execSiteFindings; R5c1 review R5C1R-4): a VERDI_GITLOG record that
// failed must refuse the execution (ledger SI-359 (3)), so `_ =
// observe(...)`, a bare observe call, or a guard that does not return the
// error each fail this test.
func TestObserverCoversEveryExecSite(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading package directory: %v", err)
	}

	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		files = append(files, f)
	}

	execSites, observeSites, findings := execSiteFindings(fset, files)
	if len(execSites) == 0 {
		t.Fatal("no exec.Command/exec.CommandContext call sites found at all — this guard would pass vacuously; the parser/AST walk itself is broken")
	}
	if len(execSites) != len(observeSites) {
		t.Fatalf("exec sites %v (%d) do not match observe( call sites %v (%d): every exec.Command/exec.CommandContext site must call observe first",
			execSites, len(execSites), observeSites, len(observeSites))
	}
	for _, f := range findings {
		t.Error(f)
	}
}

// execSiteFindings reads files' exec sites (exec.Command and
// exec.CommandContext calls) and observe call sites, each as file:line,
// and reports every way an exec site could run git without first returning
// observe's error: an observe call whose error is not returned (it is not
// the sole initializer of an `if err := observe(...); err != nil` whose
// body returns that error), and an exec call with no such guard earlier in
// the top-level statements of the function (or function literal) that
// holds it.
func execSiteFindings(fset *token.FileSet, files []*ast.File) (execSites, observeSites, findings []string) {
	at := func(n ast.Node) string {
		p := fset.Position(n.Pos())
		return fmt.Sprintf("%s:%d", p.Filename, p.Line)
	}
	for _, f := range files {
		guarded := map[*ast.CallExpr]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			if is, ok := n.(*ast.IfStmt); ok {
				if call := observeGuard(is); call != nil {
					guarded[call] = true
				}
			}
			call, ok := n.(*ast.CallExpr)
			switch {
			case !ok:
			case isExecCall(call):
				execSites = append(execSites, at(call))
			case isObserveCall(call):
				observeSites = append(observeSites, at(call))
			}
			return true
		})
		ast.Inspect(f, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok && isObserveCall(call) && !guarded[call] {
				findings = append(findings, fmt.Sprintf("%s: observe's error is not returned: it must be the sole initializer of `if err := observe(...); err != nil { return ..., err }`", at(call)))
			}
			var body *ast.BlockStmt
			switch fn := n.(type) {
			case *ast.FuncDecl:
				body = fn.Body
			case *ast.FuncLit:
				body = fn.Body
			}
			if body == nil {
				return true
			}
			guardSeen := false
			for _, stmt := range body.List {
				for _, call := range execCallsIn(stmt) {
					if !guardSeen {
						findings = append(findings, fmt.Sprintf("%s: the exec site runs git with no earlier `if err := observe(...); err != nil { return ..., err }` in its function", at(call)))
					}
				}
				if is, ok := stmt.(*ast.IfStmt); ok && observeGuard(is) != nil {
					guardSeen = true
				}
			}
			return true
		})
	}
	return execSites, observeSites, findings
}

// isExecCall reports a call of exec.Command or exec.CommandContext.
func isExecCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "exec" && (sel.Sel.Name == "Command" || sel.Sel.Name == "CommandContext")
}

// isObserveCall reports a call of observe.
func isObserveCall(call *ast.CallExpr) bool {
	id, ok := call.Fun.(*ast.Ident)
	return ok && id.Name == "observe"
}

// execCallsIn returns the exec calls in stmt outside any function literal
// it holds, which is judged as a function of its own.
func execCallsIn(stmt ast.Stmt) []*ast.CallExpr {
	var out []*ast.CallExpr
	ast.Inspect(stmt, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			if isExecCall(n) {
				out = append(out, n)
			}
		}
		return true
	})
	return out
}

// observeGuard returns the observe call is guards when is has the shape
// `if err := observe(...); err != nil { ...; return ..., <uses err> }`,
// or nil: its initializer binds one variable to observe's result alone,
// its condition is that variable != nil, and its body's own statements
// include a return whose last result uses the variable.
func observeGuard(is *ast.IfStmt) *ast.CallExpr {
	init, ok := is.Init.(*ast.AssignStmt)
	if !ok || len(init.Lhs) != 1 || len(init.Rhs) != 1 {
		return nil
	}
	errVar, ok := init.Lhs[0].(*ast.Ident)
	if !ok || errVar.Name == "_" {
		return nil
	}
	call, ok := init.Rhs[0].(*ast.CallExpr)
	if !ok || !isObserveCall(call) {
		return nil
	}
	cond, ok := is.Cond.(*ast.BinaryExpr)
	if !ok || cond.Op != token.NEQ {
		return nil
	}
	if x, ok := cond.X.(*ast.Ident); !ok || x.Name != errVar.Name {
		return nil
	}
	if y, ok := cond.Y.(*ast.Ident); !ok || y.Name != "nil" {
		return nil
	}
	for _, stmt := range is.Body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok || len(ret.Results) == 0 {
			continue
		}
		uses := false
		ast.Inspect(ret.Results[len(ret.Results)-1], func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == errVar.Name {
				uses = true
			}
			return !uses
		})
		if uses {
			return call
		}
	}
	return nil
}

// TestExecSiteFindings is execSiteFindings' table: the shape every gitx
// exec site has today passes, and every way of letting git run after a
// failed VERDI_GITLOG record is reported (R5c1 review R5C1R-4).
func TestExecSiteFindings(t *testing.T) {
	const header = "package gitx\n\nimport (\n\t\"context\"\n\t\"fmt\"\n\t\"os/exec\"\n)\n\n"
	tests := []struct {
		name string
		body string
		want string // a word of the one finding; "" for none
	}{
		{"the guarded shape", `func run(ctx context.Context, dir string, args ...string) error {
	if err := observe(ctx, dir, args); err != nil {
		return fmt.Errorf("gitx: %w", err)
	}
	return exec.CommandContext(ctx, "git", args...).Run()
}`, ""},
		{"a guard returning the bare error", `func run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if err := observe(ctx, dir, args); err != nil {
		return nil, err
	}
	return exec.CommandContext(ctx, "git", args...).Output()
}`, ""},
		{"the error discarded", `func run(ctx context.Context, dir string, args ...string) error {
	_ = observe(ctx, dir, args)
	return exec.CommandContext(ctx, "git", args...).Run()
}`, "observe's error is not returned"},
		{"a bare observe call", `func run(ctx context.Context, dir string, args ...string) error {
	observe(ctx, dir, args)
	return exec.CommandContext(ctx, "git", args...).Run()
}`, "observe's error is not returned"},
		{"a guard that does not return", `func run(ctx context.Context, dir string, args ...string) error {
	if err := observe(ctx, dir, args); err != nil {
		fmt.Println(err)
	}
	return exec.CommandContext(ctx, "git", args...).Run()
}`, "observe's error is not returned"},
		{"a guard returning another value", `func run(ctx context.Context, dir string, args ...string) error {
	if err := observe(ctx, dir, args); err != nil {
		return nil
	}
	return exec.CommandContext(ctx, "git", args...).Run()
}`, "observe's error is not returned"},
		{"the guard after the exec", `func run(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	if err := observe(ctx, dir, args); err != nil {
		return err
	}
	return cmd.Run()
}`, "no earlier"},
		{"the guard inside a branch", `func run(ctx context.Context, dir string, ok bool, args ...string) error {
	if ok {
		if err := observe(ctx, dir, args); err != nil {
			return err
		}
	}
	return exec.CommandContext(ctx, "git", args...).Run()
}`, "no earlier"},
		{"a literal's exec site guarded only outside it", `func run(ctx context.Context, dir string, args ...string) func() error {
	if err := observe(ctx, dir, args); err != nil {
		return func() error { return err }
	}
	return func() error { return exec.CommandContext(ctx, "git", args...).Run() }
}`, "no earlier"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "site.go", header+tt.body+"\n", 0)
			if err != nil {
				t.Fatal(err)
			}
			_, _, findings := execSiteFindings(fset, []*ast.File{f})
			got := strings.Join(findings, "\n")
			if tt.want == "" {
				if got != "" {
					t.Fatalf("findings:\n%s\nwant none", got)
				}
				return
			}
			if !strings.Contains(got, tt.want) {
				t.Fatalf("findings:\n%s\nwant one naming %q", got, tt.want)
			}
		})
	}
}
