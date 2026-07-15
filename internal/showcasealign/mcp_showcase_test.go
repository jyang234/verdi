// TestMCPShowcaseCoverage (Task 3.4, MCP axis) drives all nine MCP tools
// against a real, provisioned examples/showcase store
// (helpers_test.go's provisionShowcaseStore) over the actual NDJSON wire
// protocol (mcpserve.NewServer + mcpserve.ServeConn) — the same
// inventory-from-the-wire discipline coverage_test.go's own mcpTools helper
// and internal/specalign/mcptools_test.go use for tools/list, extended here
// to tools/call. This file never reaches into mcpserve's unexported result
// types (searchResultItem, artifactResult, matrixResult, boardResult, ...
// are all unexported and, being in a different package, unreachable from
// here regardless); it defines its own minimal decode shapes instead,
// mirroring internal/mcpserve/backend_test.go's own black-box-from-the-
// wire discipline (toolResultJSON there decodes the same way, just without
// the wire hop this file adds).
//
// Every assertion below is checked against REAL examples/showcase content
// (spec/stale-decline, adr/0002-outbox-events, the real STORY-1482 board,
// stale-decline's own active/expired waivers) — never a synthetic
// throwaway fixture. This is the load-bearing distinction from
// internal/mcpserve/fixture_test.go's buildFixture, whose own doc comment
// discloses it is deliberately NOT examples/showcase (PLAN.md Phase 9: "do
// NOT hard-code examples/showcase golden SHAs, another agent is rebaking
// them") — that disclosure is exactly why Task 3.2's coverage_test.go left
// the whole MCP axis unmapped (task-3.2-report.md's "(b) Disclaimed-
// scratch-fixture gaps"). This one file closes all nine mcp: capabilities
// at once, each via a genuine tool call and a genuine assertion:
//
//   - search_artifacts: finds adr/0002-outbox-events by real full-text
//     content ("outbox").
//   - get_artifact: unpinned spec/stale-decline (current working tree) and
//     pinned adr/0002-outbox-events@HEAD (historical git resolution).
//   - get_links: stale-decline's real `implements` edge to
//     adr/0002-outbox-events, plus the computed inverse "implemented-by"
//     backlink.
//   - get_matrix: story jira:LOAN-1482 resolves to spec/stale-decline's
//     real 4 ACs; ac-4's real, active (no-expiry) waiver
//     (.verdi/waivers/jira-loan-1482/ac-4.md) folds to "waived" — a
//     genuine, non-trivial fact about committed showcase content, not a
//     synthetic assertion. (ac-1..ac-3 fold to no-signal/pending because
//     provisionShowcaseStore's own doc comment discloses it does not copy
//     examples/showcase/derived/ into the provisioned store — disclosed,
//     not silently assumed.)
//   - get_context_bundle: an explicit pinned-refs manifest resolves
//     adr/0002-outbox-events@HEAD to its real historical body. (The
//     `spec:` form is deliberately not exercised here: both specs in the
//     corpus that declare a context: field — stale-decline and
//     escrow-autopay — pin adr/0002-outbox-events at layer 1's golden head
//     (goldenHeads[0], examples/showcase/layers.txt), a commit that
//     predates adr/0002's own introduction in layer 2; resolving that pin
//     via a literal `git show <commit>:<path>` is not guaranteed to
//     succeed, and no VL lint rule validates context: pins (VL-009 checks
//     only frozen.commit, never context:), so this is genuinely
//     unverified corpus content — disclosed here rather than asserted on
//     by guesswork. The explicit-refs form already fully exercises
//     get_context_bundle's own resolution logic against real showcase
//     content.)
//   - get_board: spec/stale-decline's real board projection (readonly
//     mode — this fixturegit-built store has no design/ branch and no
//     forge configured — and its real ac-4 card).
//   - add_annotation + list_annotations: a targeted annotation pinned at
//     spec/stale-decline@HEAD, anchored to a real, verbatim line from its
//     committed "Design notes" section — list_annotations reports it back
//     with DriftFresh, proving I-17's anchor-drift computation against
//     genuine prose.
//   - add_annotation + list_tasks: a board-only agent-task annotation on
//     the real STORY-1482 board (examples/showcase/mutable/boards/
//     STORY-1482.json) — list_tasks reports it back as an open task.
package showcasealign

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/mcpserve"
)

