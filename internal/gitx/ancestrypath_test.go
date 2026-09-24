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

// headOf returns dir's HEAD commit.
func headOf(t *testing.T, dir string) string {
	t.Helper()
	return strings.TrimSpace(runForOutput(t, dir, "rev-parse", "HEAD"))
}

// commitFile writes content to path in dir and commits it, returning the
// new commit.
func commitFile(t *testing.T, dir, path, content, msg string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runFor(t, dir, "add", "-A")
	runFor(t, dir, "commit", "-q", "--no-verify", "-m", msg)
	return headOf(t, dir)
}

func sortedIDs(ids []string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)
	return out
}

// TestAncestryPathChanges_Linear: only the commits after from that change
// path are listed, newest first; from itself and commits that leave path
// alone are not.
func TestAncestryPathChanges_Linear(t *testing.T) {
	isolateGitConfig(t)
	ctx := context.Background()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"f.md": "v1\n", "other.txt": "x\n"}, Message: "base"},
		{Files: map[string]string{"f.md": "v2\n"}, Message: "change f"},
		{Files: map[string]string{"other.txt": "y\n"}, Message: "change other"},
		{Files: map[string]string{"f.md": "v3\n"}, Message: "change f again"},
	})
	base, a, c := repo.Heads[0], repo.Heads[1], repo.Heads[3]

	tests := []struct {
		name, from, to string
		want           []string
	}{
		{name: "from the base", from: base, to: c, want: []string{c, a}},
		{name: "from a change", from: a, to: c, want: []string{c}},
		{name: "from equals to", from: c, to: c, want: []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := AncestryPathChanges(ctx, repo.Dir, tc.from, tc.to, "f.md")
			if err != nil {
				t.Fatalf("AncestryPathChanges: %v", err)
			}
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("AncestryPathChanges(%s..%s) = %v, want %v", tc.from[:8], tc.to[:8], got, tc.want)
			}
		})
	}
}

// TestAncestryPathChanges_Merges pins the merge shapes signedapproval
// relies on: a withdrawal W on the path from C is always listed, even when
// a merge restores the old content from a stale branch, and a side-branch
// commit that leaves the path alone is not.
func TestAncestryPathChanges_Merges(t *testing.T) {
	isolateGitConfig(t)
	ctx := context.Background()

	t.Run("ours-strategy merge on a stale branch", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{
			{Files: map[string]string{"f.md": "base\n", "other.txt": "x\n"}, Message: "base"},
			{Files: map[string]string{"f.md": "base\nrow\n"}, Message: "C"},
		})
		c := repo.Head
		runFor(t, repo.Dir, "branch", "stale")
		w := commitFile(t, repo.Dir, "f.md", "base\n", "W")
		runFor(t, repo.Dir, "checkout", "-q", "stale")
		runFor(t, repo.Dir, "merge", "-q", "-s", "ours", "--no-edit", "-m", "M", "main")
		m := headOf(t, repo.Dir)

		got, err := AncestryPathChanges(ctx, repo.Dir, c, m, "f.md")
		if err != nil {
			t.Fatalf("AncestryPathChanges: %v", err)
		}
		if want := sortedIDs([]string{m, w}); !reflect.DeepEqual(sortedIDs(got), want) {
			t.Fatalf("AncestryPathChanges = %v, want %v (the merge and the withdrawal)", got, want)
		}
	})

	t.Run("merge that checks the path out from a stale branch", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{
			{Files: map[string]string{"f.md": "base\n", "other.txt": "x\n"}, Message: "base"},
			{Files: map[string]string{"f.md": "base\nrow\n"}, Message: "C"},
		})
		c := repo.Head
		runFor(t, repo.Dir, "branch", "stale")
		w := commitFile(t, repo.Dir, "f.md", "base\n", "W")
		runFor(t, repo.Dir, "checkout", "-q", "stale")
		s := commitFile(t, repo.Dir, "other.txt", "stale\n", "S")
		runFor(t, repo.Dir, "checkout", "-q", "main")
		runFor(t, repo.Dir, "merge", "-q", "--no-commit", "--no-ff", "stale")
		runFor(t, repo.Dir, "checkout", "stale", "--", "f.md")
		runFor(t, repo.Dir, "commit", "-q", "--no-verify", "-m", "M")
		m := headOf(t, repo.Dir)

		got, err := AncestryPathChanges(ctx, repo.Dir, c, m, "f.md")
		if err != nil {
			t.Fatalf("AncestryPathChanges: %v", err)
		}
		if want := sortedIDs([]string{m, w}); !reflect.DeepEqual(sortedIDs(got), want) {
			t.Fatalf("AncestryPathChanges = %v, want %v (the merge and the withdrawal, never %s)", got, want, s[:8])
		}
	})

	t.Run("no-ff merge of the changing branch", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{
			{Files: map[string]string{"f.md": "base\n", "other.txt": "x\n"}, Message: "base"},
		})
		runFor(t, repo.Dir, "checkout", "-q", "-b", "approve")
		c := commitFile(t, repo.Dir, "f.md", "base\nrow\n", "C")
		runFor(t, repo.Dir, "checkout", "-q", "main")
		commitFile(t, repo.Dir, "other.txt", "y\n", "U")
		runFor(t, repo.Dir, "merge", "-q", "--no-ff", "--no-edit", "-m", "M", "approve")
		m := headOf(t, repo.Dir)

		got, err := AncestryPathChanges(ctx, repo.Dir, c, m, "f.md")
		if err != nil {
			t.Fatalf("AncestryPathChanges: %v", err)
		}
		// M carries C's f.md unchanged, so git may omit it; whatever is
		// listed must be M, the only commit on the path.
		for _, id := range got {
			if id != m {
				t.Fatalf("AncestryPathChanges = %v, want at most the merge %s", got, m)
			}
		}
	})
}

