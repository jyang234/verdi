package gitx

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

var fortyHex = regexp.MustCompile(`^[0-9a-f]{40}$`)

// buildBlobAtRepo builds a fixture repo covering every tree-entry shape
// BlobAt must discriminate: a plain tracked file, an executable file, a
// symlink (git object type "blob" too, but mode 120000 — not a plain
// file), a directory with exactly one child, and a directory with several
// children (addressed with a trailing slash so `git ls-tree` expands it
// into more than one record instead of naming the directory itself).
func buildBlobAtRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/specs/active/payments/spec.md": "payments spec v1\n",
				"dirsingle/only.txt":                   "only\n",
				"dirmulti/a.txt":                       "a\n",
				"dirmulti/b.txt":                       "b\n",
				"target.txt":                           "target\n",
			},
			Message: "seed tree shapes",
		},
	})

	scriptPath := filepath.Join(repo.Dir, "bin", "tool.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatalf("write bin/tool.sh: %v", err)
	}
	if err := os.Symlink("target.txt", filepath.Join(repo.Dir, "link.txt")); err != nil {
		t.Fatalf("symlink link.txt: %v", err)
	}

	runGitForTest(t, repo.Dir, "add", "-A")
	runGitForTest(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "add executable and symlink")
	repo.Head = strings.TrimSpace(runGitForTest(t, repo.Dir, "rev-parse", "HEAD"))
	return repo
}

// TestBlobAt_Happy proves BlobAt resolves a tracked file's blob OID at ref
// for both a plain regular file and an executable one — mode 100755 is
// still a legitimate file blob, just with the executable bit set.
func TestBlobAt_Happy(t *testing.T) {
	repo := buildBlobAtRepo(t)
	ctx := context.Background()

	cases := []struct {
		name string
		path string
	}{
		{"regular file", ".verdi/specs/active/payments/spec.md"},
		{"executable file", "bin/tool.sh"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			oid, found, err := BlobAt(ctx, repo.Dir, repo.Head, tc.path)
			if err != nil || !found || !fortyHex.MatchString(oid) {
				t.Fatalf("BlobAt() = (%q, %v, %v), want a proven blob", oid, found, err)
			}
		})
	}
}

// TestBlobAt_ExampleAssertion pins the brief's own worked example verbatim.
func TestBlobAt_ExampleAssertion(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files:   map[string]string{".verdi/specs/active/payments/spec.md": "payments spec\n"},
			Message: "add payments spec",
		},
	})
	ctx := context.Background()

	oid, found, err := BlobAt(ctx, repo.Dir, "main", ".verdi/specs/active/payments/spec.md")
	if err != nil || !found || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(oid) {
		t.Fatalf("BlobAt() = (%q, %v, %v), want a proven blob", oid, found, err)
	}
}

// TestBlobAt_Negative covers every way BlobAt must refuse or report absence
// rather than guess: an absent path (not an error), an invalid ref (an
// operational error), and tree entries that are not a single plain file
// blob — a directory addressed directly (one non-blob record), a directory
// addressed with a trailing slash that expands into several records
// (ambiguous), and a symlink (blob-typed but not mode 100644/100755).
func TestBlobAt_Negative(t *testing.T) {
	repo := buildBlobAtRepo(t)
	ctx := context.Background()

	t.Run("absent path is not an error", func(t *testing.T) {
		oid, found, err := BlobAt(ctx, repo.Dir, repo.Head, ".verdi/specs/active/does-not-exist.md")
		if err != nil {
			t.Fatalf("BlobAt(absent): unexpected error: %v", err)
		}
		if found || oid != "" {
			t.Fatalf("BlobAt(absent) = (%q, %v), want (\"\", false)", oid, found)
		}
	})

	t.Run("invalid ref is an operational error", func(t *testing.T) {
		if _, _, err := BlobAt(ctx, repo.Dir, "not-a-real-ref", ".verdi/specs/active/payments/spec.md"); err == nil {
			t.Fatal("BlobAt(bogus ref): want error, got nil")
		}
	})

	t.Run("directory (single non-blob record) is refused", func(t *testing.T) {
		if _, _, err := BlobAt(ctx, repo.Dir, repo.Head, "dirsingle"); err == nil {
			t.Fatal("BlobAt(directory): want error, got nil")
		}
	})

	t.Run("directory contents (ambiguous, multiple records) are refused", func(t *testing.T) {
		if _, _, err := BlobAt(ctx, repo.Dir, repo.Head, "dirmulti/"); err == nil {
			t.Fatal("BlobAt(directory contents): want error, got nil")
		}
	})

	t.Run("symlink (non-file blob) is refused", func(t *testing.T) {
		if _, _, err := BlobAt(ctx, repo.Dir, repo.Head, "link.txt"); err == nil {
			t.Fatal("BlobAt(symlink): want error, got nil")
		}
	})

	t.Run("not a repository at all", func(t *testing.T) {
		notARepo := t.TempDir()
		if _, _, err := BlobAt(ctx, notARepo, repo.Head, ".verdi/specs/active/payments/spec.md"); err == nil {
			t.Fatal("BlobAt outside a repo: want error, got nil")
		}
	})
}

