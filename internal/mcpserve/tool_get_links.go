package mcpserve

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/OWNER/verdi/internal/artifact"
)

// linkItem/backlinkItem are get_links's result rows.
type linkItem struct {
	Type string `json:"type"`
	Ref  string `json:"ref"`
	Note string `json:"note,omitempty"`
}

type backlinkItem struct {
	From string `json:"from"`
	Type string `json:"type"`
}

// GetLinks implements the get_links tool: an artifact's typed outgoing
// links plus computed backlinks (internal/index's inverted edge table).
func (b *Backend) GetLinks(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	var args struct {
		Ref string `json:"ref"`
	}
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return toolError("get_links: malformed arguments: " + err.Error())
	}
	if args.Ref == "" {
		return toolError("get_links: ref is required")
	}

	ref, err := artifact.ParseRef(args.Ref)
	if err != nil {
		return toolError("get_links: " + err.Error())
	}
	unpinned := artifact.Ref{Kind: ref.Kind, Name: ref.Name}

	ix, err := b.buildIndex()
	if err != nil {
		return toolError("get_links: " + err.Error())
	}

	entry, ok := ix.Get(unpinned.String())
	if !ok {
		return toolError(fmt.Sprintf("get_links: %q not found in the index", unpinned))
	}

	links := make([]linkItem, 0, len(entry.Links))
	for _, l := range entry.Links {
		links = append(links, linkItem{Type: string(l.Type), Ref: l.Ref, Note: l.Note})
	}

	backs := ix.Backlinks(unpinned.String())
	backlinks := make([]backlinkItem, 0, len(backs))
	for _, bl := range backs {
		backlinks = append(backlinks, backlinkItem{From: bl.From, Type: bl.Type})
	}

	return toolJSON(map[string]any{
		"ref":       entry.Ref,
		"links":     links,
		"backlinks": backlinks,
	})
}