// TestAncestryPathChanges_LiteralPath: path is matched byte for byte,
// never as a glob, so a sibling the glob would match is not a change.
func TestAncestryPathChanges_LiteralPath(t *testing.T) {
	isolateGitConfig(t)
	ctx := context.Background()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a*.md": "1\n", "ab.md": "1\n"}, Message: "base"},
		{Files: map[string]string{"ab.md": "2\n"}, Message: "change the sibling"},
		{Files: map[string]string{"a*.md": "2\n"}, Message: "change the literal path"},
	})
	got, err := AncestryPathChanges(ctx, repo.Dir, repo.Heads[0], repo.Head, "a*.md")
	if err != nil {
		t.Fatalf("AncestryPathChanges: %v", err)
	}
	if want := []string{repo.Heads[2]}; !reflect.DeepEqual(got, want) {
		t.Fatalf("AncestryPathChanges = %v, want only %v", got, want)
	}
}

// TestAncestryPathChanges_ObservedArgv pins the exact argv.
func TestAncestryPathChanges_ObservedArgv(t *testing.T) {
	isolateGitConfig(t)
	repo := buildBlameRepo(t)
	obs := &recordingObserver{}
	ctx := WithObserver(context.Background(), obs)
	if _, err := AncestryPathChanges(ctx, repo.Dir, repo.Heads[0], repo.Head, "f.txt"); err != nil {
		t.Fatalf("AncestryPathChanges: %v", err)
	}
	want := [][]string{{repo.Dir,
		"--literal-pathspecs", "rev-list", "--ancestry-path", "--full-history",
		repo.Heads[0] + ".." + repo.Head, "--", "f.txt"}}
	if !reflect.DeepEqual(obs.calls, want) {
		t.Fatalf("observed %v, want %v", obs.calls, want)
	}
}

func TestAncestryPathChanges_Negative(t *testing.T) {
	isolateGitConfig(t)
	ctx := context.Background()
	repo := buildBlameRepo(t)
	base, head := repo.Heads[0], repo.Head

	tests := []struct {
		name, dir, from, to, path, wantSubstr string
	}{
		{name: "empty from", dir: repo.Dir, from: "", to: head, path: "f.txt", wantSubstr: "rev"},
		{name: "empty to", dir: repo.Dir, from: base, to: "", path: "f.txt", wantSubstr: "rev"},
		{name: "option-shaped from", dir: repo.Dir, from: "--all", to: head, path: "f.txt", wantSubstr: "rev"},
		{name: "option-shaped to", dir: repo.Dir, from: base, to: "-n1", path: "f.txt", wantSubstr: "rev"},
		{name: "range-shaped from", dir: repo.Dir, from: base + ".." + head, to: head, path: "f.txt", wantSubstr: "rev"},
		{name: "empty path", dir: repo.Dir, from: base, to: head, path: "", wantSubstr: "path"},
		{name: "unknown revision", dir: repo.Dir, from: strings.Repeat("e", 40), to: head, path: "f.txt", wantSubstr: "AncestryPathChanges"},
		{name: "not a repository", dir: t.TempDir(), from: base, to: head, path: "f.txt", wantSubstr: "AncestryPathChanges"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := AncestryPathChanges(ctx, tc.dir, tc.from, tc.to, tc.path)
			if err == nil {
				t.Fatalf("AncestryPathChanges: want error, got %v", got)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("error %q does not mention %q", err, tc.wantSubstr)
			}
		})
	}
}

func TestParseRevList(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	const sha256 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	good := []struct {
		name, out string
		want      []string
	}{
		{name: "empty", out: "", want: []string{}},
		{name: "one", out: sha + "\n", want: []string{sha}},
		{name: "sha1 and sha256", out: sha + "\n" + sha256 + "\n", want: []string{sha, sha256}},
	}
	for _, tc := range good {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRevList([]byte(tc.out))
			if err != nil {
				t.Fatalf("parseRevList: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseRevList = %v, want %v", got, tc.want)
			}
		})
	}
	bad := []struct{ name, out string }{
		{name: "short id", out: "abc123\n"},
		{name: "uppercase id", out: strings.ToUpper(sha) + "\n"},
		{name: "blank line", out: sha + "\n\n" + sha + "\n"},
		{name: "no trailing newline", out: sha},
		{name: "extra field", out: sha + " " + sha + "\n"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := parseRevList([]byte(tc.out)); err == nil {
				t.Fatalf("parseRevList: want error, got %v", got)
			}
		})
	}
}
