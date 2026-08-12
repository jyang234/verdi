package journey

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/repositoryfacts"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

const testFeatureSpecMD = `---
id: spec/payments
kind: spec
class: feature
title: "Payments"
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
---
# body
`

const testFeatureSpecWithStoryMD = `---
id: spec/loans
kind: spec
class: feature
title: "Loans"
owners: [platform-team]
story: jira:LOAN-1482
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
---
# body
`

const testComponentSpecMD = `---
id: spec/shared-lib
kind: spec
class: component
title: "Shared lib"
owners: [platform-team]
status: active
---
# body
`

const testMalformedSpecMD = `---
id: spec/broken
kind: spec
class: feature
title: "Broken"
owners: [platform-team]
unknown_field: true
---
# body
`

func writeSpec(t *testing.T, root, zone, name, content string) {
	t.Helper()
	dir := store.SpecDir(root, zone, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func alwaysUnresolvedDefaultBranch(context.Context, string) (specstate.Branch, bool) {
	return specstate.Branch{}, false
}

// --- target resolution ------------------------------------------------

func TestResolveTargetBytes_DirectRef_Active(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, store.ZoneActive, "payments", testFeatureSpecMD)

	p := newProjector(noOpGitReader(), &fakeStateResolver{}, alwaysUnresolvedDefaultBranch, noOpRepositoryFactsGatherer())
	name, relPath, content, foundOnDisk, err := p.resolveTargetBytes(context.Background(), root, "spec/payments")
	if err != nil {
		t.Fatalf("resolveTargetBytes: %v", err)
	}
	if name != "payments" || relPath != store.ActiveSpecRelPath("payments") || !foundOnDisk {
		t.Fatalf("got name=%q relPath=%q foundOnDisk=%v", name, relPath, foundOnDisk)
	}
	if string(content) != testFeatureSpecMD {
		t.Fatalf("content mismatch")
	}
}

func TestResolveTargetBytes_DirectRef_Archive(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, store.ZoneArchive, "payments", testFeatureSpecMD)

	p := newProjector(noOpGitReader(), &fakeStateResolver{}, alwaysUnresolvedDefaultBranch, noOpRepositoryFactsGatherer())
	_, relPath, _, foundOnDisk, err := p.resolveTargetBytes(context.Background(), root, "spec/payments")
	if err != nil {
		t.Fatalf("resolveTargetBytes: %v", err)
	}
	if relPath != store.SpecRelPath(store.ZoneArchive, "payments") || !foundOnDisk {
		t.Fatalf("got relPath=%q foundOnDisk=%v", relPath, foundOnDisk)
	}
}

func TestResolveTargetBytes_DirectRef_RemoteRefFallback(t *testing.T) {
	root := t.TempDir()
	// No local copy anywhere; the default branch has it.
	git := noOpGitReader()
	git.showFn = func(_ context.Context, dir, ref, path string) ([]byte, error) {
		if ref == "origin/main" && path == store.ActiveSpecRelPath("payments") {
			return []byte(testFeatureSpecMD), nil
		}
		return nil, errors.New("not found at ref")
	}
	resolveDB := func(context.Context, string) (specstate.Branch, bool) {
		return specstate.Branch{Name: "main", Ref: "origin/main"}, true
	}
	p := newProjector(git, &fakeStateResolver{}, resolveDB, noOpRepositoryFactsGatherer())

	name, relPath, content, foundOnDisk, err := p.resolveTargetBytes(context.Background(), root, "spec/payments")
	if err != nil {
		t.Fatalf("resolveTargetBytes: %v", err)
	}
	if name != "payments" || relPath != store.ActiveSpecRelPath("payments") || foundOnDisk {
		t.Fatalf("got name=%q relPath=%q foundOnDisk=%v", name, relPath, foundOnDisk)
	}
	if string(content) != testFeatureSpecMD {
		t.Fatalf("content mismatch: %s", content)
	}
}

func TestResolveTargetBytes_DirectRef_NotFound_NoDefaultBranch(t *testing.T) {
	root := t.TempDir()
	p := newProjector(noOpGitReader(), &fakeStateResolver{}, alwaysUnresolvedDefaultBranch, noOpRepositoryFactsGatherer())

	_, _, _, _, err := p.resolveTargetBytes(context.Background(), root, "spec/nope")
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("resolveTargetBytes error = %v, want *NotFoundError", err)
	}
}

func TestResolveTargetBytes_DirectRef_NotFound_DefaultBranchAlsoMissing(t *testing.T) {
	root := t.TempDir()
	git := noOpGitReader()
	git.showFn = func(context.Context, string, string, string) ([]byte, error) {
		return nil, errors.New("no such path")
	}
	resolveDB := func(context.Context, string) (specstate.Branch, bool) {
		return specstate.Branch{Name: "main", Ref: "origin/main"}, true
	}
	p := newProjector(git, &fakeStateResolver{}, resolveDB, noOpRepositoryFactsGatherer())

	_, _, _, _, err := p.resolveTargetBytes(context.Background(), root, "spec/nope")
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("resolveTargetBytes error = %v, want *NotFoundError", err)
	}
}

