package gitx

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestWriteBlob_HashObjectAgree proves WriteBlob's returned SHA is the
// exact blob id `git hash-object` (no -w) would compute for the same
// bytes, and that the object is actually written to the store (readable
// back via `git cat-file`) — WITHOUT touching the working tree or the
// repo's real index (no file is created on disk by this call).
func TestWriteBlob_HashObjectAgree(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()
	content := []byte("hello scoping canvas\n")

	sha, err := WriteBlob(ctx, repo.Dir, content)
	if err != nil {
		t.Fatalf("WriteBlob: %v", err)
	}
	if sha == "" {
		t.Fatal("WriteBlob returned an empty sha")
	}

	// Write the same bytes to a real file and hash it the ordinary way —
	// same content must produce the same blob id.
	tmp := filepath.Join(t.TempDir(), "same.txt")
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		t.Fatal(err)
	}
	want, err := HashObject(ctx, repo.Dir, tmp)
	if err != nil {
		t.Fatalf("HashObject: %v", err)
	}
	if sha != want {
		t.Fatalf("WriteBlob sha = %q, want %q (HashObject of identical bytes)", sha, want)
	}

	out, err := run(ctx, repo.Dir, "cat-file", "-p", sha)
	if err != nil {
		t.Fatalf("cat-file -p %s: %v", sha, err)
	}
	if string(out) != string(content) {
		t.Fatalf("cat-file -p %s = %q, want %q", sha, out, content)
	}
}

// TestBuildTreeWithFile_AddsOneFileOntoBaseTree proves BuildTreeWithFile
// builds a NEW tree carrying every file the base tree had, plus one new
// path — without writing anything to the working directory or the repo's
// real index (checked via git's own status: still clean afterward).
func TestBuildTreeWithFile_AddsOneFileOntoBaseTree(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	blobSHA, err := WriteBlob(ctx, repo.Dir, []byte("new file content\n"))
	if err != nil {
		t.Fatalf("WriteBlob: %v", err)
	}
	newTree, err := BuildTreeWithFile(ctx, repo.Dir, repo.Head+"^{tree}", "sub/new.txt", blobSHA)
	if err != nil {
		t.Fatalf("BuildTreeWithFile: %v", err)
	}
	if newTree == "" {
		t.Fatal("BuildTreeWithFile returned an empty tree sha")
	}

	// The base tree's own files are still present in the new tree.
	if _, err := run(ctx, repo.Dir, "cat-file", "-e", newTree+":a.txt"); err != nil {
		t.Fatalf("new tree lost the base tree's a.txt: %v", err)
	}
	if _, err := run(ctx, repo.Dir, "cat-file", "-e", newTree+":dir/b.txt"); err != nil {
		t.Fatalf("new tree lost the base tree's dir/b.txt: %v", err)
	}
	// The new file is present with the exact content.
	out, err := run(ctx, repo.Dir, "cat-file", "-p", newTree+":sub/new.txt")
	if err != nil {
		t.Fatalf("new tree missing sub/new.txt: %v", err)
	}
	if string(out) != "new file content\n" {
		t.Fatalf("sub/new.txt = %q, want %q", out, "new file content\n")
	}

	// The repository's OWN index and working tree were never touched.
	dirty, err := StatusDirty(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if dirty {
		t.Fatal("BuildTreeWithFile left the real working tree/index dirty")
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, "sub", "new.txt")); err == nil {
		t.Fatal("BuildTreeWithFile wrote the new file into the real working tree")
	}
}

// TestCommitTree_ProducesACommitWithoutMovingAnyRef proves CommitTree
// creates a commit object reachable only by the sha it returns — HEAD and
// every branch are untouched.
func TestCommitTree_ProducesACommitWithoutMovingAnyRef(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	commit, err := CommitTree(ctx, repo.Dir, repo.Head+"^{tree}", repo.Head, "a plumbing-built commit")
	if err != nil {
		t.Fatalf("CommitTree: %v", err)
	}
	if commit == "" || commit == repo.Head {
		t.Fatalf("CommitTree returned %q, want a new, non-empty sha", commit)
	}

	head, err := RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if head != repo.Head {
		t.Fatalf("HEAD moved to %q after CommitTree (should be untouched)", head)
	}

	// The parent link is exactly repo.Head.
	out, err := run(ctx, repo.Dir, "rev-parse", commit+"^")
	if err != nil {
		t.Fatalf("rev-parse %s^: %v", commit, err)
	}
	if got := string(out); got[:len(got)-1] != repo.Head { // trim trailing newline
		t.Fatalf("commit parent = %q, want %q", got, repo.Head)
	}
}

