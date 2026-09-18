package skillpack

import (
	"context"
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
}
