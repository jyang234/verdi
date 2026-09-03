package constitutionapp

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func newOverlayContent(t testing.TB, root string) string {
	t.Helper()
	return readFixtureFile(t, root, "overlays/frontend-go-version.md")
}

func TestPropose_CreatesBranchAndCommits(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()
	content := strings.Replace(newOverlayContent(t, root), `title: "Frontend Go version overlay"`, `title: "Frontend Go version overlay (v2)"`, 1)

	result, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch:   "policy/frontend-retitle",
		Kind:     KindOverlay,
		Name:     "frontend-go-version",
		Content:  []byte(content),
		Expected: Expected{Branch: "policy/frontend-retitle"},
	})
	if typed != nil {
		t.Fatalf("Propose: %v", typed)
	}
	if result.ZeroEffect {
		t.Fatal("expected a real effect")
	}
	if result.ArtifactID != "policy-overlay/frontend-go-version" {
		t.Fatalf("artifact id = %q", result.ArtifactID)
	}
	if result.Identity.Branch != "policy/frontend-retitle" {
		t.Fatalf("branch = %q, want the proposal branch", result.Identity.Branch)
	}
}

func TestPropose_ZeroEffectOnAmendWithNoChange(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()
	ctx := context.Background()
	content := []byte(strings.Replace(newOverlayContent(t, root), `title: "Frontend Go version overlay"`, `title: "Frontend Go version overlay (v2)"`, 1))

	first, typed := svc.Propose(ctx, root, ProposeRequest{
		Branch: "policy/no-op", Kind: KindOverlay, Name: "frontend-go-version",
		Content: content, Expected: Expected{Branch: "policy/no-op"},
	})
	if typed != nil {
		t.Fatalf("first Propose: %v", typed)
	}
	if first.ZeroEffect {
		t.Fatal("first propose changed the content relative to main and should not be zero-effect")
	}

	second, typed := svc.Propose(ctx, root, ProposeRequest{
		Branch: "policy/no-op", Kind: KindOverlay, Name: "frontend-go-version",
		Content: content, Expected: Expected{Branch: "policy/no-op", Head: first.Identity.Head},
	})
	if typed != nil {
		t.Fatalf("second Propose: %v", typed)
	}
	if !second.ZeroEffect {
		t.Fatal("expected the second, byte-identical Propose to be zero-effect")
	}
	if second.Commit != "" {
		t.Fatalf("expected no new commit on a zero-effect propose, got %q", second.Commit)
	}
	// The zero-effect claim is only honest if the COMMITTED tree really
	// already carries these exact bytes.
	if got := committedBlob(t, root, "policy/no-op", "overlays/frontend-go-version.md"); got != string(content) {
		t.Fatalf("zero-effect claimed over a committed blob that differs:\ncommitted = %q", got)
	}
}

// TestPropose_UncommittedWorkingTreeEditIsARealEffect proves ZeroEffect is
// measured against the COMMITTED blob at the proposal branch, never against
// the working tree. Probe: commit content A on the branch, then edit the
// working-tree artifact to content B WITHOUT committing, then Propose those
// same B bytes. A working-tree comparison short-circuits as zero-effect and
// silently drops the edit — an exit-0 "success" over a branch whose
// committed tree still carries A (the lost-edit shape). Propose must instead
// commit B, because the committed state is what a proposal submits.
func TestPropose_UncommittedWorkingTreeEditIsARealEffect(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()
	ctx := context.Background()
	const rel = "overlays/frontend-go-version.md"

	base := newOverlayContent(t, root)
	contentA := []byte(strings.Replace(base, `title: "Frontend Go version overlay"`, `title: "Frontend Go version overlay (A)"`, 1))
	contentB := []byte(strings.Replace(base, `title: "Frontend Go version overlay"`, `title: "Frontend Go version overlay (B)"`, 1))

	first, typed := svc.Propose(ctx, root, ProposeRequest{
		Branch: "policy/dirty-artifact", Kind: KindOverlay, Name: "frontend-go-version",
		Content: contentA, Expected: Expected{Branch: "policy/dirty-artifact"},
	})
	if typed != nil {
		t.Fatalf("first Propose: %v", typed)
	}

	// The uncommitted edit: exactly the bytes the next Propose will request.
	if err := os.WriteFile(root+"/.verdi/policy/"+rel, contentB, 0o644); err != nil {
		t.Fatal(err)
	}

	second, typed := svc.Propose(ctx, root, ProposeRequest{
		Branch: "policy/dirty-artifact", Kind: KindOverlay, Name: "frontend-go-version",
		Content: contentB, Expected: Expected{Branch: "policy/dirty-artifact", Head: first.Identity.Head},
	})
	if typed != nil {
		t.Fatalf("second Propose: %v", typed)
	}
	if second.ZeroEffect {
		t.Fatal("an uncommitted working-tree edit is a REAL effect: the branch's committed tree does not carry it yet")
	}
	if second.Commit == "" {
		t.Fatal("expected the uncommitted edit to be committed onto the proposal branch")
	}
	if got := committedBlob(t, root, "policy/dirty-artifact", rel); got != string(contentB) {
		t.Fatalf("the proposal branch's committed tree lacks the proposed content:\ncommitted = %q\nwant      = %q", got, string(contentB))
	}
}

