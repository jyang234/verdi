package lint

import "fmt"

// vl006 enforces "every AC declares ≥1 expected evidence kind (activation
// lint)" (02 §Lint rules), reading the raw decoded AC list directly (see
// doc.go's design note: this is why the overlay's empty-evidence AC
// decodes successfully under VL-001 yet still fires here).
type vl006 struct{}

func (vl006) ID() string { return "VL-006" }

func (vl006) Check(in *RunInput) []Finding {
	var findings []Finding
	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Spec == nil {
			continue
		}
		for _, ac := range d.Spec.AcceptanceCriteria {
			if len(ac.Evidence) == 0 {
				findings = append(findings, Finding{Rule: "VL-006", Path: d.RelPath, Message: fmt.Sprintf("acceptance criterion %s declares no expected evidence kind", ac.ID)})
			}
		}
	}
	return findings
}
