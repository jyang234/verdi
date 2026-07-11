package lint

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OWNER/verdi/internal/artifact"
)

// knownTopLevelEntries is D1's fixed set of names legal directly under
// .verdi/ (01 §Directory layout), plus verdi.yaml/.gitignore/bin per
// VL-007's own enumeration in testdata/violations/README.md.
var knownTopLevelEntries = map[string]bool{
	"verdi.yaml":   true,
	".gitignore":   true,
	"specs":        true,
	"adr":          true,
	"diagrams":     true,
	"attestations": true,
	"waivers":      true,
	"conflicts":    true,
	"bin":          true,
	"data":         true,
}

// classifyArtifactPath maps a .verdi/-relative slash path to the artifact
// kind it should decode as, mirroring internal/index/walk.go's
// classifyArtifactPath (duplicated rather than imported: that function is
// unexported, and lint's tolerant walk needs different failure handling —
// it never aborts on a single bad file).
func classifyArtifactPath(rel string) (kind string, ok bool) {
	switch {
	case strings.HasPrefix(rel, "adr/") && strings.HasSuffix(rel, ".md"):
		return "adr", true
	case strings.HasPrefix(rel, "diagrams/") && strings.HasSuffix(rel, ".mermaid"):
		return "diagram", true
	case strings.HasPrefix(rel, "attestations/") && strings.HasSuffix(rel, ".md"):
		return "attestation", true
	case strings.HasPrefix(rel, "waivers/") && strings.HasSuffix(rel, ".md"):
		return "waiver", true
	case strings.HasPrefix(rel, "conflicts/") && strings.HasSuffix(rel, ".md"):
		return "conflict", true
	case (strings.HasPrefix(rel, "specs/active/") || strings.HasPrefix(rel, "specs/archive/")) &&
		strings.HasSuffix(rel, "/spec.md"):
		return "spec", true
	default:
		return "", false
	}
}

// walkDocuments walks root/.verdi (skipping data/) and tolerantly decodes
// every classified artifact file into a Document, sorted by RelPath for
// deterministic output.
func walkDocuments(root string, opts Options) ([]*Document, error) {
	verdiDir := filepath.Join(root, ".verdi")

	var docs []*Document
	err := filepath.WalkDir(verdiDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if filepath.Dir(path) == verdiDir && d.Name() == "data" {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(verdiDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		kind, ok := classifyArtifactPath(rel)
		if !ok {
			return nil
		}

		relFromRoot := ".verdi/" + rel
		grandfathered := opts.GrandfatherArchive && strings.HasPrefix(rel, "specs/archive/")

		doc := &Document{Kind: kind, Path: path, RelPath: relFromRoot, Grandfathered: grandfathered}
		decodeDocument(doc)
		docs = append(docs, doc)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("lint: walking %s: %w", verdiDir, err)
	}

	sort.Slice(docs, func(i, j int) bool { return docs[i].RelPath < docs[j].RelPath })
	return docs, nil
}

// decodeDocument reads doc.Path and tolerantly decodes it: SplitFrontmatter
// or DecodeStrict failure sets DecodeErr (VL-001's finding) and leaves
// every other field zero; success populates Base/Status/Body and the one
// kind-specific pointer matching doc.Kind.
func decodeDocument(doc *Document) {
	data, err := os.ReadFile(doc.Path)
	if err != nil {
		doc.DecodeErr = err
		return
	}

	fm, body, err := artifact.SplitFrontmatter(data)
	if err != nil {
		doc.DecodeErr = err
		return
	}
	doc.Body = string(body)

	switch doc.Kind {
	case "spec":
		var fmv artifact.SpecFrontmatter
		if err := artifact.DecodeStrict(fm, &fmv); err != nil {
			doc.DecodeErr = err
			return
		}
		doc.Spec = &fmv
		doc.Base, doc.Status = fmv.Base, string(fmv.Status)

	case "adr":
		var fmv artifact.ADRFrontmatter
		if err := artifact.DecodeStrict(fm, &fmv); err != nil {
			doc.DecodeErr = err
			return
		}
		doc.ADR = &fmv
		doc.Base, doc.Status = fmv.Base, string(fmv.Status)

	case "diagram":
		var fmv artifact.DiagramFrontmatter
		if err := artifact.DecodeStrict(fm, &fmv); err != nil {
			doc.DecodeErr = err
			return
		}
		doc.Diagram = &fmv
		doc.Base, doc.Status = fmv.Base, string(fmv.Status)

	case "attestation":
		var fmv artifact.AttestationFrontmatter
		if err := artifact.DecodeStrict(fm, &fmv); err != nil {
			doc.DecodeErr = err
			return
		}
		doc.Attestation = &fmv
		doc.Base = fmv.Base

	case "waiver":
		var fmv artifact.WaiverFrontmatter
		if err := artifact.DecodeStrict(fm, &fmv); err != nil {
			doc.DecodeErr = err
			return
		}
		doc.Waiver = &fmv
		doc.Base, doc.Status = fmv.Base, string(fmv.Status)

	case "conflict":
		var fmv artifact.ConflictFrontmatter
		if err := artifact.DecodeStrict(fm, &fmv); err != nil {
			doc.DecodeErr = err
			return
		}
		doc.Conflict = &fmv
		doc.Base, doc.Status = fmv.Base, string(fmv.Status)

	default:
		doc.DecodeErr = fmt.Errorf("lint: unreachable: unhandled kind %q", doc.Kind)
	}
}

// topLevelEntries lists the names directly under root/.verdi (VL-007).
func topLevelEntries(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, ".verdi"))
	if err != nil {
		return nil, fmt.Errorf("lint: reading .verdi: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}
