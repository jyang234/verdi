package workbench

// Render tests for the derivation drawer's one server-side body renderer
// (spec/derivation-drawer ac-2/ac-4): the drawer is a pure function of
// the badge's derivation record — same record, same bytes; every drawer
// line traces to a record field; every cited revision is a digest/sha;
// and no render path reads a clock (proven by rendering the same record
// at two different wall-clock times and comparing bytes, so a smuggled
// time.Now fails the test rather than a string-pattern assertion).

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

// drawerFixtureRecord is a full-featured record: provenance block,
// two pinned inputs, records, and disclosures — the judged-sweep chip's
// shape, which exercises every drawer section at once.
func drawerFixtureRecord() badgeView {
	return badgeView{
		Source: "align:judged-sweep",
		Label:  "2 judged findings",
		Inputs: []badgeInputView{
			{Name: "covers", Path: ".verdi/specs/active/x/spec.md", Revision: "96b44f049d11bfef37e017d5e8f7dcb462a58ef4"},
			{Name: "decision-conflict-report", Path: ".verdi/specs/active/x/decision-conflict-report.md", Revision: "sha256:2972c86a2e1d59d9fee0983fc9893b7a9082f518b2291b1a35e7d3e41d324658"},
		},
		Records: []string{
			"judged-dcf-1 [no-conflict] first finding — note: cleared",
			"judged-dcf-2 [undispositioned] second finding",
		},
		Disclosures: []string{"dc-2 is not in decisions_scanned"},
		Provenance: []string{
			"sweep covers 96b44f049d11bfef37e017d5e8f7dcb462a58ef4",
			"adr_corpus_digest sha256:37517e5f3dc66819f61f5a7bb8ace1921282415f10551d2defa5c3eb0985b570",
			"decisions_scanned: spec/x#dc-1",
		},
	}
}

func renderDrawer(bd badgeView) string {
	var b strings.Builder
	writeBadgeDrawer(&b, bd)
	return b.String()
}

