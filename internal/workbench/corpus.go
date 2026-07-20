// The corpus artifact page: GET /a/{kind}/{name} — a server-rendered view
// of one committed-zone artifact (05 §Workbench: "Every page is
// server-rendered ... except ... the board"). Frontmatter card, rendered
// body, links/backlinks panel, and — for a feature spec — the I-5
// dispositions table (views never authoritative, constitution 6; the same
// table internal/dex renders on its own permalink pages, via the same
// internal/render.DispositionsTable).
package workbench

import (
	"bytes"
	stdhtml "html"
	"html/template"
	"net/http"
	"os"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/artifactview"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/render"
)

// corpusHandler answers GET /a/{kind}/{name}. mdl is the store's resolved
// operating model — the metadata card's class-word display sites resolve
// through it (corpusMetaRows); nil serves bare ids.
func corpusHandler(root string, mdl *model.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		kind := r.PathValue("kind")
		name := r.PathValue("name")
		if kind == "" || name == "" {
			http.NotFound(w, r)
			return
		}
		ref := kind + "/" + name

		ix, err := index.Build(root)
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		entry, ok := ix.Get(ref)
		if !ok || entry.Kind == "external" {
			http.NotFound(w, r)
			return
		}

		data, err := os.ReadFile(entry.Path)
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		fm, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		meta, err := artifactview.DecodeMeta(entry.Kind, fm)
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}

		bodyHTML, err := render.RenderBody(entry.Kind, entry.DiagramClass, entry.Body)
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}

		extra := corpusConnectionsHTML(ix, ref, entry.Links)
		// A class: proposal diagram's corpus page links to its board
		// editor (spec/board-editor dc-1: "reachable from ... the corpus
		// page"). Incumbent authored-living diagrams have no editor
		// surface, so they get no link.
		if entry.Kind == "diagram" {
			if d, derr := artifact.DecodeDiagram(fm); derr == nil && d.Class == artifact.DiagramClassProposal {
				link := `<p class="diagram-editor-link"><a data-testid="open-editor-link" href="/board/diagram/` +
					// vocab:identity — "draft" the VERB (draft this proposal), not the lifecycle state
					stdhtml.EscapeString(name) + `">Open in the board editor</a> &#8212; draft this proposal with a live preview and structural operations.</p>`
				extra = template.HTML(link) + extra
			}
		}

		page := pageData{
			Title:            entry.Title,
			Nav:              template.HTML(`<a href="/">index</a>`),
			MetaRows:         corpusMetaRows(entry, meta, mdl),
			BodyHTML:         template.HTML(bodyHTML),
			DispositionsHTML: render.DispositionsTable(meta.Dispositions),
			ExtraHTML:        extra,
		}
		out, err := renderPage(page)
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(out) // response body write; post-header error is unactionable
	}
}

// corpusMetaRows builds the frontmatter card: kind, status, owners, class,
// story, decided, frozen, reason/expiry. Display words resolve through
// mdl exactly as the dex twin's artifactMetaRows does (one rename, both
// twins at once): the Class VALUE and the "Story" row LABEL are class
// words, the Status VALUE a state word; the kind, the story tracker ref,
// and every other value are identity and stay verbatim.
func corpusMetaRows(e *index.Entry, m artifactview.Meta, mdl *model.Model) []metaRow {
	rows := []metaRow{{Label: "Kind", Value: e.Kind}}
	if e.Status != "" {
		// The entry's own declared class rides along (Q2's caller
		// convention at DisplayState) — "" for a class-less knowledge
		// artifact, the documented degenerate.
		rows = append(rows, metaRow{Label: "Status", Value: mdl.DisplayState(string(m.Class), e.Status)})
	}
	if len(m.Base.Owners) > 0 {
		rows = append(rows, metaRow{Label: "Owners", Value: strings.Join(m.Base.Owners, ", ")})
	}
	if m.Class != "" {
		rows = append(rows, metaRow{Label: "Class", Value: mdl.DisplayClass(string(m.Class))})
	}
	if m.Story != "" {
		rows = append(rows, metaRow{Label: model.Capitalize(mdl.DisplayClass("story")), Value: m.Story})
	}
	if m.Decided != "" {
		rows = append(rows, metaRow{Label: "Decided", Value: m.Decided})
	}
	if m.Base.Frozen != nil {
		rows = append(rows, metaRow{Label: "Frozen", Value: m.Base.Frozen.At + " @ " + m.Base.Frozen.Commit})
	}
	if m.Reason != "" {
		rows = append(rows, metaRow{Label: "Reason", Value: m.Reason})
	}
	if m.Expiry != "" {
		rows = append(rows, metaRow{Label: "Expiry", Value: m.Expiry})
	}
	return rows
}

// corpusConnectionsHTML renders the links/backlinks panel: this artifact's
// outgoing typed links, plus internal/index's computed backlinks pointing
// at it.
func corpusConnectionsHTML(ix *index.Index, ref string, links []artifact.Link) template.HTML {
	var buf bytes.Buffer
	buf.WriteString(`<section class="connections"><h2>Links and backlinks</h2><ul>`)
	for _, l := range links {
		buf.WriteString("<li>")
		buf.WriteString(stdhtml.EscapeString(string(l.Type)))
		buf.WriteString(" &rarr; ")
		writeRefLink(&buf, l.Ref)
		if l.Note != "" {
			buf.WriteString(" &mdash; ")
			buf.WriteString(stdhtml.EscapeString(l.Note))
		}
		buf.WriteString("</li>")
	}
	for _, bl := range ix.Backlinks(ref) {
		buf.WriteString("<li>")
		buf.WriteString(stdhtml.EscapeString(bl.Type))
		buf.WriteString(" &larr; ")
		writeRefLink(&buf, bl.From)
		buf.WriteString("</li>")
	}
	buf.WriteString("</ul></section>")
	return template.HTML(buf.String())
}

// writeRefLink writes ref as an <a> to its corpus page when ref has the
// plain kind/name shape (a permalink this workbench serves), or as plain
// text otherwise (an external svc/... ref, or a scheme:key story ref —
// neither has a workbench page in v0's read-path scope).
func writeRefLink(buf *bytes.Buffer, ref string) {
	kind, name, ok := splitSimpleRef(ref)
	if !ok {
		buf.WriteString(stdhtml.EscapeString(ref))
		return
	}
	buf.WriteString(`<a href="/a/`)
	buf.WriteString(stdhtml.EscapeString(kind))
	buf.WriteString("/")
	buf.WriteString(stdhtml.EscapeString(name))
	buf.WriteString(`">`)
	buf.WriteString(stdhtml.EscapeString(ref))
	buf.WriteString("</a>")
}

// splitSimpleRef splits a "kind/name" ref (no pin, no svc/ external form)
// into its two parts.
func splitSimpleRef(ref string) (kind, name string, ok bool) {
	if strings.Contains(ref, "@") || strings.HasPrefix(ref, "svc/") || strings.Contains(ref, ":") {
		return "", "", false
	}
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
