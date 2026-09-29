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
// TestIndex_DisclosuresCountSharesThePageEnumeration proves the budget
// half of SI-295 on a git-backed store: the index and /disclosures read
// one cached enumeration, so the second page runs no lint at all.
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
	indexCountRe    = regexp.MustCompile(`<p class="home-disclosures" data-disclosures-count="([0-9]+)">`)
	indexUnprovenRe = regexp.MustCompile(`<p class="home-disclosures" data-disclosures-unproven="([^"]*)">`)
	pageCountRe     = regexp.MustCompile(`<section class="disclosures-view" data-count="([0-9]+)">`)
)

// lintRuns counts lint enumerations through gitx's observer: every lint
// run starts with lint.BuildContext's `git symbolic-ref --short -q HEAD`.
type lintRuns struct {
	mu sync.Mutex
	n  int
}

func (l *lintRuns) Observe(_ string, args []string) {
	if strings.Join(args, " ") == "symbolic-ref --short -q HEAD" {
		l.mu.Lock()
		l.n++
		l.mu.Unlock()
	}
}

func (l *lintRuns) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.n
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
			ctx := gitx.WithObserver(context.Background(), runs)
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

// TestIndex_DisclosuresCountSharesThePageEnumeration: on a git-backed
// store whose inputs are older than the cache's racy window, the index and
// /disclosures read one enumeration — the second page runs no lint — and
// carry the same count, extras included.
func TestIndex_DisclosuresCountSharesThePageEnumeration(t *testing.T) {
	neutralizeCIEnv(t)
	repo := fixturegit.Build(t, []fixturegit.Layer{{Message: "seed the store", Files: map[string]string{
		".verdi/verdi.yaml": indexDisclosureManifestYAML,
		".verdi/.gitignore": "data/\n",
		".verdi/specs/active/index-panel-fixture/spec.md": indexDisclosureSpecMD,
	}}})
	setDefaultBranchSymref(t, repo.Dir)
	// Step past the cache's two-second racy window: a result read within
	// it is never stored.
	time.Sleep(2100 * time.Millisecond)

	extra := disclosure.New("mcp:review-feed", "", "forge configured but unreachable")
	runs := &lintRuns{}
	ctx := gitx.WithObserver(context.Background(), runs)
	h := NewHandlerWithHome(repo.Dir, Deps{Disclosures: []disclosure.Disclosure{extra}}, HomeDeps{Index: cannedIndex(nil, nil)})

	index := serveWith(t, h, ctx, "/")
	page := serveWith(t, h, ctx, "/disclosures")
	if index.Code != http.StatusOK || page.Code != http.StatusOK {
		t.Fatalf("GET / = %d, GET /disclosures = %d, want 200 and 200", index.Code, page.Code)
	}
	if got := runs.count(); got != 1 {
		t.Fatalf("lint ran %d times for the index and /disclosures, want 1 (one shared enumeration)", got)
	}
	countMatch := indexCountRe.FindStringSubmatch(index.Body.String())
	pageMatch := pageCountRe.FindStringSubmatch(page.Body.String())
	if countMatch == nil || pageMatch == nil || countMatch[1] != pageMatch[1] {
		t.Fatalf("index count %v, /disclosures count %v, want equal", countMatch, pageMatch)
	}
	want, err := disclosureview.Current(context.Background(), repo.Dir, extra)
	if err != nil {
		t.Fatal(err)
	}
	if countMatch[1] != strconv.Itoa(len(want)) {
		t.Fatalf("index count %s, fresh enumeration %d", countMatch[1], len(want))
	}
}
