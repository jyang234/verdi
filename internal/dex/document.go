package dex

import (
	"context"
	"fmt"
	"html/template"
	"path"
	"runtime"
	"strings"
	"sync"

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

// writeAllSpecDocuments renders every spec page's documents (see
// writeSpecDocuments), concurrently across pages with a bounded pool.
// Each spec's loader call resolves its status and folds the matrix over
// the whole store (seconds each on a large store, 103 specs in this
// repository's own), so a sequential pass multiplied the site's build
// time several times over and pushed the self-hosted spec-align gate past
// Go's default test timeout. The pool changes only wall time: every
// spec's files are its own, written from its own result, so the output
// tree is byte-identical to a sequential pass (the rebuild tests prove
// it). The first error cancels the rest and is returned.
func writeAllSpecDocuments(parent context.Context, outDir, root string, stamp buildStamp, mdl *model.Model, pages []*artifactPage) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	sem := make(chan struct{}, documentWorkers())
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)
schedule:
	for _, p := range pages {
		if specDocumentURL(p.Entry.Ref) == "" {
			continue
		}
		// A slot, or the pass is over (a worker failed, or the caller
		// cancelled): stop scheduling — never spawn a worker that holds
		// no slot, since its release would then block or steal one.
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break schedule
		}
		wg.Add(1)
		go func(p *artifactPage) {
			defer wg.Done()
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			if err := writeSpecDocuments(ctx, outDir, root, stamp, mdl, p); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
					cancel()
				}
				mu.Unlock()
			}
		}(p)
	}
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}
	return parent.Err() // nil unless the caller itself cancelled
}

// documentWorkers is the pool's width: one per CPU, never fewer than one.
func documentWorkers() int {
	if n := runtime.NumCPU(); n > 1 {
		return n
	}
	return 1
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
//
// The loader runs ONCE per spec and its Input is reused for the three
// kinds: nothing the loader assembles varies with the requested kind
// except Input.Kind itself (the kind only selects which sections Build
// renders), so the plan and tasks bytes are exactly what a per-kind load
// would render — TestWriteSpecDocuments_KindsShareOneLoad proves it
// against the per-kind path the CLI and MCP take.
func writeSpecDocuments(ctx context.Context, outDir, root string, stamp buildStamp, mdl *model.Model, p *artifactPage) error {
	if specDocumentURL(p.Entry.Ref) == "" {
		return nil
	}
	name := strings.TrimPrefix(p.Entry.Ref, "spec/")
	base := path.Dir(permalinkOutPath(p.Entry.Ref)) // a/spec/<name>
	res, err := specdocload.Load(ctx, specdocload.Request{Root: root, Name: name, Mode: specdocload.ModeAt, At: stamp.SHA, Kind: specdoc.KindSpec, Model: mdl})
	if err != nil {
		return fmt.Errorf("dex: document for %s: %w", p.Entry.Ref, err)
	}
	var specHTML string
	for _, kind := range []specdoc.Kind{specdoc.KindSpec, specdoc.KindPlan, specdoc.KindTasks} {
		in := res.Input
		in.Kind = kind
		doc, err := specdoc.Build(in)
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
