package mcpserve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/verdi/internal/fixturegit"
)

const fixtureADR = `---
id: adr/0001-outbox
kind: adr
title: "Outbox pattern"
status: accepted
owners: [platform-team]
decided: 2026-01-01
frozen: { at: 2026-01-01, commit: 0000000000000000000000000000000000000a }
---
# Outbox pattern

We use the outbox pattern for retries.
`

const fixtureComponentSpec = `---
id: spec/widget-notes
kind: spec
class: component
title: "Widget notes"
status: active
owners: [platform-team]
---
# Widget notes

Some component notes.
`

// buildFixture builds a small, self-contained fixturegit repo exercising
// enough of the store to exhaust every mcpserve tool — deliberately NOT
// testdata/corpus (PLAN.md Phase 9: "do NOT hard-code testdata/corpus
// golden SHAs, another agent is rebaking them"). Every commit SHA used
// below is learned from a real (deterministic, per fixturegit's fixed
// author/committer/date) git build, never hardcoded: a throwaway
// single-layer probe build learns the ADR's commit before the second
// layer's spec.md is authored (spec.md's context: field pins to it), then
// the real two-layer repo is built — fixturegit's fixed identity and
// commit date make the probe's layer-1 SHA reproduce byte-for-byte inside
// the real build.
//
// Returns the built repo (repo.Dir is the store root — a real git
// checkout) and the ADR's commit SHA (repo.Heads[0], returned separately
// for tests that need to construct their own pinned refs).
func buildFixture(t *testing.T) (*fixturegit.Repo, string) {
	t.Helper()

	adrLayer := fixturegit.Layer{
		Files:   map[string]string{".verdi/adr/0001-outbox.md": fixtureADR},
		Message: "layer 1: adr",
	}
	probe := fixturegit.Build(t, []fixturegit.Layer{adrLayer})
	adrCommit := probe.Head

	spec := fmt.Sprintf(`---
id: spec/widget-retry
kind: spec
class: feature
title: "Widget retry"
status: accepted-pending-build
owners: [platform-team]
story: jira:LOAN-1482
context: ["adr/0001-outbox@%s"]
links:
  - { type: implements, ref: adr/0001-outbox }
acceptance_criteria:
  - { id: ac-1, text: "retries succeed", evidence: [static] }
frozen: { at: 2026-05-14, commit: %s }
---
# Widget retry

## Design notes

The charge API needs a retry note, retried through the outbox pattern.

## AC 1

The retry worker drains the outbox on a fixed interval.
`, adrCommit, adrCommit)

	layer2 := fixturegit.Layer{
		Files: map[string]string{
			".verdi/specs/active/widget-retry/spec.md": spec,
			".verdi/specs/active/widget-notes/spec.md": fixtureComponentSpec,
		},
		Message: "layer 2: specs",
	}

	repo := fixturegit.Build(t, []fixturegit.Layer{adrLayer, layer2})
	if repo.Heads[0] != adrCommit {
		t.Fatalf("fixturegit probe/real build disagree on layer 1's SHA: probe=%s real=%s (fixturegit determinism assumption broken)", adrCommit, repo.Heads[0])
	}
	return repo, adrCommit
}

// newTestBackend builds the fixture and returns a ready Backend rooted at
// it.
func newTestBackend(t *testing.T) (*Backend, *fixturegit.Repo, string) {
	t.Helper()
	repo, adrCommit := buildFixture(t)
	return &Backend{Root: repo.Dir}, repo, adrCommit
}

// mustArgs marshals v to json.RawMessage or fails the test.
func mustArgs(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshaling args: %v", err)
	}
	return data
}

// toolResultText extracts the single text content item's string from an
// MCP tool result map (every tool in this package returns exactly one).
func toolResultText(t *testing.T, result map[string]any) string {
	t.Helper()
	content, ok := result["content"].([]map[string]any)
	if !ok || len(content) != 1 {
		t.Fatalf("tool result has no single content item: %#v", result)
	}
	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatalf("tool result content[0].text is not a string: %#v", content[0])
	}
	return text
}

// toolResultJSON decodes a successful tool result's text content as JSON
// into out.
func toolResultJSON(t *testing.T, result map[string]any, out any) {
	t.Helper()
	if isToolError(result) {
		t.Fatalf("tool result is an error: %s", toolResultText(t, result))
	}
	if err := json.Unmarshal([]byte(toolResultText(t, result)), out); err != nil {
		t.Fatalf("decoding tool result JSON: %v\ntext: %s", err, toolResultText(t, result))
	}
}

// isToolError reports whether result carries isError: true.
func isToolError(result map[string]any) bool {
	v, _ := result["isError"].(bool)
	return v
}

// writeMutableFile writes content at root/.verdi/data/<relPath>, creating
// parent directories — the mutable zone is never git-tracked (VL-013), so
// these files are written directly, outside fixturegit.
func writeMutableFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	full := filepath.Join(root, ".verdi", "data", filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", relPath, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", relPath, err)
	}
}
