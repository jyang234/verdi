package workbench

import (
	"bytes"
	stdhtml "html"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/refindex"
)

// datesNow is the injected render clock every carrier case is decided
// against — never the wall clock.
var datesNow = time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

// daysBeforeNow renders the committer date days before datesNow.
func daysBeforeNow(days int) string {
	return datesNow.Add(-time.Duration(days) * 24 * time.Hour).Format("2006-01-02T15:04:05-07:00")
}

// dirEntryAttrs returns the attributes of name's rendered <li class=
// "dir-entry"> open tag, HTML-unescaped.
func dirEntryAttrs(t *testing.T, body, name string) map[string]string {
	t.Helper()
	re := regexp.MustCompile(`<li class="dir-entry[^"]*" data-testid="dir-entry-` + regexp.QuoteMeta(name) + `"[^>]*>`)
	tags := re.FindAllString(body, -1)
	if len(tags) != 1 {
		t.Fatalf("dir-entry open tags for %s = %d, want exactly 1 in: %s", name, len(tags), body)
	}
	attrs := map[string]string{}
	for _, m := range regexp.MustCompile(`([a-z-]+)="([^"]*)"`).FindAllStringSubmatch(tags[0], -1) {
		attrs[m[1]] = stdhtml.UnescapeString(m[2])
	}
	return attrs
}

// dateCarrierRe matches every SI-297 carrier attribute.
var dateCarrierRe = regexp.MustCompile(` data-(last-change|date-unproven|quiet)="[^"]*"`)