// mcpToolCallResult is the minimal tools/call envelope this file decodes:
// the one text content item every mcpserve tool result carries, plus
// isError. Kept local (never importing mcpserve's own unexported
// toolText/toolJSON shapes) per this file's own doc comment.
type mcpToolCallResult struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`
}

// callMCPTool issues one real "tools/call" JSON-RPC request for name/args
// against srv over mcpserve.ServeConn's actual NDJSON wire framing —
// never calling a Backend method directly — and returns the result
// envelope's sole text content item plus its isError flag.
func callMCPTool(t *testing.T, srv *mcpserve.Server, name string, args map[string]any) (text string, isError bool) {
	t.Helper()

	reqObj := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	}
	data, err := json.Marshal(reqObj)
	if err != nil {
		t.Fatalf("callMCPTool(%s): marshaling request: %v", name, err)
	}
	data = append(data, '\n')

	var out bytes.Buffer
	if err := mcpserve.ServeConn(context.Background(), bytes.NewReader(data), &out, srv); err != nil {
		t.Fatalf("callMCPTool(%s): ServeConn: %v", name, err)
	}

	var resp struct {
		Result *mcpToolCallResult `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("callMCPTool(%s): decoding response %q: %v", name, out.String(), err)
	}
	if resp.Error != nil {
		t.Fatalf("callMCPTool(%s): tools/call returned a JSON-RPC error: %s", name, resp.Error.Message)
	}
	if resp.Result == nil || len(resp.Result.Content) != 1 {
		t.Fatalf("callMCPTool(%s): result has no single content item: %#v", name, resp.Result)
	}
	return resp.Result.Content[0].Text, resp.Result.IsError
}

// callMCPToolOK is callMCPTool plus the common "this call must succeed"
// assertion, returning just the result text for the caller to decode.
func callMCPToolOK(t *testing.T, srv *mcpserve.Server, name string, args map[string]any) string {
	t.Helper()
	text, isError := callMCPTool(t, srv, name, args)
	if isError {
		t.Fatalf("%s: want success, got a tool error: %s", name, text)
	}
	return text
}

// decodeToolJSON decodes a tool result's text content (already known
// successful) as JSON into out.
func decodeToolJSON(t *testing.T, text string, out any) {
	t.Helper()
	if err := json.Unmarshal([]byte(text), out); err != nil {
		t.Fatalf("decoding tool result JSON: %v\ntext: %s", err, text)
	}
}

