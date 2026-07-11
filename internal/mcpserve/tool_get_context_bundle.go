package mcpserve

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/storyresolve"
)

// contextItem is one resolved pinned ref in a get_context_bundle result.
type contextItem struct {
	Ref   string `json:"ref"`
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// GetContextBundle implements the get_context_bundle tool. Stub scope
// (PLAN.md Phase 9): resolves pinned refs to contents only — no
// transitive expansion, since 05 gives no deeper semantics for a
// manifest. Exactly one of refs (an explicit manifest) or spec (a feature
// spec ref whose context: field supplies the manifest) is required.
func (b *Backend) GetContextBundle(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	var args struct {
		Refs []string `json:"refs"`
		Spec string   `json:"spec"`
	}
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return toolError("get_context_bundle: malformed arguments: " + err.Error())
	}
	if len(args.Refs) == 0 && args.Spec == "" {
		return toolError("get_context_bundle: exactly one of refs or spec is required")
	}
	if len(args.Refs) > 0 && args.Spec != "" {
		return toolError("get_context_bundle: refs and spec are mutually exclusive")
	}

	refs := args.Refs
	if args.Spec != "" {
		specRef, err := artifact.ParseRef(args.Spec)
		if err != nil || specRef.Kind != artifact.KindSpec {
			return toolError(fmt.Sprintf("get_context_bundle: spec %q must be a spec ref (kind/name)", args.Spec))
		}
		spec, lerr := storyresolve.LoadActiveSpec(b.Root, specRef.Name)
		if lerr != nil {
			return toolError("get_context_bundle: " + lerr.Error())
		}
		refs = spec.Context
	}

	ix, err := b.buildIndex()
	if err != nil {
		return toolError("get_context_bundle: " + err.Error())
	}

	items := make([]contextItem, 0, len(refs))
	for _, r := range refs {
		pinned, perr := artifact.ParsePinnedRef(r)
		if perr != nil {
			return toolError(fmt.Sprintf("get_context_bundle: %q: %v", r, perr))
		}
		entry, gerr := ix.GetPinned(ctx, pinned)
		if gerr != nil {
			return toolError(fmt.Sprintf("get_context_bundle: %q: %v", r, gerr))
		}
		items = append(items, contextItem{Ref: entry.Ref, Kind: entry.Kind, Title: entry.Title, Body: entry.Body})
	}

	return toolJSON(map[string]any{"refs": refs, "items": items})
}
