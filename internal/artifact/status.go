package artifact

import "sort"

// Status is a per-kind lifecycle status string (02 §Kind registry). Each
// kind (and, for specs, each class) has its own closed enum; unknown
// values fail closed.
type Status string

var (
	// specFeatureStatuses is the feature- and story-class status enum
	// (02 §Kind registry, as amended for round 5's terminal `superseded`).
	// `superseded` is a terminal status a predecessor spec reaches when its
	// successor is accepted (03 §The amendment ladder): `verdi accept` of a
	// spec carrying a `supersedes` edge flips the predecessor's status-only
	// line to `superseded` in the same ritual (cmd/verdi/accept.go), the
	// sole legal writer of the accepted-pending-build→superseded transition
	// (VL-004). A superseded spec keeps its `frozen:` stamp and stays in
	// specs/active/ — it is never re-editable and never re-buildable
	// (cmd/verdi/buildstart.go refuses it).
	specFeatureStatuses = map[Status]bool{
		"draft":                  true,
		"accepted-pending-build": true,
		"closed":                 true,
		"superseded":             true,
	}
	specComponentStatuses = map[Status]bool{
		"draft":      true,
		"active":     true,
		"superseded": true,
	}
	adrStatuses = map[Status]bool{
		"proposed":   true,
		"accepted":   true,
		"superseded": true,
	}
	diagramStatuses = map[Status]bool{
		"active":     true,
		"superseded": true,
	}
	// proposalStatuses is the class: proposal diagram's AUTHORED status
	// enum (02 §Diagram proposals, spec/proposal-artifact ac-1/dc-1):
	// "proposed -> accepted" only. realized/stale (the four-value
	// DISCLOSED vocabulary's two computed members, DiagramDisclosedStatus
	// in diagram.go) are deliberately ABSENT from this map — that absence
	// is itself the enforcement mechanism (ac-4/dc-3) making strict decode
	// refuse them as authored frontmatter, with no separate runtime guard
	// needed.
	proposalStatuses = map[Status]bool{
		"proposed": true,
		"accepted": true,
	}
	waiverStatuses = map[Status]bool{
		"active":  true,
		"expired": true,
	}
	conflictStatuses = map[Status]bool{
		"open":       true,
		"superseded": true,
		"dismissed":  true,
	}
)

// SpecFeatureStatuses returns the feature- and story-class status enum's
// members, sorted (internal/model's spec/model-schema ac-2 parity proof:
// the embedded canonical model's lifecycle states must equal this set
// exactly — via this exported accessor, never reflection on the private
// specFeatureStatuses map above, so the two can never silently drift).
func SpecFeatureStatuses() []string {
	out := make([]string, 0, len(specFeatureStatuses))
	for s := range specFeatureStatuses {
		out = append(out, string(s))
	}
	sort.Strings(out)
	return out
}
