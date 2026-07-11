package mcpserve

import (
	"context"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/boardio"
)

func TestSearchArtifacts_Happy(t *testing.T) {
	b, _, _ := newTestBackend(t)
	result := b.SearchArtifacts(context.Background(), mustArgs(t, map[string]any{"query": "outbox"}))
	var out struct {
		Query   string             `json:"query"`
		Results []searchResultItem `json:"results"`
	}
	toolResultJSON(t, result, &out)
	found := false
	for _, r := range out.Results {
		if r.Ref == "adr/0001-outbox" {
			found = true
		}
	}
	if !found {
		t.Fatalf("search_artifacts(outbox) did not find adr/0001-outbox: %+v", out.Results)
	}
}

func TestSearchArtifacts_Negative(t *testing.T) {
	b, _, _ := newTestBackend(t)

	t.Run("missing query", func(t *testing.T) {
		result := b.SearchArtifacts(context.Background(), mustArgs(t, map[string]any{}))
		if !isToolError(result) {
			t.Fatal("search_artifacts(no query): want isError, got success")
		}
	})

	t.Run("no matches", func(t *testing.T) {
		result := b.SearchArtifacts(context.Background(), mustArgs(t, map[string]any{"query": "zzz-nonexistent-token"}))
		var out struct {
			Results []searchResultItem `json:"results"`
		}
		toolResultJSON(t, result, &out)
		if len(out.Results) != 0 {
			t.Fatalf("expected no results, got %+v", out.Results)
		}
	})
}

func TestGetArtifact_Happy(t *testing.T) {
	b, _, adrCommit := newTestBackend(t)
	ctx := context.Background()

	t.Run("unpinned resolves current working tree", func(t *testing.T) {
		result := b.GetArtifact(ctx, mustArgs(t, map[string]any{"ref": "adr/0001-outbox"}))
		var out artifactResult
		toolResultJSON(t, result, &out)
		if out.Kind != "adr" || !strings.Contains(out.Body, "outbox pattern") {
			t.Fatalf("unexpected result: %+v", out)
		}
		if !strings.Contains(out.Frontmatter, "id: adr/0001-outbox") {
			t.Fatalf("frontmatter missing id: %q", out.Frontmatter)
		}
	})

	t.Run("pinned resolves the historical commit", func(t *testing.T) {
		result := b.GetArtifact(ctx, mustArgs(t, map[string]any{"ref": "adr/0001-outbox@" + adrCommit}))
		var out artifactResult
		toolResultJSON(t, result, &out)
		if !strings.Contains(out.Body, "outbox pattern") {
			t.Fatalf("unexpected pinned result: %+v", out)
		}
	})
}

func TestGetArtifact_Negative(t *testing.T) {
	b, _, _ := newTestBackend(t)
	ctx := context.Background()

	t.Run("malformed ref", func(t *testing.T) {
		result := b.GetArtifact(ctx, mustArgs(t, map[string]any{"ref": "not-a-ref"}))
		if !isToolError(result) {
			t.Fatal("get_artifact(malformed ref): want isError")
		}
	})

	t.Run("unknown artifact", func(t *testing.T) {
		result := b.GetArtifact(ctx, mustArgs(t, map[string]any{"ref": "adr/does-not-exist"}))
		if !isToolError(result) {
			t.Fatal("get_artifact(unknown): want isError")
		}
	})
}

func TestGetLinks_Happy(t *testing.T) {
	b, _, _ := newTestBackend(t)
	ctx := context.Background()

	result := b.GetLinks(ctx, mustArgs(t, map[string]any{"ref": "spec/widget-retry"}))
	var out struct {
		Links     []linkItem     `json:"links"`
		Backlinks []backlinkItem `json:"backlinks"`
	}
	toolResultJSON(t, result, &out)
	if len(out.Links) != 1 || out.Links[0].Ref != "adr/0001-outbox" {
		t.Fatalf("expected one implements link to adr/0001-outbox, got %+v", out.Links)
	}

	// The ADR's computed backlink: spec/widget-retry implements it.
	back := b.GetLinks(ctx, mustArgs(t, map[string]any{"ref": "adr/0001-outbox"}))
	var backOut struct {
		Backlinks []backlinkItem `json:"backlinks"`
	}
	toolResultJSON(t, back, &backOut)
	found := false
	for _, bl := range backOut.Backlinks {
		if bl.From == "spec/widget-retry" && bl.Type == "implemented-by" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a computed implemented-by backlink from spec/widget-retry, got %+v", backOut.Backlinks)
	}
}

func TestGetLinks_Negative(t *testing.T) {
	b, _, _ := newTestBackend(t)
	result := b.GetLinks(context.Background(), mustArgs(t, map[string]any{"ref": "adr/does-not-exist"}))
	if !isToolError(result) {
		t.Fatal("get_links(unknown ref): want isError")
	}
}

