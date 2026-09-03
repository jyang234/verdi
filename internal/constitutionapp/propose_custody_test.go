package constitutionapp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// retitledOverlay returns the fixture overlay's exact committed bytes with a
// new title, read from testdata rather than from the checkout — these tests
// deliberately mutate the checkout's own store layout before proposing, so
// the content must be sourced before that happens.
func retitledOverlay(t testing.TB, marker string) []byte {
	t.Helper()
	base := readTestdataFile(t, "overlays/frontend-go-version.md")
	return []byte(strings.Replace(base, `title: "Frontend Go version overlay"`, `title: "Frontend Go version overlay (`+marker+`)"`, 1))
}

// TestPropose_RefusesSymlinkedStoreDirectoryComponent is the reviewer's
// symlink-escape probe: an intermediate component of the proposal artifact's
// own path (.verdi/policy/overlays) is a symlink pointing OUTSIDE the
// checkout. filepath.Join + atomicfile.Write resolve that link, so the
// artifact lands in an external directory — outside the repository, outside
// Git, outside review — before `git add` refuses the beyond-a-symlink path.
// Propose must instead refuse before any write, leaving the checkout exactly
// as it found it and creating nothing outside it.
func TestPropose_RefusesSymlinkedStoreDirectoryComponent(t *testing.T) {
	root := buildFixtureRepo(t)
	content := retitledOverlay(t, "symlink-escape")

	outside := t.TempDir()
	overlays := filepath.Join(root, ".verdi", "policy", "overlays")
	if err := os.RemoveAll(overlays); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, overlays); err != nil {
		t.Fatal(err)
	}

	before := captureRepoState(t, root)
	svc := testService()
	_, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch: "policy/symlink-escape", Kind: KindOverlay, Name: "frontend-go-version",
		Content: content, Expected: Expected{Branch: "policy/symlink-escape"},
	})
	escaped := filepath.Join(outside, "frontend-go-version.md")
	if _, err := os.Stat(escaped); !os.IsNotExist(err) {
		t.Fatalf("Propose wrote OUTSIDE the checkout through a symlinked store component: %s exists (stat err = %v)", escaped, err)
	}
	if typed == nil {
		t.Fatal("expected a refusal when a component of the proposal path is a symlink")
	}
	if typed.Code != "unsafe-path" {
		t.Fatalf("code = %q, want unsafe-path (got %+v)", typed.Code, typed)
	}
	assertRefusalLeftRepositoryUntouched(t, root, before, "policy/symlink-escape")
}

// TestPropose_RefusesSymlinkedArtifactDestination is the same custody proof
// for the FINAL component: the artifact path itself is already a symlink.
// A write there replaces a link the repository never reviewed; the
// destination must be absent or a real regular file.
func TestPropose_RefusesSymlinkedArtifactDestination(t *testing.T) {
	root := buildFixtureRepo(t)
	content := retitledOverlay(t, "symlink-destination")

	outside := t.TempDir()
	external := filepath.Join(outside, "frontend-go-version.md")
	if err := os.WriteFile(external, []byte("external\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, ".verdi", "policy", "overlays", "frontend-go-version.md")
	if err := os.Remove(destination); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, destination); err != nil {
		t.Fatal(err)
	}

	before := captureRepoState(t, root)
	svc := testService()
	_, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch: "policy/symlink-destination", Kind: KindOverlay, Name: "frontend-go-version",
		Content: content, Expected: Expected{Branch: "policy/symlink-destination"},
	})
	if typed == nil {
		t.Fatal("expected a refusal when the proposal artifact's own destination is a symlink")
	}
	if typed.Code != "unsafe-path" {
		t.Fatalf("code = %q, want unsafe-path (got %+v)", typed.Code, typed)
	}
	if got, err := os.ReadFile(external); err != nil || string(got) != "external\n" {
		t.Fatalf("Propose wrote through the destination symlink: content = %q, err = %v", got, err)
	}
	assertRefusalLeftRepositoryUntouched(t, root, before, "policy/symlink-destination")
}

