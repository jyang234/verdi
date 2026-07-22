package workbench

// The derivation drawer's server-side body renderer (spec/derivation-
// drawer ac-2, dc-1): the ONE place drawer markup is emitted, a pure
// function of the badge's canonical derivation record — same record, same
// drawer bytes. It is written per badge as a hidden sibling of the badge
// button (writeBadgeButton, badgerender.go — the board's writePlacardFull
// idiom), so the full page and the post-mutation fragment reach it
// through the same renderBoardRegion path, and assets/boardspec.js only
// opens, positions, and closes it: no client-side templating of
// derivation data, ever.
//
// Every citation below is a field read off the record — the rule id, the
// pinned inputs with their digest/sha revisions, the firing records, the
// provenance block, the disclosure lines. Nothing here recomputes, reads
// the store, or reads a clock (ac-4/co-1): the receipt is re-verifiable
// against its pinned inputs, not datable against a wall clock.

import (
	stdhtml "html"
	"strings"
)

// writeBadgeDrawer writes one badge's drawer body from its derivation
// record alone. The drawer is a role=dialog panel (dc-4) carrying its own
// close control; it renders `hidden` and stays the server's own
// projection — boardspec.js toggles it, never rebuilds it.
func writeBadgeDrawer(b *strings.Builder, bd badgeView) {
	esc := stdhtml.EscapeString
	b.WriteString(`<div class="badge-drawer" role="dialog" aria-modal="true" aria-label="derivation: ` + esc(bd.Source) + `" data-testid="badge-drawer" data-badge-source="` + esc(bd.Source) + `" hidden>`)

	// The head: the namespaced source rule id that fired (wall-receipts
	// dc-2's first demand), the chip's own label as the receipt's plain-
	// language headline, and the close control.
	b.WriteString(`<div class="drawer-head"><span class="drawer-source">` + esc(bd.Source) + `</span>`)
	// vocab:identity — non-vocabulary homograph: dismiss-this-drawer UI aria-label plus the "drawer-close" CSS class fragment (identity)
	b.WriteString(`<button type="button" class="drawer-close" aria-label="Close derivation drawer">&#215;</button></div>`)
	if bd.Label != "" {
		b.WriteString(`<p class="drawer-label">` + esc(bd.Label) + `</p>`)
	}

	// The pinned-provenance block, stamped once at the drawer's head
	// (derivation-drawer dc-2) — present only when the record carries one
	// (the judged-sweep chip's covers/adr_corpus_digest/decisions_scanned).
	if len(bd.Provenance) > 0 {
		b.WriteString(`<div class="drawer-provenance" data-testid="drawer-provenance">`)
		for _, line := range bd.Provenance {
			b.WriteString(`<span class="drawer-provenance-line">` + esc(line) + `</span>`)
		}
		b.WriteString(`</div>`)
	}

	// The pinned inputs, each with its revision — a digest, sha, or
	// pinned field carried verbatim from the record (ac-4).
	if len(bd.Inputs) > 0 {
		b.WriteString(`<table class="drawer-inputs" data-testid="drawer-inputs"><caption>pinned inputs</caption>`)
		b.WriteString(`<thead><tr><th scope="col">input</th><th scope="col">path</th><th scope="col">revision</th></tr></thead><tbody>`)
		for _, in := range bd.Inputs {
			b.WriteString(`<tr class="drawer-input"><td class="drawer-input-name">` + esc(in.Name) +
				`</td><td class="drawer-input-path">` + esc(in.Path) +
				`</td><td class="drawer-input-rev">` + esc(in.Revision) + `</td></tr>`)
		}
		b.WriteString(`</tbody></table>`)
	}

	// The firing records — receipts, not verdicts.
	if len(bd.Records) > 0 {
		b.WriteString(`<div class="drawer-records"><span class="drawer-section-label">firing records</span><ul>`)
		for _, r := range bd.Records {
			b.WriteString(`<li class="drawer-record">` + esc(r) + `</li>`)
		}
		b.WriteString(`</ul></div>`)
	}

	// Disclosure lines (dc-3): the compute's own honesty — staleness
	// contrasts and unprovable inputs — rendered as quiet disclosures,
	// never a verdict badge.
	if len(bd.Disclosures) > 0 {
		b.WriteString(`<div class="drawer-disclosures"><span class="drawer-section-label">disclosures</span><ul>`)
		for _, d := range bd.Disclosures {
			b.WriteString(`<li class="drawer-disclosure">` + esc(d) + `</li>`)
		}
		b.WriteString(`</ul></div>`)
	}

	b.WriteString(`</div>`)
}
