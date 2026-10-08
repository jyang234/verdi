package workbench

// The case-file strip (spec/wall-strip-and-drawer-v2 ac-1, ac-2, dc-1;
// ledger SI-368 (13), (19); lane F3a): the spec's problem and outcome on
// one line each, the class tag and the chips beside them, in place of the
// placards lockup. Each half keeps the placard hooks the wall's script
// reads — placard-problem/placard-outcome, .placard-text, the hidden
// .placard-full body and the .placard-more control — so the full case
// file opens through the existing expand dialog in every mode, and the
// one-line clamp is measured by the same pass that marks every clamped
// paper. On a live authoring wall each half names its typed operation
// (data-strip-op) and anchor, and wallstrip.js edits it in place: Enter
// applies one set-problem or set-outcome, Escape cancels. In review and
// read-only a click still expands.

import (
	stdhtml "html"
	"html/template"
	"strings"
)

// writeCaseStrip writes the strip for a wall carrying a problem or an
// outcome (the caller's hasCaseFile: a spec with neither gets no strip,
// and its disclosures stay in .board-notices, SI-368 (19)). editable is
// the wall's live domain surface: only then does a half carry its typed
// operation.
func writeCaseStrip(b *strings.Builder, p *BoardProjection, asd *asdView, editable bool) {
	b.WriteString(`<header class="case-strip case-file" data-testid="case-strip">`)
	if p.Problem != "" {
		writeCaseStripHalf(b, "problem", p.Problem, p.ProblemBodyHTML, editable, "set-problem", asd.ProblemAnchor)
	}
	if p.Problem != "" && p.Outcome != "" {
		b.WriteString(`<div class="case-arrow" aria-hidden="true">&#8594;</div>`)
	}
	if p.Outcome != "" {
		writeCaseStripHalf(b, "outcome", p.Outcome, p.OutcomeBodyHTML, editable, "set-outcome", asd.OutcomeAnchor)
	}
	// The chips (ac-2, dc-1): the class tag with every case-file badge
	// chip beside it (writeCaseTopline), then the disclosed-unproven
	// values in the disclosure style (writeCaseDisclosureChips).
	b.WriteString(`<div class="case-strip-chips" data-testid="case-strip-chips">`)
	writeCaseTopline(b, p)
	writeCaseDisclosureChips(b, p)
	b.WriteString(`</div>`)
	b.WriteString(`</header>`)
}

// writeCaseStripHalf writes one half: its tag, its one-line headline, the
// hidden full body when the spec carried one (writePlacardFull), and the
// "full case file" control, which the wall's script keeps while there is
// more to read than the line shows and removes otherwise. An editable
// half carries its operation and anchor, and its headline is a focusable
// control, so the keyboard reaches the in-place editor too.
func writeCaseStripHalf(b *strings.Builder, which, text string, body template.HTML, editable bool, op, anchor string) {
	esc := stdhtml.EscapeString
	b.WriteString(`<div class="placard placard--` + which + `"`)
	if editable {
		if anchor == "" {
			anchor = "#" + which
		}
		b.WriteString(` data-strip-op="` + op + `" data-strip-anchor="` + esc(anchor) + `"`)
	}
	b.WriteString(` data-testid="placard-` + which + `"><span class="placard-tag">` + which + `</span><p class="placard-text"`)
	if editable {
		b.WriteString(` role="button" tabindex="0" title="Click to edit"`)
	}
	b.WriteString(`>` + esc(text) + `</p>`)
	writePlacardFull(b, which, body)
	b.WriteString(`<button type="button" class="placard-more" aria-label="Read the full ` + which + `" aria-haspopup="dialog">full case file</button>`)
	b.WriteString(`</div>`)
}