// TestUpdateRef_CreatesBranchWithoutCheckout proves UpdateRef creates a
// new branch ref pointing at the given commit without moving HEAD or
// touching the working tree — the no-checkout plumbing stub-instantiate
// depends on (spec/scoping-canvas ac-6).
func TestUpdateRef_CreatesBranchWithoutCheckout(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	commit, err := CommitTree(ctx, repo.Dir, repo.Head+"^{tree}", repo.Head, "scaffold commit")
	if err != nil {
		t.Fatalf("CommitTree: %v", err)
	}
	if err := UpdateRef(ctx, repo.Dir, "refs/heads/design/plumbing-fixture", commit); err != nil {
		t.Fatalf("UpdateRef: %v", err)
	}

	branches, err := LocalBranches(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, b := range branches {
		if b == "design/plumbing-fixture" {
			found = true
		}
	}
	if !found {
		t.Fatalf("branches = %v, want design/plumbing-fixture", branches)
	}

	branch, err := CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if branch == "design/plumbing-fixture" {
		t.Fatal("UpdateRef checked the new branch out — it must not")
	}
	head, err := RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if head != repo.Head {
		t.Fatal("UpdateRef moved HEAD")
	}
}

// TestUpdateRef_Negative_RefAlreadyExists proves UpdateRef fails closed
// rather than silently moving an existing branch (stub-instantiate: "fail
// closed if the branch exists").
func TestUpdateRef_Negative_RefAlreadyExists(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := UpdateRef(ctx, repo.Dir, "refs/heads/design/dup", repo.Head); err != nil {
		t.Fatalf("first UpdateRef: %v", err)
	}
	commit, err := CommitTree(ctx, repo.Dir, repo.Head+"^{tree}", repo.Head, "second commit")
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateRef(ctx, repo.Dir, "refs/heads/design/dup", commit); err == nil {
		t.Fatal("UpdateRef onto an existing ref succeeded, want error")
	}
	// The existing ref must be untouched by the failed attempt.
	got, err := RevParse(ctx, repo.Dir, "refs/heads/design/dup")
	if err != nil {
		t.Fatal(err)
	}
	if got != repo.Head {
		t.Fatalf("refs/heads/design/dup = %q after a refused UpdateRef, want unchanged %q", got, repo.Head)
	}
}

// repoState is everything a refused or failed UpdateRef must leave as it
// was: every ref with its value, what HEAD names, the local config (a
// branch's upstream lives there) and the working tree's status.
func repoState(t *testing.T, dir string) string {
	t.Helper()
	ctx := context.Background()
	var b strings.Builder
	for _, args := range [][]string{
		{"for-each-ref", "--format=%(refname) %(objectname)"},
		{"symbolic-ref", "HEAD"},
		{"config", "--local", "--list"},
		{"status", "--porcelain", "--untracked-files=all"},
	} {
		out, err := run(ctx, dir, args...)
		if err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
		b.WriteString(strings.Join(args, " ") + ":\n" + string(out))
	}
	return b.String()
}

// TestUpdateRef_RunsCreateOnlyGitBranch pins UpdateRef's whole git
// command log: one `git branch <name> <commit>`, never update-ref
// (ritual-write-scope-v3 dc-8, ledger SI-359 (5)), and the branch it
// creates at a commit has no upstream configured.
func TestUpdateRef_RunsCreateOnlyGitBranch(t *testing.T) {
	repo := buildRepo(t)
	obs := &recordingObserver{}
	ctx := WithObserver(context.Background(), obs)

	if err := UpdateRef(ctx, repo.Dir, "refs/heads/design/argv", repo.Heads[0]); err != nil {
		t.Fatalf("UpdateRef: %v", err)
	}
	want := [][]string{{repo.Dir, "branch", "design/argv", repo.Heads[0]}}
	if !reflect.DeepEqual(obs.calls, want) {
		t.Fatalf("observed %q, want %q", obs.calls, want)
	}
	got, err := RevParse(context.Background(), repo.Dir, "refs/heads/design/argv")
	if err != nil {
		t.Fatal(err)
	}
	if got != repo.Heads[0] {
		t.Fatalf("refs/heads/design/argv = %s, want %s", got, repo.Heads[0])
	}
	cfg, err := run(context.Background(), repo.Dir, "config", "--local", "--list")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(cfg), "branch.design/argv.") {
		t.Fatalf("UpdateRef configured the new branch:\n%s", cfg)
	}
}

