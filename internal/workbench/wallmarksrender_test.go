package workbench

// Render tests for the wall's readiness marks (spec/wall-canvas-v2 ac-1,
// dc-1; ledger SI-350 (1)–(2), SI-360 (4), SI-362 (2); lane M-ui): the
// dot and the chip a named card wears beside the coverage chip it keeps,
// the one unavailable notice, and nothing at all for marks that were
// never composed. Presentation over M-go's facts (wallmarks.go), rendered
// through renderBoardRegion over the scoping fixture's feature wall: ac-1
// covered by plain-one, ac-2 uncovered ("no stub"), oq-1 claimed twice,
// oq-2 unclaimed, and the stubs plain-one, spike-one and spike-two.

import (
	stdhtml "html"
	"regexp"
	"strings"
	"testing"
)

// renderWithMarks renders the scoping fixture's authoring wall with the
// given marks on its view.
func renderWithMarks(t *testing.T, marks *wallMarks) string {
	t.Helper()
	view := testASDView()
	view.Marks = marks
	return renderBoardRegion(scopingRenderProjection(t, modeAuthoring), &boardGitState{}, view)
}

// cardSlice is one paper's markup: from its testid to the next paper's
// open tag (every paper opens with one of these four class prefixes).
func cardSlice(t *testing.T, body, testid string) string {
	t.Helper()
	start := strings.Index(body, `data-testid="`+testid+`"`)
	if start < 0 {
		t.Fatalf("no paper %s in the region", testid)
	}
	rest := body[start:]
	end := len(rest)
	for _, open := range []string{`<div class="objcard`, `<div class="refcard`, `<div class="stubcard`, `<div class="sticky`} {
		if i := strings.Index(rest, open); i >= 0 && i < end {
			end = i
		}
	}
	return rest[:end]
}

var readinessChipRe = regexp.MustCompile(`<span class="readiness-chip" data-mark="([^"]*)" title="([^"]*)">([^<]*)</span>`)

// chipsOf reads one paper's mark: its chip words and titles, in order,
// or nil when it wears no mark. A chip whose visible word is not its
// data-mark fails the test.
func markChipsOf(t *testing.T, paper, owner string) [][2]string {
	t.Helper()
	open := `<span class="readiness-mark" data-testid="readiness-mark-` + owner + `">`
	start := strings.Index(paper, open)
	if start < 0 {
		if hasMarkMarkup(paper) {
			t.Fatalf("%s carries chip or dot markup without its mark container:\n%s", owner, paper)
		}
		return nil
	}
	rest := paper[start+len(open):]
	end := strings.Index(rest, `</span></span>`)
	if end < 0 {
		t.Fatalf("%s's mark never closes:\n%s", owner, paper)
	}
	var out [][2]string
	for _, m := range readinessChipRe.FindAllStringSubmatch(rest[:end+len(`</span>`)], -1) {
		if m[1] != m[3] {
			t.Fatalf("%s's chip says %q but is marked %q", owner, m[3], m[1])
		}
		out = append(out, [2]string{m[1], m[2]})
	}
	return out
}

func dotOf(owner string) string {
	return `<span class="readiness-dot" data-testid="readiness-dot-` + owner + `" aria-hidden="true"></span>`
}

const (
	uncoveredChipAC2 = `<span class="coverage-chip coverage-chip--none" data-testid="coverage-ac-2" data-coverage="0">no stub</span>`
	coveredChipAC1   = `<span class="coverage-chip coverage-chip--covered" data-testid="coverage-ac-1" data-coverage="1">covered by 1 stub</span>`
	marksNoticeOpen  = `<div class="board-notice wall-marks-notice" data-testid="wall-marks-unavailable" role="status">`
)

// readableMarks names ac-2 by its coverage concern, oq-2 by its question,
// and plain-one by its stub-unreconciled concern — the fixture's three
// kinds of named card (SI-350 (1)).
func readableMarks() *wallMarks {
	return &wallMarks{
		Objects: map[string][]wallMark{
			"ac-2": {{Concern: "success/coverage/ac-2", Chip: markChipNoStub}},
			"oq-2": {{Concern: "shape/question/oq-2", Chip: markChipUnresolved}},
		},
		Stubs: map[string]wallMark{
			"plain-one": {Concern: "review/blocker/stub-unreconciled/plain-one", Chip: markChipUnresolved},
		},
	}
}

