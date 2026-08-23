package experimentapp

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestValidateDraftV2RegistrationReadinessUsesOnePolicyResolutionAndWritesNothing(t *testing.T) {
	root := t.TempDir()
	copyExperimentFixture(t, root, "experiment-v2", "request-path-v2")
	before := snapshotWorktree(t, root)
	capabilities, err := os.ReadFile(filepath.Join("testdata", "capabilities.json"))
	if err != nil {
		t.Fatal(err)
	}
	policy := &fakePolicyResolver{decision: resolveTestPolicy(t), mutate: true}
	git := &fakeGit{revision: DefaultBranch{Name: "main", Ref: "refs/remotes/origin/main", Head: testHead}}
	discovery := &fakeCapabilities{bytes: capabilities}
	service, err := NewService(policy, git, discovery, &acceptingVerifier{})
	if err != nil {
		t.Fatal(err)
	}

	result := service.ValidateDraft(context.Background(), testIdentity(t, root, "request-path-v2"))
	if result.Outcome.Classification != ClassificationClean || result.Outcome.ExitCode() != 0 {
		t.Fatalf("ValidateDraft() outcome = %+v", result.Outcome)
	}
	if result.DefinitionDigest == "" || result.CapabilitiesDigest == "" || result.PolicyDigest == "" {
		t.Fatalf("ValidateDraft() omitted sealed identities: %+v", result)
	}
	if policy.calls != 1 || discovery.calls != 1 || git.headCalls != 1 || len(git.treeCalls) != 0 {
		t.Fatalf("calls policy=%d discovery=%d head=%d tree=%v", policy.calls, discovery.calls, git.headCalls, git.treeCalls)
	}
	if got := policy.requests[0].CandidatePaths; !reflect.DeepEqual(got, []string{"spikes/baseline.go", "spikes/cache.go"}) {
		t.Fatalf("policy candidate paths = %#v", got)
	}
	if after := snapshotWorktree(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("ValidateDraft wrote worktree\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestValidateDraftClassifiesV1AndMalformedCapabilityBytes(t *testing.T) {
	tests := []struct {
		name         string
		fixture      string
		experimentID string
		capabilities []byte
		want         Classification
		exit         int
	}{
		{name: "v1 decode-only is not registration-ready", fixture: "experiment-v1", experimentID: "request-path-v1", capabilities: []byte(`{}`), want: ClassificationVerdict, exit: 1},
		{name: "malformed discovery is operational", fixture: "experiment-v2", experimentID: "request-path-v2", capabilities: []byte(`{"schema":"unknown"}`), want: ClassificationOperational, exit: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			copyExperimentFixture(t, root, tt.fixture, tt.experimentID)
			git := &fakeGit{revision: DefaultBranch{Name: "main", Ref: "refs/remotes/origin/main", Head: testHead}}
			policy := &fakePolicyResolver{decision: resolveTestPolicy(t)}
			service, err := NewService(policy, git, &fakeCapabilities{bytes: tt.capabilities}, &acceptingVerifier{})
			if err != nil {
				t.Fatal(err)
			}
			result := service.ValidateDraft(context.Background(), testIdentity(t, root, tt.experimentID))
			if result.Outcome.Classification != tt.want || result.Outcome.ExitCode() != tt.exit {
				t.Fatalf("ValidateDraft() outcome = %+v, want %s exit %d", result.Outcome, tt.want, tt.exit)
			}
			if policy.calls != 1 {
				t.Fatalf("policy calls = %d, want exactly 1", policy.calls)
			}
		})
	}
}

func TestValidateDraftClassifiesOnlyTypedPolicyRefusalAsVerdict(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want Classification
		code string
		exit int
	}{
		{name: "disjoint class policy", err: resolveTestPolicyRefusal(t, "class"), want: ClassificationVerdict, code: "policy-refused", exit: 1},
		{name: "generic resolver failure", err: errors.New("policy backend unavailable"), want: ClassificationOperational, code: "policy-resolution-failed", exit: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			copyExperimentFixture(t, root, "experiment-v2", "request-path-v2")
			service, err := NewService(
				&fakePolicyResolver{err: tt.err},
				&fakeGit{revision: DefaultBranch{Name: "main", Ref: "refs/remotes/origin/main", Head: testHead}},
				&fakeCapabilities{},
				&acceptingVerifier{},
			)
			if err != nil {
				t.Fatal(err)
			}
			result := service.ValidateDraft(context.Background(), testIdentity(t, root, "request-path-v2"))
			if result.Outcome.Classification != tt.want || result.Outcome.Code != tt.code || result.Outcome.ExitCode() != tt.exit {
				t.Fatalf("ValidateDraft() outcome = %+v exit %d, want %s/%s exit %d", result.Outcome, result.Outcome.ExitCode(), tt.want, tt.code, tt.exit)
			}
		})
	}
}

func copyExperimentFixture(t *testing.T, root, fixture, experimentID string) {
	t.Helper()
	source := filepath.Join("testdata", fixture)
	targetRoot := filepath.Join(root, ".verdi", "specs", "active", "request-path-spike", "experiments", experimentID)
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(targetRoot, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy experiment fixture: %v", err)
	}
}

func snapshotWorktree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot worktree: %v", err)
	}
	return out
}
