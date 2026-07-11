package dex

import (
	"fmt"
	"html/template"

	"github.com/OWNER/verdi/internal/canonjson"
	"github.com/OWNER/verdi/internal/index"
)

// tokenPosting is one entry of search-index.json's per-token postings
// list.
type tokenPosting struct {
	Ref   string `json:"ref"`
	Score int    `json:"score"`
}

// refInfo is search-index.json's per-ref metadata: just enough for
// search.js to render a human-readable result without a second round
// trip.
type refInfo struct {
	Title string `json:"title"`
	Kind  string `json:"kind"`
}

// searchIndexDoc is search-index.json's top-level shape (05 §Verdi-dex
// mechanics: "search over a build-emitted JSON inverted index with a small
// vanilla lookup").
type searchIndexDoc struct {
	Tokens map[string][]tokenPosting `json:"tokens"`
	Refs   map[string]refInfo        `json:"refs"`
}

// buildSearchIndexDoc projects ix's full inverted index and every entry's
// (title, kind) into searchIndexDoc.
func buildSearchIndexDoc(ix *index.Index) searchIndexDoc {
	doc := searchIndexDoc{
		Tokens: make(map[string][]tokenPosting),
		Refs:   make(map[string]refInfo),
	}
	for _, tok := range ix.AllTokens() {
		postings := ix.Postings(tok)
		out := make([]tokenPosting, 0, len(postings))
		for _, p := range postings {
			out = append(out, tokenPosting{Ref: p.Ref, Score: p.Score})
		}
		doc.Tokens[tok] = out
	}
	for _, e := range ix.All() {
		doc.Refs[e.Ref] = refInfo{Title: e.Title, Kind: e.Kind}
	}
	return doc
}

// writeSearchIndex emits search-index.json (canonical JSON: sorted keys,
// deterministic regardless of Go map iteration order) and the search page
// search.js wires up via #search-box / #search-results.
func writeSearchIndex(outDir string, stamp buildStamp, ix *index.Index) error {
	doc := buildSearchIndexDoc(ix)
	data, err := canonjson.Marshal(doc)
	if err != nil {
		return fmt.Errorf("dex: marshaling search-index.json: %w", err)
	}
	if err := writeFile(outDir, "search-index.json", data); err != nil {
		return err
	}

	page := pageData{
		Title:      "Search",
		Breadcrumb: []breadcrumbEntry{{Label: "Home", URL: "/"}, {Label: "Search", URL: ""}},
		Banner:     livingGatedBanner(stamp),
		BodyHTML:   template.HTML(`<input id="search-box" type="search" placeholder="Search the store..." autofocus><ul id="search-results"></ul>`),
	}
	out, err := renderPage(page)
	if err != nil {
		return err
	}
	return writeFile(outDir, "search/index.html", out)
}
