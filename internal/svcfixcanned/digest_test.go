// Package svcfixcanned hermetically verifies
// testdata/svcfix-canned/digests.json's sha256 ratchet: `make fixture`
// checks every canned file's on-disk content against its committed digest
// with no exec and no network — regenerating the canned captures for real
// is `make fixture-regen`'s job (opt-in, non-hermetic), never this one's
// (PLAN.md §4: "make fixture verifies digests hermetically").
package svcfixcanned

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

const cannedDir = "../../testdata/svcfix-canned"

type digestsFile struct {
	Schema string            `json:"schema"`
	Files  map[string]string `json:"files"`
}

func loadDigests(t *testing.T) digestsFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(cannedDir, "digests.json"))
	if err != nil {
		t.Fatalf("reading digests.json: %v", err)
	}
	var d digestsFile
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatalf("decoding digests.json: %v", err)
	}
	return d
}

func sha256Hex(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// TestDigestsRatchet_MatchesOnDiskContent proves every file digests.json
// names has exactly the committed sha256 (a bit-for-bit tamper/drift
// check on the canned upstream captures).
func TestDigestsRatchet_MatchesOnDiskContent(t *testing.T) {
	d := loadDigests(t)
	if d.Schema != "verdi.fixture-digests/v1" {
		t.Fatalf("digests.json schema = %q, want verdi.fixture-digests/v1", d.Schema)
	}
	if len(d.Files) == 0 {
		t.Fatal("digests.json lists no files")
	}

	names := make([]string, 0, len(d.Files))
	for name := range d.Files {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		want := d.Files[name]
		got := sha256Hex(t, filepath.Join(cannedDir, name))
		if got != want {
			t.Errorf("%s: digest = %s, want %s (ratchet mismatch: content drifted since the last commit — see README.md's fixture-regen instructions)", name, got, want)
		}
	}
}

// TestDigestsRatchet_CoversEveryCannedFile proves digests.json is not
// stale in the other direction: every JSON file actually present in
// testdata/svcfix-canned/ (except digests.json and bundle-golden/, a
// derived-output fixture with its own consumers, not an upstream capture)
// is named in the ratchet.
func TestDigestsRatchet_CoversEveryCannedFile(t *testing.T) {
	d := loadDigests(t)

	entries, err := os.ReadDir(cannedDir)
	if err != nil {
		t.Fatalf("reading %s: %v", cannedDir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "digests.json" || name == "README.md" {
			continue
		}
		if _, ok := d.Files[name]; !ok {
			t.Errorf("%s exists on disk but is not covered by digests.json's ratchet", name)
		}
	}
}

func TestDigestsRatchet_Negative_TamperedContentFails(t *testing.T) {
	// Not a mutation of the committed fixture — proves the comparison
	// logic itself would catch a mismatch, using a throwaway temp file.
	dir := t.TempDir()
	path := filepath.Join(dir, "x.json")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	original := sha256Hex(t, path)

	if err := os.WriteFile(path, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	tampered := sha256Hex(t, path)

	if original == tampered {
		t.Fatal("sha256Hex did not change after tampering the file content")
	}
}
