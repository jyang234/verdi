package workbench

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/refindex"
)

// seamFeatureSpecMD is an accepted-shape feature (statusless: its state is
// the canned index's) with two criteria, neither listed by a stub.
func seamFeatureSpecMD(name string) string {
	return fmt.Sprintf(`---
id: spec/%s
kind: spec
class: feature
title: "Seam %s"
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "one", evidence: [static] }
  - { id: ac-2, text: "two", evidence: [static] }
---
# Seam %s
`, name, name, name)
}

// seamStorySpecMD implements spec/seam-0#ac-1, so the first feature's
// call to action reads a story through the corpus's backlinks.
const seamStorySpecMD = `---
id: spec/seam-story
kind: spec
title: "Seam Story"
owners: [platform-team]
class: story
story: jira:SEAM-1
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a", evidence: [behavioral], anchor: "#ac-1" }
links:
  - { type: implements, ref: "spec/seam-0#ac-1" }
---
# Seam Story
`

// seamStore writes a store whose working tree carries the given number of
// accepted features (seam-0, seam-1, …) and one story implementing seam-0#ac-1,
// and returns its root with the canned index entries naming them.
func seamStore(t *testing.T, features int) (string, []refindex.Entry) {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".verdi/verdi.yaml", "schema: verdi.layout/v1\n")
	write(".verdi/specs/active/seam-story/spec.md", seamStorySpecMD)
	var entries []refindex.Entry
	for i := 0; i < features; i++ {
		name := fmt.Sprintf("seam-%d", i)
		write(".verdi/specs/active/"+name+"/spec.md", seamFeatureSpecMD(name))
		entries = append(entries, refindex.Entry{
			Ref: "spec/" + name, Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupAcceptedPendingBuild,
			SpecStatus: "accepted-pending-build", Zone: refindex.ZoneActive, Date: "2024-06-01T12:00:00+00:00",
		})
	}
	return root, entries
}

// gitCalls counts every git invocation gitx observes.
type gitCalls struct {
	mu sync.Mutex
	n  int
}

func (g *gitCalls) Observe(string, []string) {
	g.mu.Lock()
	g.n++
	g.mu.Unlock()
}

func (g *gitCalls) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.n
}

// TestRenderHome_OneComputationPerRender (spec/index-v2 ac-1, ac-4;
// SI-366 (12)): the page budget proven structurally. Every home render
// makes exactly one directory index computation (HomeDeps.Index), one
// corpus index build (HomeDeps.Corpus), and one disclosures count
// (countDisclosures), however many accepted features carry a call to
// action — and the cards add no git read of their own: a render whose
// index lists the features makes exactly as many git calls as one whose
// index lists nothing, over the same store.
func TestRenderHome_OneComputationPerRender(t *testing.T) {
	neutralizeCIEnv(t)
	for _, features := range []int{1, 3} {
		t.Run(fmt.Sprintf("%d accepted features", features), func(t *testing.T) {
			root, entries := seamStore(t, features)
			disclosureCalls := countIndexEnumerations(t)
			var indexCalls, corpusCalls int
			var corpusErr error
			home := HomeDeps{
				Index: func(context.Context) ([]refindex.Entry, error) {
					indexCalls++
					return entries, nil
				},
				Corpus: func(root string) (*index.Index, error) {
					corpusCalls++
					ix, err := index.Build(root)
					corpusErr = err
					return ix, err
				},
				Git:   fakeHomeGit{},
				Clock: func() time.Time { return time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC) },
			}
			h := NewHandlerWithHome(root, Deps{}, home)
			for render := 1; render <= 2; render++ {
				rec := serveWith(t, h, context.Background(), "/")
				if rec.Code != http.StatusOK {
					t.Fatalf("render %d: GET / = %d\n%s", render, rec.Code, rec.Body.String())
				}
				if indexCalls != render || corpusCalls != render || *disclosureCalls != render {
					t.Fatalf("after %d renders: %d index computations, %d corpus builds, %d disclosure counts; want %d of each", render, indexCalls, corpusCalls, *disclosureCalls, render)
				}
			}
			if corpusErr != nil {
				t.Fatalf("the fixture's corpus must build, so every call to action is computed: %v", corpusErr)
			}
		})
	}

	t.Run("the cards add no git read", func(t *testing.T) {
		root, entries := seamStore(t, 3)
		render := func(listed []refindex.Entry) int {
			calls := &gitCalls{}
			h := NewHandlerWithHome(root, Deps{}, HomeDeps{Index: cannedIndex(listed, nil), Git: fakeHomeGit{}})
			if rec := serveWith(t, h, gitx.WithObserver(context.Background(), calls), "/"); rec.Code != http.StatusOK {
				t.Fatalf("GET / = %d", rec.Code)
			}
			return calls.count()
		}
		render(nil) // warm whatever a first render fills, so both measured renders start alike
		without, with := render(nil), render(entries)
		t.Logf("git calls per render: %d listing none, %d listing %d accepted features", without, with, len(entries))
		if without == 0 {
			t.Fatal("a render made no git call the observer saw, so this comparison proves nothing: the observer is not wired to the render")
		}
		if with != without {
			t.Fatalf("a render listing %d accepted features made %d git calls, one listing none made %d: the cards must read no git", len(entries), with, without)
		}
	})
}

