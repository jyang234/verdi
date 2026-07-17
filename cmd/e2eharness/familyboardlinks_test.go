package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestProvisionFamilyBoardLinks_PairBranch: the ADJ-70 branch-pair rig —
// design/family-links-pair exists carrying BOTH halves of the family
// (feature + implementing story), neither leaks into the serving
// checkout's tree, and the serving checkout is restored. Mirrors
// TestProvisionDraftBoards_Happy's shape over the same store builder.
func TestProvisionFamilyBoardLinks_PairBranch(t *testing.T) {
	storeRoot := newDraftBoardsTestStore(t)

	if err := provisionFamilyBoardLinks(storeRoot); err != nil {
		t.Fatalf("provisionFamilyBoardLinks: %v", err)
	}

	if err := runGit(storeRoot, nil, "rev-parse", "--verify", "refs/heads/"+flPairBranch); err != nil {
		t.Fatalf("pair branch %s missing: %v", flPairBranch, err)
	}
	for _, name := range []string{flPairFeatureName, flPairStoryName} {
		rel := ".verdi/specs/active/" + name + "/spec.md"
		if err := runGit(storeRoot, nil, "cat-file", "-e", flPairBranch+":"+rel); err != nil {
			t.Errorf("%s missing from %s's tree: %v", rel, flPairBranch, err)
		}
		if _, err := os.Stat(filepath.Join(storeRoot, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("%s leaked into the serving checkout's working tree", rel)
		}
	}

	out, err := exec.Command("git", "-C", storeRoot, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != designBranch {
		t.Errorf("serving checkout on %q after provisioning, want %q restored", got, designBranch)
	}
}

// TestProvisionFamilyBoardLinks_Negative_NoRepo: a store that is not a git
// repository fails loudly rather than half-provisioning.
func TestProvisionFamilyBoardLinks_Negative_NoRepo(t *testing.T) {
	if err := provisionFamilyBoardLinks(t.TempDir()); err == nil {
		t.Fatal("provisionFamilyBoardLinks over a non-repo: got nil error")
	}
}
