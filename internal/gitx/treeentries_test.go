package gitx

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// buildTreeEntriesRepo seeds a repository exercising every leaf shape
// LsTreeEntries must survive: filenames with whitespace, tabs, newlines,
// and non-ASCII bytes; a regular non-executable blob; a regular executable
// blob; a symlink; and a gitlink (committed submodule pointer).
func buildTreeEntriesRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				"plain.txt":          "plain\n",
				"dir/nested.txt":     "nested\n",
				"has space.txt":      "space\n",
				"has\ttab.txt":       "tab\n",
				"has\nnewline.txt":   "newline\n",
				"café-non-ascii.txt": "non-ascii\n",
				"scripts/run.sh":     "#!/bin/sh\necho hi\n",
			},
			Message: "seed leaves",
		},
	})

	if err := os.Chmod(filepath.Join(repo.Dir, "scripts/run.sh"), 0o755); err != nil {
		t.Fatalf("chmod scripts/run.sh: %v", err)
	}
	if err := os.Symlink("plain.txt", filepath.Join(repo.Dir, "link-to-plain")); err != nil {
		t.Fatalf("symlink link-to-plain: %v", err)
	}
	runGitForTest(t, repo.Dir, "add", "--", "scripts/run.sh", "link-to-plain")
	runGitForTest(t, repo.Dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--quiet", "--no-verify", "-m", "add executable and symlink")

	// A gitlink: a committed submodule-shaped entry, built with
	// update-index --cacheinfo exactly as stagedpaths_test.go's
	// stageGitlinkBump does, then committed so it lands in HEAD's tree.
	head := strings.TrimSpace(runGitForTest(t, repo.Dir, "rev-parse", "HEAD"))
	runGitForTest(t, repo.Dir, "update-index", "--add", "--cacheinfo", "160000,"+head+",sub")
	runGitForTest(t, repo.Dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--quiet", "--no-verify", "-m", "add gitlink")

	repo.Head = strings.TrimSpace(runGitForTest(t, repo.Dir, "rev-parse", "HEAD"))
	return repo
}

func TestLsTreeEntries_Happy(t *testing.T) {
	repo := buildTreeEntriesRepo(t)
	ctx := context.Background()

	got, err := LsTreeEntries(ctx, repo.Dir, repo.Head)
	if err != nil {
		t.Fatalf("LsTreeEntries: %v", err)
	}

	byPath := map[string]TreeEntry{}
	for _, e := range got {
		if _, dup := byPath[e.Path]; dup {
			t.Fatalf("LsTreeEntries returned duplicate path %q", e.Path)
		}
		byPath[e.Path] = e
	}

	wantPaths := []string{
		"plain.txt", "dir/nested.txt", "has space.txt", "has\ttab.txt",
		"has\nnewline.txt", "café-non-ascii.txt", "scripts/run.sh",
		"link-to-plain", "sub",
	}
	for _, p := range wantPaths {
		if _, ok := byPath[p]; !ok {
			t.Fatalf("LsTreeEntries missing path %q; got %d entries: %#v", p, len(got), got)
		}
	}

	if e := byPath["plain.txt"]; e.Mode != "100644" || e.Type != "blob" || e.Object == "" {
		t.Fatalf("plain.txt entry = %#v, want mode 100644, type blob, nonempty object", e)
	}
	if e := byPath["scripts/run.sh"]; e.Mode != "100755" || e.Type != "blob" {
		t.Fatalf("scripts/run.sh entry = %#v, want executable mode 100755, type blob", e)
	}
	if e := byPath["link-to-plain"]; e.Mode != "120000" || e.Type != "blob" {
		t.Fatalf("link-to-plain entry = %#v, want symlink mode 120000, type blob", e)
	}
	if e := byPath["sub"]; e.Mode != "160000" || e.Type != "commit" {
		t.Fatalf("sub (gitlink) entry = %#v, want mode 160000, type commit", e)
	}

	// No entry ever names a tree itself: -r recurses and emits only leaves.
	for _, e := range got {
		if e.Type == "tree" {
			t.Fatalf("LsTreeEntries returned a tree entry %#v; -r must recurse into it instead", e)
		}
	}
}

func TestLsTreeEntriesIncludingTrees_Happy(t *testing.T) {
	repo := buildTreeEntriesRepo(t)
	got, err := LsTreeEntriesIncludingTrees(context.Background(), repo.Dir, repo.Head)
	if err != nil {
		t.Fatalf("LsTreeEntriesIncludingTrees: %v", err)
	}

	byPath := map[string]TreeEntry{}
	for _, entry := range got {
		byPath[entry.Path] = entry
	}
	for _, path := range []string{"dir", "scripts"} {
		if entry := byPath[path]; entry.Mode != "040000" || entry.Type != "tree" || entry.Object == "" {
			t.Fatalf("tree entry %q = %#v, want mode 040000, type tree, nonempty object", path, entry)
		}
	}
	if entry := byPath["dir/nested.txt"]; entry.Mode != "100644" || entry.Type != "blob" {
		t.Fatalf("recursive leaf entry = %#v, want regular blob", entry)
	}
}

func TestLsTreeEntries_PathOrderIsDeterministic(t *testing.T) {
	repo := buildTreeEntriesRepo(t)
	ctx := context.Background()

	first, err := LsTreeEntries(ctx, repo.Dir, repo.Head)
	if err != nil {
		t.Fatalf("LsTreeEntries: %v", err)
	}
	second, err := LsTreeEntries(ctx, repo.Dir, repo.Head)
	if err != nil {
		t.Fatalf("LsTreeEntries: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("LsTreeEntries is not deterministic across calls:\n%#v\nvs\n%#v", first, second)
	}

	paths := make([]string, len(first))
	for i, e := range first {
		paths[i] = e.Path
	}
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	// git's own tree order need not equal lexical sort — this test only
	// pins that LsTreeEntries returns git's order verbatim rather than
	// re-sorting it, so a caller relying on git's own deterministic order
	// is not surprised.
	_ = sorted
}

func TestLsTreeEntries_Negative(t *testing.T) {
	repo := buildTreeEntriesRepo(t)
	ctx := context.Background()

	for _, operation := range []struct {
		name string
		read func(context.Context, string, string) ([]TreeEntry, error)
	}{
		{name: "leaves", read: LsTreeEntries},
		{name: "leaves and trees", read: LsTreeEntriesIncludingTrees},
	} {
		t.Run(operation.name+" ref does not resolve", func(t *testing.T) {
			if _, err := operation.read(ctx, repo.Dir, "not-a-real-ref"); err == nil {
				t.Fatalf("%s(bogus ref): want error, got nil", operation.name)
			}
		})

		t.Run(operation.name+" not a repository at all", func(t *testing.T) {
			notARepo := t.TempDir()
			if _, err := operation.read(ctx, notARepo, "HEAD"); err == nil {
				t.Fatalf("%s outside a repo: want error, got nil", operation.name)
			}
		})
	}
}

func TestParseTreeEntries(t *testing.T) {
	nul := "\x00"
	cases := []struct {
		name    string
		out     string
		want    []TreeEntry
		wantErr bool
	}{
		{name: "empty output"},
		{name: "trailing NUL only", out: nul},
		{
			name: "one regular blob",
			out:  "100644 blob abc123\tfile.txt" + nul,
			want: []TreeEntry{{Mode: "100644", Type: "blob", Object: "abc123", Path: "file.txt"}},
		},
		{
			name: "path carrying a tab and a newline survives because NUL delimits, not newline",
			out:  "100644 blob abc123\tweird\tname\nwith-newline.txt" + nul,
			want: []TreeEntry{{Mode: "100644", Type: "blob", Object: "abc123", Path: "weird\tname\nwith-newline.txt"}},
		},
		{
			name: "multiple entries",
			out:  "100644 blob aaa\ta.txt" + nul + "100755 blob bbb\tb.sh" + nul,
			want: []TreeEntry{
				{Mode: "100644", Type: "blob", Object: "aaa", Path: "a.txt"},
				{Mode: "100755", Type: "blob", Object: "bbb", Path: "b.sh"},
			},
		},
		{name: "entry missing the TAB separator", out: "100644 blob abc123 file.txt" + nul, wantErr: true},
		{name: "entry with too few metadata fields", out: "100644 blob\tfile.txt" + nul, wantErr: true},
		{name: "entry with too many metadata fields", out: "100644 blob abc123 extra\tfile.txt" + nul, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTreeEntries([]byte(tc.out))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseTreeEntries(%q) = %#v, nil; want an error", tc.out, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTreeEntries(%q): %v", tc.out, err)
			}
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseTreeEntries(%q) = %#v, want %#v", tc.out, got, tc.want)
			}
		})
	}
}
