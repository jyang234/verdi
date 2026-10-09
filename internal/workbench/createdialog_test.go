package workbench

import (
	stdhtml "html"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/designscaffold"
)

// The New story dialog's markup (spec/new-story-dialog-v2 ac-1, ac-2, co-1,
// dc-1; SI-369 (1)-(3), (6)-(8)): the header and its branch preview, the
// name's hint and grammar report, the gated Create and its status line,
// and one row per criterion carrying the wall chip's text and the story
// half — every word server-rendered, every class word through the store's
// vocabulary. newstorydialog.js only composes these attributes.

// dialogProjection is a sealed feature wall with criteria ac-1..ac-4 and an
// open question: ac-1 covered by one stub, ac-2 by a story only, ac-3 by
// nothing, and ac-4 by two stubs and two stories.
func dialogProjection() *BoardProjection {
	ac := string(boardlayout.ZoneAC)
	return &BoardProjection{
		Spec: "f", Class: "feature", Status: "accepted-pending-build", Mode: modeReadOnly,
		Cards: []cardView{
			{ID: "ac-1", Kind: ac, Text: "first criterion"},
			{ID: "ac-2", Kind: ac, Text: "second criterion"},
			{ID: "ac-3", Kind: ac, Text: "third criterion"},
			{ID: "ac-4", Kind: ac, Text: "fourth <criterion>"},
			{ID: "oq-1", Kind: string(boardlayout.ZoneOpenQuestion), Text: "an open question"},
		},
		CreateFields: []designscaffold.Field{
			{Name: "Ref", Kind: designscaffold.FieldIdentity},
			{Name: "Title", Kind: designscaffold.FieldInput},
			{Name: "Owners", Kind: designscaffold.FieldInput},
			{Name: "StoryRef", Kind: designscaffold.FieldInput},
			{Name: "Problem", Kind: designscaffold.FieldStatement},
			{Name: "Outcome", Kind: designscaffold.FieldStatement},
			{Name: "Links", Kind: designscaffold.FieldStructural},
		},
		CreateCoverage: createCoverageView{
			Criteria: []createCriterionView{
				{ID: "ac-1", Stubs: 1},
				{ID: "ac-2", Stories: []string{"spec/second"}},
				{ID: "ac-3", Uncovered: true},
				{ID: "ac-4", Stubs: 2, Stories: []string{"spec/fourth-a", "spec/fourth-b"}},
			},
			Uncovered: 1,
		},
	}
}

func renderCreateDialog(p *BoardProjection) string {
	var b strings.Builder
	writeCreateDialog(&b, p)
	return b.String()
}

// dialogVisibleText is the dialog's text content: its markup without tags,
// unescaped — what a reader sees, attributes excluded.
func dialogVisibleText(html string) string {
	return stdhtml.UnescapeString(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(html, " "))
}

// elementByID returns the opening tag and text of the element whose id is
// id (a leaf element: its text holds no tags).
func elementByID(t *testing.T, html, id string) (tag, text string) {
	t.Helper()
	m := regexp.MustCompile(`(<[a-z0-9]+[^>]* id="` + regexp.QuoteMeta(id) + `"[^>]*>)([^<]*)<`).FindStringSubmatch(html)
	if m == nil {
		t.Fatalf("the dialog has no #%s:\n%s", id, html)
	}
	return m[1], stdhtml.UnescapeString(m[2])
}

// attr is the unescaped value of the attribute name in tag, and whether it
// is present.
func attr(tag, name string) (string, bool) {
	m := regexp.MustCompile(` ` + regexp.QuoteMeta(name) + `(?:="([^"]*)")?[ >]`).FindStringSubmatch(tag)
	if m == nil {
		return "", false
	}
	return stdhtml.UnescapeString(m[1]), true
}

