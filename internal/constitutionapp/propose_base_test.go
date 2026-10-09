package constitutionapp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/branchbase"
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
					t.Fatalf("proposalBase = %+v, %+v; want an operational %s refusal", got, typed, tt.wantCode)
				}
				return
			}
			if typed != nil || got.Commit != want {
				t.Fatalf("proposalBase = %+v, %+v; want commit %q", got, typed, want)
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

// withOrigin gives root a local bare remote, origin, holding main, so the
// default branch resolves to origin/main (no network: the remote is a
// directory), and returns main's commit.
func withOrigin(t *testing.T, root string) string {
	t.Helper()
	bare := filepath.Join(t.TempDir(), "origin.git")
	runFixtureGit(t, root, "init", "-q", "--bare", bare)
	runFixtureGit(t, root, "remote", "add", "origin", bare)
	runFixtureGit(t, root, "push", "-q", "origin", "main")
	return strings.TrimSpace(runFixtureGit(t, root, "rev-parse", "refs/remotes/origin/main"))
}

// TestPropose_RemoteCollision (ledger SI-367 (3), parent ac-4 under
// SI-333): like build start, a new proposal branch is never cut beside a
// remote-tracking branch of the same name on the remote its base resolves
// from, which the next push would turn into a divergence. With origin/main
// as the base, a name that exists only as origin/<branch> is refused, as
// build start refuses it (operational, exit 2, naming the branch and
// "already exists"), before any mutation; a name on another remote, a
// fresh name, and the amend path of a local branch that is also on origin
// all still complete; a name expected at a head with no local branch keeps
// its stale-head refusal.
func TestPropose_RemoteCollision(t *testing.T) {
	const branch = "policy/remote-collision"
	pushOnly := func(t *testing.T, root string) {
		runFixtureGit(t, root, "branch", branch, "main")
		runFixtureGit(t, root, "push", "-q", "origin", branch)
		runFixtureGit(t, root, "branch", "-D", "-q", branch)
	}
	tests := []struct {
		name     string
		seed     func(t *testing.T, root string)
		expected func(t *testing.T, root string) string // the request's expected head
		wantCode string                                 // "" for a completion
		wantExit int
		detail   string
	}{
		{name: "a fresh name completes", seed: func(*testing.T, string) {}},
		{name: "a name only on the base's remote is refused", seed: pushOnly,
			wantCode: "branch-exists", wantExit: 2, detail: "already exists as refs/remotes/origin/" + branch},
		{name: "a name on another remote is not the base's", seed: func(t *testing.T, root string) {
			runFixtureGit(t, root, "update-ref", "refs/remotes/upstream/"+branch, "main")
		}},
		{name: "a name on the remote expected at a head keeps its stale-head refusal", seed: pushOnly,
			expected: func(t *testing.T, root string) string {
				return strings.TrimSpace(runFixtureGit(t, root, "rev-parse", "refs/remotes/origin/"+branch))
			},
			wantCode: "stale-head", wantExit: 1, detail: "does not exist"},
		{name: "amending a local branch that is also on origin completes", seed: func(t *testing.T, root string) {
			runFixtureGit(t, root, "branch", branch, "main")
			runFixtureGit(t, root, "push", "-q", "origin", branch)
		}, expected: func(t *testing.T, root string) string {
			return strings.TrimSpace(runFixtureGit(t, root, "rev-parse", "refs/heads/"+branch))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := buildFixtureRepo(t)
			main := withOrigin(t, root)
			tt.seed(t, root)
			req := ProposeRequest{
				Branch: branch, Kind: KindOverlay, Name: "frontend-go-version",
				Content: retitledOverlay(t, "remote-collision"), Expected: Expected{Branch: branch},
			}
			if tt.expected != nil {
				req.Expected.Head = tt.expected(t, root)
			}
			before := captureRepoState(t, root)
			res, typed := testService().Propose(context.Background(), root, req)
			if tt.wantCode == "" {
				if typed != nil {
					t.Fatalf("Propose: %+v", typed)
				}
				if parent := strings.TrimSpace(runFixtureGit(t, root, "rev-parse", res.Commit+"^")); parent != main {
					t.Fatalf("the proposal commit's parent = %s, want origin/main's commit %s", parent, main)
				}
				return
			}
			if typed == nil {
				t.Fatalf("Propose completed (commit %s), want a %s refusal", res.Commit, tt.wantCode)
			}
			if typed.Code != tt.wantCode || typed.ExitCode() != tt.wantExit || !strings.Contains(typed.Detail, tt.detail) {
				t.Fatalf("Propose = %+v (exit %d), want code %s, exit %d, detail naming %q", typed, typed.ExitCode(), tt.wantCode, tt.wantExit, tt.detail)
			}
			if typed.RepositoryEffects != nil {
				t.Fatalf("a refusal before any mutation carries repository effects: %+v", typed.RepositoryEffects)
			}
			assertRefusalLeftRepositoryUntouched(t, root, before, branch)
		})
	}
}

// TestProposalCollision is proposalCollision's table: no collision is no
// refusal; a name on the base's remote is build start's class of refusal
// (operational, exit 2, "already exists"); a failed read is an
// operational io-failure, never a pass.
func TestProposalCollision(t *testing.T) {
	const branch = "policy/collision"
	originMain := branchbase.Resolution{Kind: branchbase.ResolvedDefault, Ref: "origin/main", BranchName: "main"}
	tests := []struct {
		name     string
		repo     bool
		seed     string // a ref to create at main, by full name
		wantCode string // "" for no refusal
	}{
		{"no collision", true, "", ""},
		{"a name on the base's remote", true, "refs/remotes/origin/" + branch, "branch-exists"},
		{"a local branch", true, "refs/heads/" + branch, "branch-exists"},
		{"not a repository", false, "", "io-failure"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if tt.repo {
				root = buildFixtureRepo(t)
				if tt.seed != "" {
					runFixtureGit(t, root, "update-ref", tt.seed, "main")
				}
			}
			typed := proposalCollision(context.Background(), root, branch, originMain)
			if tt.wantCode == "" {
				if typed != nil {
					t.Fatalf("proposalCollision = %+v, want none", typed)
				}
				return
			}
			if typed == nil || typed.Code != tt.wantCode || typed.ExitCode() != 2 {
				t.Fatalf("proposalCollision = %+v, want an operational %s refusal", typed, tt.wantCode)
			}
			if tt.seed != "" && !strings.Contains(typed.Detail, "already exists as "+tt.seed) {
				t.Fatalf("proposalCollision detail %q, want it to name %s", typed.Detail, tt.seed)
			}
		})
	}
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
