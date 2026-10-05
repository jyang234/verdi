package specstate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/gitx/readcensus"
)

// originRepo is a one-layer repository whose default branch is the
// remote-tracking origin/main, discovered through origin/HEAD, with the
// local branch renamed away so nothing shadows it.
func originRepo(t *testing.T, files map[string]string) *fixturegit.Repo {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "")
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "land"}})
	fabricateRemoteRef(t, repo.Dir, "main", repo.Head)
	setSymbolicRef(t, repo.Dir, "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	renameCurrentBranch(t, repo.Dir, "feature/work")
	return repo
}

// TestWithAcceptedHead_PinsOneResolution (Wave 6 §5.3; ledger SI-356): a
// pinned context answers every ResolveDefaultBranch for its root with the
// one resolution — the unpinned Name and Ref, plus the id Ref resolves to
// and the commit it peels to — and runs no git process to do so, however
// often it is asked or re-pinned. An unpinned context resolves as before,
// with no ids.
func TestWithAcceptedHead_PinsOneResolution(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T) (root string, want Branch, ok bool)
	}{
		{name: "origin/HEAD names a remote-tracking default", setup: func(t *testing.T) (string, Branch, bool) {
			repo := originRepo(t, map[string]string{"a.txt": "a\n"})
			return repo.Dir, Branch{Name: "main", Ref: "origin/main", Tip: repo.Head, Commit: repo.Head}, true
		}},
		{name: "CI_DEFAULT_BRANCH names a local default", setup: func(t *testing.T) (string, Branch, bool) {
			repo := buildBranchRepo(t)
			t.Setenv("CI_DEFAULT_BRANCH", "main")
			return repo.Dir, Branch{Name: "main", Ref: "main", Tip: repo.Head, Commit: repo.Head}, true
		}},
		{name: "the default ref names an annotated tag", setup: func(t *testing.T) (string, Branch, bool) {
			repo := originRepo(t, map[string]string{"a.txt": "a\n"})
			runGitForTest(t, repo.Dir, "-c", "user.name=t", "-c", "user.email=t@t", "tag", "-a", "-m", "v1", "v1", repo.Head)
			tag := strings.TrimSpace(runGitForTest(t, repo.Dir, "rev-parse", "v1"))
			runGitForTest(t, repo.Dir, "update-ref", "refs/remotes/origin/main", tag)
			return repo.Dir, Branch{Name: "main", Ref: "origin/main", Tip: tag, Commit: repo.Head}, true
		}},
		{name: "no default branch resolves", setup: func(t *testing.T) (string, Branch, bool) {
			repo := buildBranchRepo(t)
			t.Setenv("CI_DEFAULT_BRANCH", "")
			return repo.Dir, Branch{}, false
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, want, wantOK := tc.setup(t)
			unpinned, unpinnedOK := ResolveDefaultBranch(context.Background(), root)
			if unpinnedOK != wantOK || unpinned != (Branch{Name: want.Name, Ref: want.Ref}) {
				t.Fatalf("unpinned ResolveDefaultBranch = (%+v, %v), want (%+v, %v) with no ids", unpinned, unpinnedOK, Branch{Name: want.Name, Ref: want.Ref}, wantOK)
			}

			pinned := WithAcceptedHead(context.Background(), root)
			census := &readcensus.Census{}
			counted := gitx.WithObserver(pinned, census)
			again := WithAcceptedHead(counted, root+string(filepath.Separator))
			for _, ctx := range []context.Context{counted, again} {
				got, ok := ResolveDefaultBranch(ctx, root)
				if ok != wantOK || got != want {
					t.Fatalf("pinned ResolveDefaultBranch = (%+v, %v), want (%+v, %v)", got, ok, want, wantOK)
				}
			}
			if events := census.Events(); len(events) != 0 {
				t.Fatalf("answering from the pin ran git: %+v", events)
			}
		})
	}
}