// TestCreateDialog_SpeaksTheStoresWords: the heading, the header's note,
// the Create button, the status line's first state and the criteria legend
// speak the store's story word (co-1; SI-369 (8)) — and a store that
// renames the class leaves no bare class word in the dialog's text.
func TestCreateDialog_SpeaksTheStoresWords(t *testing.T) {
	for _, tc := range []struct {
		name    string
		renamed bool
		word    string
	}{
		{"no rename", false, "story"},
		{"a renamed story class", true, "Change Request"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := dialogProjection()
			if tc.renamed {
				p.applyModelVocabulary(vocabTestModel())
			}
			html := renderCreateDialog(p)
			if n := strings.Count(html, "<h2"); n != 1 {
				t.Fatalf("the dialog renders %d headings, want exactly one", n)
			}
			for _, want := range []string{
				`<h2>New ` + tc.word + `</h2>`,
				`<p class="create-lede">Cuts a design branch carrying the ` + tc.word + ` spec. Nothing is written until you press Create.</p>`,
				`>Create ` + tc.word + `</button>`,
				`Implements · the acceptance criteria this ` + tc.word + ` claims, at least one`,
			} {
				if !strings.Contains(html, want) {
					t.Errorf("the dialog lacks %q:\n%s", want, html)
				}
			}
			tag, text := elementByID(t, html, "create-status")
			if want := "name the " + tc.word + " to continue"; text != want {
				t.Errorf("the status line reads %q, want %q", text, want)
			}
			if got, _ := attr(tag, "data-name"); got != "name the "+tc.word+" to continue" {
				t.Errorf("the status line's unnamed state is %q", got)
			}
			if tc.renamed {
				if bare := regexp.MustCompile(`\b(story|stories|spike|feature)\b`).FindString(dialogVisibleText(html)); bare != "" {
					t.Errorf("the renamed store's dialog still shows the bare class word %q", bare)
				}
			}
		})
	}
}

// TestCreateDialog_BranchPreviewStartsUnnamed: the "will cut" chip keeps its
// id and starts at design/… with the server's prefix as its template
// (SI-369 (6)); the name's hint is its description and carries the valid
// and the grammar-break texts, the latter quoting the server's pattern
// (SI-369 (5)); and the receipt names the branch through the same prefix,
// so no client composes it.
func TestCreateDialog_BranchPreviewStartsUnnamed(t *testing.T) {
	html := renderCreateDialog(dialogProjection())

	chip, text := elementByID(t, html, "create-branch-tab")
	if text != "design/…" {
		t.Errorf("the chip starts at %q, want design/…", text)
	}
	if got, _ := attr(chip, "data-cut"); got != designPrefix+"{name}" {
		t.Errorf("the chip's template is %q, want %q", got, designPrefix+"{name}")
	}
	if _, hidden := attr(chip, "aria-hidden"); hidden {
		t.Errorf("the chip is hidden from assistive technology: %s", chip)
	}
	if !strings.Contains(html, `<span class="create-cut-label">will cut</span>`) {
		t.Error("the chip has no \"will cut\" label")
	}

	input, _ := elementByID(t, html, "create-name")
	if got, _ := attr(input, "aria-describedby"); got != "create-name-hint" {
		t.Errorf("#create-name is described by %q, want create-name-hint", got)
	}
	if got, _ := attr(input, "data-pattern"); got != specNameRe.String() {
		t.Errorf("#create-name carries the pattern %q, want specNameRe's", got)
	}
	if _, invalid := attr(input, "aria-invalid"); invalid {
		t.Error("an empty name starts marked invalid")
	}

	hint, hintText := elementByID(t, html, "create-name-hint")
	for _, tc := range []struct{ attr, want string }{
		{"data-text-empty", "spec/<name> · design/<name>"},
		{"data-text-valid", "spec/{name} · design/{name}"},
		{"data-text-invalid", `"{name}" is not kebab-case; the grammar is ` + specNameRe.String()},
	} {
		if got, _ := attr(hint, tc.attr); got != tc.want {
			t.Errorf("the hint's %s is %q, want %q", tc.attr, got, tc.want)
		}
	}
	if hintText != "spec/<name> · design/<name>" {
		t.Errorf("the hint starts as %q, want its empty text", hintText)
	}

	dialog := regexp.MustCompile(`<div role="dialog"[^>]*>`).FindString(html)
	body, _ := attr(dialog, "data-receipt-body")
	if !strings.HasPrefix(body, "Branch design/{name} now carries spec/{name},") || strings.Contains(body, "{branch}") {
		t.Errorf("the receipt does not name the branch through the server's prefix: %q", body)
	}
}

