package gitx

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestCheckoutNewBranch_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := CheckoutNewBranch(ctx, repo.Dir, "design/my-feature"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	got, err := CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if got != "design/my-feature" {
		t.Fatalf("CurrentBranch = %q, want %q", got, "design/my-feature")
	}
	// The new branch starts at the same commit as its parent.
	head, err := RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatalf("RevParse(HEAD): %v", err)
	}
	if head != repo.Head {
		t.Fatalf("HEAD after CheckoutNewBranch = %q, want unchanged %q", head, repo.Head)
	}
}

func TestCheckoutNewBranch_Negative(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := CheckoutNewBranch(ctx, repo.Dir, "dup"); err != nil {
		t.Fatalf("first CheckoutNewBranch: %v", err)
	}
	// Back on main so the second attempt is a real "branch already exists"
	// collision, not a no-op re-checkout of the branch we're already on.
	if _, err := run(ctx, repo.Dir, "checkout", "main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if err := CheckoutNewBranch(ctx, repo.Dir, "dup"); err == nil {
		t.Fatal("CheckoutNewBranch(existing branch name): want error, got nil")
	}

	notARepo := t.TempDir()
	if err := CheckoutNewBranch(ctx, notARepo, "whatever"); err == nil {
		t.Fatal("CheckoutNewBranch outside a repo: want error, got nil")
	}
}

func TestAddAllCommit_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := os.WriteFile(filepath.Join(repo.Dir, "new.txt"), []byte("new content\n"), 0o644); err != nil {
		t.Fatalf("writing new.txt: %v", err)
	}
	if err := AddAll(ctx, repo.Dir); err != nil {
		t.Fatalf("AddAll: %v", err)
	}
	sha, err := CreateCommit(ctx, repo.Dir, "add new.txt")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if sha == repo.Head {
		t.Fatal("Commit did not produce a new HEAD")
	}
	got, err := RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatalf("RevParse(HEAD): %v", err)
	}
	if got != sha {
		t.Fatalf("RevParse(HEAD) = %q, want the just-created commit %q", got, sha)
	}

	// The new commit's tree really contains new.txt (AddAll actually staged
	// it, not just a message-only commit).
	show, err := Show(ctx, repo.Dir, sha, "new.txt")
	if err != nil {
		t.Fatalf("Show(sha, new.txt): %v", err)
	}
	if strings.TrimSpace(string(show)) != "new content" {
		t.Fatalf("Show(sha, new.txt) = %q, want %q", show, "new content")
	}
}

func TestCommit_Negative(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if _, err := CreateCommit(ctx, repo.Dir, ""); err == nil {
		t.Fatal("Commit(empty message): want error, got nil")
	}

	// Nothing staged: a plain `git commit` with no changes fails.
	if _, err := CreateCommit(ctx, repo.Dir, "empty commit attempt"); err == nil {
		t.Fatal("Commit with nothing staged: want error, got nil")
	}
}

// TestCreateCommit_RecordsTheWholeIndex pins the behaviour CreateCommitPaths
// exists to avoid, so the difference between the two is a proven fact of this
// package rather than a claim in a doc comment: a pathspec-less `git commit`
// records EVERY staged entry, including one the caller never touched.
func TestCreateCommit_RecordsTheWholeIndex(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	writeFixtureFiles(t, repo.Dir, map[string]string{"mine.txt": "mine\n", "theirs.txt": "theirs\n"})
	if err := AddPaths(ctx, repo.Dir, "mine.txt", "theirs.txt"); err != nil {
		t.Fatalf("AddPaths: %v", err)
	}

	sha, err := CreateCommit(ctx, repo.Dir, "commit everything staged")
	if err != nil {
		t.Fatalf("CreateCommit: %v", err)
	}
	if got := diffPaths(t, ctx, repo.Dir, repo.Head, sha); len(got) != 2 || got[0] != "mine.txt" || got[1] != "theirs.txt" {
		t.Fatalf("CreateCommit recorded %v, want both staged paths (this is the whole-index behaviour CreateCommitPaths replaces)", got)
	}
}

