package lint

import (
	"github.com/jyang234/verdi/internal/artifact"
)

// headingAnchors extracts every ATX ("# ", "## ", ...) heading in body and
// returns the set of GitHub-flavored-markdown-style anchor slugs those
// headings resolve to — VL-014's `where: "#slug"` resolution target. This
// is the same exact-match anchor-slug algorithm 02 §Object model's general
// anchor-resolution rule uses (V1-P1); the implementation now lives once,
// in internal/artifact (shared per CLAUDE.md: "anything used by two or
// more packages lives in a shared internal/ package"), and this package
// delegates rather than keeping its own copy.
func headingAnchors(body string) map[string]bool {
	return artifact.HeadingAnchors([]byte(body))
}

// slugify delegates to artifact.SlugifyHeading (see headingAnchors' doc
// comment).
func slugify(text string) string {
	return artifact.SlugifyHeading(text)
}

// resolveAnchor reports whether where (a "#slug" reference) resolves to a
// heading in anchors.
func resolveAnchor(anchors map[string]bool, where string) bool {
	return artifact.ResolveAnchor(anchors, where)
}
