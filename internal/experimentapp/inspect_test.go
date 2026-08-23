package experimentapp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
)

func TestInspectUsesOneAcceptedCommitAndIgnoresDivergentWorktree(t *testing.T) {
	root := t.TempDir()
	worktreePath := filepath.Join(root, ".verdi", "specs", "active", "request-path-spike", "experiments", "request-path-v1", "experiment.yaml")
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(worktreePath, []byte("not: accepted bytes\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	git := gitFixture(t, "experiment-v1", "request-path-v1")
	policy := &fakePolicyResolver{decision: resolveTestPolicy(t)}
	service, err := NewService(policy, git, &fakeCapabilities{}, &acceptingVerifier{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	result := service.Inspect(context.Background(), testIdentity(t, root, "request-path-v1"))
	if result.Outcome.Classification != ClassificationClean || result.Outcome.ExitCode() != 0 {
		t.Fatalf("Inspect() outcome = %+v", result.Outcome)
	}
	if result.AcceptedHead != testHead || result.Definition.Schema != experiment.DefinitionSchemaV1 || result.State.State != experiment.StateExploratory {
		t.Fatalf("Inspect() = %+v", result)
	}
	if policy.calls != 1 {
		t.Fatalf("policy calls = %d, want exactly 1", policy.calls)
	}
	if git.headCalls != 1 || len(git.treeCalls) != 1 || git.treeCalls[0] != testHead {
		t.Fatalf("accepted git calls = head:%d tree:%v", git.headCalls, git.treeCalls)
	}
	for _, call := range git.blobCalls {
		if !strings.HasPrefix(call, testHead+":") {
			t.Fatalf("mixed accepted blob call %q", call)
		}
	}
}

func TestInspectClassifiesStaleAndMalformedAcceptedFacts(t *testing.T) {
	tests := []struct {
		name string
		edit func(*fakeGit)
		want Classification
		exit int
	}{
		{name: "stale expected head", edit: func(g *fakeGit) { g.revision.Head = oldHead }, want: ClassificationVerdict, exit: 1},
		{name: "non-blob experiment entry", edit: func(g *fakeGit) { g.entries[0].Mode = "120000" }, want: ClassificationOperational, exit: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git := gitFixture(t, "experiment-v1", "request-path-v1")
			tt.edit(git)
			service, err := NewService(&fakePolicyResolver{decision: resolveTestPolicy(t)}, git, &fakeCapabilities{}, &acceptingVerifier{})
			if err != nil {
				t.Fatal(err)
			}
			result := service.Inspect(context.Background(), testIdentity(t, t.TempDir(), "request-path-v1"))
			if result.Outcome.Classification != tt.want || result.Outcome.ExitCode() != tt.exit {
				t.Fatalf("Inspect() outcome = %+v, want %s exit %d", result.Outcome, tt.want, tt.exit)
			}
		})
	}
}
