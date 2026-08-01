package refindex

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/specstate"
)

// fakeGitRunner is the in-process GitRunner double dc-2 requires: every
// ComputeIndex behavior must be provable against a fake with no real git
// process at all, in addition to the hermetic fixturegit exercise
// (refindex_test.go). Each method delegates to an optional func field; a
// nil field panics if called, so a test that forgets to wire a dependency
// fails loudly rather than silently returning a zero value.
type fakeGitRunner struct {
	defaultBranchFn func(ctx context.Context, dir string) (string, error)
	localDesignFn   func(ctx context.Context, dir string) ([]string, error)
	remoteDesignFn  func(ctx context.Context, dir string) ([]string, error)
	showFn          func(ctx context.Context, dir, ref, path string) ([]byte, error)
	listTreeFn      func(ctx context.Context, dir, ref, path string) ([]string, error)
	isAncestorFn    func(ctx context.Context, dir, ancestor, ref string) (bool, error)
}

func (f *fakeGitRunner) DefaultBranch(ctx context.Context, dir string) (string, error) {
	return f.defaultBranchFn(ctx, dir)
}

func (f *fakeGitRunner) LocalDesignBranches(ctx context.Context, dir string) ([]string, error) {
	return f.localDesignFn(ctx, dir)
}

func (f *fakeGitRunner) RemoteDesignBranches(ctx context.Context, dir string) ([]string, error) {
	return f.remoteDesignFn(ctx, dir)
}

func (f *fakeGitRunner) Show(ctx context.Context, dir, ref, path string) ([]byte, error) {
	return f.showFn(ctx, dir, ref, path)
}

func (f *fakeGitRunner) ListTree(ctx context.Context, dir, ref, path string) ([]string, error) {
	return f.listTreeFn(ctx, dir, ref, path)
}

func (f *fakeGitRunner) IsAncestor(ctx context.Context, dir, ancestor, ref string) (bool, error) {
	return f.isAncestorFn(ctx, dir, ancestor, ref)
}

var _ GitRunner = (*fakeGitRunner)(nil)

// fakeStateResolver is the in-process StateResolver double Task 6a's Step 1
// requires: every ComputeIndex behavior that touches lifecycle state must
// be provable against a fake with no real git process, and no real
// specstate.Projector, at all. resolveManyFn is required (a nil field
// panics if called — fakeGitRunner's own "fails loudly, never silently"
// convention); Resolve delegates to it with a one-candidate slice so a
// test only ever has to wire one function. calls counts ResolveMany
// invocations — Step 1's own "a 50-spec fake records ONE default-corpus
// scan, not 50" obligation is proven by asserting calls == 1 after
// ComputeIndex returns, never by inspecting specstate's own internals.
type fakeStateResolver struct {
	resolveManyFn func(ctx context.Context, root string, candidates []specstate.Candidate) ([]specstate.Result, error)
	calls         int
}

func (f *fakeStateResolver) Resolve(ctx context.Context, root string, c specstate.Candidate) (specstate.Result, error) {
	results, err := f.ResolveMany(ctx, root, []specstate.Candidate{c})
	if err != nil {
		return specstate.Result{}, err
	}
	return results[0], nil
}

func (f *fakeStateResolver) ResolveMany(ctx context.Context, root string, candidates []specstate.Candidate) ([]specstate.Result, error) {
	f.calls++
	return f.resolveManyFn(ctx, root, candidates)
}

var _ StateResolver = (*fakeStateResolver)(nil)

// panicResolver is a StateResolver whose ResolveMany panics if ever
// called — for a test whose fixture never reaches a candidate collection
// point at all (e.g. an unconfigured default branch, or a GitRunner
// failure raised before any spec is decoded): wiring this, rather than a
// resolver that quietly returns an empty result, makes an unexpected
// resolver call fail loudly instead of silently passing.
func panicResolver() *fakeStateResolver {
	return &fakeStateResolver{resolveManyFn: func(context.Context, string, []specstate.Candidate) ([]specstate.Result, error) {
		panic("fakeStateResolver: ResolveMany called but this test's fixture should never reach a candidate")
	}}
}

// proposedResolver returns specstate.Proposed (ArtifactStatus "draft") for
// every candidate — a convenience for a test that exercises a design-branch
// draft (whose StatusGroup is unconditionally StatusGroupDraftsInProgress
// regardless of the resolver's answer, ac-3) and does not itself care about
// the resolved state's exact value.
func proposedResolver() *fakeStateResolver {
	return &fakeStateResolver{resolveManyFn: func(_ context.Context, _ string, candidates []specstate.Candidate) ([]specstate.Result, error) {
		out := make([]specstate.Result, len(candidates))
		for i := range candidates {
			out[i] = specstate.Result{State: specstate.Proposed, Relation: specstate.RelationNew}
		}
		return out, nil
	}}
}

