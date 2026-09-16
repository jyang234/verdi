package workbench

// Server-rendered HTML for the spec importer's browser adapter
// (specimport.go): the import page and the read-only source-record view.
// Everything a source or the service supplies — labels, field text,
// finding messages, refs, digests — is escaped as text; no source byte is
// ever emitted as markup or executable Markdown. Every visible class word
// resolves through classWords (vocabulary.go); the element ids, testids
// and enum values stay bare. The page's small stylesheet lives here with
// the component rather than in the shared dex stylesheet.
//
// The page's shape follows the owner-approved usability correction (main's
// usability-ui-adjudication over the FABLE D1-D6 proposal): source files,
// reading profile, target, two plain-language explicit choices, and the
// advanced mapping/link controls in one collapsible region; then the
// preview in reading order — readiness, blockers, statements, the evidence
// helper BEFORE the criteria it governs, the object cards, the rest of the
// source, the byte accounting collapsed, and the confirmation bound to the
// digest. Nothing is served pre-checked. The page's own stylesheet gives
// its labels the ordinary body face in sentence case, overriding the shared
// stylesheet's uppercase mono label face within this page only.

import (
	"fmt"
	stdhtml "html"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specimport"
)

// specImportCSS is the importer's own narrow stylesheet: page-local
// typography (body face, sentence case, restrained measure, visible focus),
// layout for the numbered steps, the collapsible advanced region, source
// rows, blockers, the evidence helper, cards, the remaining-source summary
// and the collapsed byte accounting. Colors and type ride the shared
// stylesheet's variables; nothing here reaches outside .import-page or the
// record view's own classes.
const specImportCSS = `<style>
.import-page { max-width: 64rem; }
.import-page p, .import-page li, .import-page label, .import-page legend, .import-page summary { max-width: 72ch; }
.import-page label, .import-page legend, .import-page summary, .import-page .field-hint, .import-page .import-hint { font-family: var(--serif); font-size: 1rem; letter-spacing: normal; text-transform: none; color: var(--ink); line-height: 1.5; }
.import-page .field-hint, .import-page .import-hint { font-size: 0.93rem; color: var(--muted-solid); margin: 0.15rem 0 0; }
.import-page input:focus-visible, .import-page select:focus-visible, .import-page textarea:focus-visible, .import-page button:focus-visible, .import-page summary:focus-visible, .import-page a:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
.import-step { border: 1px solid var(--rule); border-radius: 6px; padding: 0.9rem 1.1rem 1rem; margin: 0 0 1.1rem; }
.import-step legend { font-weight: 600; padding: 0 0.35rem; }
.import-step .field { display: flex; flex-direction: column; gap: 0.3rem; margin: 0.7rem 0; }
.import-step .field > label { font-weight: 600; }
.import-step input[type="text"], .import-step input:not([type]), .import-step select, .import-step textarea, .import-step input[type="number"] { font: inherit; font-family: var(--mono); font-size: 0.92rem; padding: 0.35rem 0.55rem; max-width: 40rem; }
.import-step textarea { min-height: 4rem; }
.import-check { display: grid; grid-template-columns: auto 1fr; column-gap: 0.6rem; row-gap: 0.15rem; align-items: start; margin: 0.8rem 0; }
.import-check > input { margin: 0.35rem 0 0; }
.import-check > .import-hint { grid-column: 2; }
.import-advanced { border: 1px dashed var(--rule); border-radius: 6px; padding: 0.4rem 1.1rem; margin: 0 0 1.1rem; }
.import-advanced > summary { cursor: pointer; font-weight: 600; padding: 0.45rem 0; }
.import-advanced[open] > summary { border-bottom: 1px solid var(--rule); margin-bottom: 0.6rem; }
.import-advanced .import-step { border: 0; padding: 0.2rem 0 0.6rem; margin: 0; }
.import-source-list, .import-mapping-list, .import-link-list { margin: 0.6rem 0; padding-left: 1.4rem; }
.import-source-list li, .import-mapping-list li, .import-link-list li { margin: 0.4rem 0; padding: 0.4rem 0.6rem; border: 1px dashed var(--rule); border-radius: 4px; max-width: none; }
.import-source-label { font-weight: 600; word-break: break-all; }
.import-inline { display: inline-flex; flex-wrap: wrap; gap: 0.6rem; align-items: center; margin-top: 0.3rem; }
.import-inline label { display: inline-flex; gap: 0.3rem; align-items: center; font-size: 0.95rem; }
.import-inline input[type="number"] { width: 6rem; }
.import-actions { display: flex; flex-wrap: wrap; gap: 1rem; align-items: center; margin: 1rem 0; }
.import-next-action { margin: 0; font-weight: 600; }
.import-error { border-left: 3px solid var(--fail-ink, #a4262c); padding: 0.6rem 0.8rem; white-space: pre-wrap; word-break: break-word; }
.import-result[data-stale="true"] .import-findings, .import-result[data-stale="true"] .import-statements, .import-result[data-stale="true"] .import-fields { opacity: 0.8; }
.import-ready { font-weight: 600; margin: 0.6rem 0; }
.import-ready[data-stale="true"] { font-weight: 400; color: var(--muted-solid); }
.import-findings { padding-left: 1.2rem; }
.import-findings li { margin: 0.5rem 0; }
.import-findings li[data-blocking="true"] > .import-finding-title { color: var(--fail-ink, #a4262c); }
.import-finding-code { font-size: 0.8rem; margin-left: 0.3rem; }
.import-finding-message { display: block; }
.import-finding-next { display: block; color: var(--muted-solid); font-size: 0.92rem; }
.import-finding-jump { margin-right: 0.8rem; font-size: 0.92rem; }
.import-finding-earlier { color: var(--muted-solid); font-size: 0.85rem; margin-left: 0.4rem; }
.import-guide { border: 1px solid var(--rule); border-left: 3px solid var(--accent); border-radius: 6px; padding: 0.7rem 1rem; margin: 0.8rem 0; background: var(--card); }
.import-guide h4 { margin: 0 0 0.4rem; }
.import-guide ul { margin: 0.4rem 0; padding-left: 1.2rem; }
.import-guide p { margin: 0.4rem 0; }
.import-group-heading { margin: 1.1rem 0 0.4rem; font-size: 1.05rem; }
.import-field { border: 1px solid var(--rule); border-radius: 6px; padding: 0.7rem 0.9rem; margin: 0.6rem 0; }
.import-field h4, .import-field h5 { margin: 0 0 0.3rem; font-size: 1rem; font-family: var(--serif); }
.import-field-missing { border-style: dashed; }
.import-field-source { margin: 0 0 0.3rem; color: var(--muted-solid); font-size: 0.92rem; }
.import-origin { font-size: 0.8rem; color: var(--muted-solid); margin-left: 0.4rem; }
.import-field-text { white-space: pre-wrap; word-break: break-word; margin: 0.4rem 0; padding: 0.4rem 0.6rem; background: var(--code-bg, rgba(127,127,127,0.08)); font-family: var(--serif); font-size: 0.98rem; }
.import-field-status { margin: 0.3rem 0; font-size: 0.95rem; }
.import-field-status[data-status="changed"] { color: var(--st-pending, #8a6414); }
.import-field-status[data-status="earlier"] { color: var(--muted-solid); }
.import-field-findings { margin: 0.3rem 0; padding-left: 1.2rem; font-size: 0.92rem; }
.import-field-findings li[data-blocking="true"] { color: var(--fail-ink, #a4262c); }
.import-evidence { border: 0; padding: 0; margin: 0.4rem 0; }
.import-evidence legend { font-size: 0.95rem; font-weight: 600; padding: 0; }
.import-evidence label { display: inline-flex; gap: 0.3rem; align-items: center; margin: 0.2rem 0.9rem 0.2rem 0; font-size: 0.95rem; }
.import-evidence button { margin-top: 0.3rem; }
.import-field-actions { display: flex; flex-wrap: wrap; gap: 0.6rem; align-items: center; margin: 0.4rem 0; }
.import-write-text { display: block; width: 100%; max-width: 40rem; min-height: 4rem; font: inherit; font-size: 0.95rem; margin: 0.4rem 0; }
.import-tech { margin: 0.3rem 0 0; font-size: 0.9rem; }
.import-tech > summary { cursor: pointer; color: var(--muted-solid); font-size: 0.9rem; }
.import-field-spans { font-size: 0.85rem; color: var(--muted-solid); margin: 0.2rem 0; font-family: var(--mono); }
.import-remaining p { margin: 0.3rem 0; }
.import-coverage { border-collapse: collapse; margin: 0.6rem 0; }
.import-coverage th, .import-coverage td { border: 1px solid var(--rule); padding: 0.3rem 0.6rem; text-align: left; }
.import-sources { padding-left: 1.2rem; font-size: 0.9rem; word-break: break-all; }
.import-confirm { margin-top: 1rem; padding-top: 0.8rem; border-top: 1px solid var(--rule); }
.import-digest-line { font-size: 0.85rem; color: var(--muted-solid); word-break: break-all; }
.import-record dl { display: grid; grid-template-columns: max-content 1fr; gap: 0.3rem 1rem; }
.import-record dd { margin: 0; word-break: break-all; }
.import-record table { border-collapse: collapse; }
.import-record th, .import-record td { border: 1px solid var(--rule); padding: 0.3rem 0.6rem; text-align: left; vertical-align: top; }
</style>`

