package gitx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/fixturegit"
)

func buildRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files:   map[string]string{"a.txt": "hello\n", "dir/b.txt": "world\n"},
			Message: "layer 1",
		},
		{
			Files:   map[string]string{"a.txt": "hello again\n"},
			Message: "layer 2",
		},
	})
}

func TestRevParse_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	got, err := RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatalf("RevParse(HEAD): %v", err)
	}
	if got != repo.Head {
		t.Fatalf("RevParse(HEAD) = %q, want %q", got, repo.Head)
	}

	// <rev>:<path> form, used to resolve a blob at a historical commit.
	got, err = RevParse(ctx, repo.Dir, repo.Heads[0]+":a.txt")
	if err != nil {
		t.Fatalf("RevParse(commit:path): %v", err)
	}
	if got == "" {
		t.Fatal("RevParse(commit:path) returned empty object id")
	}
}

func TestRevParse_Negative(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if _, err := RevParse(ctx, repo.Dir, "not-a-real-ref"); err == nil {
		t.Fatal("RevParse(bogus ref): want error, got nil")
	}

	notARepo := t.TempDir()
	if _, err := RevParse(ctx, notARepo, "HEAD"); err == nil {
		t.Fatal("RevParse outside a repo: want error, got nil")
	}
}

func TestHashObject_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	got, err := HashObject(ctx, repo.Dir, "a.txt")
	if err != nil {
		t.Fatalf("HashObject: %v", err)
	}
	// Independently verify against git's own committed blob sha for the
	// same content (a.txt's current content was committed in layer 2).
	want, err := RevParse(ctx, repo.Dir, "HEAD:a.txt")
	if err != nil {
		t.Fatalf("RevParse(HEAD:a.txt): %v", err)
	}
	if got != want {
		t.Fatalf("HashObject(a.txt) = %q, want %q (git's own committed blob sha)", got, want)
	}

	// Dirty working file (uncommitted edit) must hash by content, not by
	// whatever git last committed (I-15).
	if err := os.WriteFile(filepath.Join(repo.Dir, "a.txt"), []byte("dirty content\n"), 0o644); err != nil {
		t.Fatalf("dirtying a.txt: %v", err)
	}
	dirtyHash, err := HashObject(ctx, repo.Dir, "a.txt")
	if err != nil {
		t.Fatalf("HashObject(dirty): %v", err)
	}
	if dirtyHash == got {
		t.Fatal("HashObject did not pick up the dirty (uncommitted) content")
	}

	// Works outside a git repository too — pure content hash.
	noRepoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(noRepoDir, "c.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("writing c.txt: %v", err)
	}
	outsideHash, err := HashObject(ctx, noRepoDir, "c.txt")
	if err != nil {
		t.Fatalf("HashObject outside a repo: %v", err)
	}
	// "hello\n" also backs the original a.txt content in layer 1 — confirm
	// the hash is content-addressed by comparing against that layer's SHA.
	origHash, err := RevParse(ctx, repo.Dir, repo.Heads[0]+":a.txt")
	if err != nil {
		t.Fatalf("RevParse(layer1 a.txt): %v", err)
	}
	if outsideHash != origHash {
		t.Fatalf("HashObject outside a repo = %q, want content-addressed match %q", outsideHash, origHash)
	}
}

func TestHashObject_Negative(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if _, err := HashObject(ctx, dir, "does-not-exist.txt"); err == nil {
		t.Fatal("HashObject(missing file): want error, got nil")
	}
}

func TestLsFiles_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	got, err := LsFiles(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("LsFiles: %v", err)
	}
	want := map[string]bool{"a.txt": true, "dir/b.txt": true}
	if len(got) != len(want) {
		t.Fatalf("LsFiles = %v, want exactly %v", got, want)
	}
	for _, p := range got {
		if !want[p] {
			t.Fatalf("LsFiles returned unexpected path %q", p)
		}
	}
}

func TestLsFiles_Negative(t *testing.T) {
	ctx := context.Background()
	notARepo := t.TempDir()
	if _, err := LsFiles(ctx, notARepo); err == nil {
		t.Fatal("LsFiles outside a repo: want error, got nil")
	}
}

