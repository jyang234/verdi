package workbench

// Server-rendered HTML for the spec importer's browser adapter
// (specimport.go): the import page and the read-only source-record view.
// Everything a source or the service supplies — labels, field text,
// finding messages, refs, digests — is escaped as text; no source byte is
// ever emitted as markup or executable Markdown. Every visible class word
// resolves through classWords (vocabulary.go); the element ids, testids
// and enum values stay bare. The page's small stylesheet lives here with
// the component rather than in the shared dex stylesheet.

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

// specImportCSS is the importer's own narrow stylesheet: layout for the
// numbered steps, source rows, findings, fields and coverage. Colors and
// type ride the shared stylesheet's variables.
const specImportCSS = `<style>
.import-step { border: 1px solid var(--rule); border-radius: 6px; padding: 0.8rem 1rem; margin: 0 0 1rem; }
.import-step legend { font-weight: 600; padding: 0 0.3rem; }
.import-step .field { display: flex; flex-direction: column; gap: 0.3rem; margin: 0.6rem 0; }
.import-step input[type="text"], .import-step input:not([type]), .import-step select, .import-step textarea, .import-step input[type="number"] { font: inherit; padding: 0.3rem 0.5rem; max-width: 40rem; }
.import-step textarea { min-height: 4rem; }
.import-check { display: block; margin: 0.5rem 0; }
.import-source-list, .import-mapping-list, .import-link-list { margin: 0.6rem 0; padding-left: 1.4rem; }
.import-source-list li, .import-mapping-list li, .import-link-list li { margin: 0.4rem 0; padding: 0.4rem 0.6rem; border: 1px dashed var(--rule); border-radius: 4px; }
.import-source-label { font-weight: 600; word-break: break-all; }
.import-inline { display: inline-flex; flex-wrap: wrap; gap: 0.6rem; align-items: center; margin-top: 0.3rem; }
.import-inline label { display: inline-flex; gap: 0.3rem; align-items: center; }
.import-inline input[type="number"] { width: 6rem; }
.import-actions { display: flex; flex-wrap: wrap; gap: 1rem; align-items: center; margin: 1rem 0; }
.import-next-action { margin: 0; font-weight: 600; }
.import-error { border-left: 3px solid var(--fail-ink, #a4262c); padding: 0.6rem 0.8rem; white-space: pre-wrap; word-break: break-word; }
.import-result[data-stale="true"] { opacity: 0.75; }
.import-findings li { margin: 0.4rem 0; }
.import-findings li[data-blocking="true"] > strong { color: var(--fail-ink, #a4262c); }
.import-finding-next { display: block; color: var(--muted-solid); font-size: 0.9rem; }
.import-field { border: 1px solid var(--rule); border-radius: 6px; padding: 0.6rem 0.8rem; margin: 0.6rem 0; }
.import-field h4 { margin: 0 0 0.3rem; }
.import-origin { font-size: 0.85rem; color: var(--muted-solid); margin-left: 0.5rem; }
.import-field-text { white-space: pre-wrap; word-break: break-word; margin: 0.4rem 0; padding: 0.4rem 0.6rem; background: var(--code-bg, rgba(127,127,127,0.08)); }
.import-field-spans { font-size: 0.85rem; color: var(--muted-solid); margin: 0.2rem 0; }
.import-evidence { border: 0; padding: 0; margin: 0.4rem 0; }
.import-evidence legend { font-size: 0.85rem; }
.import-evidence label { margin-right: 0.8rem; }
.import-coverage { border-collapse: collapse; margin: 0.6rem 0; }
.import-coverage th, .import-coverage td { border: 1px solid var(--rule); padding: 0.3rem 0.6rem; text-align: left; }
.import-confirm { margin-top: 1rem; padding-top: 0.8rem; border-top: 1px solid var(--rule); }
.import-record dl { display: grid; grid-template-columns: max-content 1fr; gap: 0.3rem 1rem; }
.import-record dd { margin: 0; word-break: break-all; }
.import-record table { border-collapse: collapse; }
.import-record th, .import-record td { border: 1px solid var(--rule); padding: 0.3rem 0.6rem; text-align: left; vertical-align: top; }
</style>`

