package journey

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
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

	p := newProjector(noOpGitReader(), &fakeStateResolver{}, alwaysUnresolvedDefaultBranch)
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

	p := newProjector(noOpGitReader(), &fakeStateResolver{}, alwaysUnresolvedDefaultBranch)
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
	p := newProjector(git, &fakeStateResolver{}, resolveDB)

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
	p := newProjector(noOpGitReader(), &fakeStateResolver{}, alwaysUnresolvedDefaultBranch)

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
	p := newProjector(git, &fakeStateResolver{}, resolveDB)

	_, _, _, _, err := p.resolveTargetBytes(context.Background(), root, "spec/nope")
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("resolveTargetBytes error = %v, want *NotFoundError", err)
	}
}

func TestResolveTargetBytes_StoryRef_Resolves(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, store.ZoneActive, "loans", testFeatureSpecWithStoryMD)

	p := newProjector(noOpGitReader(), &fakeStateResolver{}, alwaysUnresolvedDefaultBranch)
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
	p := newProjector(noOpGitReader(), &fakeStateResolver{}, alwaysUnresolvedDefaultBranch)

	_, _, _, _, err := p.resolveTargetBytes(context.Background(), root, "jira:NOPE-1")
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("resolveTargetBytes error = %v, want *NotFoundError", err)
	}
}

func TestResolveTargetBytes_NeitherForm(t *testing.T) {
	root := t.TempDir()
	p := newProjector(noOpGitReader(), &fakeStateResolver{}, alwaysUnresolvedDefaultBranch)

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

// --- repository facts ---------------------------------------------------

func TestGatherRepositoryFacts_RemoteOrigin(t *testing.T) {
	tests := []struct {
		name      string
		remoteURL func(context.Context, string, string) (string, error)
		wantKnown bool
		wantValue string
		wantDiscl string
	}{
		{
			name:      "known",
			remoteURL: func(context.Context, string, string) (string, error) { return "git@example.com:x/y.git", nil },
			wantKnown: true,
			wantValue: "git@example.com:x/y.git",
		},
		{
			name: "no such remote",
			remoteURL: func(context.Context, string, string) (string, error) {
				return "", gitx.ErrNoSuchRemote
			},
			wantKnown: false,
			wantDiscl: "no origin remote is configured",
		},
		{
			name: "other error",
			remoteURL: func(context.Context, string, string) (string, error) {
				return "", errors.New("boom")
			},
			wantKnown: false,
			wantDiscl: "remote origin could not be read: boom",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git := baseGitReaderForRepoFacts()
			git.remoteURLFn = tt.remoteURL
			p := newProjector(git, &fakeStateResolver{}, alwaysUnresolvedDefaultBranch)

			rf, discl, err := p.gatherRepositoryFacts(context.Background(), t.TempDir(), "rel/spec.md", []byte("x"), true)
			if err != nil {
				t.Fatalf("gatherRepositoryFacts: %v", err)
			}
			if rf.RemoteOrigin.Known != tt.wantKnown || rf.RemoteOrigin.Value != tt.wantValue {
				t.Fatalf("RemoteOrigin = %+v", rf.RemoteOrigin)
			}
			if tt.wantDiscl != "" && !containsString(discl, tt.wantDiscl) {
				t.Fatalf("disclosures = %v, want to contain %q", discl, tt.wantDiscl)
			}
		})
	}
}

func TestGatherRepositoryFacts_Branch(t *testing.T) {
	tests := []struct {
		name          string
		currentBranch func(context.Context, string) (string, error)
		wantKnown     bool
		wantValue     string
	}{
		{"known", func(context.Context, string) (string, error) { return "main", nil }, true, "main"},
		{"detached HEAD", func(context.Context, string) (string, error) { return "", nil }, false, ""},
		{"error", func(context.Context, string) (string, error) { return "", errors.New("boom") }, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git := baseGitReaderForRepoFacts()
			git.currentBranchFn = tt.currentBranch
			p := newProjector(git, &fakeStateResolver{}, alwaysUnresolvedDefaultBranch)

			rf, _, err := p.gatherRepositoryFacts(context.Background(), t.TempDir(), "rel/spec.md", []byte("x"), true)
			if err != nil {
				t.Fatalf("gatherRepositoryFacts: %v", err)
			}
			if rf.Branch.Known != tt.wantKnown || rf.Branch.Value != tt.wantValue {
				t.Fatalf("Branch = %+v", rf.Branch)
			}
		})
	}
}

