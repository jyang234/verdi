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

// toolDescription pulls one tool's description text out of a live
// tools/list response.
func toolDescription(t *testing.T, resp map[string]any, name string) string {
	t.Helper()
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/list response has no result: %#v", resp)
	}
	tools, _ := result["tools"].([]any)
	for _, raw := range tools {
		def, _ := raw.(map[string]any)
		if def["name"] == name {
			desc, _ := def["description"].(string)
			return desc
		}
	}
	t.Fatalf("tools/list response carries no %q tool", name)
	return ""
}

// TestToolsList_VocabularyRenamedClassWord drives the real server over a
// vocab-rename store: get_context_bundle's description speaks the
// renamed class display word in place of today's "feature" literal.
func TestToolsList_VocabularyRenamedClassWord(t *testing.T) {
	sockPath, stop := startTestServer(t, vocabRepoDir(t))
	defer stop()

	c := dialTestServer(t, sockPath)
	defer c.conn.Close()

	desc := toolDescription(t, c.call(t, "tools/list", nil), "get_context_bundle")
	if !strings.Contains(desc, "Initiative spec's context: field") {
		t.Fatalf("get_context_bundle description = %q, want the renamed class word (\"... Initiative spec's context: field ...\")", desc)
	}
	if strings.Contains(desc, "feature spec") {
		t.Fatalf("get_context_bundle description = %q, still carries the bare class literal \"feature spec\"", desc)
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

	desc := toolDescription(t, c.call(t, "tools/list", nil), "get_context_bundle")
	if !strings.Contains(desc, "read from a feature spec's context: field") {
		t.Fatalf("get_context_bundle description = %q, want today's literal text over a no-model store", desc)
	}
}
