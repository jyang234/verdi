package lint

import (
	"fmt"

	"github.com/OWNER/verdi/internal/gitx"
)

// vl009 enforces "frozen artifacts carry valid frozen stamp and provenance
// where generated" (02 §Lint rules): the stamp's own shape (Frozen.Validate,
// which VL-001's DecodeStrict-only scope does not check — see doc.go's
// design note) plus, beyond what any Go-level shape check can see, that
// the stamped commit is real git history.
type vl009 struct{}

func (vl009) ID() string { return "VL-009" }

func (vl009) Check(in *RunInput) []Finding {
	var findings []Finding
	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Base.Frozen == nil {
			continue
		}

		if err := d.Base.Frozen.Validate(); err != nil {
			findings = append(findings, Finding{Rule: "VL-009", Path: d.RelPath, Message: err.Error()})
		} else {
			ok, err := gitx.CommitExists(in.Ctx, in.Root, d.Base.Frozen.Commit)
			if err != nil {
				findings = append(findings, Finding{Rule: "VL-009", Path: d.RelPath, Message: fmt.Sprintf("checking frozen.commit %s: %v", d.Base.Frozen.Commit, err)})
			} else if !ok {
				findings = append(findings, Finding{Rule: "VL-009", Path: d.RelPath, Message: fmt.Sprintf("frozen.commit %s is not a real commit in this repository's history", d.Base.Frozen.Commit)})
			}
		}

		if d.Base.Provenance != nil {
			if err := d.Base.Provenance.Validate(); err != nil {
				findings = append(findings, Finding{Rule: "VL-009", Path: d.RelPath, Message: fmt.Sprintf("frozen and generated, but provenance is invalid: %v", err)})
			}
		}
	}
	return findings
}
