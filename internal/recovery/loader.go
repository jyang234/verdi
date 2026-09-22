package recovery

import (
	"context"
	"fmt"

	"github.com/jyang234/verdi/internal/store"
)

// Loader is the production RecoveryLoader every surface wires — MCP's
// get_recovery tool (internal/mcpserve/tool_get_recovery.go, mirroring
// its ReadinessLoader/readinessload.Loader precedent) and cmd/verdi's
// `verdi recover` read path (recover.go): store.Open, then Gather, then
// Derive, fresh on every call. Nothing here caches: Derive is pure and
// Gather always re-reads live git/store facts, so a cached result would
// only ever be its own stale copy of the truth co-2 forbids the
// projection from becoming.
type Loader struct {
	Root string
}

// Load derives ref's recovery projection fresh from l.Root.
func (l Loader) Load(ctx context.Context, ref string) (Projection, error) {
	cfg, err := store.Open(l.Root)
	if err != nil {
		return Projection{}, fmt.Errorf("recovery: load: %w", err)
	}
	f, err := NewGatherer().Gather(ctx, cfg, ref)
	if err != nil {
		// Gather's own errors already self-prefix "recovery: gather: ...".
		return Projection{}, err
	}
	return Derive(f), nil
}
