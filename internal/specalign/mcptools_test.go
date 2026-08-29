// MCP tool inventory (deliverable 1c): the live server's tools/list
// result must equal this file's own `want` list exactly — no more, no
// fewer, same spelling. Grown at V1-P9 (item 4, the spec-align
// regrowth): get_board (05's own coverage gap, closed at V1-P9 item 1)
// joined the inventory, and list_annotations' description is asserted to
// actually document its mirrored review-sticky population (05: "covers...
// and (mirrored) review stickies") rather than silently leaving that half
// of the row unverified. Grown again for CSE Wave 5B's `experiment` tool
// (SI-145) and Wave 6 Task 1's five new ASD tools (AC-8) — the gate
// grows, never shrinks (CLAUDE.md).
//
// BINDING NOTE — this want-list DIVERGES from verdi-surfaces §MCP server.
// That specification's tool table is the nominal authority for this
// inventory, and it does not carry the six ASD rows; its add_annotation
// row still reads "append to the mutable zone (the only write tool)",
// which mutate_draft's arrival makes false. The divergence is deliberate
// and disclosed, not an oversight: the ASD specification
// (.verdi/specs/active/ai-assisted-spec-design/spec.md, AC-8) fixes the
// six-operation surface, both specifications are accepted, and reconciling
// them requires a verdi-surfaces amendment — owner-routed, and outside a
// Task 1 implementation unit's write set (the Task 1 brief excludes
// "specifications, plans, invention-ledger entries").
//
// Until that amendment lands, THIS self-owned want-list is the operative
// inventory: it is what the live server is gated against, and a reader
// must not take verdi-surfaces' table as the current truth for the MCP
// tool set or for which tools write. When the amendment lands, this note
// goes away and that table becomes the single authority again.
package specalign

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/mcpserve"
)

// mcpToolDef is one tools/list entry's shape this test needs: name plus
// description (the description carries the data-never-instructions note
// AND, for list_annotations, the review-population documentation this
// test locks in below).
type mcpToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// listMCPTools drives the real, live server's tools/list over the exact
// NDJSON wire framing a client would (mcpserve.ServeConn), never reaching
// into mcpserve's internals directly — this is an inventory-from-the-wire
// check, not a unit test of tooldefs.go.
func listMCPTools(t *testing.T) []mcpToolDef {
	t.Helper()
	srv := mcpserve.NewServer(verdiRepoRoot)
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	var out bytes.Buffer
	if err := mcpserve.ServeConn(context.Background(), strings.NewReader(req), &out, srv); err != nil {
		t.Fatalf("ServeConn(tools/list): %v", err)
	}

	var resp struct {
		Result struct {
			Tools []mcpToolDef `json:"tools"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decoding tools/list response %q: %v", out.String(), err)
	}
	if resp.Error != nil {
		t.Fatalf("tools/list returned a JSON-RPC error: %s", resp.Error.Message)
	}
	return resp.Result.Tools
}

// guide-claim: 12-mcp-tools
func TestMCPToolInventory(t *testing.T) {
	// 05-surfaces.md §MCP server's table, verbatim, plus `experiment` (CSE
	// Wave 5B, docs/superpowers/specs/2026-08-23-cse-wave-5-application-
	// lifecycle-design.md §8, ledger SI-145): the one agent-facing typed
	// gateway over internal/experimentapp, registered in the same commit as
	// its live tooldefs.go row (the MCP tool inventory is a serialized
	// shared registry).
	// Wave 6 Task 1 (docs/superpowers/plans/2026-08-29-wave-6-workbench-
	// presentation.md §6.1, AC-8): adds the five remaining ASD operations
	// alongside the pre-existing get_board, completing AC-8's exact
	// six-operation surface — the MCP tool inventory is a serialized
	// shared registry; its spec-align row changes in the same commit as
	// the live tooldefs.go registration.
	want := []string{
		"search_artifacts",
		"get_artifact",
		"get_links",
		"get_matrix",
		"get_context_bundle",
		"list_annotations",
		"list_tasks",
		"get_board",
		"add_annotation",
		"experiment",
		"get_design_context",
		"get_design_capabilities",
		"mutate_draft",
		"get_design_provenance",
		"prepare_design_review",
	}

	tools := listMCPTools(t)

	var got []string
	for _, tool := range tools {
		got = append(got, tool.Name)
	}

	gotSorted := append([]string(nil), got...)
	sort.Strings(gotSorted)
	wantSorted := append([]string(nil), want...)
	sort.Strings(wantSorted)

	if len(gotSorted) != len(wantSorted) {
		t.Fatalf("mcp tools/list returned %d tool(s), want %d\n  got:  %v\n  want: %v", len(gotSorted), len(wantSorted), got, want)
	}
	for i := range gotSorted {
		if gotSorted[i] != wantSorted[i] {
			t.Fatalf("mcp tool inventory mismatch:\n  got (sorted):  %v\n  want (sorted): %v", gotSorted, wantSorted)
		}
	}
}

// TestMCPToolInventory_ListAnnotationsDocumentsReviewPopulation proves
// list_annotations' live tool description actually documents its
// mirrored review-sticky population (05 §MCP server's row: "covers the R4
// annotation types ... AND ... (mirrored) review stickies") — not just
// that the tool exists, which TestMCPToolInventory already covers. A
// description regression that silently drops this documentation (the gap
// V1-P9 item 1 found and fixed: the description previously named only the
// R4 annotation types, never review stickies at all) fails this test.
func TestMCPToolInventory_ListAnnotationsDocumentsReviewPopulation(t *testing.T) {
	tools := listMCPTools(t)
	for _, tool := range tools {
		if tool.Name != "list_annotations" {
			continue
		}
		lower := strings.ToLower(tool.Description)
		if !strings.Contains(lower, "review") {
			t.Fatalf("list_annotations description does not mention review-sticky population: %q", tool.Description)
		}
		return
	}
	t.Fatal("list_annotations not found in tools/list")
}
