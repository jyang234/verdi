// The Wave 3.5 readiness pilot cockpit: GET /readiness renders the
// startup snapshot injected by the caller (Deps.Readiness) — a read-only
// projection of internal/readinesspilot's contract. The page mutates
// nothing, fetches nothing, and never recomputes readiness: an edit to
// the store is only reflected after the author restarts verdi serve,
// which the page's stale notice says out loud.
package workbench

import (
	"errors"
	"net/http"

	"github.com/jyang234/verdi/internal/readinesspilot"
)

// readinessHandler serves GET /readiness from the immutable startup
// snapshot. nil means the pilot is not wired for this process: the page
// discloses that as an honest 503 rather than rendering anything vacuous
// (three-valued honesty — silence is never a pass).
func readinessHandler(snap *readinesspilot.Snapshot) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if snap == nil {
			renderError(w, http.StatusServiceUnavailable, errors.New(
				"no readiness snapshot was injected at startup: this verdi serve process runs without the readiness pilot wired, so there is nothing honest to render"))
			return
		}
		out, err := renderReadiness(*snap)
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(out) // response body write; post-header error is unactionable
	}
}
