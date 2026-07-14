package wallbadge

import (
	"context"

	"github.com/jyang234/verdi/internal/evidence"
)

// SupersessionCandidateLoader is the pending-supersession ladder badge's
// forge access — a consumer-defined port (04 §port pattern), mirroring
// internal/workbench.CommentFeed's own shape and the same reasoning
// cmd/verdi/reviewfeed.go's doc comment states: the concrete forge.Forge-
// backed adapter is wired by cmd/verdi (and internal/mcpserve, for
// get_board), keeping internal/forge out of both this package and
// internal/workbench.
//
// LoadCandidates returns the confirmed open supersession candidates for
// featureRef (evidence.LoadPendingSupersessionCandidates's own result
// shape) at the conventional candidate path specPath. ok is false when
// open MRs could not be enumerated at all — no forge configured, or the
// checkout's default branch could not be resolved — which is the
// disclosed-unproven case (ac-3): never an error, and never silently
// "no candidates found" (which would misreport as proven-unflagged).
type SupersessionCandidateLoader interface {
	LoadCandidates(ctx context.Context, featureRef, specPath string) (candidates []evidence.OpenSupersessionCandidate, ok bool, err error)
}
