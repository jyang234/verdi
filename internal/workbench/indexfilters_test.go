package workbench

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/refindex"
)

// filterFixtureCards is a population spanning every filter: two quiet
// drafts (one of them in review), a fresh draft, a disclosed no-draft
// branch, and a default-branch spec that no filter but everything selects.
func filterFixtureCards(t *testing.T) []cardFacts {
	t.Helper()
	noDraft := disclosure.New("refindex:no-draft-spec", "spec/uncharted-idea", `design branch "design/uncharted-idea" resolves but has no spec.md yet`)
	entries := []refindex.Entry{
		{Ref: "spec/quiet-draft", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Zone: refindex.ZoneActive, Title: "Quiet", Date: daysBeforeNow(20)},
		{Ref: "spec/reviewed-draft", Source: refindex.SourceBoth, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Zone: refindex.ZoneActive, Title: "Reviewed", Date: daysBeforeNow(30)},
		{Ref: "spec/fresh-draft", Source: refindex.SourceRemote, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Zone: refindex.ZoneActive, Title: "Fresh", Date: daysBeforeNow(1)},
		{Ref: "spec/uncharted-idea", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, Zone: refindex.ZoneActive, Disclosed: &noDraft, Date: daysBeforeNow(2)},
		{Ref: "spec/live-component", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupActiveComponents, SpecStatus: "active", Zone: refindex.ZoneActive, Date: daysBeforeNow(40)},
	}
	return homeCards(t.TempDir(), entries, cardContext{
		review: reviewConsultation{configured: true, kind: ForgeGitLab, inReview: map[string][]string{"design/reviewed-draft": {"5"}}},
		now:    datesNow,
	})
}

// TestCountFilters: each pill's count is the number of cards whose own
// attribute the script will read (SI-366 (19), (21)(b)) — quiet from
// index-data's IsQuiet, in review from the forge consultation, disclosed
// from any card-level unproven fact — and everything counts them all.
func TestCountFilters(t *testing.T) {
	tests := []struct {
		name  string
		cards []cardFacts
		want  filterCounts
	}{
		{"no cards", nil, filterCounts{}},
		{"the fixture population", filterFixtureCards(t), filterCounts{everything: 5, quiet: 2, inReview: 1, disclosed: 1}},
		{
			"an unproven age and an unproven title both read disclosed",
			homeCards(t.TempDir(), []refindex.Entry{
				{Ref: "spec/lost-date", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Zone: refindex.ZoneActive, Title: "Lost"},
				{Ref: "spec/untitled", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Zone: refindex.ZoneActive, Date: daysBeforeNow(3)},
			}, cardContext{now: datesNow}),
			filterCounts{everything: 2, disclosed: 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countFilters(tt.cards); got != tt.want {
				t.Fatalf("countFilters = %+v, want %+v", got, tt.want)
			}
		})
	}
	// The counts agree with the attributes the cards carry: what the
	// script selects is what the pill counted.
	var buf bytes.Buffer
	for _, c := range filterFixtureCards(t) {
		writeDirectoryEntry(&buf, c, nil, datesNow)
	}
	body := buf.String()
	for attr, want := range map[string]int{`data-quiet="true"`: 2, `data-review="open"`: 1, `data-disclosed="true"`: 1} {
		if got := strings.Count(body, attr); got != want {
			t.Errorf("%s appears %d times, want %d (the pill's count)", attr, got, want)
		}
	}
}

