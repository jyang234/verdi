package experimentapp

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentpolicy"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

const (
	testHead = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	oldHead  = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

type fakePolicyResolver struct {
	decision *experimentpolicy.Decision
	calls    int
	requests []PolicyRequest
	mutate   bool
	err      error
}

func (f *fakePolicyResolver) ResolvePolicy(_ context.Context, request PolicyRequest) (*experimentpolicy.Decision, error) {
	f.calls++
	f.requests = append(f.requests, clonePolicyRequest(request))
	if f.mutate {
		if len(request.Definition.Candidates) > 0 {
			request.Definition.Candidates[0].ID = "mutated"
		}
		if len(request.Capabilities.ProtocolVersions) > 0 {
			request.Capabilities.ProtocolVersions[0] = "mutated"
		}
		if len(request.CandidatePaths) > 0 {
			request.CandidatePaths[0] = "mutated"
		}
	}
	return f.decision, f.err
}

type fakeGit struct {
	revision  DefaultBranch
	entries   []GitTreeEntry
	blobs     map[string][]byte
	headCalls int
	treeCalls []string
	blobCalls []string
	err       error
}

func (f *fakeGit) ResolveDefaultBranch(context.Context, string) (DefaultBranch, error) {
	f.headCalls++
	if f.err != nil {
		return DefaultBranch{}, f.err
	}
	return f.revision, nil
}

func (f *fakeGit) ListTree(_ context.Context, _ string, commit string) ([]GitTreeEntry, error) {
	f.treeCalls = append(f.treeCalls, commit)
	return append([]GitTreeEntry(nil), f.entries...), nil
}

func (f *fakeGit) ReadBlob(_ context.Context, _ string, commit, object, path string) ([]byte, error) {
	f.blobCalls = append(f.blobCalls, commit+":"+object+":"+path)
	data, ok := f.blobs[path]
	if !ok {
		return nil, fmt.Errorf("missing fake blob %s", path)
	}
	return append([]byte(nil), data...), nil
}

type fakeCapabilities struct {
	bytes []byte
	calls int
	err   error
}

func (f *fakeCapabilities) DiscoverCapabilities(context.Context, CapabilityRequest) (CapabilityDiscovery, error) {
	f.calls++
	return CapabilityDiscovery{Bytes: append([]byte(nil), f.bytes...)}, f.err
}

type acceptingVerifier struct{ calls int }

func (v *acceptingVerifier) VerifyResult(experiment.Definition, []experiment.Observation, *experiment.ExecutionReceipt, experiment.Result) error {
	v.calls++
	return nil
}

func testActor(t *testing.T) Actor {
	t.Helper()
	actor, err := NewDelegatedAgent("codex", "session-1")
	if err != nil {
		t.Fatal(err)
	}
	return actor
}

func resolveTestPolicy(t *testing.T) *experimentpolicy.Decision {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join("testdata", "policy")
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil || rel == "." {
			return err
		}
		target := filepath.Join(root, ".verdi", "policy", rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy policy fixture: %v", err)
	}
	store, err := policyauthority.Load(root)
	if err != nil {
		t.Fatalf("policyauthority.Load() error = %v", err)
	}
	effective, err := policyauthority.Resolve(store)
	if err != nil {
		t.Fatalf("policyauthority.Resolve() error = %v", err)
	}
	selection, err := contextcompile.SelectApplicablePayloads(effective, experimentpolicy.PayloadKind, contextcompile.PayloadSelectionInput{
		Request:       policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		CandidatePath: ".verdi/specs/active/request-path-spike/experiments/request-path-v2",
		CandidateRef:  "spec/request-path-spike",
		Phase:         contextcompile.PhaseDesign,
		Environment:   "local",
	})
	if err != nil {
		t.Fatalf("SelectApplicablePayloads() error = %v", err)
	}
	decision, err := experimentpolicy.Resolve(selection)
	if err != nil {
		t.Fatalf("experimentpolicy.Resolve() error = %v", err)
	}
	return decision
}

func gitFixture(t *testing.T, fixture, experimentID string) *fakeGit {
	t.Helper()
	base := filepath.Join("testdata", fixture)
	prefix := filepath.ToSlash(filepath.Join(".verdi", "specs", "active", "request-path-spike", "experiments", experimentID))
	blobs := map[string][]byte{}
	var entries []GitTreeEntry
	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		treePath := prefix + "/" + filepath.ToSlash(rel)
		object := fmt.Sprintf("object-%03d", len(entries)+1)
		entries = append(entries, GitTreeEntry{Mode: "100644", Type: "blob", Object: object, Path: treePath})
		blobs[treePath] = data
		return nil
	})
	if err != nil {
		t.Fatalf("read git fixture: %v", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return &fakeGit{
		revision: DefaultBranch{Name: "main", Ref: "refs/remotes/origin/main", Head: testHead},
		entries:  entries,
		blobs:    blobs,
	}
}

func testIdentity(t *testing.T, root, experimentID string) Identity {
	t.Helper()
	return Identity{
		CheckoutRoot:         root,
		Spike:                "spec/request-path-spike",
		ExperimentID:         experimentID,
		ExpectedAcceptedHEAD: testHead,
		Actor:                testActor(t),
	}
}