// TestWithAcceptedHead_EachRootItsOwn: pinning a second root keeps the
// first findable; a root never pinned resolves for itself.
func TestWithAcceptedHead_EachRootItsOwn(t *testing.T) {
	a := originRepo(t, map[string]string{"a.txt": "a\n"})
	b := originRepo(t, map[string]string{"b.txt": "b\n"})
	c := originRepo(t, map[string]string{"c.txt": "c\n"})
	ctx := WithAcceptedHead(WithAcceptedHead(context.Background(), a.Dir), b.Dir)
	census := &readcensus.Census{}
	ctx = gitx.WithObserver(ctx, census)
	for root, head := range map[string]string{a.Dir: a.Head, b.Dir: b.Head} {
		if got, ok := ResolveDefaultBranch(ctx, root); !ok || got.Commit != head {
			t.Fatalf("ResolveDefaultBranch(%s) = (%+v, %v), want the pinned commit %s", root, got, ok, head)
		}
	}
	if events := census.Events(); len(events) != 0 {
		t.Fatalf("two pinned roots ran git: %+v", events)
	}
	if got, ok := ResolveDefaultBranch(ctx, c.Dir); !ok || got.Commit != "" || got.Ref != "origin/main" {
		t.Fatalf("an unpinned root = (%+v, %v), want its own resolution with no ids", got, ok)
	}
	if len(census.Events()) == 0 {
		t.Fatal("an unpinned root resolved without running git")
	}
}

// TestBranch_Rev: a read names the pinned commit when there is one.
func TestBranch_Rev(t *testing.T) {
	for _, tc := range []struct {
		b    Branch
		want string
	}{
		{Branch{Name: "main", Ref: "origin/main"}, "origin/main"},
		{Branch{Name: "main", Ref: "origin/main", Tip: "t", Commit: "c"}, "c"},
		{Branch{Name: "main", Ref: "main", Tip: "t"}, "main"},
		{Branch{}, ""},
	} {
		if got := tc.b.Rev(); got != tc.want {
			t.Errorf("%+v.Rev() = %q, want %q", tc.b, got, tc.want)
		}
	}
}

// listingGit is a gitReader answering only the listing reads
// storeTreePaths makes, and counting them.
type listingGit struct {
	stubGit
	prefix    string
	prefixErr error
	entries   []gitx.TreeEntry
	listErr   error
	listed    []string
}

func (g *listingGit) RepoPrefix(context.Context, string) (string, error) {
	return g.prefix, g.prefixErr
}

func (g *listingGit) LsTreeEntries(_ context.Context, _, rev string) ([]gitx.TreeEntry, error) {
	g.listed = append(g.listed, rev)
	return g.entries, g.listErr
}

