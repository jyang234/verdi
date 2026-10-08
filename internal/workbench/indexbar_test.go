package workbench

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/refindex"
)

// TestIndexBarControls is the bar's witness (SI-366 (5), (17)): the view
// toggle placeholder, the Disclosures link carrying the count both as its
// carrier and visibly, the import link under its pinned test id and text,
// and parent dc-9's New feature link to the import page with its CLI
// hint — the class word through the model's vocabulary.
func TestIndexBarControls(t *testing.T) {
	restore := countDisclosures
	t.Cleanup(func() { countDisclosures = restore })

	tests := []struct {
		name    string
		count   int
		err     error
		mdl     *model.Model
		want    []string
		wantNot []string
	}{
		{
			name:  "a counted enumeration, no renames",
			count: 4,
			want: []string{
				`<div class="topbar-view" data-testid="index-view-toggle" role="group" aria-label="View"><span class="topbar-view-current" aria-current="page">Pipeline</span><button type="button" class="topbar-view-list" disabled title="the list view is not built yet">List</button></div>`,
				`<a class="home-disclosures" data-disclosures-count="4" href="/disclosures" data-testid="home-disclosures" title="every claim this checkout is currently not proving, in one view">Disclosures <span class="count">4</span></a>`,
				`<a class="topbar-import" href="/design/import" data-testid="home-import-link" title="`,
				`">Import existing spec</a>`,
				`<a class="btn-primary home-new-feature" data-testid="home-new-feature" href="/design/import">New feature</a> <span class="topbar-cli-hint">or <code>verdi design start --kind feature --name &lt;name&gt;</code></span>`,
			},
			wantNot: []string{"data-disclosures-unproven", "count unproven"},
		},
		{
			name: "an enumeration that failed: the reason, never a zero",
			err:  errors.New("lint: boom"),
			want: []string{
				`<a class="home-disclosures" data-disclosures-unproven="lint: boom" href="/disclosures" data-testid="home-disclosures" title="every claim this checkout is currently not proving, in one view">Disclosures <span class="dir-unproven" title="lint: boom">count unproven</span></a>`,
			},
			wantNot: []string{"data-disclosures-count", `<span class="count">0</span>`},
		},
		{
			name:  "a renaming store: New <display word>",
			count: 0,
			mdl:   &model.Model{Vocabulary: model.Vocabulary{Classes: map[string]string{"feature": "Initiative"}}},
			want:  []string{`>New Initiative</a>`, `data-disclosures-count="0"`, `Disclosures <span class="count">0</span>`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			countDisclosures = func(context.Context, string, ...disclosure.Disclosure) (int, error) {
				calls++
				return tt.count, tt.err
			}
			got := string(indexBarControls(context.Background(), t.TempDir(), nil, classWords{m: tt.mdl}))
			if calls != 1 {
				t.Fatalf("countDisclosures called %d times per bar render, want 1", calls)
			}
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("bar controls missing %s; got: %s", w, got)
				}
			}
			for _, w := range tt.wantNot {
				if strings.Contains(got, w) {
					t.Errorf("bar controls must not carry %q; got: %s", w, got)
				}
			}
			// The handoff's order: view toggle, Disclosures, import, New feature.
			last := -1
			for _, marker := range []string{"index-view-toggle", "home-disclosures", "home-import-link", "home-new-feature"} {
				i := strings.Index(got, marker)
				if i < last {
					t.Errorf("%s is out of the bar's order", marker)
				}
				last = i
			}
		})
	}
}

// TestRenderHome_BarCarriesThePointers: the served index draws the
// controls in the bar's slot, ahead of the directory, and no longer draws
// the body's old pointer paragraphs — the import link and the Disclosures
// link each appear exactly once on the page.
func TestRenderHome_BarCarriesThePointers(t *testing.T) {
	root := t.TempDir()
	_, body := getHome(t, root, HomeDeps{Index: cannedIndex([]refindex.Entry{}, nil), Git: fakeHomeGit{}})
	slot := strings.Index(body, `<div class="topbar-controls" data-testid="topbar-controls">`)
	directory := strings.Index(body, `class="home-directory"`)
	if slot < 0 || directory < 0 || slot > directory {
		t.Fatalf("the bar's controls must precede the directory (slot at %d, directory at %d)", slot, directory)
	}
	for _, once := range []string{`data-testid="home-import-link"`, `class="home-disclosures"`, `data-testid="home-new-feature"`, `data-testid="index-view-toggle"`} {
		if got := strings.Count(body, once); got != 1 {
			t.Errorf("%s appears %d times, want exactly 1 (in the bar)", once, got)
		}
	}
	if strings.Contains(body, `<p class="home-disclosures"`) || strings.Contains(body, `<p class="home-import"`) {
		t.Fatalf("the body's old pointer paragraphs must be gone; got: %s", body)
	}
}
