package evidence

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
)

// CascadeStory is one story's edges into a feature that has since been
// superseded — the rung-4 cascade fold's per-story input (03 §The
// amendment ladder rung 4: "every story with edges into the feature gets a
// computed verdict").
type CascadeStory struct {
	SpecRef string
	// ObjectIDs are every predecessor-feature object id (an AC, constraint,
	// or decision id) this story's edges into that feature name — the
	// fragment target of its implements/resolves/exempts/depends-on edges.
	ObjectIDs []string
}

// CascadeVerdict is one story's rung-4 cascade verdict (03 §The amendment
// ladder rung 4).
type CascadeVerdict string

const (
	// CascadeUnaffected: every touched object is carried or
	// amended_advisory.
	CascadeUnaffected CascadeVerdict = "unaffected"
	// CascadeStale: at least one touched object is amended (and none
	// removed) — requires a re-affirmation record before the merge gate
	// and `verdi build start` proceed (V1-P4's job to enforce; this fold
	// only computes the verdict).
	CascadeStale CascadeVerdict = "stale"
	// CascadeInvalidated: at least one touched object is removed — a
	// dangling edge into content that no longer exists.
	CascadeInvalidated CascadeVerdict = "invalidated"
)

// CascadeResult is one story's cascade-fold outcome.
type CascadeResult struct {
	SpecRef string
	Verdict CascadeVerdict
	Amended []string // touched objects classified `amended` (the re-affirmation input: one per (story, amended object))
	Removed []string // touched objects classified `removed`
}

// FoldCascade implements 03 §The amendment ladder rung 4's "downstream
// impact is a fold, not a meeting": every story with edges into a
// superseded feature gets a computed unaffected/stale/invalidated verdict,
// derived from the superseding revision's `supersession:` manifest
// (artifact.Supersession, already decoded — VL-015's completeness and
// carried-byte-identity checks are V1-P2's job, not re-validated here).
//
// Precedence (03 gives the three buckets; the ordering when a story's
// edges span more than one bucket is this fold's own reduction, mirroring
// every other fold's total-precedence, worst-wins shape in this package):
// invalidated > stale > unaffected — one edge into a removed object marks
// the story invalidated regardless of how many other edges are merely
// carried.
//
// FoldCascade fails loudly when a story's edge names an object id that the
// supersession manifest does not classify at all (in any of
// carried/amended/amended_advisory/removed) — the same dangling-reference
// fail-closed posture as every other fold in this package; VL-015 already
// guarantees every PREDECESSOR object is classified exactly once, so a
// story naming an unclassified id can only mean the story's own edge is
// wrong (a typo, or an edge into an object the predecessor never declared).
func FoldCascade(supersession artifact.Supersession, stories []CascadeStory) ([]CascadeResult, error) {
	carried := toSet(supersession.Carried)
	amendedAdvisory := make(map[string]bool, len(supersession.AmendedAdvisory))
	for _, n := range supersession.AmendedAdvisory {
		amendedAdvisory[n.ID] = true
	}
	amended := make(map[string]bool, len(supersession.Amended))
	for _, n := range supersession.Amended {
		amended[n.ID] = true
	}
	removed := make(map[string]bool, len(supersession.Removed))
	for _, n := range supersession.Removed {
		removed[n.ID] = true
	}

	out := make([]CascadeResult, 0, len(stories))
	for _, s := range stories {
		res := CascadeResult{SpecRef: s.SpecRef, Verdict: CascadeUnaffected}
		for _, id := range s.ObjectIDs {
			switch {
			case removed[id]:
				res.Removed = append(res.Removed, id)
			case amended[id]:
				res.Amended = append(res.Amended, id)
			case carried[id], amendedAdvisory[id]:
				// unaffected; no bucket to record.
			default:
				return nil, fmt.Errorf("evidence: FoldCascade: story %s edge names object %q, which the supersession manifest does not classify in any bucket", s.SpecRef, id)
			}
		}
		switch {
		case len(res.Removed) > 0:
			res.Verdict = CascadeInvalidated
		case len(res.Amended) > 0:
			res.Verdict = CascadeStale
		default:
			res.Verdict = CascadeUnaffected
		}
		out = append(out, res)
	}
	return out, nil
}

func toSet(ids []string) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}
