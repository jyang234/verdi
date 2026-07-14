// The real forge adapter over wallbadge.SupersessionCandidateLoader
// (spec/badge-computes ac-3): the pending-supersession wall badge's forge
// access. Lives in cmd/verdi (not internal/workbench or
// internal/wallbadge) for the exact reason reviewfeed.go's doc comment
// gives for forgeCommentFeed: it keeps internal/forge out of both
// packages, the dependency direction the port pattern wants.
package main

import (
	"context"

	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/wallbadge"
)

// forgeSupersessionLoader adapts a forge.Forge over
// wallbadge.SupersessionCandidateLoader for the checkout rooted at root.
type forgeSupersessionLoader struct {
	f    forge.Forge
	root string
}

// newForgeSupersessionLoader wraps f for the checkout rooted at root.
func newForgeSupersessionLoader(f forge.Forge, root string) *forgeSupersessionLoader {
	return &forgeSupersessionLoader{f: f, root: root}
}

// LoadCandidates implements wallbadge.SupersessionCandidateLoader,
// resolving this checkout's default branch fresh per call — branch state
// can change across `verdi serve`'s lifetime — and loading confirmed
// candidates via evidence.LoadPendingSupersessionCandidates, the same
// exported entry point internal/dex/lens.go calls (co-3). ok is false —
// never an error — when the default branch cannot be resolved: the
// disclosed-unproven case (ac-3), mirroring forgeCommentFeed's own
// "nothing to mirror, never an error" posture.
func (a *forgeSupersessionLoader) LoadCandidates(ctx context.Context, featureRef, specPath string) ([]evidence.OpenSupersessionCandidate, bool, error) {
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

var _ wallbadge.SupersessionCandidateLoader = (*forgeSupersessionLoader)(nil)
