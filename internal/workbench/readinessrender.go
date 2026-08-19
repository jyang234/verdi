// The readiness cockpit's server-side renderer (F-01 corrected form,
// SI-125): the page answers "where am I?" then "what should I focus on
// next?". An orientation block leads with the snapshot's exact target
// title and current step; a four-step process rail keeps the plain area
// labels; a ranked focus list shows the first three priorities with the
// exact remainder behind one inline disclosure; completed checks hold
// every proven fact. Every state is summarized with a plain label
// (Ready / Needs attention / Not enough evidence yet) while each row's
// Technical details retain the exact formal state, ids, flags,
// witnesses, and destination data — nothing is dropped, reclassified,
// or synthesized. All snapshot text is escaped here; nothing on the
// page mutates or refetches anything, and the only interactive state is
// the ephemeral open/closed state of native disclosures.
package workbench

import (
	stdhtml "html"
	"html/template"
	"strconv"
	"strings"

	"github.com/jyang234/verdi/internal/readinesspilot"
)

// readinessFocusVisible is how many ranked priorities render outside the
// inline disclosure (the F-01 mockup's fixed top three).
const readinessFocusVisible = 3

// readinessPlainState maps a formal three-valued state to its primary
// plain label. The formal word itself stays in Technical details.
func readinessPlainState(state readinesspilot.State) string {
	switch state {
	case readinesspilot.StateProven:
		return "Ready"
	case readinesspilot.StateViolated:
		return "Needs attention"
	default:
		return "Not enough evidence yet"
	}
}

// writeReadinessState writes the one plain state chip; the modifier class
// keeps the formal state machine-readable.
func writeReadinessState(b *strings.Builder, state readinesspilot.State) {
	b.WriteString(`<span class="readiness-state readiness-state--`)
	b.WriteString(string(state))
	b.WriteString(`">`)
	b.WriteString(readinessPlainState(state))
	b.WriteString(`</span>`)
}

// readinessDownstreamViolated counts the known downstream problems: the
// concerns whose formal state is violated-with-witness and whose area
// occurs strictly after CurrentFocus in the snapshot's area order. With
// no current focus the count is zero by definition.
func readinessDownstreamViolated(snap readinesspilot.Snapshot) int {
	if snap.CurrentFocus == "" {
		return 0
	}
	order := make(map[readinesspilot.AreaID]int, len(snap.Areas))
	for i, area := range snap.Areas {
		order[area.ID] = i
	}
	focus, ok := order[snap.CurrentFocus]
	if !ok {
		return 0
	}
	count := 0
	for _, concern := range snap.AllConcerns {
		if concern.State == readinesspilot.StateViolated && order[concern.Area] > focus {
			count++
		}
	}
	return count
}

// readinessEmission carries the per-render shared lookups: area labels by
// id and the first-row-per-area fragment anchors.
type readinessEmission struct {
	labels   map[readinesspilot.AreaID]string
	anchored map[readinesspilot.AreaID]bool
}

func newReadinessEmission(snap readinesspilot.Snapshot) *readinessEmission {
	labels := make(map[readinesspilot.AreaID]string, len(snap.Areas))
	for _, area := range snap.Areas {
		labels[area.ID] = area.Label
	}
	return &readinessEmission{labels: labels, anchored: make(map[readinesspilot.AreaID]bool, len(snap.Areas))}
}

