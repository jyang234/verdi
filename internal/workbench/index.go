package workbench

import (
	"fmt"
	"html"
	"net/http"
)

// indexHandler answers GET / with a minimal server-rendered index page:
// enough for a human to confirm `verdi serve` is up and see the store
// root it's serving. Phase 10 replaces this with the real corpus index
// (05 §Workbench); this handler owns exactly the "/" route so that
// replacement is a same-route swap, not a rewire.
func indexHandler(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `<!doctype html>
<html><head><title>verdi</title></head>
<body>
<h1>verdi serve</h1>
<p>store root: <code>%s</code></p>
<p><a href="/healthz">/healthz</a></p>
<p>Pages: <code>/a/&lt;kind&gt;/&lt;name&gt;</code> (corpus), <code>/verdict/&lt;story&gt;</code>
(verdict viewer), <code>/matrix/&lt;story&gt;</code> (advisory preview), <code>/board/&lt;key&gt;</code> (board).</p>
</body></html>
`, html.EscapeString(root))
	}
}