// noDesignBranches is a convenience for tests exercising only the
// default-branch walk.
func noDesignBranches() (func(context.Context, string) ([]string, error), func(context.Context, string) ([]string, error)) {
	empty := func(context.Context, string) ([]string, error) { return nil, nil }
	return empty, empty
}

const fakeComponentSpec = `---
id: spec/fake
kind: spec
class: component
title: "Fake"
status: active
owners: [platform-team]
---
# Fake
`

// TestComputeIndex_Fake_DefaultBranchUnconfigured proves an unconfigured
// default branch (gitx.DefaultBranch's own "", nil contract) is treated
// honestly as "nothing to walk", not a fabricated entry or an error.
func TestComputeIndex_Fake_DefaultBranchUnconfigured(t *testing.T) {
	local, remote := noDesignBranches()
	f := &fakeGitRunner{
		defaultBranchFn: func(context.Context, string) (string, error) { return "", nil },
		localDesignFn:   local,
		remoteDesignFn:  remote,
	}
	got, err := ComputeIndex(context.Background(), "/fake/root", f, panicResolver())
	if err != nil {
		t.Fatalf("ComputeIndex: unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ComputeIndex with unconfigured default branch = %v, want empty", got)
	}
}

// TestComputeIndex_Fake_PropagatesOperationalErrors proves every git-runner
// failure propagates as a real Go error (ac-1: "a ref that fails to resolve
// at all ... propagates as a real Go error rather than a silently-skipped
// entry") rather than being swallowed into an empty or partial result.
func TestComputeIndex_Fake_PropagatesOperationalErrors(t *testing.T) {
	sentinel := errors.New("boom")
	local, remote := noDesignBranches()

	t.Run("DefaultBranch fails", func(t *testing.T) {
		f := &fakeGitRunner{
			defaultBranchFn: func(context.Context, string) (string, error) { return "", sentinel },
			localDesignFn:   local,
			remoteDesignFn:  remote,
		}
		if _, err := ComputeIndex(context.Background(), "/fake", f, panicResolver()); !errors.Is(err, sentinel) {
			t.Fatalf("ComputeIndex: want error wrapping %v, got %v", sentinel, err)
		}
	})

	t.Run("ListTree fails during default-branch walk", func(t *testing.T) {
		f := &fakeGitRunner{
			defaultBranchFn: func(context.Context, string) (string, error) { return "main", nil },
			localDesignFn:   local,
			remoteDesignFn:  remote,
			listTreeFn: func(ctx context.Context, dir, ref, path string) ([]string, error) {
				return nil, sentinel
			},
		}
		if _, err := ComputeIndex(context.Background(), "/fake", f, panicResolver()); !errors.Is(err, sentinel) {
			t.Fatalf("ComputeIndex: want error wrapping %v, got %v", sentinel, err)
		}
	})

	t.Run("LocalDesignBranches fails", func(t *testing.T) {
		f := &fakeGitRunner{
			defaultBranchFn: func(context.Context, string) (string, error) { return "", nil },
			localDesignFn: func(context.Context, string) ([]string, error) {
				return nil, sentinel
			},
			remoteDesignFn: remote,
		}
		if _, err := ComputeIndex(context.Background(), "/fake", f, panicResolver()); !errors.Is(err, sentinel) {
			t.Fatalf("ComputeIndex: want error wrapping %v, got %v", sentinel, err)
		}
	})

	t.Run("RemoteDesignBranches fails", func(t *testing.T) {
		f := &fakeGitRunner{
			defaultBranchFn: func(context.Context, string) (string, error) { return "", nil },
			localDesignFn:   local,
			remoteDesignFn: func(context.Context, string) ([]string, error) {
				return nil, sentinel
			},
		}
		if _, err := ComputeIndex(context.Background(), "/fake", f, panicResolver()); !errors.Is(err, sentinel) {
			t.Fatalf("ComputeIndex: want error wrapping %v, got %v", sentinel, err)
		}
	})

	t.Run("IsAncestor fails", func(t *testing.T) {
		f := &fakeGitRunner{
			defaultBranchFn: func(context.Context, string) (string, error) { return "main", nil },
			localDesignFn: func(context.Context, string) ([]string, error) {
				return []string{"design/foo"}, nil
			},
			remoteDesignFn: remote,
			listTreeFn: func(ctx context.Context, dir, ref, path string) ([]string, error) {
				if ref == "main" {
					return nil, nil // empty default-branch tree
				}
				return []string{path}, nil // design branch has its spec.md
			},
			isAncestorFn: func(context.Context, string, string, string) (bool, error) {
				return false, sentinel
			},
		}
		if _, err := ComputeIndex(context.Background(), "/fake", f, panicResolver()); !errors.Is(err, sentinel) {
			t.Fatalf("ComputeIndex: want error wrapping %v, got %v", sentinel, err)
		}
	})
}

