package render

import (
	"bytes"
	stdhtml "html"
	"html/template"

	"github.com/OWNER/verdi/internal/artifact"
)

// DispositionsTable renders a feature spec's I-5 `dispositions:` block as an
// HTML table — "workbench and dex render the block as a table so humans
// never read raw YAML" (I-5) — one row per disposition: the board sticky
// id, its disposition value, and the value-specific detail (an incorporated
// entry's `where` anchor, rendered as a same-page link since I-5 requires
// it to resolve within the spec body; a contradicted entry's `note`; an
// open-question entry has neither). Returns "" for an empty block (component
// specs and draft feature specs with no board history carry none).
func DispositionsTable(ds []artifact.Disposition) template.HTML {
	if len(ds) == 0 {
		return ""
	}
	var buf bytes.Buffer
	buf.WriteString("<table class=\"dispositions-table\"><thead><tr><th>Sticky</th><th>Disposition</th><th>Detail</th></tr></thead><tbody>\n")
	for _, d := range ds {
		// The sticky id (a ULID) is visually truncated by CSS (.ulid) while
		// the full id stays intact as the cell's text — selecting/copying the
		// cell yields the full form — and in title for hover/AT.
		buf.WriteString("<tr><td><code class=\"ulid\" title=\"")
		buf.WriteString(stdhtml.EscapeString(d.Sticky))
		buf.WriteString("\">")
		buf.WriteString(stdhtml.EscapeString(d.Sticky))
		buf.WriteString("</code></td><td class=\"disposition-")
		buf.WriteString(stdhtml.EscapeString(string(d.Disposition)))
		buf.WriteString("\"><span class=\"status-badge\">")
		buf.WriteString(stdhtml.EscapeString(string(d.Disposition)))
		buf.WriteString("</span></td><td>")
		buf.WriteString(dispositionDetail(d))
		buf.WriteString("</td></tr>\n")
	}
	buf.WriteString("</tbody></table>\n")
	return template.HTML(buf.String())
}

// dispositionDetail renders one disposition's value-specific detail cell.
func dispositionDetail(d artifact.Disposition) string {
	switch d.Disposition {
	case artifact.DispositionIncorporated:
		return "<a href=\"" + stdhtml.EscapeString(d.Where) + "\">" + stdhtml.EscapeString(d.Where) + "</a>"
	case artifact.DispositionContradicted:
		return stdhtml.EscapeString(d.Note)
	default:
		return ""
	}
}
