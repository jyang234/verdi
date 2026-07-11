package mcpserve

import (
	"context"
	"encoding/json"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/boardio"
)

// ListTasks implements the list_tasks tool: every OPEN agent-task
// annotation across the whole mutable zone (05 §Workbench dispatch's
// "lane 1 ... a /tasks skill lists open agent-task annotations via MCP").
// Drift is included for targeted tasks, exactly as list_annotations
// reports it, since an agent picking up a task benefits from knowing
// whether its anchor still holds.
func (b *Backend) ListTasks(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	all, err := boardio.ReadAllAnnotations(b.annotationsDir())
	if err != nil {
		return toolError("list_tasks: " + err.Error())
	}

	ix, ierr := b.buildIndex()
	bodyOf := func(ref artifact.Ref) string {
		if ierr != nil {
			return ""
		}
		if e, ok := ix.Get(ref.String()); ok {
			return e.Body
		}
		return ""
	}

	items := make([]annotationItem, 0)
	for _, a := range all {
		if a.Type != artifact.AnnotationAgentTask || a.Status != artifact.AnnotationOpen {
			continue
		}
		item := annotationItem{ID: a.ID, TS: a.TS, Author: a.Author, Type: string(a.Type), Body: a.Body, Status: string(a.Status), Board: a.Board}
		if a.Target != nil {
			if targetRef, terr := artifact.ParsePinnedRef(a.Target.Ref); terr == nil {
				unpinned := artifact.Ref{Kind: targetRef.Kind, Name: targetRef.Name}
				item.Target = &targetItem{Ref: a.Target.Ref, Selector: a.Target.Selector, Drift: ComputeDrift(a.Target.Selector, bodyOf(unpinned))}
			}
		}
		items = append(items, item)
	}
	return toolJSON(map[string]any{"tasks": items})
}
