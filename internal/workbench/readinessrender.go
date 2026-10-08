// The readiness page's server-side renderer (spec/readiness-page-v2;
// ledger SI-339; handoff screen 3b). The page renders one per-request
// readiness snapshot — the one readiness seam (SI-339 (1)) — in the
// design's layout: where you are (the spec, its class chip, its branch),
// the four-step stepper with each step's state and one line of count and
// reason, the sentence explaining the order, Focus next, Known problems in
// later steps, and Completed checks. "Open the wall →" rides the shared
// bar's controls slot (SI-339 (9)); the per-request derivation stamp stays
// in the body with its element, role, aria label and data hook unchanged.
//
// Focus next lists every concern of the current step, marked with its
// step and "now"; the later steps' concerns sit behind one inline
// disclosure, each marked "later" with what it waits on (SI-339 (3), (4)).
// Each unresolved row leads with its source-derived guidance, its fact,
// timing and blocking flag filed in the technical disclosure (SI-339 (2));
// human-review work is labeled plainly with the formal obligation as
// secondary text (SI-339 (5)); every state reads as the plain triad —
// Proven, Violated with its witness, Not enough evidence yet (SI-339
// (10)). Everything here is a projection of the snapshot's own facts:
// nothing is derived, dropped, reclassified, or synthesized, and the only
// interactive state is the open/closed state of native disclosures. All
// snapshot text is escaped here.
//
// The body is one renderer the page shares with the wall's Readiness tab
// (SI-368 (1)): writeReadinessBody renders one snapshot on one surface,
// and a surface supplies only its state words (SI-368 (14)), its scoping
// class and purpose line, and, on the wall, each concern's wall target
// (SI-368 (16)). The page's bytes are the ones it had before the body was
// shared (TestReadinessPage_BytesUnchangedByTheSharedRenderer).
//
// writeReadinessFact is shared with the wall shell, which reuses the
// readiness-* classes; its output does not change.
package workbench

import (
	"context"
	stdhtml "html"
	"html/template"
	"net/url"
	"strconv"
	"strings"

	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/readinesspilot"
)

// readinessWords is one readiness surface's plain state words (SI-368
// (14)): each formal state's chip label and its word in the stepper's
// counts. The formal word itself stays in Technical details on every
// surface, and a violated row's witness is there too.
type readinessWords struct {
	proven, violated, unproven                string
	provenCount, violatedCount, unprovenCount string
}

// readinessPageWords is the readiness page's plain triad (SI-339 (10)):
// Proven, Violated, Not enough evidence yet.
func readinessPageWords() readinessWords {
	return readinessWords{
		proven: "Proven", violated: "Violated", unproven: "Not enough evidence yet",
		provenCount: "proven", violatedCount: "violated", unprovenCount: "not enough evidence yet",
	}
}

// label is the state's chip label.
func (w readinessWords) label(state readinesspilot.State) string {
	switch state {
	case readinesspilot.StateProven:
		return w.proven
	case readinesspilot.StateViolated:
		return w.violated
	default:
		return w.unproven
	}
}

// count is the state as a count's word in the stepper line: "2
// violated", "1 proven".
func (w readinessWords) count(state readinesspilot.State) string {
	switch state {
	case readinesspilot.StateProven:
		return w.provenCount
	case readinesspilot.StateViolated:
		return w.violatedCount
	default:
		return w.unprovenCount
	}
}

// writeReadinessState writes the one plain state chip in the surface's
// words; the modifier class keeps the formal state machine-readable.
func writeReadinessState(b *strings.Builder, words readinessWords, state readinesspilot.State) {
	b.WriteString(`<span class="readiness-state readiness-state--`)
	b.WriteString(string(state))
	b.WriteString(`">`)
	b.WriteString(words.label(state))
	b.WriteString(`</span>`)
}

// readinessSurface is what one surface hands the shared body renderer
// (SI-368 (1)): its state words (SI-368 (14)), the body's scoping class
// and test id, the orientation's line saying what the surface derives,
// and whether the body renders inside the wall. Inside the wall (the
// drawer's Readiness tab) each concern carries its wall target, from
// targets (SI-368 (16)), the page's new-tab board link is not reused, and
// no wall address is named, since the reader is on the wall.
type readinessSurface struct {
	words   readinessWords
	class   string
	testID  string
	purpose string
	wall    bool
	targets map[string]readinessTarget
}

