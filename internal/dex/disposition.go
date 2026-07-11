package dex

import (
	"bytes"
	stdhtml "html"
	"html/template"

	"github.com/OWNER/verdi/internal/artifact"
)

// renderDispositionsTable renders a feature spec's I-5 `dispositions:`
// block as an HTML table — "workbench and dex render the block as a table
// so humans never read raw YAML" (I-5) — one row per disposition: the
// board sticky id, its disposition value, and the value-specific detail
// (an incorporated entry's `where` anchor, rendered as a same-page link
// since I-5 requires it to resolve within the spec body; a contradicted
// entry's `note`; an open-question entry has neither). Returns "" for an
// empty block (component specs and draft feature specs carry none).
func renderDispositionsTable(ds []artifact.Disposition) template.HTML {
	if len(ds) == 0 {
		return ""
	}
	var buf bytes.Buffer
	buf.WriteString("<table class=\"dispositions-table\"><thead><tr><th>Sticky</th><th>Disposition</th><th>Detail</th></tr></thead><tbody>\n")
	for _, d := range ds {
		buf.WriteString("<tr><td>")
		buf.WriteString(stdhtml.EscapeString(d.Sticky))
		buf.WriteString("</td><td class=\"disposition-")
		buf.WriteString(stdhtml.EscapeString(string(d.Disposition)))
		buf.WriteString("\">")
		buf.WriteString(stdhtml.EscapeString(string(d.Disposition)))
		buf.WriteString("</td><td>")
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
