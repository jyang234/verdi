// Package capture captures internal/dex's chrome golden: the docs-site
// build of ../store, written to ../site, for spec/chrome-and-tokens-v2
// ac-4 (obligation chrome-and-tokens-v2--ac-4--static).
//
// It lives under testdata so `go test ./...`, `make verify`, and CI never
// run it: the golden is captured ONCE, from the base code, before the
// story's first production change, and internal/dex's
// TestWorkbenchChromeLeavesDocsSiteUnchanged compares every later build
// with it byte for byte. That test has no update path of its own. Running
// this capture after a production change would record the change instead
// of proving its absence, so a re-capture is a reviewed change to the
// golden, never a way to make the comparison pass.
//
// Run, from verdi/, only at the capture commit:
//
//	go test -count=1 ./internal/dex/testdata/chromegolden/capture
//
// It builds the store exactly as the golden test does: one fixturegit
// layer of every file under ../store (fixed identity and date, so the
// commit SHA the site stamps is stable), then dex.Build with Root and
// OutDir only and the CI environment cleared.
package capture

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/dex"
	"github.com/jyang234/verdi/internal/fixturegit"
)

func TestCaptureChromeGolden(t *testing.T) {
	for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITHUB_BASE_REF", "CI_DEFAULT_BRANCH", "CI_MERGE_REQUEST_TARGET_BRANCH_NAME"} {
		t.Setenv(key, "")
	}
	files := map[string]string{}
	err := filepath.WalkDir("../store", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel("../store", path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("reading the fixture store: %v", err)
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "chrome golden fixture store"}})
	t.Logf("fixture HEAD %s", repo.Head)

	if err := os.RemoveAll("../site"); err != nil {
		t.Fatalf("clearing the previous golden: %v", err)
	}
	if err := dex.Build(context.Background(), dex.Options{Root: repo.Dir, OutDir: "../site"}); err != nil {
		t.Fatalf("dex.Build: %v", err)
	}
}
