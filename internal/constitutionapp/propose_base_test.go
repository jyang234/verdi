package constitutionapp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// divergeFromMain checks out a new branch, elsewhere, one commit past
// main with a tree that differs from it, and returns main's commit.
func divergeFromMain(t *testing.T, root string) string {
	t.Helper()
	main := strings.TrimSpace(runFixtureGit(t, root, "rev-parse", "main"))
	runFixtureGit(t, root, "checkout", "-q", "-b", "elsewhere")
	if err := os.WriteFile(filepath.Join(root, "elsewhere.txt"), []byte("only on elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runFixtureGit(t, root, "add", "--", "elsewhere.txt")
	runFixtureGit(t, root, "commit", "-q", "-m", "diverge", "--", "elsewhere.txt")
	return main
}

// TestProposalBase: a new proposal branch's base is the resolved default
// branch's commit (UAT-023), branchbase's disclosed HEAD fallback when no
// origin remote exists, and a refusal when origin exists but its default
// branch does not resolve.
func TestProposalBase(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, root string) (want string)
		wantCode string
	}{
		{"the resolved default branch, not HEAD", func(t *testing.T, root string) string {
			return divergeFromMain(t, root)
		}, ""},
		{"no origin remote keeps the disclosed HEAD fallback", func(t *testing.T, root string) string {
			t.Setenv("CI_DEFAULT_BRANCH", "")
			divergeFromMain(t, root)
			return strings.TrimSpace(runFixtureGit(t, root, "rev-parse", "HEAD"))
		}, ""},
		{"origin with an unresolved default branch is refused", func(t *testing.T, root string) string {
			t.Setenv("CI_DEFAULT_BRANCH", "")
			runFixtureGit(t, root, "remote", "add", "origin", t.TempDir())
			return ""
		}, "accepted-identity-unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := buildFixtureRepo(t)
			want := tt.setup(t, root)
			got, typed := proposalBase(context.Background(), root)
			if tt.wantCode != "" {
				if typed == nil || typed.Code != tt.wantCode || typed.Classification != ClassificationOperational {
					t.Fatalf("proposalBase = %q, %+v; want an operational %s refusal", got, typed, tt.wantCode)
				}
				return
			}
			if typed != nil || got != want {
				t.Fatalf("proposalBase = %q, %+v; want %q", got, typed, want)
			}
		})
	}
}

// TestPropose_NewBranchCutsFromTheDefaultBranch: with HEAD on another
// branch whose tree differs, a new proposal branch is created at main's
// commit and gains only the proposal commit.
func TestPropose_NewBranchCutsFromTheDefaultBranch(t *testing.T) {
	root := buildFixtureRepo(t)
	main := divergeFromMain(t, root)
	const branch = "policy/cut-from-main"
	res, typed := testService().Propose(context.Background(), root, ProposeRequest{
		Branch: branch, Kind: KindOverlay, Name: "frontend-go-version",
		Content: retitledOverlay(t, "cut-from-main"), Expected: Expected{Branch: branch},
	})
	if typed != nil {
		t.Fatalf("Propose: %+v", typed)
	}
	if parent := strings.TrimSpace(runFixtureGit(t, root, "rev-parse", res.Commit+"^")); parent != main {
		t.Fatalf("the proposal commit's parent = %s, want main's commit %s", parent, main)
	}
	if files := strings.TrimSpace(runFixtureGit(t, root, "diff-tree", "--no-commit-id", "--name-only", "-r", res.Commit)); files != ".verdi/policy/overlays/frontend-go-version.md" {
		t.Fatalf("the proposal commit recorded %q, want only the overlay", files)
	}
}

// TestPropose_UnresolvedDefaultBranchRefusesBeforeMutation: the base is
// resolved before the checkout, so its refusal leaves the repository
// untouched and carries no repository effects.
func TestPropose_UnresolvedDefaultBranchRefusesBeforeMutation(t *testing.T) {
	root := buildFixtureRepo(t)
	t.Setenv("CI_DEFAULT_BRANCH", "")
	runFixtureGit(t, root, "remote", "add", "origin", t.TempDir())
	before := captureRepoState(t, root)
	const branch = "policy/no-base"
	_, typed := testService().Propose(context.Background(), root, ProposeRequest{
		Branch: branch, Kind: KindOverlay, Name: "frontend-go-version",
		Content: retitledOverlay(t, "no-base"), Expected: Expected{Branch: branch},
	})
	if typed == nil || typed.Code != "accepted-identity-unavailable" {
		t.Fatalf("Propose = %+v, want an accepted-identity-unavailable refusal", typed)
	}
	if typed.RepositoryEffects != nil {
		t.Fatalf("a refusal before any mutation carries repository effects: %+v", typed.RepositoryEffects)
	}
	assertRefusalLeftRepositoryUntouched(t, root, before, branch)
}

// TestPropose_ScopedCommitOmitsAPreStagedEntry: an entry the caller staged
// before Propose stays staged and out of the proposal commit (UAT-036).
func TestPropose_ScopedCommitOmitsAPreStagedEntry(t *testing.T) {
	root := buildFixtureRepo(t)
	if err := os.WriteFile(filepath.Join(root, "foreign.txt"), []byte("a colleague's staged work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runFixtureGit(t, root, "add", "--", "foreign.txt")
	const branch = "policy/scoped"
	res, typed := testService().Propose(context.Background(), root, ProposeRequest{
		Branch: branch, Kind: KindOverlay, Name: "frontend-go-version",
		Content: retitledOverlay(t, "scoped"), Expected: Expected{Branch: branch},
	})
	if typed != nil {
		t.Fatalf("Propose: %+v", typed)
	}
	if files := strings.TrimSpace(runFixtureGit(t, root, "diff-tree", "--no-commit-id", "--name-only", "-r", res.Commit)); files != ".verdi/policy/overlays/frontend-go-version.md" {
		t.Fatalf("the proposal commit recorded %q, want only the overlay", files)
	}
	if staged := strings.TrimSpace(runFixtureGit(t, root, "diff", "--cached", "--name-only")); staged != "foreign.txt" {
		t.Fatalf("staged after Propose = %q, want the caller's foreign.txt still staged", staged)
	}
}