func TestGatherRepositoryFacts_DefaultBranchAndRelationship(t *testing.T) {
	tests := []struct {
		name         string
		resolveDB    DefaultBranchResolver
		revParse     func(context.Context, string, string) (string, error)
		isAncestor   func(context.Context, string, string, string) (bool, error)
		wantDBKnown  bool
		wantRelation string
	}{
		{
			name:         "unresolved default branch",
			resolveDB:    alwaysUnresolvedDefaultBranch,
			revParse:     func(_ context.Context, _, rev string) (string, error) { return "headsha", nil },
			wantDBKnown:  false,
			wantRelation: "unknown",
		},
		{
			name: "equal",
			resolveDB: func(context.Context, string) (specstate.Branch, bool) {
				return specstate.Branch{Name: "main", Ref: "origin/main"}, true
			},
			revParse: func(_ context.Context, _, rev string) (string, error) {
				return "samesha", nil
			},
			wantDBKnown:  true,
			wantRelation: "equal",
		},
		{
			name: "ahead",
			resolveDB: func(context.Context, string) (specstate.Branch, bool) {
				return specstate.Branch{Name: "main", Ref: "origin/main"}, true
			},
			revParse: func(_ context.Context, _, rev string) (string, error) {
				if rev == "HEAD" {
					return "headsha", nil
				}
				return "defaultsha", nil
			},
			isAncestor: func(_ context.Context, _, ancestor, ref string) (bool, error) {
				return ancestor == "defaultsha" && ref == "headsha", nil
			},
			wantDBKnown:  true,
			wantRelation: "ahead",
		},
		{
			name: "behind",
			resolveDB: func(context.Context, string) (specstate.Branch, bool) {
				return specstate.Branch{Name: "main", Ref: "origin/main"}, true
			},
			revParse: func(_ context.Context, _, rev string) (string, error) {
				if rev == "HEAD" {
					return "headsha", nil
				}
				return "defaultsha", nil
			},
			isAncestor: func(_ context.Context, _, ancestor, ref string) (bool, error) {
				return ancestor == "headsha" && ref == "defaultsha", nil
			},
			wantDBKnown:  true,
			wantRelation: "behind",
		},
		{
			name: "diverged",
			resolveDB: func(context.Context, string) (specstate.Branch, bool) {
				return specstate.Branch{Name: "main", Ref: "origin/main"}, true
			},
			revParse: func(_ context.Context, _, rev string) (string, error) {
				if rev == "HEAD" {
					return "headsha", nil
				}
				return "defaultsha", nil
			},
			isAncestor:   func(context.Context, string, string, string) (bool, error) { return false, nil },
			wantDBKnown:  true,
			wantRelation: "diverged",
		},
		{
			name: "ancestry error yields unknown relationship",
			resolveDB: func(context.Context, string) (specstate.Branch, bool) {
				return specstate.Branch{Name: "main", Ref: "origin/main"}, true
			},
			revParse: func(_ context.Context, _, rev string) (string, error) {
				if rev == "HEAD" {
					return "headsha", nil
				}
				return "defaultsha", nil
			},
			isAncestor:   func(context.Context, string, string, string) (bool, error) { return false, errors.New("boom") },
			wantDBKnown:  true,
			wantRelation: "unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git := baseGitReaderForRepoFacts()
			git.revParseFn = tt.revParse
			if tt.isAncestor != nil {
				git.isAncestorFn = tt.isAncestor
			}
			p := newProjector(git, &fakeStateResolver{}, tt.resolveDB)

			rf, _, err := p.gatherRepositoryFacts(context.Background(), t.TempDir(), "rel/spec.md", []byte("x"), true)
			if err != nil {
				t.Fatalf("gatherRepositoryFacts: %v", err)
			}
			if rf.DefaultBranch.Known != tt.wantDBKnown {
				t.Fatalf("DefaultBranch.Known = %v, want %v (%+v)", rf.DefaultBranch.Known, tt.wantDBKnown, rf.DefaultBranch)
			}
			if rf.Relationship != tt.wantRelation {
				t.Fatalf("Relationship = %q, want %q", rf.Relationship, tt.wantRelation)
			}
		})
	}
}

