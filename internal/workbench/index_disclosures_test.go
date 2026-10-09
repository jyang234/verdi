// TestIndex_DisclosuresCount (spec/index-coverage ac-3--behavioral)
// proves the served index carries the disclosures count from the SAME
// enumeration the disclosures page shows, computed once per render. One
// handler serves both pages with the process's own Deps.Disclosures, over
// fixture stores with lint disclosures, with none, with process
// disclosures only (B3-R1: a forge configured without credentials), and
// with an unenumerable root (B3-R5: the carrier then says unproven, never
// a count). Per render it asserts exactly one call of the index's
// enumeration seam (countDisclosures) and exactly one lint run (observed
// through gitx: lint.BuildContext's HEAD probe), so a second enumeration
// that bypasses the seam is caught too (B3-R2); it compares the served
// value with a fresh enumeration and with the page's own count.
//
// TestDisclosuresPage_EnumeratesEveryRender and
// TestDisclosuresPage_LeavesTheCacheAsItFoundIt prove SI-295's split on a
// git-backed store: the page computes fresh on every render and neither
// reads nor writes the cache; only the index reads and fills it.
//
// The carrier is non-visible markup on the index's existing Disclosures
// pointer: data-disclosures-count="<n>", or data-disclosures-unproven=
// "<reason>" with no count when the enumeration fails (SI-295).
package workbench

