package objsupersede

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/objsupersede/scenario"
)

// memTree is a TreeReader over an in-memory repo-path -> content map.
type memTree map[string]string

func (m memTree) Files(_ context.Context, dir string) ([]TreeFile, error) {
	var paths []string
	for p := range m {
		if strings.HasPrefix(p, dir+"/") {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	out := make([]TreeFile, 0, len(paths))
	for _, p := range paths {
		out = append(out, TreeFile{Path: p, Regular: true})
	}
	return out, nil
}

func (m memTree) ReadFile(_ context.Context, p string) ([]byte, error) {
	v, ok := m[p]
	if !ok {
		return nil, fmt.Errorf("memTree: %s: no such file", p)
	}
	return []byte(v), nil
}

// failTree fails every read of one path, and lists like memTree.
type failTree struct {
	memTree
	bad string
}

func (f failTree) ReadFile(ctx context.Context, p string) ([]byte, error) {
	if p == f.bad {
		return nil, errors.New("disk on fire")
	}
	return f.memTree.ReadFile(ctx, p)
}

// listFailTree lists like memTree but fails listing one directory.
type listFailTree struct {
	memTree
	dir string
}

func (f listFailTree) Files(ctx context.Context, dir string) ([]TreeFile, error) {
	if dir == f.dir {
		return nil, errors.New("listing on fire")
	}
	return f.memTree.Files(ctx, dir)
}

// layerTree is the committed fixture's base tree with the named layers
// written over it, as one tree.
func layerTree(t *testing.T, layers ...string) memTree {
	t.Helper()
	m, err := scenario.Load(scenario.Dir())
	if err != nil {
		t.Fatal(err)
	}
	base, err := m.BaseFiles(scenario.Dir())
	if err != nil {
		t.Fatal(err)
	}
	files, err := m.Files(scenario.Dir(), layers...)
	if err != nil {
		t.Fatal(err)
	}
	for p, c := range files {
		base[p] = c
	}
	return memTree(base)
}

func mustRead(t *testing.T, tr TreeReader) *Records {
	t.Helper()
	recs, err := ReadRecords(context.Background(), tr)
	if err != nil {
		t.Fatalf("ReadRecords: %v", err)
	}
	return recs
}

func TestReadRecords_WorkTreeAndCommit(t *testing.T) {
	repo := scenario.Build(t, "proposed")
	ctx := context.Background()
	tests := []struct {
		name      string
		tree      TreeReader
		specs     string
		conflicts string
	}{
		{"design branch working tree", WorkTree{Root: repo.Dir}, "closed-feature*,closed-story*,other-feature,successor", "successor-closed-feature,successor-closed-story"},
		{"main commit", CommitTree{Root: repo.Dir, Commit: "main"}, "closed-feature*,closed-story*,other-feature", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recs, err := ReadRecords(ctx, tc.tree)
			if err != nil {
				t.Fatal(err)
			}
			var specs, conflicts []string
			for name, s := range recs.Specs {
				if s.Archived {
					name += "*"
				}
				specs = append(specs, name)
			}
			sort.Strings(specs)
			for _, c := range recs.Conflicts {
				conflicts = append(conflicts, c.Name)
			}
			if got := strings.Join(specs, ","); got != tc.specs {
				t.Errorf("specs %s, want %s", got, tc.specs)
			}
			if got := strings.Join(conflicts, ","); got != tc.conflicts {
				t.Errorf("conflicts %s, want %s", got, tc.conflicts)
			}
			if len(recs.Failures) != 0 {
				t.Errorf("failures %v", recs.Failures)
			}
		})
	}
	if _, err := ReadRecords(ctx, CommitTree{Root: repo.Dir, Commit: "no-such-ref"}); err == nil {
		t.Error("an unreadable commit read clean")
	}
	if _, err := ReadRecords(ctx, CommitTree{Root: t.TempDir(), Commit: "HEAD"}); err == nil {
		t.Error("a store root outside any repository read clean")
	}
	if recs := mustRead(t, WorkTree{Root: t.TempDir()}); len(recs.Specs)+len(recs.Conflicts)+len(recs.Failures) != 0 {
		t.Errorf("an empty directory read records: %+v", recs)
	}
}

// TestReadRecords_Parity reads one committed tree through both readers,
// with records whose names git quotes in a plain listing (review a I-1):
// the records and failures must be equal, and neither reader may skip one.
func TestReadRecords_Parity(t *testing.T) {
	repo := scenario.Build(t, "proposed")
	for p, from := range map[string]string{
		".verdi/conflicts/successor-closed-feature-2é.md": ".verdi/conflicts/successor-closed-feature.md",
		".verdi/specs/active/café/spec.md":                ".verdi/specs/active/successor/spec.md",
	} {
		raw, err := os.ReadFile(filepath.Join(repo.Dir, from))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(repo.Dir, p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo.Dir, p), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitIn(t, repo.Dir, "add", "-A")
	gitIn(t, repo.Dir, "commit", "-q", "--no-verify", "-m", "Add oddly named records")
	work, commit := mustRead(t, WorkTree{Root: repo.Dir}), mustRead(t, CommitTree{Root: repo.Dir, Commit: "HEAD"})
	if len(work.Failures) != 2 || !reflect.DeepEqual(work, commit) {
		t.Fatalf("readers differ on one tree:\nwork   %v\ncommit %v", work.Failures, commit.Failures)
	}
}

// underSubdir rewrites a built scenario's whole history so its store sits
// in product/, below the git root, a layout gitx.RepoPrefix exists for; the
// rewrite keeps every author, committer, and date. It returns the store
// root.
func underSubdir(t *testing.T, repo *scenario.Repo) string {
	t.Helper()
	cmd := exec.Command("git", "filter-branch", "-f", "--index-filter",
		`GIT_INDEX_FILE="$GIT_INDEX_FILE.new" git read-tree --prefix=product/ "$GIT_COMMIT" && mv "$GIT_INDEX_FILE.new" "$GIT_INDEX_FILE"`,
		"--", "--all")
	cmd.Dir = repo.Dir
	cmd.Env = append(os.Environ(), "FILTER_BRANCH_SQUELCH_WARNING=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git filter-branch: %v\n%s", err, out)
	}
	return filepath.Join(repo.Dir, "product")
}

// TestReadRecords_SubRootParity reads one committed tree whose store sits
// below the git root through both readers (lane L3 re-review a I-A): the
// commit reader must see the same records, never an empty tree.
func TestReadRecords_SubRootParity(t *testing.T) {
	root := underSubdir(t, scenario.Build(t, "accepted"))
	work, commit := mustRead(t, WorkTree{Root: root}), mustRead(t, CommitTree{Root: root, Commit: "HEAD"})
	if len(work.Specs) != 4 || len(work.Conflicts) != 2 || !reflect.DeepEqual(work, commit) {
		t.Fatalf("a store below the git root:\nwork   %d specs, %d conflicts, %v\ncommit %d specs, %d conflicts, %v",
			len(work.Specs), len(work.Conflicts), work.Failures, len(commit.Specs), len(commit.Conflicts), commit.Failures)
	}
}

// TestReadRecords_SymlinkParity reads one committed tree holding a
// symlinked conflict file and a symlinked spec directory through both
// readers (review a M-5): each is a recorded Failure in both, never read
// through and never skipped.
func TestReadRecords_SymlinkParity(t *testing.T) {
	repo := scenario.Build(t, "proposed")
	for link, target := range map[string]string{
		".verdi/conflicts/successor-closed-story.md": "outside/successor-closed-story.md",
		".verdi/specs/active/successor":              "outside/successor",
	} {
		full, dest := filepath.Join(repo.Dir, link), filepath.Join(repo.Dir, target)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(full, dest); err != nil {
			t.Fatal(err)
		}
		rel, err := filepath.Rel(filepath.Dir(full), dest)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(rel, full); err != nil {
			t.Fatal(err)
		}
	}
	gitIn(t, repo.Dir, "add", "-A")
	gitIn(t, repo.Dir, "commit", "-q", "--no-verify", "-m", "Link a conflict and a spec directory")
	want := []string{
		".verdi/conflicts/successor-closed-story.md: " + errNotRegular.Error(),
		".verdi/specs/active/successor: " + errNotRegular.Error(),
	}
	work, commit := mustRead(t, WorkTree{Root: repo.Dir}), mustRead(t, CommitTree{Root: repo.Dir, Commit: "HEAD"})
	if !reflect.DeepEqual(work, commit) || !reflect.DeepEqual(work.Failures, want) || work.Specs["successor"] != nil || len(work.Conflicts) != 1 {
		t.Fatalf("symlinked records:\nwork   %v (conflicts %d, successor read %v)\ncommit %v\nwant   %v",
			work.Failures, len(work.Conflicts), work.Specs["successor"] != nil, commit.Failures, want)
	}
}

// TestReadRecords_LinkLayouts pins which links are Failures (lane L3
// re-review a m-1, m-2): a link on a record path, or on a directory records
// sit in, is a Failure in both readers and is never followed; a link that
// is no record is ignored alike by both.
func TestReadRecords_LinkLayouts(t *testing.T) {
	notRegular := ": " + errNotRegular.Error()
	tests := []struct {
		name, link, target string // link replaces the path link with a symlink to target
		want               []string
		successorRead      bool
	}{
		{"a linked non-record file in a spec directory is ignored", ".verdi/specs/active/successor/notes.md", ".verdi/specs/active/successor/spec.md", nil, true},
		{"a linked non-record file among the conflicts is ignored", ".verdi/conflicts/README", ".verdi/conflicts/successor-closed-feature.md", nil, true},
		{"a linked .verdi", ".verdi", "real/verdi", []string{".verdi" + notRegular}, false},
		{"a linked specs directory", ".verdi/specs", "real/specs", []string{".verdi/specs" + notRegular}, false},
		{"a linked zone directory", ".verdi/specs/active", "real/active", []string{".verdi/specs/active" + notRegular}, false},
		{"a linked conflicts directory", ".verdi/conflicts", "real/conflicts", []string{".verdi/conflicts" + notRegular}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := scenario.Build(t, "proposed")
			link, target := filepath.Join(repo.Dir, tc.link), filepath.Join(repo.Dir, tc.target)
			if tc.want != nil { // a linked directory: its contents move to the target
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(link, target); err != nil {
					t.Fatal(err)
				}
			}
			rel, err := filepath.Rel(filepath.Dir(link), target)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(rel, link); err != nil {
				t.Fatal(err)
			}
			gitIn(t, repo.Dir, "add", "-A")
			gitIn(t, repo.Dir, "commit", "-q", "--no-verify", "-m", "Link "+tc.link)
			work, commit := mustRead(t, WorkTree{Root: repo.Dir}), mustRead(t, CommitTree{Root: repo.Dir, Commit: "HEAD"})
			if !reflect.DeepEqual(work, commit) || !reflect.DeepEqual(work.Failures, tc.want) || tc.successorRead != (work.Specs["successor"] != nil) {
				t.Fatalf("work %v (successor read %v)\ncommit %v\nwant %v", work.Failures, work.Specs["successor"] != nil, commit.Failures, tc.want)
			}
		})
	}
}

