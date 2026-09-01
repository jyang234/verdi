package workbench

// Server-side rendering for the ASD workbench's Wave 6 additions (design
// §§3.1, 4.2, 6.2): the revision/posture header, the promoted four-area
// shell (the readiness pilot's exact presentation idioms — SI-125 plain
// labels, Step N of 4, exactly-three preview with the exact remaining
// count expanded inline, current-area-first ordering with explicit
// timing/dependency, plain state chips with the formal state retained in
// technical details), the typed-operation forms, and the on-demand
// provenance/semantic-review/context panels. Rendered INSIDE the board
// region so the page, the fragment, the snapshot, and every mutation
// response carry one identical projection.

import (
	"fmt"
	stdhtml "html"
	"strconv"
	"strings"
)

// asdShellPreview mirrors the pilot's fixed three-item preview (SI-125).
const asdShellPreview = 3

// asdPlainState maps the formal three-valued state to its plain primary
// label — the pilot's exact participant-ratified words.
func asdPlainState(state string) string {
	switch state {
	case asdStateProven:
		return "Ready"
	case asdStateViolated:
		return "Needs attention"
	default:
		return "Not enough evidence yet"
	}
}

// writeASDState writes one plain state chip; the modifier class keeps the
// formal state machine-readable (e2e asserts chip classes, never label
// text).
func writeASDState(b *strings.Builder, state string) {
	b.WriteString(`<span class="readiness-state readiness-state--` + state + `">` + asdPlainState(state) + `</span>`)
}

// writeASDPosture renders the revision/posture header (design §4.2):
// checkout, branch, worktree HEAD, accepted HEAD, clean/dirty,
// ahead/behind when resolvable, and whether the displayed bytes are
// proposed or accepted — with the mode stamp and terminal status badge
// riding here so every posture fact refreshes with the projection.
func writeASDPosture(b *strings.Builder, p *BoardProjection, git *boardGitState, asd *asdView) {
	esc := stdhtml.EscapeString
	b.WriteString(`<section class="asd-posture" id="asd-posture" data-testid="asd-posture" aria-label="Repository posture">`)
	b.WriteString(`<span class="board-mode-tag board-mode-tag--` + esc(string(p.Mode)) + `">` + esc(modeStampLabels[p.Mode]) + `</span>`)
	if badge := terminalStatusBadge(p.Status); badge != "" {
		label := badge
		if p.StatusLabel != "" {
			label = p.StatusLabel
		}
		b.WriteString(`<span class="badge badge-` + esc(badge) + ` board-status-badge" data-testid="board-status-badge">` + esc(label) + `</span>`)
	}
	// Proposed vs accepted: what the displayed bytes ARE (design §4.2).
	proposed := p.Mode != modeReadOnly || asd.StateFormal == "proposed"
	byteWord := "proposed"
	if asd.StateFormal == "accepted-pending-build" && !proposed {
		byteWord = "accepted"
	}
	if asd.StateFormal == "accepted-pending-build" {
		byteWord = "accepted"
	}
	b.WriteString(`<span class="asd-posture-bytes" data-testid="asd-posture-bytes" data-state="` + esc(asd.StateFormal) + `">displayed bytes: ` + esc(byteWord) + ` <span class="asd-posture-formal">(` + esc(asd.StateFormal) + `)</span></span>`)
	dirtyWord, dirtyState := "clean", "clean"
	if asd.Dirty {
		dirtyWord, dirtyState = "uncommitted changes", "dirty"
	}
	b.WriteString(`<span class="asd-posture-tree" data-testid="asd-posture-tree" data-dirty="` + dirtyState + `">working tree: ` + dirtyWord + `</span>`)
	b.WriteString(`<button type="button" class="asd-refresh" id="asd-refresh" data-testid="asd-refresh">Refresh</button>`)
	b.WriteString(`<details class="readiness-tech asd-posture-tech"><summary>Repository details</summary><dl class="readiness-tech-facts">`)
	writeReadinessFact(b, "Checkout", asd.Checkout)
	writeReadinessFact(b, "Branch", asd.Branch)
	writeReadinessFact(b, "Worktree HEAD", orUnproven(asd.WorktreeHead))
	writeReadinessFact(b, "Accepted branch", orUnproven(asd.DefaultBranch))
	writeReadinessFact(b, "Accepted HEAD", orUnproven(asd.AcceptedHead))
	if asd.AheadBehindKnown {
		writeReadinessFact(b, "Ahead/behind", fmt.Sprintf("%d ahead, %d behind %s", asd.Ahead, asd.Behind, asd.DefaultBranch))
		if asd.Ahead > 0 && asd.Behind > 0 {
			writeReadinessFact(b, "Divergence", "diverged: both sides carry commits the other lacks")
		}
	} else {
		writeReadinessFact(b, "Ahead/behind", "unproven: the accepted branch could not be resolved")
	}
	writeReadinessFact(b, "Base digest", asd.BaseDigest)
	b.WriteString(`</dl></details>`)
	b.WriteString(`</section>`)
}

