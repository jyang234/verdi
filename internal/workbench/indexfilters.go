// The index's filter row (spec/index-v2 ac-3; SI-366 (6), (19), (21)(b)):
// four pills — everything, quiet, in review, disclosed — each carrying the
// count of the cards it selects. The filtering itself runs in the index's
// own asset (assets/index.js) over the one server-rendered DOM, reading
// each card's data-quiet, data-review and data-disclosed attributes, so
// before any script runs every card is shown and the pills only count.
// Column counts stay totals while a filter hides cards (19). When the
// forge could not be consulted this render, the in-review pill says so
// and carries no number — never a zero standing in for an unknown — and
// with no forge configured it says that instead (SI-366 (4)).
package workbench

import (
	"bytes"
	"fmt"
	stdhtml "html"
	"strconv"

	"github.com/jyang234/verdi/internal/refindex"
)

// filterCounts is the filter row's counts over the render's cards.
type filterCounts struct {
	everything int
	// quiet counts the cards index-data reads quiet (ageOf: IsQuiet
	// against the render's clock) — the cards whose data-quiet is "true".
	quiet int
	// inReview counts the cards whose branch the forge lists as in review
	// — the cards whose data-review is "open".
	inReview int
	// disclosed counts the cards stating any unproven fact (SI-366
	// (21)(b)) — the cards whose data-disclosed is "true".
	disclosed int
}

// countFilters counts what each filter selects, from the same facts the
// cards carry as attributes, so a pill's number and the cards the script
// shows can never disagree.
func countFilters(cards []cardFacts) filterCounts {
	var n filterCounts
	for _, c := range cards {
		n.everything++
		if c.age.quiet {
			n.quiet++
		}
		if c.review == reviewOpen {
			n.inReview++
		}
		if c.disclosed {
			n.disclosed++
		}
	}
	return n
}

// The filter ids, as the pills' data-filter and the script's switch read
// them.
const (
	filterEverything = "everything"
	filterQuiet      = "quiet"
	filterInReview   = "in-review"
	filterDisclosed  = "disclosed"
)

// writeFilterRow draws the filter row: everything is pressed first (every
// card shown), and the in-review pill reads its disabled disclosure when
// the forge is unconfigured or unavailable.
func writeFilterRow(buf *bytes.Buffer, cards []cardFacts, mrConfigured, mrUnavailable bool) {
	n := countFilters(cards)
	buf.WriteString(`<div class="dir-filters" data-testid="dir-filters" role="group" aria-label="Show"><span class="dir-filters-label">Show</span>`)
	writeFilterPill(buf, filterEverything, "everything", n.everything, true, "every card")
	writeFilterPill(buf, filterQuiet, "quiet", n.quiet, false, fmt.Sprintf("on the desk for more than %d days without a change", refindex.QuietAfterDays))
	switch {
	case !mrConfigured:
		writeFilterPillDisabled(buf, filterInReview, "in review · no forge configured", "no forge is configured to consult, so no review state is known for any branch")
	case mrUnavailable:
		writeFilterPillDisabled(buf, filterInReview, "in review · unavailable", "the forge could not be consulted this render, so no review state is known for any branch")
	default:
		writeFilterPill(buf, filterInReview, "in review", n.inReview, false, "the forge lists an open merge request from the branch")
	}
	writeFilterPill(buf, filterDisclosed, "disclosed", n.disclosed, false, "the card states an unproven fact")
	buf.WriteString(`</div>`)
}

// writeFilterPill draws one pill with its count.
func writeFilterPill(buf *bytes.Buffer, id, label string, count int, pressed bool, title string) {
	buf.WriteString(`<button type="button" class="dir-filter" data-filter="`)
	buf.WriteString(id)
	buf.WriteString(`" data-testid="dir-filter-`)
	buf.WriteString(id)
	buf.WriteString(`" aria-pressed="`)
	buf.WriteString(strconv.FormatBool(pressed))
	buf.WriteString(`" title="`)
	buf.WriteString(stdhtml.EscapeString(title))
	buf.WriteString(`">`)
	buf.WriteString(stdhtml.EscapeString(label))
	buf.WriteString(` <span class="count">`)
	buf.WriteString(strconv.Itoa(count))
	buf.WriteString(`</span></button>`)
}

// writeFilterPillDisabled draws a pill that cannot select, because what it
// would select is unknown: its label says why, and it carries no number.
func writeFilterPillDisabled(buf *bytes.Buffer, id, label, title string) {
	buf.WriteString(`<button type="button" class="dir-filter dir-filter-unavailable" data-filter="`)
	buf.WriteString(id)
	buf.WriteString(`" data-testid="dir-filter-`)
	buf.WriteString(id)
	buf.WriteString(`" aria-pressed="false" disabled title="`)
	buf.WriteString(stdhtml.EscapeString(title))
	buf.WriteString(`">`)
	buf.WriteString(stdhtml.EscapeString(label))
	buf.WriteString(`</button>`)
}
