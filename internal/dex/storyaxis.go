// The by-story axis (V1-P8; 05 §Verdi-dex IA: "by story (the archived
// quartet: spec, board, rollup, deviation report …)"). One hub listing
// every archived story record, one page per record rendering the quartet.
// Round four archives `layout.json` (the board coordinate sidecar) in the
// quartet's board-artifact slot in place of v0's frozen `board.json`;
// `board.json` is the grandfathered form, still valid and unrewritten in
// pre-R4 archives (00 §Glossary "the quartet"; 03 §Alignment report,
// round-four note) — the board slot renders whichever form the archive
// actually holds, labeled as what it is.
package dex

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/model"
)

// storyAxisURL is an archived story record's by-story page URL, keyed by
// the spec's name (a ref, not a path — stable like every dex permalink).
func storyAxisURL(name string) string {
	return "/by-story/" + name + "/"
}

// writeStoryAxis emits the by-story hub plus one quartet page per
// archived spec. pages is loadArtifactPages' sorted slice, so iteration
// — and the built bytes — are deterministic.
func writeStoryAxis(outDir string, stamp buildStamp, pages []*artifactPage, mdl *model.Model) error {
	var hub []listItem
	for _, p := range pages {
		if p.Entry.Kind != "spec" || !isArchivedSpec(p.RelPath) {
			continue
		}
		name := strings.TrimPrefix(p.Entry.Ref, "spec/")
		hub = append(hub, listItem{Title: p.Entry.Title, URL: storyAxisURL(name), Status: p.Entry.Status, StatusLabel: mdl.DisplayState("", p.Entry.Status), Sub: p.Meta.Story})
		if err := writeQuartetPage(outDir, stamp, p, name, mdl); err != nil {
			return err
		}
	}
	return writeListingPage(outDir, "/by-story/", "By story",
		[]breadcrumbEntry{{Label: "Home", URL: "/"}, {Label: "By story", URL: ""}}, stamp, hub)
}

// writeQuartetPage renders one archived story record's quartet.
func writeQuartetPage(outDir string, stamp buildStamp, p *artifactPage, name string, mdl *model.Model) error {
	dir := filepath.Dir(p.Entry.Path)

	var b strings.Builder

	// 1. The spec — its permalink page carries the full anatomy; the
	// quartet page links rather than re-rendering it.
	b.WriteString("<h2>Spec</h2>\n")
	fmt.Fprintf(&b, `<p><a href="%s">%s</a> <span class="badge badge-%s">%s</span></p>`+"\n",
		template.HTMLEscapeString(permalinkURL(p.Entry.Ref)), template.HTMLEscapeString(p.Entry.Ref),
		template.HTMLEscapeString(p.Entry.Status), template.HTMLEscapeString(mdl.DisplayState("", p.Entry.Status)))

	// 2. The board slot: layout.json (round four) or board.json
	// (grandfathered v0) — labeled as what it is, never guessed.
	boardHTML, err := renderBoardSlot(dir)
	if err != nil {
		return fmt.Errorf("dex: quartet %s: %w", p.Entry.Ref, err)
	}
	b.WriteString("<h2>Board</h2>\n")
	b.WriteString(boardHTML)

	// 3. The rollup.
	b.WriteString("<h2>Rollup</h2>\n")
	rollupHTML, err := renderQuartetJSON(filepath.Join(dir, "rollup.json"), "rollup.json", "the frozen closure rollup")
	if err != nil {
		return fmt.Errorf("dex: quartet %s: %w", p.Entry.Ref, err)
	}
	b.WriteString(rollupHTML)

	// 4. The deviation report.
	b.WriteString("<h2>Deviation report</h2>\n")
	devHTML, err := renderQuartetDeviation(filepath.Join(dir, "deviation-report.md"))
	if err != nil {
		return fmt.Errorf("dex: quartet %s: %w", p.Entry.Ref, err)
	}
	b.WriteString(devHTML)

	data := pageData{
		Title:       p.Entry.Title,
		Status:      p.Entry.Status,
		StatusLabel: mdl.DisplayState("", p.Entry.Status),
		Breadcrumb: []breadcrumbEntry{
			{Label: "Home", URL: "/"},
			{Label: "By story", URL: "/by-story/"},
			{Label: p.Entry.Title, URL: ""},
		},
		Banner:   livingGatedBanner(stamp),
		MetaRows: quartetMetaRows(p),
		BodyHTML: template.HTML(b.String()),
	}
	out, err := renderPage(data)
	if err != nil {
		return err
	}
	return writeFile(outDir, strings.TrimPrefix(storyAxisURL(name), "/")+"index.html", out)
}