func orUnproven(v string) string {
	if v == "" {
		return "unproven"
	}
	return v
}

// writeASDShell renders the promoted four-area shell (design §3.1,
// SI-125): orientation (Step N of 4), the process rail, the ranked focus
// queue (exactly three, exact remainder inline), the sequencing
// explainer (F-04), and completed checks — all from the derived shell's
// facts, nothing suppressed or reclassified.
func writeASDShell(b *strings.Builder, asd *asdView) {
	esc := stdhtml.EscapeString
	shell := asd.Shell
	b.WriteString(`<section class="asd-shell readiness-page" id="asd-shell" data-testid="asd-shell" aria-label="Design readiness">`)

	// Orientation: Where am I? What next? (F-01's two anchor questions.)
	b.WriteString(`<div class="readiness-orient asd-orient"><p class="readiness-eyebrow">Where this design stands</p>`)
	b.WriteString(`<p class="readiness-step" data-testid="asd-step">`)
	if shell.CurrentFocus == "" {
		b.WriteString(`All four steps are complete.`)
	} else {
		for i, area := range shell.Areas {
			if area.ID != shell.CurrentFocus {
				continue
			}
			b.WriteString(`Step ` + strconv.Itoa(i+1) + ` of 4 — ` + esc(area.Label))
			break
		}
	}
	b.WriteString(`</p></div>`)

	// The four-step rail (data-state carries the formal state; the chip
	// carries the plain label).
	b.WriteString(`<nav class="readiness-rail" aria-label="Design process rail"><ol class="readiness-rail-list">`)
	for i, area := range shell.Areas {
		focused := shell.CurrentFocus == area.ID
		b.WriteString(`<li class="readiness-station`)
		if focused {
			b.WriteString(` readiness-station--focus`)
		}
		b.WriteString(`" data-area-id="` + esc(string(area.ID)) + `" data-state="` + esc(area.State) + `">`)
		b.WriteString(`<a class="readiness-station-link" href="#asd-focus"`)
		if focused {
			b.WriteString(` aria-current="step"`)
		}
		b.WriteString(`><span class="readiness-station-num">` + strconv.Itoa(i+1) + `</span>`)
		b.WriteString(`<span class="readiness-station-label">` + esc(area.Label) + `</span>`)
		writeASDState(b, area.State)
		b.WriteString(`</a></li>`)
	}
	b.WriteString(`</ol></nav>`)

	// F-04: the fixed order is explained, never left to inference.
	b.WriteString(`<p class="asd-sequence-note" data-testid="asd-sequence-note">The four steps run in this order: define the work, define success, check constraints, then get approval. Later steps stay visible while an earlier step is unresolved, but the current step's items are what move this design forward now.</p>`)

	// Focus queue.
	b.WriteString(`<section class="readiness-queue asd-focus" id="asd-focus" aria-label="Focus next">`)
	b.WriteString(`<h2 class="readiness-heading">Focus next</h2>`)
	if len(shell.Attention) == 0 {
		b.WriteString(`<p class="readiness-queue-empty" data-testid="asd-queue-empty">Nothing needs attention: every check on this wall is proven.</p>`)
	} else {
		b.WriteString(`<p class="readiness-downstream" data-testid="asd-downstream">Known problems in later steps: ` + strconv.Itoa(shell.DownstreamViolated) + `</p>`)
		visible := shell.Attention
		var rest []asdConcern
		if len(visible) > asdShellPreview {
			rest = visible[asdShellPreview:]
			visible = visible[:asdShellPreview]
		}
		b.WriteString(`<ol class="readiness-queue-list">`)
		for i, c := range visible {
			b.WriteString(`<li>`)
			writeASDConcern(b, c, asd, i+1)
			b.WriteString(`</li>`)
		}
		b.WriteString(`</ol>`)
		if len(rest) > 0 {
			b.WriteString(`<details class="readiness-more" data-testid="asd-more"><summary class="readiness-more-summary">`)
			// vocab:identity — "-closed" is this disclosure's CSS class fragment (collapsed label), not the lifecycle state.
			b.WriteString(`<span class="readiness-more-closed">` + esc(asdCountLabel(len(rest), "more item", "more items")) + `</span>`)
			b.WriteString(`<span class="readiness-more-open">Show fewer</span></summary>`)
			b.WriteString(`<ol class="readiness-queue-list readiness-queue-rest" start="` + strconv.Itoa(asdShellPreview+1) + `">`)
			for i, c := range rest {
				b.WriteString(`<li>`)
				writeASDConcern(b, c, asd, asdShellPreview+i+1)
				b.WriteString(`</li>`)
			}
			b.WriteString(`</ol></details>`)
		}
	}
	b.WriteString(`</section>`)

	// Completed checks: every proven fact, lossless.
	b.WriteString(`<section class="readiness-completed asd-completed" aria-label="Completed checks"><h2 class="readiness-heading">Completed checks</h2>`)
	for _, c := range shell.All {
		if c.State != asdStateProven {
			continue
		}
		writeASDConcern(b, c, asd, 0)
	}
	b.WriteString(`</section>`)
	b.WriteString(`</section>`)
}

