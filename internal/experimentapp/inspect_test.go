package experimentapp

import (
	"bytes"
	"context"
	"errors"
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

func TestInspectV2RequiresExactCapabilitiesAuthority(t *testing.T) {
	capabilities, err := os.ReadFile(filepath.Join("testdata", "capabilities.json"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name             string
		capabilities     []byte
		definitionChange func([]byte) []byte
		want             Classification
		code             string
	}{
		{name: "exact capabilities authorize", capabilities: capabilities, want: ClassificationClean, code: "clean"},
		{name: "missing capabilities", want: ClassificationOperational, code: "capabilities-unreadable"},
		{
			name:         "capabilities digest mismatch",
			capabilities: bytes.Replace(capabilities, []byte("fixture/1.0.0"), []byte("fixture/1.0.1"), 1),
			want:         ClassificationOperational, code: "capabilities-digest-mismatch",
		},
		{name: "invalid capabilities", capabilities: []byte(`{"schema":"unknown"}`), want: ClassificationOperational, code: "capabilities-invalid"},
		{
			name:         "exact capabilities still require authorization",
			capabilities: capabilities,
			definitionChange: func(data []byte) []byte {
				return bytes.Replace(data, []byte("class: request-path-performance"), []byte("class: storage-throughput"), 1)
			},
			want: ClassificationVerdict, code: "policy-refused",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git := gitFixture(t, "experiment-v2", "request-path-v2")
			if tt.capabilities != nil {
				setAcceptedExperimentFile(t, git, "request-path-v2", "evaluator-capabilities.json", tt.capabilities)
			}
			if tt.definitionChange != nil {
				definitionPath := acceptedExperimentFilePath("request-path-v2", "experiment.yaml")
				git.blobs[definitionPath] = tt.definitionChange(git.blobs[definitionPath])
			}
			policy := &fakePolicyResolver{decision: resolveTestPolicy(t)}
			service, err := NewService(policy, git, &fakeCapabilities{}, &acceptingVerifier{})
			if err != nil {
				t.Fatal(err)
			}
			result := service.Inspect(context.Background(), testIdentity(t, t.TempDir(), "request-path-v2"))
			if result.Outcome.Classification != tt.want || result.Outcome.Code != tt.code {
				t.Fatalf("Inspect() outcome = %+v, want %s/%s", result.Outcome, tt.want, tt.code)
			}
			if tt.want == ClassificationOperational && (result.State.State != "" || result.PolicyDigest != "") {
				t.Fatalf("operational Inspect() exposed favorable state or policy identity: %+v", result)
			}
		})
	}
}

func TestInspectClassifiesOnlyTypedPolicyRefusalAsVerdict(t *testing.T) {
	capabilities, err := os.ReadFile(filepath.Join("testdata", "capabilities.json"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		err  error
		want Classification
		code string
		exit int
	}{
		{name: "disjoint environment policy", err: resolveTestPolicyRefusal(t, "environment"), want: ClassificationVerdict, code: "policy-refused", exit: 1},
		{name: "generic resolver failure", err: errors.New("policy backend unavailable"), want: ClassificationOperational, code: "policy-resolution-failed", exit: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git := gitFixture(t, "experiment-v2", "request-path-v2")
			setAcceptedExperimentFile(t, git, "request-path-v2", "evaluator-capabilities.json", capabilities)
			service, err := NewService(&fakePolicyResolver{err: tt.err}, git, &fakeCapabilities{}, &acceptingVerifier{})
			if err != nil {
				t.Fatal(err)
			}
			result := service.Inspect(context.Background(), testIdentity(t, t.TempDir(), "request-path-v2"))
			if result.Outcome.Classification != tt.want || result.Outcome.Code != tt.code || result.Outcome.ExitCode() != tt.exit {
				t.Fatalf("Inspect() outcome = %+v exit %d, want %s/%s exit %d", result.Outcome, result.Outcome.ExitCode(), tt.want, tt.code, tt.exit)
			}
		})
	}
}
