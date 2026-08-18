// The readiness cockpit's server-side renderer: a hybrid layout with a
// continuously visible four-area rail, a visually primary deterministic
// attention queue, and a complete "All concerns" section that never
// hides a proven or unresolved fact. Every state is spoken as its exact
// contract word (proven / violated-with-witness / unproven) beside a
// shape-coded glyph — color is never the only signal. All snapshot text
// is escaped here; nothing on the page mutates or refetches anything.
package workbench

import (
	stdhtml "html"
	"html/template"
	"strings"

	"github.com/jyang234/verdi/internal/readinesspilot"
)

// readinessStateGlyph encodes state by SHAPE, not color: a solid check
// for proven, a saltire for violated-with-witness, a dotted ring for
// unproven. Rendered aria-hidden — the exact state word beside it is the
// accessible signal.
func readinessStateGlyph(state readinesspilot.State) string {
	switch state {
	case readinesspilot.StateProven:
		return "✓"
	case readinesspilot.StateViolated:
		return "✕"
	default:
		return "◌"
	}
}

// writeReadinessState writes the one state chip: exact contract word
// plus shape glyph, always together.
func writeReadinessState(b *strings.Builder, state readinesspilot.State) {
	b.WriteString(`<span class="readiness-state readiness-state--`)
	b.WriteString(string(state))
	b.WriteString(`"><span class="readiness-glyph" aria-hidden="true">`)
	b.WriteString(readinessStateGlyph(state))
	b.WriteString(`</span>`)
	b.WriteString(string(state))
	b.WriteString(`</span>`)
}

// renderReadiness renders the cockpit page for one immutable snapshot.
func renderReadiness(snap readinesspilot.Snapshot) ([]byte, error) {
	var b strings.Builder
	b.WriteString(`<div class="readiness-page">`)
	writeReadinessStale(&b, snap)
	b.WriteString(`<div class="readiness-layout">`)
	writeReadinessRail(&b, snap)
	b.WriteString(`<div class="readiness-main">`)
	writeReadinessQueue(&b, snap)
	writeReadinessAll(&b, snap)
	b.WriteString(`</div>`) // .readiness-main
	b.WriteString(`</div>`) // .readiness-layout
	b.WriteString(`</div>`) // .readiness-page
	return renderPage(pageData{
		Title: "Readiness",
		Nav:   template.HTML(`<a href="/">index</a> <span class="current">readiness</span>`),
		MetaRows: []metaRow{
			{Label: "Target", Value: snap.TargetRef},
			{Label: "Class", Value: snap.TargetClass},
			{Label: "Branch", Value: snap.Branch},
			{Label: "Head", Value: snap.Head},
			{Label: "Request digest", Value: snap.RequestDigest},
		},
		BodyHTML:  template.HTML(b.String()), //nolint:gosec // built above from escaped snapshot text only
		ExtraHTML: template.HTML(`<script src="/assets/readiness.js" defer></script>`),
	})
}

// writeReadinessStale writes the startup-snapshot notice: the snapshot's
// own stale text (it names the exact HEAD and tells the author to
// restart verdi serve after an edit), keyboard-reachable so its
// inspection is observable to the pilot instrumentation.
func writeReadinessStale(b *strings.Builder, snap readinesspilot.Snapshot) {
	b.WriteString(`<aside class="readiness-stale" role="note" tabindex="0" data-readiness-stale="1" aria-label="Startup snapshot notice">`)
	b.WriteString(`<span class="readiness-stale-badge" aria-hidden="true">⏱</span>`)
	b.WriteString(`<span class="readiness-stale-text"><strong>Startup snapshot.</strong> `)
	b.WriteString(stdhtml.EscapeString(snap.StaleNotice))
	b.WriteString(`</span></aside>`)
}

// writeReadinessRail writes the continuously visible four-area rail: the
// snapshot's areas in their fixed order, each station an anchor into its
// complete section, the current-focus area marked in text.
func writeReadinessRail(b *strings.Builder, snap readinesspilot.Snapshot) {
	b.WriteString(`<nav class="readiness-rail" aria-label="Readiness rail"><ol class="readiness-rail-list">`)
	for _, area := range snap.Areas {
		focused := snap.CurrentFocus == area.ID
		b.WriteString(`<li class="readiness-station`)
		if focused {
			b.WriteString(` readiness-station--focus`)
		}
		b.WriteString(`" data-area-id="`)
		b.WriteString(stdhtml.EscapeString(string(area.ID)))
		b.WriteString(`"><a class="readiness-station-link" href="#area-`)
		b.WriteString(stdhtml.EscapeString(string(area.ID)))
		b.WriteString(`"`)
		if focused {
			b.WriteString(` aria-current="true"`)
		}
		b.WriteString(`><span class="readiness-station-glyph readiness-station-glyph--`)
		b.WriteString(string(area.State))
		b.WriteString(`" aria-hidden="true">`)
		b.WriteString(readinessStateGlyph(area.State))
		b.WriteString(`</span><span class="readiness-station-label">`)
		b.WriteString(stdhtml.EscapeString(area.Label))
		b.WriteString(`</span>`)
		writeReadinessState(b, area.State)
		if focused {
			b.WriteString(`<span class="readiness-focus">current focus</span>`)
		}
		b.WriteString(`</a></li>`)
	}
	b.WriteString(`</ol></nav>`)
}