// TestComputeIndex_Fake_SourceBoth_SingleEntry proves a branch present in
// both LocalDesignBranches and RemoteDesignBranches folds into exactly one
// SourceBoth entry (ac-2), against the in-process fake — no real git
// process, and no local/remote loop drift, since ComputeIndex reads both
// through mergeDesignSources's one shared path.
func TestComputeIndex_Fake_SourceBoth_SingleEntry(t *testing.T) {
	f := &fakeGitRunner{
		defaultBranchFn: func(context.Context, string) (string, error) { return "", nil },
		localDesignFn: func(context.Context, string) ([]string, error) {
			return []string{"design/both"}, nil
		},
		remoteDesignFn: func(context.Context, string) ([]string, error) {
			return []string{"design/both"}, nil
		},
		listTreeFn: func(ctx context.Context, dir, ref, path string) ([]string, error) {
			return []string{path}, nil
		},
		showFn: func(ctx context.Context, dir, ref, path string) ([]byte, error) {
			return []byte(fakeComponentSpec), nil
		},
		isAncestorFn: func(context.Context, string, string, string) (bool, error) {
			return false, nil
		},
	}
	got, err := ComputeIndex(context.Background(), "/fake", f, proposedResolver())
	if err != nil {
		t.Fatalf("ComputeIndex: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ComputeIndex = %d entries, want exactly 1: %+v", len(got), got)
	}
	if got[0].Source != SourceBoth {
		t.Fatalf("Source = %q, want %q", got[0].Source, SourceBoth)
	}
}

// fakeStatuslessStorySpec is a valid class: story spec.md carrying NO
// status: field at all — Task 4's live scaffold shape (internal/artifact's
// Status is `omitempty`; validateStory tolerates the empty value) — the
// exact shape whose default-branch StatusGroup a raw `spec.Status`
// comparison would have failed closed on, or silently misclassified.
const fakeStatuslessStorySpec = `---
id: spec/fake-story
kind: spec
class: story
title: "Fake Story"
owners: [platform-team]
story: jira:TEST-1
problem: { text: "a problem", anchor: "#problem" }
outcome: { text: "an outcome", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
---
# Fake Story
`

// defaultBranchOneSpecGitRunner returns a fakeGitRunner exposing exactly
// one default-branch active-zone spec at specPath, decoding as content —
// a convenience shared by Step 1's own dedicated RED tests below.
func defaultBranchOneSpecGitRunner(specPath, content string) *fakeGitRunner {
	local, remote := noDesignBranches()
	return &fakeGitRunner{
		defaultBranchFn: func(context.Context, string) (string, error) { return "main", nil },
		localDesignFn:   local,
		remoteDesignFn:  remote,
		listTreeFn: func(ctx context.Context, dir, ref, path string) ([]string, error) {
			if path == ".verdi/specs/active" {
				return []string{specPath}, nil
			}
			return nil, nil
		},
		showFn: func(ctx context.Context, dir, ref, path string) ([]byte, error) {
			return []byte(content), nil
		},
	}
}

// TestComputeIndex_Fake_StatuslessDefaultEntry_AcceptedPendingBuild is Task
// 6a Step 1's own required RED test: a statusless default-branch class:
// story entry maps to StatusGroupAcceptedPendingBuild with SpecStatus
// "accepted-pending-build" — the resolver's Result, never a raw `status:`
// field read (which would have been empty, and mapStatusGroup would have
// failed the whole walk closed on it before this migration).
func TestComputeIndex_Fake_StatuslessDefaultEntry_AcceptedPendingBuild(t *testing.T) {
	f := defaultBranchOneSpecGitRunner(".verdi/specs/active/fake-story/spec.md", fakeStatuslessStorySpec)
	resolver := &fakeStateResolver{resolveManyFn: func(_ context.Context, _ string, candidates []specstate.Candidate) ([]specstate.Result, error) {
		out := make([]specstate.Result, len(candidates))
		for i := range candidates {
			out[i] = specstate.Result{State: specstate.AcceptedPendingBuild, Relation: specstate.RelationExact}
		}
		return out, nil
	}}

	got, err := ComputeIndex(context.Background(), "/fake", f, resolver)
	if err != nil {
		t.Fatalf("ComputeIndex: %v", err)
	}
	e := entryByRef(t, got, "spec/fake-story")
	if e.StatusGroup != StatusGroupAcceptedPendingBuild {
		t.Errorf("StatusGroup = %q, want %q", e.StatusGroup, StatusGroupAcceptedPendingBuild)
	}
	if e.SpecStatus != "accepted-pending-build" {
		t.Errorf("SpecStatus = %q, want %q", e.SpecStatus, "accepted-pending-build")
	}
	if e.Disclosed != nil {
		t.Errorf("Disclosed = %+v, want nil (a proven accepted entry is never disclosed)", e.Disclosed)
	}
}

