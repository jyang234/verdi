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
	"fmt"
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
// unchanged. A ?spec= that is not one whole spec name — malformed, or
// carrying a commit pin or an object fragment, both of which select part
// of a spec rather than the spec the loader derives — is a 400 disclosing
// which of those it was, and never reaches the loader (the loader's own
// "not an unpinned whole spec ref" refusal would arrive as a 503, the
// wrong code for a malformed query, after a derivation attempt the
// handler never had to make). A loader error (an unknown spec, a
// derivation failure) is a 503 naming the error's own text.
func readinessHandler(loader ReadinessLoader, defaultSpec string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ref := ""
		if name := r.URL.Query().Get("spec"); name != "" {
			parsed, err := artifact.ParseRef("spec/" + name)
			if err != nil {
				renderError(w, http.StatusBadRequest, err)
				return
			}
			if parsed.Pinned() {
				renderError(w, http.StatusBadRequest, fmt.Errorf("?spec= names one whole spec, but %s carries a commit pin", name))
				return
			}
			if parsed.Fragment() {
				renderError(w, http.StatusBadRequest, fmt.Errorf("?spec= names one whole spec, but %s carries an object fragment", name))
				return
			}
			// The parsed ref's own canonical spelling, never the raw query
			// text: what the loader is asked for is exactly what the gate
			// above accepted.
			ref = parsed.String()
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