// readinessPageSurface is the readiness page's surface: its own words,
// the readiness-standalone scope its stylesheet block reads, and its
// purpose line.
func readinessPageSurface() readinessSurface {
	return readinessSurface{
		words:   readinessPageWords(),
		class:   "readiness-page readiness-standalone",
		purpose: "This page derives readiness for the current design work on every request.",
	}
}

// readinessStepCounts counts one step's concerns by state.
type readinessStepCount struct {
	violated, unproven, proven int
}

func readinessStepCounts(snap readinesspilot.Snapshot, area readinesspilot.AreaID) readinessStepCount {
	var c readinessStepCount
	for _, concern := range snap.AllConcerns {
		if concern.Area != area {
			continue
		}
		switch concern.State {
		case readinesspilot.StateViolated:
			c.violated++
		case readinesspilot.StateProven:
			c.proven++
		default:
			c.unproven++
		}
	}
	return c
}

// readinessStepReason is the stepper line's reason from the step order
// (SI-339 (8)): a proven step is complete wherever it sits; the current
// step is the current focus; any other step waits on the current step's
// label. The second value is the reason's machine kind. With no current
// focus every step is proven, so no step waits.
func readinessStepReason(snap readinesspilot.Snapshot, area readinesspilot.Area) (string, string) {
	if area.State == readinesspilot.StateProven {
		return "complete", "complete"
	}
	if area.ID == snap.CurrentFocus {
		return "current focus", "current-focus"
	}
	return "waits on " + readinessFocusLabel(snap), "waits"
}

// readinessCurrentStep is the current step's position, 1 to 4, and its
// plain label; 0 and "" when the snapshot names no current focus, every
// step being proven.
func readinessCurrentStep(snap readinesspilot.Snapshot) (int, string) {
	for i, area := range snap.Areas {
		if area.ID == snap.CurrentFocus {
			return i + 1, area.Label
		}
	}
	return 0, ""
}

// readinessFocusLabel is the current step's plain label, or a plain
// disclosure when the snapshot names none.
func readinessFocusLabel(snap readinesspilot.Snapshot) string {
	for _, area := range snap.Areas {
		if area.ID == snap.CurrentFocus {
			return area.Label
		}
	}
	return "an earlier step"
}

// readinessStepLine is the stepper's one line in the surface's words: the
// counts by state, zero counts omitted, then the reason.
func readinessStepLine(words readinessWords, c readinessStepCount, reason string) string {
	var parts []string
	for _, p := range []struct {
		n     int
		state readinesspilot.State
	}{
		{c.violated, readinesspilot.StateViolated},
		{c.unproven, readinesspilot.StateUnproven},
		{c.proven, readinesspilot.StateProven},
	} {
		if p.n > 0 {
			parts = append(parts, strconv.Itoa(p.n)+" "+words.count(p.state))
		}
	}
	counts := "no concerns"
	if len(parts) > 0 {
		counts = strings.Join(parts, ", ")
	}
	return counts + " — " + reason
}

// readinessKnownProblems lists the known problems in later steps (SI-339
// (7)): the violated concerns whose step comes strictly after the current
// one, in the attention list's own order. Unproven is not a known
// problem; with no current focus there are no later steps.
func readinessKnownProblems(snap readinesspilot.Snapshot) []readinesspilot.Concern {
	if snap.CurrentFocus == "" {
		return nil
	}
	order := readinessAreaOrder(snap)
	focus, ok := order[snap.CurrentFocus]
	if !ok {
		return nil
	}
	var known []readinesspilot.Concern
	for _, concern := range snap.Attention {
		if concern.State == readinesspilot.StateViolated && order[concern.Area] > focus {
			known = append(known, concern)
		}
	}
	return known
}

func readinessAreaOrder(snap readinesspilot.Snapshot) map[readinesspilot.AreaID]int {
	order := make(map[readinesspilot.AreaID]int, len(snap.Areas))
	for i, area := range snap.Areas {
		order[area.ID] = i
	}
	return order
}