// TestComputeIndex_Fake_UnmergedDesignEntry_DraftsInProgress is Task 6a
// Step 1's own required RED test: an unmerged design-branch draft maps to
// StatusGroupDraftsInProgress with SpecStatus "draft" — the resolver's
// Result.ArtifactStatus() (specstate.Proposed, since the branch's content
// is by construction unreachable from the default branch), never a raw
// `status:` field read, and never conditioned on the resolver's answer for
// StatusGroup (ac-3's unconditional override, unchanged).
func TestComputeIndex_Fake_UnmergedDesignEntry_DraftsInProgress(t *testing.T) {
	f := &fakeGitRunner{
		defaultBranchFn: func(context.Context, string) (string, error) { return "", nil },
		localDesignFn: func(context.Context, string) ([]string, error) {
			return []string{"design/gamma"}, nil
		},
		remoteDesignFn: func(context.Context, string) ([]string, error) { return nil, nil },
		listTreeFn: func(ctx context.Context, dir, ref, path string) ([]string, error) {
			return []string{path}, nil
		},
		showFn: func(ctx context.Context, dir, ref, path string) ([]byte, error) {
			return []byte(fakeStatuslessStorySpec), nil
		},
	}
	resolver := proposedResolver()

	got, err := ComputeIndex(context.Background(), "/fake", f, resolver)
	if err != nil {
		t.Fatalf("ComputeIndex: %v", err)
	}
	e := entryByRef(t, got, "spec/gamma")
	if e.StatusGroup != StatusGroupDraftsInProgress {
		t.Errorf("StatusGroup = %q, want %q", e.StatusGroup, StatusGroupDraftsInProgress)
	}
	if e.SpecStatus != "draft" {
		t.Errorf("SpecStatus = %q, want %q (specstate.Proposed.ArtifactStatus())", e.SpecStatus, "draft")
	}
}

// TestComputeIndex_Fake_UnprovenEntry_DisclosedNeverAccepted is Task 6a
// Step 1's own required RED test: an unproven default-branch entry carries
// a Disclosed disclosure and never lands in the accepted group — proven
// against a fake resolver returning specstate.Unproven directly (no real
// git failure needed to reach this branch).
func TestComputeIndex_Fake_UnprovenEntry_DisclosedNeverAccepted(t *testing.T) {
	f := defaultBranchOneSpecGitRunner(".verdi/specs/active/fake-story/spec.md", fakeStatuslessStorySpec)
	wantText := "specstate: fake-story cannot be proven not-superseded — the default-branch active-spec scan is incomplete"
	resolver := &fakeStateResolver{resolveManyFn: func(_ context.Context, _ string, candidates []specstate.Candidate) ([]specstate.Result, error) {
		out := make([]specstate.Result, len(candidates))
		for i := range candidates {
			out[i] = specstate.Result{State: specstate.Unproven, Relation: specstate.RelationUnproven, Disclosures: []string{wantText}}
		}
		return out, nil
	}}

	got, err := ComputeIndex(context.Background(), "/fake", f, resolver)
	if err != nil {
		t.Fatalf("ComputeIndex: %v", err)
	}
	e := entryByRef(t, got, "spec/fake-story")
	if e.Disclosed == nil {
		t.Fatal("Disclosed = nil, want a populated disclosure (an unproven entry must never silently pass)")
	}
	if !strings.Contains(e.Disclosed.Text, wantText) {
		t.Errorf("Disclosed.Text = %q, want it to contain specstate's own witness %q", e.Disclosed.Text, wantText)
	}
	if e.StatusGroup == StatusGroupAcceptedPendingBuild {
		t.Fatalf("StatusGroup = %q, want anything but the accepted group (an unproven entry must never enter it)", e.StatusGroup)
	}
}

