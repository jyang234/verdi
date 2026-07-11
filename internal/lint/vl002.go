package lint

import (
	"fmt"
	"path"
	"strings"

	"github.com/OWNER/verdi/internal/artifact"
)

// vl002 enforces "id/path agreement; global ref uniqueness. Status-in-path
// applies to the feature class only: superseded component specs remain in
// specs/active/" (02 §Lint rules).
type vl002 struct{}

func (vl002) ID() string { return "VL-002" }

// singleFileKindDir maps a single-file kind to its 01 §Directory layout
// directory name and file extension. Attestation/waiver are deliberately
// absent: their path is the nested "<kind-dir>/<story>/<ac-id>.md" shape,
// not "<kind-dir>/<name>.<ext>" — VL-011 owns their path/id agreement.
var singleFileKindDir = map[string]struct{ dir, ext string }{
	"adr":      {"adr", ".md"},
	"diagram":  {"diagrams", ".mermaid"},
	"conflict": {"conflicts", ".md"},
}

func (vl002) Check(in *RunInput) []Finding {
	var findings []Finding

	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil {
			continue
		}

		ref, err := artifact.ParseRef(d.Base.ID)
		if err != nil {
			findings = append(findings, Finding{Rule: "VL-002", Path: d.RelPath, Message: fmt.Sprintf("id %q does not parse as a valid ref: %v", d.Base.ID, err)})
			continue
		}
		if string(d.Base.Kind) != d.Kind {
			findings = append(findings, Finding{Rule: "VL-002", Path: d.RelPath, Message: fmt.Sprintf("frontmatter kind %q does not agree with the directory this file lives in (expected %q)", d.Base.Kind, d.Kind)})
		}

		switch d.Kind {
		case "spec":
			findings = append(findings, checkSpecPath(d, ref)...)
		case "attestation", "waiver":
			// Path/id agreement for the nested story/ac-id shape is VL-011's
			// job; VL-002 has already checked id parses and kind agrees.
		default:
			if want, ok := singleFileKindDir[d.Kind]; ok {
				wantPath := fmt.Sprintf(".verdi/%s/%s%s", want.dir, ref.Name, want.ext)
				if d.RelPath != wantPath {
					findings = append(findings, Finding{Rule: "VL-002", Path: d.RelPath, Message: fmt.Sprintf("id %q implies path %q, but this file lives at %q", d.Base.ID, wantPath, d.RelPath)})
				}
			}
		}
	}

	for ref, docs := range in.Snapshot.ByRef {
		if len(docs) < 2 {
			continue
		}
		for _, d := range docs {
			findings = append(findings, Finding{Rule: "VL-002", Path: d.RelPath, Message: fmt.Sprintf("ref %q is declared by more than one file (also %s)", ref, otherPaths(docs, d))})
		}
	}

	return findings
}

// otherPaths lists every doc in docs other than exclude, for a duplicate-
// ref finding's message.
func otherPaths(docs []*Document, exclude *Document) string {
	var others []string
	for _, d := range docs {
		if d != exclude {
			others = append(others, d.RelPath)
		}
	}
	return strings.Join(others, ", ")
}

// checkSpecPath checks a spec document's directory name against its id's
// name, and its status-dir (active/archive) against 02's class-scoped
// rule: feature specs move to archive/ once closed; component specs
// always stay in active/, even superseded (02 §Kind registry).
func checkSpecPath(d *Document, ref artifact.Ref) []Finding {
	var findings []Finding

	dir := specDirOf(d)
	dirName := path.Base(dir)
	if dirName != ref.Name {
		findings = append(findings, Finding{Rule: "VL-002", Path: d.RelPath, Message: fmt.Sprintf("id %q disagrees with containing directory %q", d.Base.ID, dirName)})
	}

	statusDir := "active"
	if strings.HasPrefix(dir, ".verdi/specs/archive/") {
		statusDir = "archive"
	}

	wantStatusDir := "active"
	if d.Spec != nil && d.Spec.Class == artifact.ClassFeature && d.Status == "closed" {
		wantStatusDir = "archive"
	}
	if statusDir != wantStatusDir {
		findings = append(findings, Finding{Rule: "VL-002", Path: d.RelPath, Message: fmt.Sprintf("spec status %q (class %s) belongs under specs/%s/, not specs/%s/", d.Status, d.Spec.Class, wantStatusDir, statusDir)})
	}

	return findings
}
