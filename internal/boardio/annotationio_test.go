package boardio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

func mustAnnotation(t *testing.T, id, story string) *artifact.Annotation {
	t.Helper()
	return &artifact.Annotation{
		ID: id, TS: "2026-05-10T14:02:11Z", Author: "john",
		Board:  &artifact.BoardAnchor{Story: story, X: 1, Y: 2},
		Type:   artifact.AnnotationComment,
		Body:   "hello",
		Status: artifact.AnnotationOpen,
	}
}

func TestReadAnnotationFile_Happy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "board--story-1.jsonl")
	a := mustAnnotation(t, "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", "STORY-1")
	if err := AppendAnnotation(dir, "board--story-1.jsonl", a); err != nil {
		t.Fatalf("AppendAnnotation: %v", err)
	}

	got, err := ReadAnnotationFile(path)
	if err != nil {
		t.Fatalf("ReadAnnotationFile: %v", err)
	}
	if len(got) != 1 || got[0].ID != a.ID {
		t.Fatalf("got %+v, want one record with id %s", got, a.ID)
	}
}

func TestReadAnnotationFile_Negative(t *testing.T) {
	t.Run("missing file is not an error", func(t *testing.T) {
		got, err := ReadAnnotationFile(filepath.Join(t.TempDir(), "nope.jsonl"))
		if err != nil {
			t.Fatalf("expected no error for a missing file, got %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil slice, got %+v", got)
		}
	})
	t.Run("malformed line errors", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bad.jsonl")
		if err := os.WriteFile(path, []byte("not json\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadAnnotationFile(path); err == nil {
			t.Fatal("expected a decode error for a malformed line")
		}
	})
}

func TestReadAllAnnotations_Happy(t *testing.T) {
	dir := t.TempDir()
	a1 := mustAnnotation(t, "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", "STORY-1")
	a2 := mustAnnotation(t, "a-01J8Z0K4BBBBBBBBBBBBBBBBBB", "STORY-1")
	if err := AppendAnnotation(dir, "board--story-1.jsonl", a1); err != nil {
		t.Fatal(err)
	}
	if err := AppendAnnotation(dir, "spec--stale-decline.jsonl", a2); err != nil {
		t.Fatal(err)
	}

	got, err := ReadAllAnnotations(dir)
	if err != nil {
		t.Fatalf("ReadAllAnnotations: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2: %+v", len(got), got)
	}
}

func TestReadAllAnnotations_Negative_MissingDir(t *testing.T) {
	got, err := ReadAllAnnotations(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("expected no error for a missing directory, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil slice, got %+v", got)
	}
}

func TestAppendAnnotation_Negative_UnwritableDir(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// blocker is a FILE, not a directory: MkdirAll(blocker/sub) must fail.
	err := AppendAnnotation(filepath.Join(blocker, "sub"), "x.jsonl", mustAnnotation(t, "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", "S"))
	if err == nil {
		t.Fatal("expected an error appending under a path blocked by a file")
	}
	if !strings.Contains(err.Error(), "boardio:") {
		t.Fatalf("error not wrapped with boardio context: %v", err)
	}
}

func TestAnnotationFileForTarget_And_ForBoard(t *testing.T) {
	if got := AnnotationFileForTarget(artifact.Ref{Kind: "spec", Name: "stale-decline"}); got != "spec--stale-decline.jsonl" {
		t.Errorf("AnnotationFileForTarget = %q", got)
	}
	if got := AnnotationFileForBoard("story-1482"); got != "board--story-1482.jsonl" {
		t.Errorf("AnnotationFileForBoard = %q", got)
	}
}
