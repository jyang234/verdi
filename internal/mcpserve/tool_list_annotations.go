package mcpserve

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/boardio"
)

// annotationItem is one list_annotations/list_tasks result row.
type annotationItem struct {
	ID     string                `json:"id"`
	TS     string                `json:"ts"`
	Author string                `json:"author"`
	Type   string                `json:"type"`
	Body   string                `json:"body"`
	Status string                `json:"status"`
	Target *targetItem           `json:"target,omitempty"`
	Board  *artifact.BoardAnchor `json:"board,omitempty"`
	// ObjectID is set only on a mirrored review-sticky item (Type
	// "review", review.go, V1-P7): the spec object id its
	// [vd:<object-id>] token resolved to (02 §Record schemas'
	// comment-token grammar) — "tokens resolved to object ids", 05 §MCP
	// server's list_annotations row. Omitted for every other annotation
	// type, which anchor via Target's selector instead.
	ObjectID string `json:"object_id,omitempty"`
}

// targetItem mirrors artifact.Target but adds the computed Drift field
// (I-17) — never persisted, always recomputed against the current
// working tree at call time.
type targetItem struct {
	Ref      string            `json:"ref"`
	Selector artifact.Selector `json:"selector"`
	Drift    DriftStatus       `json:"drift"`
}

// ListAnnotations implements the list_annotations tool: every annotation
// targeting ref, each with its I-17 three-valued drift status.
func (b *Backend) ListAnnotations(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	var args struct {
		Ref string `json:"ref"`
	}
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return toolError("list_annotations: malformed arguments: " + err.Error())
	}
	if args.Ref == "" {
		return toolError("list_annotations: ref is required")
	}

	ref, err := artifact.ParseRef(args.Ref)
	if err != nil {
		return toolError("list_annotations: " + err.Error())
	}
	unpinned := artifact.Ref{Kind: ref.Kind, Name: ref.Name}

	file := filepath.Join(b.annotationsDir(), annotationFileForTarget(unpinned))
	records, err := boardio.ReadAnnotationFile(file)
	if err != nil {
		return toolError("list_annotations: " + err.Error())
	}

	// Current working-tree body of the target artifact, for drift.
	// A target artifact that no longer resolves at all yields "" —
	// ComputeDrift correctly reports DriftGone for every selector then.
	currentBody := ""
	if ix, ierr := b.buildIndex(); ierr == nil {
		if e, ok := ix.Get(unpinned.String()); ok {
			currentBody = e.Body
		}
	}

	items := make([]annotationItem, 0, len(records))
	for _, a := range records {
		if a.Target == nil {
			continue // this file holds only targeted annotations (annotationFileForTarget's contract)
		}
		targetRef, terr := artifact.ParsePinnedRef(a.Target.Ref)
		if terr != nil || targetRef.Kind != unpinned.Kind || targetRef.Name != unpinned.Name {
			continue // defensive: skip a record that doesn't actually name this target
		}
		items = append(items, annotationItem{
			ID: a.ID, TS: a.TS, Author: a.Author, Type: string(a.Type), Body: a.Body, Status: string(a.Status),
			Target: &targetItem{Ref: a.Target.Ref, Selector: a.Target.Selector, Drift: ComputeDrift(a.Target.Selector, currentBody)},
			Board:  a.Board,
		})
	}

	// Review population (V1-P7, review.go): mirrored, read-only, live
	// forge review comments whose token resolves to one of unpinned's OWN
	// declared objects — merged into the same result set the local
	// mutable-zone streams populate above, per 05 §MCP server's
	// list_annotations row ("covers the R4 annotation types... and
	// (mirrored) review stickies"). A forge error here IS surfaced (never
	// silently swallowed — constitution 2/10); every "nothing to mirror"
	// case (no forge, not a design branch, no open MR, non-spec target)
	// returns (nil, nil) from reviewMirroredAnnotations itself.
	reviewItems, err := b.reviewMirroredAnnotations(ctx, unpinned)
	if err != nil {
		return toolError("list_annotations: " + err.Error())
	}
	items = append(items, reviewItems...)

	return toolJSON(map[string]any{"ref": unpinned.String(), "annotations": items})
}
