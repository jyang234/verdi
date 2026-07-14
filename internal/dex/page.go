package dex

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/artifactview"
	"github.com/jyang234/verdi/internal/index"
)

// meta and decodeMeta delegate to internal/artifactview — see that
// package's doc comment for why this moved out of dex (CLAUDE.md: shared
// code lives in a shared internal/ package once a second consumer,
// internal/workbench, needs it too). Kept as a local type alias and
// package-level var, not a straight rename at every call site, so the
// rest of this package's files (artifactpage.go, serviceaxis.go, ...) are
// untouched.
type meta = artifactview.Meta

var decodeMeta = artifactview.DecodeMeta

// artifactPage pairs one committed-zone index.Entry with its decoded meta —
// everything dex's page anatomy (breadcrumb, title/badge, temporal banner,
// metadata card, body, connections, TOC) needs for one page.
type artifactPage struct {
	Entry   *index.Entry
	Meta    meta
	RelPath string // root-relative, slash-separated source path
}

// loadArtifactPages decodes full metadata for every committed-zone entry
// (i.e. every indexed entry except Kind == "external", which carries no
// frontmatter of its own) in ix, sorted by ref for deterministic
// iteration.
func loadArtifactPages(root string, ix *index.Index) ([]*artifactPage, error) {
	var pages []*artifactPage
	for _, e := range ix.All() {
		if e.Kind == "external" {
			continue
		}
		data, err := os.ReadFile(e.Path)
		if err != nil {
			return nil, fmt.Errorf("dex: reading %s: %w", e.Path, err)
		}
		fm, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			return nil, fmt.Errorf("dex: %s: %w", e.Path, err)
		}
		m, err := decodeMeta(e.Kind, fm)
		if err != nil {
			return nil, fmt.Errorf("dex: %s: %w", e.Path, err)
		}
		rel, err := filepath.Rel(root, e.Path)
		if err != nil {
			return nil, fmt.Errorf("dex: %s: %w", e.Path, err)
		}
		pages = append(pages, &artifactPage{Entry: e, Meta: m, RelPath: filepath.ToSlash(rel)})
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Entry.Ref < pages[j].Entry.Ref })
	return pages, nil
}
