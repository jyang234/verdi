// The Wave 3.5 readiness pilot cockpit: GET /readiness renders readiness
// derived fresh, per request, through the injected ReadinessLoader
// (spec/readiness-recovery ac-2/ac-4) — never a startup-frozen snapshot.
// ?spec=<name> names the ref explicitly; with no query the process's own
// default spec (typically `verdi serve --context-request`'s target)
// stands in; with neither, the page discloses that honestly rather than
// rendering anything vacuous (three-valued honesty — silence is never a
// pass).
package workbench

import (
	"errors"
	"net/http"

	"github.com/jyang234/verdi/internal/artifact"
)

// errReadinessNotWired is the existing (pre-loader) 503 disclosure body,
// unchanged byte-for-byte: no loader is wired for this process, or a
// loader is wired but neither a ?spec= query nor a default spec names
// anything to derive.
var errReadinessNotWired = errors.New(
	"no readiness snapshot was injected at startup: this verdi serve process runs without the readiness pilot wired, so there is nothing honest to render")

// readinessHandler serves GET /readiness by deriving readiness through
// loader for the request's own ref: ?spec=<name> when present, else
// defaultSpec when non-empty, else neither — the existing 503 disclosure,
// unchanged. A malformed ?spec= name is a 400 (never reaches the loader);
// a loader error (an unknown spec, a derivation failure) is a 503 naming
// the error's own text.
func readinessHandler(loader ReadinessLoader, defaultSpec string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ref := ""
		if name := r.URL.Query().Get("spec"); name != "" {
			if _, err := artifact.ParseRef("spec/" + name); err != nil {
				renderError(w, http.StatusBadRequest, err)
				return
			}
			ref = "spec/" + name
		} else {
			ref = defaultSpec
		}

		if ref == "" || loader == nil {
			renderError(w, http.StatusServiceUnavailable, errReadinessNotWired)
			return
		}

		snap, err := loader.Load(r.Context(), ref)
		if err != nil {
			renderError(w, http.StatusServiceUnavailable, err)
			return
		}
		out, err := renderReadiness(snap)
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(out) // response body write; post-header error is unactionable
	}
}
