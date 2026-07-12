// Package disclosureview is the one disclosures view over the
// internal/disclosure seam (spec/disclosures-panel, implementing
// spec/disclosure-legibility#ac-2's "one view"): a fresh, never-persisted
// enumeration of the checkout's current disclosures (Current) plus the
// single shared item/empty-state markup both editions consume (HTML) —
// the workbench's live page and the dex's read-only, main-only edition
// render through these same two functions, never a private
// reimplementation (05 §Lenses: "computed the same way — no separate
// logic path", the law the story-page ladder badges already obey).
//
// Placement note, disclosed: the spike
// (docs/spikes/v1/disclosure-enumeration-spike.md) sketched the
// aggregator as internal/disclosure.Enumerate. internal/lint renders its
// findings through internal/disclosure (spec/disclosure-seam-v2), so an
// aggregator inside internal/disclosure that calls the lint engine would
// be an import cycle; it therefore lives one package over, still
// consuming and emitting only the seam's own Disclosure values.
package disclosureview

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"sort"

	"github.com/OWNER/verdi/internal/disclosure"
	"github.com/OWNER/verdi/internal/lint"
)

// Current enumerates every current disclosure for the checkout at root,
// computed fresh on every call and never persisted (spec/disclosures-panel
// ac-1; the feature's dc-1: a disclosure is a rendered state reflecting
// the checkout's current condition, not a historical log — nothing here
// writes a file, a cache, or a log).
//
// It calls the same decision points the producing surfaces already call:
// the full lint engine (the same VL-001..018 run `verdi lint` performs,
// under the same BuildContext) contributes every disclosure-severity
// finding via the one Finding->Disclosure mapping lint's own CLI line
// renders through; extras carries the calling process's own
// already-computed disclosed context (e.g. the review-feed-unavailable
// state `verdi serve` computes at startup) — already seam values, simply
// collected. Violation-severity findings are deliberately absent: a
// violation is a verdict failure reported through its own channel, never
// a disclosure (the seam's own severity reasoning).
//
// The result is deterministically ordered (by the seam's stable id, then
// text) so two calls against the same checkout state enumerate
// identically.
func Current(ctx context.Context, root string, extras ...disclosure.Disclosure) ([]disclosure.Disclosure, error) {
	lctx := lint.BuildContext(ctx, root)
	findings, err := lint.NewEngine().Run(ctx, root, lctx, lint.Options{})
	if err != nil {
		return nil, fmt.Errorf("disclosureview: enumerating lint disclosures: %w", err)
	}

	var items []disclosure.Disclosure
	for _, f := range findings {
		if f.Severity == lint.SeverityDisclosure {
			items = append(items, f.Disclosure())
		}
	}
	items = append(items, extras...)

	sort.Slice(items, func(i, j int) bool {
		if items[i].ID != items[j].ID {
			return items[i].ID < items[j].ID
		}
		return items[i].Text < items[j].Text
	})
	return items, nil
}

// viewTemplate is the one item/empty-state markup both editions render
// (spec/disclosures-panel ac-2/ac-3): every item, regardless of producer,
// carries the seam's fields — severity, source, scope, text, and the
// stable id — in the identical structure, so recognizing one item teaches
// you to recognize all of them. An empty enumeration renders an explicit
// positive claim, never a blank region: in this product the empty state
// is the good news, and it reads like a verdict, not an absence.
// Producer-authored text is data, never instructions (05 §MCP server's
// safety note) — html/template contextually escapes every field.
var viewTemplate = template.Must(template.New("disclosures").Parse(`<section class="disclosures-view" data-count="{{len .Items}}">
{{if .Items}}<p class="disclosures-count"><strong>{{len .Items}}</strong> current {{if eq (len .Items) 1}}disclosure{{else}}disclosures{{end}} — claims this checkout is not proving right now.</p>
<ul class="disclosure-list">
{{range .Items}}<li class="disclosure-item" data-disclosure-id="{{.ID}}">
<div class="disclosure-head"><span class="disclosure-severity">{{.Severity}}</span><code class="disclosure-source">{{.Source}}</code>{{if .Scope}}<code class="disclosure-scope">{{.Scope}}</code>{{end}}</div>
<p class="disclosure-text">{{.Text}}</p>
</li>
{{end}}</ul>
{{else}}<div class="disclosures-empty">
<p class="disclosures-empty-claim">No current disclosures.</p>
<p class="disclosures-empty-detail">Every source this view enumerates reports nothing disclosed-unproven for this checkout — a computed claim, not a silent pass.</p>
</div>
{{end}}{{if .Note}}<p class="disclosures-note">{{.Note}}</p>
{{end}}</section>
`))

// HTML renders items through the shared view markup. note is the
// edition's own compute-provenance line (the workbench's "computed fresh
// per render, never persisted"; the dex's build-time equivalent) — chrome
// text, not disclosure logic, so it is the one parameter the two editions
// legitimately differ on.
func HTML(items []disclosure.Disclosure, note string) template.HTML {
	var buf bytes.Buffer
	err := viewTemplate.Execute(&buf, struct {
		Items []disclosure.Disclosure
		Note  string
	}{Items: items, Note: note})
	if err != nil {
		// The template is compile-time fixed and the data is two plain
		// fields; execution cannot fail on well-formed inputs. Render the
		// failure honestly rather than silently returning nothing.
		return template.HTML("<section class=\"disclosures-view\" data-count=\"0\"><p class=\"disclosures-note\">disclosures view failed to render: " + template.HTMLEscapeString(err.Error()) + "</p></section>")
	}
	return template.HTML(buf.String()) //nolint:gosec // buf is this fixed template's own output; all data fields are contextually escaped by html/template
}
