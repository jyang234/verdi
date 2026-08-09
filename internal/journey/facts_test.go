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
			// The record stores the CANONICAL repository identity, never
			// the raw origin URL: scheme, userinfo, and the ".git" suffix
			// are gone (gitx.CanonicalRemoteIdentity).
			name:      "known, scp-like spelling",
			remoteURL: func(context.Context, string, string) (string, error) { return "git@example.com:x/y.git", nil },
			wantKnown: true,
			wantValue: "example.com/x/y",
		},
		{
			// Same repository, https spelling: the SAME identity, so two
			// checkouts of one repository never disagree (nor do their
			// record digests).
			name:      "known, https spelling of the same repository",
			remoteURL: func(context.Context, string, string) (string, error) { return "https://Example.com/x/y.git", nil },
			wantKnown: true,
			wantValue: "example.com/x/y",
		},
		{
			// GLG v3's security decision: a journey projection carries no
			// credentials. A credential-bearing origin URL reaches the
			// record only as its credential-free identity.
			name: "credential-bearing url is reduced to an identity",
			remoteURL: func(context.Context, string, string) (string, error) {
				return "https://user:s3cr3t-TOKEN@example.com/x/y.git", nil
			},
			wantKnown: true,
			wantValue: "example.com/x/y",
		},
		{
			// A URL that cannot be canonicalized is unproven and disclosed
			// — and the raw URL itself is never routed into the record or
			// into the disclosure (the same F1(a) posture as a read error).
			name:      "uncanonicalizable url is unknown and disclosed",
			remoteURL: func(context.Context, string, string) (string, error) { return "file:///srv/git/y.git", nil },
			wantKnown: false,
			wantDiscl: "the origin remote URL could not be canonicalized to a repository identity",
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
			// F1(a): the underlying error text ("boom") is never routed
			// into the record — only a fixed, cause-classified disclosure.
			wantDiscl: "remote origin could not be read from this checkout",
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

// TestGatherRepositoryFacts_RemoteOriginNeverCarriesCredentials is the
// negative twin of the table above, stated as the security property
// itself (GLG v3: journey projections contain no credentials): for a
// credential-bearing origin URL — canonicalizable or not — neither the
// fact value nor any disclosure may contain the secret, the userinfo, or
// the raw URL.
func TestGatherRepositoryFacts_RemoteOriginNeverCarriesCredentials(t *testing.T) {
	const secret = "s3cr3t-TOKEN"
	urls := []string{
		"https://user:" + secret + "@example.com/x/y.git",
		"ssh://user:" + secret + "@example.com:2222/x/y.git",
		// Not canonicalizable (unsupported scheme) AND credential-bearing:
		// the unknown path must not leak it into the disclosure either.
		"file://user:" + secret + "@example.com/x/y.git",
	}
	for _, raw := range urls {
		t.Run(raw, func(t *testing.T) {
			git := baseGitReaderForRepoFacts()
			git.remoteURLFn = func(context.Context, string, string) (string, error) { return raw, nil }
			p := newProjector(git, &fakeStateResolver{}, alwaysUnresolvedDefaultBranch)

			rf, discl, err := p.gatherRepositoryFacts(context.Background(), t.TempDir(), "rel/spec.md", []byte("x"), true)
			if err != nil {
				t.Fatalf("gatherRepositoryFacts: %v", err)
			}
			for _, s := range append([]string{rf.RemoteOrigin.Value}, discl...) {
				if strings.Contains(s, secret) || strings.Contains(s, "user@") || strings.Contains(s, raw) {
					t.Fatalf("credential material leaked into the record: %q (origin %q)", s, raw)
				}
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

// TestGatherRepositoryFacts_DefaultBranchRevParseFails is F2's test: the
// default branch NAME resolves, but RevParse of its own ref fails — a
// distinct, disclosed failure (never silently the same "unknown" as no
// default branch resolving at all).
func TestGatherRepositoryFacts_DefaultBranchRevParseFails(t *testing.T) {
	git := baseGitReaderForRepoFacts()
	git.revParseFn = func(_ context.Context, _, rev string) (string, error) {
		if rev == "HEAD" {
			return "headsha", nil
		}
		return "", errors.New("boom")
	}
	resolveDB := func(context.Context, string) (specstate.Branch, bool) {
		return specstate.Branch{Name: "main", Ref: "origin/main"}, true
	}
	p := newProjector(git, &fakeStateResolver{}, resolveDB)

	rf, discl, err := p.gatherRepositoryFacts(context.Background(), t.TempDir(), "rel/spec.md", []byte("x"), true)
	if err != nil {
		t.Fatalf("gatherRepositoryFacts: %v", err)
	}
	if rf.DefaultBranch.Known {
		t.Fatalf("DefaultBranch = %+v, want unknown", rf.DefaultBranch)
	}
	want := "the resolved default branch ref could not be resolved to a commit"
	if !containsString(discl, want) {
		t.Fatalf("disclosures = %v, want to contain %q", discl, want)
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
	// F1(a): fixed, cause-classified disclosures — the underlying "boom"
	// error text never reaches the record.
	if !containsString(discl2, "working-tree dirty state could not be determined from this checkout") ||
		!containsString(discl2, "staged paths could not be determined from this checkout") {
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
	p := newProjector(git, state, alwaysUnresolvedDefaultBranch)

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
	p := newProjector(git, state, alwaysUnresolvedDefaultBranch)

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