import (
	"context"
	stdhtml "html"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/disclosureview"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// indexDisclosureManifestYAML mirrors internal/disclosureview's own test
// manifest (a configured jira scheme so VL-005 has nothing to say about
// the fixture's story ref).
const indexDisclosureManifestYAML = `schema: verdi.layout/v1
forge: gitlab
providers:
  jira:
    base_url: https://example.atlassian.net
    rollup_field: customfield_00000
services:
  discovery: flowmap
`

// indexDisclosureSpecMD is a minimal, decodable new-class (story) spec —
// the shape VL-017's disclosed-unproven notice applies to when the
// mutable zone is absent (a bare clone).
const indexDisclosureSpecMD = `---
id: spec/index-panel-fixture
kind: spec
title: "Index Panel Fixture"
owners: [platform-team]
class: story
status: draft
story: jira:FIX-1
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a", evidence: [behavioral], anchor: "#ac-1" }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
---
# Index Panel Fixture

## Problem

p

## Outcome

o

## ac-1

a
`

// buildIndexDisclosureFixture writes a minimal store (no mutable zone —
// the bare-clone shape) whose lint run yields exactly one
// disclosure-severity finding (VL-017), for the "with a disclosure" case.
func buildIndexDisclosureFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	specDir := filepath.Join(root, ".verdi", "specs", "active", "index-panel-fixture")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(indexDisclosureSpecMD), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "verdi.yaml"), []byte(indexDisclosureManifestYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// buildIndexDisclosureFreeFixture is buildIndexDisclosureFixture with its
// mutable zone present, so VL-017 no longer fires (mirroring
// internal/disclosureview's TestCurrent_FreshPerCall) — the "with none"
// case.
func buildIndexDisclosureFreeFixture(t *testing.T) string {
	t.Helper()
	root := buildIndexDisclosureFixture(t)
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "data", "mutable"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

var (
	indexCountRe    = regexp.MustCompile(`<a class="home-disclosures" data-disclosures-count="([0-9]+)" href="/disclosures"`)
	indexUnprovenRe = regexp.MustCompile(`<a class="home-disclosures" data-disclosures-unproven="([^"]*)" href="/disclosures"`)
	pageCountRe     = regexp.MustCompile(`<section class="disclosures-view" data-count="([0-9]+)">`)
)

// lintRuns counts lint enumerations through gitx's observer: every lint
// run starts with lint.BuildContext's `git symbolic-ref --short -q HEAD`.
// So does every page's top bar (SI-323 (1)): its facts read the checkout's
// branch through gitState once per page render. The bar probe records
// those renders, and count subtracts the branch-level ones, leaving the
// lint runs alone.
type lintRuns struct {
	mu   sync.Mutex
	n    int
	bars barProbe
}

func (l *lintRuns) Observe(_ string, args []string) {
	if strings.Join(args, " ") == "symbolic-ref --short -q HEAD" {
		l.mu.Lock()
		l.n++
		l.mu.Unlock()
	}
}

// count is the lint runs: the branch reads observed, less one per
// branch-level bar the probe recorded (each built by exactly one gitState
// branch read).
func (l *lintRuns) count() int {
	bars := 0
	for _, f := range l.bars.facts() {
		if f.Spec == nil {
			bars++
		}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.n - bars
}

// context returns a request context that reports both the Git reads and
// the bar renders to l.
func (l *lintRuns) context() context.Context {
	return withBarProbe(gitx.WithObserver(context.Background(), l), &l.bars)
}

// countIndexEnumerations wraps the index's enumeration seam and returns
// its call count. Callers do not run in parallel.
func countIndexEnumerations(t *testing.T) *int {
	t.Helper()
	orig := countDisclosures
	calls := new(int)
	countDisclosures = func(ctx context.Context, root string, extras ...disclosure.Disclosure) (int, error) {
		*calls++
		return orig(ctx, root, extras...)
	}
	t.Cleanup(func() { countDisclosures = orig })
	return calls
}

func serveWith(t *testing.T, h http.Handler, ctx context.Context, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx))
	return rec
}

func TestIndex_DisclosuresCount(t *testing.T) {
	neutralizeCIEnv(t)
	noCredentials := disclosure.New("mcp:review-feed", "", `forge "gitlab" is configured (verdi.yaml) but no credentials are available to reach it; review state cannot be shown`)
	tests := []struct {
		name   string
		setup  func(t *testing.T) string
		extras []disclosure.Disclosure
		// want is the count the index must carry, or -1 for an
		// unenumerable store (the unproven carrier).
		want int
	}{
		{"a store with a lint disclosure", buildIndexDisclosureFixture, nil, 1},
		{"a store with none", buildIndexDisclosureFreeFixture, nil, 0},
		{"a store with only the process's own disclosure", buildIndexDisclosureFreeFixture, []disclosure.Disclosure{noCredentials}, 1},
		{"a store with a lint and a process disclosure", buildIndexDisclosureFixture, []disclosure.Disclosure{noCredentials}, 2},
		{"an unenumerable root (no .verdi)", func(t *testing.T) string { return t.TempDir() }, []disclosure.Disclosure{noCredentials}, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := tt.setup(t)
			calls := countIndexEnumerations(t)
			runs := &lintRuns{}
			ctx := runs.context()
			h := NewHandlerWithHome(root, Deps{Disclosures: tt.extras}, HomeDeps{Index: cannedIndex(nil, nil)})

			var body string
			for render := 1; render <= 2; render++ {
				rec := serveWith(t, h, ctx, "/")
				if rec.Code != http.StatusOK {
					t.Fatalf("GET / = %d, want 200\n%s", rec.Code, rec.Body.String())
				}
				body = rec.Body.String()
				if *calls != render {
					t.Fatalf("after %d renders the index enumerated %d times, want once per render", render, *calls)
				}
				if got := runs.count(); got != render {
					t.Fatalf("after %d renders lint ran %d times, want once per render (this store's cache key is uncomputable, so every render enumerates)", render, got)
				}
			}

			want, wantErr := disclosureview.Current(context.Background(), root, tt.extras...)
			page := serveWith(t, h, context.Background(), "/disclosures")
			countMatch := indexCountRe.FindStringSubmatch(body)
			unprovenMatch := indexUnprovenRe.FindStringSubmatch(body)

			if tt.want < 0 {
				if wantErr == nil {
					t.Fatal("fixture must be unenumerable")
				}
				if countMatch != nil {
					t.Fatalf("an unenumerable store carries a count %q; want no count, only the unproven reason", countMatch[0])
				}
				if unprovenMatch == nil || unprovenMatch[1] != stdhtml.EscapeString(wantErr.Error()) {
					t.Fatalf("unproven carrier = %v, want data-disclosures-unproven=%q\n%s", unprovenMatch, stdhtml.EscapeString(wantErr.Error()), body)
				}
				if page.Code != http.StatusInternalServerError {
					t.Fatalf("GET /disclosures = %d, want 500 for the same unenumerable store", page.Code)
				}
				return
			}

			if wantErr != nil {
				t.Fatalf("fresh enumeration: %v", wantErr)
			}
			if unprovenMatch != nil {
				t.Fatalf("an enumerable store carries %q", unprovenMatch[0])
			}
			if countMatch == nil {
				t.Fatalf("index carries no data-disclosures-count:\n%s", body)
			}
			pageMatch := pageCountRe.FindStringSubmatch(page.Body.String())
			if page.Code != http.StatusOK || pageMatch == nil {
				t.Fatalf("GET /disclosures = %d with no data-count:\n%s", page.Code, page.Body.String())
			}
			index, _ := strconv.Atoi(countMatch[1])
			pageCount, _ := strconv.Atoi(pageMatch[1])
			if index != len(want) || index != pageCount || index != tt.want {
				t.Fatalf("index count %d, /disclosures shows %d, fresh enumeration %d, want all %d", index, pageCount, len(want), tt.want)
			}
		})
	}
}