// TestPropose_PostCheckoutUnsafePathDisclosesRepositoryEffects catches the
// branch-tree variant of unsafe-path: main is safe, but the existing proposal
// branch itself contains a symlink at the destination. The second custody
// check can run only after checkout, so its refusal must carry the exact
// branch/worktree/index state that remains.
func TestPropose_PostCheckoutUnsafePathDisclosesRepositoryEffects(t *testing.T) {
	root := buildFixtureRepo(t)
	ctx := context.Background()
	const branch = "policy/unsafe-branch-tree"

	runFixtureGit(t, root, "checkout", "-q", "-b", branch)
	overlays := filepath.Join(root, ".verdi", "policy", "overlays")
	if err := os.RemoveAll(overlays); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), overlays); err != nil {
		t.Fatal(err)
	}
	runFixtureGit(t, root, "add", "-A")
	runFixtureGit(t, root, "commit", "-q", "-m", "install unsafe proposal tree")
	branchHead := strings.TrimSpace(runFixtureGit(t, root, "rev-parse", "HEAD"))
	runFixtureGit(t, root, "checkout", "-q", "main")
	initialHead := strings.TrimSpace(runFixtureGit(t, root, "rev-parse", "HEAD"))

	_, typed := testService().Propose(ctx, root, ProposeRequest{
		Branch: branch, Kind: KindOverlay, Name: "frontend-go-version",
		Content:  retitledOverlay(t, "post-checkout-unsafe"),
		Expected: Expected{Branch: branch, Head: branchHead},
	})
	if typed == nil || typed.Classification != ClassificationVerdict || typed.Code != "unsafe-path" {
		t.Fatalf("failure = %+v, want verdict/unsafe-path", typed)
	}
	effects := typed.Failure().RepositoryEffects
	if effects == nil {
		t.Fatal("post-checkout refusal hid the repository effects")
	}
	if effects.Operation != "propose" || effects.InitialBranch != "main" || effects.InitialHead != initialHead ||
		effects.TargetBranch != branch || effects.TargetHeadBefore != branchHead || effects.CurrentBranch != branch ||
		effects.CurrentHead != branchHead || effects.BranchCreated {
		t.Fatalf("branch effects = %+v", effects)
	}
	if len(effects.WorktreePaths) != 0 || len(effects.StagedPaths) != 0 || len(effects.Unproven) != 0 {
		t.Fatalf("non-branch effects = %+v, want known-empty worktree/index state", effects)
	}
}

type commitRefusalGitReader struct{ GitReader }

func (commitRefusalGitReader) CreateCommit(context.Context, string, string) (string, error) {
	return "", errors.New("injected commit refusal")
}

type addRefusalGitReader struct{ GitReader }

func (addRefusalGitReader) AddPaths(context.Context, string, ...string) error {
	return errors.New("injected staging refusal")
}

type observationRefusalGitReader struct{ GitReader }

func (observationRefusalGitReader) CreateCommit(context.Context, string, string) (string, error) {
	return "", errors.New("injected commit refusal")
}

func (observationRefusalGitReader) StagedPaths(context.Context, string) ([]string, error) {
	return nil, errors.New("injected index observation refusal")
}

type identityRefusalGitReader struct{ GitReader }

func (identityRefusalGitReader) StatusDirty(context.Context, string) (bool, error) {
	return false, errors.New("injected post-commit identity refusal")
}

// TestPropose_StageFailureDisclosesBranchAndWorktreeEffects pins the staging
// call site to the same honesty contract as the later commit failure. The
// artifact write has landed, but the real index is still known empty.
func TestPropose_StageFailureDisclosesBranchAndWorktreeEffects(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()
	svc.Git = addRefusalGitReader{GitReader: svc.Git}
	initialHead := strings.TrimSpace(runFixtureGit(t, root, "rev-parse", "HEAD"))
	const branch = "policy/stage-refusal"
	const artifactPath = ".verdi/policy/overlays/frontend-go-version.md"

	_, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch: branch, Kind: KindOverlay, Name: "frontend-go-version",
		Content:  retitledOverlay(t, "stage-refusal"),
		Expected: Expected{Branch: branch},
	})
	if typed == nil || typed.Classification != ClassificationOperational || typed.Code != "io-failure" {
		t.Fatalf("failure = %+v, want operational/io-failure", typed)
	}
	effects := typed.Failure().RepositoryEffects
	if effects == nil || effects.CurrentBranch != branch || effects.CurrentHead != initialHead || !effects.BranchCreated {
		t.Fatalf("branch effects = %+v", effects)
	}
	if want := []string{artifactPath}; !reflect.DeepEqual(effects.WorktreePaths, want) {
		t.Fatalf("worktree_paths = %v, want %v", effects.WorktreePaths, want)
	}
	if len(effects.StagedPaths) != 0 || len(effects.Unproven) != 0 {
		t.Fatalf("staged/unproven effects = %+v, want known-empty", effects)
	}
}