// TestWallMarksRender_NamedCardsWearTheMark (ac-1, dc-1; SI-350 (1)): each
// card a mark names wears the dot and the chip with the mark's own word,
// beside the coverage chip it keeps byte for byte; every other paper
// wears none, and no notice is drawn.
func TestWallMarksRender_NamedCardsWearTheMark(t *testing.T) {
	body := renderWithMarks(t, readableMarks())

	ac2 := cardSlice(t, body, "card-ac-2")
	if !strings.Contains(ac2, uncoveredChipAC2) {
		t.Errorf("ac-2 lost its coverage chip (dc-1):\n%s", ac2)
	}
	if !strings.Contains(ac2, dotOf("ac-2")) {
		t.Errorf("ac-2 wears no dot:\n%s", ac2)
	}
	if got, want := markChipsOf(t, ac2, "ac-2"), [][2]string{{markChipNoStub, "Focus next: success/coverage/ac-2"}}; !equalChips(got, want) {
		t.Errorf("ac-2's chips = %q, want %q", got, want)
	}
	// Beside the coverage chip: the mark is a separate element after it,
	// and the chip's own text is untouched (the coverage chip is the one
	// element saying "no stub" through its data-coverage).
	if strings.Index(ac2, uncoveredChipAC2) > strings.Index(ac2, `readiness-mark-ac-2`) {
		t.Errorf("ac-2's mark precedes its coverage chip:\n%s", ac2)
	}
	if strings.Count(ac2, `data-coverage=`) != 1 {
		t.Errorf("ac-2 carries %d coverage chips, want one", strings.Count(ac2, `data-coverage=`))
	}

	oq2 := cardSlice(t, body, "card-oq-2")
	if !strings.Contains(oq2, dotOf("oq-2")) {
		t.Errorf("oq-2 wears no dot:\n%s", oq2)
	}
	if got, want := markChipsOf(t, oq2, "oq-2"), [][2]string{{markChipUnresolved, "Focus next: shape/question/oq-2"}}; !equalChips(got, want) {
		t.Errorf("oq-2's chips = %q, want %q", got, want)
	}
	if strings.Contains(oq2, "coverage-chip") {
		t.Errorf("oq-2 grew a coverage chip:\n%s", oq2)
	}

	stub := cardSlice(t, body, "stub-card-plain-one")
	if !strings.HasPrefix(stub[strings.Index(stub, ">")+1:], `<span class="stub-tab">`) {
		t.Errorf("the stub's slug is no longer its first line (ac-1):\n%s", stub)
	}
	if !strings.Contains(stub, dotOf("stub-plain-one")) {
		t.Errorf("the stub wears no dot:\n%s", stub)
	}
	if got, want := markChipsOf(t, stub, "stub-plain-one"), [][2]string{{markChipUnresolved, "Focus next: review/blocker/stub-unreconciled/plain-one"}}; !equalChips(got, want) {
		t.Errorf("the stub's chips = %q, want %q", got, want)
	}
	if meta, mark := strings.Index(stub, `class="stub-meta"`), strings.Index(stub, `readiness-mark-stub-plain-one`); meta < 0 || mark < meta {
		t.Errorf("the stub's mark does not follow its meta line:\n%s", stub)
	}

	for _, unnamed := range []string{"card-ac-1", "card-oq-1", "stub-card-spike-one", "stub-card-spike-two"} {
		paper := cardSlice(t, body, unnamed)
		if markChipsOf(t, paper, strings.TrimPrefix(strings.TrimPrefix(unnamed, "card-"), "stub-card-")) != nil || hasMarkMarkup(paper) {
			t.Errorf("%s, which nothing names, wears a mark:\n%s", unnamed, paper)
		}
	}
	if !strings.Contains(body, coveredChipAC1) {
		t.Error("ac-1's coverage chip changed")
	}
	if got := strings.Count(body, `class="readiness-mark"`); got != 3 {
		t.Errorf("the region draws %d marks, want 3", got)
	}
	if got := strings.Count(body, `class="readiness-dot"`); got != 3 {
		t.Errorf("the region draws %d dots, want 3", got)
	}
	if strings.Contains(body, "wall-marks-unavailable") {
		t.Error("readable marks drew the unavailable notice")
	}
}

