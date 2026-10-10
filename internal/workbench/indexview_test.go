package workbench

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/refindex"
)

// The index's view (spec/index-v2 ac-6; SI-366 (6), (7); lane F7c).

// TestIndexViewOf: `view=list` selects the list view and everything else
// fails closed to the pipeline — the parameter absent, named, or a value
// the enum does not know — and each view's address is the one the
// toggle links.
func TestIndexViewOf(t *testing.T) {
	tests := []struct {
		query string
		want  indexView
	}{
		{"", viewPipeline},
		{"view=pipeline", viewPipeline},
		{"view=list", viewList},
		{"view=List", viewPipeline},
		{"view=grid", viewPipeline},
		{"view=", viewPipeline},
		{"filter=quiet", viewPipeline},
		{"view=list&view=pipeline", viewList},
	}
	for _, tt := range tests {
		t.Run("?"+tt.query, func(t *testing.T) {
			q, err := url.ParseQuery(tt.query)
			if err != nil {
				t.Fatal(err)
			}
			if got := indexViewOf(q); got != tt.want {
				t.Fatalf("indexViewOf(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
	if viewPipeline.href() != "/" || viewList.href() != "/?view=list" {
		t.Fatalf("hrefs = %q, %q; want / and /?view=list", viewPipeline.href(), viewList.href())
	}
	if got := indexView("").href(); got != "/" {
		t.Fatalf("the zero view's href = %q, want the pipeline's /", got)
	}
}

// TestWriteViewToggle: the toggle is two real links, Pipeline to the bare
// route and List to its query, exactly one marked current — the one the
// render draws — and nothing disabled: both reach a page without script.
func TestWriteViewToggle(t *testing.T) {
	tests := []struct {
		view        indexView
		wantCurrent string
	}{
		{viewPipeline, `<a class="topbar-view-pipeline" data-view="pipeline" href="/" aria-current="page">Pipeline</a><a class="topbar-view-list" data-view="list" href="/?view=list">List</a>`},
		{viewList, `<a class="topbar-view-pipeline" data-view="pipeline" href="/">Pipeline</a><a class="topbar-view-list" data-view="list" href="/?view=list" aria-current="page">List</a>`},
	}
	for _, tt := range tests {
		t.Run(string(tt.view), func(t *testing.T) {
			var b strings.Builder
			writeViewToggle(&b, tt.view)
			got := b.String()
			want := `<div class="topbar-view" data-testid="index-view-toggle" role="group" aria-label="View">` + tt.wantCurrent + `</div>`
			if got != want {
				t.Fatalf("toggle =\n%s\nwant\n%s", got, want)
			}
			if strings.Count(got, `aria-current="page"`) != 1 || strings.Contains(got, "disabled") {
				t.Fatalf("the toggle must mark exactly one view current and disable nothing; got: %s", got)
			}
		})
	}
}

// TestRenderHome_ViewFromTheQuery: GET / draws the pipeline and GET
// /?view=list the list, on the same DOM — the directory carrying the
// view as data-view, the toggle naming it current, every card once in
// either — and an unknown view draws the pipeline, saying so in the
// toggle rather than inventing a view. With the index failed, both views
// carry the one failure notice and no column.
func TestRenderHome_ViewFromTheQuery(t *testing.T) {
	root := t.TempDir()
	writeActiveSpec(t, root, "next-build", "feature", "accepted-pending-build", "")
	entries := []refindex.Entry{
		{Ref: "spec/next-build", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupAcceptedPendingBuild, SpecStatus: "accepted-pending-build", Zone: refindex.ZoneActive, Date: daysBeforeNow(3)},
		{Ref: "spec/local-draft", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Zone: refindex.ZoneActive, Title: "Local draft", Date: daysBeforeNow(2)},
	}
	h := NewHandlerWithHome(root, Deps{}, HomeDeps{Index: cannedIndex(entries, nil), Git: fakeHomeGit{}})
	failed := NewHandlerWithHome(root, Deps{}, HomeDeps{Index: cannedIndex(nil, errors.New("refindex: boom")), Git: fakeHomeGit{}})

	tests := []struct {
		path    string
		view    indexView
		current string
	}{
		{"/", viewPipeline, `href="/" aria-current="page">Pipeline</a>`},
		{"/?view=pipeline", viewPipeline, `href="/" aria-current="page">Pipeline</a>`},
		{"/?view=list", viewList, `href="/?view=list" aria-current="page">List</a>`},
		{"/?view=sideways", viewPipeline, `href="/" aria-current="page">Pipeline</a>`},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := serveWith(t, h, context.Background(), tt.path)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s = %d", tt.path, rec.Code)
			}
			body := rec.Body.String()
			if !strings.Contains(body, `<section class="home-directory" data-view="`+string(tt.view)+`">`) {
				t.Errorf("GET %s: the directory does not carry data-view=%q; got: %s", tt.path, tt.view, body)
			}
			if !strings.Contains(body, tt.current) || strings.Count(body, `aria-current="page"`) != 1 {
				t.Errorf("GET %s: the toggle does not name %s current exactly once; got: %s", tt.path, tt.view, body)
			}
			for _, name := range []string{"next-build", "local-draft"} {
				if n := strings.Count(body, `data-testid="dir-entry-`+name+`"`); n != 1 {
					t.Errorf("GET %s: card %s renders %d times, want exactly once", tt.path, name, n)
				}
			}
			if n := strings.Count(body, `class="dir-group `); n != 4 {
				t.Errorf("GET %s: %d columns, want the same four in either view", tt.path, n)
			}

			// The failure, in this view: one notice, no column, heading,
			// count, filter or card list (SI-366 (15), (21)(f)).
			rec = serveWith(t, failed, context.Background(), tt.path)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s on the failed index = %d, want 200 (never a dead end)", tt.path, rec.Code)
			}
			body = rec.Body.String()
			if n := strings.Count(body, `<p class="notice dir-index-failed">`); n != 1 {
				t.Errorf("GET %s on the failed index: %d failure notices, want exactly 1", tt.path, n)
			}
			if !strings.Contains(body, `<section class="home-directory" data-view="`+string(tt.view)+`"><p class="dir-provenance">`) {
				t.Errorf("GET %s on the failed index: the directory does not carry its view; got: %s", tt.path, body)
			}
			for _, partial := range []string{`class="dir-group`, `dir-filters`, `dir-cards`, `dir-entry`, `dir-group-head`, `class="dir-archived"`} {
				if strings.Contains(body, partial) {
					t.Errorf("GET %s on the failed index draws a partial group (%s); got: %s", tt.path, partial, body)
				}
			}
			directory := body[strings.Index(body, `class="home-directory"`):]
			directory = directory[:strings.Index(directory, `</section>`)]
			if strings.Contains(directory, `class="count"`) {
				t.Errorf("GET %s on the failed index draws a count inside the directory; got: %s", tt.path, directory)
			}
		})
	}
}
