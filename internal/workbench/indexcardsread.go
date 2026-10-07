// The index cards' one per-entry read (spec/index-v2): each default-branch
// entry's spec frontmatter from the serving working tree, read once per
// render and handed to the pure card projection (indexcards.go). It is
// presentation enrichment, never the entry's truth (spec/directory-home
// dc-2: the index is computed from refs) — except that the call to
// action's criteria and stubs come from it, and its failure is disclosed
// there as unproven coverage (SI-366 (10)).
package workbench

import (
	"fmt"
	"os"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/artifactview"
	"github.com/jyang234/verdi/internal/refindex"
	"github.com/jyang234/verdi/internal/store"
)

// specTreeMeta is a default-branch entry's working-tree read.
type specTreeMeta struct {
	title string
	class artifact.SpecClass
	story string
	// boardServable is whether the ACTIVE-zone file exists, which is what
	// makes /board/spec/<name> servable.
	boardServable bool
	// criteria are the declared acceptance-criterion ids, in declared
	// order; stubs the declared stubs.
	criteria []string
	stubs    []artifact.Stub
	// unreadable is why nothing decoded — no file in either zone, or a
	// frontmatter that does not decode — and empty when it decoded.
	unreadable string
}

// readSpecTreeMeta reads name's spec frontmatter from the serving working
// tree, active zone first, then archive. Every failure degrades to zero
// trim and names itself in unreadable; boardServable stays true when the
// active-zone file exists but does not decode.
func readSpecTreeMeta(root, name string) specTreeMeta {
	var m specTreeMeta
	data, err := os.ReadFile(store.ActiveSpecPath(root, name))
	if err == nil {
		m.boardServable = true
	} else if data, err = os.ReadFile(store.ArchiveSpecPath(root, name)); err != nil {
		m.unreadable = fmt.Sprintf("spec/%s has no readable working-tree file in the active or archive zone", name)
		return m
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		m.unreadable = fmt.Sprintf("spec/%s's working-tree file does not split: %v", name, err)
		return m
	}
	meta, err := artifactview.DecodeMeta("spec", fm)
	if err != nil {
		m.unreadable = fmt.Sprintf("spec/%s's working-tree file does not decode: %v", name, err)
		return m
	}
	m.title, m.class, m.story = meta.Base.Title, meta.Class, meta.Story
	for _, ac := range meta.AcceptanceCriteria {
		m.criteria = append(m.criteria, ac.ID)
	}
	m.stubs = meta.Stubs
	return m
}

// homeCards projects every entry onto its card facts, index-aligned with
// entries: each default-branch entry's working tree is read once here,
// and every card shares cc, the render's one reading of everything else.
func homeCards(root string, entries []refindex.Entry, cc cardContext) []cardFacts {
	cards := make([]cardFacts, len(entries))
	for i, e := range entries {
		var tree specTreeMeta
		if e.Source == refindex.SourceDefault {
			tree = readSpecTreeMeta(root, strings.TrimPrefix(e.Ref, "spec/"))
		}
		cards[i] = projectCard(e, tree, cc)
	}
	return cards
}