// TestWriteFilterRow is ac-3's filter-row witness: four pills in order —
// everything pressed, the rest not — each with its count; the in-review
// pill reads its disabled disclosure with no number when the forge was
// unavailable (SI-366 (4)) or is unconfigured, never a zero.
func TestWriteFilterRow(t *testing.T) {
	cards := filterFixtureCards(t)
	pill := func(id, label string, count int, pressed string) string {
		return `<button type="button" class="dir-filter" data-filter="` + id + `" data-testid="dir-filter-` + id + `" aria-pressed="` + pressed + `" title="`
	}
	tests := []struct {
		name                     string
		configured, unavailable  bool
		wantInReview, wantAbsent string
	}{
		{"forge consulted", true, false, `data-testid="dir-filter-in-review" aria-pressed="false" title="the forge lists an open merge request from the branch">in review <span class="count">1</span></button>`, "unavailable"},
		{"forge unavailable", true, true, `<button type="button" class="dir-filter dir-filter-unavailable" data-filter="in-review" data-testid="dir-filter-in-review" aria-pressed="false" disabled title="the forge could not be consulted this render, so no review state is known for any branch">in review · unavailable</button>`, `in review <span`},
		{"no forge configured", false, false, `<button type="button" class="dir-filter dir-filter-unavailable" data-filter="in-review" data-testid="dir-filter-in-review" aria-pressed="false" disabled title="no forge is configured to consult, so no review state is known for any branch">in review · no forge configured</button>`, `in review <span`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writeFilterRow(&buf, cards, tt.configured, tt.unavailable)
			row := buf.String()
			if !strings.HasPrefix(row, `<div class="dir-filters" data-testid="dir-filters" role="group" aria-label="Show"><span class="dir-filters-label">Show</span>`) || !strings.HasSuffix(row, `</div>`) {
				t.Fatalf("filter row shape: %s", row)
			}
			for _, want := range []string{
				pill("everything", "everything", 5, "true") + `every card">everything <span class="count">5</span></button>`,
				pill("quiet", "quiet", 2, "false") + `on the desk for more than 14 days without a change">quiet <span class="count">2</span></button>`,
				tt.wantInReview,
				pill("disclosed", "disclosed", 1, "false") + `the card states an unproven fact">disclosed <span class="count">1</span></button>`,
			} {
				if !strings.Contains(row, want) {
					t.Errorf("filter row missing %s; got: %s", want, row)
				}
			}
			if strings.Contains(row, tt.wantAbsent) {
				t.Errorf("filter row must not carry %q; got: %s", tt.wantAbsent, row)
			}
			// The pills keep their order, and the disabled pill carries no digit.
			order := regexp.MustCompile(`data-filter="([a-z-]+)"`).FindAllStringSubmatch(row, -1)
			var ids []string
			for _, m := range order {
				ids = append(ids, m[1])
			}
			if strings.Join(ids, " ") != "everything quiet in-review disclosed" {
				t.Errorf("pill order = %v", ids)
			}
			inReview := regexp.MustCompile(`(?s)<button[^>]*data-filter="in-review"[^>]*>(.*?)</button>`).FindStringSubmatch(row)
			if inReview == nil {
				t.Fatalf("no in-review pill in: %s", row)
			}
			if (tt.unavailable || !tt.configured) && regexp.MustCompile(`[0-9]`).MatchString(inReview[1]) {
				t.Errorf("an in-review pill whose answer is unknown must show no number; got: %s", inReview[1])
			}
		})
	}
	// With no cards at all, every count is an honest zero and the row
	// still renders (the population is computed; it is empty).
	var buf bytes.Buffer
	writeFilterRow(&buf, nil, true, false)
	if got := strings.Count(buf.String(), `<span class="count">0</span>`); got != 4 {
		t.Errorf("empty population: %d zero counts, want 4; got: %s", got, buf.String())
	}
}

// TestRenderHome_FilterRowAndScript: the served index carries the filter
// row between the provenance line and the columns, loads its own script
// deferred, and — on an index failure — draws neither (SI-366 (15): no
// filter counts over a population that was not computed).
func TestRenderHome_FilterRowAndScript(t *testing.T) {
	root := t.TempDir()
	_, body := getHome(t, root, HomeDeps{Index: cannedIndex(directoryFixtureEntries(), nil), Git: fakeHomeGit{}})
	row := strings.Index(body, `<div class="dir-filters"`)
	columns := strings.Index(body, `<div class="dir-columns">`)
	if row < 0 || columns < 0 || row > columns {
		t.Fatalf("the filter row must precede the columns (row at %d, columns at %d); body: %s", row, columns, body)
	}
	if !strings.Contains(body, `<script src="/assets/index.js" defer></script>`) {
		t.Fatalf("the index must load its own deferred script; got: %s", body)
	}

	_, failed := getHome(t, root, HomeDeps{Index: cannedIndex(nil, errors.New("refindex: boom")), Git: fakeHomeGit{}})
	if strings.Contains(failed, "dir-filters") || strings.Contains(failed, "dir-columns") {
		t.Fatalf("an index failure must draw no filter row and no columns; got: %s", failed)
	}
}