func TestGatherRepositoryFacts_DirtyStaged(t *testing.T) {
	git := baseGitReaderForRepoFacts()
	git.statusDirtyFn = func(context.Context, string) (bool, error) { return true, nil }
	git.stagedPathsFn = func(context.Context, string) ([]string, error) { return []string{"a", "b"}, nil }
	p := newProjector(git, &fakeStateResolver{}, alwaysUnresolvedDefaultBranch)

	rf, _, err := p.gatherRepositoryFacts(context.Background(), t.TempDir(), "rel/spec.md", []byte("x"), true)
	if err != nil {
		t.Fatalf("gatherRepositoryFacts: %v", err)
	}
	if !rf.Dirty.Known || !rf.Dirty.Value {
		t.Fatalf("Dirty = %+v", rf.Dirty)
	}
	if !rf.Staged.Known || !rf.Staged.Value {
		t.Fatalf("Staged = %+v", rf.Staged)
	}

	git.statusDirtyFn = func(context.Context, string) (bool, error) { return false, errors.New("boom") }
	git.stagedPathsFn = func(context.Context, string) ([]string, error) { return nil, errors.New("boom") }
	rf2, discl2, err := p.gatherRepositoryFacts(context.Background(), t.TempDir(), "rel/spec.md", []byte("x"), true)
	if err != nil {
		t.Fatalf("gatherRepositoryFacts: %v", err)
	}
	if rf2.Dirty.Known || rf2.Staged.Known {
		t.Fatalf("Dirty/Staged should be unknown on error: %+v %+v", rf2.Dirty, rf2.Staged)
	}
	if !containsString(discl2, "working-tree dirty state could not be determined: boom") ||
		!containsString(discl2, "staged paths could not be determined: boom") {
		t.Fatalf("disclosures = %v, want the dirty and staged failure messages", discl2)
	}
}

func TestGatherRepositoryFacts_Source(t *testing.T) {
	tests := []struct {
		name        string
		foundOnDisk bool
		showFn      func(context.Context, string, string, string) ([]byte, error)
		content     []byte
		want        string
	}{
		{
			name:        "matches HEAD",
			foundOnDisk: true,
			showFn:      func(context.Context, string, string, string) ([]byte, error) { return []byte("same"), nil },
			content:     []byte("same"),
			want:        "head",
		},
		{
			name:        "differs from HEAD",
			foundOnDisk: true,
			showFn:      func(context.Context, string, string, string) ([]byte, error) { return []byte("old"), nil },
			content:     []byte("new"),
			want:        "working-tree",
		},
		{
			name:        "absent at HEAD",
			foundOnDisk: true,
			showFn:      func(context.Context, string, string, string) ([]byte, error) { return nil, errors.New("not found") },
			content:     []byte("new"),
			want:        "working-tree",
		},
		{
			name:        "remote-ref fallback",
			foundOnDisk: false,
			showFn:      func(context.Context, string, string, string) ([]byte, error) { panic("should not be called") },
			content:     []byte("remote"),
			want:        "remote-ref",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git := baseGitReaderForRepoFacts()
			git.showFn = tt.showFn
			p := newProjector(git, &fakeStateResolver{}, alwaysUnresolvedDefaultBranch)

			rf, _, err := p.gatherRepositoryFacts(context.Background(), t.TempDir(), "rel/spec.md", tt.content, tt.foundOnDisk)
			if err != nil {
				t.Fatalf("gatherRepositoryFacts: %v", err)
			}
			if rf.Source != tt.want {
				t.Fatalf("Source = %q, want %q", rf.Source, tt.want)
			}
		})
	}
}

