// The advisory preview matrix page: GET /matrix/{story...} — the same
// fold `verdi matrix --preview` computes (03 §Evidence records: "advisory
// renders as preview"), rendered as an HTML table behind an explicit,
// impossible-to-miss advisory-preview disclosure banner distinguishing
// advisory (source: local) evidence from authoritative (source: ci)
// evidence — 03's own requirement that a preview never be mistaken for
// the gate's real answer.
package workbench

import (
	"bytes"
	stdhtml "html"
	"html/template"
	"net/http"
	"strings"

	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

func matrixHandler(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		storyArg := r.PathValue("story")
		if storyArg == "" {
			http.NotFound(w, r)
			return
		}

		ctx := r.Context()
		spec, err := storyresolve.Resolve(root, storyArg)
		if err != nil {
			renderError(w, http.StatusNotFound, err)
			return
		}

		commit, err := gitx.RevParse(ctx, root, "HEAD")
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		derivedRoot := store.DerivedSpecDir(root, store.RefSlug(spec.ID))
		records, err := evidence.LoadRecords(ctx, root, derivedRoot, commit)
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}

		result, err := evidence.Fold(evidence.Input{
			Spec: spec, Records: records, Preview: true,
			StoreRoot: root, StorySlug: store.RefSlug(spec.Story),
		})
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}

		page := pageData{
			Title:     "Advisory preview matrix: " + result.Story,
			Nav:       template.HTML(`<a href="/">index</a>`),
			ExtraHTML: renderMatrixHTML(result),
		}
		out, err := renderPage(page)
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(out) // response body write; post-header error is unactionable
	}
}

// renderMatrixHTML renders result as an HTML table behind the mandatory
// advisory-preview disclosure banner (03 §Evidence records — the banner
// this page exists to guarantee is never absent, since this handler always
// folds with Preview: true; there is no non-preview mode on this page —
// the authoritative gate is `verdi gate`/CI, never a workbench read page).
//
// The banner's text content is EXACTLY the seam-rendered line
// disclosure.Render(disclosure.AdvisoryPreview()) — the same line `verdi
// matrix --preview` prints — never a page-local wording of the state
// (spec/disclosure-legibility#ac-1: one vocabulary; the pre-seam banner
// hand-authored a third, "PREVIEW — ADVISORY", for this same state). Only
// the vocabulary prefix (severity + [source]) is additionally wrapped in
// <strong>, so the part a CLI reader already recognizes is the part the
// eye lands on; the split is derived from the rendered line and the
// Disclosure's own Text — never a second copy of Render's grammar — and
// falls back to the whole line unemphasized if the shapes ever diverge.
func renderMatrixHTML(result evidence.StoryResult) template.HTML {
	var buf bytes.Buffer
	d := disclosure.AdvisoryPreview()
	line := disclosure.Render(d)
	buf.WriteString(`<div class="preview-banner" role="alert">`)
	if prefix, ok := strings.CutSuffix(line, ": "+d.Text); ok {
		buf.WriteString("<strong>")
		buf.WriteString(stdhtml.EscapeString(prefix))
		buf.WriteString("</strong>: ")
		buf.WriteString(stdhtml.EscapeString(d.Text))
	} else {
		buf.WriteString(stdhtml.EscapeString(line))
	}
	buf.WriteString(`</div>`)
	buf.WriteString(`<table class="matrix-table"><thead><tr><th>AC</th><th>Status</th><th>Evidence</th><th>Text</th></tr></thead><tbody>`)
	for _, r := range result.ACs {
		buf.WriteString("<tr><td>")
		buf.WriteString(stdhtml.EscapeString(r.ID))
		buf.WriteString(`</td><td class="status-`)
		buf.WriteString(stdhtml.EscapeString(string(r.Status)))
		buf.WriteString(`"><span class="status-badge">`)
		buf.WriteString(stdhtml.EscapeString(string(r.Status)))
		buf.WriteString("</span></td><td>")
		buf.WriteString(stdhtml.EscapeString(r.Summary))
		buf.WriteString("</td><td>")
		buf.WriteString(stdhtml.EscapeString(r.Text))
		buf.WriteString("</td></tr>")
	}
	buf.WriteString("</tbody></table>")
	buf.WriteString(`<p class="matrix-summary">story.violated: `)
	buf.WriteString(boolStr(result.Violated))
	buf.WriteString(" &middot; story.eligible: ")
	buf.WriteString(boolStr(result.Eligible))
	buf.WriteString("</p>")
	return template.HTML(buf.String())
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
