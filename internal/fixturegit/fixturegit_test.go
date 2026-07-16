package fixturegit

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestBuild_Deterministic proves spike S2 (PLAN.md §6 R2, §4 fixturegit):
// building the same layers twice yields byte-identical HEAD SHAs. Every
// later phase's pinned refs, frozen stamps, and cross-commit diffs depend
// on this holding across machines and runs.
func TestBuild_Deterministic(t *testing.T) {
	layers := []Layer{
		{Files: map[string]string{"a.txt": "hello\n"}, Message: "add a"},
		{Files: map[string]string{"dir/b.txt": "world\n"}, Message: "add b"},
		{Files: map[string]string{"a.txt": "hello again\n"}, Message: "update a"},
	}

	r1 := Build(t, layers)
	r2 := Build(t, layers)

	if r1.Head == "" {
		t.Fatal("Build: HEAD SHA is empty")
	}
	if r1.Head != r2.Head {
		t.Fatalf("HEAD SHAs differ across identical builds: %s vs %s", r1.Head, r2.Head)
	}
	if r1.Dir == r2.Dir {
		t.Fatalf("expected distinct temp dirs per build, got the same: %s", r1.Dir)
	}
}

// TestBuild_DifferentContentDifferentSHA is the negative complement: a
// content change must change the SHA, or the determinism guarantee would be
// vacuously true (a helper that always returns the same SHA regardless of
// input would also "pass" TestBuild_Deterministic).
func TestBuild_DifferentContentDifferentSHA(t *testing.T) {
	base := []Layer{{Files: map[string]string{"a.txt": "hello\n"}, Message: "add a"}}
	changed := []Layer{{Files: map[string]string{"a.txt": "goodbye\n"}, Message: "add a"}}

	rBase := Build(t, base)
	rChanged := Build(t, changed)

	if rBase.Head == rChanged.Head {
		t.Fatalf("expected different SHAs for different content, both got %s", rBase.Head)
	}
}

// TestBuild_DifferentMessageDifferentSHA: the commit message is part of the
// commit object, so it must also affect the SHA (guards against a helper
// that silently drops messages).
func TestBuild_DifferentMessageDifferentSHA(t *testing.T) {
	base := []Layer{{Files: map[string]string{"a.txt": "hello\n"}, Message: "add a"}}
	renamed := []Layer{{Files: map[string]string{"a.txt": "hello\n"}, Message: "add a (renamed)"}}

	rBase := Build(t, base)
	rRenamed := Build(t, renamed)

	if rBase.Head == rRenamed.Head {
		t.Fatalf("expected different SHAs for different messages, both got %s", rBase.Head)
	}
}

// TestBuild_HeadsPerLayer proves Repo.Heads records one SHA per layer, in
// order, with the last entry equal to Head — corpus fixtures pin frontmatter
// refs and frozen stamps at specific earlier layers' commits, not always the
// final head.
func TestBuild_HeadsPerLayer(t *testing.T) {
	layers := []Layer{
		{Files: map[string]string{"a.txt": "hello\n"}, Message: "add a"},
		{Files: map[string]string{"b.txt": "world\n"}, Message: "add b"},
		{Files: map[string]string{"c.txt": "!\n"}, Message: "add c"},
	}
	r := Build(t, layers)

	if len(r.Heads) != len(layers) {
		t.Fatalf("len(Heads) = %d, want %d", len(r.Heads), len(layers))
	}
	if r.Heads[len(r.Heads)-1] != r.Head {
		t.Fatalf("Heads[last] = %s, want Head = %s", r.Heads[len(r.Heads)-1], r.Head)
	}
	seen := map[string]bool{}
	for i, h := range r.Heads {
		if h == "" {
			t.Fatalf("Heads[%d] is empty", i)
		}
		if seen[h] {
			t.Fatalf("Heads[%d] = %s duplicates an earlier layer's SHA", i, h)
		}
		seen[h] = true
	}

	// Determinism: building the same layers again reproduces every
	// per-layer SHA exactly, not just the final head.
	r2 := Build(t, layers)
	for i := range r.Heads {
		if r.Heads[i] != r2.Heads[i] {
			t.Fatalf("Heads[%d] differs across builds: %s vs %s", i, r.Heads[i], r2.Heads[i])
		}
	}
}

// TestBuild_ProducesWorkingTree sanity-checks that Build leaves a real,
// checked-out working tree behind (not just a bare object store) since
// later phases read fixture files directly off disk.
func TestBuild_ProducesWorkingTree(t *testing.T) {
	r := Build(t, []Layer{
		{Files: map[string]string{"a.txt": "hello\n"}, Message: "add a"},
	})

	got, err := os.ReadFile(r.Dir + "/a.txt")
	if err != nil {
		t.Fatalf("reading committed file: %v", err)
	}
	if string(got) != "hello\n" {
		t.Fatalf("a.txt = %q, want %q", got, "hello\n")
	}
}

// TestBuild_DisablesDetachedGitMaintenance proves the D6-31 fix: every
// fixture repo Build creates must have gc.autoDetach and maintenance.auto
// both forced to false, so a `git commit`/`git add` invocation never forks
// a detached background gc/maintenance child. Without this, the observed
// CI flake class is a detached writer still touching .git when the test's
// t.TempDir() cleanup runs concurrently: "TempDir RemoveAll cleanup:
// unlinkat .../.git: directory not empty".
func TestBuild_DisablesDetachedGitMaintenance(t *testing.T) {
	r := Build(t, []Layer{
		{Files: map[string]string{"a.txt": "hello\n"}, Message: "add a"},
	})

	for _, key := range []string{"gc.autoDetach", "maintenance.auto"} {
		out, err := exec.Command("git", "-C", r.Dir, "config", "--get", key).Output()
		if err != nil {
			t.Fatalf("git config --get %s: %v (fixture repo never sets this key)", key, err)
		}
		if got := strings.TrimSpace(string(out)); got != "false" {
			t.Fatalf("git config --get %s = %q, want %q", key, got, "false")
		}
	}
}
