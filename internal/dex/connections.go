package dex

import (
	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/index"
)

// linkConnections projects an entry's outgoing typed links into the
// connections panel's shape, resolving each to its permalink URL when dex
// has a page for it (05 §Verdi-dex page anatomy: "connections panel
// (typed links plus computed backlinks)").
func linkConnections(links []artifact.Link, known map[string]bool) []connection {
	var conns []connection
	for _, l := range links {
		url, _ := resolvableLinkURL(l.Ref, known)
		conns = append(conns, connection{Type: string(l.Type), Ref: l.Ref, URL: url, Note: l.Note})
	}
	return conns
}

// backlinkConnections projects ref's computed backlinks (internal/index's
// inversion of every other entry's outgoing links) into the connections
// panel's shape.
func backlinkConnections(ix *index.Index, ref string, known map[string]bool) []connection {
	var conns []connection
	for _, bl := range ix.Backlinks(ref) {
		url, _ := resolvableLinkURL(bl.From, known)
		conns = append(conns, connection{Type: bl.Type, Ref: bl.From, URL: url})
	}
	return conns
}

// allConnections merges outgoing links and backlinks into one panel, links
// first (in frontmatter order), then backlinks (already type-then-ref
// sorted by internal/index).
func allConnections(ix *index.Index, ref string, links []artifact.Link, known map[string]bool) []connection {
	conns := linkConnections(links, known)
	conns = append(conns, backlinkConnections(ix, ref, known)...)
	return conns
}
