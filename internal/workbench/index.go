// The workbench home page: GET / — a real, server-rendered index of this
// store (DEFECT A), replacing Phase 9's health skeleton that printed the
// store root and a list of route patterns as dead-end plain text. It lists
// active specs (title, status badge, corpus link; feature specs also link
// their verdict/matrix pages via the spec's scalar story ref), archived
// specs, the other kinds grouped with counts, discovered services, and the
// store's boards — every entry a real, clickable link, so a human landing
// on the workbench has somewhere to go (05 §Workbench).
package workbench

import (
	"bytes"
	stdhtml "html"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/artifactview"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/store"
)

// indexHandler answers GET / with the store's home index. It owns exactly
// the "/" route; any other path 404s here (RegisterRoutes maps "/" to this
// catch-all).
//
// The page is built defensively: a section whose data cannot be read
// (a store with no committed zone, an unreadable boards dir) renders an
// honest note rather than failing the whole page — the home page is the
// one landing surface that must never itself be a dead end.
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
		out, err := renderHome(root)
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
// keeping the landing page reachable even for a half-initialised store.
func renderHome(root string) ([]byte, error) {
	var body bytes.Buffer

	body.WriteString(`<p class="store-root">Store root: <code>`)
	body.WriteString(stdhtml.EscapeString(root))
	body.WriteString(`</code></p>`)

	// The disclosures view (spec/disclosures-panel): one landing-page
	// pointer so the checkout's "what is verdi not proving right now"
	// surface is discoverable, not tribal knowledge.
	body.WriteString(`<p class="home-disclosures"><a href="/disclosures">Disclosures</a> — every claim this checkout is currently not proving, in one view.</p>`)

	if ix, err := index.Build(root); err != nil {
		body.WriteString(`<p class="notice">Could not read the corpus for this store: `)
		body.WriteString(stdhtml.EscapeString(err.Error()))
		body.WriteString(`</p>`)
	} else {
		writeSpecsSections(&body, ix)
		writeOtherKindsSection(&body, ix)
	}

	writeServicesSection(&body, root)
	writeBoardsSection(&body, root)

	return renderPage(pageData{
		Title:    "Workbench",
		BodyHTML: template.HTML(body.String()),
	})
}

// writeSpecsSections renders the active- and archived-spec sections. Active
// vs archived is read from each spec's on-disk path (specs/active/ vs
// specs/archive/ — 01 §Directory layout), the only signal that distinguishes
// them; the index Entry itself carries no zone field.
func writeSpecsSections(buf *bytes.Buffer, ix *index.Index) {
	var active, archived []*index.Entry
	for _, e := range ix.All() {
		if e.Kind != "spec" {
			continue
		}
		if strings.Contains(filepath.ToSlash(e.Path), "/specs/archive/") {
			archived = append(archived, e)
		} else {
			active = append(active, e)
		}
	}

	buf.WriteString(`<section class="home-specs"><h2>Active specs</h2>`)
	if len(active) == 0 {
		buf.WriteString(`<p class="empty">No active specs.</p>`)
	} else {
		buf.WriteString("<ul>")
		for _, e := range active {
			writeSpecItem(buf, e, true)
		}
		buf.WriteString("</ul>")
	}
	buf.WriteString(`</section>`)

	buf.WriteString(`<section class="home-specs-archived"><h2>Archived specs</h2>`)
	if len(archived) == 0 {
		buf.WriteString(`<p class="empty">No archived specs.</p>`)
	} else {
		buf.WriteString("<ul>")
		for _, e := range archived {
			writeSpecItem(buf, e, false)
		}
		buf.WriteString("</ul>")
	}
	buf.WriteString(`</section>`)
}

// writeSpecItem renders one spec as a list entry: title (linked to its
// corpus page), a status badge, and — when withStory is set and the spec is
// a feature spec with a scalar story ref — links to its verdict and matrix
// pages, keyed by that story ref (the same argument form storyresolve
// accepts, 05 §CLI). The class/story fields are not on the index Entry, so
// this re-decodes the spec's frontmatter through the shared artifactview
// seam; a decode failure degrades to "just the corpus link", never a broken
// page.
func writeSpecItem(buf *bytes.Buffer, e *index.Entry, withStory bool) {
	name := strings.TrimPrefix(e.Ref, "spec/")

	buf.WriteString(`<li><a href="/a/spec/`)
	buf.WriteString(stdhtml.EscapeString(name))
	buf.WriteString(`">`)
	buf.WriteString(stdhtml.EscapeString(e.Title))
	buf.WriteString(`</a>`)
	if e.Status != "" {
		// badge-<status> is the same per-status styling hook internal/dex's
		// listing pages emit, so a draft reads ochre and an accepted spec
		// green on both surfaces.
		buf.WriteString(` <span class="badge badge-`)
		buf.WriteString(stdhtml.EscapeString(e.Status))
		buf.WriteString(`">`)
		buf.WriteString(stdhtml.EscapeString(e.Status))
		buf.WriteString(`</span>`)
	}

	if withStory {
		// Every active spec has a projection board (05 §Workbench, R4:
		// the board is a view of the spec) — the four-concept minimum
		// path starts from this link.
		buf.WriteString(` &middot; <a href="/board/spec/`)
		buf.WriteString(stdhtml.EscapeString(name))
		buf.WriteString(`">board</a>`)
		if class, story := specClassStory(e); class == artifact.ClassFeature && story != "" {
			buf.WriteString(` <a href="/matrix/`)
			buf.WriteString(stdhtml.EscapeString(story))
			buf.WriteString(`">matrix</a> <a href="/verdict/`)
			buf.WriteString(stdhtml.EscapeString(story))
			buf.WriteString(`">verdict</a>`)
		}
	}
	buf.WriteString(`</li>`)
}

// specClassStory re-decodes a spec entry's frontmatter for its class and
// scalar story ref. Returns zero values on any read/decode failure — the
// caller treats that as "not a linkable feature spec".
func specClassStory(e *index.Entry) (artifact.SpecClass, string) {
	data, err := os.ReadFile(e.Path)
	if err != nil {
		return "", ""
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return "", ""
	}
	m, err := artifactview.DecodeMeta(e.Kind, fm)
	if err != nil {
		return "", ""
	}
	return m.Class, m.Story
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
