package mcpserve

import (
	"context"
	"encoding/json"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/designapp"
	"github.com/jyang234/verdi/internal/draftmutation"
)

// mutateDraftArgs mirrors draftmutation.Request's exact wire fields
// (schema/spec/base_digest/base_spec_b64/expected/operations/excerpts)
// flattened alongside harness/session — the same flattened-single-object
// convention the "experiment" tool already uses (tooldefs.go). harness/
// session are never part of draftmutation.Request itself (CO-4: actor
// identity is adapter-controlled, never a request payload field); this
// server, not the caller, mints the Actor from them.
type mutateDraftArgs struct {
	Harness     string                         `json:"harness"`
	Session     string                         `json:"session,omitempty"`
	Schema      string                         `json:"schema"`
	Spec        string                         `json:"spec"`
	BaseDigest  string                         `json:"base_digest"`
	BaseSpecB64 string                         `json:"base_spec_b64"`
	Expected    draftmutation.ExpectedIdentity `json:"expected"`
	Operations  []draftmutation.Operation      `json:"operations"`
	Excerpts    []draftmutation.ExcerptRequest `json:"excerpts,omitempty"`
}

// MutateDraft implements the mutate_draft tool (AC-8): applies one atomic
// typed transaction. The actor is always minted as a delegated agent
// here — SI-163: "MCP actor stays delegated-agent (agent-controlled);
// browser attribution is NOT implemented" — never accepted as a caller-
// supplied field (CO-4: an agent does not self-authorize).
//
// Response shape mirrors `verdi design mutate`'s own three-way CLI
// branching exactly (cmd/verdi/designmutate.go), the same conformance
// pairing internal/designapp/conformance_test.go proves: a clean result
// renders its typed JSON with no isError; a stale refusal renders its
// typed StaleRefusal JSON WITH isError (05 §MCP server: "verdict ...
// results are tool errors carrying the same typed JSON projection"); any
// other verdict/operational diagnostic is a plain toolError, exactly as
// unstructured as the CLI's own non-stale diagnostic line.
func (b *Backend) MutateDraft(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	var args mutateDraftArgs
	if err := strictUnmarshal(argsRaw, &args); err != nil {
		return toolError("mutate_draft: malformed arguments: " + err.Error())
	}

	actor, err := draftmutation.NewDelegatedAgent(args.Harness, args.Session)
	if err != nil {
		return toolError("mutate_draft: " + err.Error())
	}

	raw, err := canonjson.Marshal(draftmutation.Request{
		Schema: args.Schema, Spec: args.Spec, BaseDigest: args.BaseDigest, BaseSpecB64: args.BaseSpecB64,
		Expected: args.Expected, Operations: args.Operations, Excerpts: args.Excerpts,
	})
	if err != nil {
		return toolError("mutate_draft: encoding request: " + err.Error())
	}
	request, err := draftmutation.DecodeRequest(raw)
	if err != nil {
		return toolError("mutate_draft: " + err.Error())
	}

	app := designapp.NewService()
	response, diagnostic := app.MutateDraft(ctx, b.Root, request, actor)
	if diagnostic != nil {
		if diagnostic.Code == draftmutation.CodeStaleBase && response.Stale != nil {
			rendered := toolJSON(response.Stale)
			rendered["isError"] = true
			return rendered
		}
		return toolError("mutate_draft: " + string(diagnostic.Code) + ": " + diagnostic.Error())
	}
	if response.Result == nil {
		// vocab:identity — ASD protocol/tool name in an operational diagnostic
		return toolError("mutate_draft: result-invalid: draft mutation service returned an invalid response union")
	}
	return toolJSON(response.Result)
}
