package workbench

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/refindex"
)

// columnBlock cuts one dir-group <section> out of a rendered body.
func columnBlock(t *testing.T, body string, g refindex.StatusGroup) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)<section class="dir-group[^"]*" data-testid="dir-group-` + string(g) + `">.*?</section>`)
	m := re.FindString(body)
	if m == "" {
		t.Fatalf("no dir-group block for %s in: %s", g, body)
	}
	return m
}

// TestWriteDirectorySection_FourColumns is ac-1's column witness: the four
// status groups render as columns in dc-2's order, each with its identity
// heading (SI-366 (18)), its count, its where line, its move line, and —
// for an empty column — an explicit empty state; the glance is gone.
func TestWriteDirectorySection_FourColumns(t *testing.T) {
	entries := []refindex.Entry{
		{Ref: "spec/local-draft", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Zone: refindex.ZoneActive, Title: "Local draft", Date: daysBeforeNow(3)},
		{Ref: "spec/remote-draft", Source: refindex.SourceRemote, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Zone: refindex.ZoneActive, Title: "Remote draft", Date: daysBeforeNow(20)},
		{Ref: "spec/settled-work", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupTerminal, SpecStatus: "closed", Zone: refindex.ZoneArchive, Date: daysBeforeNow(400)},
	}
	var buf bytes.Buffer
	writeDirectorySection(&buf, homeCards(t.TempDir(), entries, cardContext{now: datesNow}), nil, "", false, nil, datesNow)
	body := buf.String()

	if strings.Contains(body, "home-glance") || strings.Contains(body, "glance-group") {
		t.Fatalf("the glance must be retired (parent dc-12); got: %s", body)
	}
	if !strings.Contains(body, `<div class="dir-columns">`) {
		t.Fatalf("the four groups are not wrapped as one column set; got: %s", body)
	}

	// The columns, in order, each with its heading, count, where and move.
	type want struct {
		group        refindex.StatusGroup
		heading      string
		count        string
		where, move  string
		emptyOrCards string
	}
	wants := []want{
		{refindex.StatusGroupDraftsInProgress, "On the desk", "2", "any branch · draft", "A draft you can still write into. A merge of its branch, or an authored status on the default branch, moves one right.", `<ul class="dir-cards">`},
		{refindex.StatusGroupAcceptedPendingBuild, "Accepted", "0", "default branch · accepted-pending-build", "Filed on the default branch. Evidence lands as stories are built.", `<p class="empty dir-empty">Nothing accepted-pending-build yet.</p>`},
		{refindex.StatusGroupActiveComponents, "Active components", "0", "default branch · active", "Components whose obligations are being checked.", `<p class="empty dir-empty">No active components.</p>`},
		{refindex.StatusGroupTerminal, "On the shelf", "1", "default branch · terminal", "Superseded or closed. Read-only; kept for the record.", `<ul class="dir-cards">`},
	}
	last := -1
	for _, w := range wants {
		block := columnBlock(t, body, w.group)
		if i := strings.Index(body, block); i < last {
			t.Errorf("column %s renders out of dc-2 order", w.group)
		} else {
			last = i
		}
		head := `<h2>` + w.heading + ` <span class="count">` + w.count + `</span></h2>`
		if !strings.Contains(block, head) {
			t.Errorf("column %s missing its heading and count %s; got: %s", w.group, head, block)
		}
		if !strings.Contains(block, `<span class="dir-group-where">`+w.where+`</span>`) {
			t.Errorf("column %s missing its where line %q; got: %s", w.group, w.where, block)
		}
		if !strings.Contains(block, `<p class="dir-group-move">`+w.move+`</p>`) {
			t.Errorf("column %s missing its move line %q; got: %s", w.group, w.move, block)
		}
		if !strings.Contains(block, w.emptyOrCards) {
			t.Errorf("column %s missing %s; got: %s", w.group, w.emptyOrCards, block)
		}
		if w.count == "0" && strings.Contains(block, "dir-entry") {
			t.Errorf("an empty column rendered a card; got: %s", block)
		}
	}
	// The desk column's empty state is absent while it holds cards, and
	// the on-the-shelf one too: an empty state is never drawn beside cards.
	for _, g := range []refindex.StatusGroup{refindex.StatusGroupDraftsInProgress, refindex.StatusGroupTerminal} {
		if strings.Contains(columnBlock(t, body, g), "dir-empty") {
			t.Errorf("column %s draws an empty state beside its cards", g)
		}
	}
}

