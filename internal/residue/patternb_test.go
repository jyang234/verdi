package residue

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

func TestArchiveSpecIsClosed_Happy(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                       "data/\n",
			".verdi/specs/archive/done/spec.md":       closedArchiveStorySpecMD("done", "feature-x"),
			".verdi/specs/archive/not-closed/spec.md": storySpecMD("not-closed", "draft", "feature-x"),
		},
		Message: "seed one closed and one non-closed archive spec",
	}})

	got, err := archiveSpecIsClosed(repo.Dir, "done")
	if err != nil {
		t.Fatalf("archiveSpecIsClosed(done): %v", err)
	}
	if !got {
		t.Fatal("archiveSpecIsClosed(done) = false, want true")
	}
}

func TestArchiveSpecIsClosed_Negative(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                       "data/\n",
			".verdi/specs/archive/not-closed/spec.md": storySpecMD("not-closed", "draft", "feature-x"),
		},
		Message: "seed one non-closed archive spec",
	}})

	t.Run("wrong status", func(t *testing.T) {
		got, err := archiveSpecIsClosed(repo.Dir, "not-closed")
		if err != nil {
			t.Fatalf("archiveSpecIsClosed(not-closed): %v", err)
		}
		if got {
			t.Fatal("archiveSpecIsClosed(not-closed) = true, want false (status: draft)")
		}
	})

	t.Run("missing entirely", func(t *testing.T) {
		got, err := archiveSpecIsClosed(repo.Dir, "never-existed")
		if err != nil {
			t.Fatalf("archiveSpecIsClosed(never-existed): unexpected error: %v", err)
		}
		if got {
			t.Fatal("archiveSpecIsClosed(never-existed) = true, want false")
		}
	})

	t.Run("malformed spec.md is a real error", func(t *testing.T) {
		malformedDir := filepath.Join(repo.Dir, ".verdi", "specs", "archive", "malformed")
		if err := os.MkdirAll(malformedDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(malformedDir, "spec.md"), []byte("not frontmatter at all"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := archiveSpecIsClosed(repo.Dir, "malformed"); err == nil {
			t.Fatal("archiveSpecIsClosed(malformed spec.md): want error, got nil")
		}
	})
}

func TestFindPatternB_Happy(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                       "data/\n",
			".verdi/specs/archive/story-one/spec.md":  closedArchiveStorySpecMD("story-one", "code-health"),
			".verdi/specs/archive/story-two/spec.md":  closedArchiveStorySpecMD("story-two", "code-health"),
			".verdi/specs/active/code-health/spec.md": featureSpecMD("code-health", "accepted-pending-build", "story-one", "story-two"),
		},
		Message: "seed a stub-complete feature",
	}})

	specs, err := walkActiveSpecs(repo.Dir)
	if err != nil {
		t.Fatalf("walkActiveSpecs: %v", err)
	}

	got, err := findPatternB(repo.Dir, specs)
	if err != nil {
		t.Fatalf("findPatternB: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("findPatternB = %+v, want exactly 1 (code-health)", got)
	}
	if got[0].SpecName != "code-health" {
		t.Fatalf("findPatternB[0].SpecName = %q, want code-health", got[0].SpecName)
	}
	want := []string{"story-one", "story-two"}
	if len(got[0].Stubs) != len(want) || got[0].Stubs[0] != want[0] || got[0].Stubs[1] != want[1] {
		t.Fatalf("findPatternB[0].Stubs = %v, want %v", got[0].Stubs, want)
	}
}

func TestFindPatternB_Negative(t *testing.T) {
	t.Run("one stub not yet realized", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{
			Files: map[string]string{
				".verdi/.gitignore":                       "data/\n",
				".verdi/specs/archive/story-one/spec.md":  closedArchiveStorySpecMD("story-one", "code-health"),
				".verdi/specs/active/code-health/spec.md": featureSpecMD("code-health", "accepted-pending-build", "story-one", "story-two"),
			},
			Message: "seed a feature with one unrealized stub",
		}})
		specs, err := walkActiveSpecs(repo.Dir)
		if err != nil {
			t.Fatalf("walkActiveSpecs: %v", err)
		}
		got, err := findPatternB(repo.Dir, specs)
		if err != nil {
			t.Fatalf("findPatternB: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("findPatternB = %+v, want empty (story-two not realized)", got)
		}
	})

	t.Run("realized stub not closed status", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{
			Files: map[string]string{
				".verdi/.gitignore":                       "data/\n",
				".verdi/specs/archive/story-one/spec.md":  storySpecMD("story-one", "superseded", "code-health"),
				".verdi/specs/active/code-health/spec.md": featureSpecMD("code-health", "accepted-pending-build", "story-one"),
			},
			Message: "seed a feature whose sole stub is archived but not status: closed",
		}})
		specs, err := walkActiveSpecs(repo.Dir)
		if err != nil {
			t.Fatalf("walkActiveSpecs: %v", err)
		}
		got, err := findPatternB(repo.Dir, specs)
		if err != nil {
			t.Fatalf("findPatternB: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("findPatternB = %+v, want empty (story-one is superseded, not closed)", got)
		}
	})

	t.Run("no stubs declared: nothing to reconcile", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{
			Files: map[string]string{
				".verdi/.gitignore":                       "data/\n",
				".verdi/specs/active/code-health/spec.md": featureSpecMD("code-health", "accepted-pending-build"),
			},
			Message: "seed a feature with zero declared stubs",
		}})
		specs, err := walkActiveSpecs(repo.Dir)
		if err != nil {
			t.Fatalf("walkActiveSpecs: %v", err)
		}
		got, err := findPatternB(repo.Dir, specs)
		if err != nil {
			t.Fatalf("findPatternB: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("findPatternB = %+v, want empty (no stubs declared)", got)
		}
	})

	t.Run("story class is never a pattern (b) candidate", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{
			Files: map[string]string{
				".verdi/.gitignore":                   "data/\n",
				".verdi/specs/active/a-story/spec.md": storySpecMD("a-story", "accepted-pending-build", "feature-x"),
			},
			Message: "seed a story, not a feature",
		}})
		specs, err := walkActiveSpecs(repo.Dir)
		if err != nil {
			t.Fatalf("walkActiveSpecs: %v", err)
		}
		got, err := findPatternB(repo.Dir, specs)
		if err != nil {
			t.Fatalf("findPatternB: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("findPatternB = %+v, want empty (class: story, never a candidate)", got)
		}
	})
}
