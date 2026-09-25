package gitx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// firstParentTopology is a main line whose spec arrives through a --no-ff
// merge of a design branch, is later edited in place on main, and then
// moves zones: the design-branch commit that added the file is reachable
// from main only through the merge's second parent.
type firstParentTopology struct {
	dir                            string
	root, design, merge, edit, mov string
}

func buildFirstParentTopology(t *testing.T) firstParentTopology {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"base.txt": "base\n"}, Message: "root"},
	})
	dir := repo.Dir
	runFor(t, dir, "checkout", "-q", "-b", "design/s")
	if err := os.MkdirAll(filepath.Join(dir, "active", "s"), 0o755); err != nil {
		t.Fatal(err)
	}
	design := commitFile(t, dir, "active/s/spec.md", "v1\n", "add s")
	runFor(t, dir, "checkout", "-q", "main")
	commitFile(t, dir, "main.txt", "main\n", "advance main")
	runFor(t, dir, "merge", "-q", "--no-ff", "--no-edit", "design/s")
	merge := headOf(t, dir)
	edit := commitFile(t, dir, "active/s/spec.md", "v2\n", "edit s in place")
	if err := os.MkdirAll(filepath.Join(dir, "archive"), 0o755); err != nil {
		t.Fatal(err)
	}
	runFor(t, dir, "mv", "active/s", "archive/s")
	runFor(t, dir, "commit", "-q", "--no-verify", "-m", "archive s")
	return firstParentTopology{dir: dir, root: repo.Head, design: design, merge: merge, edit: edit, mov: headOf(t, dir)}
}

func TestFirstParentPathCommits_Happy(t *testing.T) {
	isolateGitConfig(t)
	top := buildFirstParentTopology(t)
	tests := []struct {
		name  string
		paths []string
		want  []string
	}{
		{name: "merge commit, not the design commit, lands the path", paths: []string{"active/s/spec.md"}, want: []string{top.merge, top.edit, top.mov}},
		{name: "either of two paths, oldest first", paths: []string{"active/s/spec.md", "archive/s/spec.md"}, want: []string{top.merge, top.edit, top.mov}},
		{name: "second path alone", paths: []string{"archive/s/spec.md"}, want: []string{top.mov}},
		{name: "never touched", paths: []string{"nowhere/spec.md"}, want: []string{}},
		{name: "pathspec magic is literal", paths: []string{"*/s/spec.md"}, want: []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FirstParentPathCommits(context.Background(), top.dir, "main", tc.paths...)
			if err != nil {
				t.Fatalf("FirstParentPathCommits: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v (design commit %s must never appear)", got, tc.want, top.design)
			}
		})
	}
}

func TestFirstParentPathCommits_Negative(t *testing.T) {
	isolateGitConfig(t)
	top := buildFirstParentTopology(t)
	shallow := fixturegit.ShallowClone(t, &fixturegit.Repo{Dir: top.dir, Head: top.mov}, 2)
	tests := []struct {
		name     string
		dir, ref string
		paths    []string
		wantErr  error
	}{
		{name: "empty ref", dir: top.dir, ref: "", paths: []string{"a"}},
		{name: "option-shaped ref", dir: top.dir, ref: "--all", paths: []string{"a"}},
		{name: "range-shaped ref", dir: top.dir, ref: "main..main", paths: []string{"a"}},
		{name: "no paths", dir: top.dir, ref: "main"},
		{name: "empty path", dir: top.dir, ref: "main", paths: []string{""}},
		{name: "unknown ref", dir: top.dir, ref: "no-such-branch", paths: []string{"a"}},
		{name: "not a repository", dir: t.TempDir(), ref: "main", paths: []string{"a"}},
		{name: "shallow clone", dir: shallow, ref: "HEAD", paths: []string{"archive/s/spec.md"}, wantErr: ErrShallowHistory},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FirstParentPathCommits(context.Background(), tc.dir, tc.ref, tc.paths...)
			if err == nil {
				t.Fatalf("got %v, want an error", got)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("error %v, want errors.Is %v", err, tc.wantErr)
			}
		})
	}
}