// TestWriteDirectorySection_DateCarriers is SI-297's render table: every
// entry's existing <li> carries data-last-change (its committer date) or
// data-date-unproven (the reason), and a drafts-in-progress entry with a
// readable date — and only such an entry — also carries data-quiet,
// decided against the injected now.
func TestWriteDirectorySection_DateCarriers(t *testing.T) {
	unreadable := disclosure.New("refindex:date-unreadable", "spec/lost-date", `last-change date unreadable: "git" said <no>`)
	unproven := disclosure.New("refindex:date-unproven", "spec/unproven-story", "last-change date unproven: this entry's effective lifecycle state is unproven")
	stateUnproven := disclosure.New("refindex:unproven-spec-state", "spec/unproven-story", "specstate could not prove it")
	noDraft := disclosure.New("refindex:no-draft-spec", "spec/empty-branch", "no spec.md yet")

	const absent = "\x00absent"
	tests := []struct {
		name string
		e    refindex.Entry
		// wantLastChange/wantUnproven/wantQuiet are each attribute's value,
		// or absent.
		wantLastChange string
		wantUnproven   string
		wantQuiet      string
	}{
		{
			name:           "design-branch draft 20 days old: dated and quiet",
			e:              refindex.Entry{Ref: "spec/old-draft", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Date: daysBeforeNow(20)},
			wantLastChange: daysBeforeNow(20), wantUnproven: absent, wantQuiet: "true",
		},
		{
			name:           "13 days: not quiet",
			e:              refindex.Entry{Ref: "spec/d13", Source: refindex.SourceRemote, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Date: daysBeforeNow(13)},
			wantLastChange: daysBeforeNow(13), wantUnproven: absent, wantQuiet: "false",
		},
		{
			name:           "exactly 14 days: not quiet (the boundary is exclusive)",
			e:              refindex.Entry{Ref: "spec/d14", Source: refindex.SourceBoth, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Date: daysBeforeNow(14)},
			wantLastChange: daysBeforeNow(14), wantUnproven: absent, wantQuiet: "false",
		},
		{
			name:           "15 days: quiet",
			e:              refindex.Entry{Ref: "spec/d15", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Date: daysBeforeNow(15)},
			wantLastChange: daysBeforeNow(15), wantUnproven: absent, wantQuiet: "true",
		},
		{
			name:           "default-branch entry grouped on the desk (a draft component): quiet decided too",
			e:              refindex.Entry{Ref: "spec/desk-component", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Date: daysBeforeNow(30)},
			wantLastChange: daysBeforeNow(30), wantUnproven: absent, wantQuiet: "true",
		},
		{
			name:           "degraded design entry (no draft spec) is still dated by its tip",
			e:              refindex.Entry{Ref: "spec/empty-branch", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, Disclosed: &noDraft, Date: daysBeforeNow(2)},
			wantLastChange: daysBeforeNow(2), wantUnproven: absent, wantQuiet: "false",
		},
		{
			name:           "active component: dated, never a quiet carrier",
			e:              refindex.Entry{Ref: "spec/live-component", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupActiveComponents, SpecStatus: "active", Date: daysBeforeNow(400)},
			wantLastChange: daysBeforeNow(400), wantUnproven: absent, wantQuiet: absent,
		},
		{
			name:           "accepted, pending build: dated, never a quiet carrier",
			e:              refindex.Entry{Ref: "spec/next-build", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupAcceptedPendingBuild, SpecStatus: "accepted-pending-build", Date: daysBeforeNow(90)},
			wantLastChange: daysBeforeNow(90), wantUnproven: absent, wantQuiet: absent,
		},
		{
			name:           "terminal: dated, never a quiet carrier",
			e:              refindex.Entry{Ref: "spec/settled", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupTerminal, SpecStatus: "closed", Date: daysBeforeNow(900)},
			wantLastChange: daysBeforeNow(900), wantUnproven: absent, wantQuiet: absent,
		},
		{
			name:           "a disclosed date: the reason, escaped, and no quiet carrier (absent means unproven)",
			e:              refindex.Entry{Ref: "spec/lost-date", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", DateDisclosed: &unreadable},
			wantLastChange: absent, wantUnproven: disclosure.Render(unreadable), wantQuiet: absent,
		},
		{
			name:           "an unproven default-branch entry on the desk: unproven date, never quiet",
			e:              refindex.Entry{Ref: "spec/unproven-story", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "unproven", Disclosed: &stateUnproven, DateDisclosed: &unproven},
			wantLastChange: absent, wantUnproven: disclosure.Render(unproven), wantQuiet: absent,
		},
		{
			name:           "no date and no disclosure (a canned entry): still an unproven reason, never silence",
			e:              refindex.Entry{Ref: "spec/undated", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft"},
			wantLastChange: absent, wantUnproven: "disclosed-unproven [workbench:date-unproven] spec/undated: no last-change date was computed for this entry", wantQuiet: absent,
		},
		{
			name:           "a date that is not a committer date: unproven, naming it",
			e:              refindex.Entry{Ref: "spec/garbled", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Date: "yesterday"},
			wantLastChange: absent, wantUnproven: `disclosed-unproven [workbench:date-unproven] spec/garbled: last-change date "yesterday" is not a committer date`, wantQuiet: absent,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writeDirectorySection(&buf, homeCards(t.TempDir(), []refindex.Entry{tt.e}, cardContext{now: datesNow}), nil, "", false, nil, datesNow, viewPipeline)
			attrs := dirEntryAttrs(t, buf.String(), strings.TrimPrefix(tt.e.Ref, "spec/"))
			for attr, want := range map[string]string{
				"data-last-change":   tt.wantLastChange,
				"data-date-unproven": tt.wantUnproven,
				"data-quiet":         tt.wantQuiet,
			} {
				got, present := attrs[attr]
				switch {
				case want == absent && present:
					t.Errorf("%s = %q, want the attribute absent", attr, got)
				case want != absent && !present:
					t.Errorf("%s absent, want %q", attr, want)
				case want != absent && got != want:
					t.Errorf("%s = %q, want %q", attr, got, want)
				}
			}
		})
	}
}

// ageChipRe matches every rendered age chip (spec/index-v2 ac-2; SI-366
// (13)) — the one visible mark the dates make on a card.
var ageChipRe = regexp.MustCompile(`<span class="dir-age[^"]*"(?: title="[^"]*")?>[^<]*</span>`)

// filterCountRe matches the quiet and disclosed pills' counts, which the
// dates decide (spec/index-v2 ac-3; indexfilters.go).
var filterCountRe = regexp.MustCompile(`(data-testid="dir-filter-(?:quiet|disclosed)"[^>]*>[a-z ]+<span class="count">)[0-9]+`)

// TestWriteDirectorySection_DateCarriersAddNothingVisible: the carriers,
// the age chip, the disclosed flag and the two filter counts over them
// (quiet and disclosed; spec/index-v2 ac-3) are the ONLY differences
// dates make to the directory's markup — no other text, class, or
// element changes (SI-297's carrier; spec/index-v2 ac-2's age, read from
// it and from nothing else).
func TestWriteDirectorySection_DateCarriersAddNothingVisible(t *testing.T) {
	dated := directoryFixtureEntries()
	for i := range dated {
		dated[i].Date = daysBeforeNow(i * 7)
	}
	undated := directoryFixtureEntries()
	root := t.TempDir()

	var withDates, withoutDates bytes.Buffer
	writeDirectorySection(&withDates, homeCards(root, dated, cardContext{now: datesNow}), nil, "", false, nil, datesNow, viewPipeline)
	writeDirectorySection(&withoutDates, homeCards(root, undated, cardContext{now: datesNow}), nil, "", false, nil, datesNow, viewPipeline)

	if !strings.Contains(withDates.String(), `data-last-change="`) || !strings.Contains(withDates.String(), `data-quiet="`) {
		t.Fatalf("the dated render carries no date carriers at all: %s", withDates.String())
	}
	if !strings.Contains(withDates.String(), `<span class="dir-age">today</span>`) || !strings.Contains(withDates.String(), `<span class="dir-age dir-age-quiet">quiet 21 d</span>`) {
		t.Fatalf("the dated render shows no ages: %s", withDates.String())
	}
	if !strings.Contains(withoutDates.String(), `>age unproven</span>`) {
		t.Fatalf("the undated render does not disclose its ages unproven: %s", withoutDates.String())
	}
	strip := func(s string) string {
		s = dateCarrierRe.ReplaceAllString(s, "")
		s = ageChipRe.ReplaceAllString(s, "")
		s = filterCountRe.ReplaceAllString(s, `${1}n`)
		return strings.ReplaceAll(s, ` data-disclosed="true"`, ` data-disclosed="false"`)
	}
	stripped, strippedUndated := strip(withDates.String()), strip(withoutDates.String())
	if stripped != strippedUndated {
		t.Fatalf("dates changed more than the carriers and the age chip:\nwith dates (stripped): %s\nwithout:               %s", stripped, strippedUndated)
	}
}

// TestRenderHome_DateCarriersUseTheInjectedClockOncePerRender: through the
// real handler, the carriers are decided against HomeDeps.Clock — read
// exactly once per render — never the wall clock.
func TestRenderHome_DateCarriersUseTheInjectedClockOncePerRender(t *testing.T) {
	entries := []refindex.Entry{
		{Ref: "spec/fresh-draft", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Date: daysBeforeNow(3)},
		{Ref: "spec/stale-draft", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Date: daysBeforeNow(21)},
	}
	calls := 0
	clock := func() time.Time {
		calls++
		return datesNow
	}
	for render := 1; render <= 2; render++ {
		code, body := getHome(t, t.TempDir(), HomeDeps{Index: cannedIndex(entries, nil), Git: fakeHomeGit{}, Clock: clock})
		if code != 200 {
			t.Fatalf("render %d: status %d", render, code)
		}
		if got := dirEntryAttrs(t, body, "fresh-draft")["data-quiet"]; got != "false" {
			t.Errorf("render %d: fresh-draft data-quiet = %q, want false against the injected clock (a wall-clock read would say true)", render, got)
		}
		if got := dirEntryAttrs(t, body, "stale-draft")["data-quiet"]; got != "true" {
			t.Errorf("render %d: stale-draft data-quiet = %q, want true", render, got)
		}
		if calls != render {
			t.Errorf("after render %d the clock was read %d times, want exactly once per render", render, calls)
		}
	}
}
