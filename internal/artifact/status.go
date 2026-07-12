package artifact

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
