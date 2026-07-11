package boardio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGraduateStickies_Happy(t *testing.T) {
	dir := t.TempDir()
	a1 := mustAnnotation(t, "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", "STORY-1")
	a2 := mustAnnotation(t, "a-01J8Z0K4BBBBBBBBBBBBBBBBBB", "STORY-1")
	a3 := mustAnnotation(t, "a-01J8Z0K5CCCCCCCCCCCCCCCCCC", "STORY-1") // untouched
	if err := AppendAnnotation(dir, "board--story-1.jsonl", a1); err != nil {
		t.Fatal(err)
	}
	if err := AppendAnnotation(dir, "board--story-1.jsonl", a2); err != nil {
		t.Fatal(err)
	}
	if err := AppendAnnotation(dir, "spec--other.jsonl", a3); err != nil {
		t.Fatal(err)
	}

	n, err := GraduateStickies(dir, []string{a1.ID, a2.ID})
	if err != nil {
		t.Fatalf("GraduateStickies: %v", err)
	}
	if n != 2 {
		t.Fatalf("graduated %d records, want 2", n)
	}

	got, err := ReadAllAnnotations(dir)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]string{}
	for _, a := range got {
		byID[a.ID] = string(a.Status)
	}
	if byID[a1.ID] != "graduated" || byID[a2.ID] != "graduated" {
		t.Fatalf("expected a1/a2 graduated, got %+v", byID)
	}
	if byID[a3.ID] != "open" {
		t.Fatalf("expected a3 untouched (still open), got %q", byID[a3.ID])
	}

	// The untouched file must not have been rewritten (no temp files left
	// anywhere, and unrelated file's content is byte-identical to what a
	// single AppendAnnotation call would have produced).
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Fatalf("leftover temp file: %s", e.Name())
		}
	}
}

func TestGraduateStickies_Negative(t *testing.T) {
	t.Run("empty ids is a no-op", func(t *testing.T) {
		n, err := GraduateStickies(t.TempDir(), nil)
		if err != nil || n != 0 {
			t.Fatalf("GraduateStickies(nil) = %d, %v; want 0, nil", n, err)
		}
	})
	t.Run("missing directory is a no-op, not an error", func(t *testing.T) {
		n, err := GraduateStickies(filepath.Join(t.TempDir(), "nope"), []string{"a-01J8Z0K3AAAAAAAAAAAAAAAAAA"})
		if err != nil || n != 0 {
			t.Fatalf("GraduateStickies(missing dir) = %d, %v; want 0, nil", n, err)
		}
	})
	t.Run("unknown id graduates nothing", func(t *testing.T) {
		dir := t.TempDir()
		a1 := mustAnnotation(t, "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", "STORY-1")
		if err := AppendAnnotation(dir, "board--story-1.jsonl", a1); err != nil {
			t.Fatal(err)
		}
		n, err := GraduateStickies(dir, []string{"a-01J8Z0K9ZZZZZZZZZZZZZZZZZZ"})
		if err != nil {
			t.Fatalf("GraduateStickies: %v", err)
		}
		if n != 0 {
			t.Fatalf("graduated %d records, want 0", n)
		}
	})
}