func TestGetMatrix_Happy(t *testing.T) {
	b, _, _ := newTestBackend(t)
	result := b.GetMatrix(context.Background(), mustArgs(t, map[string]any{"story": "jira:LOAN-1482"}))
	var out matrixResult
	toolResultJSON(t, result, &out)
	if out.SpecRef != "spec/widget-retry" {
		t.Fatalf("SpecRef = %q, want spec/widget-retry", out.SpecRef)
	}
	if len(out.ACs) != 1 || out.ACs[0].ID != "ac-1" {
		t.Fatalf("ACs = %+v, want one ac-1", out.ACs)
	}

	// Also resolvable by spec ref, per I-30's two accepted forms.
	bySpec := b.GetMatrix(context.Background(), mustArgs(t, map[string]any{"story": "spec/widget-retry"}))
	var bySpecOut matrixResult
	toolResultJSON(t, bySpec, &bySpecOut)
	if bySpecOut.SpecRef != "spec/widget-retry" {
		t.Fatalf("get_matrix(spec ref) SpecRef = %q, want spec/widget-retry", bySpecOut.SpecRef)
	}
}

func TestGetMatrix_Negative(t *testing.T) {
	b, _, _ := newTestBackend(t)

	t.Run("missing story", func(t *testing.T) {
		result := b.GetMatrix(context.Background(), mustArgs(t, map[string]any{}))
		if !isToolError(result) {
			t.Fatal("get_matrix(no story): want isError")
		}
	})

	t.Run("bare tracker key rejected per I-30", func(t *testing.T) {
		result := b.GetMatrix(context.Background(), mustArgs(t, map[string]any{"story": "LOAN-1482"}))
		if !isToolError(result) {
			t.Fatal("get_matrix(bare key): want isError")
		}
	})
}

func TestGetContextBundle_Happy(t *testing.T) {
	b, _, adrCommit := newTestBackend(t)
	ctx := context.Background()

	t.Run("explicit refs", func(t *testing.T) {
		result := b.GetContextBundle(ctx, mustArgs(t, map[string]any{"refs": []string{"adr/0001-outbox@" + adrCommit}}))
		var out struct {
			Items []contextItem `json:"items"`
		}
		toolResultJSON(t, result, &out)
		if len(out.Items) != 1 || out.Items[0].Ref != "adr/0001-outbox" {
			t.Fatalf("unexpected items: %+v", out.Items)
		}
	})

	t.Run("from a spec's context: field", func(t *testing.T) {
		result := b.GetContextBundle(ctx, mustArgs(t, map[string]any{"spec": "spec/widget-retry"}))
		var out struct {
			Items []contextItem `json:"items"`
		}
		toolResultJSON(t, result, &out)
		if len(out.Items) != 1 || out.Items[0].Ref != "adr/0001-outbox" {
			t.Fatalf("unexpected items resolved from spec context: %+v", out.Items)
		}
	})
}

func TestGetContextBundle_Negative(t *testing.T) {
	b, _, _ := newTestBackend(t)
	ctx := context.Background()

	t.Run("neither refs nor spec", func(t *testing.T) {
		result := b.GetContextBundle(ctx, mustArgs(t, map[string]any{}))
		if !isToolError(result) {
			t.Fatal("get_context_bundle(neither): want isError")
		}
	})

	t.Run("both refs and spec", func(t *testing.T) {
		result := b.GetContextBundle(ctx, mustArgs(t, map[string]any{"refs": []string{"adr/0001-outbox@abcdef1"}, "spec": "spec/widget-retry"}))
		if !isToolError(result) {
			t.Fatal("get_context_bundle(both): want isError")
		}
	})

	t.Run("unpinned ref in an explicit manifest is rejected", func(t *testing.T) {
		result := b.GetContextBundle(ctx, mustArgs(t, map[string]any{"refs": []string{"adr/0001-outbox"}}))
		if !isToolError(result) {
			t.Fatal("get_context_bundle(unpinned ref): want isError")
		}
	})
}

func TestAddAnnotation_Happy(t *testing.T) {
	b, _, adrCommit := newTestBackend(t)
	ctx := context.Background()

	t.Run("targeted annotation", func(t *testing.T) {
		result := b.AddAnnotation(ctx, mustArgs(t, map[string]any{
			"author":         "jane",
			"target_ref":     "adr/0001-outbox@" + adrCommit,
			"target_heading": "outbox-pattern",
			"target_quote":   "outbox pattern for retries",
			"type":           "comment",
			"body":           "worth a second look",
		}))
		var out struct {
			ID   string `json:"id"`
			File string `json:"file"`
		}
		toolResultJSON(t, result, &out)
		if !annotationIDShapeRe.MatchString(out.ID) {
			t.Fatalf("id %q does not match a-<ULID>", out.ID)
		}
		if out.File != "adr--0001-outbox.jsonl" {
			t.Fatalf("file = %q, want adr--0001-outbox.jsonl", out.File)
		}

		annos, err := boardio.ReadAnnotationFile(b.annotationsDir() + "/" + out.File)
		if err != nil {
			t.Fatalf("reading back appended annotation: %v", err)
		}
		if len(annos) != 1 || annos[0].ID != out.ID {
			t.Fatalf("appended annotation not found back on disk: %+v", annos)
		}
	})

	t.Run("board-only annotation", func(t *testing.T) {
		result := b.AddAnnotation(ctx, mustArgs(t, map[string]any{
			"author":      "claude",
			"board_story": "STORY-1482",
			"board_x":     10.0,
			"board_y":     20.0,
			"type":        "agent-task",
			"body":        "wire up the retry worker",
		}))
		if isToolError(result) {
			t.Fatalf("add_annotation(board-only): %s", toolResultText(t, result))
		}
	})
}

