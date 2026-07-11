package index

import (
	"regexp"
	"sort"
	"strings"
)

// tokenRe splits on maximal runs of lowercase letters and digits — the
// stdlib-only tokenizer 01 §Scale envelope calls for ("full-text search
// over the corpus is in-memory, stdlib-only").
var tokenRe = regexp.MustCompile(`[a-z0-9]+`)

// tokenize lowercases s and splits it into tokenRe's alphabet.
func tokenize(s string) []string {
	return tokenRe.FindAllString(strings.ToLower(s), -1)
}

// SearchResult is one hit: Ref plus its relevance Score (01 §Scale
// envelope: "simple relevance = hit count").
type SearchResult struct {
	Ref   string
	Score int
}

// indexTokens folds one entry's id + title + body into the inverted
// token index, token -> ref -> hit count.
func (ix *Index) indexTokens(e *Entry) {
	counts := make(map[string]int)
	for _, tok := range tokenize(e.Ref + " " + e.Title + " " + e.Body) {
		counts[tok]++
	}
	for tok, c := range counts {
		if ix.tokens[tok] == nil {
			ix.tokens[tok] = make(map[string]int)
		}
		ix.tokens[tok][e.Ref] = c
	}
}

// Search tokenizes query the same way entries were indexed, sums hit
// counts across the query's distinct tokens per matching ref, and returns
// results ordered by score descending, then by ref ascending for
// determinism (01 §Scale envelope). A query with no recognizable tokens,
// or one that matches nothing, returns nil.
func (ix *Index) Search(query string) []SearchResult {
	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return nil
	}

	seen := make(map[string]bool, len(queryTokens))
	scores := make(map[string]int)
	for _, qt := range queryTokens {
		if seen[qt] {
			continue
		}
		seen[qt] = true
		for ref, count := range ix.tokens[qt] {
			scores[ref] += count
		}
	}
	if len(scores) == 0 {
		return nil
	}

	results := make([]SearchResult, 0, len(scores))
	for ref, score := range scores {
		results = append(results, SearchResult{Ref: ref, Score: score})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Ref < results[j].Ref
	})
	return results
}

// AllTokens returns every token in the index's vocabulary, sorted — the
// full term list a caller emitting a build-time JSON inverted index
// (dex build's search-index.json, 05 §Verdi-dex mechanics) walks to
// serialize the whole index rather than resolve a single query.
func (ix *Index) AllTokens() []string {
	tokens := make([]string, 0, len(ix.tokens))
	for tok := range ix.tokens {
		tokens = append(tokens, tok)
	}
	sort.Strings(tokens)
	return tokens
}

// Postings returns token's raw single-token posting list — (ref, hit
// count) pairs, sorted by ref for determinism — with no cross-token
// scoring (unlike Search, which ORs and sums across every token in a
// query). A token absent from the vocabulary returns nil.
func (ix *Index) Postings(token string) []SearchResult {
	refs := ix.tokens[token]
	if len(refs) == 0 {
		return nil
	}
	results := make([]SearchResult, 0, len(refs))
	for ref, count := range refs {
		results = append(results, SearchResult{Ref: ref, Score: count})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Ref < results[j].Ref })
	return results
}
