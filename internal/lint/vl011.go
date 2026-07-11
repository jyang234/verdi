package lint

import (
	"fmt"
	"strings"

	"github.com/OWNER/verdi/internal/artifact"
)

// vl011 enforces "attestation/waiver files live under the story/AC they
// name; waiver has owner + reason, expiry optional" (02 §Lint rules; I-6's
// compound-name convention: id name is "<story>--<ac-id>", path is nested
// "<kind-dir>/<story>/<ac-id>.md").
type vl011 struct{}

func (vl011) ID() string { return "VL-011" }

func (vl011) Check(in *RunInput) []Finding {
	var findings []Finding
	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil {
			continue
		}
		if d.Kind != "attestation" && d.Kind != "waiver" {
			continue
		}

		ref, err := artifact.ParseRef(d.Base.ID)
		if err != nil {
			continue // VL-002 already reports this
		}
		story, acID, ok := strings.Cut(ref.Name, "--")
		if !ok {
			continue // shape already enforced at decode (I-6 compoundNameRe)
		}

		dir := d.Kind + "s"
		wantPath := fmt.Sprintf(".verdi/%s/%s/%s.md", dir, story, acID)
		if d.RelPath != wantPath {
			findings = append(findings, Finding{Rule: "VL-011", Path: d.RelPath, Message: fmt.Sprintf("id %q names story %q ac %q, which implies path %q, but this file lives at %q", d.Base.ID, story, acID, wantPath, d.RelPath)})
		}

		if d.Kind == "waiver" && d.Waiver != nil {
			if len(d.Waiver.Owners) == 0 {
				findings = append(findings, Finding{Rule: "VL-011", Path: d.RelPath, Message: "waiver has no owner"})
			}
			if d.Waiver.Reason == "" {
				findings = append(findings, Finding{Rule: "VL-011", Path: d.RelPath, Message: "waiver has no reason"})
			}
		}
	}
	return findings
}
