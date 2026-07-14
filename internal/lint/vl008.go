package lint

import (
	"fmt"

	"github.com/jyang234/verdi/internal/store"
)

// vl008 enforces "generated provenance in committed zone ⇒ on
// lint.gated_generated allowlist OR frozen-stamped" (02 §Lint rules; 01
// §Temporal classes: "there is no third state").
type vl008 struct{}

func (vl008) ID() string { return "VL-008" }

func (vl008) Check(in *RunInput) []Finding {
	var findings []Finding
	allowlist := gatedGeneratedSet(in.Snapshot.Manifest)

	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Base.Provenance == nil {
			continue
		}
		if d.Base.Frozen != nil {
			continue
		}
		if allowlist[d.Base.ID] {
			continue
		}
		findings = append(findings, Finding{Rule: "VL-008", Path: d.RelPath, Message: fmt.Sprintf("%q carries generated provenance but is neither frozen-stamped nor on verdi.yaml's lint.gated_generated allowlist", d.Base.ID)})
	}
	return findings
}

func gatedGeneratedSet(m *store.Manifest) map[string]bool {
	set := map[string]bool{}
	if m == nil || m.Lint == nil {
		return set
	}
	for _, ref := range m.Lint.GatedGenerated {
		set[ref] = true
	}
	return set
}