// writeReadinessQueue writes the visually primary attention queue in the
// snapshot's exact deterministic order. An empty queue states its honest
// reason instead of rendering nothing.
func writeReadinessQueue(b *strings.Builder, snap readinesspilot.Snapshot) {
	b.WriteString(`<section class="readiness-queue" aria-label="Attention queue">`)
	b.WriteString(`<h2 class="readiness-heading">Attention queue</h2>`)
	if len(snap.Attention) == 0 {
		b.WriteString(`<p class="readiness-queue-empty"><span class="readiness-glyph" aria-hidden="true">✓</span>The attention queue is empty: every concern is proven.</p>`)
		b.WriteString(`</section>`)
		return
	}
	b.WriteString(`<p class="readiness-queue-note">Fix-next order, fixed by the snapshot — not a suggestion engine.</p>`)
	b.WriteString(`<ol class="readiness-queue-list">`)
	for _, concern := range snap.Attention {
		b.WriteString(`<li>`)
		writeReadinessConcern(b, concern, "readiness-card", true)
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ol></section>`)
}

// writeReadinessAll writes the complete concern enumeration grouped by
// area — every fact the snapshot carries, proven ones included, with
// nothing collapsed or hidden.
func writeReadinessAll(b *strings.Builder, snap readinesspilot.Snapshot) {
	b.WriteString(`<section class="readiness-all" aria-label="All concerns">`)
	b.WriteString(`<h2 class="readiness-heading">All concerns</h2>`)
	for _, area := range snap.Areas {
		b.WriteString(`<section class="readiness-area" id="area-`)
		b.WriteString(stdhtml.EscapeString(string(area.ID)))
		b.WriteString(`" data-area-id="`)
		b.WriteString(stdhtml.EscapeString(string(area.ID)))
		b.WriteString(`"><h3 class="readiness-area-head">`)
		b.WriteString(stdhtml.EscapeString(area.Label))
		writeReadinessState(b, area.State)
		b.WriteString(`</h3>`)
		for _, concern := range snap.AllConcerns {
			if concern.Area != area.ID {
				continue
			}
			writeReadinessConcern(b, concern, "readiness-row", false)
		}
		b.WriteString(`</section>`)
	}
	b.WriteString(`</section>`)
}

// writeReadinessConcern writes one concern's complete facts: exact state
// word, blocking/timing flags, the source-provided work-class label
// (omitted entirely when the source supplies none — never guessed),
// summary, exact witnesses, and exactly one destination (board link or
// CLI token vector) for an unresolved concern, none for a proven one.
// anchored selects the queue edition, whose id links into the complete
// section.
func writeReadinessConcern(b *strings.Builder, concern readinesspilot.Concern, class string, anchored bool) {
	id := stdhtml.EscapeString(concern.ID)
	area := stdhtml.EscapeString(string(concern.Area))
	b.WriteString(`<article class="`)
	b.WriteString(class)
	b.WriteString(` readiness-concern--`)
	b.WriteString(string(concern.State))
	b.WriteString(`" data-concern-id="`)
	b.WriteString(id)
	b.WriteString(`" data-area-id="`)
	b.WriteString(area)
	b.WriteString(`"`)
	if !anchored {
		b.WriteString(` id="concern-`)
		b.WriteString(id)
		b.WriteString(`"`)
	}
	b.WriteString(`><header class="readiness-concern-head">`)
	writeReadinessState(b, concern.State)
	if concern.Blocking {
		b.WriteString(`<span class="readiness-flag readiness-flag--blocking">blocking</span>`)
	}
	if concern.Timing == readinesspilot.TimingEventual {
		b.WriteString(`<span class="readiness-flag readiness-flag--eventual">eventual</span>`)
	}
	if concern.WorkClass != "" {
		b.WriteString(`<span class="readiness-workclass">`)
		b.WriteString(stdhtml.EscapeString(string(concern.WorkClass)))
		b.WriteString(`</span>`)
	}
	b.WriteString(`</header>`)
	b.WriteString(`<p class="readiness-summary">`)
	b.WriteString(stdhtml.EscapeString(concern.Summary))
	b.WriteString(`</p>`)
	if anchored {
		b.WriteString(`<a class="readiness-concern-link" href="#concern-`)
		b.WriteString(id)
		b.WriteString(`"><code>`)
		b.WriteString(id)
		b.WriteString(`</code></a>`)
	} else {
		b.WriteString(`<code class="readiness-concern-id">`)
		b.WriteString(id)
		b.WriteString(`</code>`)
	}
	if len(concern.Witnesses) > 0 {
		b.WriteString(`<ul class="readiness-witnesses">`)
		for _, witness := range concern.Witnesses {
			b.WriteString(`<li><code>`)
			b.WriteString(stdhtml.EscapeString(witness))
			b.WriteString(`</code></li>`)
		}
		b.WriteString(`</ul>`)
	}
	writeReadinessDestination(b, concern.Destination)
	b.WriteString(`</article>`)
}

// writeReadinessDestination writes the concern's one corrective path:
// a board link (new tab, noopener) or the exact CLI token vector —
// token elements, never a joined shell string. A proven concern carries
// neither and writes nothing.
func writeReadinessDestination(b *strings.Builder, dest readinesspilot.Destination) {
	if dest.BoardPath != "" {
		b.WriteString(`<p class="readiness-dest"><a class="readiness-board-link" href="`)
		b.WriteString(stdhtml.EscapeString(dest.BoardPath))
		b.WriteString(`" target="_blank" rel="noopener">Open the board<span aria-hidden="true"> ↗</span></a></p>`)
		return
	}
	if len(dest.CLI) == 0 {
		return
	}
	b.WriteString(`<p class="readiness-dest readiness-cli" data-readiness-cli="1">`)
	for i, token := range dest.CLI {
		if i > 0 {
			b.WriteString(` `)
		}
		b.WriteString(`<code class="readiness-cli-token">`)
		b.WriteString(stdhtml.EscapeString(token))
		b.WriteString(`</code>`)
	}
	b.WriteString(`</p>`)
}