func TestReadRecords_Failures(t *testing.T) {
	const bad = "---\nid: [\n---\n"
	tests := []struct {
		name, path, content, want string
	}{
		{"undecodable spec", ".verdi/specs/active/broken/spec.md", bad, ".verdi/specs/active/broken/spec.md:"},
		{"spec id disagrees with its directory", ".verdi/specs/active/elsewhere/spec.md", "", "id spec/successor"},
		{"spec in both zones", ".verdi/specs/archive/successor/spec.md", "", "both zones"},
		{"undecodable conflict", ".verdi/conflicts/broken.md", bad, ".verdi/conflicts/broken.md:"},
		{"conflict id disagrees with its file", ".verdi/conflicts/elsewhere.md", "conflict", "id conflict/successor-closed-feature"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tree := layerTree(t, "successor", "conflict-feature")
			switch tc.content {
			case "":
				tc.content = tree[".verdi/specs/active/successor/spec.md"]
			case "conflict":
				tc.content = tree[".verdi/conflicts/successor-closed-feature.md"]
			}
			tree[tc.path] = tc.content
			recs := mustRead(t, tree)
			if len(recs.Failures) != 1 || !strings.Contains(recs.Failures[0], tc.want) {
				t.Fatalf("failures %v, want one containing %q", recs.Failures, tc.want)
			}
		})
	}
}

// TestReadRecords_OperationalErrors pins that a read or listing error is
// returned as an error, never downgraded to a Failure or ignored (review a
// M-6).
func TestReadRecords_OperationalErrors(t *testing.T) {
	tree := layerTree(t, "successor", "conflict-feature")
	locked := t.TempDir()
	dir := filepath.Join(locked, ".verdi", "specs", "active", "locked")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	tests := []struct {
		name string
		tr   TreeReader
	}{
		{"a spec read error", failTree{tree, ".verdi/specs/active/successor/spec.md"}},
		{"a conflict read error", failTree{tree, ".verdi/conflicts/successor-closed-feature.md"}},
		{"a specs listing error", listFailTree{tree, specsDir}},
		{"a conflicts listing error", listFailTree{tree, conflictsDir}},
		{"a working-tree walk error", WorkTree{Root: locked}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "a working-tree walk error" && os.Geteuid() == 0 {
				t.Skip("root reads a mode-0 directory, so no walk error can be provoked this way")
			}
			if recs, err := ReadRecords(context.Background(), tc.tr); err == nil {
				t.Fatalf("no error; records %+v", recs)
			}
		})
	}
}
