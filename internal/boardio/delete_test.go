package boardio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Owner UAT round 6, item 3: a scratch sticky or untyped thread dies
// from the mutable stream — 05 §Workbench's "graduate … or they die",
// the dying half.
func TestDeleteAnnotations_Happy(t *testing.T) {
	dir := t.TempDir()
	a1 := mustAnnotation(t, "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", "STORY-1")
	a2 := mustAnnotation(t, "a-01J8Z0K4BBBBBBBBBBBBBBBBBB", "STORY-1") // survives
	a3 := mustAnnotation(t, "a-01J8Z0K5CCCCCCCCCCCCCCCCCC", "STORY-1") // other file, untouched
	if err := AppendAnnotation(dir, "board--story-1.jsonl", a1); err != nil {
		t.Fatal(err)
	}
	if err := AppendAnnotation(dir, "board--story-1.jsonl", a2); err != nil {
		t.Fatal(err)
	}
	if err := AppendAnnotation(dir, "spec--other.jsonl", a3); err != nil {
		t.Fatal(err)
	}
	untouchedBefore, err := os.ReadFile(filepath.Join(dir, "spec--other.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	n, err := DeleteAnnotations(dir, []string{a1.ID})
	if err != nil {
		t.Fatalf("DeleteAnnotations: %v", err)
	}
	if n != 1 {
		t.Fatalf("deleted %d records, want 1", n)
	}

	got, err := ReadAllAnnotations(dir)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, a := range got {
		ids[a.ID] = true
	}
	if ids[a1.ID] {
		t.Error("deleted record still present")
	}
	if !ids[a2.ID] || !ids[a3.ID] {
		t.Errorf("unrelated records vanished: %+v", ids)
	}

	// The untouched file was not rewritten, and no temp litter remains.
	untouchedAfter, _ := os.ReadFile(filepath.Join(dir, "spec--other.jsonl"))
	if string(untouchedBefore) != string(untouchedAfter) {
		t.Error("an unrelated stream file was rewritten")
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".annotations.tmp-") {
			t.Errorf("temp litter left behind: %s", e.Name())
		}
	}
}

func TestDeleteAnnotations_Negative(t *testing.T) {
	t.Run("unknown id deletes nothing", func(t *testing.T) {
		dir := t.TempDir()
		a1 := mustAnnotation(t, "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", "STORY-1")
		if err := AppendAnnotation(dir, "board--story-1.jsonl", a1); err != nil {
			t.Fatal(err)
		}
		n, err := DeleteAnnotations(dir, []string{"a-01J8Z0K9ZZZZZZZZZZZZZZZZZZ"})
		if err != nil {
			t.Fatalf("DeleteAnnotations: %v", err)
		}
		if n != 0 {
			t.Fatalf("deleted %d records, want 0", n)
		}
		got, _ := ReadAllAnnotations(dir)
		if len(got) != 1 {
			t.Fatalf("record count changed: %d", len(got))
		}
	})

	t.Run("missing directory and empty ids are calm no-ops", func(t *testing.T) {
		if n, err := DeleteAnnotations(filepath.Join(t.TempDir(), "nope"), []string{"a-01J8Z0K3AAAAAAAAAAAAAAAAAA"}); err != nil || n != 0 {
			t.Fatalf("missing dir: n=%d err=%v, want 0, nil", n, err)
		}
		if n, err := DeleteAnnotations(t.TempDir(), nil); err != nil || n != 0 {
			t.Fatalf("empty ids: n=%d err=%v, want 0, nil", n, err)
		}
	})
}
