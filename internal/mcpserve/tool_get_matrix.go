package mcpserve

import (
	"context"
	"encoding/json"

	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

// acRow is one get_matrix result row.
type acRow struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

// matrixResult is get_matrix's result shape, mirroring
// evidence.StoryResult (cmd/verdi/matrix.go's own printMatrix reads the
// identical fields).
type matrixResult struct {
	Story    string  `json:"story"`
	SpecRef  string  `json:"spec_ref"`
	Preview  bool    `json:"preview"`
	ACs      []acRow `json:"acs"`
	Violated bool    `json:"violated"`
	Eligible bool    `json:"eligible"`
}

// GetMatrix implements the get_matrix tool: the same fold `verdi matrix`
// computes (internal/evidence.Fold), reached via the same I-30 resolution
// policy (internal/storyresolve, shared with cmd/verdi/matrix.go).
func (b *Backend) GetMatrix(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	var args struct {
		Story   string `json:"story"`
		Preview bool   `json:"preview"`
	}
	if err := strictUnmarshal(argsRaw, &args); err != nil {
		return toolError("get_matrix: malformed arguments: " + err.Error())
	}
	if args.Story == "" {
		// vocab:identity — MCP tool ARGUMENT name (wire schema)
		return toolError("get_matrix: story is required")
	}

	spec, err := storyresolve.Resolve(b.Root, args.Story)
	if err != nil {
		return toolError("get_matrix: " + err.Error())
	}

	commit, err := gitx.RevParse(ctx, b.Root, "HEAD")
	if err != nil {
		return toolError("get_matrix: resolving HEAD: " + err.Error())
	}

	derivedRoot := store.DerivedSpecDir(b.Root, store.RefSlug(spec.ID))
	records, err := evidence.LoadRecords(ctx, b.Root, derivedRoot, commit)
	if err != nil {
		return toolError("get_matrix: " + err.Error())
	}

	slug := store.RefSlug(spec.Story)
	result, err := evidence.Fold(evidence.Input{
		Spec:      spec,
		Records:   records,
		Preview:   args.Preview,
		StoreRoot: b.Root,
		StorySlug: slug,
	})
	if err != nil {
		return toolError("get_matrix: " + err.Error())
	}

	out := matrixResult{Story: result.Story, SpecRef: result.SpecRef, Preview: args.Preview, Violated: result.Violated, Eligible: result.Eligible}
	for _, r := range result.ACs {
		out.ACs = append(out.ACs, acRow{ID: r.ID, Text: r.Text, Status: string(r.Status), Summary: r.Summary})
	}
	if out.ACs == nil {
		out.ACs = []acRow{}
	}
	return toolJSON(out)
}
