package fixturegit

import (
	"os"
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