// renderSpecImportPage renders GET /design/import: the labeled file,
// reading-profile, target and choice controls; the collapsed advanced
// mapping and link controls; the preview region the script fills in
// reading order (readiness, blockers, statements, the evidence helper,
// object cards, the rest of the source, collapsed byte accounting); the
// confirmation bound to the current digest; and the created result. The
// markup is complete without JavaScript (the noscript note names the
// CLI); the script owns file reading, request composition, invalidation
// and the two fetches. Nothing is served checked.
func renderSpecImportPage(mdl *model.Model) ([]byte, error) {
	esc := stdhtml.EscapeString
	words := classWords{m: mdl}
	featureWord := words.word("feature")
	storyWord := words.word("story")
	var b strings.Builder
	b.WriteString(specImportCSS)
	b.WriteString(`<div class="import-page">`)

	b.WriteString(`<section class="import-intro">`)
	b.WriteString(`<p class="ritual-note">Import an existing spec from files you choose. The browser reads each file's exact bytes and sends them as they are; the importer maps them mechanically by their labeled structure, invents no wording, calls no assistant or provider, and creates nothing until you confirm the exact preview below.</p>`)
	b.WriteString(`<noscript><p class="notice">This page needs JavaScript to read files and call the importer. Without it, use the CLI: <code>verdi design import source</code>, <code>verdi design import preview</code> and <code>verdi design import apply</code>.</p></noscript>`)
	b.WriteString(`</section>`)

	b.WriteString(`<form id="import-form" data-testid="import-form" novalidate autocomplete="off">`)

	b.WriteString(`<fieldset class="import-step"><legend>1. Source files</legend>`)
	b.WriteString(`<label for="import-files">Source files</label>`)
	b.WriteString(`<input type="file" id="import-files" data-testid="import-files" multiple>`)
	b.WriteString(`<p class="field-hint">Explicit files only; folders, archives, URLs and commands are never read. Each file's label is its name, never a path. The first file is the primary source. Every other file can be kept whole with the import as reference material when you choose to keep the remaining source text (step 4), or mapped in Advanced. Optional line ranges are inclusive and start at 1.</p>`)
	b.WriteString(`<ol id="import-source-list" data-testid="import-source-list" class="import-source-list" aria-label="Selected source files"></ol>`)
	b.WriteString(`</fieldset>`)

	b.WriteString(`<fieldset class="import-step"><legend>2. How the primary file is read</legend>`)
	b.WriteString(`<div class="field"><label for="import-format">Reading profile</label><select id="import-format" data-testid="import-format">`)
	b.WriteString(`<option value="markdown-v1">Labeled Markdown (markdown-v1): a title heading with Problem, Outcome, Acceptance Criteria, Constraints, Decisions and Open Questions sections</option>`)
	b.WriteString(`<option value="native">Existing verdi spec.md (native): kept byte-identical; no mappings, links or TODO placeholders</option>`)
	b.WriteString(`<option value="f13-reference-v1">Pinned F13 reference profile (f13-reference-v1): only the exact reference primary bytes; refuses any other primary</option>`)
	b.WriteString(`<option value="manual-v1">Manual mappings only (manual-v1): no automatic fields; every value comes from Advanced</option>`)
	b.WriteString(`</select><span class="field-hint">A profile reads only the structure it names: labeled headings and flat lists become fields. A label it cannot find or resolve is reported as missing, never guessed from the wording.</span></div></fieldset>`)

	b.WriteString(`<fieldset class="import-step"><legend>3. Target</legend>`)
	b.WriteString(`<div class="field"><label for="import-slug">Spec name (slug)</label><input id="import-slug" data-testid="import-slug" spellcheck="false" pattern="` + esc(specNameRe.String()) + `"><span class="field-hint">kebab-case; becomes spec/&lt;slug&gt; on the new design branch design/&lt;slug&gt;. An existing name is refused, never renamed.</span></div>`)
	b.WriteString(`<div class="field"><label for="import-class">Class</label><select id="import-class" data-testid="import-class">`)
	b.WriteString(`<option value="feature">` + esc(words.word("feature")) + `</option>`)
	b.WriteString(`<option value="story">` + esc(words.word("story")) + `</option>`)
	b.WriteString(`</select></div>`)
	b.WriteString(`<div class="field"><label for="import-title">Title</label><input id="import-title" data-testid="import-title"></div>`)
	b.WriteString(`<div class="field"><label for="import-story">Tracker reference (optional)</label><input id="import-story" data-testid="import-story" placeholder="scheme:ID" spellcheck="false"><span class="field-hint">` + esc("Required when the class is "+words.word("story")+", with the tracker the store already configures; the importer never synthesizes one.") + `</span></div>`)
	b.WriteString(`<p class="import-hint" id="import-story-note" data-testid="import-story-note" hidden>` + esc("A "+storyWord+" also needs one declared implements or resolves link to its parent, and its tracker reference above; the importer never synthesizes either.") + ` <a href="#import-advanced" class="import-open-advanced" data-testid="import-story-links-jump" data-focus="import-add-link">Open Advanced to declare the link</a></p>`)
	b.WriteString(`</fieldset>`)

	b.WriteString(`<fieldset class="import-step"><legend>4. Your choices</legend>`)
	b.WriteString(`<p class="field-hint">Both are explicit; nothing here is chosen for you. Evidence kinds for acceptance criteria are chosen per criterion in the preview below, which explains them first.</p>`)
	b.WriteString(`<label class="import-check"><input type="checkbox" id="import-retain" data-testid="import-retain"><span class="import-check-text">Keep the remaining source text as reference material</span><span class="import-hint">Source text that did not become a field will be kept with the import as reference material. It is not promoted into spec fields, and keeping it never fills a missing or ambiguous field. Until you choose this, leftover text blocks creation.</span></label>`)
	b.WriteString(`<label class="import-check"><input type="checkbox" id="import-defer" data-testid="import-defer"><span class="import-check-text">Leave Problem and Outcome as TODOs for now</span><span class="import-hint">Replaces both statements with the shared TODO placeholder, including any statement copied from your source or written here; the preview discloses what was displaced. The placeholders are visibly incomplete: the proposal can be created, but it is not ready for review until real statements are written on the board.</span></label>`)
	b.WriteString(`</fieldset>`)

	b.WriteString(`<details id="import-advanced" data-testid="import-advanced" class="import-advanced"><summary>Advanced: manual field mappings (<span id="import-mapping-count">0</span>) and declared links (<span id="import-link-count">0</span>)</summary>`)
	b.WriteString(`<p class="field-hint">Byte ranges, transforms and link refs live here. Ordinary choices made on the preview cards (evidence kinds, edited text, written statements) are recorded here as mappings, so the count includes them.</p>`)
	b.WriteString(`<fieldset class="import-step" id="import-mappings-fieldset"><legend>Manual field mappings</legend>`)
	b.WriteString(`<p class="field-hint">Override or add a field by hand: name the target (problem, outcome, or an ac-/co-/dc-/oq- id), then either select a byte range of a source with a transform or supply your own text, and choose evidence kinds for acceptance criteria. Offsets count UTF-8 bytes of the selected slice, start inclusive, end exclusive.</p>`)
	b.WriteString(`<ol id="import-mapping-list" data-testid="import-mapping-list" class="import-mapping-list" aria-label="Explicit mappings"></ol>`)
	b.WriteString(`<button type="button" id="import-add-mapping" data-testid="import-add-mapping">Add mapping</button>`)
	b.WriteString(`</fieldset>`)
	b.WriteString(`<fieldset class="import-step"><legend>Declared links</legend>`)
	b.WriteString(`<p class="field-hint">Explicit relationships only, in the link vocabulary the store already validates; nothing is inferred from URLs in the source.</p>`)
	b.WriteString(`<ol id="import-link-list" data-testid="import-link-list" class="import-link-list" aria-label="Declared links"></ol>`)
	b.WriteString(`<button type="button" id="import-add-link" data-testid="import-add-link">Add link</button>`)
	b.WriteString(`</fieldset>`)
	b.WriteString(`</details>`)

	b.WriteString(`<div class="import-actions">`)
	b.WriteString(`<button type="button" id="import-preview-btn" data-testid="import-preview-btn" class="btn-primary">Preview</button>`)
	b.WriteString(`<p id="import-next-action" data-testid="import-next-action" class="import-next-action" role="status" aria-live="polite">Add at least one source file, then preview.</p>`)
	b.WriteString(`</div>`)
	b.WriteString(`</form>`)

	b.WriteString(`<section id="import-error" data-testid="import-error" class="import-error notice" role="alert" hidden></section>`)
	// The lost-response recovery (a creation request that was sent but
	// whose answer never arrived): the same request bytes and digest can be
	// resent visibly while the inputs are unchanged; the server reconciles
	// an already-made publication to already-created, never a duplicate.
	b.WriteString(`<section id="import-retry" data-testid="import-retry" class="import-retry notice" role="alert" hidden><p id="import-retry-note"></p><button type="button" id="import-retry-btn" data-testid="import-retry-btn">Retry the same request</button></section>`)

	b.WriteString(`<section id="import-result" data-testid="import-result" class="import-result" data-stale="false" data-ready="false" hidden>`)
	b.WriteString(`<h2>Preview</h2>`)
	b.WriteString(`<p id="import-stale-note" data-testid="import-stale-note" class="notice" hidden>Inputs changed since this preview. Everything below is the earlier result; preview again before creating anything.</p>`)
	b.WriteString(`<p id="import-ready" data-testid="import-ready" class="import-ready" data-ready="false" data-stale="false"></p>`)
	b.WriteString(`<h3 id="import-findings-heading" data-testid="import-findings-heading">What still blocks creation</h3><ul id="import-findings" data-testid="import-findings" class="import-findings"></ul>`)
	b.WriteString(`<h3>Statements</h3><div id="import-statements" data-testid="import-statements" class="import-statements"></div>`)
	b.WriteString(`<section id="import-evidence-guide" data-testid="import-evidence-guide" class="import-guide" hidden>`)
	b.WriteString(`<h4>Before you choose evidence kinds</h4>`)
	b.WriteString(`<p>Each acceptance criterion declares which kinds of proof it expects. Choosing a kind states what must be produced before the criterion counts as met; it does not produce or attach that proof, and the importer never infers a kind from the criterion's wording. The import records these declarations only; it is not evidence, acceptance or review.</p>`)
	b.WriteString(`<ul>`)
	b.WriteString(`<li><strong>Code fact (static)</strong>: a code-fact the store can prove.</li>`)
	b.WriteString(`<li><strong>Suite test (behavioral)</strong>: a test the suite runs.</li>`)
	b.WriteString(`<li><strong>Live probe (runtime)</strong>: a probe against a live surface.</li>`)
	b.WriteString(`<li><strong>Human sign-off (attestation)</strong>: a human sign-off on file.</li>`)
	b.WriteString(`</ul>`)
	b.WriteString(`<p data-import-class="feature" data-testid="import-evidence-floor-feature">` + esc("Each "+featureWord+" criterion must include human sign-off (attestation). Selecting it means that sign-off will be required later; it does not record anyone's sign-off. The preview reports a criterion that lacks it.") + `</p>`)
	b.WriteString(`<p data-import-class="story" data-testid="import-evidence-floor-story" hidden>` + esc("Each "+storyWord+" criterion must include at least one kind.") + `</p>`)
	b.WriteString(`</section>`)
	b.WriteString(`<h3>Acceptance criteria and other objects</h3><div id="import-fields" data-testid="import-fields" class="import-fields"></div>`)
	b.WriteString(`<h3>The rest of the source</h3><div id="import-remaining" data-testid="import-remaining" class="import-remaining"></div>`)
	b.WriteString(`<details id="import-technical" data-testid="import-technical" class="import-tech"><summary>Byte accounting and source digests</summary>`)
	b.WriteString(`<table id="import-coverage" data-testid="import-coverage" class="import-coverage"><caption class="field-hint">Byte accounting only: every selected byte is mapped, retained-only or unresolved exactly once. It is not semantic completeness.</caption><thead><tr><th scope="col">Source</th><th scope="col">Total bytes</th><th scope="col">Mapped</th><th scope="col">Retained-only</th><th scope="col">Unresolved</th></tr></thead><tbody></tbody></table>`)
	b.WriteString(`<ul id="import-sources" data-testid="import-sources" class="import-sources"></ul>`)
	b.WriteString(`</details>`)
	b.WriteString(`<div class="import-confirm">`)
	b.WriteString(`<p class="import-digest-line">Preview digest <code id="import-digest" data-testid="import-digest"></code></p>`)
	b.WriteString(`<label class="import-check"><input type="checkbox" id="import-confirm" data-testid="import-confirm" disabled><span class="import-check-text">I reviewed exactly this preview and want to create the proposal from it</span></label>`)
	b.WriteString(`<button type="button" id="import-apply-btn" data-testid="import-apply-btn" class="btn-primary" disabled>Create proposal</button>`)
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)

	b.WriteString(`<section id="import-created" data-testid="import-created" class="import-created" data-status="" hidden>`)
	b.WriteString(`<h2>Created</h2>`)
	b.WriteString(`<p id="import-created-summary" class="import-created-summary"></p>`)
	b.WriteString(`<p><a id="import-board-link" data-testid="import-board-link" href="#">Open the board</a> &middot; <a id="import-record-link" data-testid="import-record-link" href="#">Source record</a></p>`)
	b.WriteString(`<h3>Disclosures</h3><ul id="import-disclosures" data-testid="import-disclosures"></ul>`)
	b.WriteString(`</section>`)
	b.WriteString(`</div>`)

	return renderPage(pageData{
		Title:     "Import existing spec",
		Nav:       template.HTML(`<span class="current">import</span>`),
		BodyHTML:  template.HTML(b.String()),
		ExtraHTML: template.HTML(`<script src="` + routeSpecImportJS + `" defer></script>`),
	})
}

