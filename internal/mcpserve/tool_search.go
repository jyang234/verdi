package mcpserve

import (
	"context"
	"encoding/json"
)

// searchResultItem is one search_artifacts result row.
type searchResultItem struct {
	Ref   string `json:"ref"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
	Score int    `json:"score"`
}

// SearchArtifacts implements the search_artifacts tool: full-text search
// over the corpus via internal/index.Search.
func (b *Backend) SearchArtifacts(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return toolError("search_artifacts: malformed arguments: " + err.Error())
	}
	if args.Query == "" {
		return toolError("search_artifacts: query is required")
	}

	ix, err := b.buildIndex()
	if err != nil {
		return toolError("search_artifacts: " + err.Error())
	}

	hits := ix.Search(args.Query)
	results := make([]searchResultItem, 0, len(hits))
	for _, h := range hits {
		e, ok := ix.Get(h.Ref)
		if !ok {
			continue // backlinks can name unresolved refs; Search only ever returns indexed refs, but stay defensive
		}
		results = append(results, searchResultItem{Ref: e.Ref, Title: e.Title, Kind: e.Kind, Score: h.Score})
	}
	return toolJSON(map[string]any{"query": args.Query, "results": results})
}