// TestStoreTreePaths is table-driven over every case the one listing
// answers and every case it leaves to the two plain listings.
func TestStoreTreePaths(t *testing.T) {
	id := strings.Repeat("c", 40)
	leaf := func(p string) gitx.TreeEntry { return gitx.TreeEntry{Mode: "100644", Type: "blob", Path: p} }
	plain := []gitx.TreeEntry{
		leaf(".verdi/conflicts/b.md"), leaf(".verdi/specs/active/x/spec.md"),
		leaf(".verdi/specs/archive/y/spec.md"), leaf(".verdi/specs-old/z/spec.md"),
		leaf("README.md"), {Mode: "160000", Type: "commit", Path: ".verdi/specs/active/sub"},
		leaf(".verdi/specs/with space/spec.md"), leaf("caf\xc3\xa9.md"),
	}
	for _, tc := range []struct {
		name                 string
		rev                  string
		git                  listingGit
		wantSpecs, wantConfl []string
		wantListed, wantErr  bool
		wantListings         int
	}{
		{name: "a full id at the top level", rev: id, git: listingGit{entries: plain},
			wantSpecs:  []string{".verdi/specs/active/x/spec.md", ".verdi/specs/archive/y/spec.md", ".verdi/specs/active/sub", ".verdi/specs/with space/spec.md"},
			wantConfl:  []string{".verdi/conflicts/b.md"},
			wantListed: true, wantListings: 1},
		{name: "an empty store", rev: id, git: listingGit{entries: []gitx.TreeEntry{leaf("README.md")}},
			wantSpecs: []string{}, wantConfl: []string{}, wantListed: true, wantListings: 1},
		{name: "a ref name is never listed", rev: "origin/main", git: listingGit{entries: plain}},
		{name: "an abbreviated id is never listed", rev: id[:12], git: listingGit{entries: plain}},
		{name: "a store below the top level", rev: id, git: listingGit{prefix: "store/", entries: plain}},
		{name: "an unreadable prefix", rev: id, git: listingGit{prefixErr: errors.New("boom"), entries: plain}},
		{name: "a failed listing is the scan's error", rev: id, git: listingGit{listErr: errors.New("boom")}, wantErr: true, wantListings: 1},
		{name: "a quoted spec path", rev: id, git: listingGit{entries: append([]gitx.TreeEntry{leaf(".verdi/specs/active/q\"/spec.md")}, plain...)}, wantListings: 1},
		{name: "a conflict path with a tab", rev: id, git: listingGit{entries: append([]gitx.TreeEntry{leaf(".verdi/conflicts/a\tb.md")}, plain...)}, wantListings: 1},
		{name: "a non-ASCII spec path", rev: id, git: listingGit{entries: append([]gitx.TreeEntry{leaf(".verdi/specs/active/caf\xc3\xa9/spec.md")}, plain...)}, wantListings: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := tc.git
			specs, confl, listed, err := Projector{git: &g}.storeTreePaths(context.Background(), "/root", tc.rev)
			if (err != nil) != tc.wantErr || listed != tc.wantListed {
				t.Fatalf("storeTreePaths = (listed %v, err %v), want (listed %v, err %v)", listed, err, tc.wantListed, tc.wantErr)
			}
			if listed && (!reflect.DeepEqual(specs, tc.wantSpecs) || !reflect.DeepEqual(confl, tc.wantConfl)) {
				t.Fatalf("storeTreePaths = (%q, %q), want (%q, %q)", specs, confl, tc.wantSpecs, tc.wantConfl)
			}
			if len(g.listed) != tc.wantListings {
				t.Fatalf("storeTreePaths listed the tree %d times, want %d", len(g.listed), tc.wantListings)
			}
		})
	}
}

// TestStoreTreePaths_MatchesThePlainListings_Integration: over real git,
// the one whole-tree listing names exactly the paths the two plain
// listings name — and a store holding a path a plain listing quotes is
// left to the plain listings.
func TestStoreTreePaths_MatchesThePlainListings_Integration(t *testing.T) {
	files := map[string]string{
		memoPredPath:                         memoPred,
		".verdi/specs/archive/done/spec.md":  "x",
		".verdi/specs/active/x/notes.md":     "x",
		".verdi/specs/with space/spec.md":    "x",
		".verdi/conflicts/one.md":            "x",
		".verdi/conflicts/nested/two.md":     "x",
		".verdi/specs-old/elsewhere/spec.md": "x",
		".verdi/verdi.yaml":                  "x",
		"README.md":                          "x",
		"caf\xc3\xa9/outside-the-store.md":   "x",
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "store"}})
	ctx := context.Background()
	specs, confl, listed, err := Projector{git: realGitReader{}}.storeTreePaths(ctx, repo.Dir, repo.Head)
	if err != nil || !listed {
		t.Fatalf("storeTreePaths = (listed %v, err %v), want listed", listed, err)
	}
	wantSpecs, _ := gitx.LsTree(ctx, repo.Dir, repo.Head, specZonesPrefix)
	wantConfl, _ := gitx.LsTree(ctx, repo.Dir, repo.Head, conflictsDir())
	for _, l := range [][]string{specs, confl, wantSpecs, wantConfl} {
		sort.Strings(l)
	}
	if !reflect.DeepEqual(specs, wantSpecs) || !reflect.DeepEqual(confl, wantConfl) {
		t.Fatalf("one listing = (%q, %q), the plain listings = (%q, %q)", specs, confl, wantSpecs, wantConfl)
	}

	writeFileForTest(t, repo.Dir, ".verdi/conflicts/caf\xc3\xa9.md", "x")
	runGitForTest(t, repo.Dir, "add", "-A")
	runGitForTest(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "a quoted conflict name")
	head := strings.TrimSpace(runGitForTest(t, repo.Dir, "rev-parse", "HEAD"))
	if _, _, listed, err := (Projector{git: realGitReader{}}).storeTreePaths(ctx, repo.Dir, head); listed || err != nil {
		t.Fatalf("a store with a quoted name: listed %v, err %v, want the plain listings", listed, err)
	}

	sub := filepath.Join(repo.Dir, "nested-store")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, listed, err := (Projector{git: realGitReader{}}).storeTreePaths(ctx, sub, head); listed || err != nil {
		t.Fatalf("a store below the top level: listed %v, err %v, want the plain listings", listed, err)
	}
}