// renderSpecImportPage renders GET /design/import: the labeled file,
// format, target, disposition, mapping and link controls; the preview
// region the script fills; the confirmation bound to the current digest;
// and the created result. The markup is complete without JavaScript (the
// noscript note names the CLI); the script owns file reading, request
// composition, invalidation and the two fetches.
func renderSpecImportPage(mdl *model.Model) ([]byte, error) {
	esc := stdhtml.EscapeString
	words := classWords{m: mdl}
	var b strings.Builder
	b.WriteString(specImportCSS)

	b.WriteString(`<section class="import-intro">`)
	b.WriteString(`<p class="ritual-note">Import an existing spec from files you choose. The browser reads each file's exact bytes and sends them as they are; the importer maps them mechanically by their labeled structure, invents no wording, calls no assistant or provider, and creates nothing until you confirm the exact preview below.</p>`)
	b.WriteString(`<noscript><p class="notice">This page needs JavaScript to read files and call the importer. Without it, use the CLI: <code>verdi design import source</code>, <code>verdi design import preview</code> and <code>verdi design import apply</code>.</p></noscript>`)
	b.WriteString(`</section>`)

	b.WriteString(`<form id="import-form" data-testid="import-form" novalidate autocomplete="off">`)

	b.WriteString(`<fieldset class="import-step"><legend>1. Source files</legend>`)
	b.WriteString(`<label for="import-files">Source files &mdash; explicit files only; folders, archives, URLs and commands are never read</label>`)
	b.WriteString(`<input type="file" id="import-files" data-testid="import-files" multiple>`)
	b.WriteString(`<p class="field-hint">Each file's label is its name, never a path. The first file is the primary source; every other file is retained-only support unless you map it explicitly. Optional line ranges are inclusive and 1-based.</p>`)
	b.WriteString(`<ol id="import-source-list" data-testid="import-source-list" class="import-source-list" aria-label="Selected source files"></ol>`)
	b.WriteString(`</fieldset>`)

	b.WriteString(`<fieldset class="import-step"><legend>2. Format</legend>`)
	b.WriteString(`<div class="field"><label for="import-format">Format profile</label><select id="import-format" data-testid="import-format">`)
	b.WriteString(`<option value="markdown-v1">markdown-v1 &mdash; labeled Markdown: a title heading with Problem, Outcome, Acceptance Criteria, Constraints, Decisions and Open Questions sections</option>`)
	b.WriteString(`<option value="native">native &mdash; an existing verdi spec.md, kept byte-identical (no mappings, links or deferral)</option>`)
	b.WriteString(`<option value="f13-reference-v1">f13-reference-v1 &mdash; the pinned F13 reference profile; refuses any other primary bytes</option>`)
	b.WriteString(`<option value="manual-v1">manual-v1 &mdash; no automatic fields; every value comes from your explicit mappings</option>`)
	b.WriteString(`</select></div></fieldset>`)

	b.WriteString(`<fieldset class="import-step"><legend>3. Target</legend>`)
	b.WriteString(`<div class="field"><label for="import-slug">Spec name (slug)</label><input id="import-slug" data-testid="import-slug" spellcheck="false" pattern="` + esc(specNameRe.String()) + `"><span class="field-hint">kebab-case; becomes spec/&lt;slug&gt; on the new design branch design/&lt;slug&gt;. An existing name is refused, never renamed.</span></div>`)
	b.WriteString(`<div class="field"><label for="import-class">Class</label><select id="import-class" data-testid="import-class">`)
	b.WriteString(`<option value="feature">` + esc(words.word("feature")) + `</option>`)
	b.WriteString(`<option value="story">` + esc(words.word("story")) + `</option>`)
	b.WriteString(`</select></div>`)
	b.WriteString(`<div class="field"><label for="import-title">Title</label><input id="import-title" data-testid="import-title"></div>`)
	b.WriteString(`<div class="field"><label for="import-story">Tracker reference (optional)</label><input id="import-story" data-testid="import-story" placeholder="scheme:ID" spellcheck="false"><span class="field-hint">` + esc("Required when the class is "+words.word("story")+", with the tracker the store already configures; the importer never synthesizes one.") + `</span></div>`)
	b.WriteString(`</fieldset>`)

	b.WriteString(`<fieldset class="import-step"><legend>4. Dispositions</legend>`)
	b.WriteString(`<label class="import-check"><input type="checkbox" id="import-retain" data-testid="import-retain"> Retain every unmapped byte as retained-only source. This is your explicit acknowledgement; without it, unmapped bytes block creation. It never resolves a missing or ambiguous field.</label>`)
	b.WriteString(`<label class="import-check"><input type="checkbox" id="import-defer" data-testid="import-defer"> Defer both statements: insert the shared TODO placeholders for Problem and Outcome. Placeholders are visibly incomplete and never source quotations; a statement the source did provide is displaced and disclosed.</label>`)
	b.WriteString(`</fieldset>`)

	b.WriteString(`<fieldset class="import-step" id="import-mappings-fieldset"><legend>5. Explicit mappings</legend>`)
	b.WriteString(`<p class="field-hint">Override or add a field by hand: name the target (problem, outcome, or an ac-/co-/dc-/oq- id), then either select a byte range of a source with a transform or supply your own text, and declare evidence kinds for acceptance criteria. The preview's own Edit and evidence controls fill this list for you.</p>`)
	b.WriteString(`<ol id="import-mapping-list" data-testid="import-mapping-list" class="import-mapping-list" aria-label="Explicit mappings"></ol>`)
	b.WriteString(`<button type="button" id="import-add-mapping" data-testid="import-add-mapping">Add mapping</button>`)
	b.WriteString(`</fieldset>`)

	b.WriteString(`<fieldset class="import-step"><legend>6. Declared links</legend>`)
	b.WriteString(`<p class="field-hint">Explicit relationships only, in the link vocabulary the store already validates; nothing is inferred from URLs in the source.</p>`)
	b.WriteString(`<ol id="import-link-list" data-testid="import-link-list" class="import-link-list" aria-label="Declared links"></ol>`)
	b.WriteString(`<button type="button" id="import-add-link" data-testid="import-add-link">Add link</button>`)
	b.WriteString(`</fieldset>`)

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
	b.WriteString(`<p id="import-stale-note" data-testid="import-stale-note" class="notice" hidden>Inputs changed since this preview; preview again before creating anything.</p>`)
	b.WriteString(`<p id="import-ready" data-testid="import-ready" data-ready="false"></p>`)
	b.WriteString(`<p class="import-digest-line">Preview digest <code id="import-digest" data-testid="import-digest"></code></p>`)
	b.WriteString(`<h3>Findings</h3><ul id="import-findings" data-testid="import-findings" class="import-findings"></ul>`)
	b.WriteString(`<h3>Fields</h3><div id="import-fields" data-testid="import-fields" class="import-fields"></div>`)
	b.WriteString(`<h3>Source coverage</h3>`)
	b.WriteString(`<table id="import-coverage" data-testid="import-coverage" class="import-coverage"><caption class="field-hint">Byte accounting only: every selected byte is mapped, retained-only or unresolved exactly once. It is not semantic completeness.</caption><thead><tr><th scope="col">Source</th><th scope="col">Total bytes</th><th scope="col">Mapped</th><th scope="col">Retained-only</th><th scope="col">Unresolved</th></tr></thead><tbody></tbody></table>`)
	b.WriteString(`<h3>Sources</h3><ul id="import-sources" data-testid="import-sources" class="import-sources"></ul>`)
	b.WriteString(`<div class="import-confirm">`)
	b.WriteString(`<label class="import-check"><input type="checkbox" id="import-confirm" data-testid="import-confirm" disabled> I reviewed exactly this preview and want to create the proposal from it</label>`)
	b.WriteString(`<button type="button" id="import-apply-btn" data-testid="import-apply-btn" class="btn-primary" disabled>Create proposal</button>`)
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)

	b.WriteString(`<section id="import-created" data-testid="import-created" class="import-created" data-status="" hidden>`)
	b.WriteString(`<h2>Created</h2>`)
	b.WriteString(`<p id="import-created-summary" class="import-created-summary"></p>`)
	b.WriteString(`<p><a id="import-board-link" data-testid="import-board-link" href="#">Open the board</a> &middot; <a id="import-record-link" data-testid="import-record-link" href="#">Source record</a></p>`)
	b.WriteString(`<h3>Disclosures</h3><ul id="import-disclosures" data-testid="import-disclosures"></ul>`)
	b.WriteString(`</section>`)

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
	fact := func(label, value string) {
		b.WriteString(`<dt>` + esc(label) + `</dt><dd><code>` + esc(value) + `</code></dd>`)
	}
	fact("Import commit", view.ImportCommit)
	fact("Base commit", rec.BaseCommit)
	fact("Preview digest", rec.PreviewDigest)
	fact("Request digest", rec.RequestDigest)
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
