package gitx

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// countingObserver counts every git process a context launches, by
// subcommand (the first argument that is not an option).
type countingObserver struct {
	mu     sync.Mutex
	counts map[string]int
	argv   [][]string
}

func newCountingObserver() *countingObserver { return &countingObserver{counts: map[string]int{}} }

func (c *countingObserver) Observe(_ string, args []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.argv = append(c.argv, append([]string(nil), args...))
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			c.counts[a]++
			return
		}
	}
}

func (c *countingObserver) count(sub string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[sub]
}

func (c *countingObserver) total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.argv)
}

// sessionRepo is a three-commit repository whose tree carries every shape a
// read session must answer exactly as the exec path does: plain text, CRLF
// text, binary bytes with NULs, an empty file, a file with no final
// newline, an executable, a symlink, a nested directory, a large file, a
// path with a space, and a file that the second commit changes.
func sessionRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	large := strings.Repeat("0123456789abcdef", 1<<13) // 128 KiB, beyond any pipe buffer
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{
			"plain.txt":           "plain\n",
			"crlf.txt":            "one\r\ntwo\r\n",
			"binary.bin":          "a\x00b\x00\xff\n",
			"empty.txt":           "",
			"nofinal.txt":         "no final newline",
			"tool.sh":             "#!/bin/sh\necho tool\n",
			"dir/nested/deep.md":  "deep\n",
			"dir/sibling.md":      "sibling\n",
			"large.txt":           large,
			"with space/file.txt": "spaced\n",
			"changes.txt":         "first\n",
		}, Message: "first"},
		{Files: map[string]string{"changes.txt": "second\n"}, Message: "second"},
	})
	if err := os.Chmod(filepath.Join(repo.Dir, "tool.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("plain.txt", filepath.Join(repo.Dir, "link.txt")); err != nil {
		t.Fatal(err)
	}
	gitT(t, repo.Dir, "add", "-A")
	gitT(t, repo.Dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--quiet", "--no-verify", "-m", "mode and link")
	return repo
}

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func errText(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}

// sessionPaths is every path the differential tests read, including the
// shapes the session must hand to the exec path.
func sessionPaths() []string {
	return []string{
		"plain.txt", "crlf.txt", "binary.bin", "empty.txt", "nofinal.txt", "tool.sh",
		"dir/nested/deep.md", "dir/sibling.md", "large.txt", "with space/file.txt", "changes.txt",
		"link.txt",                  // a symlink: ls-tree refuses it by mode
		"dir", "dir/nested", "dir/", // directories, addressed bare and with a slash
		"missing.txt", "dir/missing.md", // absent at an existing parent
		"nope/missing.md",                 // absent parent
		"./plain.txt", "dir/../plain.txt", // relative forms
		"*.txt", "dir/*.md", // pathspec magic
		"", // no path at all
	}
}

// sessionRevs is every revision the differential tests read at.
func sessionRevs(repo *fixturegit.Repo) []string {
	return []string{"HEAD", "HEAD~1", repo.Heads[0], "main", "no-such-ref", strings.Repeat("0", 40)}
}

// TestReadSession_ShowMatchesExec proves Show answers byte for byte, and
// error for error, the same inside a session as through its own process,
// over every revision and path shape, read in an order that interleaves
// different blobs (so a batch that handed one read's bytes to another
// would differ).
func TestReadSession_ShowMatchesExec(t *testing.T) {
	repo := sessionRepo(t)
	ctx, release := WithReadSession(context.Background(), repo.Dir)
	defer release()
	for _, rev := range sessionRevs(repo) {
		for _, p := range sessionPaths() {
			want, wantErr := Show(context.Background(), repo.Dir, rev, p)
			got, gotErr := Show(ctx, repo.Dir, rev, p)
			if !bytes.Equal(got, want) || errText(gotErr) != errText(wantErr) {
				t.Errorf("Show(%s, %q) in a session = (%q, %v), want (%q, %v)", rev, p, got, gotErr, want, wantErr)
			}
		}
	}
}

// TestReadSession_BlobAtMatchesExec is BlobAt's differential proof.
func TestReadSession_BlobAtMatchesExec(t *testing.T) {
	repo := sessionRepo(t)
	ctx, release := WithReadSession(context.Background(), repo.Dir)
	defer release()
	for _, rev := range sessionRevs(repo) {
		for _, p := range sessionPaths() {
			wantOID, wantFound, wantErr := BlobAt(context.Background(), repo.Dir, rev, p)
			gotOID, gotFound, gotErr := BlobAt(ctx, repo.Dir, rev, p)
			if gotOID != wantOID || gotFound != wantFound || errText(gotErr) != errText(wantErr) {
				t.Errorf("BlobAt(%s, %q) in a session = (%q, %v, %v), want (%q, %v, %v)", rev, p, gotOID, gotFound, gotErr, wantOID, wantFound, wantErr)
			}
		}
	}
}

// TestReadSession_SubdirectoryMatchesExec: ls-tree reads paths relative to
// its working directory, so in a subdirectory the session must not answer
// BlobAt from the top-level tree.
func TestReadSession_SubdirectoryMatchesExec(t *testing.T) {
	repo := sessionRepo(t)
	sub := filepath.Join(repo.Dir, "dir")
	ctx, release := WithReadSession(context.Background(), sub)
	defer release()
	for _, p := range []string{"sibling.md", "nested/deep.md", "dir/sibling.md", "plain.txt"} {
		wantOID, wantFound, wantErr := BlobAt(context.Background(), sub, "HEAD", p)
		gotOID, gotFound, gotErr := BlobAt(ctx, sub, "HEAD", p)
		if gotOID != wantOID || gotFound != wantFound || errText(gotErr) != errText(wantErr) {
			t.Errorf("BlobAt(HEAD, %q) from dir/ in a session = (%q, %v, %v), want (%q, %v, %v)", p, gotOID, gotFound, gotErr, wantOID, wantFound, wantErr)
		}
		want, wantErr := Show(context.Background(), sub, "HEAD", p)
		got, gotErr := Show(ctx, sub, "HEAD", p)
		if !bytes.Equal(got, want) || errText(gotErr) != errText(wantErr) {
			t.Errorf("Show(HEAD, %q) from dir/ in a session = (%q, %v), want (%q, %v)", p, got, gotErr, want, wantErr)
		}
	}
}

// TestReadSession_BatchesReadsInOneProcess pins the cost: any number of
// Show and BlobAt reads in one session run one cat-file process (plus one
// prefix probe for BlobAt), never a show or ls-tree per read.
func TestReadSession_BatchesReadsInOneProcess(t *testing.T) {
	repo := sessionRepo(t)
	obs := newCountingObserver()
	ctx, release := WithReadSession(WithObserver(context.Background(), obs), repo.Dir)
	defer release()
	for _, p := range []string{"plain.txt", "crlf.txt", "binary.bin", "dir/nested/deep.md", "large.txt"} {
		if _, err := Show(ctx, repo.Dir, "HEAD", p); err != nil {
			t.Fatal(err)
		}
		if _, _, err := BlobAt(ctx, repo.Dir, "HEAD~1", p); err != nil {
			t.Fatal(err)
		}
	}
	if obs.count("cat-file") != 1 || obs.count("show") != 0 || obs.count("ls-tree") != 0 || obs.total() != 2 {
		t.Fatalf("ten reads launched %v, want one cat-file and one rev-parse --show-prefix", obs.argv)
	}
}

// TestReadSession_RefReadsRunOnce pins "the accepted ref is resolved once
// per load": a repeated ref resolution runs one process per session, and
// replays its answer — including a negative one — exactly.
func TestReadSession_RefReadsRunOnce(t *testing.T) {
	repo := sessionRepo(t)
	gitT(t, repo.Dir, "update-ref", "refs/remotes/origin/main", repo.Head)
	gitT(t, repo.Dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	reads := func(ctx context.Context) string {
		var b strings.Builder
		for range 3 {
			head, err := RevParse(ctx, repo.Dir, "HEAD")
			b.WriteString(head + errText(err) + "|")
			branch, err := DefaultBranch(ctx, repo.Dir)
			b.WriteString(branch + errText(err) + "|")
			has, err := HasRemoteTrackingBranch(ctx, repo.Dir, "origin", "main")
			b.WriteString(boolText(has) + errText(err) + "|")
			has, err = HasRemoteTrackingBranch(ctx, repo.Dir, "origin", "master")
			b.WriteString(boolText(has) + errText(err) + "|")
			local, err := HasLocalBranch(ctx, repo.Dir, "nope")
			b.WriteString(boolText(local) + errText(err) + "|")
			exists, err := CommitExists(ctx, repo.Dir, repo.Heads[0])
			b.WriteString(boolText(exists) + errText(err) + "|")
		}
		return b.String()
	}
	want := reads(context.Background())

	obs := newCountingObserver()
	ctx, release := WithReadSession(WithObserver(context.Background(), obs), repo.Dir)
	defer release()
	if got := reads(ctx); got != want {
		t.Fatalf("ref reads in a session = %q, want %q", got, want)
	}
	if obs.total() != 6 {
		t.Fatalf("three rounds of six ref reads launched %d processes, want 6 (one per distinct read): %v", obs.total(), obs.argv)
	}
}

func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// TestWithReadSession_Scope covers the session's own contract: a nested
// call reuses the open session, a different directory gets none, and
// reads after release still answer, through the exec path.
func TestWithReadSession_Scope(t *testing.T) {
	repo := sessionRepo(t)
	ctx, release := WithReadSession(context.Background(), repo.Dir)
	s := sessionFor(ctx, repo.Dir)
	if s == nil {
		t.Fatal("WithReadSession opened no session")
	}
	nested, nestedRelease := WithReadSession(ctx, repo.Dir+string(filepath.Separator))
	if sessionFor(nested, repo.Dir) != s {
		t.Fatal("a nested WithReadSession for the same directory opened a second session")
	}
	nestedRelease()
	if sessionFor(ctx, t.TempDir()) != nil {
		t.Fatal("a session answered for another directory")
	}
	if _, err := Show(ctx, repo.Dir, "HEAD", "plain.txt"); err != nil {
		t.Fatal(err)
	}
	release()
	release() // idempotent
	obs := newCountingObserver()
	got, err := Show(WithObserver(ctx, obs), repo.Dir, "HEAD", "plain.txt")
	if err != nil || string(got) != "plain\n" {
		t.Fatalf("Show after release = (%q, %v), want the blob", got, err)
	}
	if obs.count("show") != 1 {
		t.Fatalf("a read after release launched %v, want its own show", obs.argv)
	}
}

// TestParseTree_Negative: a malformed tree object is an error, never a
// guess.
func TestParseTree_Negative(t *testing.T) {
	for name, tc := range map[string]struct {
		data   []byte
		oidLen int
	}{
		"bad id length": {data: []byte("100644 a\x00" + strings.Repeat("x", 20)), oidLen: 16},
		"no space":      {data: []byte("100644a\x00" + strings.Repeat("x", 20)), oidLen: 20},
		"no NUL":        {data: []byte("100644 a" + strings.Repeat("x", 20)), oidLen: 20},
		"truncated id":  {data: []byte("100644 a\x00" + strings.Repeat("x", 19)), oidLen: 20},
		"empty name":    {data: []byte("100644 \x00" + strings.Repeat("x", 20)), oidLen: 20},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseTree(tc.data, tc.oidLen); err == nil {
				t.Fatal("parseTree: want an error")
			}
		})
	}
}
