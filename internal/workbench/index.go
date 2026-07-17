// The workbench home page: GET / — since spec/directory-home, the
// whole-store DIRECTORY (dc-1): the computed directory index rendered as
// the page's organizing structure (directory.go), replacing the old
// single-checkout active/archived listing in place — no new route, no
// second landing page. The surviving home affordances keep their sections
// beneath the directory: the disclosures pointer, the other-artifacts
// corpus index, discovered services, and the grandfathered v0 boards.
package workbench

import (
	"bytes"
	"context"
	stdhtml "html"
	"html/template"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/store"
)

// indexHandler answers GET / with the whole-store directory home. It owns
// exactly the "/" route; any other path that falls through to this
// catch-all renders the disclosed 404 surface (notfound.go — dc-5: never a
// bare NotFound), including the stale-entry shape for a deleted design
// branch's board address.
func indexHandler(root string, home HomeDeps) http.HandlerFunc {
	home = home.resolve(root)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			renderCatchAllNotFound(w, r, root, home.Git)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		out, err := renderHome(r.Context(), root, home)
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(out) // response body write; post-header error is unactionable
	}
}

// renderHome assembles the home page body and renders it through the shared
// shell. It returns an error only if the shell template itself fails to
// execute — every data-source failure is captured inline as an honest note,
// keeping the landing page reachable even for a half-initialised store
// (dc-5: the home page is never itself a dead end).
//
// The directory is computed through the ref-index seam — ONE call per
// render (dc-2); this renderer enumerates no git refs of its own and holds
// no second copy of the grouping rules. The in-review consultation (dc-4)
// is per-render, bounded, and non-blocking: its failure is disclosed while
// the refs-computed directory still renders fully.
func renderHome(ctx context.Context, root string, home HomeDeps) ([]byte, error) {
	var body bytes.Buffer

	body.WriteString(`<p class="store-root">Store root: <code>`)
	body.WriteString(stdhtml.EscapeString(root))
	body.WriteString(`</code></p>`)

	// The disclosures view (spec/disclosures-panel): one landing-page
	// pointer so the checkout's "what is verdi not proving right now"
	// surface is discoverable, not tribal knowledge.
	body.WriteString(`<p class="home-disclosures"><a href="/disclosures">Disclosures</a> &mdash; every claim this checkout is currently not proving, in one view.</p>`)

	// The whole-store directory (spec/directory-home ac-1): the ref-index
	// seam consumed once, then the per-render forge consultation.
	entries, indexErr := home.Index(ctx)

	// The leading status glance (spec/home-status-glance dc-1): a second,
	// additive rendering pass over the SAME entries/indexErr above — no
	// second index computation. Rendered BEFORE the exhaustive Directory
	// section below (dc-5's fixed placement); it needs neither inReview
	// nor mrNotice, since a glance card never carries an in-review chip or
	// any other evidence-bearing state (dc-3).
	writeGlanceSection(&body, root, entries, indexErr)

	inReview, mrNotice := consultOpenMRs(ctx, home.OpenMRs)
	writeDirectorySection(&body, root, entries, indexErr, inReview, mrNotice, home.OpenMRs != nil)

	// The non-spec corpus kinds (adr, diagram, attestation, waiver,
	// conflict) — a surviving affordance of the old home page, still read
	// from the serving working tree (they have no per-branch story).
	if ix, err := index.Build(root); err != nil {
		body.WriteString(`<p class="notice">Could not read the corpus for this store: `)
		body.WriteString(stdhtml.EscapeString(err.Error()))
		body.WriteString(`</p>`)
	} else {
		writeOtherKindsSection(&body, ix)
	}

	writeServicesSection(&body, root)
	writeBoardsSection(&body, root)

	return renderPage(pageData{
		Title:    "Workbench",
		BodyHTML: template.HTML(body.String()),
	})
}

