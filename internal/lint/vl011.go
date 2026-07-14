package lint

import (
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// vl011 enforces "attestation/waiver/reaffirmation files live under the
// story/object they name ... feature outcome-attestations validate the
// same nested form" (02 §Lint rules, as amended for reaffirmation R4-I-4;
// I-6's compound-name convention: id name is "<story>--<ac-id>" (or
// "<feature-slug>--<ac-id>" for a feature outcome-attestation, or
// "<story>--<object-id>" for a reaffirmation), path is nested
// "<kind-dir>/<story-or-feature-slug>/<ac-or-object-id>.md"). The
// outcome-attestation and reaffirmation forms need no special-casing here
// (R4-I-11: "the mapping already generalizes") — both are still exactly
// the generic "<kind-dir>s/<left>/<right>.md" shape this rule already
// checks, driven only by Kind and the compound id split.
//
// spec/obligation-artifact DC-2 extends this rule's own shape one level
// deeper for the obligation kind: id is the THREE-segment
// "<story-slug>--<ac-id>--<for-kind>", and the path nests the same
// story-slug directory but the FILENAME is itself a second compound of the
// id's last two segments — ".verdi/obligations/<story-slug>/<ac-id>--<for-
// kind>.md" — since DecodeObligation (like DecodeAttestation) takes only
// raw frontmatter bytes and has no path to compare id against (that
// function's own doc comment: "leave path agreement to the lint walk").
// checkObligationPath is this rule's obligation-scoped sibling, kept
// separate from the loop above since its path shape genuinely differs
// (one extra segment folded into the filename, not the directory).
type vl011 struct{}

func (vl011) ID() string { return "VL-011" }

func (vl011) Check(in *RunInput) []Finding {
	var findings []Finding
	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil {
			continue
		}

		if d.Kind == "obligation" {
			findings = append(findings, checkObligationPath(d)...)
			continue
		}

		if d.Kind != "attestation" && d.Kind != "waiver" && d.Kind != "reaffirmation" {
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

// checkObligationPath is vl011's obligation-kind sibling (spec/
// obligation-artifact DC-2): an obligation's id
// "<story-slug>--<ac-id>--<for-kind>" implies on-disk path
// ".verdi/obligations/<story-slug>/<ac-id>--<for-kind>.md".
func checkObligationPath(d *Document) []Finding {
	ref, err := artifact.ParseRef(d.Base.ID)
	if err != nil {
		return nil // VL-002 already reports this
	}
	story, acID, forKind, ok := artifact.SplitObligationName(ref.Name)
	if !ok {
		return nil // shape already enforced at decode (obligationNameRe)
	}

	wantPath := fmt.Sprintf(".verdi/obligations/%s/%s--%s.md", story, acID, forKind)
	if d.RelPath != wantPath {
		return []Finding{{Rule: "VL-011", Path: d.RelPath, Message: fmt.Sprintf("id %q names story %q ac %q for_kind %q, which implies path %q, but this file lives at %q", d.Base.ID, story, acID, forKind, wantPath, d.RelPath)}}
	}
	return nil
}