func TestPropose_StaleHeadOnExistingBranch(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()
	ctx := context.Background()
	content := strings.Replace(newOverlayContent(t, root), `title: "Frontend Go version overlay"`, `title: "Frontend Go version overlay (v2)"`, 1)

	first, typed := svc.Propose(ctx, root, ProposeRequest{
		Branch: "policy/stale", Kind: KindOverlay, Name: "frontend-go-version",
		Content: []byte(content), Expected: Expected{Branch: "policy/stale"},
	})
	if typed != nil {
		t.Fatalf("first Propose: %v", typed)
	}

	again := strings.Replace(content, "(v2)", "(v3)", 1)
	_, typed = svc.Propose(ctx, root, ProposeRequest{
		Branch: "policy/stale", Kind: KindOverlay, Name: "frontend-go-version",
		Content: []byte(again), Expected: Expected{Branch: "policy/stale", Head: "not-" + first.Identity.Head},
	})
	if typed == nil {
		t.Fatal("expected a stale-head refusal")
	}
	if typed.Classification != ClassificationVerdict || typed.Code != "stale-head" {
		t.Fatalf("expected verdict/stale-head, got %+v", typed)
	}
}

func TestPropose_StaleHeadOnFreshBranchExpectingExisting(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()

	_, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch: "policy/new", Kind: KindOverlay, Name: "frontend-go-version",
		Content: []byte(newOverlayContent(t, root)), Expected: Expected{Branch: "policy/new", Head: "deadbeef"},
	})
	if typed == nil || typed.Code != "stale-head" {
		t.Fatalf("expected stale-head for a fresh branch whose Expected.Head names a nonexistent commit, got %+v", typed)
	}
}

