package workbench

// Server-side rendering for the wall badges (spec/badge-computes ac-5,
// dc-4): the visual grammar over the badge data attachBadges (badges.go)
// computed. Card badges are compact chips riding the card's existing
// receipt-row vocabulary (the coverage-chip/obligation idiom of
// boardspecrender.go's writeScopingReceipts/writeObligations); case-file
// badges are stamps on the case-file lockup beside the class tag
// (writeCaseClassTag's position). Every badge element is a BUTTON
// carrying data-badge-source and its serialized derivation record
// (data-badge-record) — the derivation-drawer story's opener contract
// (dc-4), rendered verbatim here so the drawer can open receipts without
// recomputation (ac-4). All markup is emitted by this one server-side
// renderer through renderBoardRegion — the page and the post-mutation
// fragment share it, and boardspec.js templates nothing.
//
// Badges render in EVERY board mode (ac-5: authoring, review, read-only
// alike, the way notices do) and are pure presentation: no write path
// reads them (co-2 — disclosure, never refusal).

import (
	"encoding/json"
	stdhtml "html"
	"strings"
)

// writeBadgeChips writes one card's badge chips (dc-4's first form): a
// receipts row of compact chip buttons inside the card element carrying
// ownerID's own testid namespace. A badge-free card writes nothing —
// never an empty receipts row.
func writeBadgeChips(b *strings.Builder, ownerID string, badges []badgeView) {
	if len(badges) == 0 {
		return
	}
	b.WriteString(`<div class="card-badges" data-testid="badges-` + stdhtml.EscapeString(ownerID) + `">`)
	for _, bd := range badges {
		writeBadgeButton(b, "badge-chip", bd)
	}
	b.WriteString(`</div>`)
}

// writeCaseTopline writes the case-file lockup's top-right lockup: the
// class tag alone when the spec wears no spec-level badge (byte-stable
// with the pre-badge markup), or the stamp row — every case-file badge as
// a stamp, then the class tag beside them (dc-4: "stamps on the case-file
// lockup beside the class tag") — when it does.
func writeCaseTopline(b *strings.Builder, p *BoardProjection) {
	if len(p.CaseFileBadges) == 0 {
		writeCaseClassTag(b, p)
		return
	}
	b.WriteString(`<span class="case-stamp-row" data-testid="case-file-badges">`)
	for _, bd := range p.CaseFileBadges {
		writeBadgeButton(b, "case-stamp", bd)
	}
	writeCaseClassTag(b, p)
	b.WriteString(`</span>`)
}

// writeCaseDisclosures writes the case file's disclosed-unproven lines
// (spec/case-file-flags ac-1/dc-4): one line per ladder disclosure, in
// the board's notice vocabulary (the board-notice voice, role="status"),
// rendered on the case-file lockup itself — never a stamp (unproven is
// never dressed as a verdict) and never silence. A disclosure-free wall
// writes nothing.
func writeCaseDisclosures(b *strings.Builder, p *BoardProjection) {
	for _, d := range p.CaseFileDisclosures {
		b.WriteString(`<p class="board-notice case-disclosure" data-testid="case-file-disclosure" role="status">` + stdhtml.EscapeString(d) + `</p>`)
	}
}

// writeBadgeButton writes one badge element: a button (dc-4 verbatim)
// whose visible text is the record's short label, whose tooltip carries
// the full firing records (the board's established "headline visible,
// full form in title=" idiom), and whose data attributes are the drawer's
// opener contract — data-badge-source (the namespaced rule id) and
// data-badge-record (the derivation record, serialized). Everything is
// HTML-escaped: a finding message is document-derived text.
func writeBadgeButton(b *strings.Builder, class string, bd badgeView) {
	esc := stdhtml.EscapeString
	title := bd.Source
	if len(bd.Records) > 0 {
		title += "\n" + strings.Join(bd.Records, "\n")
	} else if bd.Label != "" {
		title += "\n" + bd.Label
	}
	b.WriteString(`<button type="button" class="` + class + `" data-badge-source="` + esc(bd.Source) +
		`" data-badge-record="` + esc(badgeRecordJSON(bd)) + `" title="` + esc(title) + `">` + esc(bd.Label) + `</button>`)
}

// badgeRecordJSON serializes one badge's derivation record for its
// data-badge-record attribute — plain encoding/json over badgeView, which
// is deterministic (fixed struct field order; the compute layer already
// sorted Inputs/Records at construction, ac-4).
func badgeRecordJSON(bd badgeView) string {
	j, err := json.Marshal(bd)
	if err != nil {
		// Unreachable: badgeView is strings and string slices only, which
		// cannot fail to marshal. Fail closed with an empty record rather
		// than panicking a render.
		return "{}"
	}
	return string(j)
}