// TestWriteDirectorySection_EveryColumnEmpty: a store with no entries at
// all still draws all four columns, each with a zero count and its empty
// state (ac-1's explicit empty state), and no card.
func TestWriteDirectorySection_EveryColumnEmpty(t *testing.T) {
	var buf bytes.Buffer
	writeDirectorySection(&buf, nil, nil, "", false, nil, datesNow)
	body := buf.String()
	for _, g := range statusGroupOrder {
		block := columnBlock(t, body, g)
		if !strings.Contains(block, `<span class="count">0</span>`) || !strings.Contains(block, `class="empty dir-empty"`) {
			t.Errorf("empty column %s missing its zero count or empty state; got: %s", g, block)
		}
	}
	if strings.Contains(body, "dir-entry") {
		t.Fatalf("an empty index rendered a card; got: %s", body)
	}
}

// TestColumnCopyFor_Vocabulary: the column copy that speaks a class, state
// or verb word — in its where line, its move line and its empty state —
// routes through the model's display vocabulary (spec/vocabulary-surfaces;
// SI-366 (22)(a)), so a renaming store reads its own words everywhere.
func TestColumnCopyFor_Vocabulary(t *testing.T) {
	renamed := &model.Model{Vocabulary: model.Vocabulary{
		Classes: map[string]string{"story": "workstream"},
		States:  map[string]string{"draft": "sketch", "accepted-pending-build": "ready", "active": "live", "superseded": "retired", "closed": "done"},
		Verbs:   map[string]string{"merge": "land"},
	}}
	tests := []struct {
		group refindex.StatusGroup
		mdl   *model.Model
		want  columnCopy
	}{
		{refindex.StatusGroupDraftsInProgress, nil, columnCopy{"On the desk", "any branch · draft", "A draft you can still write into. A merge of its branch, or an authored status on the default branch, moves one right.", "Nothing on the desk."}},
		{refindex.StatusGroupDraftsInProgress, renamed, columnCopy{"On the desk", "any branch · sketch", "A sketch you can still write into. A land of its branch, or an authored status on the default branch, moves one right.", "Nothing on the desk."}},
		{refindex.StatusGroupAcceptedPendingBuild, nil, columnCopy{"Accepted", "default branch · accepted-pending-build", "Filed on the default branch. Evidence lands as stories are built.", "Nothing accepted-pending-build yet."}},
		{refindex.StatusGroupAcceptedPendingBuild, renamed, columnCopy{"Accepted", "default branch · ready", "Filed on the default branch. Evidence lands as workstreams are built.", "Nothing ready yet."}},
		{refindex.StatusGroupActiveComponents, nil, columnCopy{"Active components", "default branch · active", "Components whose obligations are being checked.", "No active components."}},
		{refindex.StatusGroupActiveComponents, renamed, columnCopy{"Active components", "default branch · live", "Components whose obligations are being checked.", "No live components."}},
		{refindex.StatusGroupTerminal, nil, columnCopy{"On the shelf", "default branch · terminal", "Superseded or closed. Read-only; kept for the record.", "Nothing on the shelf."}},
		{refindex.StatusGroupTerminal, renamed, columnCopy{"On the shelf", "default branch · terminal", "Retired or done. Read-only; kept for the record.", "Nothing on the shelf."}},
	}
	for _, tt := range tests {
		t.Run(string(tt.group)+"/"+tt.want.where, func(t *testing.T) {
			if got := columnCopyFor(tt.group, tt.mdl); got != tt.want {
				t.Fatalf("columnCopyFor = %+v, want %+v", got, tt.want)
			}
		})
	}
	// An unknown group fails closed to no copy at all, never an invented
	// heading.
	if got := columnCopyFor(refindex.StatusGroup("sideways"), nil); got != (columnCopy{}) {
		t.Fatalf("unknown group copy = %+v, want the zero value", got)
	}
}