// blobOIDFor is a test-local helper that fetches a proven blob OID via
// BlobAt, failing the test loudly if it is not found. Arranging test
// fixtures relies on the very function under test, so a lookup failure
// here is a fixture bug, not a case FirstParentBlobLanding's own tests
// exercise.
func blobOIDFor(t *testing.T, ctx context.Context, dir, ref, path string) string {
	t.Helper()
	oid, found, err := BlobAt(ctx, dir, ref, path)
	if err != nil || !found {
		t.Fatalf("blobOIDFor(%s:%s): BlobAt = (%q, %v, %v), want a proven blob", ref, path, oid, found, err)
	}
	return oid
}

// assertLanding is the brief's own worked assertion, parameterized over the
// wanted landing commit.
func assertLanding(t *testing.T, landing string, found bool, err error, wantLanding string) {
	t.Helper()
	if err != nil || !found || landing != wantLanding {
		t.Fatalf("FirstParentBlobLanding() = (%q, %v, %v), want (%q, true, nil)", landing, found, err, wantLanding)
	}
}

// mergeTopology names which merge strategy buildMergeTopology sets up.
type mergeTopology string

const (
	topologyMerge  mergeTopology = "merge"
	topologySquash mergeTopology = "squash"
	topologyRebase mergeTopology = "rebase"
)

// buildMergeTopology builds a diverged main/feature history — main
// advances past the branch point before feature (which alone adds
// spec.md) lands back onto main — then integrates feature by the named
// strategy. It returns the built repo and the commit that actually carries
// spec.md's addition onto main's first-parent chain: the real two-parent
// merge commit for "merge", the linear squash commit for "squash", or
// feature's own rebased commit (a new SHA, since its parent changed) for
// "rebase". Only test-local raw git invocations perform the merge/
// squash/rebase mechanics — gitx exposes no production API for any of
// them; fixturegit.Build seeds only the common root and main's divergent
// commit.
func buildMergeTopology(t *testing.T, strategy mergeTopology) (*fixturegit.Repo, string) {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"base.txt": "base\n"}, Message: "root"},
	})
	root := repo.Head

	runGitForTest(t, repo.Dir, "checkout", "--quiet", "-b", "feature", root)
	if err := os.WriteFile(filepath.Join(repo.Dir, "spec.md"), []byte("v1\n"), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	runGitForTest(t, repo.Dir, "add", "-A")
	runGitForTest(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "add spec on feature")

	runGitForTest(t, repo.Dir, "checkout", "--quiet", "main")
	if err := os.WriteFile(filepath.Join(repo.Dir, "main-only.txt"), []byte("main-only\n"), 0o644); err != nil {
		t.Fatalf("write main-only.txt: %v", err)
	}
	runGitForTest(t, repo.Dir, "add", "-A")
	runGitForTest(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "advance main")

	switch strategy {
	case topologyMerge:
		runGitForTest(t, repo.Dir, "merge", "--no-ff", "--no-edit", "feature")
	case topologySquash:
		runGitForTest(t, repo.Dir, "merge", "--squash", "feature")
		runGitForTest(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "squash spec from feature")
	case topologyRebase:
		runGitForTest(t, repo.Dir, "checkout", "--quiet", "feature")
		runGitForTest(t, repo.Dir, "rebase", "main")
		runGitForTest(t, repo.Dir, "checkout", "--quiet", "main")
		runGitForTest(t, repo.Dir, "merge", "--ff-only", "feature")
	default:
		t.Fatalf("buildMergeTopology: unknown strategy %q", strategy)
	}

	repo.Head = strings.TrimSpace(runGitForTest(t, repo.Dir, "rev-parse", "HEAD"))
	return repo, repo.Head
}

