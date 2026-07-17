// The disclosed 404 surface (spec/directory-home dc-5): every path the
// mux's "/" catch-all receives besides "/" itself renders as a legible
// notice page — HTTP 404 as the honest status, a human-readable body, and
// a link back to the directory — never a bare http.NotFound, never a blank
// response. Every such path gets the generic disclosed notice
// (renderPathNotFound). The stale directory entry (ac-3) — a design branch
// deleted between the directory render and the click — is now owned by the
// registered per-branch board route (branchboard.go's dispatch →
// renderBranchGone), which reuses this file's renderStaleEntryNotice; the
// "/" catch-all no longer parses that grammar itself.
package workbench

import (
	stdhtml "html"
	"html/template"
	"net/http"
	"strings"
)

// renderStaleEntryNotice renders ac-3's deleted-mid-session shape: a
// rendered, disclosed notice page — HTTP 404 as the honest status, a body
// naming the branch that vanished, and a working link back to the
// directory (dc-5). Never a bare NotFound. Its one caller is the per-branch
// board route's no-ref path (branchboard.go's renderBranchGone): now that
// the /b/ routes are registered, the vanished-branch case resolves through
// dispatch, not the "/" catch-all.
func renderStaleEntryNotice(w http.ResponseWriter, branch string) {
	var body strings.Builder
	body.WriteString(`<div class="error-page" role="alert" data-testid="stale-entry-notice">`)
	body.WriteString(`<p class="error-message"><strong>This entry&rsquo;s design branch is gone.</strong></p>`)
	body.WriteString(`<p>The design branch <code>`)
	body.WriteString(stdhtml.EscapeString(branch))
	body.WriteString(`</code> no longer resolves to any ref in this store &mdash; neither a local branch nor a remote-tracking one. It may have been deleted or merged after the directory page was rendered.</p>`)
	writeBackToDirectory(&body)
	body.WriteString(`</div>`)
	writeNotFoundPage(w, body.String())
}

// renderPathNotFound renders the generic disclosed 404 for any path this
// workbench serves nothing at — still a legible page with a way back,
// never a bare NotFound (dc-5's posture applied to the whole catch-all).
func renderPathNotFound(w http.ResponseWriter, path string) {
	var body strings.Builder
	body.WriteString(`<div class="error-page" role="alert" data-testid="path-not-found">`)
	body.WriteString(`<p class="error-message"><strong>No page is served at this address.</strong></p>`)
	body.WriteString(`<pre class="error-detail">`)
	body.WriteString(stdhtml.EscapeString(path))
	body.WriteString(`</pre>`)
	writeBackToDirectory(&body)
	body.WriteString(`</div>`)
	writeNotFoundPage(w, body.String())
}

func writeBackToDirectory(body *strings.Builder) {
	body.WriteString(`<p class="error-hint"><a href="/" data-testid="back-to-directory">Back to the directory</a></p>`)
}

// writeNotFoundPage writes bodyHTML through the shared shell at HTTP 404,
// mirroring errorpage.go's posture: the status stays loud and honest, only
// the body becomes legible. If the shell itself fails, it falls back to a
// bare text 404 rather than a blank success.
func writeNotFoundPage(w http.ResponseWriter, bodyHTML string) {
	out, err := renderPage(pageData{
		Title:    "Not found",
		Nav:      template.HTML(`<span class="current">not found</span>`),
		BodyHTML: template.HTML(bodyHTML),
	})
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write(out) // response body write; post-header error is unactionable
}