func TestLsFilesWithUntracked_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	// A brand-new, never-git-added file: invisible to LsFiles, but the whole
	// point of LsFilesWithUntracked is to surface it.
	if err := os.WriteFile(filepath.Join(repo.Dir, "untracked.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("writing untracked.txt: %v", err)
	}

	got, err := LsFilesWithUntracked(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("LsFilesWithUntracked: %v", err)
	}
	want := map[string]bool{"a.txt": true, "dir/b.txt": true, "untracked.txt": true}
	if len(got) != len(want) {
		t.Fatalf("LsFilesWithUntracked = %v, want exactly %v", got, want)
	}
	for _, p := range got {
		if !want[p] {
			t.Fatalf("LsFilesWithUntracked returned unexpected path %q", p)
		}
	}

	// A tracked file deleted from the working tree (but not `git rm`'d) stays
	// listed — it is still in the index. Callers hashing on-disk content rely
	// on this to detect a deletion instead of erroring on a missing file.
	if err := os.Remove(filepath.Join(repo.Dir, "dir/b.txt")); err != nil {
		t.Fatalf("removing dir/b.txt: %v", err)
	}
	got, err = LsFilesWithUntracked(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("LsFilesWithUntracked after delete: %v", err)
	}
	stillListed := false
	for _, p := range got {
		if p == "dir/b.txt" {
			stillListed = true
		}
	}
	if !stillListed {
		t.Fatal("LsFilesWithUntracked dropped a working-tree-deleted tracked file; want it still listed via the index")
	}
}

// TestLsFilesWithUntracked_ExcludesGitignored proves `--exclude-standard`
// keeps .gitignore'd paths out — the mechanism the store relies on so that
// `.verdi/data/` (ignored by the committed `.verdi/.gitignore`) never enters
// the corpus enumeration.
func TestLsFilesWithUntracked_ExcludesGitignored(t *testing.T) {
	ctx := context.Background()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				"tracked.txt": "t\n",
				".gitignore":  "data/\n",
			},
			Message: "layer 1",
		},
	})

	// Untracked file under the ignored dir (must NOT appear) and an untracked
	// file outside it (must appear).
	if err := os.MkdirAll(filepath.Join(repo.Dir, "data", "cache"), 0o755); err != nil {
		t.Fatalf("mkdir data/cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "data", "cache", "blob"), []byte("noise\n"), 0o644); err != nil {
		t.Fatalf("writing ignored blob: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "visible.txt"), []byte("v\n"), 0o644); err != nil {
		t.Fatalf("writing visible.txt: %v", err)
	}

	got, err := LsFilesWithUntracked(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("LsFilesWithUntracked: %v", err)
	}
	want := map[string]bool{"tracked.txt": true, ".gitignore": true, "visible.txt": true}
	if len(got) != len(want) {
		t.Fatalf("LsFilesWithUntracked = %v, want exactly %v (ignored data/ excluded)", got, want)
	}
	for _, p := range got {
		if !want[p] {
			t.Fatalf("LsFilesWithUntracked returned unexpected path %q (gitignored path leaked?)", p)
		}
	}
}

func TestLsFilesWithUntracked_Negative(t *testing.T) {
	ctx := context.Background()
	notARepo := t.TempDir()
	if _, err := LsFilesWithUntracked(ctx, notARepo); err == nil {
		t.Fatal("LsFilesWithUntracked outside a repo: want error, got nil")
	}
}

func TestShow_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	got, err := Show(ctx, repo.Dir, repo.Heads[0], "a.txt")
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if strings.TrimSpace(string(got)) != "hello" {
		t.Fatalf("Show(layer1, a.txt) = %q, want historical content %q", got, "hello")
	}

	// Current HEAD content differs from layer 1's.
	head, err := Show(ctx, repo.Dir, repo.Head, "a.txt")
	if err != nil {
		t.Fatalf("Show(HEAD, a.txt): %v", err)
	}
	if strings.TrimSpace(string(head)) != "hello again" {
		t.Fatalf("Show(HEAD, a.txt) = %q, want %q", head, "hello again")
	}
}

func TestShow_Negative(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if _, err := Show(ctx, repo.Dir, repo.Head, "does-not-exist.txt"); err == nil {
		t.Fatal("Show(missing path): want error, got nil")
	}
	if _, err := Show(ctx, repo.Dir, "0000000000000000000000000000000000000000", "a.txt"); err == nil {
		t.Fatal("Show(bogus commit): want error, got nil")
	}
}