// pinnedResolve resolves candidate in a request that pinned root's
// accepted HEAD inside a read session, counting what the resolve itself
// reads (the pin's own resolution is made before the census attaches).
func pinnedResolve(t *testing.T, root string, candidate Candidate) (Result, readcensus.Budget, error) {
	t.Helper()
	ctx, release := gitx.WithReadSession(context.Background(), root)
	defer release()
	ctx = WithAcceptedHead(ctx, root)
	branch, _ := ResolveDefaultBranch(ctx, root)
	census := &readcensus.Census{}
	result, err := NewProjector().Resolve(gitx.WithObserver(ctx, census), root, candidate)
	full := "refs/heads/" + branch.Ref
	if strings.HasPrefix(branch.Ref, "origin/") {
		full = "refs/remotes/" + branch.Ref
	}
	return result, census.Budget(readcensus.Accepted{Spellings: []string{branch.Ref, full}, IDs: []string{branch.Tip, branch.Commit}}), err
}

// TestProjector_PinnedReadsMatchTheRef_Integration (ledger SI-356): in a
// pinned request every effective state — new, exact (cold and cached
// corpus), diverged, superseded by a landed successor, closed, a story
// superseded through its rung-3 records — is exactly the unpinned
// projector's, and the resolve names the default branch by its id alone:
// no discovery, no explicit resolution and no read of the ref.
func TestProjector_PinnedReadsMatchTheRef_Integration(t *testing.T) {
	succ := string(validSuccessorSpec("new-feature", "old-feature"))
	story := ssStory("ss-story", "")
	for _, tc := range []struct {
		name      string
		files     map[string]string
		candidate Candidate
	}{
		{name: "new", files: map[string]string{"a.txt": "a\n"}, candidate: Candidate{Path: memoPredPath, Content: []byte(memoPred)}},
		{name: "exact", files: map[string]string{memoPredPath: memoPred}, candidate: Candidate{Path: memoPredPath, Content: []byte(memoPred)}},
		{name: "diverged", files: map[string]string{memoPredPath: memoPred}, candidate: Candidate{Path: memoPredPath, Content: []byte(memoPred + "edit\n")}},
		{name: "superseded", files: map[string]string{memoPredPath: memoPred, memoSuccPath: succ}, candidate: Candidate{Path: memoPredPath, Content: []byte(memoPred)}},
		{name: "closed", files: map[string]string{".verdi/specs/archive/old-feature/spec.md": memoPred}, candidate: Candidate{Path: ".verdi/specs/archive/old-feature/spec.md", Content: []byte(memoPred)}},
		{name: "story superseded by its rung-3 records", files: map[string]string{
			ssV1Path: story, ssV2Path: ssStory("ss-story-v2", "", ssSupersedes("ss-story")),
			ssConflictPath: ssConflict("ss-story-wrong", "superseded", "", "spec/ss-story"),
		}, candidate: Candidate{Path: ssV1Path, Content: []byte(story)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := originRepo(t, tc.files)
			want, wantErr := Projector{git: realGitReader{}}.Resolve(context.Background(), repo.Dir, tc.candidate)
			for _, round := range []string{"first", "cached"} {
				got, budget, err := pinnedResolve(t, repo.Dir, tc.candidate)
				if errText(err) != errText(wantErr) || !reflect.DeepEqual(got, want) {
					t.Fatalf("%s pinned Resolve = (%+v, %v), unpinned (%+v, %v)", round, got, err, want, wantErr)
				}
				if chains, explicit, operand := budget.Resolutions(); chains != 0 || explicit != 0 || operand != 0 {
					t.Fatalf("%s pinned Resolve resolved the ref again: chains %d, explicit %d, operand %v", round, chains, explicit, budget.Operand)
				}
			}
		})
	}
}