func TestMCPShowcaseCoverage(t *testing.T) {
	root := provisionShowcaseStore(t)
	srv := mcpserve.NewServer(root)
	ctx := context.Background()

	head, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		t.Fatalf("resolving HEAD of the provisioned showcase store: %v", err)
	}

	t.Run("search_artifacts", func(t *testing.T) {
		text := callMCPToolOK(t, srv, "search_artifacts", map[string]any{"query": "outbox"})
		var out struct {
			Results []struct {
				Ref  string `json:"ref"`
				Kind string `json:"kind"`
			} `json:"results"`
		}
		decodeToolJSON(t, text, &out)

		found := false
		for _, r := range out.Results {
			if r.Ref == "adr/0002-outbox-events" {
				found = true
			}
		}
		if !found {
			t.Fatalf("search_artifacts(outbox) against the showcase store did not find adr/0002-outbox-events: %+v", out.Results)
		}
	})

	t.Run("get_artifact", func(t *testing.T) {
		text := callMCPToolOK(t, srv, "get_artifact", map[string]any{"ref": "spec/stale-decline"})
		var out struct {
			Kind  string `json:"kind"`
			Title string `json:"title"`
			Body  string `json:"body"`
		}
		decodeToolJSON(t, text, &out)
		if out.Kind != "spec" {
			t.Fatalf("get_artifact(spec/stale-decline).Kind = %q, want spec", out.Kind)
		}
		if !strings.Contains(out.Body, "Every stale-decline consequence is routed through the outbox pattern") {
			t.Fatalf("get_artifact(spec/stale-decline).Body missing its real showcase prose: %q", out.Body)
		}

		pinnedText := callMCPToolOK(t, srv, "get_artifact", map[string]any{"ref": "adr/0002-outbox-events@" + head})
		var pinnedOut struct {
			Kind string `json:"kind"`
			Body string `json:"body"`
		}
		decodeToolJSON(t, pinnedText, &pinnedOut)
		if pinnedOut.Kind != "adr" {
			t.Fatalf("get_artifact(adr/0002-outbox-events@HEAD).Kind = %q, want adr", pinnedOut.Kind)
		}
		if !strings.Contains(pinnedOut.Body, "Transactional outbox") {
			t.Fatalf("get_artifact(adr/0002-outbox-events@HEAD).Body missing its real ADR prose: %q", pinnedOut.Body)
		}
	})

	t.Run("get_links", func(t *testing.T) {
		text := callMCPToolOK(t, srv, "get_links", map[string]any{"ref": "spec/stale-decline"})
		var out struct {
			Links []struct {
				Type string `json:"type"`
				Ref  string `json:"ref"`
			} `json:"links"`
		}
		decodeToolJSON(t, text, &out)
		found := false
		for _, l := range out.Links {
			if l.Type == "implements" && l.Ref == "adr/0002-outbox-events" {
				found = true
			}
		}
		if !found {
			t.Fatalf("get_links(spec/stale-decline) missing its real implements->adr/0002-outbox-events link: %+v", out.Links)
		}

		backText := callMCPToolOK(t, srv, "get_links", map[string]any{"ref": "adr/0002-outbox-events"})
		var backOut struct {
			Backlinks []struct {
				From string `json:"from"`
				Type string `json:"type"`
			} `json:"backlinks"`
		}
		decodeToolJSON(t, backText, &backOut)
		foundBack := false
		for _, bl := range backOut.Backlinks {
			if bl.From == "spec/stale-decline" && bl.Type == "implemented-by" {
				foundBack = true
			}
		}
		if !foundBack {
			t.Fatalf("get_links(adr/0002-outbox-events) missing the computed implemented-by backlink from spec/stale-decline: %+v", backOut.Backlinks)
		}
	})

	t.Run("get_matrix", func(t *testing.T) {
		text := callMCPToolOK(t, srv, "get_matrix", map[string]any{"story": "jira:LOAN-1482"})
		var out struct {
			SpecRef string `json:"spec_ref"`
			ACs     []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"acs"`
			Eligible bool `json:"eligible"`
		}
		decodeToolJSON(t, text, &out)
		if out.SpecRef != "spec/stale-decline" {
			t.Fatalf("get_matrix(jira:LOAN-1482).SpecRef = %q, want spec/stale-decline", out.SpecRef)
		}
		if len(out.ACs) != 4 {
			t.Fatalf("get_matrix(jira:LOAN-1482) returned %d ACs, want 4 (ac-1..ac-4): %+v", len(out.ACs), out.ACs)
		}

		gotAC4 := false
		for _, ac := range out.ACs {
			if ac.ID == "ac-4" {
				gotAC4 = true
				if ac.Status != "waived" {
					t.Fatalf("get_matrix(jira:LOAN-1482) ac-4.Status = %q, want waived (examples/showcase's own real, active, no-expiry waiver)", ac.Status)
				}
			}
		}
		if !gotAC4 {
			t.Fatalf("get_matrix(jira:LOAN-1482) missing ac-4: %+v", out.ACs)
		}
		if out.Eligible {
			t.Fatalf("get_matrix(jira:LOAN-1482).Eligible = true, want false (ac-1..ac-3 carry no derived evidence under this harness's provisioning — helpers_test.go's own disclosed gap: examples/showcase/derived/ is not copied)")
		}
	})

	t.Run("get_context_bundle", func(t *testing.T) {
		text := callMCPToolOK(t, srv, "get_context_bundle", map[string]any{
			"refs": []string{"adr/0002-outbox-events@" + head},
		})
		var out struct {
			Items []struct {
				Ref  string `json:"ref"`
				Kind string `json:"kind"`
				Body string `json:"body"`
			} `json:"items"`
		}
		decodeToolJSON(t, text, &out)
		if len(out.Items) != 1 || out.Items[0].Ref != "adr/0002-outbox-events" {
			t.Fatalf("get_context_bundle(adr/0002-outbox-events@HEAD) = %+v, want exactly one item for the real ADR", out.Items)
		}
		if !strings.Contains(out.Items[0].Body, "Transactional outbox") {
			t.Fatalf("get_context_bundle item body missing its real ADR prose: %q", out.Items[0].Body)
		}
	})

	t.Run("get_board", func(t *testing.T) {
		text := callMCPToolOK(t, srv, "get_board", map[string]any{"ref": "spec/stale-decline"})
		var out struct {
			Spec  string `json:"spec"`
			Mode  string `json:"mode"`
			Cards []struct {
				ID string `json:"id"`
			} `json:"cards"`
		}
		decodeToolJSON(t, text, &out)
		if out.Spec != "stale-decline" {
			t.Fatalf("get_board(spec/stale-decline).Spec = %q, want stale-decline", out.Spec)
		}
		if out.Mode != "readonly" {
			t.Fatalf("get_board(spec/stale-decline).Mode = %q, want readonly (no design branch, no forge configured in this fixturegit-built store)", out.Mode)
		}
		found := false
		for _, c := range out.Cards {
			if c.ID == "ac-4" {
				found = true
			}
		}
		if !found {
			t.Fatalf("get_board(spec/stale-decline) missing its real ac-4 card: %+v", out.Cards)
		}
	})

	// add_annotation, list_annotations, and list_tasks are exercised as
	// plain sequential Go code within two subtests (not three independent
	// t.Run bodies) so the write genuinely precedes its read — never
	// relying on subtest execution ordering.
	t.Run("add_annotation_then_list_annotations", func(t *testing.T) {
		addText := callMCPToolOK(t, srv, "add_annotation", map[string]any{
			"author":         "showcase-coverage-test",
			"target_ref":     "spec/stale-decline@" + head,
			"target_heading": "Design notes",
			"target_quote":   "Every stale-decline consequence is routed through the outbox pattern",
			"type":           "comment",
			"body":           "showcase-coverage: genuine annotation anchored to real stale-decline prose",
		})
		var addOut struct {
			ID   string `json:"id"`
			File string `json:"file"`
		}
		decodeToolJSON(t, addText, &addOut)
		if addOut.ID == "" || addOut.File == "" {
			t.Fatalf("add_annotation(spec/stale-decline) returned no id/file: %+v", addOut)
		}

		listText := callMCPToolOK(t, srv, "list_annotations", map[string]any{"ref": "spec/stale-decline"})
		var listOut struct {
			Annotations []struct {
				ID     string `json:"id"`
				Target *struct {
					Drift string `json:"drift"`
				} `json:"target"`
			} `json:"annotations"`
		}
		decodeToolJSON(t, listText, &listOut)

		found := false
		for _, a := range listOut.Annotations {
			if a.ID == addOut.ID {
				found = true
				if a.Target == nil || a.Target.Drift != "fresh" {
					t.Fatalf("list_annotations: annotation %s Target = %+v, want Drift fresh (the pinned quote is real, unmoved stale-decline prose)", a.ID, a.Target)
				}
			}
		}
		if !found {
			t.Fatalf("list_annotations(spec/stale-decline) did not return the annotation just added (id %s): %+v", addOut.ID, listOut.Annotations)
		}
	})

	t.Run("add_board_task_then_list_tasks", func(t *testing.T) {
		addText := callMCPToolOK(t, srv, "add_annotation", map[string]any{
			"author":      "showcase-coverage-test",
			"board_story": "STORY-1482",
			"board_x":     10.0,
			"board_y":     20.0,
			"type":        "agent-task",
			"body":        "showcase-coverage: genuine agent-task on the real STORY-1482 board",
		})
		var addOut struct {
			ID string `json:"id"`
		}
		decodeToolJSON(t, addText, &addOut)
		if addOut.ID == "" {
			t.Fatalf("add_annotation(board_story STORY-1482) returned no id: %+v", addOut)
		}

		listText := callMCPToolOK(t, srv, "list_tasks", map[string]any{})
		var listOut struct {
			Tasks []struct {
				ID   string `json:"id"`
				Type string `json:"type"`
			} `json:"tasks"`
		}
		decodeToolJSON(t, listText, &listOut)

		found := false
		for _, task := range listOut.Tasks {
			if task.ID == addOut.ID && task.Type == "agent-task" {
				found = true
			}
		}
		if !found {
			t.Fatalf("list_tasks did not return the open agent-task just added (id %s): %+v", addOut.ID, listOut.Tasks)
		}
	})
}
