package workbench

// The workbench top bar (spec/chrome-and-tokens-v2 ac-1, ac-2, ac-5; ledger
// SI-323): one row at the head of every page verdi serve renders, drawn
// from the page's barFacts — the wordmark linking to the index, the page
// title, the class chip on a page about one spec, the posture group (the
// mode chip, the terminal status badge, the branch text, the posture
// disclosure, and the wall's Refresh), the page's own nav links, and a
// slot for its controls. It replaces the site header, the board header,
// and the posture row; every control those rows carried moves here with
// its id unchanged (dc-3; SI-323 (5)).
//
// The posture group is one fragment (writeASDPosture, boardshellrender.go),
// rendered here for the page and by the wall's snapshot for every refresh
// (SI-323 (3)). The full posture is a native <details>: it opens before
// any script runs (dc-2), and the bar's own asset, /assets/topbar.js,
// enhances it to the handoff's popover — Escape or an outside press closes
// it and returns focus to the posture text (SI-331).
//
// The bar is one row (handoff "Global chrome": 52 px, padding 0 20px,
// full-bleed), wrapping only below the 640 px breakpoint; the wall's last
// action result is a trailing row of its own under it.

import (
	stdhtml "html"
	"html/template"
	"strings"
)

// topBarOptions is what a page adds to its facts: which element carries
// the title, the wordmark's surface word, the nav links, page-specific
// chips, the controls slot, a trailing row, and whether the posture
// carries the wall's Refresh.
type topBarOptions struct {
	// Heading draws the title as the page's h1. The Document page's
	// rendered document carries the page's one h1 (spec-documents), so
	// its bar draws the title as plain text.
	Heading bool
	// Surface adds the WORKBENCH surface word to the wordmark — the index
	// only (handoff README, "Global chrome").
	Surface bool
	// Nav is the page's own links (index, artifact), pre-rendered.
	Nav template.HTML
	// Chips are page-specific chips drawn after the title: the diagram
	// editor's mode stamp and status badge.
	Chips template.HTML
	// Controls is the page's controls slot (dc-3): the Wall and Document
	// switch, Commit & push, the diagram editor's exit, the autosave
	// status, and the live regions.
	Controls template.HTML
	// Tail is a full-width trailing row: the wall's last action result,
	// which lives outside the swapped region and survives every refresh.
	Tail template.HTML
	// Refresh adds the wall's manual Refresh control to the posture group
	// — the pages with the snapshot transport.
	Refresh bool
}

// topBarScriptTag loads the bar's script, /assets/topbar.js (parent co-1:
// new behavior ships in a new asset within 64 KiB, never in boardspec.js),
// deferred, so the bar — and its native posture disclosure — is complete
// and usable before the script enhances the disclosure to the handoff's
// popover (dc-2; SI-331). A page's own scripts register their listeners
// first, which the diagram editor's Escape guard relies on alongside the
// keystroke's defaultPrevented mark (boarddiagram.js).
const topBarScriptTag = `<script src="/assets/topbar.js" defer></script>`

// renderTopBar draws the bar for one page from its facts and options:
// one row (.topbar-row), then any trailing row, then the script tag.
// Every fact is escaped here; the option fragments arrive pre-rendered by
// the page's own renderer.
func renderTopBar(f *barFacts, o topBarOptions) template.HTML {
	esc := stdhtml.EscapeString
	var b strings.Builder
	b.WriteString(`<header class="topbar" data-testid="topbar"><div class="topbar-row">`)
	b.WriteString(`<a class="wordmark topbar-wordmark" data-testid="topbar-wordmark" href="/"><span class="leafmark" aria-hidden="true"></span>verdi`)
	if o.Surface {
		b.WriteString(`<span class="wordmark-surface">workbench</span>`)
	}
	b.WriteString(`</a><span class="topbar-sep" aria-hidden="true">|</span>`)
	tag := "span"
	if o.Heading {
		tag = "h1"
	}
	b.WriteString(`<` + tag + ` class="topbar-title" data-testid="topbar-title">` + esc(f.Title) + `</` + tag + `>`)
	writeTopBarClassChip(&b, f.Spec)
	b.WriteString(string(o.Chips))
	writeASDPosture(&b, f, o.Refresh)
	if o.Nav != "" {
		b.WriteString(`<nav class="topbar-nav workbench-nav" aria-label="Workbench pages">` + string(o.Nav) + `</nav>`)
	}
	if o.Controls != "" {
		b.WriteString(`<div class="topbar-controls" data-testid="topbar-controls">` + string(o.Controls) + `</div>`)
	}
	b.WriteString(`</div>`)
	b.WriteString(string(o.Tail))
	b.WriteString(topBarScriptTag)
	b.WriteString(`</header>`)
	return template.HTML(b.String()) //nolint:gosec // every fact is escaped above; the option fragments are the page's own rendered markup
}

// writeTopBarClassChip writes the class chip on a page about one spec
// (ac-1): the class's display word, in the class's colour. A spec whose
// facts could not be computed gets a disclosed chip naming why, never a
// silent gap; a spec that declares no class gets none.
func writeTopBarClassChip(b *strings.Builder, spec *barSpec) {
	if spec == nil {
		return
	}
	esc := stdhtml.EscapeString
	if spec.Unproven != "" {
		b.WriteString(`<span class="topbar-chip topbar-chip--disclosed" data-testid="topbar-spec-unproven" title="` + esc(spec.Unproven) + `">spec facts unproven<span class="topbar-sr">: ` + esc(spec.Unproven) + `</span></span>`)
		return
	}
	if spec.Class == "" {
		return
	}
	b.WriteString(`<span class="topbar-chip topbar-chip--class topbar-chip--class-` + esc(spec.Class) + `" data-testid="topbar-class-chip">` + esc(spec.ClassLabel) + `</span>`)
}
