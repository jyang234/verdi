package mcpserve

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/draftmutation"
)

func mutateDraftArgsFor(t *testing.T, root string, operations []map[string]any) map[string]any {
	t.Helper()
	base := []byte(asdSampleSpec)
	identity, err := draftmutation.ResolveCanonicalIdentity(context.Background(), root, "spec/sample", draftmutation.GitIdentityReader{})
	if err != nil {
		t.Fatalf("resolving identity: %v", err)
	}
	return map[string]any{
		"harness": "codex", "session": "session-1",
		"schema": draftmutation.RequestSchema, "spec": "spec/sample",
		"base_digest": draftmutation.DigestBytes(base), "base_spec_b64": base64.StdEncoding.EncodeToString(base),
		"expected":   map[string]any{"checkout": identity.Checkout, "branch": identity.Branch, "head": identity.Head},
		"operations": operations,
	}
}

func TestMutateDraft_Happy(t *testing.T) {
	b, root := newASDTestBackend(t)
	result := b.MutateDraft(context.Background(), mustArgs(t, mutateDraftArgsFor(t, root, []map[string]any{
		{"op": "set-problem", "text": "mutated via MCP", "anchor": "#problem"},
	})))
	var out struct {
		ResultDigest string `json:"result_digest"`
		Changes      []struct {
			Target string `json:"target"`
			Change string `json:"change"`
		} `json:"changes"`
	}
	toolResultJSON(t, result, &out)
	if len(out.Changes) != 1 || out.Changes[0].Target != "problem" || out.Changes[0].Change != "replaced" {
		t.Fatalf("Changes = %+v", out.Changes)
	}
	if out.ResultDigest == "" {
		t.Fatal("ResultDigest is empty")
	}

	// The mutation must have actually landed and be visible via
	// get_design_provenance (proving MutateDraft's actor was minted as a
	// delegated agent, never a caller-supplied kind — CO-4/SI-163).
	prov := b.GetDesignProvenance(context.Background(), mustArgs(t, map[string]any{"ref": "spec/sample"}))
	var provOut struct {
		Entries []struct {
			Attribution struct {
				Unauthenticated bool `json:"unauthenticated"`
			} `json:"attribution"`
			Harness string `json:"harness"`
		} `json:"entries"`
	}
	toolResultJSON(t, prov, &provOut)
	if len(provOut.Entries) != 1 || !provOut.Entries[0].Attribution.Unauthenticated || provOut.Entries[0].Harness != "codex" {
		t.Fatalf("provenance entries = %+v, want one unauthenticated delegated-agent entry", provOut.Entries)
	}
}

func TestMutateDraft_Negative(t *testing.T) {
	ctx := context.Background()

	t.Run("malformed arguments", func(t *testing.T) {
		b, _ := newASDTestBackend(t)
		result := b.MutateDraft(ctx, mustArgs(t, map[string]any{"unknown_field": true}))
		if !isToolError(result) {
			t.Fatal("mutate_draft(malformed args): want isError")
		}
	})

	t.Run("missing harness", func(t *testing.T) {
		b, root := newASDTestBackend(t)
		args := mutateDraftArgsFor(t, root, []map[string]any{{"op": "set-problem", "text": "x", "anchor": "#problem"}})
		delete(args, "harness")
		result := b.MutateDraft(ctx, mustArgs(t, args))
		if !isToolError(result) {
			t.Fatal("mutate_draft(no harness): want isError")
		}
	})

	t.Run("stale base digest is a structured, marked error", func(t *testing.T) {
		b, root := newASDTestBackend(t)
		args := mutateDraftArgsFor(t, root, []map[string]any{{"op": "set-problem", "text": "x", "anchor": "#problem"}})
		// A self-consistent but STALE base (base_digest matches
		// base_spec_b64, neither matches the actual current spec bytes) —
		// draftmutation.DecodeRequest itself would reject a base_digest
		// that merely disagrees with its OWN base_spec_b64, so both must
		// change together to exercise the service's own stale-base check.
		stale := []byte(asdSampleSpec + "\nStale trailing content.\n")
		args["base_digest"] = draftmutation.DigestBytes(stale)
		args["base_spec_b64"] = base64.StdEncoding.EncodeToString(stale)
		result := b.MutateDraft(ctx, mustArgs(t, args))
		if !isToolError(result) {
			t.Fatal("mutate_draft(stale): want isError")
		}
		var out struct {
			Code          string `json:"code"`
			CurrentDigest string `json:"current_digest"`
		}
		if err := json.Unmarshal([]byte(toolResultText(t, result)), &out); err != nil {
			t.Fatalf("decoding stale refusal JSON: %v", err)
		}
		if out.Code != "stale-base" || out.CurrentDigest == "" {
			t.Fatalf("stale refusal = %+v, want the typed stale-base payload", out)
		}
	})

	// The 1 MiB ceiling must bind the RAW incoming arguments, not this
	// server's smaller canonical re-marshal of them. The oversize payload
	// rides in a field the wire schema accepts (an excerpt's text), so the
	// refusal can only come from the byte ceiling itself.
	t.Run("arguments over the request ceiling are refused", func(t *testing.T) {
		b, root := newASDTestBackend(t)
		args := mutateDraftArgsFor(t, root, []map[string]any{{"op": "set-problem", "text": "x", "anchor": "#problem"}})
		args["excerpts"] = []map[string]any{{
			"target": "problem", "classification": "human-stated",
			"representation": "verbatim", "text": strings.Repeat("a", draftmutation.MaxRequestBytes+1),
		}}
		raw := mustArgs(t, args)
		if len(raw) <= draftmutation.MaxRequestBytes {
			t.Fatalf("oversize fixture is only %d bytes", len(raw))
		}
		result := b.MutateDraft(ctx, raw)
		if !isToolError(result) {
			t.Fatal("mutate_draft(oversize arguments): want isError")
		}
		if text := toolResultText(t, result); !strings.Contains(text, "1 MiB") {
			t.Fatalf("mutate_draft(oversize arguments) = %q, want the request-ceiling refusal", text)
		}
	})

	t.Run("policy mode off refuses the delegated agent", func(t *testing.T) {
		b, root := offModeASDTestBackend(t)
		args := mutateDraftArgsFor(t, root, []map[string]any{{"op": "set-problem", "text": "x", "anchor": "#problem"}})
		result := b.MutateDraft(ctx, mustArgs(t, args))
		if !isToolError(result) {
			t.Fatal("mutate_draft(mode off): want isError")
		}
	})
}
