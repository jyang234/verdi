// spec/vocabulary-surfaces ac-3: MCP tool descriptions speak the model's
// class display names — proven against the REAL server end to end (the
// startTestServer/ServeConn convention this package already drives, never
// a package-internal read of tooldefs.go's Go value), over a fixture
// store carrying model-schema's vocab-rename manifest, with the
// description text read back from the tool-list response on the wire.
package mcpserve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// vocabRepoDir is mustRepoDir plus internal/model/testdata's
// vocab-rename.yaml (reused verbatim, never duplicated) as the store's
// .verdi/model.yaml: feature -> "Initiative".
func vocabRepoDir(t *testing.T) string {
	t.Helper()
	root := mustRepoDir(t)
	modelYAML, err := os.ReadFile(filepath.Join("..", "model", "testdata", "vocab-rename.yaml"))
	if err != nil {
		t.Fatalf("reading vocab-rename.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "model.yaml"), modelYAML, 0o644); err != nil {
		t.Fatalf("writing model.yaml: %v", err)
	}
	return root
}

// toolDef pulls one tool's full definition out of a live tools/list
// response.
func toolDef(t *testing.T, resp map[string]any, name string) map[string]any {
	t.Helper()
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/list response has no result: %#v", resp)
	}
	tools, _ := result["tools"].([]any)
	for _, raw := range tools {
		def, _ := raw.(map[string]any)
		if def["name"] == name {
			return def
		}
	}
	t.Fatalf("tools/list response carries no %q tool", name)
	return nil
}

// toolDescription pulls one tool's description text out of a live
// tools/list response.
func toolDescription(t *testing.T, resp map[string]any, name string) string {
	t.Helper()
	desc, _ := toolDef(t, resp, name)["description"].(string)
	return desc
}

// toolArgDescription pulls one argument's description out of a tool's
// inputSchema properties.
func toolArgDescription(t *testing.T, resp map[string]any, tool, arg string) string {
	t.Helper()
	schema, _ := toolDef(t, resp, tool)["inputSchema"].(map[string]any)
	props, _ := schema["properties"].(map[string]any)
	prop, ok := props[arg].(map[string]any)
	if !ok {
		t.Fatalf("tool %q inputSchema carries no %q argument (properties: %#v)", tool, arg, props)
	}
	desc, _ := prop["description"].(string)
	return desc
}

// TestToolsList_VocabularyRenamedClassWord drives the real server over a
// vocab-rename store: get_context_bundle's description speaks the
// renamed class display word in place of today's "feature" literal.
func TestToolsList_VocabularyRenamedClassWord(t *testing.T) {
	sockPath, stop := startTestServer(t, vocabRepoDir(t))
	defer stop()

	c := dialTestServer(t, sockPath)
	defer c.conn.Close()

	resp := c.call(t, "tools/list", nil)
	desc := toolDescription(t, resp, "get_context_bundle")
	if !strings.Contains(desc, "Initiative spec's context: field") {
		t.Fatalf("get_context_bundle description = %q, want the renamed class word (\"... Initiative spec's context: field ...\")", desc)
	}
	if strings.Contains(desc, "feature spec") {
		t.Fatalf("get_context_bundle description = %q, still carries the bare class literal \"feature spec\"", desc)
	}

	// get_matrix (vocabulary-prose closure, closure finding 1): the
	// description's class word resolves — over this fixture "story"
	// resolves through the SECOND rung of the chain, the class's own
	// Class.Display "Story" — while the fold's verdict keys and the
	// story ARGUMENT name are identity and stay bare.
	matrixDesc := toolDescription(t, resp, "get_matrix")
	if !strings.Contains(matrixDesc, "The evidence fold for a Story (03") {
		t.Fatalf("get_matrix description = %q, want the resolved class word (\"The evidence fold for a Story ...\")", matrixDesc)
	}
	if !strings.Contains(matrixDesc, "a scheme-prefixed Story ref (jira:LOAN-1482)") {
		t.Fatalf("get_matrix description = %q, want the resolved class word in the ref-form prose", matrixDesc)
	}
	if strings.Contains(matrixDesc, "for a story") || strings.Contains(matrixDesc, "story ref") {
		t.Fatalf("get_matrix description = %q, still carries bare class-word prose", matrixDesc)
	}
	if !strings.Contains(matrixDesc, "story.violated/story.eligible") {
		t.Fatalf("get_matrix description = %q, must keep the fold's verdict KEYS bare (identity layer)", matrixDesc)
	}
	argDesc := toolArgDescription(t, resp, "get_matrix", "story")
	if !strings.Contains(argDesc, "a scheme-prefixed Story ref (e.g. jira:LOAN-1482)") {
		t.Fatalf("get_matrix story-argument description = %q, want the resolved class word", argDesc)
	}

	// The argument NAME itself is wire schema — toolArgDescription above
	// already proves properties["story"] still exists; the required list
	// must name it bare too.
	schema, _ := toolDef(t, resp, "get_matrix")["inputSchema"].(map[string]any)
	required, _ := schema["required"].([]any)
	if len(required) != 1 || required[0] != "story" {
		t.Fatalf("get_matrix required = %#v, want the bare argument name [story]", required)
	}

	// add_annotation's board_story argument: description prose resolves,
	// the argument names referencing it stay bare.
	boardStoryDesc := toolArgDescription(t, resp, "add_annotation", "board_story")
	if !strings.Contains(boardStoryDesc, "the Story this sticky is placed on a board for") {
		t.Fatalf("add_annotation board_story description = %q, want the resolved class word", boardStoryDesc)
	}
	if got := toolArgDescription(t, resp, "add_annotation", "board_x"); !strings.Contains(got, "requires board_story") {
		t.Fatalf("add_annotation board_x description = %q, must keep the bare board_story argument NAME", got)
	}
}

// TestToolsList_NoModelYAMLKeepsTodaysText is the parity floor on the
// wire: a store with no model.yaml serves byte-identical description
// text to today's literal.
func TestToolsList_NoModelYAMLKeepsTodaysText(t *testing.T) {
	sockPath, stop := startTestServer(t, mustRepoDir(t))
	defer stop()

	c := dialTestServer(t, sockPath)
	defer c.conn.Close()

	resp := c.call(t, "tools/list", nil)
	desc := toolDescription(t, resp, "get_context_bundle")
	if !strings.Contains(desc, "read from a feature spec's context: field") {
		t.Fatalf("get_context_bundle description = %q, want today's literal text over a no-model store", desc)
	}

	matrixDesc := toolDescription(t, resp, "get_matrix")
	if !strings.Contains(matrixDesc, "The evidence fold for a story (03") {
		t.Fatalf("get_matrix description = %q, want today's literal text over a no-model store", matrixDesc)
	}
	if !strings.Contains(matrixDesc, "a scheme-prefixed story ref (jira:LOAN-1482)") {
		t.Fatalf("get_matrix description = %q, want today's ref-form literal over a no-model store", matrixDesc)
	}
	if got := toolArgDescription(t, resp, "add_annotation", "board_story"); !strings.Contains(got, "the story this sticky is placed on a board for") {
		t.Fatalf("add_annotation board_story description = %q, want today's literal over a no-model store", got)
	}
}
