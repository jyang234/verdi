package constitutionapp

import (
	"context"
	"os"
	"path/filepath"
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
