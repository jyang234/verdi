package mcpserve

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// toolFn is the shape every Backend tool method shares — enough to drive
// a table over several tools without repeating each call by hand.
type toolFn func(context.Context, json.RawMessage) map[string]any

// TestToolArgsStrictDecode_UnknownField is spec/fail-loud ac-3's core
// witness: a typo'd argument field (target_reff, the spec's own example)
// is refused BY NAME, never silently dropped — for a lone-ref tool
// (get_artifact) and a multi-field tool (add_annotation), table-driven
// per co-1. Before this change (bare json.Unmarshal, no
// DisallowUnknownFields) both cases here would have SUCCEEDED, silently
// ignoring target_reff and running on target_ref's zero value instead.
func TestToolArgsStrictDecode_UnknownField(t *testing.T) {
	b, _, _ := newTestBackend(t)
	ctx := context.Background()

	cases := []struct {
		name      string // also the tool-name error prefix (toolError's convention)
		fn        toolFn
		args      map[string]any
		wantField string
	}{
		{
			name:      "get_artifact",
			fn:        b.GetArtifact,
			args:      map[string]any{"ref": "adr/0001-outbox", "target_reff": "bogus"},
			wantField: "target_reff",
		},
		{
			name: "add_annotation",
			fn:   b.AddAnnotation,
			args: map[string]any{
				"author":      "agent",
				"type":        "comment",
				"body":        "a body",
				"target_reff": "bogus", // the spec's own example typo
			},
			wantField: "target_reff",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.fn(ctx, mustArgs(t, tc.args))
			if !isToolError(result) {
				t.Fatalf("%s(unknown field %q): want isError, got success: %#v", tc.name, tc.wantField, result)
			}
			text := toolResultText(t, result)
			if !strings.HasPrefix(text, tc.name+":") {
				t.Fatalf("%s: error text missing the tool-name prefix: %q", tc.name, text)
			}
			if !strings.Contains(text, tc.wantField) {
				t.Fatalf("%s: error text does not NAME the unknown field %q: %q", tc.name, tc.wantField, text)
			}
		})
	}
}

// TestToolArgsStrictDecode_TrailingData proves dc-2's trailing-data
// rejection half (strictUnmarshal delegates to artifact.DecodeStrictJSON,
// which checks dec.More() after the top-level value) fires from a real
// tool call, not just at the helper's own unit level.
func TestToolArgsStrictDecode_TrailingData(t *testing.T) {
	b, _, _ := newTestBackend(t)
	ctx := context.Background()

	cases := []struct {
		name string
		fn   toolFn
		raw  json.RawMessage
	}{
		{
			name: "get_artifact",
			fn:   b.GetArtifact,
			raw:  json.RawMessage(`{"ref":"adr/0001-outbox"}{}`),
		},
		{
			name: "add_annotation",
			fn:   b.AddAnnotation,
			raw:  json.RawMessage(`{"author":"agent","type":"comment","body":"x"} garbage`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.fn(ctx, tc.raw)
			if !isToolError(result) {
				t.Fatalf("%s(trailing data): want isError, got success: %#v", tc.name, result)
			}
			text := toolResultText(t, result)
			if !strings.HasPrefix(text, tc.name+":") {
				t.Fatalf("%s: error text missing the tool-name prefix: %q", tc.name, text)
			}
		})
	}
}

// TestToolArgsStrictDecode_HappyPathUnaffected is co-2's proof for
// mcpserve: a well-formed caller — every field named, nothing extra — is
// completely unaffected by the strict-decode switch. (Exhaustive
// per-tool happy-path coverage already lives in backend_test.go; this
// pins the specific two tools this file's negative cases exercise, so a
// reader sees the positive and negative case for the same tool
// side-by-side.)
func TestToolArgsStrictDecode_HappyPathUnaffected(t *testing.T) {
	b, _, _ := newTestBackend(t)
	ctx := context.Background()

	result := b.GetArtifact(ctx, mustArgs(t, map[string]any{"ref": "adr/0001-outbox"}))
	if isToolError(result) {
		t.Fatalf("get_artifact(well-formed args): want success, got error: %s", toolResultText(t, result))
	}

	result = b.AddAnnotation(ctx, mustArgs(t, map[string]any{
		"author": "agent", "type": "comment", "body": "a body", "board_story": "widget-retry",
	}))
	if isToolError(result) {
		t.Fatalf("add_annotation(well-formed args): want success, got error: %s", toolResultText(t, result))
	}
}
