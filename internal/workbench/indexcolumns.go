// The index's four columns (spec/index-v2 ac-1; parent workbench-redesign
// dc-12): the directory's four status groups drawn side by side in
// workbench-directory dc-2's order, each with its identity heading, its
// count, the line saying where its specs live, the line naming the move
// that advances a card (rewritten to D-WR-12: a merge moves a card right,
// never Commit and push; SI-366 (18)), and an explicit empty state when
// it holds no card. The grouping itself stays refindex's (directory.go:
// statusGroupOrder); this file only names and draws the columns. The
// column copy that speaks a class, state or verb word routes it through
// the model's display vocabulary (spec/vocabulary-surfaces).
package workbench

import (
	"bytes"
	stdhtml "html"
	"strconv"
	"time"

	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/refindex"
)

// columnCopy is one column's fixed copy.
type columnCopy struct {
	// heading is the StatusGroup's identity word (SI-366 (18)).
	heading string
	// where names where the column's specs live.
	where string
	// move names the move that advances a card out of the column.
	move string
	// empty is the column's explicit empty state.
	empty string
}

// columnCopyFor is g's copy, through mdl's display vocabulary for every
// class, state or verb word it speaks, and true for every entry the
// column holds (SI-366 (22)(a)): the desk holds design-branch drafts AND
// default-branch specs whose authored status is draft, so its where and
// move lines name both. An unknown group fails closed to no copy at all,
// never an invented heading.
func columnCopyFor(g refindex.StatusGroup, mdl *model.Model) columnCopy {
	words := classWords{m: mdl}
	switch g {
	case refindex.StatusGroupDraftsInProgress:
		draft := mdl.DisplayState("", "draft")
		return columnCopy{
			heading: "On the desk",
			where:   "any branch · " + draft,
			move:    "A " + draft + " you can still write into. A " + words.verb("merge") + " of its branch, or an authored status on the default branch, moves one right.",
			empty:   "Nothing on the desk.",
		}
	case refindex.StatusGroupAcceptedPendingBuild:
		accepted := mdl.DisplayState("", "accepted-pending-build")
		return columnCopy{
			heading: "Accepted",
			where:   "default branch · " + accepted,
			move:    "Filed on the default branch. Evidence lands as " + words.plural("story") + " are built.",
			empty:   "Nothing " + accepted + " yet.",
		}
	case refindex.StatusGroupActiveComponents:
		active := mdl.DisplayState("", "active")
		return columnCopy{
			heading: "Active components",
			where:   "default branch · " + active,
			move:    "Components whose obligations are being checked.",
			empty:   "No " + active + " components.",
		}
	case refindex.StatusGroupTerminal:
		return columnCopy{
			heading: "On the shelf",
			where:   "default branch · terminal",
			move:    model.Capitalize(mdl.DisplayState("", "superseded")) + " or " + mdl.DisplayState("", "closed") + ". Read-only; kept for the record.",
			empty:   "Nothing on the shelf.",
		}
	}
	return columnCopy{}
}

// writeDirectoryColumn draws one column: its section keeps the dir-group
// test id every pinned surface addresses (dc-12), and holds the heading
// with its count, the where and move lines, then either the cards or the
// empty state — never both, never neither.
func writeDirectoryColumn(buf *bytes.Buffer, g refindex.StatusGroup, cards []cardFacts, mdl *model.Model, now time.Time) {
	c := columnCopyFor(g, mdl)
	buf.WriteString(`<section class="dir-group dir-group--`)
	buf.WriteString(string(g))
	buf.WriteString(`" data-testid="dir-group-`)
	buf.WriteString(string(g))
	buf.WriteString(`"><header class="dir-group-head"><h2>`)
	buf.WriteString(stdhtml.EscapeString(c.heading))
	buf.WriteString(` <span class="count">`)
	buf.WriteString(strconv.Itoa(len(cards)))
	buf.WriteString(`</span></h2><span class="dir-group-where">`)
	buf.WriteString(stdhtml.EscapeString(c.where))
	buf.WriteString(`</span></header><p class="dir-group-move">`)
	buf.WriteString(stdhtml.EscapeString(c.move))
	buf.WriteString(`</p>`)
	if len(cards) == 0 {
		buf.WriteString(`<p class="empty dir-empty">`)
		buf.WriteString(stdhtml.EscapeString(c.empty))
		buf.WriteString(`</p></section>`)
		return
	}
	buf.WriteString(`<ul class="dir-cards">`)
	for _, card := range cards {
		writeDirectoryEntry(buf, card, mdl, now)
	}
	buf.WriteString(`</ul></section>`)
}