// TestPropose_CommitFailureDisclosesBranchWorktreeAndIndexEffects exercises
// the real checkout, atomic write, and scoped git-add path, failing only the
// final commit. Removing any tracked phase from the failure disclosure must
// make this test fail.
func TestPropose_CommitFailureDisclosesBranchWorktreeAndIndexEffects(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()
	svc.Git = commitRefusalGitReader{GitReader: svc.Git}
	initialHead := strings.TrimSpace(runFixtureGit(t, root, "rev-parse", "HEAD"))
	const branch = "policy/commit-refusal"
	const artifactPath = ".verdi/policy/overlays/frontend-go-version.md"

	_, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch: branch, Kind: KindOverlay, Name: "frontend-go-version",
		Content:  retitledOverlay(t, "commit-refusal"),
		Expected: Expected{Branch: branch}, CommitMessage: "propose reviewed overlay",
	})
	if typed == nil || typed.Classification != ClassificationOperational || typed.Code != "io-failure" {
		t.Fatalf("failure = %+v, want operational/io-failure", typed)
	}
	effects := typed.Failure().RepositoryEffects
	if effects == nil {
		t.Fatal("commit failure hid the repository effects")
	}
	if effects.InitialBranch != "main" || effects.InitialHead != initialHead || effects.TargetBranch != branch ||
		effects.CurrentBranch != branch || effects.CurrentHead != initialHead || !effects.BranchCreated {
		t.Fatalf("branch effects = %+v", effects)
	}
	if want := []string{artifactPath}; !reflect.DeepEqual(effects.WorktreePaths, want) {
		t.Fatalf("worktree_paths = %v, want %v", effects.WorktreePaths, want)
	}
	if want := []string{artifactPath}; !reflect.DeepEqual(effects.StagedPaths, want) {
		t.Fatalf("staged_paths = %v, want %v", effects.StagedPaths, want)
	}
	if len(effects.Unproven) != 0 {
		t.Fatalf("unproven = %v, want exact observed effects", effects.Unproven)
	}
}

// TestPropose_EffectObservationFailureIsExplicitlyUnproven ensures a second
// operational failure while constructing the disclosure cannot collapse an
// unknown repository dimension into a falsely known-empty array.
func TestPropose_EffectObservationFailureIsExplicitlyUnproven(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()
	svc.Git = observationRefusalGitReader{GitReader: svc.Git}

	_, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch: "policy/observation-refusal", Kind: KindOverlay, Name: "frontend-go-version",
		Content:  retitledOverlay(t, "observation-refusal"),
		Expected: Expected{Branch: "policy/observation-refusal"},
	})
	if typed == nil || typed.Classification != ClassificationOperational || typed.Code != "io-failure" {
		t.Fatalf("failure = %+v, want operational/io-failure", typed)
	}
	effects := typed.Failure().RepositoryEffects
	if effects == nil {
		t.Fatal("observation failure hid repository effects")
	}
	if len(effects.StagedPaths) != 0 {
		t.Fatalf("staged_paths = %v, want empty with index disclosed as unproven", effects.StagedPaths)
	}
	if want := []string{"index"}; !reflect.DeepEqual(effects.Unproven, want) {
		t.Fatalf("unproven = %v, want %v", effects.Unproven, want)
	}
}

// TestPropose_PostCommitIdentityFailureDisclosesLandedCommit covers the last
// failure edge in Propose: the commit succeeds, then identity resolution
// fails. CurrentHead must expose the landed commit while the now-clean
// worktree and index remain known empty.
func TestPropose_PostCommitIdentityFailureDisclosesLandedCommit(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()
	svc.Git = identityRefusalGitReader{GitReader: svc.Git}
	initialHead := strings.TrimSpace(runFixtureGit(t, root, "rev-parse", "HEAD"))
	const branch = "policy/post-commit-identity-refusal"

	_, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch: branch, Kind: KindOverlay, Name: "frontend-go-version",
		Content:  retitledOverlay(t, "post-commit-identity-refusal"),
		Expected: Expected{Branch: branch},
	})
	if typed == nil || typed.Classification != ClassificationOperational || typed.Code != "io-failure" {
		t.Fatalf("failure = %+v, want operational/io-failure", typed)
	}
	landedHead := strings.TrimSpace(runFixtureGit(t, root, "rev-parse", "HEAD"))
	if landedHead == initialHead {
		t.Fatal("injected identity failure occurred before the proposal commit landed")
	}
	effects := typed.Failure().RepositoryEffects
	if effects == nil || effects.InitialHead != initialHead || effects.CurrentBranch != branch ||
		effects.CurrentHead != landedHead || !effects.BranchCreated {
		t.Fatalf("landed branch effects = %+v", effects)
	}
	if len(effects.WorktreePaths) != 0 || len(effects.StagedPaths) != 0 || len(effects.Unproven) != 0 {
		t.Fatalf("post-commit residue = %+v, want known-clean worktree/index", effects)
	}
}

