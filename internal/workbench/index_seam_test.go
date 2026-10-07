package workbench

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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

// corpusBuildRe matches a reference to index.Build in Go source.
var corpusBuildRe = regexp.MustCompile(`\bindex\.Build\b`)

// TestRenderHome_CorpusBuiltOnlyThroughTheSeam: the home page's files
// reference index.Build in exactly one place — HomeDeps.resolve's
// production default for HomeDeps.Corpus — so the seam the call-count
// test watches is the only way a render can build the corpus (a direct
// second call would bypass it).
func TestRenderHome_CorpusBuiltOnlyThroughTheSeam(t *testing.T) {
	sites := 0
	for _, file := range []string{"index.go", "directory.go", "glance.go", "indexcards.go", "indexcardsread.go"} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("reading %s: %v", file, err)
		}
		for _, line := range strings.Split(string(src), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if corpusBuildRe.MatchString(line) {
				sites++
				if file != "directory.go" {
					t.Errorf("%s references index.Build outside HomeDeps.resolve: %q", file, strings.TrimSpace(line))
				}
			}
		}
	}
	if sites != 1 {
		t.Fatalf("index.Build references in the home page's files = %d, want exactly 1 (HomeDeps.resolve)", sites)
	}
}