func TestWorktreeFact(t *testing.T) {
	tests := []struct {
		name        string
		root        string
		wantManaged bool
		wantName    string
	}{
		{"unmanaged plain checkout", "/home/user/code/verdi", false, ""},
		{"managed worktree", "/home/user/code/verdi/.verdi/data/worktrees/my-design", true, "my-design"},
		{"managed worktree nested path", "/home/user/code/verdi/.verdi/data/worktrees/my-design/sub", true, "my-design"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := worktreeFact(tt.root)
			if got.Managed != tt.wantManaged || got.Name != tt.wantName {
				t.Fatalf("worktreeFact(%q) = %+v, want managed=%v name=%q", tt.root, got, tt.wantManaged, tt.wantName)
			}
		})
	}
}

// baseGitReaderForRepoFacts returns a fake wired with harmless defaults for
// every method gatherRepositoryFacts calls, so a table-driven test only
// needs to override the one or two methods it actually exercises.
func baseGitReaderForRepoFacts() *fakeGitReader {
	g := noOpGitReader()
	g.remoteURLFn = func(context.Context, string, string) (string, error) { return "", gitx.ErrNoSuchRemote }
	g.currentBranchFn = func(context.Context, string) (string, error) { return "main", nil }
	g.revParseFn = func(context.Context, string, string) (string, error) { return "sha", nil }
	g.statusDirtyFn = func(context.Context, string) (bool, error) { return false, nil }
	g.stagedPathsFn = func(context.Context, string) ([]string, error) { return nil, nil }
	g.showFn = func(context.Context, string, string, string) ([]byte, error) { return nil, errors.New("not found") }
	g.isAncestorFn = func(context.Context, string, string, string) (bool, error) { return false, nil }
	return g
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
			p := newProjector(git, &fakeStateResolver{}, alwaysUnresolvedDefaultBranch)

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
	p := newProjector(git, &fakeStateResolver{}, alwaysUnresolvedDefaultBranch)

	_, _, err := p.resolveActiveBranch(context.Background(), t.TempDir(), "foo")
	if err == nil {
		t.Fatal("resolveActiveBranch: want error")
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
	p := newProjector(git, state, alwaysUnresolvedDefaultBranch)

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
	p := newProjector(noOpGitReader(), state, alwaysUnresolvedDefaultBranch)

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

	git := baseGitReaderForRepoFacts()
	git.hasLocalBranchFn = func(context.Context, string, string) (bool, error) { return false, nil }
	git.hasRemoteTrackingBranchFn = func(context.Context, string, string, string) (bool, error) { return false, nil }
	state := &fakeStateResolver{
		resolveFn: func(context.Context, string, specstate.Candidate) (specstate.Result, error) {
			return specstate.Result{State: specstate.Proposed, Relation: specstate.RelationNew}, nil
		},
	}
	p := newProjector(git, state, alwaysUnresolvedDefaultBranch)

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
	p := newProjector(noOpGitReader(), &fakeStateResolver{}, alwaysUnresolvedDefaultBranch)

	_, err := p.GatherFacts(context.Background(), &store.Config{Root: root}, "spec/nope")
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("GatherFacts error = %v, want *NotFoundError", err)
	}
}

func TestGatherFacts_ComponentClassRefused(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, store.ZoneActive, "shared-lib", testComponentSpecMD)
	p := newProjector(noOpGitReader(), &fakeStateResolver{}, alwaysUnresolvedDefaultBranch)

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
