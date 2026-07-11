package mcpserve

import (
	"os"
	"path/filepath"
	"testing"
)

const validAnnotationLine = `{"id":"a-01ARZ3NDEKTSV4RRFFQ69G5FAV","ts":"2026-05-10T14:02:11Z","author":"jane","board":{"story":"STORY-1","x":1,"y":2},"type":"comment","body":"hi","status":"open"}`

func TestReadAnnotationFile_Happy(t *testing.T) {
	t.Run("missing file is empty, not an error", func(t *testing.T) {
		got, err := readAnnotationFile(filepath.Join(t.TempDir(), "does-not-exist.jsonl"))
		if err != nil {
			t.Fatalf("readAnnotationFile(missing): %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("got %d records, want 0", len(got))
		}
	})

	t.Run("decodes valid lines, skipping blanks", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "a.jsonl")
		content := validAnnotationLine + "\n\n" + validAnnotationLine + "\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("writing fixture: %v", err)
		}
		got, err := readAnnotationFile(path)
		if err != nil {
			t.Fatalf("readAnnotationFile: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("got %d records, want 2", len(got))
		}
	})
}

func TestReadAnnotationFile_Negative(t *testing.T) {
	t.Run("malformed JSON line", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "bad.jsonl")
		if err := os.WriteFile(path, []byte("not json\n"), 0o644); err != nil {
			t.Fatalf("writing fixture: %v", err)
		}
		if _, err := readAnnotationFile(path); err == nil {
			t.Fatal("readAnnotationFile(malformed line): want error, got nil")
		}
	})

	t.Run("valid JSON but invalid annotation record", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "invalid.jsonl")
		if err := os.WriteFile(path, []byte(`{"id":"not-a-ulid"}`+"\n"), 0o644); err != nil {
			t.Fatalf("writing fixture: %v", err)
		}
		if _, err := readAnnotationFile(path); err == nil {
			t.Fatal("readAnnotationFile(invalid record): want error, got nil")
		}
	})
}

func TestReadAllAnnotations_Happy(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a--one.jsonl"), []byte(validAnnotationLine+"\n"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b--two.jsonl"), []byte(validAnnotationLine+"\n"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	// A non-.jsonl file in the same directory must be ignored.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("not jsonl"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	got, err := readAllAnnotations(dir)
	if err != nil {
		t.Fatalf("readAllAnnotations: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d records across 2 files, want 2", len(got))
	}
}

func TestReadAllAnnotations_Negative(t *testing.T) {
	t.Run("missing directory is empty, not an error", func(t *testing.T) {
		got, err := readAllAnnotations(filepath.Join(t.TempDir(), "does-not-exist"))
		if err != nil {
			t.Fatalf("readAllAnnotations(missing dir): %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("got %d records, want 0", len(got))
		}
	})

	t.Run("one malformed file fails the whole read", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "a--one.jsonl"), []byte(validAnnotationLine+"\n"), 0o644); err != nil {
			t.Fatalf("writing fixture: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "b--bad.jsonl"), []byte("not json\n"), 0o644); err != nil {
			t.Fatalf("writing fixture: %v", err)
		}
		if _, err := readAllAnnotations(dir); err == nil {
			t.Fatal("readAllAnnotations(one malformed file): want error, got nil")
		}
	})
}
