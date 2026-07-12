package workbench

// The reference-card peek: GET /board/spec/{name}/peek?ref=<ref> renders
// a small HTML fragment of the referenced artifact — title, kind,
// status, rendered body, and the full corpus-page link — so a reader
// never leaves the board to understand an external reference (owner UAT
// round 6, item 4). Read-only information, served in EVERY board mode;
// an unresolvable ref renders a DISCLOSED explanation with the same 200
// fragment shape — never a dead click, never a silent nothing. The
// fragment is a pure function of the working tree (index.Build is
// deterministic; no clock, no randomness).

import (
	stdhtml "html"
	"net/http"
	"strings"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/index"
	"github.com/OWNER/verdi/internal/render"
)

func (s *boardSpecServer) boardPeekHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ref := r.URL.Query().Get("ref")
		if ref == "" {
			http.Error(w, "peek requires a ref query parameter", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(renderPeekFragment(s.root, ref))) // response body write; post-header error is unactionable
	}
}

// renderPeekFragment renders the peek for one ref: the artifact when the
// ref resolves in this corpus, a disclosed explanation otherwise.
func renderPeekFragment(root, ref string) string {
	parsed, err := artifact.ParseRef(ref)
	if err != nil {
		return peekErrorFragment(ref, "is not an artifact reference this corpus can resolve (an external tracker key or service ref has no page here)")
	}
	simple := string(parsed.Kind) + "/" + parsed.Name

	ix, err := index.Build(root)
	if err != nil {
		return peekErrorFragment(ref, "could not be looked up — the corpus index failed to build: "+err.Error())
	}
	entry, ok := ix.Get(simple)
	if !ok || entry.Kind == "external" {
		return peekErrorFragment(ref, "is not in this corpus — it may name something outside this repository")
	}

	bodyHTML, err := render.RenderBody(entry.Kind, entry.Body)
	if err != nil {
		return peekErrorFragment(ref, "resolved, but its body could not be rendered: "+err.Error())
	}

	esc := stdhtml.EscapeString
	var b strings.Builder
	b.WriteString(`<div class="peek-head"><span class="peek-kind">` + esc(entry.Kind) + `</span>`)
	if entry.Status != "" {
		b.WriteString(`<span class="peek-status">` + esc(entry.Status) + `</span>`)
	}
	b.WriteString(`</div>`)
	b.WriteString(`<h2 class="peek-title">` + esc(entry.Title) + `</h2>`)
	b.WriteString(`<div class="peek-body">` + bodyHTML + `</div>`)
	b.WriteString(`<a class="peek-open" data-testid="ref-peek-open" href="/a/` + esc(simple) + `">open full page</a>`)
	return b.String()
}

// peekErrorFragment is the disclosed unresolvable state (constitution
// 2/10: silence is never a pass — a dead click would be one).
func peekErrorFragment(ref, reason string) string {
	esc := stdhtml.EscapeString
	return `<p class="peek-error" data-testid="ref-peek-error"><code>` + esc(ref) + `</code> ` + esc(reason) + `.</p>`
}
