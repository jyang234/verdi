package index

import (
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
)

// Backlink is one inverted edge: From names the artifact whose forward
// link produced this entry, and Type is that link type's computed inverse
// (02 §Link taxonomy).
type Backlink struct {
	From string
	Type string
}

// inverseOf is 02 §Link taxonomy's "Inverse (computed)" column. `story`
// has no inverse ("—" in the table: a feature spec -> tracker item edge
// is one-way) and is deliberately absent here.
var inverseOf = map[artifact.LinkType]string{
	artifact.LinkImplements:  "implemented-by",
	artifact.LinkSupersedes:  "superseded-by",
	artifact.LinkVerifies:    "verified-by",
	artifact.LinkDerivedFrom: "source-of",
	artifact.LinkAnnotates:   "annotated-by",
	artifact.LinkDependsOn:   "depended-on-by",
	artifact.LinkImpacts:     "impacted-by",
	artifact.LinkChallenges:  "challenged-by",
	artifact.LinkResolves:    "resolved-by",
	artifact.LinkExempts:     "exempted-by",
}

// buildBacklinks inverts every entry's outgoing links into a target-ref ->
// []Backlink map. A link's target need not itself be an indexed Entry
// (lint, not the index, owns ref resolution — VL-003) — the backlink is
// still recorded, keyed by the literal ref string the link named, so a
// later Get on that ref (once resolvable) sees it.
func buildBacklinks(entries []*Entry) map[string][]Backlink {
	backlinks := make(map[string][]Backlink)
	for _, e := range entries {
		for _, l := range e.Links {
			inv, ok := inverseOf[l.Type]
			if !ok {
				continue
			}
			backlinks[l.Ref] = append(backlinks[l.Ref], Backlink{From: e.Ref, Type: inv})
		}
	}
	for ref, bl := range backlinks {
		bl := bl
		// Deterministic order: by type, then by source ref.
		sort.Slice(bl, func(i, j int) bool {
			if bl[i].Type != bl[j].Type {
				return bl[i].Type < bl[j].Type
			}
			return bl[i].From < bl[j].From
		})
		backlinks[ref] = bl
	}
	return backlinks
}
