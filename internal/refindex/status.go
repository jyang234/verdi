package refindex

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
)

// mapStatusGroup maps a default-branch spec's raw frontmatter status field
// to feature dc-2's four-value StatusGroup vocabulary, through a total
// function that fails closed (returns an error, never a silent default
// bucket) for a status value it does not recognize — CLAUDE.md: "unknown
// enum values fail closed" (ac-3's static obligation). It covers every
// status value legal on a spec that can actually live on the default
// branch's committed tree: story/feature's {accepted-pending-build, closed,
// superseded} (03 §Lifecycle: merging a feature/story spec's MR IS
// acceptance, so a story/feature spec never lands on the default branch
// still carrying status: draft) and component's {draft, active,
// superseded} (component specs are authored-living, never frozen, and
// legitimately carry draft or active on the default branch).
func mapStatusGroup(status artifact.Status) (StatusGroup, error) {
	switch status {
	case "draft":
		return StatusGroupDraftsInProgress, nil
	case "accepted-pending-build":
		return StatusGroupAcceptedPendingBuild, nil
	case "active":
		return StatusGroupActiveComponents, nil
	case "closed", "superseded":
		return StatusGroupTerminal, nil
	default:
		return "", fmt.Errorf("refindex: spec status %q does not map to any known StatusGroup (fail-closed)", status)
	}
}
