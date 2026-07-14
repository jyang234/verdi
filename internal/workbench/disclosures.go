// The disclosures page (spec/disclosures-panel ac-1; 05 §Workbench
// server-rendered pages): the operator's "what is verdi not proving right
// now" surface. GET /disclosures enumerates every current disclosure for
// the checkout through internal/disclosureview's shared compute path —
// the same enumeration and the same item markup the dex's read-only
// edition renders (ac-3's no-separate-logic-path law) — computed fresh on
// every request and never persisted.
package workbench

import (
	"html/template"
	"net/http"

	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/disclosureview"
)

// disclosuresNote is the workbench edition's compute-provenance line —
// the one piece of chrome that differs from the dex edition, because the
// two editions' temporal claims genuinely differ (live render vs. build
// stamp).
const disclosuresNote = "Enumerated fresh from this checkout's current state on every render — never persisted. An entry here is a claim verdi is honestly not proving right now, not a failure."

// disclosuresHandler serves GET /disclosures. extras is the serving
// process's own disclosed context (Deps.Disclosures) — already seam
// values, appended to the fresh enumeration on every render.
func disclosuresHandler(root string, extras []disclosure.Disclosure) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		items, err := disclosureview.Current(r.Context(), root, extras...)
		if err != nil {
			// An unenumerable store is an operational failure and must
			// say so — a vacuous "no disclosures" here would be the exact
			// silent pass this page exists to forbid (constitution 2).
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		out, err := renderPage(pageData{
			Title:    "Disclosures",
			Nav:      template.HTML(`<a href="/">index</a>`),
			BodyHTML: disclosureview.HTML(items, disclosuresNote),
		})
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(out) // response body write; post-header error is unactionable
	}
}