func TestWriteBadgeDrawer_RendersEveryRecordField(t *testing.T) {
	bd := drawerFixtureRecord()
	html := renderDrawer(bd)

	// dc-4's interaction shape: a role=dialog panel, hidden until opened,
	// with its own close control.
	for _, want := range []string{
		`class="badge-drawer" role="dialog"`,
		`aria-label="derivation: align:judged-sweep"`,
		` hidden>`,
		`class="drawer-close" aria-label="Close derivation drawer"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("drawer markup lacks %q:\n%s", want, html)
		}
	}

	// The rule id, the label, every provenance line, every input with its
	// revision, every record, every disclosure — field-for-field.
	if !strings.Contains(html, `<span class="drawer-source">align:judged-sweep</span>`) {
		t.Error("drawer does not name its source rule id")
	}
	if !strings.Contains(html, `<p class="drawer-label">2 judged findings</p>`) {
		t.Error("drawer does not carry the record's label")
	}
	for _, line := range bd.Provenance {
		if !strings.Contains(html, `<span class="drawer-provenance-line">`+line+`</span>`) {
			t.Errorf("provenance line %q missing from the drawer head", line)
		}
	}
	for _, in := range bd.Inputs {
		row := `<tr class="drawer-input"><td class="drawer-input-name">` + in.Name +
			`</td><td class="drawer-input-path">` + in.Path +
			`</td><td class="drawer-input-rev">` + in.Revision + `</td></tr>`
		if !strings.Contains(html, row) {
			t.Errorf("input row for %q missing:\n%s", in.Name, html)
		}
	}
	for _, r := range bd.Records {
		if !strings.Contains(html, `<li class="drawer-record">`+r+`</li>`) {
			t.Errorf("firing record %q missing", r)
		}
	}
	for _, d := range bd.Disclosures {
		if !strings.Contains(html, `<li class="drawer-disclosure">`+d+`</li>`) {
			t.Errorf("disclosure %q missing", d)
		}
	}
}

// TestWriteBadgeDrawer_NothingBeyondTheRecord is ac-2's falsifiability
// half: nothing appears in the drawer that is not in the record. Element
// counts equal the record's own lengths, and stripping the markup leaves
// only record fields plus the renderer's fixed chrome labels.
func TestWriteBadgeDrawer_NothingBeyondTheRecord(t *testing.T) {
	bd := drawerFixtureRecord()
	html := renderDrawer(bd)

	counts := []struct {
		marker string
		want   int
	}{
		{`<tr class="drawer-input">`, len(bd.Inputs)},
		{`<li class="drawer-record">`, len(bd.Records)},
		{`<li class="drawer-disclosure">`, len(bd.Disclosures)},
		{`<span class="drawer-provenance-line">`, len(bd.Provenance)},
	}
	for _, c := range counts {
		if got := strings.Count(html, c.marker); got != c.want {
			t.Errorf("%d × %s, want exactly %d (one per record entry)", got, c.marker, c.want)
		}
	}

	// Strip tags; every remaining text run must be a record field or one
	// of the renderer's fixed chrome strings — any other text is a drawer
	// line with no record source (the pure-function claim's witness).
	fixed := map[string]bool{
		"&#215;": true, "pinned inputs": true, "input": true, "path": true,
		"revision": true, "firing records": true, "disclosures": true,
	}
	fromRecord := map[string]bool{bd.Source: true, bd.Label: true}
	for _, in := range bd.Inputs {
		fromRecord[in.Name], fromRecord[in.Path], fromRecord[in.Revision] = true, true, true
	}
	for _, s := range bd.Records {
		fromRecord[s] = true
	}
	for _, s := range bd.Disclosures {
		fromRecord[s] = true
	}
	for _, s := range bd.Provenance {
		fromRecord[s] = true
	}
	for _, run := range regexp.MustCompile(`>([^<>]+)<`).FindAllStringSubmatch(html, -1) {
		text := strings.TrimSpace(run[1])
		if text == "" || fixed[text] || fromRecord[text] {
			continue
		}
		t.Errorf("drawer text %q has no source in the record", text)
	}
}

// TestWriteBadgeDrawer_EmptySectionsRenderNothing is the negative path:
// a record without provenance/records/disclosures gets no empty section
// markup — never an empty receipts block.
func TestWriteBadgeDrawer_EmptySectionsRenderNothing(t *testing.T) {
	bd := badgeView{
		Source: "lint:VL-003",
		Label:  "dangling ref",
		Inputs: []badgeInputView{{Name: "spec", Path: "p", Revision: "sha256:aabb"}},
	}
	html := renderDrawer(bd)
	for _, forbidden := range []string{"drawer-provenance", "drawer-records", "drawer-disclosures", "drawer-record\"", "drawer-disclosure\""} {
		if strings.Contains(html, forbidden) {
			t.Errorf("empty section %q rendered on a record without it:\n%s", forbidden, html)
		}
	}
	if !strings.Contains(html, "drawer-inputs") {
		t.Error("the pinned-inputs table went missing")
	}
}

// TestWriteBadgeDrawer_PureFunctionAcrossTime is the ac-4--behavioral
// obligation's clock witness: the same record rendered at two different
// wall-clock times (straddling a real second boundary) produces
// byte-identical drawer markup, so ANY smuggled clock read — however
// formatted — fails this test.
func TestWriteBadgeDrawer_PureFunctionAcrossTime(t *testing.T) {
	bd := drawerFixtureRecord()
	first := renderDrawer(bd)

	// Sleep past the next wall-clock second so second-resolution
	// timestamps (the coarsest common formatting) cannot collide.
	now := time.Now()
	time.Sleep(now.Truncate(time.Second).Add(1050 * time.Millisecond).Sub(now))

	second := renderDrawer(bd)
	if first != second {
		t.Errorf("drawer bytes differ across wall-clock time:\n%s\n%s", first, second)
	}
}

// TestWriteBadgeDrawer_RevisionsAreDigestsOrShas is ac-4's citation
// shape: every cited input revision in drawer markup is a content digest
// or a commit sha read from the record — never a date, never a time.
func TestWriteBadgeDrawer_RevisionsAreDigestsOrShas(t *testing.T) {
	html := renderDrawer(drawerFixtureRecord())
	revRe := regexp.MustCompile(`<td class="drawer-input-rev">([^<]*)</td>`)
	pinnedRe := regexp.MustCompile(`^(sha256:[0-9a-f]{64}|[0-9a-f]{7,40})$`)
	revs := revRe.FindAllStringSubmatch(html, -1)
	if len(revs) == 0 {
		t.Fatal("no input revisions rendered")
	}
	for _, m := range revs {
		if !pinnedRe.MatchString(m[1]) {
			t.Errorf("cited revision %q is not a digest or sha", m[1])
		}
	}
}

// TestWriteBadgeDrawer_EscapesHostileRecord: every drawer line is
// document-derived text and must never inject markup.
func TestWriteBadgeDrawer_EscapesHostileRecord(t *testing.T) {
	hostile := `"><script>alert(1)</script>`
	bd := badgeView{
		Source:      hostile,
		Label:       hostile,
		Inputs:      []badgeInputView{{Name: hostile, Path: hostile, Revision: hostile}},
		Records:     []string{hostile},
		Disclosures: []string{hostile},
		Provenance:  []string{hostile},
	}
	html := renderDrawer(bd)
	if strings.Contains(html, "<script>") {
		t.Fatalf("hostile record content reached drawer markup unescaped:\n%s", html)
	}
}

// TestRenderBoardRegion_DrawerIsHiddenSiblingOfEveryBadge is dc-1 at the
// region level: every badge button is immediately followed by its own
// hidden drawer body, on card chips and case-file stamps alike, in every
// mode — and two renders of the same projection are byte-identical
// (ac-2: the page and the post-mutation fragment share this renderer).
func TestRenderBoardRegion_DrawerIsHiddenSiblingOfEveryBadge(t *testing.T) {
	for _, mode := range []boardModeKind{modeAuthoring, modeReview, modeReadOnly} {
		t.Run(string(mode), func(t *testing.T) {
			p := badgeRenderProjection(mode)
			git := &boardGitState{Branch: "design/x", DefaultBranch: "main"}
			html := renderBoardRegion(p, git)

			if got, want := strings.Count(html, `class="badge-drawer"`), 3; got != want {
				t.Fatalf("%d drawers rendered, want %d (one per badge)", got, want)
			}
			// Sibling adjacency: every badge button's closing tag is
			// immediately followed by its drawer element.
			for _, m := range regexp.MustCompile(`data-badge-source="[^"]*"[^>]*>[^<]*</button>`).FindAllStringIndex(html, -1) {
				after := html[m[1]:]
				if !strings.HasPrefix(after, `<div class="badge-drawer"`) {
					t.Errorf("badge button is not immediately followed by its drawer sibling: …%s", after[:min(80, len(after))])
				}
			}
			if html != renderBoardRegion(p, git) {
				t.Error("two renders of the same projection differ — the drawer is not a pure function of the record")
			}
		})
	}
}