// TestWriteDirectoryEntry_CardFacts is ac-2's card witness: each card shows
// its title, ref, status badge, working links, age, source, in-review
// chip, next move and disclosure — drawn from the card facts, nothing
// re-derived — and carries its review and disclosed facts as attributes
// after the date carriers.
func TestWriteDirectoryEntry_CardFacts(t *testing.T) {
	root := t.TempDir()
	writeActiveSpec(t, root, "next-build", "feature", "accepted-pending-build", "jira:X-1")
	writeActiveSpec(t, root, "storyless", "feature", "accepted-pending-build", "")
	writeActiveSpec(t, root, "live-component", "component", "active", "")
	writeActiveSpec(t, root, "old-way", "component", "superseded", "")
	noDraft := disclosure.New("refindex:no-draft-spec", "spec/uncharted-idea", `design branch "design/uncharted-idea" resolves but has no spec.md yet`)
	lostDate := disclosure.New("refindex:date-unreadable", "spec/lost-date", "last-change date unreadable: boom")
	successor := fakeBacklinks{"spec/old-way": {{From: "spec/live-component", Type: "superseded-by"}}}
	cc := cardContext{
		review: reviewConsultation{configured: true, inReview: map[string]bool{"design/in-review": true}},
		corpus: corpusRead{links: successor},
		now:    datesNow,
	}

	tests := []struct {
		name    string
		e       refindex.Entry
		want    []string
		wantNot []string
	}{
		{
			name: "a default-branch feature with a story: title link first, every link, sealed wall",
			e:    refindex.Entry{Ref: "spec/next-build", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupAcceptedPendingBuild, SpecStatus: "accepted-pending-build", Zone: refindex.ZoneActive, Date: daysBeforeNow(90)},
			want: []string{
				`data-source="default" data-last-change="` + daysBeforeNow(90) + `" data-review="not-open" data-disclosed="false">`,
				`<div class="dir-card-title"><a class="dir-title" href="/a/spec/next-build">Title of next-build</a></div>`,
				`<span class="dir-ref">spec/next-build</span>`,
				`<span class="badge badge-accepted-pending-build">accepted-pending-build</span>`,
				`<span class="badge badge-src badge-src-default">default branch</span>`,
				`<span class="dir-age">90 d ago</span>`,
				`<div class="dir-links"><a class="dir-board" href="/board/spec/next-build">board</a> <a href="/matrix/jira:X-1">matrix</a> <a href="/verdict/jira:X-1">verdict</a></div>`,
				`<div class="dir-move">&rarr; sealed wall</div>`,
			},
			wantNot: []string{"dir-inreview", "dir-disclosed", "dir-unproven"},
		},
		{
			name:    "a feature with no story: board link, never matrix or verdict",
			e:       refindex.Entry{Ref: "spec/storyless", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupAcceptedPendingBuild, SpecStatus: "accepted-pending-build", Zone: refindex.ZoneActive, Date: daysBeforeNow(1)},
			want:    []string{`<div class="dir-links"><a class="dir-board" href="/board/spec/storyless">board</a></div>`, `<span class="dir-age">1 d ago</span>`},
			wantNot: []string{"/matrix/", "/verdict/"},
		},
		{
			name:    "an active component: obligations, no matrix or verdict",
			e:       refindex.Entry{Ref: "spec/live-component", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupActiveComponents, SpecStatus: "active", Zone: refindex.ZoneActive, Date: daysBeforeNow(0)},
			want:    []string{`<span class="badge badge-active">active</span>`, `<span class="dir-age">today</span>`, `<div class="dir-move">&rarr; obligations</div>`},
			wantNot: []string{"/matrix/", "/verdict/"},
		},
		{
			name: "a superseded component in the active zone: see successor, linked",
			e:    refindex.Entry{Ref: "spec/old-way", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupTerminal, SpecStatus: "superseded", Zone: refindex.ZoneActive, Date: daysBeforeNow(300)},
			want: []string{`<div class="dir-move"><a href="/a/spec/live-component">&rarr; see successor</a></div>`},
		},
		{
			name:    "an archived spec: title and corpus link, no board link, no move",
			e:       refindex.Entry{Ref: "spec/settled-work", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupTerminal, SpecStatus: "closed", Zone: refindex.ZoneArchive, Date: daysBeforeNow(900)},
			want:    []string{`<a class="dir-title" href="/a/spec/settled-work">spec/settled-work</a>`, `<span class="badge badge-closed">closed</span>`, `<span class="dir-age">900 d ago</span>`},
			wantNot: []string{"dir-board", "dir-links", "dir-move"},
		},
		{
			name: "a titled local draft: the title is its board link, quiet age, open the wall",
			e:    refindex.Entry{Ref: "spec/local-draft", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Zone: refindex.ZoneActive, Title: "Local draft", Date: daysBeforeNow(20)},
			want: []string{
				`data-source="local" data-last-change="` + daysBeforeNow(20) + `" data-quiet="true" data-review="not-open" data-disclosed="false">`,
				`<div class="dir-card-title"><a class="dir-board dir-title" href="/b/design%2Flocal-draft/board/spec/local-draft">Local draft</a></div>`,
				`<span class="dir-ref">spec/local-draft</span>`,
				`<span class="badge badge-draft">draft</span>`,
				`<span class="badge badge-src badge-src-local">local branch</span>`,
				`<span class="dir-age dir-age-quiet">quiet 20 d</span>`,
				`<div class="dir-move">&rarr; open the wall</div>`,
			},
			wantNot: []string{"dir-inreview", "dir-links", "/a/spec/"},
		},
		{
			name: "a draft in review: the chip, awaiting merge, review open",
			e:    refindex.Entry{Ref: "spec/in-review", Source: refindex.SourceBoth, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Zone: refindex.ZoneActive, Title: "In review", Date: daysBeforeNow(2)},
			want: []string{
				`data-review="open" data-disclosed="false">`,
				`<span class="dir-age">2 d ago</span>`,
				`<span class="badge badge-open dir-inreview">in review</span>`,
				`<div class="dir-move">&rarr; awaiting merge</div>`,
			},
		},
		{
			name: "an untitled draft: the board link names the ref, and the title is disclosed unproven",
			e:    refindex.Entry{Ref: "spec/untitled", Source: refindex.SourceRemote, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Zone: refindex.ZoneActive, Date: daysBeforeNow(2)},
			want: []string{
				`data-review="not-open" data-disclosed="true">`,
				`<div class="dir-card-title"><a class="dir-board dir-title" href="/b/design%2Funtitled/board/spec/untitled">spec/untitled</a> <span class="dir-unproven dir-title-unproven" title="disclosed-unproven [workbench:title-unproven] spec/untitled: no title was decoded from this design branch&#39;s spec">title unproven</span></div>`,
			},
		},
		{
			name: "a branch with no draft spec: the ref, no link, its disclosure, inspect the branch",
			e:    refindex.Entry{Ref: "spec/uncharted-idea", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, Zone: refindex.ZoneActive, Disclosed: &noDraft, Date: daysBeforeNow(2)},
			want: []string{
				`<li class="dir-entry dir-entry-disclosed" data-testid="dir-entry-uncharted-idea" data-source="local" data-last-change="` + daysBeforeNow(2) + `" data-quiet="false" data-review="not-open" data-disclosed="true">`,
				`<div class="dir-card-title"><span class="dir-ref">spec/uncharted-idea</span></div>`,
				`<span class="badge badge-src badge-src-local">local branch</span>`,
				`<div class="dir-move">&rarr; inspect the branch</div>`,
				`<span class="dir-disclosed">disclosed-unproven [refindex:no-draft-spec] spec/uncharted-idea: design branch &#34;design/uncharted-idea&#34; resolves but has no spec.md yet</span>`,
			},
			wantNot: []string{"<a ", "href=", "dir-title", "badge badge-draft"},
		},
		{
			name: "an unreadable date: age unproven, disclosed, with the reason",
			e:    refindex.Entry{Ref: "spec/lost-date", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Zone: refindex.ZoneActive, Title: "Lost date", DateDisclosed: &lostDate},
			want: []string{
				`data-date-unproven="disclosed-unproven [refindex:date-unreadable] spec/lost-date: last-change date unreadable: boom" data-review="not-open" data-disclosed="true">`,
				`<span class="dir-age dir-age-unproven dir-unproven" title="disclosed-unproven [refindex:date-unreadable] spec/lost-date: last-change date unreadable: boom">age unproven</span>`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writeDirectorySection(&buf, homeCards(root, []refindex.Entry{tt.e}, cc), nil, "", true, nil, datesNow)
			block := entryBlock(t, buf.String(), strings.TrimPrefix(tt.e.Ref, "spec/"))
			for _, w := range tt.want {
				if !strings.Contains(block, w) {
					t.Errorf("card missing %s\ngot: %s", w, block)
				}
			}
			for _, w := range tt.wantNot {
				if strings.Contains(block, w) {
					t.Errorf("card must not carry %s\ngot: %s", w, block)
				}
			}
		})
	}
}