// TestUpdateRef_AutoSetupMergeAlwaysWritesNoUpstream proves why the start
// point must be a full object id (ledger SI-359 (5c)): even under
// branch.autoSetupMerge=always, a branch created at a commit id gets no
// upstream configuration.
func TestUpdateRef_AutoSetupMergeAlwaysWritesNoUpstream(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()
	if _, err := run(ctx, repo.Dir, "config", "branch.autoSetupMerge", "always"); err != nil {
		t.Fatal(err)
	}
	before, err := run(ctx, repo.Dir, "config", "--local", "--list")
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateRef(ctx, repo.Dir, "refs/heads/design/always", repo.Heads[0]); err != nil {
		t.Fatalf("UpdateRef: %v", err)
	}
	after, err := run(ctx, repo.Dir, "config", "--local", "--list")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("UpdateRef changed the local config:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestUpdateRef_Negative_ExistingBranchChangesNothing proves the create is
// create-only: onto an existing branch it fails, and the branch, every
// other ref, HEAD, the config and the working tree are exactly as before.
func TestUpdateRef_Negative_ExistingBranchChangesNothing(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()
	if err := UpdateRef(ctx, repo.Dir, "refs/heads/design/dup", repo.Heads[0]); err != nil {
		t.Fatalf("first UpdateRef: %v", err)
	}
	before := repoState(t, repo.Dir)
	if err := UpdateRef(ctx, repo.Dir, "refs/heads/design/dup", repo.Head); err == nil {
		t.Fatal("UpdateRef onto an existing branch succeeded, want error")
	}
	if after := repoState(t, repo.Dir); after != before {
		t.Fatalf("a refused UpdateRef changed the repository:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestUpdateRef_Negative_RefusedBeforeGit is ledger SI-359 (5b): UpdateRef
// refuses a ref git branch cannot create (outside refs/heads/, or with an
// empty short name) and anything git would parse as an option (a short
// name or a commit starting with "-"), before any git runs, so the
// function is create-only for every input. Given refs/remotes/origin/x,
// git branch would silently create refs/heads/refs/remotes/origin/x; given
// refs/heads/-f, or the commit "-f", it would force-move a branch. It
// also refuses a name containing "@{", which git expands to another branch
// and creates (R5AR-1), and a start point that is not a full object id,
// which under branch.autoSetupMerge=always writes upstream configuration
// (R5AR-2); both are ledger SI-359 (5c). The repository is set up so that
// every refused input would otherwise do something: @{-1} names the
// deleted branch ghost, @{u} names main's upstream up, and v1 is a tag.
func TestUpdateRef_Negative_RefusedBeforeGit(t *testing.T) {
	repo := buildRepo(t)
	for _, args := range [][]string{
		{"checkout", "-q", "-b", "ghost"},
		{"checkout", "-q", "main"},
		{"branch", "-q", "-D", "ghost"},
		{"config", "branch.main.remote", "."},
		{"config", "branch.main.merge", "refs/heads/up"},
		{"tag", "v1", repo.Heads[0]},
	} {
		if _, err := run(context.Background(), repo.Dir, args...); err != nil {
			t.Fatalf("setup: git %s: %v", strings.Join(args, " "), err)
		}
	}
	tests := []struct {
		name, ref, commit string
	}{
		{"a remote-tracking ref", "refs/remotes/origin/x", repo.Head},
		{"a tag", "refs/tags/v1", repo.Head},
		{"a short name", "design/x", repo.Head},
		{"HEAD", "HEAD", repo.Head},
		{"refs/heads without a slash", "refs/heads", repo.Head},
		{"an empty short name", "refs/heads/", repo.Head},
		{"an empty ref", "", repo.Head},
		{"a short name that is a flag", "refs/heads/-f", repo.Head},
		{"a short name that is a long flag", "refs/heads/--force", repo.Head},
		{"a short name starting with a dash", "refs/heads/-design/x", repo.Head},
		{"a commit that is a flag", "refs/heads/design/x", "-f"},
		{"an empty commit", "refs/heads/design/x", ""},
		{"the previous-branch name", "refs/heads/@{-1}", repo.Head},
		{"the upstream name", "refs/heads/@{u}", repo.Head},
		{"a name containing @{", "refs/heads/x@{y", repo.Head},
		{"a start point that is HEAD", "refs/heads/design/x", "HEAD"},
		{"a start point that is a branch", "refs/heads/design/x", "main"},
		{"a start point that is an abbreviated object id", "refs/heads/design/x", repo.Head[:7]},
		{"a start point that is a tag", "refs/heads/design/x", "v1"},
		{"a start point that is an uppercase object id", "refs/heads/design/x", strings.ToUpper(repo.Head)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := repoState(t, repo.Dir)
			obs := &recordingObserver{}
			err := UpdateRef(WithObserver(context.Background(), obs), repo.Dir, tt.ref, tt.commit)
			if err == nil {
				t.Fatalf("UpdateRef(%q, %q) succeeded, want a refusal", tt.ref, tt.commit)
			}
			if !strings.Contains(err.Error(), "UpdateRef") {
				t.Errorf("error %q does not name UpdateRef", err)
			}
			if len(obs.calls) != 0 {
				t.Fatalf("UpdateRef(%q, %q) ran git before refusing: %q", tt.ref, tt.commit, obs.calls)
			}
			if after := repoState(t, repo.Dir); after != before {
				t.Fatalf("a refused UpdateRef changed the repository:\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}
}