// quietGitStore builds a git-backed store and waits out the cache's
// two-second racy window: a result read within it is never stored.
func quietGitStore(t *testing.T) string {
	t.Helper()
	neutralizeCIEnv(t)
	repo := fixturegit.Build(t, []fixturegit.Layer{{Message: "seed the store", Files: map[string]string{
		".verdi/verdi.yaml": indexDisclosureManifestYAML,
		".verdi/.gitignore": "data/\n",
		".verdi/specs/active/index-panel-fixture/spec.md": indexDisclosureSpecMD,
	}}})
	setDefaultBranchSymref(t, repo.Dir)
	time.Sleep(2100 * time.Millisecond)
	return repo.Dir
}

// TestDisclosuresPage_EnumeratesEveryRender (SI-295 as corrected): the
// page computes fresh on every render (closed spec/disclosures-panel
// ac-1), even on an unchanged store whose enumeration the index has
// already cached.
func TestDisclosuresPage_EnumeratesEveryRender(t *testing.T) {
	tests := []struct {
		name  string
		store func(t *testing.T) string
	}{
		{"a quiet git-backed store the index has cached", quietGitStore},
		{"a store without git", func(t *testing.T) string {
			neutralizeCIEnv(t)
			return buildIndexDisclosureFixture(t)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := tt.store(t)
			runs := &lintRuns{}
			ctx := runs.context()
			h := NewHandlerWithHome(root, Deps{}, HomeDeps{Index: cannedIndex(nil, nil)})

			if rec := serveWith(t, h, ctx, "/"); rec.Code != http.StatusOK {
				t.Fatalf("GET / = %d", rec.Code)
			}
			if rec := serveWith(t, h, ctx, "/"); rec.Code != http.StatusOK {
				t.Fatalf("GET / = %d", rec.Code)
			}
			base := runs.count()
			for render := 1; render <= 3; render++ {
				rec := serveWith(t, h, ctx, "/disclosures")
				if rec.Code != http.StatusOK {
					t.Fatalf("GET /disclosures = %d", rec.Code)
				}
				if got := runs.count() - base; got != render {
					t.Fatalf("after %d page renders lint ran %d times, want once per render: the page never serves a cached value", render, got)
				}
			}
		})
	}
}

// TestDisclosuresPage_LeavesTheCacheAsItFoundIt (SI-295; closed
// spec/disclosures-panel: "no file, no cache, no log is written by
// rendering the view"): a page render neither fills nor replaces the
// index's cache — with no entry the index still enumerates for itself;
// with one, the index still reads it — and the index's count equals the
// page's for the same inputs.
func TestDisclosuresPage_LeavesTheCacheAsItFoundIt(t *testing.T) {
	tests := []struct {
		name string
		// indexFirst renders the index before the page, so the cache holds
		// an entry when the page renders.
		indexFirst bool
		// wantIndexRuns is how many times the index runs lint after the
		// page render.
		wantIndexRuns int
	}{
		{"no entry: the page does not fill the cache", false, 1},
		{"an entry: the page leaves it, the index still reads it", true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := quietGitStore(t)
			extra := disclosure.New("mcp:review-feed", "", "forge configured but unreachable")
			runs := &lintRuns{}
			ctx := runs.context()
			h := NewHandlerWithHome(root, Deps{Disclosures: []disclosure.Disclosure{extra}}, HomeDeps{Index: cannedIndex(nil, nil)})

			if tt.indexFirst {
				if rec := serveWith(t, h, ctx, "/"); rec.Code != http.StatusOK {
					t.Fatalf("GET / = %d", rec.Code)
				}
			}
			before := runs.count()
			page := serveWith(t, h, ctx, "/disclosures")
			if page.Code != http.StatusOK || runs.count()-before != 1 {
				t.Fatalf("GET /disclosures = %d with %d lint runs, want 200 and 1", page.Code, runs.count()-before)
			}
			before = runs.count()
			index := serveWith(t, h, ctx, "/")
			if index.Code != http.StatusOK {
				t.Fatalf("GET / = %d", index.Code)
			}
			if got := runs.count() - before; got != tt.wantIndexRuns {
				t.Fatalf("the index ran lint %d times after the page render, want %d", got, tt.wantIndexRuns)
			}

			countMatch := indexCountRe.FindStringSubmatch(index.Body.String())
			pageMatch := pageCountRe.FindStringSubmatch(page.Body.String())
			if countMatch == nil || pageMatch == nil || countMatch[1] != pageMatch[1] {
				t.Fatalf("index count %v, /disclosures count %v, want equal", countMatch, pageMatch)
			}
			want, err := disclosureview.Current(context.Background(), root, extra)
			if err != nil {
				t.Fatal(err)
			}
			if countMatch[1] != strconv.Itoa(len(want)) {
				t.Fatalf("index count %s, fresh enumeration %d", countMatch[1], len(want))
			}
		})
	}
}