func TestAddAnnotation_Negative(t *testing.T) {
	b, _, adrCommit := newTestBackend(t)
	ctx := context.Background()

	cases := map[string]map[string]any{
		"missing author":           {"target_ref": "adr/0001-outbox@" + adrCommit, "type": "comment", "body": "x"},
		"missing body":             {"author": "jane", "target_ref": "adr/0001-outbox@" + adrCommit, "type": "comment"},
		"neither target nor board": {"author": "jane", "type": "comment", "body": "x"},
		"unknown type":             {"author": "jane", "board_story": "S", "type": "not-a-type", "body": "x"},
		"malformed target ref":     {"author": "jane", "target_ref": "not-a-ref", "type": "comment", "body": "x"},
		"unpinned target ref":      {"author": "jane", "target_ref": "adr/0001-outbox", "type": "comment", "body": "x"},
		"target does not resolve":  {"author": "jane", "target_ref": "adr/does-not-exist@" + adrCommit, "type": "comment", "body": "x"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			result := b.AddAnnotation(ctx, mustArgs(t, args))
			if !isToolError(result) {
				t.Fatalf("add_annotation(%s): want isError, got success: %s", name, toolResultText(t, result))
			}
		})
	}
}

func TestListAnnotations_Happy(t *testing.T) {
	b, _, adrCommit := newTestBackend(t)
	ctx := context.Background()

	b.AddAnnotation(ctx, mustArgs(t, map[string]any{
		"author": "jane", "target_ref": "adr/0001-outbox@" + adrCommit,
		"target_heading": "outbox-pattern", "target_quote": "outbox pattern for retries",
		"type": "comment", "body": "worth a second look",
	}))

	result := b.ListAnnotations(ctx, mustArgs(t, map[string]any{"ref": "adr/0001-outbox"}))
	var out struct {
		Annotations []annotationItem `json:"annotations"`
	}
	toolResultJSON(t, result, &out)
	if len(out.Annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d: %+v", len(out.Annotations), out.Annotations)
	}
	if out.Annotations[0].Target == nil || out.Annotations[0].Target.Drift != DriftFresh {
		t.Fatalf("expected fresh drift, got %+v", out.Annotations[0].Target)
	}
}

func TestListAnnotations_Negative(t *testing.T) {
	b, _, _ := newTestBackend(t)

	t.Run("missing ref", func(t *testing.T) {
		result := b.ListAnnotations(context.Background(), mustArgs(t, map[string]any{}))
		if !isToolError(result) {
			t.Fatal("list_annotations(no ref): want isError")
		}
	})

	t.Run("no annotations yet is empty, not an error", func(t *testing.T) {
		result := b.ListAnnotations(context.Background(), mustArgs(t, map[string]any{"ref": "adr/0001-outbox"}))
		var out struct {
			Annotations []annotationItem `json:"annotations"`
		}
		toolResultJSON(t, result, &out)
		if len(out.Annotations) != 0 {
			t.Fatalf("expected no annotations, got %+v", out.Annotations)
		}
	})
}

func TestListTasks_Happy(t *testing.T) {
	b, _, adrCommit := newTestBackend(t)
	ctx := context.Background()

	b.AddAnnotation(ctx, mustArgs(t, map[string]any{
		"author": "claude", "board_story": "STORY-1482", "type": "agent-task", "body": "do the thing",
	}))
	b.AddAnnotation(ctx, mustArgs(t, map[string]any{
		"author": "jane", "target_ref": "adr/0001-outbox@" + adrCommit, "type": "comment", "body": "not a task",
	}))

	result := b.ListTasks(ctx, mustArgs(t, map[string]any{}))
	var out struct {
		Tasks []annotationItem `json:"tasks"`
	}
	toolResultJSON(t, result, &out)
	if len(out.Tasks) != 1 || out.Tasks[0].Type != "agent-task" {
		t.Fatalf("expected exactly 1 open agent-task, got %+v", out.Tasks)
	}
}

func TestListTasks_Negative(t *testing.T) {
	b, _, _ := newTestBackend(t)
	// No annotations at all yet: an empty task list, not an error.
	result := b.ListTasks(context.Background(), mustArgs(t, map[string]any{}))
	if isToolError(result) {
		t.Fatalf("list_tasks(empty store): want success, got error: %s", toolResultText(t, result))
	}
	var out struct {
		Tasks []annotationItem `json:"tasks"`
	}
	toolResultJSON(t, result, &out)
	if len(out.Tasks) != 0 {
		t.Fatalf("expected no tasks, got %+v", out.Tasks)
	}
}
