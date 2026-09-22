package mcpserve

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/recovery"
)

type getRecoveryArgs struct {
	Ref string `json:"ref"`
}

// GetRecovery is the read-only MCP tool over the readiness-recovery
// projection (spec/readiness-recovery-v2 ac-8, ac-10's MCP half): it
// takes exactly one argument (ref) and returns the same canonical
// projection object `verdi recover`'s read path prints — never an
// apply/choice argument, so nothing this tool call can express ever
// reaches either executor (co-3), structurally rather than merely by
// convention.
func (b *Backend) GetRecovery(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	var args getRecoveryArgs
	if err := strictUnmarshal(argsRaw, &args); err != nil {
		return toolError("get_recovery: " + err.Error())
	}
	if args.Ref == "" {
		return toolError("get_recovery: ref is required")
	}
	ref, err := artifact.ParseRef(args.Ref)
	if err != nil || ref.Kind != artifact.KindSpec {
		return toolError(fmt.Sprintf("get_recovery: %q is not a spec/<name> ref", args.Ref))
	}
	if b.RecoveryLoader == nil {
		return toolError("get_recovery: recovery projection not supplied")
	}
	proj, err := b.RecoveryLoader.Load(ctx, args.Ref)
	if err != nil {
		return toolError("get_recovery: " + err.Error())
	}
	data, err := recovery.Canonical(proj)
	if err != nil {
		return toolError("get_recovery: " + err.Error())
	}
	return toolText(string(data))
}
