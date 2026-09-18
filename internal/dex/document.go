package dex

import (
	"context"
	"fmt"
	"html/template"
	"path"
	"runtime"
	"strings"
	"sync"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specdocload"
	"github.com/jyang234/verdi/internal/store"
)

// documentPageDir is the subdirectory, beside a spec's permalink page,
// that serves its Document view: /a/spec/<name>/document/.
const documentPageDir = "document"

// documentSet is the set of spec refs this build writes documents for:
// the specs whose spec.md exists at the build commit. ac-4 pins the
// Document view to "the site's pinned commit", so a spec the index found
// in the working tree but that the build commit does not carry (a draft
// not yet committed, or a build of an older commit) has no document to
// render from — its page is still built from the working tree, exactly
// as before this feature, but it gets no files and no dangling
// "document/" link, rather than aborting the whole site (fix round 1,
// F1: degrade honestly like every other dex writer — cf. noHistoryBanner).
type documentSet map[string]bool

// documentedSpecs resolves the documentSet once per build: one
// `git ls-tree` per spec zone at sha (the zone directory is the store
// accessor's parent, never a literal), matched against each spec page's
// name through the same accessors — never by parsing tree paths.
func documentedSpecs(ctx context.Context, root, sha string, pages []*artifactPage) (documentSet, error) {
	present := make(map[string]bool)
	for _, zone := range []string{store.ZoneActive, store.ZoneArchive} {
		paths, err := gitx.LsTree(ctx, root, sha, path.Dir(store.SpecDirRelPath(zone, "any")))
		if err != nil {
			return nil, fmt.Errorf("dex: listing %s specs at %s: %w", zone, sha, err)
		}
		for _, p := range paths {
			present[p] = true
		}
	}
	docs := make(documentSet)
	for _, p := range pages {
		if !strings.HasPrefix(p.Entry.Ref, "spec/") {
			continue
		}
		name := strings.TrimPrefix(p.Entry.Ref, "spec/")
		if present[store.ActiveSpecRelPath(name)] || present[store.SpecRelPath(store.ZoneArchive, name)] {
			docs[p.Entry.Ref] = true
		}
	}
	return docs, nil
}

// specDocumentURL returns the page-relative URL of ref's Document view
// ("document/", resolving under the spec page's directory-form permalink),
// or "" for a page that gets no document: anything that is not a spec —
// only specs have documents (spec/spec-documents ac-4) — or a spec absent
// at the build commit (documentSet).
func specDocumentURL(ref string, docs documentSet) string {
	if !docs[ref] {
		return ""
	}
	return documentPageDir + "/"
}

// renderSpecDocuments is the pool's per-spec step, a variable so tests can
// stand in a render that panics or cancels (fix round 1, F4/F5).
var renderSpecDocuments = writeSpecDocuments

// writeAllSpecDocuments renders every spec page's documents (see
// writeSpecDocuments), concurrently across pages with a bounded pool.
// Each spec's loader call resolves its status and folds the matrix over
// the whole store (seconds each on a large store, 103 specs in this
// repository's own), so a sequential pass multiplied the site's build
// time several times over and pushed the self-hosted spec-align gate past
// Go's default test timeout. The pool changes only wall time: every
// spec's files are its own, written from its own result, so the output
// tree is byte-identical to a sequential pass (the rebuild tests prove
// it). The first error cancels the rest and is returned; a worker panic
// is folded into that error (naming the spec) so the build still exits 2
// through the normal path instead of killing the process.
func writeAllSpecDocuments(parent context.Context, outDir, root string, stamp buildStamp, mdl *model.Model, pages []*artifactPage, docs documentSet) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	sem := make(chan struct{}, documentWorkers())
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)
	fail := func(err error) {
		mu.Lock()
		if firstErr == nil {
			firstErr = err
			cancel()
		}
		mu.Unlock()
	}
schedule:
	for _, p := range pages {
		if specDocumentURL(p.Entry.Ref, docs) == "" {
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
			defer func() {
				if r := recover(); r != nil {
					fail(fmt.Errorf("dex: document for %s: panic: %v", p.Entry.Ref, r))
				}
			}()
			if ctx.Err() != nil {
				return
			}
			if err := renderSpecDocuments(ctx, outDir, root, stamp, mdl, p); err != nil {
				fail(err)
			}
		}(p)
	}
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}
	return parent.Err() // nil unless the caller itself cancelled
}

// documentWorkers is the pool's width: one per CPU, never fewer than one
// and never more than eight — the loader's per-spec work is git- and
// filesystem-bound, so wider pools stop paying off, and the cap keeps the
// cancellation witness (TestWriteAllSpecDocuments_CallerCancel, which
// schedules 64 pages and expects fewer than all to render) deterministic
// on any host, however many cores it has.
func documentWorkers() int {
	return max(1, min(runtime.NumCPU(), 8))
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
	if !strings.HasPrefix(p.Entry.Ref, "spec/") {
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
		TOC:      documentTOC(extractTOC(specHTML)),
		CopyRef:  p.Entry.Ref + "@" + stamp.SHA,
	})
	if err != nil {
		return err
	}
	return writeFile(outDir, path.Join(base, documentPageDir, "index.html"), page)
}

// documentTOC keeps only the document's section headings (level 2) for
// the side rail: the document's h3s are whole sentences (a decision's
// text, a criterion's text — hundreds of characters), which would turn
// the rail into a wall of prose and defeat its sticky positioning (fix
// round 1, F2). The headings themselves stay in the body, with their ids.
func documentTOC(entries []TOCEntry) []TOCEntry {
	var out []TOCEntry
	for _, e := range entries {
		if e.Level == 2 {
			out = append(out, e)
		}
	}
	return out
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
