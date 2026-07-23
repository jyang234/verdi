package gitx

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStagedPaths(t *testing.T) {
	ctx := context.Background()

	t.Run("lists staged additions modifications and deletions deterministically", func(t *testing.T) {
		repo := buildRepo(t)
		added := "z-added\nwith-newline.txt"

		if err := os.WriteFile(filepath.Join(repo.Dir, "a.txt"), []byte("staged modification\n"), 0o644); err != nil {
			t.Fatalf("modifying a.txt: %v", err)
		}
		if err := os.Remove(filepath.Join(repo.Dir, "dir", "b.txt")); err != nil {
			t.Fatalf("deleting dir/b.txt: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repo.Dir, added), []byte("staged addition\n"), 0o644); err != nil {
			t.Fatalf("adding %q: %v", added, err)
		}
		if err := AddPaths(ctx, repo.Dir, "a.txt", "dir/b.txt", added); err != nil {
			t.Fatalf("AddPaths: %v", err)
		}

		got, err := StagedPaths(ctx, repo.Dir)
		if err != nil {
			t.Fatalf("StagedPaths: %v", err)
		}
		want := []string{"a.txt", "dir/b.txt", added}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("StagedPaths = %#v, want sorted add/modify/delete paths %#v", got, want)
		}
	})

	t.Run("clean index is empty", func(t *testing.T) {
		repo := buildRepo(t)

		got, err := StagedPaths(ctx, repo.Dir)
		if err != nil {
			t.Fatalf("StagedPaths: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("StagedPaths = %#v, want no paths", got)
		}
	})
}

func TestStagedPaths_OutsideRepository(t *testing.T) {
	_, err := StagedPaths(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("StagedPaths outside a repository: want operational error, got nil")
	}
}