// renderReadiness renders the cockpit page for one immutable snapshot.
func renderReadiness(snap readinesspilot.Snapshot) ([]byte, error) {
	em := newReadinessEmission(snap)
	var b strings.Builder
	b.WriteString(`<div class="readiness-page">`)
	writeReadinessOrientation(&b, snap)
	writeReadinessRail(&b, snap)
	writeReadinessFocus(&b, snap, em)
	writeReadinessCompleted(&b, snap, em)
	b.WriteString(`</div>`)
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

// writeReadinessOrientation answers "where am I?": the exact target
// title, the current step (or plain completion), the page's purpose, and
// the startup-snapshot notice.
func writeReadinessOrientation(b *strings.Builder, snap readinesspilot.Snapshot) {
	b.WriteString(`<section class="readiness-orient" aria-label="Where you are">`)
	b.WriteString(`<div class="readiness-orient-main">`)
	b.WriteString(`<p class="readiness-eyebrow">Where you are</p>`)
	b.WriteString(`<h2 class="readiness-title">`)
	b.WriteString(stdhtml.EscapeString(snap.TargetTitle))
	b.WriteString(`</h2>`)
	b.WriteString(`<p class="readiness-step">`)
	if snap.CurrentFocus == "" {
		b.WriteString(`All four steps are complete.`)
	} else {
		for i, area := range snap.Areas {
			if area.ID != snap.CurrentFocus {
				continue
			}
			b.WriteString(`Step `)
			b.WriteString(strconv.Itoa(i + 1))
			b.WriteString(` of 4 — `)
			b.WriteString(stdhtml.EscapeString(area.Label))
			break
		}
	}
	b.WriteString(`</p>`)
	b.WriteString(`<p class="readiness-purpose">This is a startup snapshot of readiness for the current design work.</p>`)
	b.WriteString(`</div>`)
	writeReadinessStale(b, snap)
	b.WriteString(`</section>`)
}

// writeReadinessStale writes the startup-snapshot notice: the snapshot's
// own stale text (it names the exact HEAD and tells the author to
// restart verdi serve after an edit), keyboard-reachable so its
// inspection is observable to the pilot instrumentation.
func writeReadinessStale(b *strings.Builder, snap readinesspilot.Snapshot) {
	b.WriteString(`<aside class="readiness-stale" role="note" tabindex="0" data-readiness-stale="1" aria-label="Startup snapshot notice">`)
	b.WriteString(`<span class="readiness-stale-text"><strong>Startup snapshot.</strong> `)
	b.WriteString(stdhtml.EscapeString(snap.StaleNotice))
	b.WriteString(`</span></aside>`)
}

// writeReadinessRail writes the four-step process rail in snapshot order:
// plain labels, plain state summaries (the exact formal state rides the
// data-state attribute), step numbers, fragment anchors, and the current
// step marked aria-current.
func writeReadinessRail(b *strings.Builder, snap readinesspilot.Snapshot) {
	b.WriteString(`<nav class="readiness-rail" aria-label="Readiness rail"><ol class="readiness-rail-list">`)
	for i, area := range snap.Areas {
		focused := snap.CurrentFocus == area.ID
		b.WriteString(`<li class="readiness-station`)
		if focused {
			b.WriteString(` readiness-station--focus`)
		}
		b.WriteString(`" data-area-id="`)
		b.WriteString(stdhtml.EscapeString(string(area.ID)))
		b.WriteString(`" data-state="`)
		b.WriteString(string(area.State))
		b.WriteString(`"><a class="readiness-station-link" href="#area-`)
		b.WriteString(stdhtml.EscapeString(string(area.ID)))
		b.WriteString(`"`)
		if focused {
			b.WriteString(` aria-current="step"`)
		}
		b.WriteString(`><span class="readiness-station-num">`)
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString(`</span><span class="readiness-station-label">`)
		b.WriteString(stdhtml.EscapeString(area.Label))
		b.WriteString(`</span>`)
		writeReadinessState(b, area.State)
		b.WriteString(`</a></li>`)
	}
	b.WriteString(`</ol></nav>`)
}

// writeReadinessFocus writes the ranked focus list: the snapshot's exact
// attention order, first three visible, the exact remainder behind one
// inline disclosure whose open/closed labels are fixed. An empty list
// states its honest reason.
func writeReadinessFocus(b *strings.Builder, snap readinesspilot.Snapshot, em *readinessEmission) {
	b.WriteString(`<section class="readiness-queue" id="readiness-focus" aria-label="Focus next">`)
	b.WriteString(`<h2 class="readiness-heading">Focus next</h2>`)
	if len(snap.Attention) == 0 {
		b.WriteString(`<p class="readiness-queue-empty">Nothing needs attention: every check in this snapshot is proven.</p>`)
		b.WriteString(`</section>`)
		return
	}
	b.WriteString(`<p class="readiness-queue-note">Ranked from the snapshot's recorded facts; unknowns stay unknown.</p>`)
	b.WriteString(`<p class="readiness-downstream">Known problems in later steps: `)
	b.WriteString(strconv.Itoa(readinessDownstreamViolated(snap)))
	b.WriteString(`</p>`)

	visible := snap.Attention
	var rest []readinesspilot.Concern
	if len(visible) > readinessFocusVisible {
		rest = visible[readinessFocusVisible:]
		visible = visible[:readinessFocusVisible]
	}
	b.WriteString(`<ol class="readiness-queue-list">`)
	for i, concern := range visible {
		b.WriteString(`<li>`)
		writeReadinessConcern(b, concern, em, "readiness-card", i+1)
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ol>`)
	if len(rest) > 0 {
		b.WriteString(`<details class="readiness-more"><summary class="readiness-more-summary">`)
		// vocab:identity — "-closed" is this disclosure's CSS class fragment (collapsed label), not the lifecycle state.
		b.WriteString(`<span class="readiness-more-closed">`)
		b.WriteString(strconv.Itoa(len(rest)))
		if len(rest) == 1 {
			b.WriteString(` more item`)
		} else {
			b.WriteString(` more items`)
		}
		b.WriteString(`</span><span class="readiness-more-open">Show fewer</span>`)
		b.WriteString(`</summary><ol class="readiness-queue-list readiness-queue-rest" start="`)
		b.WriteString(strconv.Itoa(readinessFocusVisible + 1))
		b.WriteString(`">`)
		for i, concern := range rest {
			b.WriteString(`<li>`)
			writeReadinessConcern(b, concern, em, "readiness-card", readinessFocusVisible+i+1)
			b.WriteString(`</li>`)
		}
		b.WriteString(`</ol></details>`)
	}
	b.WriteString(`</section>`)
}

// writeReadinessCompleted writes the completed checks: every proven
// concern, in the snapshot's existing order, with full technical details.
func writeReadinessCompleted(b *strings.Builder, snap readinesspilot.Snapshot, em *readinessEmission) {
	b.WriteString(`<section class="readiness-completed" id="readiness-completed" aria-label="Completed checks">`)
	b.WriteString(`<h2 class="readiness-heading">Completed checks</h2>`)
	b.WriteString(`<p class="readiness-completed-note">Checks already proven in this snapshot.</p>`)
	for _, concern := range snap.AllConcerns {
		if concern.State != readinesspilot.StateProven {
			continue
		}
		writeReadinessConcern(b, concern, em, "readiness-row", 0)
	}
	b.WriteString(`</section>`)
}

// writeReadinessConcern writes one concern exactly once: source summary
// as the primary copy, plain state chip, complete Technical details
// (exact formal state, concern id, area id, blocking, timing, work class
// when present, witnesses, destination data), and — for an unresolved
// concern — its one usable action. rank > 0 renders the focus ranking.
// The first concern of each area carries that area's fragment anchor.
func writeReadinessConcern(b *strings.Builder, concern readinesspilot.Concern, em *readinessEmission, class string, rank int) {
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
	if !em.anchored[concern.Area] {
		em.anchored[concern.Area] = true
		b.WriteString(` id="area-`)
		b.WriteString(area)
		b.WriteString(`"`)
	}
	b.WriteString(`>`)
	if rank > 0 {
		b.WriteString(`<span class="readiness-rank">`)
		b.WriteString(strconv.Itoa(rank))
		b.WriteString(`</span>`)
	}
	b.WriteString(`<div class="readiness-copy">`)
	b.WriteString(`<p class="readiness-stage">`)
	b.WriteString(stdhtml.EscapeString(em.labels[concern.Area]))
	b.WriteString(`</p>`)
	b.WriteString(`<p class="readiness-summary">`)
	b.WriteString(stdhtml.EscapeString(concern.Summary))
	b.WriteString(`</p>`)
	writeReadinessState(b, concern.State)
	writeReadinessTech(b, concern)
	writeReadinessDestination(b, concern.Destination)
	b.WriteString(`</div></article>`)
}

// writeReadinessTech writes the concern's Technical details disclosure:
// every formal fact, verbatim, none normalized away.
func writeReadinessTech(b *strings.Builder, concern readinesspilot.Concern) {
	b.WriteString(`<details class="readiness-tech"><summary>Technical details</summary><dl class="readiness-tech-facts">`)
	writeReadinessFact(b, "State", string(concern.State))
	writeReadinessFact(b, "Concern", concern.ID)
	writeReadinessFact(b, "Area", string(concern.Area))
	writeReadinessFact(b, "Blocking", strconv.FormatBool(concern.Blocking))
	writeReadinessFact(b, "Timing", string(concern.Timing))
	if concern.WorkClass != "" {
		writeReadinessFact(b, "Work class", string(concern.WorkClass))
	}
	if len(concern.Witnesses) > 0 {
		b.WriteString(`<dt>Witnesses</dt><dd><ul class="readiness-witnesses">`)
		for _, witness := range concern.Witnesses {
			b.WriteString(`<li><code>`)
			b.WriteString(stdhtml.EscapeString(witness))
			b.WriteString(`</code></li>`)
		}
		b.WriteString(`</ul></dd>`)
	}
	if concern.Destination.BoardPath != "" {
		writeReadinessFact(b, "Destination", concern.Destination.BoardPath)
	} else if len(concern.Destination.CLI) > 0 {
		b.WriteString(`<dt>Destination</dt><dd>`)
		for i, token := range concern.Destination.CLI {
			if i > 0 {
				b.WriteString(` `)
			}
			b.WriteString(`<code>`)
			b.WriteString(stdhtml.EscapeString(token))
			b.WriteString(`</code>`)
		}
		b.WriteString(`</dd>`)
	}
	b.WriteString(`</dl></details>`)
}

func writeReadinessFact(b *strings.Builder, label, value string) {
	b.WriteString(`<dt>`)
	b.WriteString(label)
	b.WriteString(`</dt><dd><code>`)
	b.WriteString(stdhtml.EscapeString(value))
	b.WriteString(`</code></dd>`)
}

// writeReadinessDestination writes the concern's one usable corrective
// action: a board link (new tab, noopener) or the exact CLI token vector
// — token elements, never a joined shell string. A proven concern
// carries neither and writes nothing.
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
	// tabindex 0: a keyboard-only author must be able to reach the vector
	// to select and copy it (Task 4 browser-exposed defect).
	b.WriteString(`<p class="readiness-dest readiness-cli" data-readiness-cli="1" tabindex="0" aria-label="CLI fallback">`)
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