// TestProjector_PinnedReadFailsAsTheRefDoes_Integration: when a read at
// the pinned commit fails, the resolve reports the ref's own failure,
// word for word — the error text a caller has always seen.
func TestProjector_PinnedReadFailsAsTheRefDoes_Integration(t *testing.T) {
	repo := originRepo(t, map[string]string{memoPredPath: memoPred})
	loseObject(t, repo.Dir, memoPredPath)
	candidate := Candidate{Path: memoPredPath, Content: []byte(memoPred)}
	_, wantErr := Projector{git: realGitReader{}}.Resolve(context.Background(), repo.Dir, candidate)
	if wantErr == nil {
		t.Fatal("the unpinned Resolve succeeded; the fixture did not break the read")
	}
	_, _, err := pinnedResolve(t, repo.Dir, candidate)
	if errText(err) != errText(wantErr) {
		t.Fatalf("pinned Resolve error:\n got: %v\nwant: %v", err, wantErr)
	}
	if strings.Contains(errText(err), repo.Head) {
		t.Fatalf("pinned Resolve error names the commit id: %v", err)
	}
}

func errText(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}

// TestProjector_OnlyAPinnedScanListsTheWholeTree (ledger SI-356): a corpus
// scan in a request that pinned its accepted HEAD enumerates the tree once
// (`ls-tree -rz --full-tree <commit>`); every other scan runs the two
// plain listings it always ran, so the git reads of an unpinned caller —
// lint for the disclosures view, whose cache key covers exactly those
// reads — do not change.
func TestProjector_OnlyAPinnedScanListsTheWholeTree(t *testing.T) {
	listings := func(c *readcensus.Census) []string {
		var out []string
		for _, e := range c.Events() {
			if e.Kind == readcensus.Exec && len(e.Args) > 1 && e.Args[0] == "ls-tree" && strings.HasPrefix(e.Args[1], "-r") {
				out = append(out, strings.Join(e.Args, " "))
			}
		}
		return out
	}
	candidate := Candidate{Path: memoPredPath, Content: []byte(memoPred)}

	unpinnedRepo := originRepo(t, map[string]string{memoPredPath: memoPred})
	unpinned := &readcensus.Census{}
	if _, err := NewProjector().Resolve(gitx.WithObserver(context.Background(), unpinned), unpinnedRepo.Dir, candidate); err != nil {
		t.Fatal(err)
	}
	wantPlain := []string{
		"ls-tree -r --name-only " + unpinnedRepo.Head + " -- " + specZonesPrefix,
		"ls-tree -r --name-only " + unpinnedRepo.Head + " -- " + conflictsDir(),
	}
	if got := listings(unpinned); !reflect.DeepEqual(got, wantPlain) {
		t.Fatalf("an unpinned scan listed %q, want the two plain listings %q", got, wantPlain)
	}

	pinnedRepo := originRepo(t, map[string]string{memoPredPath: memoPred, "other.txt": "x\n"})
	ctx, release := gitx.WithReadSession(context.Background(), pinnedRepo.Dir)
	defer release()
	ctx = WithAcceptedHead(ctx, pinnedRepo.Dir)
	pinned := &readcensus.Census{}
	if _, err := NewProjector().Resolve(gitx.WithObserver(ctx, pinned), pinnedRepo.Dir, candidate); err != nil {
		t.Fatal(err)
	}
	if got, want := listings(pinned), []string{"ls-tree -rz --full-tree " + pinnedRepo.Head}; !reflect.DeepEqual(got, want) {
		t.Fatalf("a pinned scan listed %q, want the one whole-tree listing %q", got, want)
	}
}
