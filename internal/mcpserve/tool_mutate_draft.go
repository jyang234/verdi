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
// other application diagnostic renders designapp's typed failure envelope,
// carrying the exact 0/1/2 classification the CLI projects onto its exit
// code.
//
// The failures BEFORE the application call — an oversized argument
// payload, malformed arguments, a rejected harness id, a request this
// server could not re-encode or the mutation codec would not decode —
// stay plain text tool errors. They are transport- and argument-level
// faults of the MCP call itself, not classifications the application core
// produced, so projecting them into its envelope would attribute a
// verdict to a core that never ran.
func (b *Backend) MutateDraft(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	// draftmutation.DecodeRequest enforces MaxRequestBytes against the
	// bytes it is handed, and the bytes it is handed here are this
	// server's own canonical re-marshal — which can be SMALLER than what
	// the caller actually sent (canonicalization drops insignificant
	// whitespace, and the flattened harness/session fields are stripped
	// before re-marshaling). Checking the raw incoming argument bytes
	// first restores the ceiling as a real resource bound on caller input,
	// rather than one on a projection of it: an oversized payload is
	// refused before this server allocates a decoded copy of it.
	if len(argsRaw) > draftmutation.MaxRequestBytes {
		return toolError("mutate_draft: arguments exceed 1 MiB")
	}
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
			// The stale refusal keeps draftmutation's OWN typed projection
			// verbatim: it is the CO-9 conformance object the CLI writes to
			// stdout byte-for-byte, and it already carries both its own
			// versioned schema and the current digest plus changed object
			// identities CO-1 requires for a reload. Wrapping it in the
			// generic failure envelope would trade those reload facts, and
			// the CLI's byte-identical parity, for a classification field
			// its schema already implies (a stale base is always a verdict:
			// the CLI exits 1 on exactly this branch).
			rendered := toolJSON(response.Stale)
			rendered["isError"] = true
			return rendered
		}
		// Every other diagnostic carries its 0/1/2 classification from
		// draftmutation's own Verdict(), so an agent can tell a refusal it
		// should correct from a breakage it should report — the same
		// distinction the CLI makes by exiting 1 or 2.
		return designAppToolFailure(designapp.MutationFailure(diagnostic))
	}
	if response.Result == nil {
		return designAppToolFailure(designapp.NewFailure(designapp.ClassificationOperational, "result-invalid",
			// vocab:identity — ASD protocol/tool name in an operational diagnostic
			"draft mutation service returned an invalid response union"))
	}
	return toolJSON(response.Result)
}
