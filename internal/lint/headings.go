package lint

import (
	"strings"
)

// headingAnchors extracts every ATX ("# ", "## ", ...) heading in body and
// returns the set of GitHub-flavored-markdown-style anchor slugs those
// headings resolve to — VL-014's `where: "#slug"` resolution target.
func headingAnchors(body string) map[string]bool {
	anchors := make(map[string]bool)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimLeft(line, " \t")
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		i := 0
		for i < len(trimmed) && trimmed[i] == '#' {
			i++
		}
		if i == 0 || i > 6 {
			continue
		}
		rest := trimmed[i:]
		if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
			continue // e.g. "#foo" is not a heading
		}
		text := strings.TrimSpace(rest)
		if text == "" {
			continue
		}
		anchors[slugify(text)] = true
	}
	return anchors
}

// slugify computes a heading's anchor slug: lowercase, spaces and hyphens
// preserved as hyphens, every other rune dropped, matching the common
// GitHub-flavored-markdown heading-anchor algorithm closely enough for
// this store's own headings (it does not implement GFM's duplicate-heading
// "-1" disambiguation suffix, which the corpus and self-hosted specs never
// need — no two headings in the same document share text).
func slugify(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-':
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// resolveAnchor reports whether where (a "#slug" reference) resolves to a
// heading in anchors.
func resolveAnchor(anchors map[string]bool, where string) bool {
	return anchors[strings.TrimPrefix(where, "#")]
}