// writeOtherKindsSection groups every non-spec, non-external committed-zone
// kind (adr, diagram, attestation, waiver, conflict) with a count, linking
// each artifact to its corpus page. External refs (discovered services) are
// their own section below and carry no corpus page.
func writeOtherKindsSection(buf *bytes.Buffer, ix *index.Index) {
	byKind := map[string][]*index.Entry{}
	var kinds []string
	for _, e := range ix.All() {
		if e.Kind == "spec" || e.Kind == "external" {
			continue
		}
		if _, ok := byKind[e.Kind]; !ok {
			kinds = append(kinds, e.Kind)
		}
		byKind[e.Kind] = append(byKind[e.Kind], e)
	}
	sort.Strings(kinds)

	buf.WriteString(`<section class="home-kinds"><h2>Other artifacts</h2>`)
	if len(kinds) == 0 {
		buf.WriteString(`<p class="empty">No other artifacts.</p></section>`)
		return
	}
	for _, k := range kinds {
		entries := byKind[k]
		buf.WriteString(`<h3>`)
		buf.WriteString(stdhtml.EscapeString(k))
		buf.WriteString(` <span class="count">(`)
		buf.WriteString(strconv.Itoa(len(entries)))
		buf.WriteString(`)</span></h3><ul>`)
		for _, e := range entries {
			buf.WriteString(`<li>`)
			writeRefLink(buf, e.Ref) // corpus.go: links kind/name refs to /a/kind/name
			if e.Title != "" {
				buf.WriteString(` &mdash; `)
				buf.WriteString(stdhtml.EscapeString(e.Title))
			}
			buf.WriteString(`</li>`)
		}
		buf.WriteString(`</ul>`)
	}
	buf.WriteString(`</section>`)
}

// writeServicesSection lists the store's discovered services (05 §MCP
// federation: verdi discovers service roots via .flowmap.yaml). Services
// have no dedicated workbench page in v0, so each is named (with its
// obligation count) rather than linked.
func writeServicesSection(buf *bytes.Buffer, root string) {
	buf.WriteString(`<section class="home-services"><h2>Services</h2>`)
	services, err := store.DiscoverServices(root)
	if err != nil {
		buf.WriteString(`<p class="notice">Could not discover services: `)
		buf.WriteString(stdhtml.EscapeString(err.Error()))
		buf.WriteString(`</p></section>`)
		return
	}
	if len(services) == 0 {
		buf.WriteString(`<p class="empty">No services discovered.</p></section>`)
		return
	}
	buf.WriteString("<ul>")
	for _, svc := range services {
		buf.WriteString(`<li>`)
		buf.WriteString(stdhtml.EscapeString(svc.Name))
		if len(svc.Obligations) > 0 {
			// The obligations registry chip — machine-checked guarantees are a
			// service's headline fact, styled like the fold's evidenced green.
			buf.WriteString(` <span class="obligation-count">`)
			buf.WriteString(strconv.Itoa(len(svc.Obligations)))
			buf.WriteString(` obligations</span>`)
		}
		buf.WriteString(`</li>`)
	}
	buf.WriteString(`</ul></section>`)
}

// writeBoardsSection enumerates data/mutable/boards/*.json under the store
// root and links each to its /board/<key> page. When none exist it says so
// honestly rather than rendering an empty list.
func writeBoardsSection(buf *bytes.Buffer, root string) {
	buf.WriteString(`<section class="home-boards"><h2>Boards</h2>`)
	entries, err := os.ReadDir(boardio.BoardsDir(root))
	if err != nil {
		if os.IsNotExist(err) {
			buf.WriteString(`<p class="empty">No boards yet.</p></section>`)
			return
		}
		buf.WriteString(`<p class="notice">Could not read boards: `)
		buf.WriteString(stdhtml.EscapeString(err.Error()))
		buf.WriteString(`</p></section>`)
		return
	}

	var keys []string
	for _, de := range entries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".json") {
			continue
		}
		key := strings.TrimSuffix(de.Name(), ".json")
		if boardio.ValidStoryKey(key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	if len(keys) == 0 {
		buf.WriteString(`<p class="empty">No boards yet.</p></section>`)
		return
	}
	buf.WriteString("<ul>")
	for _, key := range keys {
		buf.WriteString(`<li><a href="/board/`)
		buf.WriteString(stdhtml.EscapeString(key))
		buf.WriteString(`">`)
		buf.WriteString(stdhtml.EscapeString(key))
		buf.WriteString(`</a></li>`)
	}
	buf.WriteString(`</ul></section>`)
}