// TestWriteDirectoryEntry_StatusBadgeThroughDisplayState: the badge's
// visible word is the model's (SI-366 (18)) while its class keeps the id.
func TestWriteDirectoryEntry_StatusBadgeThroughDisplayState(t *testing.T) {
	root := t.TempDir()
	writeActiveSpec(t, root, "next-build", "feature", "accepted-pending-build", "")
	mdl := &model.Model{Vocabulary: model.Vocabulary{States: map[string]string{"accepted-pending-build": "Ready to build"}}}
	e := refindex.Entry{Ref: "spec/next-build", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupAcceptedPendingBuild, SpecStatus: "accepted-pending-build", Zone: refindex.ZoneActive, Date: daysBeforeNow(1)}
	var buf bytes.Buffer
	writeDirectorySection(&buf, homeCards(root, []refindex.Entry{e}, cardContext{now: datesNow, words: classWords{m: mdl}, corpus: corpusRead{links: fakeBacklinks{}}}), nil, "", false, mdl, datesNow)
	if !strings.Contains(buf.String(), `<span class="badge badge-accepted-pending-build">Ready to build</span>`) {
		t.Fatalf("badge does not speak the model's word; got: %s", buf.String())
	}
}

// TestWriteDirectoryEntry_SuccessorUnlinkedWithoutBacklink: a superseded
// spec whose successor the corpus does not name reads see successor with
// no link — never an invented address.
func TestWriteDirectoryEntry_SuccessorUnlinkedWithoutBacklink(t *testing.T) {
	root := t.TempDir()
	writeActiveSpec(t, root, "old-way", "component", "superseded", "")
	e := refindex.Entry{Ref: "spec/old-way", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupTerminal, SpecStatus: "superseded", Zone: refindex.ZoneActive, Date: daysBeforeNow(300)}
	var buf bytes.Buffer
	writeDirectorySection(&buf, homeCards(root, []refindex.Entry{e}, cardContext{now: datesNow, corpus: corpusRead{links: fakeBacklinks{}}}), nil, "", false, nil, datesNow)
	block := entryBlock(t, buf.String(), "old-way")
	if !strings.Contains(block, `<div class="dir-move">&rarr; see successor</div>`) {
		t.Fatalf("unlinked successor move missing; got: %s", block)
	}
}