// TestCreateDialog_CreateStartsGated: an empty name and no claim are
// today's state, so Create renders disabled and described by the status
// line, a polite status region opening on its first state and carrying
// the other states' copy (SI-369 (7)); the statements stay required, so an
// empty one is still refused on click; and the error slot keeps its id.
func TestCreateDialog_CreateStartsGated(t *testing.T) {
	html := renderCreateDialog(dialogProjection())

	ok, text := elementByID(t, html, "create-ok")
	if _, disabled := attr(ok, "disabled"); !disabled {
		t.Errorf("Create renders enabled before a name or a claim: %s", ok)
	}
	if got, _ := attr(ok, "aria-describedby"); got != "create-status" {
		t.Errorf("Create is described by %q, want create-status", got)
	}
	if text != "Create story" {
		t.Errorf("Create reads %q", text)
	}

	status, _ := elementByID(t, html, "create-status")
	for _, tc := range []struct{ attr, want string }{
		{"role", "status"},
		{"data-testid", "create-status"},
		{"data-acs", "claim at least one acceptance criterion"},
		{"data-ready", "cuts design/{name} · claims {acs}"},
		{"data-missing-one", "{fields} is required"},
		{"data-missing-many", "{fields} are required"},
	} {
		if got, _ := attr(status, tc.attr); got != tc.want {
			t.Errorf("the status line's %s is %q, want %q", tc.attr, got, tc.want)
		}
	}

	for _, field := range []string{"Problem", "Outcome"} {
		area, _ := elementByID(t, html, "create-field-"+field)
		if _, required := attr(area, "required"); !required || !strings.HasPrefix(area, "<textarea") {
			t.Errorf("%s is not a required statement: %s", field, area)
		}
		if got, _ := attr(area, "data-label"); got != field {
			t.Errorf("%s's data-label is %q", field, got)
		}
	}
	for _, field := range []string{"Title", "Owners", "StoryRef"} {
		in, _ := elementByID(t, html, "create-field-"+field)
		if _, required := attr(in, "required"); required {
			t.Errorf("the input %s is required: %s", field, in)
		}
	}
	if !strings.Contains(html, `<p class="create-error" id="create-error" data-testid="create-error" role="alert" hidden></p>`) {
		t.Error("the error slot changed")
	}
	if !strings.Contains(html, `id="create-cancel"`) {
		t.Error("the dialog lost #create-cancel")
	}
}

// dialogRow is one criterion row as rendered.
type dialogRow struct {
	state, input, cov, covClass, note, noteTitle string
	hasNote                                      bool
}