// TestPropose_RefusesDivergentUncommittedTarget is the reviewer's
// stale-precondition probe in its A/B/C shape: the proposal branch's
// COMMITTED blob is A, the working tree carries an uncommitted edit B at the
// same path, and the request proposes a third content C. Expected.Head still
// names the branch's real HEAD, so the stale-head precondition passes, and
// atomicfile.Write then erases B without ever disclosing it. Propose must
// instead refuse and name the divergence, leaving B intact for the caller to
// reconcile.
func TestPropose_RefusesDivergentUncommittedTarget(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()
	ctx := context.Background()
	const rel = "overlays/frontend-go-version.md"

	contentA := retitledOverlay(t, "A")
	contentB := retitledOverlay(t, "B")
	contentC := retitledOverlay(t, "C")

	first, typed := svc.Propose(ctx, root, ProposeRequest{
		Branch: "policy/divergent", Kind: KindOverlay, Name: "frontend-go-version",
		Content: contentA, Expected: Expected{Branch: "policy/divergent"},
	})
	if typed != nil {
		t.Fatalf("first Propose: %v", typed)
	}

	// The uncommitted, unrelated edit this request must not erase.
	worktreePath := filepath.Join(root, ".verdi", "policy", filepath.FromSlash(rel))
	if err := os.WriteFile(worktreePath, contentB, 0o644); err != nil {
		t.Fatal(err)
	}

	_, typed = svc.Propose(ctx, root, ProposeRequest{
		Branch: "policy/divergent", Kind: KindOverlay, Name: "frontend-go-version",
		Content: contentC, Expected: Expected{Branch: "policy/divergent", Head: first.Identity.Head},
	})
	if typed == nil {
		t.Fatal("expected a refusal: the working tree carries an uncommitted edit that differs from both the committed blob and the request")
	}
	if typed.Classification != ClassificationVerdict || typed.Code != "divergent-worktree" {
		t.Fatalf("expected verdict/divergent-worktree, got %+v", typed)
	}
	got, err := os.ReadFile(worktreePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(contentB) {
		t.Fatalf("the uncommitted working-tree edit was erased:\ngot  = %q\nwant = %q", got, contentB)
	}
	if committed := committedBlob(t, root, "policy/divergent", rel); committed != string(contentA) {
		t.Fatalf("the refused Propose committed something: %q", committed)
	}
}

// TestPropose_UncommittedTargetMatchingTheRequestStillCommits pins the
// round-1 behavior the divergence refusal must NOT regress: when the
// uncommitted working-tree edit carries exactly the requested bytes there is
// no divergence to reconcile, and Propose commits it (an uncommitted edit is
// a real effect the committed tree does not yet carry).
func TestPropose_UncommittedTargetMatchingTheRequestStillCommits(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()
	ctx := context.Background()
	const rel = "overlays/frontend-go-version.md"

	contentA := retitledOverlay(t, "same-A")
	contentB := retitledOverlay(t, "same-B")

	first, typed := svc.Propose(ctx, root, ProposeRequest{
		Branch: "policy/same-bytes", Kind: KindOverlay, Name: "frontend-go-version",
		Content: contentA, Expected: Expected{Branch: "policy/same-bytes"},
	})
	if typed != nil {
		t.Fatalf("first Propose: %v", typed)
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "policy", filepath.FromSlash(rel)), contentB, 0o644); err != nil {
		t.Fatal(err)
	}

	second, typed := svc.Propose(ctx, root, ProposeRequest{
		Branch: "policy/same-bytes", Kind: KindOverlay, Name: "frontend-go-version",
		Content: contentB, Expected: Expected{Branch: "policy/same-bytes", Head: first.Identity.Head},
	})
	if typed != nil {
		t.Fatalf("second Propose: %v", typed)
	}
	if second.ZeroEffect || second.Commit == "" {
		t.Fatalf("expected the matching uncommitted edit to be committed, got %+v", second)
	}
	if committed := committedBlob(t, root, "policy/same-bytes", rel); committed != string(contentB) {
		t.Fatalf("committed blob = %q, want the proposed content", committed)
	}
}