// writeASDConcern renders one shell row: plain summary, plain state chip,
// explicit timing/dependency (F-02), source-derived guidance (F-03), the
// plain human-review label (F-05), and the complete technical details.
func writeASDConcern(b *strings.Builder, c asdConcern, asd *asdView, rank int) {
	esc := stdhtml.EscapeString
	class := "readiness-row"
	if rank > 0 {
		class = "readiness-card"
	}
	b.WriteString(`<article class="` + class + ` readiness-concern--` + esc(c.State) + `" data-concern-id="` + esc(c.ID) + `" data-area-id="` + esc(string(c.Area)) + `">`)
	if rank > 0 {
		b.WriteString(`<span class="readiness-rank">` + strconv.Itoa(rank) + `</span>`)
	}
	b.WriteString(`<div class="readiness-copy">`)
	b.WriteString(`<p class="readiness-stage">` + esc(asdAreaLabels[c.Area]))
	// Explicit timing and dependency (F-02): current-step rows say "now";
	// later-step rows name what they wait on.
	if asd.Shell.CurrentFocus != "" && c.State != asdStateProven {
		if c.Area == asd.Shell.CurrentFocus {
			b.WriteString(` <span class="asd-timing asd-timing--now" data-timing="now">· now</span>`)
		} else if asdAreaAfter(c.Area, asd.Shell.CurrentFocus) {
			b.WriteString(` <span class="asd-timing asd-timing--later" data-timing="later">· later — waits on ` + esc(asdAreaLabels[asd.Shell.CurrentFocus]) + `</span>`)
		}
	}
	b.WriteString(`</p>`)
	if c.HumanReview {
		b.WriteString(`<p class="asd-human-review" data-testid="asd-human-review">Human review</p>`)
	}
	b.WriteString(`<p class="readiness-summary">` + esc(c.Summary) + `</p>`)
	writeASDState(b, c.State)
	if c.Guidance != "" {
		b.WriteString(`<p class="asd-guidance" data-testid="asd-guidance-` + esc(c.ID) + `">` + esc(c.Guidance) + `</p>`)
	}
	b.WriteString(`<details class="readiness-tech"><summary>Technical details</summary><dl class="readiness-tech-facts">`)
	writeReadinessFact(b, "State", c.State)
	writeReadinessFact(b, "Concern", c.ID)
	writeReadinessFact(b, "Area", string(c.Area))
	writeReadinessFact(b, "Blocking", strconv.FormatBool(c.Blocking))
	if len(c.Witnesses) > 0 {
		b.WriteString(`<dt>Witnesses</dt><dd><ul class="readiness-witnesses">`)
		for _, wtn := range c.Witnesses {
			b.WriteString(`<li><code>` + esc(wtn) + `</code></li>`)
		}
		b.WriteString(`</ul></dd>`)
	}
	b.WriteString(`</dl></details>`)
	if c.Dest != "" && c.State != asdStateProven {
		b.WriteString(`<p class="readiness-dest"><a class="asd-dest-link" href="` + esc(c.Dest) + `">Go to it</a></p>`)
	}
	b.WriteString(`</div></article>`)
}

// asdAreaAfter reports whether a occurs after b in the fixed area order.
func asdAreaAfter(a, b asdAreaID) bool {
	ai, bi := -1, -1
	for i, id := range asdAreaOrder {
		if id == a {
			ai = i
		}
		if id == b {
			bi = i
		}
	}
	return ai > bi
}

