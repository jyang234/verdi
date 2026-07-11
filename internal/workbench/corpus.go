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

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/artifactview"
	"github.com/OWNER/verdi/internal/index"
	"github.com/OWNER/verdi/internal/render"
)

// corpusHandler answers GET /a/{kind}/{name}.
func corpusHandler(root string) http.HandlerFunc {
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
			http.Error(w, "workbench: building index: "+err.Error(), http.StatusInternalServerError)
			return
		}
		entry, ok := ix.Get(ref)
		if !ok || entry.Kind == "external" {
			http.NotFound(w, r)
			return
		}

		data, err := os.ReadFile(entry.Path)
		if err != nil {
			http.Error(w, "workbench: reading "+entry.Path+": "+err.Error(), http.StatusInternalServerError)
			return
		}
		fm, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			http.Error(w, "workbench: "+entry.Path+": "+err.Error(), http.StatusInternalServerError)
			return
		}
		meta, err := artifactview.DecodeMeta(entry.Kind, fm)
		if err != nil {
			http.Error(w, "workbench: "+entry.Path+": "+err.Error(), http.StatusInternalServerError)
			return
		}

		bodyHTML, err := render.RenderMarkdown(entry.Body)
		if err != nil {
			http.Error(w, "workbench: rendering "+ref+": "+err.Error(), http.StatusInternalServerError)
			return
		}

		page := pageData{
			Title:            entry.Title,
			Nav:              `<a href="/">index</a>`,
			MetaRows:         corpusMetaRows(entry, meta),
			BodyHTML:         template.HTML(bodyHTML),
			DispositionsHTML: render.DispositionsTable(meta.Dispositions),
			ExtraHTML:        corpusConnectionsHTML(ix, ref, entry.Links),
		}
		out, err := renderPage(page)
		if err != nil {
			http.Error(w, "workbench: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(out)
	}
}

// corpusMetaRows builds the frontmatter card: kind, status, owners, class,
// story, decided, frozen, reason/expiry.
func corpusMetaRows(e *index.Entry, m artifactview.Meta) []metaRow {
	rows := []metaRow{{Label: "Kind", Value: e.Kind}}
	if e.Status != "" {
		rows = append(rows, metaRow{Label: "Status", Value: e.Status})
	}
	if len(m.Base.Owners) > 0 {
		rows = append(rows, metaRow{Label: "Owners", Value: strings.Join(m.Base.Owners, ", ")})
	}
	if m.Class != "" {
		rows = append(rows, metaRow{Label: "Class", Value: string(m.Class)})
	}
	if m.Story != "" {
		rows = append(rows, metaRow{Label: "Story", Value: m.Story})
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
