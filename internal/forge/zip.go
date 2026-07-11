package forge

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path"
)

// bundleFileNames are the four files a verdi-evidence CI artifact must
// contain (01 §Directory layout's derived tree), matched by base name
// regardless of the directory prefix the archive stores them under —
// verdi's own CI templates zip the derived/<ref-slug>/<commit>/ directory
// as-is, but this package does not need to recompute the ref slug to find
// them.
var bundleFileNames = []string{"verdicts.json", "tests.json", "review.json", "boundary-diff.json"}

// ExtractBundleFromZip reads a verdi-evidence artifact zip (shared by the
// gitlab and github adapters, since verdi's own CI templates produce the
// same archive shape on both forges) and returns the four bundle files'
// raw content. It fails loudly if any of the four is missing or
// duplicated — a partial or malformed artifact is never silently
// accepted.
func ExtractBundleFromZip(data []byte) (*EvidenceBundle, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("forge: reading evidence bundle zip: %w", err)
	}

	var bundle EvidenceBundle
	targets := map[string]*[]byte{
		"verdicts.json":      &bundle.Verdicts,
		"tests.json":         &bundle.Tests,
		"review.json":        &bundle.Review,
		"boundary-diff.json": &bundle.BoundaryDiff,
	}
	found := make(map[string]bool, len(bundleFileNames))

	for _, f := range r.File {
		base := path.Base(f.Name)
		target, ok := targets[base]
		if !ok {
			continue
		}
		if found[base] {
			return nil, fmt.Errorf("forge: evidence bundle zip contains more than one %s", base)
		}
		content, err := readZipFile(f)
		if err != nil {
			return nil, fmt.Errorf("forge: reading %s from evidence bundle zip: %w", base, err)
		}
		*target = content
		found[base] = true
	}

	for _, name := range bundleFileNames {
		if !found[name] {
			return nil, fmt.Errorf("forge: evidence bundle zip is missing %s", name)
		}
	}
	return &bundle, nil
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