func TestResolveTargetBytes_StoryRef_Resolves(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, store.ZoneActive, "loans", testFeatureSpecWithStoryMD)

	p := newProjector(noOpGitReader(), &fakeStateResolver{}, alwaysUnresolvedDefaultBranch, noOpRepositoryFactsGatherer())
	name, relPath, content, foundOnDisk, err := p.resolveTargetBytes(context.Background(), root, "jira:LOAN-1482")
	if err != nil {
		t.Fatalf("resolveTargetBytes: %v", err)
	}
	if name != "loans" || relPath != store.ActiveSpecRelPath("loans") || !foundOnDisk {
		t.Fatalf("got name=%q relPath=%q foundOnDisk=%v", name, relPath, foundOnDisk)
	}
	if string(content) != testFeatureSpecWithStoryMD {
		t.Fatalf("content mismatch")
	}
}

func TestResolveTargetBytes_StoryRef_Unmatched(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "specs", "active"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	p := newProjector(noOpGitReader(), &fakeStateResolver{}, alwaysUnresolvedDefaultBranch, noOpRepositoryFactsGatherer())

	_, _, _, _, err := p.resolveTargetBytes(context.Background(), root, "jira:NOPE-1")
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("resolveTargetBytes error = %v, want *NotFoundError", err)
	}
}

func TestResolveTargetBytes_NeitherForm(t *testing.T) {
	root := t.TempDir()
	p := newProjector(noOpGitReader(), &fakeStateResolver{}, alwaysUnresolvedDefaultBranch, noOpRepositoryFactsGatherer())

	_, _, _, _, err := p.resolveTargetBytes(context.Background(), root, "not-a-valid-arg")
	if err == nil {
		t.Fatal("resolveTargetBytes: want error for a neither-form argument")
	}
	var nf *NotFoundError
	if errors.As(err, &nf) {
		t.Fatalf("want a plain form error, not *NotFoundError: %v", err)
	}
}

func TestDecodeTargetSpec(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{"feature class decodes", testFeatureSpecMD, false},
		{"component class refused", testComponentSpecMD, true},
		{"malformed frontmatter refused", testMalformedSpecMD, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := decodeTargetSpec("spec/x", []byte(tt.content))
			if tt.wantErr {
				if err == nil {
					t.Fatal("decodeTargetSpec: want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeTargetSpec: %v", err)
			}
			if spec == nil {
				t.Fatal("decodeTargetSpec: want non-nil spec")
			}
		})
	}
}

// --- repository facts -----------------------------------------------------
//
// Fine-grained repository-fact-gathering behavior (remote-origin
// canonicalization, branch/HEAD/default-branch/relationship resolution,
// dirty/staged detection, source classification, worktree identity) is
// SI-85's shared internal/repositoryfacts leaf's own responsibility now,
// proven by internal/repositoryfacts/gather_test.go against that
// package's own fakes. This package's remaining, NEW responsibility is
// exactly two things: correctly delegate to the injected
// RepositoryFactsGatherer port, and map its closed DisclosureCode
// vocabulary to this projection's byte-identical prose.

