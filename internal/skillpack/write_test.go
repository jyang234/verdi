package skillpack

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteThenCheckClean(t *testing.T) {
	root := t.TempDir()
	res, err := Write(context.Background(), root, Hosts())
	if err != nil {
		t.Fatal(err)
	}
	if res.RenderCommit != "none" || len(res.Files) != 8 {
		t.Fatalf("Write result = %+v", res)
	}
	for i := 1; i < len(res.Files); i++ {
		if res.Files[i-1].Path >= res.Files[i].Path {
			t.Fatalf("Write result not sorted by path: %v", res.Files)
		}
	}
	rep, err := Check(root, Hosts())
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Clean() || rep.Checked != 8 {
		t.Fatalf("Check after Write = %+v", rep)
	}
	// A second Write is byte-idempotent.
	res2, _ := Write(context.Background(), root, Hosts())
	for i := range res.Files {
		if res.Files[i] != res2.Files[i] {
			t.Fatalf("Write not idempotent at %v vs %v", res.Files[i], res2.Files[i])
		}
	}

	// task-1-review.md finding 8: the above only compares two in-memory
	// Render calls, so a Write that wrote the wrong bytes to disk (while
	// still returning the correct digest, since Result's digest comes
	// from Render's own output, not a re-read) would still pass. Re-read
	// one written file from disk and compare it to Render's Content, and
	// its digest to the first Write's recorded digest for that path.
	wantPath := Path(HostClaude, "specify")
	want, err := Render(HostClaude, "specify", res.RenderCommit)
	if err != nil {
		t.Fatal(err)
	}
	onDisk, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(wantPath)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(onDisk, want.Content) {
		t.Fatalf("on-disk bytes for %s do not match Render(...).Content", wantPath)
	}
	found := false
	for _, f := range res.Files {
		if f.Path == wantPath {
			found = true
			if f.Digest != want.Digest {
				t.Fatalf("Write's recorded digest %s != Render's digest %s for %s", f.Digest, want.Digest, wantPath)
			}
		}
	}
	if !found {
		t.Fatalf("Write result has no entry for %s", wantPath)
	}
}
