package workbench

// The wall's readiness marks, presentation side (spec/wall-canvas-v2 ac-1,
// dc-1; ledger SI-350 (1)–(2), SI-360 (4), SI-362 (2); lane M-ui): the
// markup a card a Focus next concern names wears, and the one notice a
// wall whose marks are unreadable shows. The facts are wallmarks.go's
// (owner F-3); nothing here decides which card a concern names or what
// its word is. The dot and the chips are server markup inside the
// region, so every composed refresh re-draws them with the region it
// swaps in; a mutation or fragment response's view carries no marks
// (Marks == nil) and draws none, until the next poll (SI-362 (2)).

import (
	stdhtml "html"
	"strings"
)

// forObject is the marks an object card wears: none when the marks were
// not composed or are unreadable (SI-350 (2): no card shows a mark then,
// whatever else the facts carry).
func (m *wallMarks) forObject(id string) []wallMark {
	if m == nil || m.Unavailable != "" {
		return nil
	}
	return m.Objects[id]
}

// forStub is the marks a stub card wears, under the same rule.
func (m *wallMarks) forStub(slug string) []wallMark {
	if m == nil || m.Unavailable != "" {
		return nil
	}
	if mark, ok := m.Stubs[slug]; ok {
		return []wallMark{mark}
	}
	return nil
}

// notice is the unavailable notice's reason, or "" when the marks were
// not composed or are readable.
func (m *wallMarks) notice() string {
	if m == nil {
		return ""
	}
	return m.Unavailable
}

// markChipView is a card's one chip: its word, and every concern naming
// the card.
type markChipView struct {
	word     string
	concerns []string
}

// markChipFor is a card's one chip (SI-350 (1): the mark is a dot and a
// chip; the handoff draws one word per card). Its word is "no stub" when
// any concern naming the card is the coverage family — the criterion's
// own, actionable word — and the marks' word ("unresolved", SI-360 (4))
// otherwise; its title names every concern naming the card, in Focus
// next order, so a card named by several concerns loses none of them.
// marks is never empty here.
func markChipFor(marks []wallMark) markChipView {
	chip := markChipView{word: marks[0].Chip}
	for _, m := range marks {
		if m.Chip == markChipNoStub {
			chip.word = markChipNoStub
		}
		chip.concerns = append(chip.concerns, m.Concern)
	}
	return chip
}

// writeReadinessMark writes one paper's readiness mark (the handoff's
// "readiness marks on a card", as dc-1 reads it): a dot at the card's
// top-right corner, and a chip whose word is the marks' own, so the
// meaning is in the text and never in the colour alone (Wave 6 §5.2).
// ownerID is the paper's testid stem (an object id, or stub-<slug>).
// Nothing is written for a paper no mark names, so the region's bytes
// are the markless render's there.
func writeReadinessMark(b *strings.Builder, ownerID string, marks []wallMark) {
	if len(marks) == 0 {
		return
	}
	esc := stdhtml.EscapeString
	chip := markChipFor(marks)
	b.WriteString(`<span class="readiness-dot" data-testid="readiness-dot-` + esc(ownerID) + `" aria-hidden="true"></span>`)
	b.WriteString(`<span class="readiness-mark" data-testid="readiness-mark-` + esc(ownerID) + `">`)
	b.WriteString(`<span class="readiness-chip" data-mark="` + esc(chip.word) + `" title="` + esc("Focus next: "+strings.Join(chip.concerns, ", ")) + `">` + esc(chip.word) + `</span>`)
	b.WriteString(`</span>`)
}

// writeMarksNotice writes the one notice a wall whose marks' input cannot
// be read shows (SI-350 (2)): the board's disclosure vocabulary (the
// .board-notice channel every other disclosed-unavailable input speaks),
// saying the marks are unavailable and why, with the reason's own words.
func writeMarksNotice(b *strings.Builder, reason string) {
	b.WriteString(`<div class="board-notice wall-marks-notice" data-testid="wall-marks-unavailable" role="status">The readiness marks are unavailable: ` + stdhtml.EscapeString(strings.TrimSuffix(reason, ".")) + `.</div>`)
}