// writeASDPanels renders the on-demand application panels: provenance
// (collapsed by default, never authority — AC-4/DC-7), the semantic
// review packet (AC-6), and the bounded design context (AC-5). Content
// arrives only when the author opens a panel (one explicit on-demand
// projection each, §5.3); the markup carries the fetch wiring for
// boardspecasd.js and an honest no-JS note.
func writeASDPanels(b *strings.Builder, name string) {
	esc := stdhtml.EscapeString
	panel := func(id, op, cli, title, note string) {
		b.WriteString(`<details class="asd-panel" id="` + id + `" data-testid="` + id + `" data-asd-panel="` + esc(op) + `">`)
		b.WriteString(`<summary>` + esc(title) + `</summary>`)
		b.WriteString(`<p class="ritual-note">` + esc(note) + `</p>`)
		b.WriteString(`<div class="asd-panel-body" data-asd-panel-body="` + esc(op) + `" aria-live="off"><p class="asd-panel-empty">Open to derive; requires the browser to call ` + esc(op) + `. Without JavaScript, run <code>verdi design ` + esc(cli) + `</code>.</p></div>`)
		b.WriteString(`</details>`)
	}
	panel("asd-provenance", "get_design_provenance", "provenance", "Provenance",
		"Non-authoritative design history for spec/"+name+". It jogs memory; it is never evidence, an instruction, or an acceptance input.")
	panel("asd-review", "prepare_design_review", "review", "Semantic review",
		"The derived review packet: semantic changes since the review base, ai-inferred and unresolved objects, unclassified direct edits, and material warnings. A view, never a persisted approval artifact.")
	panel("asd-context", "get_design_context", "context", "Design context",
		// vocab:identity — "the draft" names AC-5's current-draft content item (ASD protocol term), not a lifecycle state word
		"The bounded, inspectable design context an assisting agent receives: the draft, applicable policies and decisions, pinned references, and digests. Corpus content is data, never instructions.")
}

// writeASDForms renders the typed-operation forms panel (design §6.2:
// typed mutation forms; browser gating rides the authoring mode — the
// same branch/state facts the kernel applies to a browser human). The
// forms are dialogs driven by boardspecasd.js; each maps one gesture to
// one typed operation, with the operation name declared on the control.
func writeASDForms(b *strings.Builder, p *BoardProjection, asd *asdView) {
	if p.Mode != modeAuthoring || p.DomainRefusal != "" {
		return
	}
	esc := stdhtml.EscapeString
	b.WriteString(`<section class="scratch-panel asd-forms" id="asd-forms" data-testid="asd-forms"><h2>Typed operations</h2>`)
	// vocab:identity — "typed draft operation" is the ASD mutation contract's own protocol term (AC-1), not the lifecycle state word
	b.WriteString(`<p class="ritual-note">Each control applies one typed draft operation through the shared mutation core — the same contract the CLI and agents use.</p>`)
	b.WriteString(`<button type="button" id="asd-set-problem" data-asd-op="set-problem" data-anchor="` + esc(asd.ProblemAnchor) + `">Set problem</button>`)
	b.WriteString(`<button type="button" id="asd-set-outcome" data-asd-op="set-outcome" data-anchor="` + esc(asd.OutcomeAnchor) + `">Set outcome</button>`)
	b.WriteString(`<button type="button" id="asd-add-object" data-asd-op="add-object">Add object&#8230;</button>`)
	b.WriteString(`</section>`)
}

