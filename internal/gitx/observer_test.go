package gitx

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
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
// false, and runStdin is the only producer of the "update-ref" token
// dc-4/ac-9's command-log check must see.
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
		{repo.Dir, "update-ref", ref, repo.Head, zeroOID},
	}
	if !reflect.DeepEqual(obs.calls, want) {
		t.Fatalf("observed %v, want %v", obs.calls, want)
	}
	if blobSHA == "" {
		t.Fatal("WriteBlob returned an empty SHA")
	}
}

// TestObserverCoversEveryExecSite is the structural guard the review
// required: it parses every non-test .go file in this package and asserts
// that the number of exec.Command/exec.CommandContext call sites equals
// the number of observe( call sites, so a fourth exec site cannot land
// unobserved without failing this test by construction (today: 3 and 3 —
// run in exec.go, ConfigValue in configvalue.go, runStdin in plumbing.go).
func TestObserverCoversEveryExecSite(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading package directory: %v", err)
	}

	var execSites, observeSites []string

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}

		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fun := call.Fun.(type) {
			case *ast.SelectorExpr:
				pkgIdent, ok := fun.X.(*ast.Ident)
				if ok && pkgIdent.Name == "exec" && (fun.Sel.Name == "Command" || fun.Sel.Name == "CommandContext") {
					execSites = append(execSites, name)
				}
			case *ast.Ident:
				if fun.Name == "observe" {
					observeSites = append(observeSites, name)
				}
			}
			return true
		})
	}

	if len(execSites) == 0 {
		t.Fatal("no exec.Command/exec.CommandContext call sites found at all — this guard would pass vacuously; the parser/AST walk itself is broken")
	}
	if len(execSites) != len(observeSites) {
		t.Fatalf("exec sites %v (%d) do not match observe( call sites %v (%d): every exec.Command/exec.CommandContext site must call observe first",
			execSites, len(execSites), observeSites, len(observeSites))
	}
}
