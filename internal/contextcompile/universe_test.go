package contextcompile

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/policyartifact"
)

func testAdapter() policyartifact.Adapter {
	return policyartifact.Adapter{ID: "claude", Version: "1"}
}

func idsBySource(cands []Candidate, source Source) []string {
	var ids []string
	for _, c := range cands {
		if c.Source == source {
			ids = append(ids, c.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

func candidateByID(t *testing.T, cands []Candidate, id string) Candidate {
	t.Helper()
	for _, c := range cands {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("no candidate with id %q among %d candidates: %#v", id, len(cands), cands)
	return Candidate{}
}

// TestBuildUniverse_SourcePrecedence proves a head-tree path that is lifted
// into store-authority or declared-context does not also surface as a
// repository-file candidate, and each lift produces exactly one
// ref-addressed candidate in its own source.
func TestBuildUniverse_SourcePrecedence(t *testing.T) {
	in := UniverseInput{
		Tree: []gitx.TreeEntry{
			{Mode: "100644", Type: "blob", Object: "aaa", Path: "spec/foo/spec.md"},
			{Mode: "100644", Type: "blob", Object: "bbb", Path: "docs/context.md"},
			{Mode: "100644", Type: "blob", Object: "ccc", Path: "ordinary.txt"},
		},
		LiftedStorePaths:   map[string]string{"spec/foo/spec.md": "spec/foo"},
		LiftedContextPaths: map[string]string{"docs/context.md": "context/docs-context"},
		Adapter:            testAdapter(),
	}

	got, err := BuildUniverse(in)
	if err != nil {
		t.Fatalf("BuildUniverse: %v", err)
	}

	if ids := idsBySource(got, SourceHeadTree); !reflect.DeepEqual(ids, []string{"path:ordinary.txt"}) {
		t.Fatalf("head-tree ids = %v, want only the non-lifted path", ids)
	}
	if ids := idsBySource(got, SourceStoreAuthority); !reflect.DeepEqual(ids, []string{"ref:spec/foo"}) {
		t.Fatalf("store-authority ids = %v, want %v", ids, []string{"ref:spec/foo"})
	}
	if ids := idsBySource(got, SourceDeclaredContext); !reflect.DeepEqual(ids, []string{"ref:context/docs-context"}) {
		t.Fatalf("declared-context ids = %v, want %v", ids, []string{"ref:context/docs-context"})
	}

	for _, id := range []string{"path:spec/foo/spec.md", "path:docs/context.md"} {
		for _, c := range got {
			if c.ID == id {
				t.Fatalf("lifted path %q leaked into the universe as %#v", id, c)
			}
		}
	}
}

// TestBuildUniverse_SamePathLiftedIntoBothFailsClosed proves the same path
// cannot enter two of the three (store-authority, declared-context,
// head-tree) sources.
func TestBuildUniverse_SamePathLiftedIntoBothFailsClosed(t *testing.T) {
	in := UniverseInput{
		LiftedStorePaths:   map[string]string{"spec/foo/spec.md": "spec/foo"},
		LiftedContextPaths: map[string]string{"spec/foo/spec.md": "context/other"},
		Adapter:            testAdapter(),
	}
	if _, err := BuildUniverse(in); err == nil {
		t.Fatal("BuildUniverse(same path lifted into both store-authority and declared-context): want error, got nil")
	}
}

// TestBuildUniverse_GeneratedProjectionRetainsBothCandidates proves a
// managed generated output produces one retained head-tree candidate
// (checkout state) and one separate projection candidate (freshly compiled
// authority payload) — the deliberate exception to the source-precedence
// rule (authority design §5).
func TestBuildUniverse_GeneratedProjectionRetainsBothCandidates(t *testing.T) {
	in := UniverseInput{
		Tree: []gitx.TreeEntry{
			{Mode: "100644", Type: "blob", Object: "aaa", Path: "CLAUDE.md"},
		},
		ProjectionPaths: []string{"CLAUDE.md"},
		Adapter:         testAdapter(),
	}

	got, err := BuildUniverse(in)
	if err != nil {
		t.Fatalf("BuildUniverse: %v", err)
	}

	headTree := candidateByID(t, got, "path:CLAUDE.md")
	if headTree.Source != SourceHeadTree || headTree.Object != "aaa" {
		t.Fatalf("head-tree CLAUDE.md candidate = %#v, want SourceHeadTree with object aaa", headTree)
	}

	var projectionCount int
	for _, c := range got {
		if c.Source == SourceProjection && c.ID == "path:CLAUDE.md" {
			projectionCount++
			if c.Object != "" {
				t.Fatalf("projection candidate carries an object id %#v, want none (no content read)", c)
			}
		}
	}
	if projectionCount != 1 {
		t.Fatalf("projection candidates for CLAUDE.md = %d, want exactly 1", projectionCount)
	}
}

// TestBuildUniverse_GitMetadataNeverEnters proves .git/ never enters the
// universe by driving BuildUniverse from real gitx output against a real
// repository, rather than only from hand-built fixtures that could hide a
// latent inclusion.
func TestBuildUniverse_GitMetadataNeverEnters(t *testing.T) {
	ctx := context.Background()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files:   map[string]string{"a.txt": "a\n", "testdata/fixture.json": "{}\n"},
			Message: "seed",
		},
	})

	tree, err := gitx.LsTreeEntries(ctx, repo.Dir, repo.Head)
	if err != nil {
		t.Fatalf("LsTreeEntries: %v", err)
	}
	worktree, err := gitx.WorktreeChangedPaths(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("WorktreeChangedPaths: %v", err)
	}

	got, err := BuildUniverse(UniverseInput{
		Head:          repo.Head,
		Tree:          tree,
		WorktreePaths: worktree,
		Adapter:       testAdapter(),
	})
	if err != nil {
		t.Fatalf("BuildUniverse: %v", err)
	}

	for _, c := range got {
		if strings.Contains(c.ID, ".git/") || strings.Contains(c.Path, ".git/") {
			t.Fatalf("candidate %#v names .git/ metadata; it must never enter the universe", c)
		}
	}
}

// TestBuildUniverse_DataZoneBoundary proves every path under `.verdi/data/`
// — whether reported via head-tree or worktree-overlay — collapses into
// exactly one boundary candidate with no descendant, content, or object.
func TestBuildUniverse_DataZoneBoundary(t *testing.T) {
	in := UniverseInput{
		Tree: []gitx.TreeEntry{
			{Mode: "100644", Type: "blob", Object: "aaa", Path: "ordinary.txt"},
		},
		WorktreePaths: []string{
			".verdi/data/secret-one.json",
			".verdi/data/nested/secret-two.json",
		},
		Adapter: testAdapter(),
	}

	got, err := BuildUniverse(in)
	if err != nil {
		t.Fatalf("BuildUniverse: %v", err)
	}

	var boundaryCount int
	for _, c := range got {
		if strings.HasPrefix(c.Path, ".verdi/data/") {
			t.Fatalf("descendant candidate %#v leaked past the .verdi/data/ boundary", c)
		}
		if c.Path == ".verdi/data" {
			boundaryCount++
			if c.Object != "" || c.Mode != "" || c.Type != "" {
				t.Fatalf("boundary candidate %#v carries object/mode/type, want none", c)
			}
		}
	}
	if boundaryCount != 1 {
		t.Fatalf("data-zone boundary candidates = %d, want exactly 1", boundaryCount)
	}
}

// TestBuildUniverse_OrdinaryTrackedDirectoriesRemainCandidates proves
// testdata/, examples, and experiment-shaped directories are not blanket-
// excluded merely because their names resemble a fixture/service
// convention (authority design §5).
func TestBuildUniverse_OrdinaryTrackedDirectoriesRemainCandidates(t *testing.T) {
	in := UniverseInput{
		Tree: []gitx.TreeEntry{
			{Mode: "100644", Type: "blob", Object: "aaa", Path: "testdata/fixture.json"},
			{Mode: "100644", Type: "blob", Object: "bbb", Path: "examples/demo.go"},
			{Mode: "100644", Type: "blob", Object: "ccc", Path: "experiments/spike.md"},
		},
		Adapter: testAdapter(),
	}

	got, err := BuildUniverse(in)
	if err != nil {
		t.Fatalf("BuildUniverse: %v", err)
	}
	for _, want := range []string{"path:testdata/fixture.json", "path:examples/demo.go", "path:experiments/spike.md"} {
		found := false
		for _, c := range got {
			if c.ID == want && c.Source == SourceHeadTree {
				found = true
			}
		}
		if !found {
			t.Fatalf("BuildUniverse dropped ordinary tracked path %q; no blanket testdata/examples/experiments exclusion is allowed here", want)
		}
	}
}

// TestBuildUniverse_WorktreeOverlayCarriesNoObjectOrDigest proves a
// worktree-overlay candidate never carries object/mode/type — the exact
// no-content-read guarantee authority design §5 requires.
func TestBuildUniverse_WorktreeOverlayCarriesNoObjectOrDigest(t *testing.T) {
	in := UniverseInput{
		WorktreePaths: []string{"dirty.txt"},
		Adapter:       testAdapter(),
	}
	got, err := BuildUniverse(in)
	if err != nil {
		t.Fatalf("BuildUniverse: %v", err)
	}
	c := candidateByID(t, got, "path:dirty.txt")
	if c.Source != SourceWorktreeOverlay {
		t.Fatalf("dirty.txt candidate source = %q, want worktree-overlay", c.Source)
	}
	if c.Object != "" || c.Mode != "" || c.Type != "" {
		t.Fatalf("worktree-overlay candidate %#v carries object/mode/type, want none", c)
	}
}

// TestBuildUniverse_CandidateIdentityForms pins the three exact logical-ID
// spellings (authority design §5): path:<repo-relative-path>,
// ref:<canonical-ref>, and opaque:harness-vendor-base/<id>/<version>.
func TestBuildUniverse_CandidateIdentityForms(t *testing.T) {
	in := UniverseInput{
		Tree: []gitx.TreeEntry{
			{Mode: "100644", Type: "blob", Object: "aaa", Path: "readme.md"},
		},
		LiftedStorePaths: map[string]string{"spec/foo/spec.md": "spec/foo"},
		Adapter:          policyartifact.Adapter{ID: "claude", Version: "2026-08-11"},
	}
	got, err := BuildUniverse(in)
	if err != nil {
		t.Fatalf("BuildUniverse: %v", err)
	}
	if c := candidateByID(t, got, "path:readme.md"); c.Path != "readme.md" {
		t.Fatalf("path candidate = %#v, want Path readme.md", c)
	}
	if c := candidateByID(t, got, "ref:spec/foo"); c.Ref != "spec/foo" {
		t.Fatalf("ref candidate = %#v, want Ref spec/foo", c)
	}
	wantOpaque := "opaque:harness-vendor-base/claude/2026-08-11"
	if c := candidateByID(t, got, wantOpaque); c.Source != SourceOpaque {
		t.Fatalf("opaque candidate id %q = %#v, want SourceOpaque", wantOpaque, c)
	}
}

// TestBuildUniverse_OpaqueCandidateIsAlwaysExactlyOne proves the opaque
// source ledger is always the single fixed adapter-owned candidate,
// regardless of every other input.
func TestBuildUniverse_OpaqueCandidateIsAlwaysExactlyOne(t *testing.T) {
	got, err := BuildUniverse(UniverseInput{Adapter: testAdapter()})
	if err != nil {
		t.Fatalf("BuildUniverse: %v", err)
	}
	if ids := idsBySource(got, SourceOpaque); !reflect.DeepEqual(ids, []string{"opaque:harness-vendor-base/claude/1"}) {
		t.Fatalf("opaque ids = %v, want exactly one fixed candidate", ids)
	}
}

func TestBuildUniverse_Negative(t *testing.T) {
	t.Run("missing adapter identity fails closed", func(t *testing.T) {
		if _, err := BuildUniverse(UniverseInput{}); err == nil {
			t.Fatal("BuildUniverse(no adapter): want error, got nil")
		}
	})

	t.Run("duplicate head-tree path fails closed", func(t *testing.T) {
		in := UniverseInput{
			Tree: []gitx.TreeEntry{
				{Mode: "100644", Type: "blob", Object: "aaa", Path: "dup.txt"},
				{Mode: "100644", Type: "blob", Object: "bbb", Path: "dup.txt"},
			},
			Adapter: testAdapter(),
		}
		if _, err := BuildUniverse(in); err == nil {
			t.Fatal("BuildUniverse(duplicate head-tree path): want error, got nil")
		}
	})

	t.Run("duplicate worktree path fails closed", func(t *testing.T) {
		in := UniverseInput{
			WorktreePaths: []string{"dup.txt", "dup.txt"},
			Adapter:       testAdapter(),
		}
		if _, err := BuildUniverse(in); err == nil {
			t.Fatal("BuildUniverse(duplicate worktree path): want error, got nil")
		}
	})

	t.Run("duplicate projection path fails closed", func(t *testing.T) {
		in := UniverseInput{
			ProjectionPaths: []string{"dup.txt", "dup.txt"},
			Adapter:         testAdapter(),
		}
		if _, err := BuildUniverse(in); err == nil {
			t.Fatal("BuildUniverse(duplicate projection path): want error, got nil")
		}
	})

	noncanonical := []string{
		"/absolute.txt",
		"trailing/slash/",
		"has//double/slash",
		"./dot-segment.txt",
		"../escape.txt",
		"back\\slash.txt",
		"",
	}
	for _, p := range noncanonical {
		t.Run("noncanonical head-tree path fails closed: "+p, func(t *testing.T) {
			in := UniverseInput{
				Tree:    []gitx.TreeEntry{{Mode: "100644", Type: "blob", Object: "aaa", Path: p}},
				Adapter: testAdapter(),
			}
			if _, err := BuildUniverse(in); err == nil {
				t.Fatalf("BuildUniverse(noncanonical path %q): want error, got nil", p)
			}
		})
	}

	t.Run("empty ref in a lift map fails closed", func(t *testing.T) {
		in := UniverseInput{
			LiftedStorePaths: map[string]string{"spec/foo/spec.md": ""},
			Adapter:          testAdapter(),
		}
		if _, err := BuildUniverse(in); err == nil {
			t.Fatal("BuildUniverse(empty lifted ref): want error, got nil")
		}
	})
}
