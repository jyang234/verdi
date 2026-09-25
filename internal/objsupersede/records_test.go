package objsupersede

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/objsupersede/scenario"
)

// memTree is a TreeReader over an in-memory repo-path -> content map.
type memTree map[string]string

func (m memTree) Files(_ context.Context, dir string) ([]string, error) {
	var out []string
	for p := range m {
		if strings.HasPrefix(p, dir+"/") {
			out = append(out, p)
		}
	}
	sort.Strings(out)
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
	if recs := mustRead(t, WorkTree{Root: t.TempDir()}); len(recs.Specs)+len(recs.Conflicts)+len(recs.Failures) != 0 {
		t.Errorf("an empty directory read records: %+v", recs)
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
	tree := layerTree(t, "successor")
	if _, err := ReadRecords(context.Background(), failTree{tree, ".verdi/specs/active/successor/spec.md"}); err == nil {
		t.Error("an operational read error was not returned")
	}
}