// dialogRows parses the dialog's criterion rows, keyed by the checkbox's
// data-create-ac.
func dialogRows(t *testing.T, html string) (map[string]dialogRow, []string) {
	t.Helper()
	rows := map[string]dialogRow{}
	var order []string
	for _, label := range regexp.MustCompile(`<label class="create-ac"[^>]*>.*?</label>`).FindAllString(html, -1) {
		open := regexp.MustCompile(`^<label[^>]*>`).FindString(label)
		input := regexp.MustCompile(`<input[^>]*>`).FindString(label)
		id, _ := attr(input, "data-create-ac")
		var r dialogRow
		r.state, _ = attr(open, "data-state")
		r.input = input
		if m := regexp.MustCompile(`(<span class="(create-ac-cov[^"]*)"[^>]*>)([^<]*)</span>`).FindStringSubmatch(label); m != nil {
			r.covClass, r.cov = m[2], stdhtml.UnescapeString(m[3])
			if got, _ := attr(m[1], "data-testid"); got != "create-coverage-"+id {
				t.Errorf("%s: the coverage text's testid is %q", id, got)
			}
		}
		if m := regexp.MustCompile(`(<span class="create-ac-note"[^>]*>)([^<]*)</span>`).FindStringSubmatch(label); m != nil {
			r.hasNote, r.note = true, stdhtml.UnescapeString(m[2])
			r.noteTitle, _ = attr(m[1], "title")
			if got, _ := attr(m[1], "data-testid"); got != "create-claims-"+id {
				t.Errorf("%s: the story-half note's testid is %q", id, got)
			}
		}
		rows[id] = r
		order = append(order, id)
	}
	return rows, order
}