// TestFirstParentBlobLanding_Happy walks every topology the brief calls
// out: a first add, unrelated later commits that leave the blob unchanged,
// a change-then-revert back to the original bytes, a regular (non-ff)
// merge, a squash merge, and a rebase-then-fast-forward — proving each
// reports the actual first-parent commit on ref that carries oid, which is
// not always the commit that first introduced those bytes anywhere in
// history.
func TestFirstParentBlobLanding_Happy(t *testing.T) {
	ctx := context.Background()

	t.Run("first add", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{
			{Files: map[string]string{"spec.md": "v1\n"}, Message: "add spec"},
		})
		oid := blobOIDFor(t, ctx, repo.Dir, repo.Head, "spec.md")

		landing, found, err := FirstParentBlobLanding(ctx, repo.Dir, repo.Head, "spec.md", oid)
		assertLanding(t, landing, found, err, repo.Head)
	})

	t.Run("unchanged later commits", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{
			{Files: map[string]string{"spec.md": "v1\n"}, Message: "add spec"},
			{Files: map[string]string{"other.txt": "x\n"}, Message: "unrelated change"},
		})
		oid := blobOIDFor(t, ctx, repo.Dir, repo.Head, "spec.md")

		landing, found, err := FirstParentBlobLanding(ctx, repo.Dir, repo.Head, "spec.md", oid)
		assertLanding(t, landing, found, err, repo.Heads[0])
	})

	t.Run("replacement after a revert", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{
			{Files: map[string]string{"spec.md": "v1\n"}, Message: "add spec v1"},
			{Files: map[string]string{"spec.md": "v2\n"}, Message: "change spec to v2"},
		})
		runGitForTest(t, repo.Dir, "revert", "--no-edit", "HEAD")
		head := strings.TrimSpace(runGitForTest(t, repo.Dir, "rev-parse", "HEAD"))
		oid := blobOIDFor(t, ctx, repo.Dir, head, "spec.md")
		if oid != blobOIDFor(t, ctx, repo.Dir, repo.Heads[0], "spec.md") {
			t.Fatal("test bug: revert did not restore v1's exact original bytes")
		}

		landing, found, err := FirstParentBlobLanding(ctx, repo.Dir, head, "spec.md", oid)
		assertLanding(t, landing, found, err, head)
	})

	t.Run("regular merge", func(t *testing.T) {
		repo, mergeCommit := buildMergeTopology(t, topologyMerge)
		oid := blobOIDFor(t, ctx, repo.Dir, mergeCommit, "spec.md")

		landing, found, err := FirstParentBlobLanding(ctx, repo.Dir, mergeCommit, "spec.md", oid)
		assertLanding(t, landing, found, err, mergeCommit)
	})

	t.Run("squash merge", func(t *testing.T) {
		repo, squashCommit := buildMergeTopology(t, topologySquash)
		oid := blobOIDFor(t, ctx, repo.Dir, squashCommit, "spec.md")

		landing, found, err := FirstParentBlobLanding(ctx, repo.Dir, squashCommit, "spec.md", oid)
		assertLanding(t, landing, found, err, squashCommit)
	})

	t.Run("rebase merge", func(t *testing.T) {
		repo, rebasedCommit := buildMergeTopology(t, topologyRebase)
		oid := blobOIDFor(t, ctx, repo.Dir, rebasedCommit, "spec.md")

		landing, found, err := FirstParentBlobLanding(ctx, repo.Dir, rebasedCommit, "spec.md", oid)
		assertLanding(t, landing, found, err, rebasedCommit)
	})
}

