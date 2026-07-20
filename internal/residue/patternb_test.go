package residue

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

func TestArchiveSpecClosedAt_Happy(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                       "data/\n",
			".verdi/specs/archive/done/spec.md":       closedArchiveStorySpecMD("done", "feature-x"),
			".verdi/specs/archive/not-closed/spec.md": storySpecMD("not-closed", "draft", "feature-x"),
		},
		Message: "seed one closed and one non-closed archive spec on main",
	}})

	got, err := archiveSpecClosedAt(context.Background(), repo.Dir, repo.Head, "done")
	if err != nil {
		t.Fatalf("archiveSpecClosedAt(done): %v", err)
	}
	if !got {
		t.Fatal("archiveSpecClosedAt(done) = false, want true")
	}
}

func TestArchiveSpecClosedAt_Negative(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                       "data/\n",
			".verdi/specs/archive/not-closed/spec.md": storySpecMD("not-closed", "draft", "feature-x"),
			".verdi/specs/archive/malformed/spec.md":  "not frontmatter at all",
		},
		Message: "seed a non-closed and a malformed archive spec on main",
	}})
	ctx := context.Background()

	t.Run("wrong status", func(t *testing.T) {
		got, err := archiveSpecClosedAt(ctx, repo.Dir, repo.Head, "not-closed")
		if err != nil {
			t.Fatalf("archiveSpecClosedAt(not-closed): %v", err)
		}
		if got {
			t.Fatal("archiveSpecClosedAt(not-closed) = true, want false (status: draft)")
		}
	})

	t.Run("missing entirely at the ref", func(t *testing.T) {
		got, err := archiveSpecClosedAt(ctx, repo.Dir, repo.Head, "never-existed")
		if err != nil {
			t.Fatalf("archiveSpecClosedAt(never-existed): unexpected error: %v", err)
		}
		if got {
			t.Fatal("archiveSpecClosedAt(never-existed) = true, want false")
		}
	})

	t.Run("present at the ref but malformed is a real error", func(t *testing.T) {
		if _, err := archiveSpecClosedAt(ctx, repo.Dir, repo.Head, "malformed"); err == nil {
			t.Fatal("archiveSpecClosedAt(malformed spec.md at ref): want error, got nil (a broken archived spec is disclosed, never a silent false)")
		}
	})

	t.Run("present on disk but not at the ref is not realized", func(t *testing.T) {
		// A closed archive spec written to the working tree but never
		// committed to the audited ref (the unmerged close-branch shape) must
		// read as NOT realized — the git-plumbing read against ref ignores an
		// uncommitted working-tree file entirely.
		dir := filepath.Join(repo.Dir, ".verdi", "specs", "archive", "uncommitted")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(closedArchiveStorySpecMD("uncommitted", "feature-x")), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := archiveSpecClosedAt(ctx, repo.Dir, repo.Head, "uncommitted")
		if err != nil {
			t.Fatalf("archiveSpecClosedAt(uncommitted): %v", err)
		}
		if got {
			t.Fatal("archiveSpecClosedAt(uncommitted, on disk but not at ref) = true, want false (realization reads the audited ref, not the working tree)")
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
		Message: "seed a stub-complete feature, its stubs closed-and-merged on main",
	}})

	specs, err := walkActiveSpecs(repo.Dir)
	if err != nil {
		t.Fatalf("walkActiveSpecs: %v", err)
	}

	got, err := findPatternB(context.Background(), repo.Dir, repo.Head, specs)
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
	ctx := context.Background()

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
		got, err := findPatternB(ctx, repo.Dir, repo.Head, specs)
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
		got, err := findPatternB(ctx, repo.Dir, repo.Head, specs)
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
		got, err := findPatternB(ctx, repo.Dir, repo.Head, specs)
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
		got, err := findPatternB(ctx, repo.Dir, repo.Head, specs)
		if err != nil {
			t.Fatalf("findPatternB: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("findPatternB = %+v, want empty (class: story, never a candidate)", got)
		}
	})
}