// corpusBuildAllowlist is every reference to index.Build this package's
// production files may carry, by "<file>:<function>" with its count: the
// home page's one seam (HomeDeps.resolve, the production default for
// HomeDeps.Corpus) and the other handlers' own per-request builds, none of
// which renderHome reaches.
func corpusBuildAllowlist() map[string]int {
	return map[string]int{
		"directory.go:HomeDeps.resolve":                     1,
		"corpus.go:corpusHandler":                           1,
		"boardpeek.go:peekFragment":                         1,
		"boardpin.go:boardSpecServer.actionPin":             1,
		"boardpin.go:boardSpecServer.boardPinSearchHandler": 1,
		"boardspec.go:boardSpecServer.loadBoard":            1,
	}
}

// corpusBuildSites counts every reference to internal/index's Build in
// this package's production (non-test) files, by "<file>:<function>" —
// "<file>:<package level>" outside any function. It reads the AST, so a
// comment never counts, and it follows the file's own import name.
func corpusBuildSites(t *testing.T) map[string]int {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	sites := map[string]int{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", file, err)
		}
		local := ""
		for _, imp := range f.Imports {
			if path, _ := strconv.Unquote(imp.Path.Value); path == "github.com/jyang234/verdi/internal/index" {
				local = "index"
				if imp.Name != nil {
					local = imp.Name.Name
				}
			}
		}
		switch local {
		case "":
			continue
		case ".", "_":
			t.Fatalf("%s imports internal/index as %q, which this guard cannot follow", file, local)
		}
		for _, decl := range f.Decls {
			scope := "package level"
			if fd, ok := decl.(*ast.FuncDecl); ok {
				scope = funcDeclName(fd)
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "Build" {
					if id, ok := sel.X.(*ast.Ident); ok && id.Name == local {
						sites[file+":"+scope]++
					}
				}
				return true
			})
		}
	}
	return sites
}

// funcDeclName is fd's name, prefixed with its receiver's type for a
// method ("boardSpecServer.loadBoard").
func funcDeclName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	typ := fd.Recv.List[0].Type
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}
	if id, ok := typ.(*ast.Ident); ok {
		return id.Name + "." + fd.Name.Name
	}
	return fd.Name.Name
}

// TestRenderHome_CorpusBuiltOnlyThroughTheSeam: across every production
// file of this package, index.Build is referenced exactly where the
// allowlist says — the home page's one seam (HomeDeps.resolve) and the
// other handlers' own builds — so the seam the call-count test watches is
// the only way a render can build the corpus. Any new reference anywhere,
// such as a second build reached from the cards through a helper in
// another file, fails here; so does an allowlisted site that is gone.
func TestRenderHome_CorpusBuiltOnlyThroughTheSeam(t *testing.T) {
	got, want := corpusBuildSites(t), corpusBuildAllowlist()
	for site, n := range got {
		if want[site] != n {
			t.Errorf("index.Build referenced %d time(s) in %s, allowlisted %d: a new corpus build bypasses HomeDeps.Corpus", n, site, want[site])
		}
	}
	for site, n := range want {
		if got[site] == 0 {
			t.Errorf("allowlisted index.Build site %s (%d) is gone: remove it from the allowlist", site, n)
		}
	}
}
