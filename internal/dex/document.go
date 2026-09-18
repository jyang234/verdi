package dex

import (
	"context"
	"fmt"
	"html/template"
	"path"
	"strings"

	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specdocload"
)

// documentPageDir is the subdirectory, beside a spec's permalink page,
// that serves its Document view: /a/spec/<name>/document/.
const documentPageDir = "document"

// specDocumentURL returns the page-relative URL of ref's Document view
// ("document/", resolving under the spec page's directory-form permalink),
// or "" for any page that is not a spec — only specs have documents
// (spec/spec-documents ac-4).
func specDocumentURL(ref string) string {
	if !strings.HasPrefix(ref, "spec/") {
		return ""
	}
	return documentPageDir + "/"
}

// writeSpecDocuments writes the three Markdown documents and the Document
// view beside a spec's page, rendered at the site's build commit through
// the shared loader so the bytes match the CLI, the board, and MCP
// (spec/spec-documents ac-4, ac-6). A page that is not a spec gets
// nothing. The build commit is the ONLY commit the render reads: ModeAt at
// stamp.SHA, never the loader's Result.Head (which may be empty when HEAD
// cannot be resolved and is in any case not the site's pin). Loader
// disclosures are not surfaced separately: the document itself already
// says, section by section, which facts were unavailable (ac-1), and the
// static site has no per-page disclosure slot to hang them on.
func writeSpecDocuments(ctx context.Context, outDir, root string, stamp buildStamp, mdl *model.Model, p *artifactPage) error {
	if specDocumentURL(p.Entry.Ref) == "" {
		return nil
	}
	name := strings.TrimPrefix(p.Entry.Ref, "spec/")
	base := path.Dir(permalinkOutPath(p.Entry.Ref)) // a/spec/<name>
	var specHTML string
	for _, kind := range []specdoc.Kind{specdoc.KindSpec, specdoc.KindPlan, specdoc.KindTasks} {
		res, err := specdocload.Load(ctx, specdocload.Request{Root: root, Name: name, Mode: specdocload.ModeAt, At: stamp.SHA, Kind: kind, Model: mdl})
		if err != nil {
			return fmt.Errorf("dex: document for %s: %w", p.Entry.Ref, err)
		}
		doc, err := specdoc.Build(res.Input)
		if err != nil {
			return fmt.Errorf("dex: document for %s: %w", p.Entry.Ref, err)
		}
		if err := writeFile(outDir, path.Join(base, string(kind)+".md"), []byte(specdoc.RenderMarkdown(doc))); err != nil {
			return err
		}
		if kind == specdoc.KindSpec {
			specHTML, err = specdoc.RenderHTML(doc)
			if err != nil {
				return fmt.Errorf("dex: document html for %s: %w", p.Entry.Ref, err)
			}
		}
	}

	// The shell's own <h1> is the document's title, so the fragment's
	// leading <h1> is folded into it rather than rendered twice; the
	// Markdown files above — the parity bytes (ac-6) — are untouched.
	crumbs := pageBreadcrumb(p.Entry.Kind, p.Entry.Title, isArchivedSpec(p.RelPath))
	crumbs[len(crumbs)-1].URL = permalinkURL(p.Entry.Ref)
	crumbs = append(crumbs, breadcrumbEntry{Label: "Document"})
	page, err := renderPage(mdl, pageData{
		Title:      p.Entry.Title + " — document",
		Breadcrumb: crumbs,
		// A dex-synthesized rendering at the build commit is living-gated,
		// the same class as every listing page (01 §Temporal classes).
		Banner:   livingGatedBanner(stamp),
		BodyHTML: template.HTML(documentViewChrome(p.Entry.Ref) + stripLeadingH1(specHTML)),
		TOC:      extractTOC(specHTML),
		CopyRef:  p.Entry.Ref + "@" + stamp.SHA,
	})
	if err != nil {
		return err
	}
	return writeFile(outDir, path.Join(base, documentPageDir, "index.html"), page)
}

// documentViewChrome is the quiet header a reader meets before the
// document: the provenance sentence (co-2: the document is never
// authority) and the three Markdown files, offered as downloads. Links are
// relative to the document page's own directory, so "../" is the spec's
// permalink page and "../<kind>.md" its files.
func documentViewChrome(ref string) string {
	esc := template.HTMLEscapeString(ref)
	return `<p class="document-note">A reading of <code>` + esc + `</code> rendered from its objects at the site's build commit; not authority. ` +
		`The <a href="../">artifact page</a> holds the verbatim body.</p>` +
		`<nav class="document-files" aria-label="Document files"><span>Markdown</span> ` +
		`<a href="../spec.md" download>spec.md</a> ` +
		`<a href="../plan.md" download>plan.md</a> ` +
		`<a href="../tasks.md" download>tasks.md</a></nav>`
}

// stripLeadingH1 drops a fragment's leading "<h1 ...>...</h1>\n" — the
// title line goldmark emits for the document's "# <title>" — and returns
// every other fragment unchanged.
func stripLeadingH1(fragment string) string {
	if !strings.HasPrefix(fragment, "<h1") {
		return fragment
	}
	const end = "</h1>\n"
	i := strings.Index(fragment, end)
	if i < 0 {
		return fragment
	}
	return fragment[i+len(end):]
}