// readinessFragment is a concern id as a fragment identifier: each path
// segment percent-encoded, so a hash, a percent sign or a space inside an
// id cannot cut the fragment short, while the browser's percent-decoded
// fragment still matches the row's verbatim id attribute.
func readinessFragment(id string) string {
	parts := strings.Split(id, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

// readinessEmission carries the per-render shared lookups: the surface,
// area labels by id, the area order, the current step's label, and which
// areas have had their fragment anchor written.
type readinessEmission struct {
	surface    readinessSurface
	labels     map[readinesspilot.AreaID]string
	order      map[readinesspilot.AreaID]int
	focusLabel string
	anchored   map[readinesspilot.AreaID]bool
}

func newReadinessEmission(snap readinesspilot.Snapshot, surface readinessSurface) *readinessEmission {
	labels := make(map[readinesspilot.AreaID]string, len(snap.Areas))
	for _, area := range snap.Areas {
		labels[area.ID] = area.Label
	}
	return &readinessEmission{
		surface:    surface,
		labels:     labels,
		order:      readinessAreaOrder(snap),
		focusLabel: readinessFocusLabel(snap),
		anchored:   make(map[readinesspilot.AreaID]bool, len(snap.Areas)),
	}
}

// anchor returns the area's fragment anchor attribute the first time the
// area is asked for, so each area's `#area-<id>` lands on its first row's
// list item, and nothing after.
func (em *readinessEmission) anchor(area readinesspilot.AreaID) string {
	if em.anchored[area] {
		return ""
	}
	em.anchored[area] = true
	return ` id="area-` + stdhtml.EscapeString(string(area)) + `"`
}

// later reports whether the concern's step comes strictly after the
// current one.
func (em *readinessEmission) later(snap readinesspilot.Snapshot, concern readinesspilot.Concern) bool {
	focus, ok := em.order[snap.CurrentFocus]
	return ok && em.order[concern.Area] > focus
}

// writeReadinessBody writes the readiness body for one immutable
// snapshot on one surface: the one body renderer the readiness page and
// the wall's Readiness tab share (SI-368 (1)). Where you are, the
// stepper, the order sentence, Focus next, Known problems in later steps
// and Completed checks are the snapshot's own facts on every surface;
// the surface supplies only its words, scope, purpose line and wall
// targets. mdl is the store's resolved operating model, whose display
// word the class chip speaks (nil renders the bare id, like a model with
// no renames).
func writeReadinessBody(b *strings.Builder, mdl *model.Model, snap readinesspilot.Snapshot, surface readinessSurface) {
	em := newReadinessEmission(snap, surface)
	b.WriteString(`<div class="` + surface.class + `"`)
	if surface.testID != "" {
		b.WriteString(` data-testid="` + surface.testID + `"`)
	}
	b.WriteString(`>`)
	writeReadinessOrientation(b, mdl, snap, em)
	writeReadinessStepper(b, snap, em)
	b.WriteString(`<p class="readiness-order" data-testid="readiness-order">The four steps run in this order: define the work, define success, check constraints, then get approval. Later steps report their known problems and wait on the current step; the current step's items are what move this design forward now.</p>`)
	b.WriteString(`<div class="readiness-columns">`)
	writeReadinessFocus(b, snap, em)
	b.WriteString(`<div class="readiness-aside">`)
	if snap.CurrentFocus != "" {
		writeReadinessKnown(b, snap, em)
	}
	writeReadinessCompleted(b, snap, em)
	b.WriteString(`</div></div></div>`)
}

// renderReadiness renders the page for one immutable snapshot, its top
// bar stating the checkout at root (renderPage) and carrying the wall
// link; mdl is the store's resolved operating model, whose display word
// the class chip speaks (nil renders the bare id, like a model with no
// renames).
func renderReadiness(ctx context.Context, root string, mdl *model.Model, snap readinesspilot.Snapshot) ([]byte, error) {
	var b strings.Builder
	writeReadinessBody(&b, mdl, snap, readinessPageSurface())
	// No MetaRows: the shared shell would render them BEFORE the body, so
	// the target's technical facts would precede the orientation. They
	// live in the orientation's own trailing disclosure instead
	// (writeReadinessTargetTech).
	return renderPage(ctx, root, pageData{
		Title:       "Readiness",
		Nav:         template.HTML(`<a href="/">index</a> <span class="current">readiness</span>`),
		BodyHTML:    template.HTML(b.String()), //nolint:gosec // built above from escaped snapshot text only
		ExtraHTML:   template.HTML(`<script src="/assets/readiness.js" defer></script>`),
		BarControls: readinessWallLink(snap),
	})
}

// readinessWallLink is the bar's "Open the wall →" control (SI-339 (9)):
// the snapshot's wall address, with its own class — never the per-concern
// destination link's, which the instrumentation and the pilot suite
// select by class. A snapshot with no wall address renders no link; the
// orientation discloses that.
func readinessWallLink(snap readinesspilot.Snapshot) template.HTML {
	if snap.BoardPath == "" {
		return ""
	}
	return template.HTML(`<a class="btn-primary readiness-wall-link" data-testid="readiness-wall-link" href="` + stdhtml.EscapeString(snap.BoardPath) + `">Open the wall<span aria-hidden="true"> →</span></a>`) //nolint:gosec // the address is escaped above
}

// writeReadinessOrientation answers "where am I?": the exact target
// title with its class chip and its spec and branch refs, the current
// step (or plain completion), the surface's purpose, the target's
// technical facts, and the derivation stamp. Off the wall, a snapshot
// with no wall address says so.
func writeReadinessOrientation(b *strings.Builder, mdl *model.Model, snap readinesspilot.Snapshot, em *readinessEmission) {
	esc := stdhtml.EscapeString
	b.WriteString(`<section class="readiness-orient" aria-label="Where you are">`)
	b.WriteString(`<div class="readiness-orient-main">`)
	b.WriteString(`<p class="readiness-eyebrow">Where you are</p>`)
	b.WriteString(`<div class="readiness-title-row">`)
	b.WriteString(`<h2 class="readiness-title">` + esc(snap.TargetTitle) + `</h2>`)
	// The class chip speaks the store's display word (plain vocabulary);
	// its class and data attribute keep the bare id.
	b.WriteString(`<span class="readiness-class-chip readiness-class-chip--` + esc(snap.TargetClass) + `" data-testid="readiness-class-chip" data-class="` + esc(snap.TargetClass) + `">` + esc(mdl.DisplayClass(snap.TargetClass)) + `</span>`)
	b.WriteString(`<p class="readiness-refs"><code class="readiness-ref" data-testid="readiness-target-ref">` + esc(snap.TargetRef) + `</code><span class="readiness-dot" aria-hidden="true">·</span><code class="readiness-ref" data-testid="readiness-branch">` + esc(snap.Branch) + `</code></p>`)
	b.WriteString(`</div>`)
	b.WriteString(`<p class="readiness-step">`)
	if snap.CurrentFocus == "" {
		b.WriteString(`All four steps are complete.`)
	} else if step, label := readinessCurrentStep(snap); step > 0 {
		b.WriteString(`Step ` + strconv.Itoa(step) + ` of 4 — ` + esc(label))
	}
	b.WriteString(`</p>`)
	b.WriteString(`<p class="readiness-purpose">` + em.surface.purpose + `</p>`)
	if snap.BoardPath == "" && !em.surface.wall {
		b.WriteString(`<p class="readiness-wall-absent">Open the wall: no wall address is known for this request.</p>`)
	}
	writeReadinessTargetTech(b, snap)
	b.WriteString(`</div>`)
	writeReadinessStale(b, snap)
	b.WriteString(`</section>`)
}

// writeReadinessTargetTech writes the target's exact technical facts —
// ref, class, branch, HEAD, request digest, verbatim — as a directly
// accessible disclosure AFTER the title, step, and purpose, so plain
// orientation always precedes technical metadata in reading order.
func writeReadinessTargetTech(b *strings.Builder, snap readinesspilot.Snapshot) {
	b.WriteString(`<details class="readiness-tech readiness-target-tech"><summary>Target technical details</summary><dl class="readiness-tech-facts">`)
	writeReadinessFact(b, "Target", snap.TargetRef)
	writeReadinessFact(b, "Class", snap.TargetClass)
	writeReadinessFact(b, "Branch", snap.Branch)
	writeReadinessFact(b, "Head", snap.Head)
	writeReadinessFact(b, "Request digest", snap.RequestDigest)
	b.WriteString(`</dl></details>`)
}

// writeReadinessStale writes the derivation stamp (spec/readiness-recovery
// ac-2; readiness-page-v2 ac-4): the snapshot's own StaleNotice text,
// which names the exact HEAD this request's derivation looked at. The
// class names, role, data attribute, and tabindex are the pilot's original
// chrome — kept so the stale-notice-inspected instrumentation keeps
// working; only the visible label and the accessible name say what the
// line is.
func writeReadinessStale(b *strings.Builder, snap readinesspilot.Snapshot) {
	b.WriteString(`<aside class="readiness-stale" role="note" tabindex="0" data-readiness-stale="1" aria-label="Derivation stamp">`)
	b.WriteString(`<span class="readiness-stale-text"><strong>Derivation stamp.</strong> `)
	b.WriteString(stdhtml.EscapeString(snap.StaleNotice))
	b.WriteString(`</span></aside>`)
}

// writeReadinessStepper writes the four-step stepper in snapshot order:
// each step's number, plain state, plain label, formal id as secondary
// text (dc-1), and one line of count by state and reason from the step
// order; the current step is marked aria-current and each station links
// to its area's first row.
func writeReadinessStepper(b *strings.Builder, snap readinesspilot.Snapshot, em *readinessEmission) {
	esc := stdhtml.EscapeString
	b.WriteString(`<nav class="readiness-rail" aria-label="Readiness rail"><ol class="readiness-rail-list">`)
	for i, area := range snap.Areas {
		focused := snap.CurrentFocus == area.ID
		reason, kind := readinessStepReason(snap, area)
		b.WriteString(`<li class="readiness-station`)
		if focused {
			b.WriteString(` readiness-station--focus`)
		}
		b.WriteString(`" data-area-id="` + esc(string(area.ID)) + `" data-state="` + string(area.State) + `" data-reason="` + kind + `">`)
		b.WriteString(`<a class="readiness-station-link" href="#area-` + esc(string(area.ID)) + `"`)
		if focused {
			b.WriteString(` aria-current="step"`)
		}
		b.WriteString(`><span class="readiness-station-head"><span class="readiness-station-step">step</span> <span class="readiness-station-num">` + strconv.Itoa(i+1) + `</span>`)
		writeReadinessState(b, em.surface.words, area.State)
		b.WriteString(`</span>`)
		b.WriteString(`<span class="readiness-station-label">` + esc(area.Label) + `</span>`)
		b.WriteString(`<span class="readiness-station-id">` + esc(string(area.ID)) + `</span>`)
		b.WriteString(`<span class="readiness-station-line">` + esc(readinessStepLine(em.surface.words, readinessStepCounts(snap, area.ID), reason)) + `</span>`)
		b.WriteString(`</a></li>`)
	}
	b.WriteString(`</ol></nav>`)
}

// writeReadinessFocus writes Focus next: every concern of the current
// step, ranked in the snapshot's attention order and marked "now"; then
// the later steps' concerns behind one inline disclosure, ranked on and
// marked "later" with what they wait on. An empty list states its honest
// reason.
func writeReadinessFocus(b *strings.Builder, snap readinesspilot.Snapshot, em *readinessEmission) {
	esc := stdhtml.EscapeString
	b.WriteString(`<section class="readiness-queue" id="readiness-focus" aria-label="Focus next">`)
	b.WriteString(`<h2 class="readiness-heading">Focus next<span class="readiness-count"> · ` + strconv.Itoa(len(snap.Attention)) + `</span></h2>`)
	if len(snap.Attention) == 0 {
		b.WriteString(`<p class="readiness-queue-empty">Nothing needs attention: every check in this snapshot is proven.</p>`)
		b.WriteString(`</section>`)
		return
	}
	b.WriteString(`<p class="readiness-queue-note">Ranked for this request: the current step first, then what waits on it. Unknowns stay unknown.</p>`)

	var now, later []readinesspilot.Concern
	for _, concern := range snap.Attention {
		if em.later(snap, concern) {
			later = append(later, concern)
		} else {
			now = append(now, concern)
		}
	}
	b.WriteString(`<ol class="readiness-queue-list">`)
	for i, concern := range now {
		b.WriteString(`<li` + em.anchor(concern.Area) + `>`)
		writeReadinessConcern(b, concern, em, "readiness-card", i+1, "now")
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ol>`)
	if len(later) > 0 {
		b.WriteString(`<details class="readiness-more" data-testid="readiness-later"><summary class="readiness-more-summary">`)
		// vocab:identity — "-closed" is this disclosure's CSS class fragment (collapsed label), not the lifecycle state.
		b.WriteString(`<span class="readiness-more-closed">` + strconv.Itoa(len(later)) + ` more, waiting on ` + esc(em.focusLabel) + `</span><span class="readiness-more-open">Show fewer</span>`)
		b.WriteString(`</summary><ol class="readiness-queue-list readiness-queue-rest" start="` + strconv.Itoa(len(now)+1) + `">`)
		for i, concern := range later {
			b.WriteString(`<li` + em.anchor(concern.Area) + `>`)
			writeReadinessConcern(b, concern, em, "readiness-card", len(now)+i+1, "later")
			b.WriteString(`</li>`)
		}
		b.WriteString(`</ol></details>`)
	}
	b.WriteString(`</section>`)
}

// writeReadinessKnown writes Known problems in later steps: the later
// steps' violated concerns only, each a link to that concern's one row in
// the later-steps disclosure — never a second row of its own.
func writeReadinessKnown(b *strings.Builder, snap readinesspilot.Snapshot, em *readinessEmission) {
	esc := stdhtml.EscapeString
	known := readinessKnownProblems(snap)
	b.WriteString(`<section class="readiness-known" id="readiness-known" aria-label="Known problems in later steps">`)
	b.WriteString(`<h2 class="readiness-heading readiness-heading--violated">Known problems in later steps<span class="readiness-count"> · ` + strconv.Itoa(len(known)) + `</span></h2>`)
	if len(known) == 0 {
		b.WriteString(`<p class="readiness-known-empty">None: no later step reports a violated check.</p></section>`)
		return
	}
	b.WriteString(`<ul class="readiness-known-list">`)
	for _, concern := range known {
		b.WriteString(`<li><a class="readiness-known-link" href="#concern-` + esc(readinessFragment(concern.ID)) + `" data-known-concern="` + esc(concern.ID) + `">`)
		b.WriteString(`<span class="readiness-stage">` + esc(em.labels[concern.Area]) + `</span>`)
		b.WriteString(`<span class="readiness-known-text">` + esc(concern.Summary) + `</span>`)
		writeReadinessState(b, em.surface.words, concern.State)
		b.WriteString(`</a></li>`)
	}
	b.WriteString(`</ul></section>`)
}

// writeReadinessCompleted writes the completed checks: every proven
// concern, in the snapshot's existing order, with full technical details.
func writeReadinessCompleted(b *strings.Builder, snap readinesspilot.Snapshot, em *readinessEmission) {
	proven := 0
	for _, concern := range snap.AllConcerns {
		if concern.State == readinesspilot.StateProven {
			proven++
		}
	}
	b.WriteString(`<section class="readiness-completed" id="readiness-completed" aria-label="Completed checks">`)
	b.WriteString(`<h2 class="readiness-heading readiness-heading--proven">Completed checks<span class="readiness-count"> · ` + strconv.Itoa(proven) + `</span></h2>`)
	if proven == 0 {
		b.WriteString(`<p class="readiness-completed-empty">No check is proven yet for this request.</p></section>`)
		return
	}
	b.WriteString(`<p class="readiness-completed-note">Checks already proven for this request.</p>`)
	b.WriteString(`<ol class="readiness-completed-list">`)
	for _, concern := range snap.AllConcerns {
		if concern.State != readinesspilot.StateProven {
			continue
		}
		b.WriteString(`<li` + em.anchor(concern.Area) + `>`)
		writeReadinessConcern(b, concern, em, "readiness-row", 0, "")
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ol></section>`)
}

// writeReadinessConcern writes one concern exactly once: its step label
// with its "now" or "later" mark (when is "now", "later", or "" for a
// completed check), the plain human-review label with the formal
// obligation as secondary text when HumanReview() holds, the primary line
// — the source-derived guidance, or the fact for a proven row, which
// carries none — the plain state chip, the complete technical details,
// and, for an unresolved concern, its one usable action. rank > 0 renders
// the focus ranking. The row's id attribute is its concern id, the
// known-problems links' target. Inside the wall the row carries its wall
// target as data attributes (SI-368 (16)): data-target-kind is object,
// stub, strip, slot or none, and data-target names the object id, the
// stub slug, the strip half or the slot's object kind.
func writeReadinessConcern(b *strings.Builder, concern readinesspilot.Concern, em *readinessEmission, class string, rank int, when string) {
	esc := stdhtml.EscapeString
	id := esc(concern.ID)
	b.WriteString(`<article class="` + class + ` readiness-concern--` + string(concern.State))
	if concern.HumanReview() {
		b.WriteString(` readiness-concern--human-review`)
	}
	b.WriteString(`" id="concern-` + id + `" data-concern-id="` + id + `" data-area-id="` + esc(string(concern.Area)) + `"`)
	if when != "" {
		b.WriteString(` data-timing="` + when + `"`)
	}
	if em.surface.wall {
		if target, ok := em.surface.targets[concern.ID]; ok {
			b.WriteString(` data-target-kind="` + target.Kind + `" data-target="` + esc(target.Value) + `"`)
		} else {
			b.WriteString(` data-target-kind="` + readinessTargetNone + `"`)
		}
	}
	b.WriteString(`>`)
	if rank > 0 {
		b.WriteString(`<span class="readiness-rank">` + strconv.Itoa(rank) + `</span>`)
	}
	b.WriteString(`<div class="readiness-copy">`)
	b.WriteString(`<p class="readiness-stage">` + esc(em.labels[concern.Area]))
	switch when {
	case "now":
		b.WriteString(` <span class="readiness-when readiness-when--now">now</span>`)
	case "later":
		b.WriteString(` <span class="readiness-when readiness-when--later">later — waits on ` + esc(em.focusLabel) + `</span>`)
	}
	b.WriteString(`</p>`)
	if concern.HumanReview() {
		b.WriteString(`<p class="readiness-human-review" data-testid="readiness-human-review">Human review<span class="readiness-human-review-formal"> · <code>` + id + `</code>`)
		if concern.WorkClass != "" {
			b.WriteString(` · <code>` + esc(string(concern.WorkClass)) + `</code>`)
		}
		b.WriteString(`</span></p>`)
	}
	if concern.Guidance != "" {
		b.WriteString(`<p class="readiness-summary readiness-guidance">` + esc(concern.Guidance) + `</p>`)
	} else {
		b.WriteString(`<p class="readiness-summary">` + esc(concern.Summary) + `</p>`)
	}
	writeReadinessState(b, em.surface.words, concern.State)
	writeReadinessTech(b, concern)
	writeReadinessDestination(b, concern.Destination, em.surface.wall)
	b.WriteString(`</div></article>`)
}

// writeReadinessTech writes the concern's Technical details disclosure:
// the fact (the summary), then every formal fact, verbatim, none
// normalized away — the timing is the snapshot's own current or eventual.
func writeReadinessTech(b *strings.Builder, concern readinesspilot.Concern) {
	b.WriteString(`<details class="readiness-tech"><summary>Technical details</summary><dl class="readiness-tech-facts">`)
	b.WriteString(`<dt>Fact</dt><dd class="readiness-fact">` + stdhtml.EscapeString(concern.Summary) + `</dd>`)
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
// carries neither and writes nothing. Inside the wall the board link is
// not reused (SI-368 (16)): the row's own wall target is its action, and
// the destination stays in Technical details.
func writeReadinessDestination(b *strings.Builder, dest readinesspilot.Destination, wall bool) {
	if dest.BoardPath != "" {
		if wall {
			return
		}
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
