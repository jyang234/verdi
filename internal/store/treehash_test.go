package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
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
				".verdi/.gitignore":    "data/\n",
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

// TestTreeHash_MutationMatrix drives D4's "staleness is detected, never
// guessed" guarantee across the corpus mutations that must (or must not)
// move the hash. Each case rebuilds the fixture, hashes, mutates the live
// filesystem, and re-hashes. Note that discoverAndHash fails the test if
// TreeHash ever errors, so every case here also proves TreeHash stays
// error-free through the mutation — in particular the working-tree deletion,
// which the old LsFiles+HashObject path turned into a hard failure.
func TestTreeHash_MutationMatrix(t *testing.T) {
	cases := []struct {
		name        string
		mutate      func(t *testing.T, dir string)
		wantChanged bool
	}{
		{
			// Defect 1: a brand-new untracked .verdi/ file the index walk
			// would pick up must move the hash. Under the old LsFiles-only
			// enumeration it was invisible — silent staleness.
			name: "untracked .verdi file added",
			mutate: func(t *testing.T, dir string) {
				writeFileT(t, filepath.Join(dir, ".verdi", "adr", "0002-new.md"),
					"---\nid: adr/0002-new\n---\nbrand new, never git-added\n")
			},
			wantChanged: true,
		},
		{
			// Defect 2: a tracked file deleted from the working tree stays in
			// ls-files but is gone on disk. It must be treated as deleted
			// (hash changes) rather than erroring the whole computation.
			name: "tracked .verdi file deleted from working tree",
			mutate: func(t *testing.T, dir string) {
				if err := os.Remove(filepath.Join(dir, ".verdi", "adr", "0001-x.md")); err != nil {
					t.Fatalf("removing tracked file: %v", err)
				}
			},
			wantChanged: true,
		},
		{
			// .verdi/data/ is gitignored (committed .verdi/.gitignore), so
			// --exclude-standard keeps this untracked file out; the
			// verdiDataPrefix filter is the second line of defence.
			name: "untracked file under .verdi/data/ (gitignored)",
			mutate: func(t *testing.T, dir string) {
				writeFileT(t, filepath.Join(dir, ".verdi", "data", "cache", "whatever"), "noise\n")
			},
			wantChanged: false,
		},
		{
			// The committed (.verdi/) half of the hash is live: a dirty,
			// uncommitted edit is picked up without a new commit (I-15).
			name: "dirty edit to a committed .verdi file",
			mutate: func(t *testing.T, dir string) {
				writeFileT(t, filepath.Join(dir, ".verdi", "adr", "0001-x.md"),
					"---\nid: adr/0001-x\n---\nedited body\n")
			},
			wantChanged: true,
		},
		{
			// D4's whole-corpus claim: a discovered service-root file change
			// invalidates the hash exactly like a spec change would.
			name: "edit to a discovered service-root file",
			mutate: func(t *testing.T, dir string) {
				changed := svcFlowmapYAML + "  - name: second-obligation\n    require: \"x#Y\"\n    before: \"x#Z\"\n"
				writeFileT(t, filepath.Join(dir, "svcfix", flowmapFile), changed)
			},
			wantChanged: true,
		},
		{
			// Converse: touching a file outside .verdi/ and outside every
			// service root must never move the hash.
			name: "edit outside .verdi/ and every service root",
			mutate: func(t *testing.T, dir string) {
				writeFileT(t, filepath.Join(dir, "README.md"), "edited, but out of scope\n")
			},
			wantChanged: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildTreeHashFixture(t, svcFlowmapYAML)
			beforeHash := discoverAndHash(t, repo.Dir)
			tc.mutate(t, repo.Dir)
			afterHash := discoverAndHash(t, repo.Dir)

			switch {
			case tc.wantChanged && beforeHash == afterHash:
				t.Fatalf("TreeHash unchanged after mutation, want it to change (hash=%s)", beforeHash)
			case !tc.wantChanged && beforeHash != afterHash:
				t.Fatalf("TreeHash changed after mutation, want it unchanged: %s -> %s", beforeHash, afterHash)
			}
		})
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