// TestRepositoryDisclosureProse_AllCodesMapped proves every disclosure
// code internal/repositoryfacts.Gather can emit maps to this projection's
// own fixed, previously-hardcoded prose — the exact strings a caller of
// GatherFacts observed before the SI-85 extraction.
func TestRepositoryDisclosureProse_AllCodesMapped(t *testing.T) {
	tests := []struct {
		code repositoryfacts.DisclosureCode
		want string
	}{
		{repositoryfacts.DisclosureRemoteOriginUncanonicalizable, "the origin remote URL could not be canonicalized to a repository identity"},
		{repositoryfacts.DisclosureRemoteOriginNotConfigured, "no origin remote is configured"},
		{repositoryfacts.DisclosureRemoteOriginReadFailed, "remote origin could not be read from this checkout"},
		{repositoryfacts.DisclosureBranchUnresolved, "the current branch could not be determined from this checkout"},
		{repositoryfacts.DisclosureBranchDetached, "the repository is in a detached HEAD state; the current branch is unknown"},
		{repositoryfacts.DisclosureHeadUnresolved, "HEAD could not be resolved from this checkout"},
		{repositoryfacts.DisclosureDefaultBranchRefUnresolved, "the resolved default branch ref could not be resolved to a commit"},
		{repositoryfacts.DisclosureDirtyUnknown, "working-tree dirty state could not be determined from this checkout"},
		{repositoryfacts.DisclosureStagedUnknown, "staged paths could not be determined from this checkout"},
	}
	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			got, ok := repositoryDisclosureProse(tt.code)
			if !ok {
				t.Fatalf("repositoryDisclosureProse(%q): ok = false, want true", tt.code)
			}
			if got != tt.want {
				t.Fatalf("repositoryDisclosureProse(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

// TestRepositoryDisclosureProse_UnknownCodeFailsClosed proves an
// unrecognized disclosure code — a contract-drift bug between
// internal/repositoryfacts and this package's own mapping table — is
// reported rather than silently dropped (CO-1).
func TestRepositoryDisclosureProse_UnknownCodeFailsClosed(t *testing.T) {
	_, ok := repositoryDisclosureProse(repositoryfacts.DisclosureCode("bogus-code"))
	if ok {
		t.Fatal("repositoryDisclosureProse(bogus-code): ok = true, want false")
	}
}

// TestGatherRepositoryFacts_DelegatesToSharedGatherer proves
// gatherRepositoryFacts passes its exact parameters through as a
// GatherInput, returns the shared leaf's Facts unchanged (RepositoryFacts
// is a type alias for repositoryfacts.Facts), and maps every disclosure
// code to this projection's own prose.
func TestGatherRepositoryFacts_DelegatesToSharedGatherer(t *testing.T) {
	wantSnap := repositoryfacts.Snapshot{
		Facts: repositoryfacts.Facts{
			RemoteOrigin:  repositoryfacts.StringFact{Known: true, Value: "example.invalid/repo"},
			Branch:        repositoryfacts.StringFact{Known: true, Value: "main"},
			Head:          repositoryfacts.StringFact{Known: true, Value: "abc123"},
			DefaultBranch: repositoryfacts.DefaultBranchFact{Known: true, Name: "main", Ref: "origin/main", Head: "abc123"},
			Relationship:  repositoryfacts.RelationshipEqual,
			Dirty:         repositoryfacts.BoolFact{Known: true, Value: false},
			Staged:        repositoryfacts.BoolFact{Known: true, Value: false},
			Worktree:      repositoryfacts.WorktreeFact{Managed: false},
			Source:        repositoryfacts.SourceHead,
		},
		Disclosures: []repositoryfacts.DisclosureCode{repositoryfacts.DisclosureRemoteOriginNotConfigured},
	}
	var gotInput repositoryfacts.GatherInput
	fake := &fakeRepositoryFactsGatherer{
		gatherFn: func(_ context.Context, in repositoryfacts.GatherInput) (repositoryfacts.Snapshot, error) {
			gotInput = in
			return wantSnap, nil
		},
	}
	p := newProjector(noOpGitReader(), &fakeStateResolver{}, alwaysUnresolvedDefaultBranch, fake)

	rf, discl, err := p.gatherRepositoryFacts(context.Background(), "the-root", "rel/spec.md", []byte("x"), true)
	if err != nil {
		t.Fatalf("gatherRepositoryFacts: %v", err)
	}
	if gotInput.Root != "the-root" || gotInput.TargetPath != "rel/spec.md" || string(gotInput.TargetContent) != "x" || !gotInput.TargetFoundOnDisk {
		t.Fatalf("GatherInput = %+v, want the exact caller parameters", gotInput)
	}
	if !reflect.DeepEqual(rf, wantSnap.Facts) {
		t.Fatalf("gatherRepositoryFacts Facts = %+v, want %+v (unchanged pass-through)", rf, wantSnap.Facts)
	}
	if len(discl) != 1 || discl[0] != "no origin remote is configured" {
		t.Fatalf("disclosures = %v, want the mapped prose for DisclosureRemoteOriginNotConfigured", discl)
	}
}

// TestGatherRepositoryFacts_PropagatesGatherError proves an error from
// the injected gatherer (the shared leaf's zero-value fail-closed
// contract) is wrapped and returned, never swallowed.
func TestGatherRepositoryFacts_PropagatesGatherError(t *testing.T) {
	fake := &fakeRepositoryFactsGatherer{
		gatherFn: func(context.Context, repositoryfacts.GatherInput) (repositoryfacts.Snapshot, error) {
			return repositoryfacts.Snapshot{}, errors.New("boom")
		},
	}
	p := newProjector(noOpGitReader(), &fakeStateResolver{}, alwaysUnresolvedDefaultBranch, fake)

	_, _, err := p.gatherRepositoryFacts(context.Background(), "root", "rel/spec.md", []byte("x"), true)
	if err == nil {
		t.Fatal("gatherRepositoryFacts: want error when the shared gatherer fails")
	}
}

// --- lifecycle facts -----------------------------------------------------

func TestDerivePosture(t *testing.T) {
	tests := []struct {
		state    specstate.State
		relation specstate.Relation
		want     string
	}{
		{specstate.Unproven, specstate.RelationUnproven, "unknown"},
		{specstate.AcceptedPendingBuild, specstate.RelationExact, "authoritative"},
		{specstate.Closed, specstate.RelationExact, "authoritative"},
		{specstate.Superseded, specstate.RelationExact, "authoritative"},
		{specstate.Proposed, specstate.RelationNew, "advisory"},
		{specstate.Proposed, specstate.RelationDiverged, "advisory"},
		{specstate.AcceptedPendingBuild, specstate.RelationDiverged, "advisory"},
	}
	for _, tt := range tests {
		t.Run(string(tt.state)+"/"+string(tt.relation), func(t *testing.T) {
			if got := derivePosture(tt.state, tt.relation); got != tt.want {
				t.Fatalf("derivePosture(%v, %v) = %q, want %q", tt.state, tt.relation, got, tt.want)
			}
		})
	}
}

func TestResolveActiveBranch(t *testing.T) {
	tests := []struct {
		name      string
		hasLocal  func(context.Context, string, string) (bool, error)
		hasRemote func(context.Context, string, string, string) (bool, error)
		wantKnown bool
		wantValue string
		wantDiscl bool
	}{
		{
			name:      "no branches",
			hasLocal:  func(context.Context, string, string) (bool, error) { return false, nil },
			hasRemote: func(context.Context, string, string, string) (bool, error) { return false, nil },
			wantKnown: false,
		},
		{
			name: "exactly one local design branch",
			hasLocal: func(_ context.Context, _, name string) (bool, error) {
				return name == "design/foo", nil
			},
			hasRemote: func(context.Context, string, string, string) (bool, error) { return false, nil },
			wantKnown: true,
			wantValue: "design/foo",
		},
		{
			name: "same branch local and remote counts once",
			hasLocal: func(_ context.Context, _, name string) (bool, error) {
				return name == "feature/foo", nil
			},
			hasRemote: func(_ context.Context, _, _, branch string) (bool, error) {
				return branch == "feature/foo", nil
			},
			wantKnown: true,
			wantValue: "feature/foo",
		},
		{
			name:      "both design and feature branch exist: ambiguous",
			hasLocal:  func(context.Context, string, string) (bool, error) { return true, nil },
			hasRemote: func(context.Context, string, string, string) (bool, error) { return false, nil },
			wantKnown: false,
			wantDiscl: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git := noOpGitReader()
			git.hasLocalBranchFn = tt.hasLocal
			git.hasRemoteTrackingBranchFn = tt.hasRemote
			p := newProjector(git, &fakeStateResolver{}, alwaysUnresolvedDefaultBranch, noOpRepositoryFactsGatherer())

			fact, discl, err := p.resolveActiveBranch(context.Background(), t.TempDir(), "foo")
			if err != nil {
				t.Fatalf("resolveActiveBranch: %v", err)
			}
			if fact.Known != tt.wantKnown || fact.Value != tt.wantValue {
				t.Fatalf("fact = %+v, want known=%v value=%q", fact, tt.wantKnown, tt.wantValue)
			}
			if tt.wantDiscl && discl == "" {
				t.Fatal("want a disclosure naming the ambiguous branches")
			}
			if !tt.wantDiscl && discl != "" {
				t.Fatalf("want no disclosure, got %q", discl)
			}
		})
	}
}

func TestResolveActiveBranch_Error(t *testing.T) {
	git := noOpGitReader()
	git.hasLocalBranchFn = func(context.Context, string, string) (bool, error) { return false, errors.New("boom") }
	p := newProjector(git, &fakeStateResolver{}, alwaysUnresolvedDefaultBranch, noOpRepositoryFactsGatherer())

	_, _, err := p.resolveActiveBranch(context.Background(), t.TempDir(), "foo")
	if err == nil {
		t.Fatal("resolveActiveBranch: want error")
	}
}

func TestSanitizeDisclosures(t *testing.T) {
	tests := []struct {
		name string
		root string
		in   []string
		want []string
	}{
		{
			name: "replaces every occurrence of root",
			root: "/tmp/repo",
			in:   []string{"specstate: no default branch could be resolved for /tmp/repo"},
			want: []string{"specstate: no default branch could be resolved for <store-root>"},
		},
		{
			name: "no occurrence: unchanged",
			root: "/tmp/repo",
			in:   []string{"specstate: unrelated disclosure"},
			want: []string{"specstate: unrelated disclosure"},
		},
		{
			name: "empty root: no-op",
			root: "",
			in:   []string{"contains /tmp/repo verbatim"},
			want: []string{"contains /tmp/repo verbatim"},
		},
		{
			name: "empty input: no-op",
			root: "/tmp/repo",
			in:   nil,
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeDisclosures(tt.root, tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("sanitizeDisclosures = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("sanitizeDisclosures = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// TestGatherLifecycleFacts_SanitizesRootFromDisclosures proves F1(b) at
// the gatherLifecycleFacts seam: a specstate disclosure embedding root is
// sanitized before it reaches LifecycleFacts.Disclosures AND before it is
// returned as the raw specstate.Result a later derivation stage reads its
// own blocker witnesses from.
func TestGatherLifecycleFacts_SanitizesRootFromDisclosures(t *testing.T) {
	spec, err := decodeTargetSpec("spec/payments", []byte(testFeatureSpecMD))
	if err != nil {
		t.Fatalf("decodeTargetSpec: %v", err)
	}
	root := t.TempDir()
	git := noOpGitReader()
	git.hasLocalBranchFn = func(context.Context, string, string) (bool, error) { return false, nil }
	git.hasRemoteTrackingBranchFn = func(context.Context, string, string, string) (bool, error) { return false, nil }
	state := &fakeStateResolver{
		resolveFn: func(context.Context, string, specstate.Candidate) (specstate.Result, error) {
			return specstate.Result{
				State:       specstate.Unproven,
				Relation:    specstate.RelationUnproven,
				Disclosures: []string{"specstate: no default branch could be resolved for " + root},
			}, nil
		},
	}
	p := newProjector(git, state, alwaysUnresolvedDefaultBranch, noOpRepositoryFactsGatherer())

	lf, result, err := p.gatherLifecycleFacts(context.Background(), root, "rel/spec.md", "payments", []byte(testFeatureSpecMD), spec)
	if err != nil {
		t.Fatalf("gatherLifecycleFacts: %v", err)
	}
	for _, d := range lf.Disclosures {
		if strings.Contains(d, root) {
			t.Fatalf("LifecycleFacts.Disclosures leaks the store root: %q", d)
		}
	}
	if !containsString(lf.Disclosures, "specstate: no default branch could be resolved for <store-root>") {
		t.Fatalf("Disclosures = %v, want the sanitized form", lf.Disclosures)
	}
	for _, d := range result.Disclosures {
		if strings.Contains(d, root) {
			t.Fatalf("returned specstate.Result.Disclosures leaks the store root: %q", d)
		}
	}
}

func TestSortDedupStrings(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil input yields empty non-nil", nil, []string{}},
		{"dedups and sorts", []string{"b", "a", "b"}, []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortDedupStrings(tt.in)
			if got == nil {
				t.Fatal("sortDedupStrings: want non-nil result")
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestGatherLifecycleFacts_HappyPath(t *testing.T) {
	spec, err := decodeTargetSpec("spec/payments", []byte(testFeatureSpecMD))
	if err != nil {
		t.Fatalf("decodeTargetSpec: %v", err)
	}

	git := noOpGitReader()
	git.hasLocalBranchFn = func(context.Context, string, string) (bool, error) { return false, nil }
	git.hasRemoteTrackingBranchFn = func(context.Context, string, string, string) (bool, error) { return false, nil }
	state := &fakeStateResolver{
		resolveFn: func(context.Context, string, specstate.Candidate) (specstate.Result, error) {
			return specstate.Result{
				State:    specstate.AcceptedPendingBuild,
				Relation: specstate.RelationExact,
				Baseline: &specstate.Baseline{Path: "p", Blob: "b", LandingCommit: "c"},
			}, nil
		},
	}
	p := newProjector(git, state, alwaysUnresolvedDefaultBranch, noOpRepositoryFactsGatherer())

	lf, result, err := p.gatherLifecycleFacts(context.Background(), t.TempDir(), "rel/spec.md", "payments", []byte(testFeatureSpecMD), spec)
	if err != nil {
		t.Fatalf("gatherLifecycleFacts: %v", err)
	}
	if lf.Class != "feature" || lf.State != "accepted-pending-build" || lf.Relation != "exact" || lf.Posture != "authoritative" {
		t.Fatalf("lf = %+v", lf)
	}
	if lf.AcceptedBaseline == nil || lf.AcceptedBaseline.Path != "p" {
		t.Fatalf("AcceptedBaseline = %+v", lf.AcceptedBaseline)
	}
	if lf.ActiveBranch.Known {
		t.Fatalf("ActiveBranch = %+v, want unknown", lf.ActiveBranch)
	}
	if lf.Disclosures == nil {
		t.Fatal("Disclosures must be non-nil")
	}
	if result.State != specstate.AcceptedPendingBuild {
		t.Fatalf("result.State = %v", result.State)
	}
}

func TestGatherLifecycleFacts_PartialBaselineNeverMapped(t *testing.T) {
	spec, err := decodeTargetSpec("spec/payments", []byte(testFeatureSpecMD))
	if err != nil {
		t.Fatalf("decodeTargetSpec: %v", err)
	}
	git := noOpGitReader()
	git.hasLocalBranchFn = func(context.Context, string, string) (bool, error) { return false, nil }
	git.hasRemoteTrackingBranchFn = func(context.Context, string, string, string) (bool, error) { return false, nil }
	state := &fakeStateResolver{
		resolveFn: func(context.Context, string, specstate.Candidate) (specstate.Result, error) {
			// A diverged candidate's own partial baseline (Path/Blob set,
			// LandingCommit empty) — specstate/resolve.go's own documented
			// shape for RelationDiverged.
			return specstate.Result{
				State:    specstate.Proposed,
				Relation: specstate.RelationDiverged,
				Baseline: &specstate.Baseline{Path: "p", Blob: "b", LandingCommit: ""},
			}, nil
		},
	}
	p := newProjector(git, state, alwaysUnresolvedDefaultBranch, noOpRepositoryFactsGatherer())

	lf, _, err := p.gatherLifecycleFacts(context.Background(), t.TempDir(), "rel/spec.md", "payments", []byte(testFeatureSpecMD), spec)
	if err != nil {
		t.Fatalf("gatherLifecycleFacts: %v", err)
	}
	if lf.AcceptedBaseline != nil {
		t.Fatalf("AcceptedBaseline = %+v, want nil for a partial (never-landed) baseline", lf.AcceptedBaseline)
	}
}

func TestGatherLifecycleFacts_ResolveError(t *testing.T) {
	spec, err := decodeTargetSpec("spec/payments", []byte(testFeatureSpecMD))
	if err != nil {
		t.Fatalf("decodeTargetSpec: %v", err)
	}
	state := &fakeStateResolver{
		resolveFn: func(context.Context, string, specstate.Candidate) (specstate.Result, error) {
			return specstate.Result{}, errors.New("boom")
		},
	}
	p := newProjector(noOpGitReader(), state, alwaysUnresolvedDefaultBranch, noOpRepositoryFactsGatherer())

	_, _, err = p.gatherLifecycleFacts(context.Background(), t.TempDir(), "rel/spec.md", "payments", []byte(testFeatureSpecMD), spec)
	if err == nil {
		t.Fatal("gatherLifecycleFacts: want error")
	}
}

// --- GatherFacts (integration of the pieces above, still fake-backed) ---

func TestGatherFacts_HappyPath(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, store.ZoneActive, "payments", testFeatureSpecMD)
	if err := os.MkdirAll(filepath.Join(root, ".verdi"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	git := noOpGitReader()
	git.hasLocalBranchFn = func(context.Context, string, string) (bool, error) { return false, nil }
	git.hasRemoteTrackingBranchFn = func(context.Context, string, string, string) (bool, error) { return false, nil }
	state := &fakeStateResolver{
		resolveFn: func(context.Context, string, specstate.Candidate) (specstate.Result, error) {
			return specstate.Result{State: specstate.Proposed, Relation: specstate.RelationNew}, nil
		},
	}
	repoFacts := &fakeRepositoryFactsGatherer{
		gatherFn: func(context.Context, repositoryfacts.GatherInput) (repositoryfacts.Snapshot, error) {
			return repositoryfacts.Snapshot{
				Facts:       repositoryfacts.Facts{Relationship: repositoryfacts.RelationshipUnknown, Source: repositoryfacts.SourceHead},
				Disclosures: []repositoryfacts.DisclosureCode{},
			}, nil
		},
	}
	p := newProjector(git, state, alwaysUnresolvedDefaultBranch, repoFacts)

	facts, err := p.GatherFacts(context.Background(), &store.Config{Root: root}, "spec/payments")
	if err != nil {
		t.Fatalf("GatherFacts: %v", err)
	}
	if facts.Target.Ref != "spec/payments" || facts.Target.Class != "feature" {
		t.Fatalf("Target = %+v", facts.Target)
	}
	if facts.Lifecycle.State != "proposed" {
		t.Fatalf("Lifecycle.State = %q", facts.Lifecycle.State)
	}
	if len(facts.Owners) != 1 || facts.Owners[0] != "platform-team" {
		t.Fatalf("Owners = %v", facts.Owners)
	}
}

func TestGatherFacts_NotFound(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".verdi"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	p := newProjector(noOpGitReader(), &fakeStateResolver{}, alwaysUnresolvedDefaultBranch, noOpRepositoryFactsGatherer())

	_, err := p.GatherFacts(context.Background(), &store.Config{Root: root}, "spec/nope")
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("GatherFacts error = %v, want *NotFoundError", err)
	}
}

func TestGatherFacts_ComponentClassRefused(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, store.ZoneActive, "shared-lib", testComponentSpecMD)
	p := newProjector(noOpGitReader(), &fakeStateResolver{}, alwaysUnresolvedDefaultBranch, noOpRepositoryFactsGatherer())

	_, err := p.GatherFacts(context.Background(), &store.Config{Root: root}, "spec/shared-lib")
	if err == nil {
		t.Fatal("GatherFacts: want error for a component-class target")
	}
}

func containsString(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func TestNotFoundError_Error(t *testing.T) {
	err := &NotFoundError{Ref: "spec/x"}
	if !strings.Contains(err.Error(), "spec/x") {
		t.Fatalf("Error() = %q, want it to name the ref", err.Error())
	}
}

// --- evidence facts ------------------------------------------------------

func TestGatherEvidenceFacts(t *testing.T) {
	const unknownOperandsDisclosure = "Context Integrity's canonical evidence-authority and freshness operands are not consumed by this delivery unit; evidence posture and freshness are unknown"

	tests := []struct {
		name             string
		criteria         []artifact.AcceptanceCriterion
		wantContributors []EvidenceContributor
		wantDisclosures  []string
	}{
		{
			name: "one declared kind yields one contributor",
			criteria: []artifact.AcceptanceCriterion{
				{ID: "ac-1", Evidence: []artifact.EvidenceKind{artifact.EvidenceStatic}},
			},
			wantContributors: []EvidenceContributor{
				{ID: "static", Kind: "static", Resolution: "unproven", Witness: evidenceContributorWitness("static")},
			},
			wantDisclosures: []string{unknownOperandsDisclosure},
		},
		{
			name: "distinct kinds across criteria are deduped and sorted by id",
			criteria: []artifact.AcceptanceCriterion{
				{ID: "ac-1", Evidence: []artifact.EvidenceKind{artifact.EvidenceStatic, artifact.EvidenceBehavioral}},
				{ID: "ac-2", Evidence: []artifact.EvidenceKind{artifact.EvidenceStatic, artifact.EvidenceAttestation}},
				{ID: "ac-3", Evidence: []artifact.EvidenceKind{artifact.EvidenceRuntime}},
			},
			wantContributors: []EvidenceContributor{
				{ID: "attestation", Kind: "attestation", Resolution: "unproven", Witness: evidenceContributorWitness("attestation")},
				{ID: "behavioral", Kind: "behavioral", Resolution: "unproven", Witness: evidenceContributorWitness("behavioral")},
				{ID: "runtime", Kind: "runtime", Resolution: "unproven", Witness: evidenceContributorWitness("runtime")},
				{ID: "static", Kind: "static", Resolution: "unproven", Witness: evidenceContributorWitness("static")},
			},
			wantDisclosures: []string{unknownOperandsDisclosure},
		},
		{
			// A spec that declares no evidence kinds at all (artifact's own
			// Validate forbids it for feature/story specs, so this is the
			// defensive branch): an empty contributor set must say so
			// rather than read as "no evidence is required".
			name:             "no declared kinds discloses the empty contributor set",
			criteria:         nil,
			wantContributors: []EvidenceContributor{},
			// Sorted, like every disclosure list in this schema.
			wantDisclosures: []string{
				unknownOperandsDisclosure,
				"the target declares no acceptance-criteria evidence kinds; this projection derives no evidence contributors for it",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gatherEvidenceFacts(&artifact.SpecFrontmatter{AcceptanceCriteria: tt.criteria})

			// This delivery unit consumes no Context Integrity operands,
			// so both always-visible operands are honestly unknown.
			if got.Authority != "unknown" {
				t.Errorf("Authority = %q, want unknown", got.Authority)
			}
			if got.Freshness != "unknown" {
				t.Errorf("Freshness = %q, want unknown", got.Freshness)
			}
			if !reflect.DeepEqual(got.Contributors, tt.wantContributors) {
				t.Errorf("Contributors = %+v, want %+v", got.Contributors, tt.wantContributors)
			}
			if !reflect.DeepEqual(got.Disclosures, tt.wantDisclosures) {
				t.Errorf("Disclosures = %q, want %q", got.Disclosures, tt.wantDisclosures)
			}
			if err := got.validate(); err != nil {
				t.Errorf("derived EvidenceFacts.validate() = %v, want nil", err)
			}
		})
	}
}

// TestGatherEvidenceFacts_UnknownKindIsRefused is the negative twin: a
// kind outside artifact's closed catalog must never reach the record as
// though it were a legal contributor. artifact.DecodeSpec rejects such a
// spec long before this point, so the derived section simply must fail
// its own Validate rather than emitting a contributor nobody can read.
func TestGatherEvidenceFacts_UnknownKindIsRefused(t *testing.T) {
	got := gatherEvidenceFacts(&artifact.SpecFrontmatter{
		AcceptanceCriteria: []artifact.AcceptanceCriterion{
			{ID: "ac-1", Evidence: []artifact.EvidenceKind{"vibes"}},
		},
	})
	if err := got.validate(); err == nil {
		t.Fatalf("EvidenceFacts.validate() = nil for contributor kind %q, want a fail-closed error", "vibes")
	}
}

// TestGatherEvidenceFacts_Deterministic proves two derivations over the
// same spec agree exactly, whatever order the criteria declared their
// kinds in — the record digest depends on it.
func TestGatherEvidenceFacts_Deterministic(t *testing.T) {
	a := gatherEvidenceFacts(&artifact.SpecFrontmatter{
		AcceptanceCriteria: []artifact.AcceptanceCriterion{
			{ID: "ac-1", Evidence: []artifact.EvidenceKind{artifact.EvidenceRuntime, artifact.EvidenceStatic}},
			{ID: "ac-2", Evidence: []artifact.EvidenceKind{artifact.EvidenceBehavioral}},
		},
	})
	b := gatherEvidenceFacts(&artifact.SpecFrontmatter{
		AcceptanceCriteria: []artifact.AcceptanceCriterion{
			{ID: "ac-2", Evidence: []artifact.EvidenceKind{artifact.EvidenceBehavioral}},
			{ID: "ac-1", Evidence: []artifact.EvidenceKind{artifact.EvidenceStatic, artifact.EvidenceRuntime}},
		},
	})
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("gatherEvidenceFacts is order-dependent:\n a = %+v\n b = %+v", a, b)
	}
}

// --- story-ref resolution across BOTH spec classes -----------------------

// testStorySpecMD is a class: story spec carrying its own REQUIRED story:
// field — the form storyresolve.Resolve deliberately never matches (its
// scan is feature-class only), and therefore the form this projection has
// to reach for itself.
const testStorySpecMD = `---
id: spec/loan-api
kind: spec
class: story
title: "Loan API"
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
story: jira:LOAN-1482
links:
  - { type: implements, ref: "spec/loans#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
---
# body
`

// testSecondStorySpecMD is a SECOND class: story spec carrying the same
// story: ref — the story-versus-story collision.
const testSecondStorySpecMD = `---
id: spec/loan-ui
kind: spec
class: story
title: "Loan UI"
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
story: jira:LOAN-1482
links:
  - { type: implements, ref: "spec/loans#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
---
# body
`

// TestResolveTargetBytes_StoryRef_StoryClassReachable proves a class:
// story target is reachable by its own story ref. storyresolve.Resolve
// matches active FEATURE specs only, so before this the story-class form
// was simply unreachable through `verdi journey`.
func TestResolveTargetBytes_StoryRef_StoryClassReachable(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, store.ZoneActive, "loan-api", testStorySpecMD)

	p := newProjector(noOpGitReader(), &fakeStateResolver{}, alwaysUnresolvedDefaultBranch, noOpRepositoryFactsGatherer())
	name, relPath, content, foundOnDisk, err := p.resolveTargetBytes(context.Background(), root, "jira:LOAN-1482")
	if err != nil {
		t.Fatalf("resolveTargetBytes: %v", err)
	}
	if name != "loan-api" || relPath != store.ActiveSpecRelPath("loan-api") || !foundOnDisk {
		t.Fatalf("got name=%q relPath=%q foundOnDisk=%v", name, relPath, foundOnDisk)
	}
	if string(content) != testStorySpecMD {
		t.Fatalf("content mismatch")
	}
}

// TestResolveTargetBytes_StoryRef_AmbiguousFailsClosed is the collision
// the real showcase corpus already contains (stale-decline, class:
// feature, and borrower-update-api, class: story, both carry
// story: jira:LOAN-1482): with two specs answering to one ref, silently
// picking either is a wrong answer, so the projection refuses and names
// both.
func TestResolveTargetBytes_StoryRef_AmbiguousFailsClosed(t *testing.T) {
	tests := []struct {
		name    string
		specs   map[string]string
		wantIDs []string
	}{
		{
			"feature and story carry the same ref",
			map[string]string{"loans": testFeatureSpecWithStoryMD, "loan-api": testStorySpecMD},
			[]string{"spec/loan-api", "spec/loans"},
		},
		{
			"two stories carry the same ref",
			map[string]string{"loan-api": testStorySpecMD, "loan-ui": testSecondStorySpecMD},
			[]string{"spec/loan-api", "spec/loan-ui"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for name, content := range tt.specs {
				writeSpec(t, root, store.ZoneActive, name, content)
			}

			p := newProjector(noOpGitReader(), &fakeStateResolver{}, alwaysUnresolvedDefaultBranch, noOpRepositoryFactsGatherer())
			_, _, _, _, err := p.resolveTargetBytes(context.Background(), root, "jira:LOAN-1482")
			if err == nil {
				t.Fatal("resolveTargetBytes: want an ambiguity refusal, got nil")
			}
			// Ambiguity is operational, never a NotFound verdict: the ref
			// resolves to too much, not to nothing.
			var nf *NotFoundError
			if errors.As(err, &nf) {
				t.Fatalf("ambiguity surfaced as *NotFoundError: %v", err)
			}
			for _, id := range tt.wantIDs {
				if !strings.Contains(err.Error(), id) {
					t.Fatalf("error = %q, want it to name every match (%v)", err.Error(), tt.wantIDs)
				}
			}
			// Sorted, so the refusal is byte-identical across runs.
			first, second := strings.Index(err.Error(), tt.wantIDs[0]), strings.Index(err.Error(), tt.wantIDs[1])
			if first > second {
				t.Fatalf("error = %q, want the matches named in ascending id order", err.Error())
			}
			if !strings.Contains(err.Error(), "spec/<name>") {
				t.Fatalf("error = %q, want it to instruct qualification via spec/<name>", err.Error())
			}
		})
	}
}

// TestNotFoundError_SearchedIsHonestPerForm pins F-4's honesty fix: the
// refusal must state what was ACTUALLY searched. The story-ref form scans
// only the working tree's active zone; the direct spec-ref form scans
// active and archive locally AND both zones at the configured default
// branch. Claiming the latter for the former was simply false.
func TestNotFoundError_SearchedIsHonestPerForm(t *testing.T) {
	t.Run("story-ref form does not claim the default branch", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".verdi", "specs", "active"), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		// A default branch DOES resolve here — so a message claiming it was
		// searched would be false rather than merely imprecise.
		resolveDB := func(context.Context, string) (specstate.Branch, bool) {
			return specstate.Branch{Name: "main", Ref: "origin/main"}, true
		}
		p := newProjector(noOpGitReader(), &fakeStateResolver{}, resolveDB, noOpRepositoryFactsGatherer())

		_, _, _, _, err := p.resolveTargetBytes(context.Background(), root, "jira:NOPE-1")
		var nf *NotFoundError
		if !errors.As(err, &nf) {
			t.Fatalf("error = %v, want *NotFoundError", err)
		}
		msg := nf.Error()
		if !strings.Contains(msg, "jira:NOPE-1") {
			t.Fatalf("Error() = %q, want it to name the ref", msg)
		}
		if strings.Contains(msg, "default branch") {
			t.Fatalf("Error() = %q, but the story-ref form never searches the default branch", msg)
		}
		if !strings.Contains(msg, "active") {
			t.Fatalf("Error() = %q, want it to name the active zone it actually scanned", msg)
		}
	})

	t.Run("spec-ref form names both zones and the default branch", func(t *testing.T) {
		root := t.TempDir()
		git := noOpGitReader()
		git.showFn = func(context.Context, string, string, string) ([]byte, error) {
			return nil, errors.New("no such path")
		}
		resolveDB := func(context.Context, string) (specstate.Branch, bool) {
			return specstate.Branch{Name: "main", Ref: "origin/main"}, true
		}
		p := newProjector(git, &fakeStateResolver{}, resolveDB, noOpRepositoryFactsGatherer())

		_, _, _, _, err := p.resolveTargetBytes(context.Background(), root, "spec/nope")
		var nf *NotFoundError
		if !errors.As(err, &nf) {
			t.Fatalf("error = %v, want *NotFoundError", err)
		}
		msg := nf.Error()
		for _, want := range []string{"spec/nope", "active", "archive", "default branch"} {
			if !strings.Contains(msg, want) {
				t.Fatalf("Error() = %q, want it to mention %q", msg, want)
			}
		}
	})
}