// TestComputeIndex_Fake_50Specs_OneDefaultCorpusScan is Task 6a Step 1's
// own required RED test: a 50-spec fake records ONE default-corpus scan —
// one resolver.ResolveMany call for the whole default-branch walk — never
// 50 individual Resolve calls (the O(specs²) shape internal/specstate's own
// successor-corpus scan would otherwise incur once per candidate).
func TestComputeIndex_Fake_50Specs_OneDefaultCorpusScan(t *testing.T) {
	const n = 50
	local, remote := noDesignBranches()
	var paths []string
	for i := 0; i < n; i++ {
		paths = append(paths, fmt.Sprintf(".verdi/specs/active/fake-story-%02d/spec.md", i))
	}
	f := &fakeGitRunner{
		defaultBranchFn: func(context.Context, string) (string, error) { return "main", nil },
		localDesignFn:   local,
		remoteDesignFn:  remote,
		listTreeFn: func(ctx context.Context, dir, ref, path string) ([]string, error) {
			if path == ".verdi/specs/active" {
				return paths, nil
			}
			return nil, nil
		},
		showFn: func(ctx context.Context, dir, ref, path string) ([]byte, error) {
			return []byte(fakeStatuslessStorySpec), nil
		},
	}
	resolver := &fakeStateResolver{resolveManyFn: func(_ context.Context, _ string, candidates []specstate.Candidate) ([]specstate.Result, error) {
		out := make([]specstate.Result, len(candidates))
		for i := range candidates {
			out[i] = specstate.Result{State: specstate.AcceptedPendingBuild, Relation: specstate.RelationExact}
		}
		return out, nil
	}}

	got, err := ComputeIndex(context.Background(), "/fake", f, resolver)
	if err != nil {
		t.Fatalf("ComputeIndex: %v", err)
	}
	if len(got) != n {
		t.Fatalf("ComputeIndex = %d entries, want %d", len(got), n)
	}
	if resolver.calls != 1 {
		t.Fatalf("resolver.calls (ResolveMany invocations) = %d, want exactly 1 for %d specs — one default-corpus scan, not %d", resolver.calls, n, n)
	}
}

// forbiddenMethodName matches a method name shaped like a checkout/switch/
// generic-escape-hatch capability — ac-5's static guarantee must hold at
// the GitRunner interface's method set itself, not merely "the current
// implementation happens not to call such a method".
var forbiddenMethodName = regexp.MustCompile(`(?i)checkout|switch|^run$`)

// TestGitRunner_MethodSet_ExposesNoCheckoutCapability reads the GitRunner
// interface's method set via reflection and asserts none of its methods is
// named or shaped like a checkout/switch/generic-run escape hatch — the
// interface itself makes a HEAD-moving call impossible to express, which is
// ac-5's static claim (spec/ref-index ac-5, obligation ac-5--static.md).
func TestGitRunner_MethodSet_ExposesNoCheckoutCapability(t *testing.T) {
	typ := reflect.TypeOf((*GitRunner)(nil)).Elem()
	if typ.NumMethod() == 0 {
		t.Fatal("GitRunner declares no methods at all — did reflection resolve the wrong type?")
	}
	for i := 0; i < typ.NumMethod(); i++ {
		m := typ.Method(i)
		if forbiddenMethodName.MatchString(m.Name) {
			t.Fatalf("GitRunner exposes method %q, which looks like a checkout/switch/generic-run capability — ac-5 requires the port's method set to make this structurally impossible", m.Name)
		}
		// A generic exec.Command-shaped escape hatch (variadic string args
		// returning ([]byte, error) or error) would also defeat ac-5's
		// guarantee even under an innocuous name; none of this interface's
		// methods take a bare variadic string arg list.
		sig := m.Type
		lastIn := sig.NumIn() - 1
		if lastIn >= 0 && sig.IsVariadic() && sig.In(lastIn).Elem().Kind() == reflect.String {
			t.Fatalf("GitRunner method %q takes a variadic string arg list — a generic git-subcommand escape hatch ac-5 forbids", m.Name)
		}
	}
}

// TestGitRunner_MethodNames documents the exact method set this test
// pins against drift, so a future addition is a deliberate, reviewed edit
// to this list rather than a silent expansion of the port's capabilities.
func TestGitRunner_MethodNames(t *testing.T) {
	typ := reflect.TypeOf((*GitRunner)(nil)).Elem()
	var names []string
	for i := 0; i < typ.NumMethod(); i++ {
		names = append(names, typ.Method(i).Name)
	}
	want := "DefaultBranch,IsAncestor,ListTree,LocalDesignBranches,RemoteDesignBranches,Show"
	got := strings.Join(sortedCopy(names), ",")
	if got != want {
		t.Fatalf("GitRunner method set = %q, want %q", got, want)
	}
}

func sortedCopy(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