// TestPropose_NeverSweepsUnrelatedUntrackedFile proves Propose stages and
// commits exactly the one requested artifact path (gitx.AddPaths), never
// gitx.AddAll's whole-tree sweep: an unrelated untracked file elsewhere in
// the checkout (the normal shape of a caller's own --request document
// sitting alongside the checkout, exactly what the CLI/e2e adapters do)
// must neither block Propose nor be absorbed into its commit.
func TestPropose_NeverSweepsUnrelatedUntrackedFile(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()

	siblingPath := root + "/unrelated-request.json"
	if err := os.WriteFile(siblingPath, []byte(`{"not":"part of this proposal"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	content := strings.Replace(newOverlayContent(t, root), `title: "Frontend Go version overlay"`, `title: "Frontend Go version overlay (sibling-file)"`, 1)
	result, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch: "policy/sibling-file", Kind: KindOverlay, Name: "frontend-go-version",
		Content: []byte(content), Expected: Expected{Branch: "policy/sibling-file"},
	})
	if typed != nil {
		t.Fatalf("Propose: %v", typed)
	}
	if result.ZeroEffect || result.Commit == "" {
		t.Fatalf("expected a real committed effect, got %+v", result)
	}
	if _, err := os.Stat(siblingPath); err != nil {
		t.Fatalf("the unrelated sibling file must still exist untouched: %v", err)
	}
	status := gitStatusPorcelain(t, root)
	if !strings.Contains(status, "unrelated-request.json") {
		t.Fatalf("expected the sibling file to remain untracked after Propose, git status: %q", status)
	}
}

func gitStatusPorcelain(t *testing.T, dir string) string {
	t.Helper()
	return gitOut(t, dir, "status", "--porcelain")
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return string(out)
}

// repoState is the complete observable repository state a REFUSED Propose
// must leave byte-identical: which branch is checked out, where it points,
// which local branches exist at all, and what the working tree/index carry.
type repoState struct {
	branch   string
	head     string
	branches string
	status   string
}

func captureRepoState(t *testing.T, dir string) repoState {
	t.Helper()
	return repoState{
		branch:   gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD"),
		head:     gitOut(t, dir, "rev-parse", "HEAD"),
		branches: gitOut(t, dir, "for-each-ref", "--format=%(refname)", "refs/heads"),
		status:   gitOut(t, dir, "status", "--porcelain"),
	}
}

// committedBlob returns the exact bytes ref's committed tree carries at the
// constitution-store-relative path rel — the only state a proposal's
// zero-effect claim may be measured against (a working-tree read would
// short-circuit on an UNCOMMITTED edit and silently drop it).
func committedBlob(t *testing.T, dir, ref, rel string) string {
	t.Helper()
	return gitOut(t, dir, "show", ref+":.verdi/policy/"+rel)
}

// TestPropose_CorruptedContentRefused proves the required corrupted-policy
// scenario AND the refusal's own no-side-effect contract: every content
// validation runs before any repository mutation, so a refused Propose
// leaves branch, HEAD, the local-branch set, and the working tree
// byte-identical to what it found. A refusal that had already created and
// checked out an empty proposal branch would be an undisclosed mutation —
// the Failure envelope carries no identity at all, so the caller could never
// learn its checkout had moved.
func TestPropose_CorruptedContentRefused(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()
	before := captureRepoState(t, root)

	_, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch: "policy/broken", Kind: KindPolicy, Name: "go-toolchain",
		Content: []byte("not a valid policy document"), Expected: Expected{Branch: "policy/broken"},
	})
	if typed == nil {
		t.Fatal("expected a refusal for undecodable content")
	}
	if typed.Code != "corrupted-policy" {
		t.Fatalf("code = %q, want corrupted-policy", typed.Code)
	}
	assertRefusalLeftRepositoryUntouched(t, root, before, "policy/broken")
}

func TestPropose_NameMismatchRefused(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()
	before := captureRepoState(t, root)

	_, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch: "policy/mismatch", Kind: KindOverlay, Name: "wrong-name",
		Content: []byte(newOverlayContent(t, root)), Expected: Expected{Branch: "policy/mismatch"},
	})
	if typed == nil || typed.Code != "name-mismatch" {
		t.Fatalf("expected name-mismatch, got %+v", typed)
	}
	assertRefusalLeftRepositoryUntouched(t, root, before, "policy/mismatch")
}

func assertRefusalLeftRepositoryUntouched(t *testing.T, root string, before repoState, branch string) {
	t.Helper()
	after := captureRepoState(t, root)
	if after != before {
		t.Fatalf("a refused Propose must leave the repository byte-identical:\nbefore = %+v\nafter  = %+v", before, after)
	}
	if strings.Contains(after.branches, "refs/heads/"+branch) {
		t.Fatalf("a refused Propose created branch %q: %q", branch, after.branches)
	}
}

func TestPropose_InputInvalid(t *testing.T) {
	svc := testService()
	type namedCase struct {
		root string
		req  ProposeRequest
	}
	cases := []namedCase{
		{root: "", req: ProposeRequest{}},
		{root: "", req: ProposeRequest{Branch: "b", Expected: Expected{Branch: "other"}}},
		{root: "x", req: ProposeRequest{}},
		{root: "x", req: ProposeRequest{Branch: "b", Expected: Expected{Branch: "other"}}},
		{root: "x", req: ProposeRequest{Branch: "b", Expected: Expected{Branch: "b"}, Kind: "nonsense", Name: "n", Content: []byte("x")}},
		{root: "x", req: ProposeRequest{Branch: "b", Expected: Expected{Branch: "b"}, Kind: KindPolicy, Content: []byte("x")}},
		{root: "x", req: ProposeRequest{Branch: "b", Expected: Expected{Branch: "b"}, Kind: KindPolicy, Name: "n"}},
	}
	for i, c := range cases {
		if _, typed := svc.Propose(context.Background(), c.root, c.req); typed == nil || typed.Classification != ClassificationVerdict {
			t.Fatalf("case %d: expected an input-invalid verdict, got %+v", i, typed)
		}
	}
}

func TestValidateCommitMessage(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    error
	}{
		{name: "default", message: ""},
		{name: "non-whitespace preserved", message: "  reviewed proposal  "},
		{name: "whitespace only", message: " \t\n", want: errCommitMessage},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateCommitMessage(tc.message); got != tc.want {
				t.Fatalf("validateCommitMessage(%q) = %v, want %v", tc.message, got, tc.want)
			}
		})
	}
}

// TestPropose_WhitespaceCommitMessageRefusesBeforeMutation catches the
// validation gap where a non-empty but whitespace-only message passed the
// application preflight, then failed only after Propose had created and
// checked out the branch, written the artifact, and staged it. The refusal is
// an input verdict and must happen before any repository effect.
func TestPropose_WhitespaceCommitMessageRefusesBeforeMutation(t *testing.T) {
	root := buildFixtureRepo(t)
	before := captureRepoState(t, root)

	_, typed := testService().Propose(context.Background(), root, ProposeRequest{
		Branch:        "policy/whitespace-message",
		Kind:          KindOverlay,
		Name:          "frontend-go-version",
		Content:       retitledOverlay(t, "whitespace-message"),
		Expected:      Expected{Branch: "policy/whitespace-message"},
		CommitMessage: " \t\n",
	})
	if typed == nil {
		t.Fatal("expected whitespace-only commit_message to be refused")
	}
	assertRefusalLeftRepositoryUntouched(t, root, before, "policy/whitespace-message")
	if typed.Classification != ClassificationVerdict || typed.Code != "input-invalid" {
		t.Fatalf("failure = %+v, want verdict/input-invalid", typed)
	}
}
