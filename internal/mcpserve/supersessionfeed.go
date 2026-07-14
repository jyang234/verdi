// Adapts Backend.Forge over wallbadge.SupersessionCandidateLoader for
// get_board's pending-supersession wall badge (spec/badge-computes ac-3),
// mirroring commentfeed.go's identical reasoning: internal/workbench (and
// internal/wallbadge, the compute layer it calls) never imports
// internal/forge directly — this package already does (Backend.Forge,
// backend.go), so the adapter lives here.
package mcpserve

import (
	"context"

	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/wallbadge"
)

// backendSupersessionLoader adapts a forge.Forge over
// wallbadge.SupersessionCandidateLoader: it resolves this checkout's
// default branch fresh per call (branch state can change across a
// long-running process, exactly like backendCommentFeed's own per-call
// resolution) and, when resolvable, loads the confirmed open
// supersession candidates via evidence.LoadPendingSupersessionCandidates
// — the same exported entry point internal/dex/lens.go calls (co-3).
type backendSupersessionLoader struct {
	f    forge.Forge
	root string
}

// LoadCandidates implements wallbadge.SupersessionCandidateLoader. ok is
// false — never an error — when the default branch cannot be resolved:
// the disclosed-unproven case (ac-3), mirroring backendCommentFeed's own
// "nothing to mirror, never an error" posture for an unresolvable branch.
func (a backendSupersessionLoader) LoadCandidates(ctx context.Context, featureRef, specPath string) ([]evidence.OpenSupersessionCandidate, bool, error) {
	defaultBranch := lint.ResolveDefaultBranch(ctx, a.root)
	if defaultBranch == "" {
		return nil, false, nil
	}
	candidates, err := evidence.LoadPendingSupersessionCandidates(ctx, a.f, defaultBranch, featureRef, specPath)
	if err != nil {
		return nil, false, err
	}
	return candidates, true, nil
}

var _ wallbadge.SupersessionCandidateLoader = backendSupersessionLoader{}
