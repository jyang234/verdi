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
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/disclosureview"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/store"
)

// countDisclosures is the index's one read of the disclosures enumeration:
// disclosureview.Count, the length of what /disclosures shows for the same
// inputs, read through disclosureview's process-wide cache, which only the
// index uses (SI-295; a variable so a test can count its calls per
// render).
var countDisclosures = disclosureview.Count

// indexHandler answers GET / with the whole-store directory home. It owns
// exactly the "/" route; any other path that falls through to this
// catch-all renders the generic disclosed 404 surface (notfound.go's
// renderPathNotFound — dc-5: never a bare NotFound). The stale-entry shape
// for a deleted design branch's board address is owned by the registered
// per-branch route (branchboard.go's dispatch), not this catch-all.
//
// extras is the serving process's own disclosed context (Deps.Disclosures)
// — the same values /disclosures appends — so the index's count and the
// page agree.
func indexHandler(root string, home HomeDeps, extras []disclosure.Disclosure) http.HandlerFunc {
	home = home.resolve(root)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			renderPathNotFound(r.Context(), w, root, r.URL.Path)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		out, err := renderHome(r.Context(), root, home, extras, indexViewOf(r.URL.Query()))
		if err != nil {
			renderError(r.Context(), w, root, http.StatusInternalServerError, err)
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
// The directory is computed only through the ref-index seam — ONE call
// per render (dc-2). This renderer's own code enumerates no git refs and
// holds no second copy of the grouping rules. The disclosures count comes
// from the disclosure seam's cached enumeration (countDisclosures →
// disclosureview.Count; SI-295), whose lint context and cache key read
// refs as that seam's own reads (SI-301), as the in-review consultation's
// forge reads are its own. The in-review consultation (dc-4) is
// per-render, bounded, and non-blocking: its failure is disclosed while
// the refs-computed directory still renders fully. view is the view the
// request chose (indexview.go): the same DOM either way, the directory
// carrying it as its data-view and the bar's toggle naming it current.
func renderHome(ctx context.Context, root string, home HomeDeps, extras []disclosure.Disclosure, view indexView) ([]byte, error) {
	var body bytes.Buffer

	body.WriteString(`<p class="store-root">Store root: <code>`)
	body.WriteString(stdhtml.EscapeString(root))
	body.WriteString(`</code></p>`)

	// The disclosures pointer (spec/disclosures-panel) and the import
	// link (spec-import-contract) are the bar's controls now (ac-5; SI-366
	// (5), (17); indexbar.go), ahead of the directory's columns.

	// The whole-store directory (spec/directory-home ac-1): the ref-index
	// seam consumed once, the render's one clock reading, the corpus index
	// built once (the other-records listing below and the cards'
	// backlinks — built ahead of the directory so its cards can read it;
	// spec/index-v2 ac-4, SI-366 (12)), then the per-render forge
	// consultation.
	entries, indexErr := home.Index(ctx)
	now := home.Clock()
	ix, corpusErr := home.Corpus(root)
	corpus := newCorpusRead(ix, corpusErr)
	inReview, mrNotice := consultOpenMRs(ctx, home.OpenMRs)

	// Every entry's card facts (indexcards.go), projected once from the
	// inputs above — no second computation of any of them.
	var cards []cardFacts
	if indexErr == nil {
		cards = homeCards(root, entries, cardContext{
			review: reviewConsultation{configured: home.OpenMRs != nil, failed: mrNotice != "", inReview: inReview},
			corpus: corpus,
			now:    now,
			words:  classWords{m: home.Model},
		})
	}

	// The directory's four columns (spec/index-v2 ac-1; parent dc-12,
	// which merged the former leading glance into them): one rendering
	// pass over the cards above, every entry exactly once.
	writeDirectorySection(&body, cards, indexErr, mrNotice, home.OpenMRs != nil, home.Model, now, view)

	// The other-records strip below the columns (spec/index-v2 ac-5): the
	// non-spec corpus kinds — a surviving affordance of the old home page,
	// still read from the serving working tree (they have no per-branch
	// story) — the discovered services, and the grandfathered v0 boards,
	// each a collapsed <details> carrying its count, so the strip never
	// leads and every listing opens without JavaScript.
	body.WriteString(`<section class="home-strip" aria-labelledby="home-strip-heading"><h2 id="home-strip-heading">Other records</h2>`)
	writeOtherKindsSection(&body, ix, corpus.err)
	writeServicesSection(&body, root)
	writeBoardsSection(&body, root)
	body.WriteString(`</section>`)

	return renderPage(ctx, root, pageData{
		Title:       "Workbench",
		Surface:     true, // the one page whose wordmark wears WORKBENCH (handoff "Global chrome")
		BodyHTML:    template.HTML(body.String()),
		ExtraHTML:   indexScriptTag,
		BarControls: indexBarControls(ctx, root, extras, classWords{m: home.Model}, view),
	})
}

// indexScriptTag loads the index's own script, /assets/index.js (the
// filter row, the view toggle and the keyboard; spec/workbench-redesign
// co-1: new behaviour ships in a new asset within 64 KiB), deferred, so
// the page — every card shown, every fold a native <details>, the view
// the query chose — is complete before the script enhances it.
const indexScriptTag = template.HTML(`<script src="/assets/index.js" defer></script>`)

// writeStripSummary opens one strip section as a collapsed <details>
// (ac-5: expandable without JavaScript) whose summary names it and
// carries its count — or, when the count could not be read, a disclosed
// unproven mark naming why, never a zero standing in for an unread
// listing. class is the section's kept class (home-kinds, home-services,
// home-boards), the test id the same.
func writeStripSummary(buf *bytes.Buffer, class, name string, count int, unproven string) {
	buf.WriteString(`<details class="`)
	buf.WriteString(class)
	buf.WriteString(`" data-testid="`)
	buf.WriteString(class)
	buf.WriteString(`"><summary>`)
	buf.WriteString(stdhtml.EscapeString(name))
	if unproven != "" {
		buf.WriteString(` <span class="dir-unproven" title="`)
		buf.WriteString(stdhtml.EscapeString(unproven))
		buf.WriteString(`">count unproven</span>`)
	} else {
		buf.WriteString(` <span class="count">`)
		buf.WriteString(strconv.Itoa(count))
		buf.WriteString(`</span>`)
	}
	buf.WriteString(`</summary>`)
}

// writeOtherKindsSection groups every non-spec, non-external committed-zone
// kind (adr, diagram, attestation, waiver, conflict) with a count, linking
// each artifact to its corpus page, folded under one summary carrying the
// total. External refs (discovered services) are their own section below
// and carry no corpus page. corpusErr is why the corpus could not be read,
// when it could not: the fold then carries the notice and no count.
func writeOtherKindsSection(buf *bytes.Buffer, ix *index.Index, corpusErr error) {
	if corpusErr != nil {
		writeStripSummary(buf, "home-kinds", "Other artifacts", 0, corpusErr.Error())
		buf.WriteString(`<p class="notice">Could not read the corpus for this store: `)
		buf.WriteString(stdhtml.EscapeString(corpusErr.Error()))
		buf.WriteString(`</p></details>`)
		return
	}
	byKind := map[string][]*index.Entry{}
	var kinds []string
	total := 0
	for _, e := range ix.All() {
		if e.Kind == "spec" || e.Kind == "external" {
			continue
		}
		if _, ok := byKind[e.Kind]; !ok {
			kinds = append(kinds, e.Kind)
		}
		byKind[e.Kind] = append(byKind[e.Kind], e)
		total++
	}
	sort.Strings(kinds)

	writeStripSummary(buf, "home-kinds", "Other artifacts", total, "")
	if len(kinds) == 0 {
		buf.WriteString(`<p class="empty">No other artifacts.</p></details>`)
		return
	}
	for _, k := range kinds {
		entries := byKind[k]
		buf.WriteString(`<h3>`)
		buf.WriteString(stdhtml.EscapeString(k))
		buf.WriteString(` <span class="count">`)
		buf.WriteString(strconv.Itoa(len(entries)))
		buf.WriteString(`</span></h3><ul>`)
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
	buf.WriteString(`</details>`)
}

// writeServicesSection lists the store's discovered services (05 §MCP
// federation: verdi discovers service roots via .flowmap.yaml). Services
// have no dedicated workbench page in v0, so each is named (with its
// obligation count) rather than linked.
func writeServicesSection(buf *bytes.Buffer, root string) {
	services, err := store.DiscoverServices(root)
	if err != nil {
		writeStripSummary(buf, "home-services", "Services", 0, err.Error())
		buf.WriteString(`<p class="notice">Could not discover services: `)
		buf.WriteString(stdhtml.EscapeString(err.Error()))
		buf.WriteString(`</p></details>`)
		return
	}
	writeStripSummary(buf, "home-services", "Services", len(services), "")
	if len(services) == 0 {
		buf.WriteString(`<p class="empty">No services discovered.</p></details>`)
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
	buf.WriteString(`</ul></details>`)
}

// writeBoardsSection enumerates data/mutable/boards/*.json under the store
// root and links each to its /board/<key> page. When none exist it says so
// honestly rather than rendering an empty list.
func writeBoardsSection(buf *bytes.Buffer, root string) {
	entries, err := os.ReadDir(boardio.BoardsDir(root))
	if err != nil {
		if os.IsNotExist(err) {
			writeStripSummary(buf, "home-boards", "Boards", 0, "")
			buf.WriteString(`<p class="empty">No boards yet.</p></details>`)
			return
		}
		writeStripSummary(buf, "home-boards", "Boards", 0, err.Error())
		buf.WriteString(`<p class="notice">Could not read boards: `)
		buf.WriteString(stdhtml.EscapeString(err.Error()))
		buf.WriteString(`</p></details>`)
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

	writeStripSummary(buf, "home-boards", "Boards", len(keys), "")
	if len(keys) == 0 {
		buf.WriteString(`<p class="empty">No boards yet.</p></details>`)
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
	buf.WriteString(`</ul></details>`)
}