// TestCreateCommitPaths_RecordsOnlyTheNamedPaths is the positive half of the
// pathspec contract: exactly the named paths enter the commit, and every
// other staged entry stays in the index — uncommitted, unreverted, still the
// caller's to deal with.
//
// This is what lets `verdi policy adopt --starter` keep the promise ac-10,
// the CLI and the workbench's policy setup guide all make in the word
// "exactly": an operator who had unrelated work staged gets a four-path
// adoption commit on policy/adopt and their own change back, still staged.
func TestCreateCommitPaths_RecordsOnlyTheNamedPaths(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	writeFixtureFiles(t, repo.Dir, map[string]string{
		"wanted-one.txt": "one\n",
		"wanted-two.txt": "two\n",
		"unrelated.txt":  "unrelated\n",
	})
	// All three staged — the unrelated one exactly as a distracted operator's
	// own `git add` would have left it before running the ritual.
	if err := AddPaths(ctx, repo.Dir, "wanted-one.txt", "wanted-two.txt", "unrelated.txt"); err != nil {
		t.Fatalf("AddPaths: %v", err)
	}

	sha, err := CreateCommitPaths(ctx, repo.Dir, "record the two wanted paths", "wanted-one.txt", "wanted-two.txt")
	if err != nil {
		t.Fatalf("CreateCommitPaths: %v", err)
	}
	if sha == repo.Head {
		t.Fatal("CreateCommitPaths did not produce a new HEAD")
	}
	head, err := RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatalf("RevParse(HEAD): %v", err)
	}
	if head != sha {
		t.Fatalf("RevParse(HEAD) = %q, want the returned SHA %q", head, sha)
	}

	if got := diffPaths(t, ctx, repo.Dir, repo.Head, sha); len(got) != 2 || got[0] != "wanted-one.txt" || got[1] != "wanted-two.txt" {
		t.Fatalf("commit records %v, want exactly [wanted-one.txt wanted-two.txt]", got)
	}
	// Not merely absent from the diff: absent from the tree.
	if _, err := Show(ctx, repo.Dir, sha, "unrelated.txt"); err == nil {
		t.Fatal("Show(sha, unrelated.txt) = nil error; the unrelated path must not be in the commit's tree")
	}
	// And still staged, so the operator loses nothing.
	staged, err := StagedPaths(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("StagedPaths: %v", err)
	}
	if len(staged) != 1 || staged[0] != "unrelated.txt" {
		t.Fatalf("StagedPaths after CreateCommitPaths = %v, want exactly [unrelated.txt] left in the index", staged)
	}
	// The recorded content is the real file content, not an empty entry.
	show, err := Show(ctx, repo.Dir, sha, "wanted-one.txt")
	if err != nil {
		t.Fatalf("Show(sha, wanted-one.txt): %v", err)
	}
	if strings.TrimSpace(string(show)) != "one" {
		t.Fatalf("Show(sha, wanted-one.txt) = %q, want %q", show, "one")
	}
}

// TestCreateCommitPaths_Negative covers every refusal: the two caller-bug
// guards this function owns (blank message, no paths — AddPaths' own posture,
// since a ritual about to commit always names at least one path it wrote),
// and the three git-side failures a caller must be able to see and disclose.
func TestCreateCommitPaths_Negative(t *testing.T) {
	ctx := context.Background()

	t.Run("blank message", func(t *testing.T) {
		repo := buildRepo(t)
		writeFixtureFiles(t, repo.Dir, map[string]string{"x.txt": "x\n"})
		if err := AddPaths(ctx, repo.Dir, "x.txt"); err != nil {
			t.Fatalf("AddPaths: %v", err)
		}
		for _, message := range []string{"", "   \n\t "} {
			if _, err := CreateCommitPaths(ctx, repo.Dir, message, "x.txt"); err == nil {
				t.Fatalf("CreateCommitPaths(message %q): want error, got nil", message)
			}
		}
		// The guard refused BEFORE running git: HEAD never moved.
		head, err := RevParse(ctx, repo.Dir, "HEAD")
		if err != nil {
			t.Fatalf("RevParse(HEAD): %v", err)
		}
		if head != repo.Head {
			t.Fatal("a blank-message refusal still produced a commit")
		}
	})

	t.Run("no paths", func(t *testing.T) {
		repo := buildRepo(t)
		writeFixtureFiles(t, repo.Dir, map[string]string{"x.txt": "x\n"})
		if err := AddPaths(ctx, repo.Dir, "x.txt"); err != nil {
			t.Fatalf("AddPaths: %v", err)
		}
		// A bare `git commit` here would SUCCEED and record x.txt, which is
		// precisely why an empty paths slice must never fall through to it.
		if _, err := CreateCommitPaths(ctx, repo.Dir, "no pathspec at all"); err == nil {
			t.Fatal("CreateCommitPaths(no paths): want error, got nil")
		}
		head, err := RevParse(ctx, repo.Dir, "HEAD")
		if err != nil {
			t.Fatalf("RevParse(HEAD): %v", err)
		}
		if head != repo.Head {
			t.Fatal("a no-paths refusal still produced a commit")
		}
	})

	t.Run("pathspec git does not know", func(t *testing.T) {
		repo := buildRepo(t)
		writeFixtureFiles(t, repo.Dir, map[string]string{"never-added.txt": "nope\n"})
		if _, err := CreateCommitPaths(ctx, repo.Dir, "commit an unstaged path", "never-added.txt"); err == nil {
			t.Fatal("CreateCommitPaths(never-staged path): want error, got nil")
		}
	})

	t.Run("named paths carry no change", func(t *testing.T) {
		repo := buildRepo(t)
		if _, err := CreateCommitPaths(ctx, repo.Dir, "commit an unchanged path", "a.txt"); err == nil {
			t.Fatal("CreateCommitPaths(unchanged path): want error, got nil")
		}
	})

	t.Run("not a repo", func(t *testing.T) {
		notARepo := t.TempDir()
		writeFixtureFiles(t, notARepo, map[string]string{"x.txt": "x\n"})
		if _, err := CreateCommitPaths(ctx, notARepo, "outside any repository", "x.txt"); err == nil {
			t.Fatal("CreateCommitPaths outside a repo: want error, got nil")
		}
	})
}

// writeFixtureFiles writes each name->content under dir, failing the test on
// the first error.
func writeFixtureFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
}

// diffPaths returns the sorted paths base..head touches.
func diffPaths(t *testing.T, ctx context.Context, dir, base, head string) []string {
	t.Helper()
	entries, err := DiffNameStatus(ctx, dir, base, head)
	if err != nil {
		t.Fatalf("DiffNameStatus(%s, %s): %v", base, head, err)
	}
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		paths = append(paths, e.Path)
	}
	sort.Strings(paths)
	return paths
}
