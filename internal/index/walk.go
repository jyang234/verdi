package index

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/artifact"
)

// walkCommittedZone visits every artifact file under root/.verdi/ per 02's
// path derivation (kind dirs; specs as directories under
// specs/{active,archive}), skips data/ and non-artifact companion files
// (board.json, rollup.json, deviation-report.md, verdi.yaml, .gitignore),
// and decodes each artifact through internal/artifact.
func walkCommittedZone(root string) ([]*Entry, error) {
	verdiDir := filepath.Join(root, ".verdi")

	var entries []*Entry
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
			return nil // not an indexed artifact file
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entry, err := decodeEntry(kind, data, path)
		if err != nil {
			return fmt.Errorf("index: decoding %s: %w", rel, err)
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("index: walking %s: %w", verdiDir, err)
	}
	return entries, nil
}

// classifyArtifactPath maps a .verdi/-relative path to the artifact kind
// it decodes as. It is a thin call into the shared
// internal/artifact.ClassifyPath table (spec/shared-homes ac-4, dc-3) —
// lint/walk.go's classifyArtifactPath calls the same table. It returns
// ok=false for every file that is not itself an indexed artifact:
// verdi.yaml, .gitignore, and
// specs/*/*/{board.json,rollup.json,deviation-report.md}.
func classifyArtifactPath(rel string) (kind string, ok bool) {
	return artifact.ClassifyPath(rel)
}

// decodeEntry dispatches to the right internal/artifact decoder for kind
// and builds the resulting Entry, using the frontmatter's own `id:` field
// as Ref (01 §D2).
func decodeEntry(kind string, data []byte, path string) (*Entry, error) {
	fm, body, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return nil, err
	}

	switch kind {
	case "spec":
		s, err := artifact.DecodeSpec(fm)
		if err != nil {
			return nil, err
		}
		return &Entry{Ref: s.ID, Kind: kind, Title: s.Title, Status: string(s.Status), Path: path, Body: string(body), Links: s.Links}, nil

	case "adr":
		a, err := artifact.DecodeADR(fm)
		if err != nil {
			return nil, err
		}
		return &Entry{Ref: a.ID, Kind: kind, Title: a.Title, Status: string(a.Status), Path: path, Body: string(body), Links: a.Links}, nil

	case "diagram":
		d, err := artifact.DecodeDiagram(fm)
		if err != nil {
			return nil, err
		}
		return &Entry{Ref: d.ID, Kind: kind, Title: d.Title, Status: string(d.Status), Path: path, Body: string(body), Links: d.Links}, nil

	case "attestation":
		at, err := artifact.DecodeAttestation(fm)
		if err != nil {
			return nil, err
		}
		return &Entry{Ref: at.ID, Kind: kind, Title: at.Title, Path: path, Body: string(body), Links: at.Links}, nil

	case "waiver":
		w, err := artifact.DecodeWaiver(fm)
		if err != nil {
			return nil, err
		}
		return &Entry{Ref: w.ID, Kind: kind, Title: w.Title, Status: string(w.Status), Path: path, Body: string(body), Links: w.Links}, nil

	case "conflict":
		c, err := artifact.DecodeConflict(fm)
		if err != nil {
			return nil, err
		}
		return &Entry{Ref: c.ID, Kind: kind, Title: c.Title, Status: string(c.Status), Path: path, Body: string(body), Links: c.Links}, nil

	case "reaffirmation":
		r, err := artifact.DecodeReaffirmation(fm)
		if err != nil {
			return nil, err
		}
		return &Entry{Ref: r.ID, Kind: kind, Title: r.Title, Path: path, Body: string(body), Links: r.Links}, nil

	case "obligation":
		o, err := artifact.DecodeObligation(fm)
		if err != nil {
			return nil, err
		}
		return &Entry{Ref: o.ID, Kind: kind, Title: o.Title, Path: path, Body: string(body), Links: o.Links}, nil

	default:
		return nil, fmt.Errorf("index: unreachable: unhandled kind %q", kind)
	}
}
