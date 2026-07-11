package mcpserve

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/boardio"
	"github.com/OWNER/verdi/internal/store"
)

// addAnnotationArgs is add_annotation's flattened argument shape (tooldefs.go's
// inputSchema): a target and/or a board placement, plus the record's own
// fields. Flattened rather than nested request objects because MCP tool
// arguments are a flat JSON object per call in every other tool here, and
// json.RawMessage sub-decoding into artifact.Target/BoardAnchor directly
// would let a caller smuggle extra fields (line, e.g.) with no schema
// validation at the tool boundary.
type addAnnotationArgs struct {
	Author        string  `json:"author"`
	TargetRef     string  `json:"target_ref"`
	TargetHeading string  `json:"target_heading"`
	TargetQuote   string  `json:"target_quote"`
	BoardStory    string  `json:"board_story"`
	BoardX        float64 `json:"board_x"`
	BoardY        float64 `json:"board_y"`
	Type          string  `json:"type"`
	Body          string  `json:"body"`
}

// AddAnnotation implements the add_annotation tool — THE ONLY WRITE: it
// appends one verdi.annotation/v1 JSONL record (02 §Record schemas) to
// data/mutable/annotations/<kind>--<name>.jsonl (a target present) or
// data/mutable/annotations/board--<story-slug>.jsonl (board-only,
// backend.go's annotationFileForBoard). id is a fresh a-<ULID> (I-11);
// ts is the current time in RFC3339; status is always "open" (a freshly
// created annotation has nothing else to be). A target, when given, must
// name a pinned ref that actually resolves (index.GetPinned) — an
// unresolvable target is rejected before anything is written.
func (b *Backend) AddAnnotation(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	var args addAnnotationArgs
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return toolError("add_annotation: malformed arguments: " + err.Error())
	}
	if args.Author == "" {
		return toolError("add_annotation: author is required")
	}
	if args.Body == "" {
		return toolError("add_annotation: body is required")
	}
	if args.TargetRef == "" && args.BoardStory == "" {
		return toolError("add_annotation: at least one of target_ref or board_story is required")
	}

	a := &artifact.Annotation{
		TS:     time.Now().UTC().Format(time.RFC3339),
		Author: args.Author,
		Type:   artifact.AnnotationType(args.Type),
		Body:   args.Body,
		Status: artifact.AnnotationOpen,
	}

	var targetRef artifact.Ref
	if args.TargetRef != "" {
		ref, err := artifact.ParsePinnedRef(args.TargetRef)
		if err != nil {
			return toolError("add_annotation: target_ref: " + err.Error())
		}
		targetRef = ref
		a.Target = &artifact.Target{
			Ref:      args.TargetRef,
			Selector: artifact.Selector{Heading: args.TargetHeading, Quote: args.TargetQuote},
		}
	}
	if args.BoardStory != "" {
		a.Board = &artifact.BoardAnchor{Story: args.BoardStory, X: args.BoardX, Y: args.BoardY}
	}

	id, err := artifact.NewAnnotationID()
	if err != nil {
		return toolError("add_annotation: generating id: " + err.Error())
	}
	a.ID = id

	if err := a.Validate(); err != nil {
		return toolError("add_annotation: " + err.Error())
	}

	// Reject an unresolvable target BEFORE taking the write lock or
	// touching disk (05's tool table: "an unresolvable target is
	// rejected" — this is stronger than artifact.Annotation.Validate's
	// own check, which only validates the ref's SHAPE, not that it
	// actually resolves).
	if a.Target != nil {
		ix, ierr := b.buildIndex()
		if ierr != nil {
			return toolError("add_annotation: " + ierr.Error())
		}
		if _, gerr := ix.GetPinned(ctx, targetRef); gerr != nil {
			return toolError(fmt.Sprintf("add_annotation: target_ref %q does not resolve: %v", args.TargetRef, gerr))
		}
	}

	var fileName string
	if a.Target != nil {
		fileName = annotationFileForTarget(artifact.Ref{Kind: targetRef.Kind, Name: targetRef.Name})
	} else {
		fileName = annotationFileForBoard(store.RefSlug(a.Board.Story))
	}

	b.writeMu.Lock()
	defer b.writeMu.Unlock()

	if err := b.appendAnnotation(fileName, a); err != nil {
		return toolError("add_annotation: " + err.Error())
	}

	return toolJSON(map[string]any{"id": a.ID, "file": fileName})
}

// appendAnnotation appends a's JSON encoding as one line to
// data/mutable/annotations/fileName (creating the directory if needed).
// Delegates to internal/boardio.AppendAnnotation.
func (b *Backend) appendAnnotation(fileName string, a *artifact.Annotation) error {
	return boardio.AppendAnnotation(b.annotationsDir(), fileName, a)
}
