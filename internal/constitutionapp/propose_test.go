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
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status --porcelain: %v", err)
	}
	return string(out)
}

func TestPropose_CorruptedContentRefused(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()

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
}

func TestPropose_NameMismatchRefused(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()

	_, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch: "policy/mismatch", Kind: KindOverlay, Name: "wrong-name",
		Content: []byte(newOverlayContent(t, root)), Expected: Expected{Branch: "policy/mismatch"},
	})
	if typed == nil || typed.Code != "name-mismatch" {
		t.Fatalf("expected name-mismatch, got %+v", typed)
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
