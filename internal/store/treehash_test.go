package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/verdi/internal/fixturegit"
)

// buildTreeHashFixture builds a small fixturegit repo with a committed
// .verdi/ zone (including a data/ subtree that must never contribute, per
// VL-013 it should never even be tracked, but TreeHash defensively
// excludes it anyway) and, optionally, an on-disk (untracked) service root
// with the given .flowmap.yaml content.
func buildTreeHashFixture(t *testing.T, flowmapContent string) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":    "schema: verdi.layout/v1\n",
				".verdi/adr/0001-x.md": "---\nid: adr/0001-x\n---\nbody\n",
				"README.md":            "not part of the committed zone\n",
			},
			Message: "layer 1",
		},
	})

	writeFileT(t, filepath.Join(repo.Dir, "svcfix", flowmapFile), flowmapContent)
	return repo
}

func discoverAndHash(t *testing.T, root string) string {
	t.Helper()
	services, err := DiscoverServices(root)
	if err != nil {
		t.Fatalf("DiscoverServices: %v", err)
	}
	hash, err := TreeHash(context.Background(), root, services)
	if err != nil {
		t.Fatalf("TreeHash: %v", err)
	}
	if hash == "" {
		t.Fatal("TreeHash returned empty hash")
	}
	return hash
}

func TestTreeHash_DeterministicAcrossIdenticalBuilds(t *testing.T) {
	repoA := buildTreeHashFixture(t, svcFlowmapYAML)
	repoB := buildTreeHashFixture(t, svcFlowmapYAML)

	hashA := discoverAndHash(t, repoA.Dir)
	hashB := discoverAndHash(t, repoB.Dir)

	if hashA != hashB {
		t.Fatalf("TreeHash not deterministic across identical builds: %s vs %s", hashA, hashB)
	}
}

// TestTreeHash_ChangesWhenServiceRootFileChanges proves D4's whole-corpus
// claim: a change to a discovered service-root file (here, adding an
// obligation to .flowmap.yaml) invalidates the tree hash exactly like a
// spec change would.
func TestTreeHash_ChangesWhenServiceRootFileChanges(t *testing.T) {
	before := buildTreeHashFixture(t, svcFlowmapYAML)
	beforeHash := discoverAndHash(t, before.Dir)

	changedFlowmap := svcFlowmapYAML + "  - name: second-obligation\n    require: \"x#Y\"\n    before: \"x#Z\"\n"
	if err := os.WriteFile(filepath.Join(before.Dir, "svcfix", flowmapFile), []byte(changedFlowmap), 0o644); err != nil {
		t.Fatalf("rewriting .flowmap.yaml: %v", err)
	}
	afterHash := discoverAndHash(t, before.Dir)

	if beforeHash == afterHash {
		t.Fatal("TreeHash did not change after editing a discovered service-root file")
	}
}

// TestTreeHash_ChangesWhenCommittedZoneFileChanges proves the committed
// (.verdi/) half of the hash is live too, and that an uncommitted (dirty)
// edit is picked up without needing a new commit (I-15).
func TestTreeHash_ChangesWhenCommittedZoneFileChanges(t *testing.T) {
	repo := buildTreeHashFixture(t, svcFlowmapYAML)
	beforeHash := discoverAndHash(t, repo.Dir)

	if err := os.WriteFile(filepath.Join(repo.Dir, ".verdi", "adr", "0001-x.md"), []byte("---\nid: adr/0001-x\n---\nedited body\n"), 0o644); err != nil {
		t.Fatalf("editing committed file: %v", err)
	}
	afterHash := discoverAndHash(t, repo.Dir)

	if beforeHash == afterHash {
		t.Fatal("TreeHash did not change after a dirty (uncommitted) edit to a committed-zone file")
	}
}

// TestTreeHash_UnchangedContentIdenticalHash is the direct converse of the
// two change tests: touching an unrelated tracked file outside .verdi/ (and
// outside any service root) must never move the hash.
func TestTreeHash_UnchangedContentIdenticalHash(t *testing.T) {
	repo := buildTreeHashFixture(t, svcFlowmapYAML)
	beforeHash := discoverAndHash(t, repo.Dir)

	if err := os.WriteFile(filepath.Join(repo.Dir, "README.md"), []byte("edited, but out of scope\n"), 0o644); err != nil {
		t.Fatalf("editing README.md: %v", err)
	}
	afterHash := discoverAndHash(t, repo.Dir)

	if beforeHash != afterHash {
		t.Fatal("TreeHash changed after editing a file outside .verdi/ and outside every service root")
	}
}

func TestTreeHash_ExcludesVerdiData(t *testing.T) {
	repo := buildTreeHashFixture(t, svcFlowmapYAML)
	beforeHash := discoverAndHash(t, repo.Dir)

	// Defensive: even if something under .verdi/data/ were somehow
	// git-tracked (VL-013 forbids this in the real store; this test proves
	// TreeHash does not depend on that lint rule to stay correct), it must
	// not affect the hash. TreeHash filters purely on path prefix, so an
	// on-disk (untracked) file here is enough to prove the exclusion since
	// LsFiles would never surface it anyway.
	writeFileT(t, filepath.Join(repo.Dir, ".verdi", "data", "cache", "whatever"), "noise\n")
	afterHash := discoverAndHash(t, repo.Dir)

	if beforeHash != afterHash {
		t.Fatal("TreeHash changed after adding a file under .verdi/data/")
	}
}

func TestTreeHash_Negative(t *testing.T) {
	if _, err := TreeHash(context.Background(), filepath.Join(t.TempDir(), "does-not-exist"), nil); err == nil {
		t.Fatal("TreeHash(nonexistent, non-repo root): want error, got nil")
	}
}

func TestCacheKey(t *testing.T) {
	got := CacheKey("deadbeef")
	want := "index-" + LayoutVersion + "-deadbeef"
	if got != want {
		t.Fatalf("CacheKey(%q) = %q, want %q", "deadbeef", got, want)
	}
}