// TestFirstParentBlobLanding_WalkCostScalesWithPathTouches is the
// path-limited walk's regression test (controller-authorized optimization):
// a long first-parent history in which the target path is touched only a
// handful of times must perform per-commit tree lookups ONLY at those
// touch commits — never one per first-parent commit (the measured
// 3.6s-per-candidate defect). Counted through the landingBlobAt seam;
// correctness of the answer is asserted alongside the count so the
// optimization can never trade the semantics away for the cost.
func TestFirstParentBlobLanding_WalkCostScalesWithPathTouches(t *testing.T) {
	ctx := context.Background()

	// spec.md is touched exactly 3 times (v1, v2, back to v1's bytes) amid
	// a long run of unrelated commits.
	layers := []fixturegit.Layer{
		{Files: map[string]string{"spec.md": "v1\n"}, Message: "add spec v1"},
	}
	for i := 0; i < 20; i++ {
		layers = append(layers, fixturegit.Layer{
			Files:   map[string]string{"noise.txt": strings.Repeat("x", i+1) + "\n"},
			Message: "unrelated churn",
		})
	}
	layers = append(layers, fixturegit.Layer{Files: map[string]string{"spec.md": "v2\n"}, Message: "change spec to v2"})
	for i := 0; i < 20; i++ {
		layers = append(layers, fixturegit.Layer{
			Files:   map[string]string{"noise.txt": strings.Repeat("y", i+1) + "\n"},
			Message: "more unrelated churn",
		})
	}
	layers = append(layers, fixturegit.Layer{Files: map[string]string{"spec.md": "v1\n"}, Message: "restore spec to v1 bytes"})
	repo := fixturegit.Build(t, layers)

	calls := 0
	orig := landingBlobAt
	landingBlobAt = func(ctx context.Context, dir, ref, path string) (string, bool, error) {
		calls++
		return orig(ctx, dir, ref, path)
	}
	defer func() { landingBlobAt = orig }()

	oid := blobOIDFor(t, ctx, repo.Dir, repo.Head, "spec.md")
	landing, found, err := FirstParentBlobLanding(ctx, repo.Dir, repo.Head, "spec.md", oid)
	// The final contiguous run starts at the restore commit — the head of
	// this linear history — never the historical v1 add.
	assertLanding(t, landing, found, err, repo.Head)

	if calls != 3 {
		t.Fatalf("landing walk performed %d per-commit tree lookups, want exactly 3 (one per path touch) for a %d-commit history", calls, len(layers))
	}
}

// TestFirstParentBlobLanding_Negative proves an unknown (but well-formed)
// OID is a proven absence, never an error, and an invalid ref is a real
// operational error.
func TestFirstParentBlobLanding_Negative(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"spec.md": "v1\n"}, Message: "add spec"},
	})
	ctx := context.Background()
	const unknownOID = "0000000000000000000000000000000000000000"

	t.Run("unknown oid", func(t *testing.T) {
		landing, found, err := FirstParentBlobLanding(ctx, repo.Dir, repo.Head, "spec.md", unknownOID)
		if err != nil {
			t.Fatalf("FirstParentBlobLanding(unknown oid): unexpected error: %v", err)
		}
		if found || landing != "" {
			t.Fatalf("FirstParentBlobLanding(unknown oid) = (%q, %v), want (\"\", false)", landing, found)
		}
	})

	t.Run("invalid ref", func(t *testing.T) {
		if _, _, err := FirstParentBlobLanding(ctx, repo.Dir, "not-a-real-ref", "spec.md", unknownOID); err == nil {
			t.Fatal("FirstParentBlobLanding(bogus ref): want error, got nil")
		}
	})

	t.Run("not a repository at all", func(t *testing.T) {
		notARepo := t.TempDir()
		if _, _, err := FirstParentBlobLanding(ctx, notARepo, "HEAD", "spec.md", unknownOID); err == nil {
			t.Fatal("FirstParentBlobLanding outside a repo: want error, got nil")
		}
	})
}
