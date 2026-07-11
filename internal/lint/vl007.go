package lint

import "fmt"

// vl007 enforces "unknown entries directly under .verdi/ fail (D1)"
// (02 §Lint rules). Not in OQ-3's VL-001..VL-006 grandfather range, so it
// always applies.
type vl007 struct{}

func (vl007) ID() string { return "VL-007" }

func (vl007) Check(in *RunInput) []Finding {
	var findings []Finding
	for _, name := range in.Snapshot.TopLevelEntries {
		if !knownTopLevelEntries[name] {
			findings = append(findings, Finding{Rule: "VL-007", Path: ".verdi/" + name, Message: fmt.Sprintf("unrecognized top-level entry %q directly under .verdi/", name)})
		}
	}
	return findings
}
