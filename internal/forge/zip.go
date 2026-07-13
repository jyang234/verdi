package forge

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path"
	"strings"
)

// bundleFileNames are the derived bundle files a verdi-evidence CI artifact
// carries (01 §Directory layout's derived tree). Only entries whose base
// name is one of these are preserved from the artifact zip; regenerated
// views (derived/<key>/<commit>/views/…) and any other incidental file are
// ignored, keeping the fetched-and-written tree bounded to what a fold or
// sync's own evaluation reads.
var bundleFileNames = map[string]bool{
	"verdicts.json":      true,
	"tests.json":         true,
	"review.json":        true,
	"boundary-diff.json": true,
}

// ExtractTreeFromZip reads a verdi-evidence artifact zip (shared by the
// gitlab and github adapters, since verdi's own CI templates produce the
// same archive shape on both forges) and returns its full DerivedTree: each
// recognized bundle file's raw content keyed by its path relative to
// data/derived/.
//
// The artifact is the WHOLE derived/ subtree CI uploaded for one
// (ref, commit) run (verify.yml: `path: .verdi/data/derived/`), so it
// legitimately carries MORE THAN ONE verdicts.json — one per per-spec
// subdir selfevidence.go wrote, plus the branch-keyed per-service bundle.
// Preserving each at its own key (rather than collapsing to a single
// four-file bundle, the pre-fix behavior that both dropped the per-spec
// subdirs and errored on the duplicate) is exactly what lets a fetched
// bundle reach the fold's per-spec readers.
//
// Keys are normalized to be relative to derived/: a leading "derived/"
// path prefix (present when CI zips the parent of derived/, absent when it
// zips derived/'s contents) is stripped so the returned key is always the
// <key>/<commit>/<file> form readers and sync write under
// .verdi/data/derived/. Entries that would escape derived/ (absolute paths
// or any ".." segment — a zip-slip attempt) are rejected loudly. An archive
// carrying no recognized bundle file at all is an error: a verdi-evidence
// artifact always has at least one verdicts.json.
func ExtractTreeFromZip(data []byte) (DerivedTree, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("forge: reading evidence bundle zip: %w", err)
	}

	tree := make(DerivedTree)
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !bundleFileNames[path.Base(f.Name)] {
			continue
		}
		key, err := derivedRelKey(f.Name)
		if err != nil {
			return nil, err
		}
		if _, dup := tree[key]; dup {
			return nil, fmt.Errorf("forge: evidence bundle zip contains more than one entry for key %q", key)
		}
		content, err := readZipFile(f)
		if err != nil {
			return nil, fmt.Errorf("forge: reading %s from evidence bundle zip: %w", f.Name, err)
		}
		tree[key] = content
	}

	if len(tree) == 0 {
		return nil, fmt.Errorf("forge: evidence bundle zip carries no recognized derived file (verdicts.json/tests.json/review.json/boundary-diff.json)")
	}
	return tree, nil
}

// derivedRelKey normalizes a zip entry name to a store-relative derived key
// (relative to data/derived/), rejecting any path that would escape that
// directory.
func derivedRelKey(name string) (string, error) {
	// Zip paths are conventionally slash-separated (archive/zip); normalize
	// any backslash first. Reject any ".." segment in the RAW entry path
	// before path.Clean can silently collapse it into an in-bounds path —
	// a traversal attempt must fail loudly, never be quietly contained.
	slashed := strings.ReplaceAll(name, "\\", "/")
	for _, seg := range strings.Split(slashed, "/") {
		if seg == ".." {
			return "", fmt.Errorf("forge: evidence bundle zip entry %q escapes derived/ (rejected)", name)
		}
	}
	// Drop any "./" noise (path.Clean), then a leading slash and a single
	// leading "derived/" prefix, so the key is always relative to derived/.
	clean := path.Clean(slashed)
	clean = strings.TrimPrefix(clean, "/")
	clean = strings.TrimPrefix(clean, "derived/")
	if clean == "" || clean == "." {
		return "", fmt.Errorf("forge: evidence bundle zip entry %q has no path under derived/", name)
	}
	return clean, nil
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
