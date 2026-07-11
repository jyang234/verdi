// MCP tool inventory (deliverable 1c): the live server's tools/list
// result must equal 05-surfaces.md §MCP server's table exactly — the
// eight named tools, no more, no fewer, same spelling.
package specalign

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/mcpserve"
)

func TestMCPToolInventory(t *testing.T) {
	// 05-surfaces.md §MCP server's table, verbatim.
	want := []string{
		"search_artifacts",
		"get_artifact",
		"get_links",
		"get_matrix",
		"get_context_bundle",
		"list_annotations",
		"list_tasks",
		"add_annotation",
	}

	srv := mcpserve.NewServer(verdiRepoRoot)
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	var out bytes.Buffer
	if err := mcpserve.ServeConn(context.Background(), strings.NewReader(req), &out, srv); err != nil {
		t.Fatalf("ServeConn(tools/list): %v", err)
	}

	var resp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
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

	var got []string
	for _, tool := range resp.Result.Tools {
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