// writeASDEditStubDialog renders the in-place stub correction dialog
// (F-06's fix: capability-driven correction of an existing stub through
// the same typed transaction — a slug rename is one atomic
// [remove-stub, add-stub] batch; a binding change is one edit-stub).
func writeASDEditStubDialog(b *strings.Builder, p *BoardProjection) {
	esc := stdhtml.EscapeString
	b.WriteString(`<div role="dialog" aria-label="Correct stub" class="board-dialog asd-stub-dialog" id="asd-stub-dialog" hidden>`)
	b.WriteString(`<h2>Correct stub</h2>`)
	b.WriteString(`<p class="ritual-note">Corrects the declared stub in place through one typed transaction. Renaming the slug replaces the stub atomically (remove-stub + add-stub in one batch).</p>`)
	b.WriteString(`<div class="field"><label for="asd-stub-slug">Slug</label><input id="asd-stub-slug" data-testid="asd-stub-slug" autocomplete="off" spellcheck="false">`)
	b.WriteString(`<span class="field-hint">kebab-case (the spec name grammar)</span></div>`)
	b.WriteString(`<p class="asd-field-error" id="asd-stub-slug-error" data-testid="asd-stub-slug-error" role="alert" hidden></p>`)
	spikeWord := p.words.word("spike")
	b.WriteString(`<label class="asd-stub-spike"><input type="checkbox" id="asd-stub-spike"> ` + esc(spikeWord) + `</label>`)
	b.WriteString(`<fieldset class="asd-stub-acs" id="asd-stub-acs"><legend>Covers acceptance criteria</legend>`)
	for _, c := range p.Cards {
		if c.Kind != "acceptance-criterion" {
			continue
		}
		b.WriteString(`<label><input type="checkbox" data-asd-stub-ac="` + esc(c.ID) + `"> ` + esc(c.ID) + `</label>`)
	}
	b.WriteString(`</fieldset>`)
	b.WriteString(`<fieldset class="asd-stub-oqs" id="asd-stub-oqs"><legend>Resolves open questions</legend>`)
	for _, c := range p.Cards {
		if c.Kind != "open-question" {
			continue
		}
		b.WriteString(`<label><input type="checkbox" data-asd-stub-oq="` + esc(c.ID) + `"> ` + esc(c.ID) + `</label>`)
	}
	b.WriteString(`</fieldset>`)
	b.WriteString(`<div class="dialog-actions"><button type="button" id="asd-stub-ok" class="btn-primary" data-testid="asd-stub-ok">Apply</button>`)
	b.WriteString(`<button type="button" id="asd-stub-cancel">Cancel</button></div>`)
	b.WriteString(`</div>`)
}

// writeASDTextDialog renders the shared set-problem/set-outcome/add-object
// dialog shell; boardspecasd.js retargets it per gesture.
func writeASDTextDialog(b *strings.Builder) {
	b.WriteString(`<div role="dialog" aria-label="Typed operation" class="board-dialog asd-op-dialog" id="asd-op-dialog" hidden>`)
	b.WriteString(`<h2 id="asd-op-title">Typed operation</h2>`)
	b.WriteString(`<p class="ritual-note" id="asd-op-note"></p>`)
	b.WriteString(`<div class="field" id="asd-op-kind-field" hidden><label for="asd-op-kind">Kind</label><select id="asd-op-kind">`)
	b.WriteString(`<option value="add-ac">acceptance criterion</option><option value="add-constraint">constraint</option><option value="add-decision">decision</option><option value="add-question">open question</option>`)
	b.WriteString(`</select><span class="field-hint" id="asd-op-id-preview" data-testid="asd-op-id-preview"></span></div>`)
	b.WriteString(`<div class="field"><label for="asd-op-text">Text</label><textarea id="asd-op-text" data-testid="asd-op-text"></textarea></div>`)
	b.WriteString(`<p class="asd-field-error" id="asd-op-error" data-testid="asd-op-error" role="alert" hidden></p>`)
	b.WriteString(`<div class="dialog-actions"><button type="button" id="asd-op-ok" class="btn-primary" data-testid="asd-op-ok">Apply</button>`)
	b.WriteString(`<button type="button" id="asd-op-cancel">Cancel</button></div>`)
	b.WriteString(`</div>`)
}

// writeASDImpactDialog renders the graduation impact preview dialog
// (F-08's fix, adjudication 6): the exact resulting refs, paths,
// relationships, downstream facts, and unknowns — all server-derived
// data composed by boardspecasd.js from data attributes, shown BEFORE the
// durable mutation, never a UI-computed guess.
func writeASDImpactDialog(b *strings.Builder) {
	b.WriteString(`<div role="alertdialog" aria-label="Graduation impact" class="board-dialog asd-impact-dialog" id="asd-impact-dialog" hidden aria-describedby="asd-impact-body">`)
	b.WriteString(`<h2 id="asd-impact-title">Graduation impact</h2>`)
	b.WriteString(`<div id="asd-impact-body" data-testid="asd-impact-body"></div>`)
	b.WriteString(`<p class="asd-field-error" id="asd-impact-error" data-testid="asd-impact-error" role="alert" hidden></p>`)
	b.WriteString(`<div class="dialog-actions"><button type="button" id="asd-impact-ok" class="btn-primary" data-testid="asd-impact-ok">Graduate</button>`)
	b.WriteString(`<button type="button" id="asd-impact-cancel">Cancel</button></div>`)
	b.WriteString(`</div>`)
}
