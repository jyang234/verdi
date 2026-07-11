package artifact

// Status is a per-kind lifecycle status string (02 §Kind registry). Each
// kind (and, for specs, each class) has its own closed enum; unknown
// values fail closed.
type Status string

var (
	specFeatureStatuses = map[Status]bool{
		"draft":                  true,
		"accepted-pending-build": true,
		"closed":                 true,
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
