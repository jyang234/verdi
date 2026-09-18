package mcpserve

import (
	"context"
	"encoding/json"
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
		{"missing ref", map[string]any{}, "ref"},
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
