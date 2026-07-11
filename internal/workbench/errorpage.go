// Shared error rendering for the workbench's page handlers (DEFECT B):
// an operational failure that used to surface as a bare `http.Error` 500
// — the error text buried in the server log, the browser showing a blank
// page — instead renders as a legible page on the workbench's own shell
// (shared stylesheet, breadcrumb-home nav), carrying the error message and,
// for the fold's deliberately-loud unknown-commit / dangling-record class,
// a hint that this store's derived/ zone may be stale.
//
// This changes only the SURFACING, never the semantics: the fold stays
// loud (03 §Evidence records) — renderError writes the same non-2xx status
// the handler chose, so a witness-bearing failure is still a failure, just
// an honest one a human can read.
package workbench

import (
	stdhtml "html"
	"html/template"
	"net/http"
	"strings"
)

// staleDerivedHint is shown when the surfaced error is the fold's
// unknown-commit / dangling-record class (isStaleDerivedError): the
// derived/ zone pins commits this repo's git history does not contain, so
// the correct fix is to refresh it, not to retry the request.
const staleDerivedHint = `This store's <code>derived/</code> records pin commits this repository's ` +
	`git history does not contain, so the fold cannot verify their ancestry. The ` +
	`derived/ zone may be stale for this store; run <code>verdi sync --or-regen</code> to refresh it.`

// renderError renders err as a legible HTML error page at the given status,
// on the workbench's shared shell (renderPage). The breadcrumb-home link is
// the shell's own `<nav>` "workbench" link. When err is the fold's
// stale-derived class, the page also carries staleDerivedHint.
//
// status is written verbatim — DEFECT B's rule is that loud stays loud: the
// caller keeps choosing the (non-2xx) code, and this helper only makes the
// body honest.
func renderError(w http.ResponseWriter, status int, err error) {
	var body strings.Builder
	body.WriteString(`<div class="error-page" role="alert">`)
	body.WriteString(`<p class="error-message"><strong>The workbench could not render this page.</strong></p>`)
	body.WriteString(`<pre class="error-detail">`)
	body.WriteString(stdhtml.EscapeString(err.Error()))
	body.WriteString(`</pre>`)
	if isStaleDerivedError(err) {
		body.WriteString(`<p class="error-hint">` + staleDerivedHint + `</p>`)
	}
	body.WriteString(`</div>`)

	out, rerr := renderPage(pageData{
		Title:    "Error",
		Nav:      template.HTML(`<span class="current">error</span>`),
		BodyHTML: template.HTML(body.String()),
	})
	if rerr != nil {
		// Last resort: the shell itself failed to render. Fall back to a
		// bare text error so the caller still gets the (loud) status and the
		// message, never a blank success.
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(out) // response body write; post-header error is unactionable
}

// isStaleDerivedError reports whether err is the fold's unknown-commit or
// dangling-record class — internal/evidence's deliberately loud failures
// that mean derived/ is out of step with this repo's git history (a stale
// store), not that the user's request was malformed. Classified by the
// stable substrings those errors carry ("checking ancestry of" from
// LoadRecords's ancestry probe; "dangling binding" from the fold's
// unknown-AC guard) rather than by a typed sentinel, so this stays a pure
// consumer of internal/evidence and touches none of its fold logic.
func isStaleDerivedError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "checking ancestry of") || strings.Contains(s, "dangling binding")
}
