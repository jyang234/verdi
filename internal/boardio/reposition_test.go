package boardio

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

func repositionFixture(t *testing.T) (dir string) {
	t.Helper()
	dir = t.TempDir()
	a := &artifact.Annotation{
		ID: "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", TS: "2026-07-10T14:02:11Z", Author: "john",
		Board: &artifact.BoardAnchor{Story: "refi-test", X: 10, Y: 20},
		Type:  artifact.AnnotationQuestion, Body: "what about partial refunds?", Status: artifact.AnnotationOpen,
	}
	b := &artifact.Annotation{
		ID: "a-01J8Z0K4BBBBBBBBBBBBBBBBBB", TS: "2026-07-10T14:03:11Z", Author: "john",
		Target: &artifact.Target{Ref: "spec/refi-test@2f230011b192c5ac1c0ed5442be76fc401c4cbca"},
		Type:   artifact.AnnotationComment, Body: "targeted, no board anchor", Status: artifact.AnnotationOpen,
	}
	for _, ann := range []*artifact.Annotation{a, b} {
		if err := AppendAnnotation(dir, "board--refi-test.jsonl", ann); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestRepositionSticky(t *testing.T) {
	dir := repositionFixture(t)
	if err := RepositionSticky(dir, "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", 260, 133.5); err != nil {
		t.Fatalf("RepositionSticky: %v", err)
	}
	records, err := ReadAnnotationFile(filepath.Join(dir, "board--refi-test.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2 (rewrite must not drop lines)", len(records))
	}
	if records[0].Board.X != 260 || records[0].Board.Y != 133.5 {
		t.Fatalf("board anchor = %+v, want 260/133.5", records[0].Board)
	}
	if records[1].Body != "targeted, no board anchor" {
		t.Fatal("unrelated record changed")
	}
	// No temp litter.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("dir has %d entries, want 1", len(entries))
	}
}

func TestRepositionSticky_Negative(t *testing.T) {
	dir := repositionFixture(t)
	if err := RepositionSticky(dir, "a-01J8Z0K5CCCCCCCCCCCCCCCCCC", 1, 1); err == nil {
		t.Fatal("repositioning a missing annotation succeeded")
	}
	// A targeted-only annotation has no board anchor to move.
	if err := RepositionSticky(dir, "a-01J8Z0K4BBBBBBBBBBBBBBBBBB", 1, 1); err == nil {
		t.Fatal("repositioning a board-less annotation succeeded")
	}
}