// TestCreateDialog_CriteriaRows: one row per declared criterion, in order,
// and none for another card kind; each row's checkbox alone carries
// data-create-ac (F7's opener and 96 count the inputs); the coverage text
// is coverageChipText's — the wall chip's own words — under the dialog's
// own testid, never the wall's coverage-<id>; the story half is a separate
// note naming the claim count with the refs in its title, "unclaimed" for
// an uncovered criterion, and "coverage unproven" for a disclosed one
// (SI-369 (1)-(3)); and the row's state rides a server attribute.
func TestCreateDialog_CriteriaRows(t *testing.T) {
	disclosed := dialogProjection()
	disclosed.CreateCoverage = createCoverageView{
		Criteria: []createCriterionView{
			{ID: "ac-1", Stubs: 1, Disclosed: []string{"the corpus index could not be built: duplicate ref"}},
			{ID: "ac-2", Disclosed: []string{"the corpus index could not be built: duplicate ref"}},
			{ID: "ac-3", Stories: []string{"spec/third"}, Disclosed: []string{"a backlink could not be read"}},
		},
		Unproven: true,
	}
	for _, tc := range []struct {
		name string
		p    *BoardProjection
		want map[string]dialogRow
	}{
		{
			name: "stub, story, neither, and both",
			p:    dialogProjection(),
			want: map[string]dialogRow{
				"ac-1": {state: "covered", cov: "covered by 1 stub", covClass: "create-ac-cov create-ac-cov--covered"},
				"ac-2": {state: "covered", cov: "no stub", covClass: "create-ac-cov create-ac-cov--none", hasNote: true, note: "claimed by 1 story", noteTitle: "spec/second"},
				"ac-3": {state: "uncovered", cov: "no stub", covClass: "create-ac-cov create-ac-cov--none", hasNote: true, note: "unclaimed"},
				"ac-4": {state: "covered", cov: "covered by 2 stubs", covClass: "create-ac-cov create-ac-cov--covered", hasNote: true, note: "claimed by 2 stories", noteTitle: "spec/fourth-a, spec/fourth-b"},
			},
		},
		{
			name: "disclosed rows, and a criterion with no coverage row at all",
			p:    disclosed,
			want: map[string]dialogRow{
				"ac-1": {state: "disclosed", cov: "covered by 1 stub", covClass: "create-ac-cov create-ac-cov--covered", hasNote: true, note: "coverage unproven", noteTitle: "the corpus index could not be built: duplicate ref"},
				"ac-2": {state: "disclosed", cov: "no stub", covClass: "create-ac-cov create-ac-cov--none", hasNote: true, note: "coverage unproven", noteTitle: "the corpus index could not be built: duplicate ref"},
				"ac-3": {state: "disclosed", cov: "no stub", covClass: "create-ac-cov create-ac-cov--none", hasNote: true, note: "claimed by 1 story · coverage unproven", noteTitle: "spec/third; a backlink could not be read"},
				"ac-4": {state: "disclosed", cov: "no stub", covClass: "create-ac-cov create-ac-cov--none", hasNote: true, note: "coverage unproven", noteTitle: "no coverage was computed for this criterion"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			html := renderCreateDialog(tc.p)
			rows, order := dialogRows(t, html)
			if strings.Join(order, ",") != "ac-1,ac-2,ac-3,ac-4" {
				t.Fatalf("the dialog lists %v, want ac-1..ac-4 in declared order and no other card", order)
			}
			if n := strings.Count(html, "data-create-ac="); n != len(order) {
				t.Errorf("data-create-ac appears %d times, want once per criterion (on its checkbox only)", n)
			}
			if strings.Contains(html, `data-testid="coverage-`) {
				t.Error("the dialog reuses the wall chip's coverage-<id> testid")
			}
			for _, id := range order {
				got, want := rows[id], tc.want[id]
				if !strings.HasPrefix(got.input, `<input type="checkbox" data-create-ac="`+id+`" data-testid="create-ac-`+id+`"`) {
					t.Errorf("%s: the checkbox is %s", id, got.input)
				}
				got.input = ""
				if got != want {
					t.Errorf("%s: row\n%+v\nwant\n%+v", id, got, want)
				}
			}
			for _, c := range tc.p.CreateCoverage.Criteria {
				if rows[c.ID].cov != coverageChipText(c.Stubs) {
					t.Errorf("%s: the dialog's coverage text %q is not the wall chip's %q", c.ID, rows[c.ID].cov, coverageChipText(c.Stubs))
				}
			}
			if !strings.Contains(html, `<span class="create-ac-text" title="fourth &lt;criterion&gt;">fourth &lt;criterion&gt;</span>`) {
				t.Error("a criterion's text is not escaped, or carries no full-text title")
			}
		})
	}
}

// TestCreateDialog_StoryNoteVocabulary: the story half names the claim
// count in the store's word, singular and plural (co-1; SI-369 (2)).
func TestCreateDialog_StoryNoteVocabulary(t *testing.T) {
	p := dialogProjection()
	p.applyModelVocabulary(vocabTestModel())
	rows, _ := dialogRows(t, renderCreateDialog(p))
	for id, want := range map[string]string{"ac-2": "claimed by 1 Change Request", "ac-4": "claimed by 2 Change Requests"} {
		if rows[id].note != want {
			t.Errorf("%s: the story half reads %q, want %q", id, rows[id].note, want)
		}
	}
}

// TestCreateDialog_Legend: the legend counts the criteria Uncovered() marks
// as "n AC unclaimed" (SI-369 (3)), and any disclosure — or a criterion the
// coverage does not row — turns it into "coverage unproven", never a count.
func TestCreateDialog_Legend(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*BoardProjection)
		want   string
	}{
		{"one unclaimed", func(*BoardProjection) {}, "1 AC unclaimed"},
		{"none unclaimed", func(p *BoardProjection) {
			p.CreateCoverage.Criteria[2] = createCriterionView{ID: "ac-3", Stubs: 1}
			p.CreateCoverage.Uncovered = 0
		}, "0 AC unclaimed"},
		{"two unclaimed", func(p *BoardProjection) {
			p.CreateCoverage.Criteria[0] = createCriterionView{ID: "ac-1", Uncovered: true}
			p.CreateCoverage.Uncovered = 2
		}, "2 AC unclaimed"},
		{"a disclosure", func(p *BoardProjection) {
			p.CreateCoverage.Criteria[1].Disclosed = []string{"unreadable"}
			p.CreateCoverage.Unproven = true
		}, "coverage unproven"},
		{"a criterion with no coverage row", func(p *BoardProjection) {
			p.CreateCoverage.Criteria = p.CreateCoverage.Criteria[:3]
		}, "coverage unproven"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := dialogProjection()
			tc.mutate(p)
			_, text := elementByID(t, renderCreateDialog(p), "create-acs-count")
			if text != tc.want {
				t.Errorf("the legend reads %q, want %q", text, tc.want)
			}
		})
	}
}
