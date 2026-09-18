package specdoc

import "github.com/jyang234/verdi/internal/render"

// RenderHTML renders the canonical Markdown through the store's own
// Markdown engine (internal/render, goldmark) and returns the fragment.
// Consumers wrap it in their shell; the bytes inside are the same reading
// of the same objects every consumer shows (spec/spec-documents ac-6).
func RenderHTML(doc Document) (string, error) {
	return render.RenderMarkdown(RenderMarkdown(doc))
}
