package mcpserve

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests use spec/widget-retry, the accepted feature spec
// buildFixture (fixture_test.go) actually carries on main — the brief's
// draft used "escrow-autopay", which this fixture does not have;
// widget-notes is the fixture's only other spec but is class "component",
// not "feature", so widget-retry is the one accepted feature spec these
// assertions need (task-3 resolution note).

func TestGetDocument_Happy(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	backend, repo, _ := newTestBackend(t)
	res := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/widget-retry"}))
	if isToolError(res) {
		t.Fatalf("tool error: %s", toolResultText(t, res))
	}
	var got struct {
		Ref, Kind, Commit, Engine, Markdown string
		Proposed                            bool
		Disclosures                         []string
	}
	if err := json.Unmarshal([]byte(toolResultText(t, res)), &got); err != nil {
		t.Fatal(err)
	}
	if got.Ref != "spec/widget-retry" || got.Kind != "spec" || got.Commit != repo.Head || got.Proposed || !strings.HasPrefix(got.Engine, "sha256:") {
		t.Fatalf("got %+v", got)
	}
	if !strings.HasPrefix(got.Markdown, "# ") || !strings.Contains(got.Markdown, "not authority") || !strings.HasSuffix(got.Markdown, "\n") {
		t.Fatalf("markdown wrong shape:\n%s", got.Markdown)
	}
	again := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/widget-retry"}))
	if toolResultText(t, res) != toolResultText(t, again) {
		t.Fatal("two calls over unchanged state must be byte-identical")
	}

	// F1 (review, fix round 2): in this fixture the checkout and HEAD
	// start byte-identical, so nothing above can fail if ModeAccepted were
	// swapped for ModeWorkingTree — dirty the checkout's own spec.md
	// (never committed) and prove the render still reflects the accepted
	// commit's title, not the dirty working tree's.
	specPath := filepath.Join(backend.Root, ".verdi", "specs", "active", "widget-retry", "spec.md")
	original, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(original), `title: "Widget retry"`) {
		t.Fatalf("fixture spec.md missing the expected title line: %s", original)
	}
	dirty := strings.Replace(string(original), `title: "Widget retry"`, `title: "UNCOMMITTED"`, 1)
	if err := os.WriteFile(specPath, []byte(dirty), 0o644); err != nil {
		t.Fatal(err)
	}
	dirtyRes := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/widget-retry"}))
	if isToolError(dirtyRes) {
		t.Fatalf("tool error after dirtying the working tree: %s", toolResultText(t, dirtyRes))
	}
	var dirtyGot struct{ Markdown string }
	if err := json.Unmarshal([]byte(toolResultText(t, dirtyRes)), &dirtyGot); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dirtyGot.Markdown, "# Widget retry\n") {
		t.Fatalf("get_document(spec/widget-retry) read the dirty working tree instead of the accepted commit:\n%s", dirtyGot.Markdown)
	}
}

// TestGetDocument_MalformedManifestDegradesToNilModel proves the
// model-resolution branch tool_get_document.go's `if cfg, cerr :=
// store.Open(b.Root); cerr == nil` takes when store.Open fails: a
// present-but-malformed verdi.yaml (decodable YAML, wrong schema value)
// still satisfies specdocload.Load's own Stat-only precondition, so the
// render must still succeed with a nil model rather than erroring.
func TestGetDocument_MalformedManifestDegradesToNilModel(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	backend, _, _ := newTestBackend(t)
	manifestPath := filepath.Join(backend.Root, ".verdi", "verdi.yaml")
	if err := os.WriteFile(manifestPath, []byte("schema: not-a-real-schema/v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/widget-retry"}))
	if isToolError(res) {
		t.Fatalf("tool error with a malformed (but present) verdi.yaml: %s", toolResultText(t, res))
	}
}

func TestGetDocument_KindsAndPin(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	backend, repo, _ := newTestBackend(t)
	plan := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/widget-retry", "kind": "plan"}))
	if isToolError(plan) || !strings.Contains(toolResultText(t, plan), "## Decisions") || strings.Contains(toolResultText(t, plan), "## Problem") {
		t.Fatalf("plan kind: %s", toolResultText(t, plan))
	}
	pinned := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/widget-retry@" + repo.Head}))
	if isToolError(pinned) || !strings.Contains(toolResultText(t, pinned), `"commit":"`+repo.Head+`"`) {
		t.Fatalf("pinned ref: %s", toolResultText(t, pinned))
	}
	viaArg := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/widget-retry", "commit": repo.Head}))
	if toolResultText(t, pinned) != toolResultText(t, viaArg) {
		t.Fatal("ref@commit and commit arg must agree")
	}
}

func TestGetDocument_Refusals(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	backend, repo, _ := newTestBackend(t)
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"missing ref", map[string]any{}, "ref is required"},
		{"not a spec", map[string]any{"ref": "adr/0001"}, "spec/<name>"},
		{"fragment", map[string]any{"ref": "spec/widget-retry#ac-1"}, "spec/<name>"},
		{"unknown field", map[string]any{"ref": "spec/widget-retry", "format": "html"}, "unknown field"},
		{"bad kind", map[string]any{"ref": "spec/widget-retry", "kind": "chapter"}, "unknown document kind"},
		{"pin and commit disagree", map[string]any{"ref": "spec/widget-retry@" + repo.Head, "commit": strings.Repeat("b", 40)}, "disagree"},
		{"unknown spec", map[string]any{"ref": "spec/nope"}, "spec/nope"},
		{"bad commit", map[string]any{"ref": "spec/widget-retry", "commit": "deadbeef"}, "deadbeef"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := backend.GetDocument(context.Background(), mustArgs(t, c.args))
			if !isToolError(res) {
				t.Fatalf("want tool error, got %s", toolResultText(t, res))
			}
			if !strings.Contains(toolResultText(t, res), c.want) {
				t.Fatalf("error %q does not name %q", toolResultText(t, res), c.want)
			}
		})
	}
}