// renderSpecImportRecord renders the read-only source-record view over a
// verified RecordView: the explanation of what the record is (and is not),
// the current-spec disclosure, the verified import facts, and the recorded
// sources, fields, mappings and coverage.
func renderSpecImportRecord(view specimport.RecordView, branch, slug string) ([]byte, error) {
	esc := stdhtml.EscapeString
	rec := view.Record
	var b strings.Builder
	b.WriteString(specImportCSS)
	b.WriteString(`<section class="import-record" data-testid="import-record" data-current-spec-matches="` + strconv.FormatBool(view.CurrentSpecMatches) + `" data-import-commit="` + esc(view.ImportCommit) + `">`)
	b.WriteString(`<p class="ritual-note">This is the read-only source record for <code>` + esc(rec.SpecRef) + `</code> on branch <code>` + esc(branch) + `</code>: it describes the ORIGINAL content copied into the spec when it was imported, verified against the branch's committed bytes. It is not an ASD provenance entry &mdash; the semantic review packet may still classify the creation as an unclassified direct edit &mdash; and it is not evidence of acceptance or of review. Source origins are copy claims, not third-party authorship proofs.</p>`)

	b.WriteString(`<h2>Current spec</h2>`)
	if view.CurrentSpecMatches {
		b.WriteString(`<p data-testid="record-current-spec-matches">The active spec on this branch still matches the imported revision byte for byte.</p>`)
	}
	if len(view.Disclosures) > 0 {
		b.WriteString(`<ul class="import-findings">`)
		for _, d := range view.Disclosures {
			testid := "record-disclosure-" + d.Code
			if d.Code == specimport.FindingCurrentSpecChanged {
				testid = "record-current-spec-changed"
			}
			b.WriteString(`<li data-testid="` + esc(testid) + `" data-code="` + esc(d.Code) + `" data-blocking="` + strconv.FormatBool(d.Blocking) + `"><strong>` + esc(d.Code) + `</strong> ` + esc(d.Message) + `</li>`)
		}
		b.WriteString(`</ul>`)
	}

	b.WriteString(`<h2>Verified import</h2><dl>`)
	factWithID := func(label, value, testid string) {
		attr := ""
		if testid != "" {
			attr = ` data-testid="` + esc(testid) + `"`
		}
		b.WriteString(`<dt>` + esc(label) + `</dt><dd` + attr + `><code>` + esc(value) + `</code></dd>`)
	}
	fact := func(label, value string) { factWithID(label, value, "") }
	fact("Import commit", view.ImportCommit)
	fact("Base commit", rec.BaseCommit)
	fact("Preview digest", rec.PreviewDigest)
	fact("Request digest", rec.RequestDigest)
	format, profileDigest := recordFormatFacts(rec)
	factWithID("Format", format, "record-format")
	factWithID("Profile primary digest", profileDigest, "record-profile-primary-digest")
	fact("Candidate digest", rec.CandidateDigest)
	fact("Engine digest", rec.EngineDigest)
	fact("Model digest", rec.ModelDigest)
	fact("Config digest", rec.ConfigDigest)
	actor := "principal " + string(rec.Actor.Attribution.PrincipalID)
	if rec.Actor.Attribution.Unauthenticated {
		actor = "unauthenticated human (browser)"
	}
	if rec.Actor.Harness != "" {
		actor += " via harness " + rec.Actor.Harness
	}
	if rec.Actor.Session != "" {
		actor += " session " + rec.Actor.Session
	}
	fact("Actor attribution", actor)
	policy := string(rec.Policy.State)
	if rec.Policy.Digest != "" {
		policy += " " + rec.Policy.Digest
	}
	fact("Policy posture", policy)
	b.WriteString(`</dl>`)

	b.WriteString(`<h2>Retained sources</h2><table><thead><tr><th scope="col">Id</th><th scope="col">Label</th><th scope="col">Lines</th><th scope="col">Original digest</th><th scope="col">Selected digest</th></tr></thead><tbody>`)
	labels := map[string]string{}
	for _, src := range rec.Sources {
		labels[src.ID] = src.Label
		lines := "whole file"
		if src.StartLine != 0 || src.EndLine != 0 {
			lines = strconv.Itoa(src.StartLine) + "–" + strconv.Itoa(src.EndLine)
		}
		b.WriteString(`<tr data-testid="record-source-` + esc(src.ID) + `"><td><code>` + esc(src.ID) + `</code></td><td>` + esc(src.Label) + `</td><td>` + esc(lines) + `</td><td><code>` + esc(src.OriginalDigest) + `</code></td><td><code>` + esc(src.Digest) + `</code></td></tr>`)
	}
	b.WriteString(`</tbody></table>`)

	b.WriteString(`<h2>Recorded fields</h2>`)
	for _, f := range rec.Fields {
		b.WriteString(`<article class="import-field" data-testid="record-field-` + esc(f.Target) + `" data-target="` + esc(f.Target) + `" data-origin="` + esc(f.Origin) + `">`)
		b.WriteString(`<h4>` + esc(f.Target) + `<span class="import-origin">` + esc(f.Origin) + `</span></h4>`)
		b.WriteString(`<pre class="import-field-text">` + esc(f.Text) + `</pre>`)
		for _, sp := range f.Spans {
			b.WriteString(`<p class="import-field-spans">from ` + esc(labels[sp.SourceID]) + ` (` + esc(sp.SourceID) + `) bytes [` + strconv.Itoa(sp.Start) + `,` + strconv.Itoa(sp.End) + `) transform ` + esc(sp.Transform) + `</p>`)
		}
		if len(f.Evidence) > 0 {
			b.WriteString(`<p class="import-field-spans">evidence (user-selected): ` + esc(strings.Join(f.Evidence, ", ")) + `</p>`)
		}
		b.WriteString(`</article>`)
	}

	if len(rec.Mappings) > 0 {
		b.WriteString(`<h2>Explicit mappings</h2><ul>`)
		for _, m := range rec.Mappings {
			desc := m.Target
			if m.SourceID != "" {
				desc += fmt.Sprintf(" from %s bytes [%d,%d) transform %s", m.SourceID, m.Start, m.End, m.Transform)
			}
			if m.Text != nil {
				desc += " with user text"
			}
			if len(m.Evidence) > 0 {
				desc += " evidence " + strings.Join(m.Evidence, ", ")
			}
			b.WriteString(`<li>` + esc(desc) + `</li>`)
		}
		b.WriteString(`</ul>`)
	}

	b.WriteString(`<h2>Coverage</h2><p class="field-hint">Byte accounting only, never semantic completeness.</p>`)
	b.WriteString(`<table><thead><tr><th scope="col">Source</th><th scope="col">Total bytes</th><th scope="col">Mapped</th><th scope="col">Retained-only</th><th scope="col">Unresolved</th></tr></thead><tbody>`)
	for _, c := range rec.Coverage {
		b.WriteString(`<tr data-testid="record-coverage-` + esc(c.SourceID) + `"><td>` + esc(labels[c.SourceID]) + ` (<code>` + esc(c.SourceID) + `</code>)</td><td>` + strconv.Itoa(c.TotalBytes) + `</td><td>` + strconv.Itoa(c.MappedBytes) + `</td><td>` + strconv.Itoa(c.RetainedBytes) + `</td><td>` + strconv.Itoa(c.UnresolvedBytes) + `</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)

	b.WriteString(`<p><a data-testid="record-board-link" href="` + esc(BranchBoardHref(branch, slug)) + `">Open the board</a></p>`)
	b.WriteString(`</section>`)

	return renderPage(pageData{
		Title:    "Source record: " + rec.SpecRef,
		Nav:      template.HTML(`<a href="` + routeSpecImportPage + `">import</a>`),
		BodyHTML: template.HTML(b.String()),
	})
}

// recordFormatNotRecorded is the record page's explicit value for a record
// committed before spec/uat-round-1 ac-4 added the format and profile
// digest fields ("existing records without them decode with the fields
// absent and the page says so") — a disclosure, never a blank cell.
const recordFormatNotRecorded = "not recorded (pre-ac-4 record)"

// recordFormatFacts derives the two ac-4 fact values from a verified
// record: the request format it was produced from and the pinned primary
// digest of the reference profile that format names. A record with no
// format is pre-ac-4 for both facts; a format that names no reference
// profile (specimport's validate() guarantees such a record carries no
// digest) states the digest as not applicable rather than missing.
func recordFormatFacts(rec specimport.Record) (format, profileDigest string) {
	switch {
	case rec.Format == "":
		return recordFormatNotRecorded, recordFormatNotRecorded
	case rec.ProfilePrimaryDigest != "":
		return rec.Format, rec.ProfilePrimaryDigest
	default:
		return rec.Format, "not applicable"
	}
}

// renderSpecImportRecordInvalid is the record view's 400: a malformed
// query, named, with a way back — distinct from unavailable proof.
func renderSpecImportRecordInvalid(w http.ResponseWriter, reason string) {
	var body strings.Builder
	body.WriteString(`<div class="error-page" role="alert" data-testid="import-record-invalid">`)
	body.WriteString(`<p class="error-message"><strong>Malformed record request.</strong></p>`)
	body.WriteString(`<p>The record view needs <code>?branch=&lt;branch&gt;&amp;spec=&lt;slug&gt;</code>: the design branch the import was published on and the bare spec name.</p>`)
	body.WriteString(`<pre class="error-detail">` + stdhtml.EscapeString(reason) + `</pre>`)
	writeBackToDirectory(&body)
	body.WriteString(`</div>`)
	writeSpecImportPage(w, http.StatusBadRequest, "Source record", body.String())
}

// renderSpecImportRecordUnavailable renders the provenance-mismatch (or
// operational) refusal of a record read as its own page: the committed
// proof is unavailable, malformed or does not verify — deliberately not
// the changed-current-spec disclosure, which only a VERIFIED record can
// carry.
func renderSpecImportRecordUnavailable(w http.ResponseWriter, status int, code, detail, branch, slug string) {
	esc := stdhtml.EscapeString
	var body strings.Builder
	body.WriteString(`<div class="error-page" role="alert" data-testid="import-record-unavailable" data-code="` + esc(code) + `">`)
	body.WriteString(`<p class="error-message"><strong>No verifiable import record.</strong></p>`)
	body.WriteString(`<p>The committed import proof for <code>spec/` + esc(slug) + `</code> on branch <code>` + esc(branch) + `</code> is unavailable, malformed or does not verify against the branch's own bytes (<code>` + esc(code) + `</code>). This is not an ordinary later edit of the spec &mdash; a verified record discloses that separately as a changed current spec &mdash; and nothing here proves or disproves the original import.</p>`)
	if detail != "" {
		body.WriteString(`<pre class="error-detail">` + esc(detail) + `</pre>`)
	} else if status == http.StatusInternalServerError {
		body.WriteString(`<p class="error-hint">An operational failure interrupted the read; the server log names the cause.</p>`)
	}
	writeBackToDirectory(&body)
	body.WriteString(`</div>`)
	writeSpecImportPage(w, status, "Source record", body.String())
}

// writeSpecImportPage writes bodyHTML through the shared shell at status,
// falling back to a bare text error if the shell itself fails — the same
// loud-stays-loud posture errorpage.go and notfound.go take.
func writeSpecImportPage(w http.ResponseWriter, status int, title, bodyHTML string) {
	out, err := renderPage(pageData{
		Title:    title,
		Nav:      template.HTML(`<a href="` + routeSpecImportPage + `">import</a>`),
		BodyHTML: template.HTML(bodyHTML),
	})
	if err != nil {
		http.Error(w, title, status)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(out) // response body write; post-header error is unactionable
}