// quartetMetaRows is the quartet page's small metadata card: the story
// tracker ref and the frozen stamp, both from the archived spec.
func quartetMetaRows(p *artifactPage) []metaRow {
	var rows []metaRow
	if p.Meta.Story != "" {
		rows = append(rows, metaRow{Label: "Story", Value: p.Meta.Story})
	}
	if p.Meta.Base.Frozen != nil {
		rows = append(rows, metaRow{Label: "Frozen", Value: fmt.Sprintf("%s @ %s", p.Meta.Base.Frozen.At, shortSHA(p.Meta.Base.Frozen.Commit))})
	}
	rows = append(rows, metaRow{Label: "Source", Value: p.RelPath})
	return rows
}

// renderBoardSlot renders the quartet's board-artifact slot from
// whichever form the archive holds. Both present would be a malformed
// archive — rendered both rather than guessing (never silently drop);
// neither present is stated, not omitted.
func renderBoardSlot(dir string) (string, error) {
	var b strings.Builder
	found := false

	if _, err := os.Stat(filepath.Join(dir, "layout.json")); err == nil {
		found = true
		b.WriteString(`<p class="quartet-slot-label"><code>layout.json</code> — the board coordinate sidecar (round four: the board is a projection of the spec, so the sidecar is what freezes)</p>` + "\n")
		h, err := renderQuartetJSON(filepath.Join(dir, "layout.json"), "layout.json", "")
		if err != nil {
			return "", err
		}
		b.WriteString(h)
	}
	if _, err := os.Stat(filepath.Join(dir, "board.json")); err == nil {
		found = true
		b.WriteString(`<p class="quartet-slot-label"><code>board.json</code> — frozen board snapshot (grandfathered v0 form, valid and unrewritten under its own schema)</p>` + "\n")
		h, err := renderQuartetJSON(filepath.Join(dir, "board.json"), "board.json", "")
		if err != nil {
			return "", err
		}
		b.WriteString(h)
	}
	if !found {
		b.WriteString(`<p class="empty">No board artifact in this archive.</p>` + "\n")
	}
	return b.String(), nil
}

// renderQuartetJSON pretty-prints and highlights one of the quartet's
// JSON artifacts; an absent file is stated, not omitted.
func renderQuartetJSON(path, label, describe string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf(`<p class="empty">No %s in this archive.</p>`+"\n", template.HTMLEscapeString(label)), nil
		}
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	pretty, err := json.MarshalIndent(jsonGeneric(data), "", "  ")
	if err != nil {
		return "", fmt.Errorf("pretty-printing %s: %w", path, err)
	}
	var b strings.Builder
	if describe != "" {
		fmt.Fprintf(&b, `<p class="quartet-slot-label"><code>%s</code> — %s</p>`+"\n", template.HTMLEscapeString(label), template.HTMLEscapeString(describe))
	}
	code, err := highlightCode(string(pretty), "json")
	if err != nil {
		return "", err
	}
	b.WriteString(string(code))
	b.WriteString("\n")
	return b.String(), nil
}

// renderQuartetDeviation renders the archived deviation report: the
// findings table (id, kind, disposition, note, text — the closure-time
// record of how the build diverged) followed by the report's own
// markdown body.
func renderQuartetDeviation(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return `<p class="empty">No deviation report in this archive.</p>` + "\n", nil
		}
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	fm, body, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	decoded, err := artifact.DecodeDeviation(fm)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}

	var b strings.Builder
	if len(decoded.Findings) > 0 {
		b.WriteString("<table><thead><tr><th>Finding</th><th>Kind</th><th>Disposition</th><th>Note</th><th>Text</th></tr></thead><tbody>\n")
		for _, f := range decoded.Findings {
			fmt.Fprintf(&b, "<tr><td><code>%s</code></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
				template.HTMLEscapeString(f.ID), template.HTMLEscapeString(string(f.Kind)),
				template.HTMLEscapeString(string(f.Disposition)), template.HTMLEscapeString(f.Note),
				template.HTMLEscapeString(f.Text))
		}
		b.WriteString("</tbody></table>\n")
	}
	bodyHTML, err := renderMarkdown(string(body))
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	b.WriteString(bodyHTML)
	return b.String(), nil
}
