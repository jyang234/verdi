package mcpserve

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/recovery"
)

// fakeRecoveryLoader is a hermetic, fixed-response RecoveryLoader test
// double — GetRecovery's own tests never depend on internal/recovery's
// real git-derived Gather (that lives in internal/recovery's own tests).
type fakeRecoveryLoader struct {
	proj recovery.Projection
	err  error
}

func (f fakeRecoveryLoader) Load(context.Context, string) (recovery.Projection, error) {
	return f.proj, f.err
}

// fixedRecoveryProjection is a minimal, Validate-clean projection (every
// schema.go rule satisfied): one recognized state with a manual-only
// choice, no digest set (Canonical recomputes it).
func fixedRecoveryProjection() recovery.Projection {
	return recovery.Projection{
		Schema: recovery.SchemaID,
		Ref:    "spec/x",
		Branch: "main",
		Head:   "abc123",
		States: []recovery.RecognizedState{
			{
				Code:           recovery.StateStaleLock,
				Scope:          recovery.ScopeStore,
				Target:         ".verdi/data/writer.lock",
				Facts:          []string{"the writer lock names a pid that is no longer running"},
				Uncertainties:  []recovery.Uncertainty{},
				StepsCompleted: []string{},
				InvariantsHeld: []string{"HEAD is abc123 and no ritual branch was modified by this run"},
				Choices: []recovery.Choice{
					{
						ID:             "manual:.verdi/data/writer.lock",
						Summary:        "remove the stale writer lock",
						Preconditions:  []string{"the named pid is not running"},
						Effects:        []string{"the lock file is removed"},
						Reversibility:  recovery.ReversibilityReversible,
						Confirmation:   "none: no executor",
						Postconditions: []string{"the lock file no longer exists"},
						Executor:       "none",
						ManualCommands: []string{"rm .verdi/data/writer.lock"},
					},
				},
			},
		},
		Disclosures: []string{},
	}
}

func TestGetRecovery_Happy(t *testing.T) {
	b := &Backend{RecoveryLoader: fakeRecoveryLoader{proj: fixedRecoveryProjection()}}
	res := b.GetRecovery(context.Background(), json.RawMessage(`{"ref":"spec/x"}`))

	if isToolError(res) {
		t.Fatalf("GetRecovery returned a tool error: %s", toolResultText(t, res))
	}
	text := toolResultText(t, res)
	proj, err := recovery.Decode([]byte(text))
	if err != nil {
		t.Fatalf("recovery.Decode(get_recovery result): %v\ntext: %s", err, text)
	}
	if proj.Ref != "spec/x" {
		t.Fatalf("proj.Ref = %q, want spec/x", proj.Ref)
	}
	if len(proj.States) != 1 || proj.States[0].Code != recovery.StateStaleLock {
		t.Fatalf("proj.States = %+v, want one stale-lock state", proj.States)
	}
}

func TestGetRecovery_NilLoader(t *testing.T) {
	b := &Backend{}
	res := b.GetRecovery(context.Background(), json.RawMessage(`{"ref":"spec/x"}`))
	if !isToolError(res) {
		t.Fatalf("GetRecovery with a nil loader: want a tool error, got %s", toolResultText(t, res))
	}
	if text := toolResultText(t, res); !strings.Contains(text, "recovery projection not supplied") {
		t.Fatalf("error text %q does not mention \"recovery projection not supplied\"", text)
	}
}

func TestGetRecovery_StrictDecodeRefusesUnknownField(t *testing.T) {
	b := &Backend{RecoveryLoader: fakeRecoveryLoader{proj: fixedRecoveryProjection()}}
	res := b.GetRecovery(context.Background(), json.RawMessage(`{"ref":"spec/x","apply":"y"}`))
	if !isToolError(res) {
		t.Fatalf("GetRecovery with an extra field: want a tool error, got %s", toolResultText(t, res))
	}
	if text := toolResultText(t, res); !strings.Contains(text, "apply") {
		t.Fatalf("error text %q does not name the unknown field", text)
	}
}

// TestGetRecovery_StoryRefIsAToolError proves a scheme-prefixed story ref
// (no "kind/name" shape) refuses before ever reaching the loader — the
// loader here would happily succeed, so a tool error proves GetRecovery's
// own ref-shape check fired.
func TestGetRecovery_StoryRefIsAToolError(t *testing.T) {
	b := &Backend{RecoveryLoader: fakeRecoveryLoader{proj: fixedRecoveryProjection()}}
	res := b.GetRecovery(context.Background(), json.RawMessage(`{"ref":"jira:LOAN-1482"}`))
	if !isToolError(res) {
		t.Fatalf("GetRecovery with a story ref: want a tool error, got %s", toolResultText(t, res))
	}
}

// TestServer_CallsGetRecovery proves the callTool dispatch switch
// actually reaches Backend.GetRecovery for the "get_recovery" tool name
// (following get_document's own dispatch wiring — server.go's switch —
// rather than only proving the Backend method in isolation above).
func TestServer_CallsGetRecovery(t *testing.T) {
	srv := &Server{Backend: &Backend{RecoveryLoader: fakeRecoveryLoader{proj: fixedRecoveryProjection()}}}
	params, err := json.Marshal(map[string]any{"name": "get_recovery", "arguments": map[string]any{"ref": "spec/x"}})
	if err != nil {
		t.Fatal(err)
	}
	res := srv.callTool(context.Background(), params)
	if isToolError(res) {
		t.Fatalf("callTool(get_recovery) returned a tool error: %s", toolResultText(t, res))
	}
	text := toolResultText(t, res)
	if _, err := recovery.Decode([]byte(text)); err != nil {
		t.Fatalf("recovery.Decode(callTool(get_recovery) result): %v\ntext: %s", err, text)
	}
}