func equalChips(a, b [][2]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestWallMarksRender_ChipWords: a card named by several concerns wears
// one chip (SI-350 (1)) — "no stub" when a coverage concern names it,
// the marks' word otherwise — whose title names every concern in Focus
// next order; the word is the facts' own, never the renderer's guess;
// and a mark naming a card the wall does not carry draws nothing.
func TestWallMarksRender_ChipWords(t *testing.T) {
	for _, tc := range []struct {
		name  string
		marks []wallMark
		want  [][2]string
	}{
		{
			name:  "coverage first",
			marks: []wallMark{{Concern: "success/coverage/ac-2", Chip: markChipNoStub}, {Concern: "review/blocker/outcome-floor/ac-2", Chip: markChipUnresolved}},
			want:  [][2]string{{markChipNoStub, "Focus next: success/coverage/ac-2, review/blocker/outcome-floor/ac-2"}},
		},
		{
			name:  "coverage second still wins the word; the title keeps the order",
			marks: []wallMark{{Concern: "review/blocker/outcome-floor/ac-2", Chip: markChipUnresolved}, {Concern: "success/coverage/ac-2", Chip: markChipNoStub}},
			want:  [][2]string{{markChipNoStub, "Focus next: review/blocker/outcome-floor/ac-2, success/coverage/ac-2"}},
		},
		{
			name:  "one word for two concerns",
			marks: []wallMark{{Concern: "review/role/x/ac-2", Chip: markChipUnresolved}, {Concern: "review/role/y/ac-2", Chip: markChipUnresolved}},
			want:  [][2]string{{markChipUnresolved, "Focus next: review/role/x/ac-2, review/role/y/ac-2"}},
		},
		{
			name:  "a non-coverage mark never says no stub",
			marks: []wallMark{{Concern: "review/role/x/ac-2", Chip: markChipUnresolved}},
			want:  [][2]string{{markChipUnresolved, "Focus next: review/role/x/ac-2"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := renderWithMarks(t, &wallMarks{Objects: map[string][]wallMark{"ac-2": tc.marks}})
			ac2 := cardSlice(t, body, "card-ac-2")
			if got := markChipsOf(t, ac2, "ac-2"); !equalChips(got, tc.want) {
				t.Errorf("chips = %q, want %q", got, tc.want)
			}
			if strings.Count(ac2, `class="readiness-dot"`) != 1 {
				t.Errorf("ac-2 wears %d dots, want one:\n%s", strings.Count(ac2, `class="readiness-dot"`), ac2)
			}
			if !strings.Contains(ac2, uncoveredChipAC2) {
				t.Errorf("ac-2 lost its coverage chip:\n%s", ac2)
			}
		})
	}

	t.Run("a mark naming no paper on the wall", func(t *testing.T) {
		body := renderWithMarks(t, &wallMarks{
			Objects: map[string][]wallMark{"ac-9": {{Concern: "success/coverage/ac-9", Chip: markChipNoStub}}},
			Stubs:   map[string]wallMark{"no-such-stub": {Concern: "review/blocker/stub-unreconciled/no-such-stub", Chip: markChipUnresolved}},
		})
		if hasMarkMarkup(body) {
			t.Errorf("a mark naming no paper drew markup:\n%s", body)
		}
	})
}

// TestWallMarksRender_UnavailableDrawsOneNoticeAndNoMark (SI-350 (2);
// SI-360 (4)): unreadable marks are one notice in the board's disclosure
// vocabulary, naming the reason, after the domain refusal when both are
// drawn — and no card wears a dot or a chip, whatever the facts also
// carry (fail closed). The coverage chips keep their texts.
func TestWallMarksRender_UnavailableDrawsOneNoticeAndNoMark(t *testing.T) {
	for _, tc := range []struct {
		name       string
		marks      wallMarks
		wantNotice string
	}{
		{
			name:       "a failed load",
			marks:      unavailableMarks("the readiness load failed: boom"),
			wantNotice: "The readiness marks are unavailable: the readiness load failed: boom.",
		},
		{
			name:       "no reason falls back to the unreadable reason",
			marks:      unavailableMarks(""),
			wantNotice: "The readiness marks are unavailable: " + stdhtml.EscapeString(marksUnreadable) + ".",
		},
		{
			name: "unavailable wins over marks the facts also carry",
			marks: wallMarks{
				Unavailable: marksBranchWall("design/other"),
				Objects:     map[string][]wallMark{"ac-2": {{Concern: "success/coverage/ac-2", Chip: markChipNoStub}}},
				Stubs:       map[string]wallMark{"plain-one": {Concern: "review/blocker/stub-unreconciled/plain-one", Chip: markChipUnresolved}},
			},
			wantNotice: "The readiness marks are unavailable: " + stdhtml.EscapeString(marksBranchWall("design/other")) + ".",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			marks := tc.marks
			body := renderWithMarks(t, &marks)
			if got := strings.Count(body, marksNoticeOpen); got != 1 {
				t.Fatalf("the region draws %d unavailable notices, want one:\n%s", got, body)
			}
			notice := body[strings.Index(body, marksNoticeOpen)+len(marksNoticeOpen):]
			notice = notice[:strings.Index(notice, "</div>")]
			if notice != tc.wantNotice {
				t.Errorf("the notice reads %q, want %q", notice, tc.wantNotice)
			}
			if !strings.Contains(body, `<div class="board-notices">`) || strings.Index(body, `<div class="board-notices">`) > strings.Index(body, marksNoticeOpen) {
				t.Error("the notice is not inside the board's notices")
			}
			if strings.Contains(body, "readiness-mark") || strings.Contains(body, "readiness-dot") || strings.Contains(body, "data-mark=") {
				t.Errorf("unavailable marks drew a mark:\n%s", body)
			}
			if !strings.Contains(body, uncoveredChipAC2) || !strings.Contains(body, coveredChipAC1) {
				t.Error("a coverage chip changed under the unavailable notice (dc-1)")
			}
		})
	}

	t.Run("after the domain refusal", func(t *testing.T) {
		p := scopingRenderProjection(t, modeAuthoring)
		p.DomainRefusal = "this branch is not the wall's"
		view := testASDView()
		marks := unavailableMarks("boom")
		view.Marks = &marks
		body := renderBoardRegion(p, &boardGitState{}, view)
		refusal, notice := strings.Index(body, `data-testid="asd-domain-refusal"`), strings.Index(body, marksNoticeOpen)
		if refusal < 0 || notice < 0 || notice < refusal {
			t.Errorf("the refusal (%d) and the marks notice (%d) are not in order", refusal, notice)
		}
		if strings.Count(body, `class="board-notices"`) != 1 {
			t.Errorf("the notices wrapper is drawn %d times", strings.Count(body, `class="board-notices"`))
		}
	})
}

// TestWallMarksRender_NilDrawsNothing (SI-362 (2)): marks that were never
// composed (a mutation or fragment response) draw no dot, no chip and no
// notice — the region's bytes are exactly the markless render's — and so
// do composed marks that name nothing.
func TestWallMarksRender_NilDrawsNothing(t *testing.T) {
	plain := renderBoardRegion(scopingRenderProjection(t, modeAuthoring), &boardGitState{}, testASDView())
	for _, tc := range []struct {
		name  string
		marks *wallMarks
	}{
		{name: "not composed", marks: nil},
		{name: "composed, naming nothing", marks: &wallMarks{}},
		{name: "composed, empty maps", marks: &wallMarks{Objects: map[string][]wallMark{}, Stubs: map[string]wallMark{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := renderWithMarks(t, tc.marks)
			if hasMarkMarkup(body) || strings.Contains(body, "wall-marks") || strings.Contains(body, "board-notices") {
				t.Errorf("drew mark or notice markup:\n%s", body)
			}
			if body != plain {
				t.Error("the region's bytes differ from the markless render's")
			}
		})
	}
	if hasMarkMarkup(plain) || strings.Contains(plain, "wall-marks") {
		t.Fatal("the markless render carries mark markup")
	}
}

// hasMarkMarkup reports whether s carries any of the mark's own markup
// (the shell's readiness-* classes are not the mark's).
func hasMarkMarkup(s string) bool {
	return strings.Contains(s, "readiness-mark") || strings.Contains(s, "readiness-dot") || strings.Contains(s, "readiness-chip") || strings.Contains(s, "data-mark=")
}
