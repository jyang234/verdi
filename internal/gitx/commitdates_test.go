package gitx_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// countingObserver counts every git invocation gitx issues on a context it
// is attached to (gitx.WithObserver) — the witness that CommitDates reads
// any number of revs in ONE invocation.
type countingObserver struct {
	mu   sync.Mutex
	argv [][]string
}

func (o *countingObserver) Observe(_ string, args []string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.argv = append(o.argv, append([]string(nil), args...))
}

func (o *countingObserver) count() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.argv)
}

// commitDatedFor commits one file change in dir with both the author and
// committer date pinned to date (git's "<unix-seconds> <tz-offset>" form),
// returning the new HEAD — a known date in a known, non-UTC offset, which
// fixturegit's single fixed date cannot express.
func commitDatedFor(t *testing.T, dir, file, content, date string) string {
	t.Helper()
	if err := os.WriteFile(dir+"/"+file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitDatedFor(t, dir, nil, "add", "-A")
	gitDatedFor(t, dir, []string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date}, "commit", "--quiet", "--no-verify", "-m", "dated "+file)
	return strings.TrimSpace(gitDatedFor(t, dir, nil, "rev-parse", "HEAD"))
}

func gitDatedFor(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// commitDatesRepo is one fixturegit repository with three commits at three
// KNOWN, DIFFERENT committer dates in three different offsets, a design
// branch, and an annotated tag whose own tagger date differs from its
// commit's committer date.
type commitDatesRepo struct {
	dir                  string
	seed, jan15, jan31   string // commit SHAs
	seedDate, jan15Date  string // canonical committer dates
	jan31Date            string
	branch, annotatedTag string
}

func buildCommitDatesRepo(t *testing.T) commitDatesRepo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "seed\n"}, Message: "seed"},
	})
	r := commitDatesRepo{
		dir:          repo.Dir,
		seed:         repo.Head,
		seedDate:     "2024-01-01T00:00:00+00:00", // fixturegit's fixed date
		branch:       "design/dated",
		annotatedTag: "v-jan31",
	}
	// 2024-01-15T00:00:00Z, committed in +05:30.
	r.jan15 = commitDatedFor(t, r.dir, "b.txt", "jan15\n", "1705276800 +0530")
	r.jan15Date = "2024-01-15T05:30:00+05:30"
	gitDatedFor(t, r.dir, nil, "branch", r.branch, r.jan15)
	// 2024-02-01T00:00:00Z, committed in -08:00.
	r.jan31 = commitDatedFor(t, r.dir, "c.txt", "jan31\n", "1706745600 -0800")
	r.jan31Date = "2024-01-31T16:00:00-08:00"
	// An annotated tag dated years later than the commit it names: the
	// commit's own committer date must win (the ^{commit} peel), never the
	// tagger date.
	gitDatedFor(t, r.dir, []string{"GIT_COMMITTER_DATE=1893456000 +0000"}, "tag", "-a", r.annotatedTag, "-m", "annotated", r.jan31)
	return r
}

// TestCommitDates is the batch primitive's table: every rev that names a
// commit answers its own committer date in gitx.CommitDate's canonical
// form (its own offset preserved), every rev that does not is simply
// absent, and however many revs are asked, exactly ONE git process runs
// (none at all for an empty ask).
func TestCommitDates(t *testing.T) {
	r := buildCommitDatesRepo(t)

	tests := []struct {
		name            string
		revs            []string
		want            map[string]string
		wantInvocations int
	}{
		{
			name:            "one branch rev: its tip's committer date, offset preserved",
			revs:            []string{r.branch},
			want:            map[string]string{r.branch: r.jan15Date},
			wantInvocations: 1,
		},
		{
			name: "many revs of every shape, one invocation",
			revs: []string{r.seed, r.branch, "HEAD", r.jan31, "HEAD~1"},
			want: map[string]string{
				r.seed:   r.seedDate,
				r.branch: r.jan15Date,
				"HEAD":   r.jan31Date,
				r.jan31:  r.jan31Date,
				"HEAD~1": r.jan15Date,
			},
			wantInvocations: 1,
		},
		{
			name:            "duplicate revs collapse to one key, one invocation",
			revs:            []string{"HEAD", "HEAD", r.jan31},
			want:            map[string]string{"HEAD": r.jan31Date, r.jan31: r.jan31Date},
			wantInvocations: 1,
		},
		{
			name:            "annotated tag peels to its commit's committer date, never the tagger date",
			revs:            []string{r.annotatedTag},
			want:            map[string]string{r.annotatedTag: r.jan31Date},
			wantInvocations: 1,
		},
		{
			name:            "a missing rev is absent while every other rev still answers",
			revs:            []string{"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "HEAD", "refs/heads/no-such-branch"},
			want:            map[string]string{"HEAD": r.jan31Date},
			wantInvocations: 1,
		},
		{
			name:            "a rev naming a blob, not a commit, is absent",
			revs:            []string{"HEAD:a.txt", r.seed},
			want:            map[string]string{r.seed: r.seedDate},
			wantInvocations: 1,
		},
		{
			name:            "a rev carrying a newline is refused, never sent as a second query",
			revs:            []string{"HEAD\n" + r.seed, "", r.branch},
			want:            map[string]string{r.branch: r.jan15Date},
			wantInvocations: 1,
		},
		{
			name:            "nothing askable: an empty answer and no git process at all",
			revs:            []string{"", "a\nb"},
			want:            map[string]string{},
			wantInvocations: 0,
		},
		{
			name:            "nil revs: an empty answer and no git process at all",
			revs:            nil,
			want:            map[string]string{},
			wantInvocations: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs := &countingObserver{}
			ctx := gitx.WithObserver(context.Background(), obs)
			got, err := gitx.CommitDates(ctx, r.dir, tt.revs)
			if err != nil {
				t.Fatalf("CommitDates: %v", err)
			}
			if got == nil {
				t.Fatal("CommitDates returned a nil map, want a usable empty map")
			}
			if len(got) != len(tt.want) {
				t.Fatalf("CommitDates = %v, want %v", got, tt.want)
			}
			for rev, want := range tt.want {
				if got[rev] != want {
					t.Errorf("CommitDates[%q] = %q, want %q", rev, got[rev], want)
				}
			}
			if n := obs.count(); n != tt.wantInvocations {
				t.Errorf("git invocations = %d (%v), want %d", n, obs.argv, tt.wantInvocations)
			}
		})
	}
}

// TestCommitDates_MatchesCommitDate pins the batch read to the single-rev
// read it replaces: for every rev both can answer, the two agree byte for
// byte (same canonical layout, same offset handling).
func TestCommitDates_MatchesCommitDate(t *testing.T) {
	r := buildCommitDatesRepo(t)
	ctx := context.Background()
	revs := []string{r.seed, r.branch, "HEAD", r.annotatedTag}
	got, err := gitx.CommitDates(ctx, r.dir, revs)
	if err != nil {
		t.Fatalf("CommitDates: %v", err)
	}
	for _, rev := range revs {
		single, err := gitx.CommitDate(ctx, r.dir, rev)
		if err != nil {
			t.Fatalf("CommitDate(%s): %v", rev, err)
		}
		if got[rev] != single {
			t.Errorf("CommitDates[%q] = %q, CommitDate = %q — the batch must match the single read", rev, got[rev], single)
		}
	}
}

// TestCommitDates_NotARepository_Errors is the operational negative path:
// the one invocation itself failing is an error, never an empty answer
// that would read as "every rev is missing".
func TestCommitDates_NotARepository_Errors(t *testing.T) {
	notARepo := t.TempDir()
	got, err := gitx.CommitDates(context.Background(), notARepo, []string{"HEAD"})
	if err == nil {
		t.Fatalf("CommitDates outside a repository = %v, nil; want an error", got)
	}
}

// TestCommitDates_CancelledContext_Errors: a cancelled context refuses
// before any git process can answer, as an error.
func TestCommitDates_CancelledContext_Errors(t *testing.T) {
	r := buildCommitDatesRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := gitx.CommitDates(ctx, r.dir, []string{"HEAD"}); err == nil {
		t.Fatal("CommitDates with a cancelled context: want an error, got nil")
	}
}

// fakeBatchGit puts a `git` first on PATH that swallows its stdin and
// answers exactly stream — a `cat-file --batch` answer no real git would
// give, to pin how CommitDates treats a malformed one. The rest of PATH
// stays reachable (the fake's own `cat`).
func fakeBatchGit(t *testing.T, stream string) {
	t.Helper()
	dir := t.TempDir()
	out := dir + "/stream"
	if err := os.WriteFile(out, []byte(stream), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncat >/dev/null\ncat '" + out + "'\n"
	if err := os.WriteFile(dir+"/git", []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// TestCommitDates_MalformedBatchStream_Errors is B1-RR2: whenever the one
// `cat-file --batch` answer stream cannot be read in step with the queries
// — truncated, garbled, short, or carrying bytes after the last answer —
// CommitDates returns an error and NO map, never the dates it managed to
// read before the stream went wrong (which a caller would present as a
// complete answer).
func TestCommitDates_MalformedBatchStream_Errors(t *testing.T) {
	body := "tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904\ncommitter C <c@x> 1704067200 +0000\n\nmsg\n"
	good := fmt.Sprintf("%s commit %d\n%s\n", strings.Repeat("a", 40), len(body), body)
	tests := []struct {
		name   string
		revs   []string
		stream string
	}{
		{name: "truncated header", revs: []string{"HEAD"}, stream: "aaaa commit 5"},
		{name: "truncated body", revs: []string{"HEAD"}, stream: "aaaa commit 500\n" + body},
		{name: "garbage", revs: []string{"HEAD"}, stream: "this is not a batch answer\n"},
		{name: "an answer read, then the stream ends short", revs: []string{"HEAD", "main"}, stream: good},
		{name: "an answer read, then garbage", revs: []string{"HEAD", "main"}, stream: good + "garbage\n"},
		{name: "trailing bytes after the last answer", revs: []string{"HEAD"}, stream: good + "extra\n"},
		{name: "a missing answer, then trailing bytes", revs: []string{"HEAD"}, stream: "HEAD^{commit} missing\nextra"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeBatchGit(t, tt.stream)
			got, err := gitx.CommitDates(context.Background(), t.TempDir(), tt.revs)
			if err == nil {
				t.Fatalf("CommitDates over a malformed stream = %v, nil; want an error", got)
			}
			if got != nil {
				t.Fatalf("CommitDates returned the partial map %v alongside its error, want nil", got)
			}
		})
	}
}

// TestCommitDates_WellFormedFakeStream is the fake's own control: the same
// fake answering a well-formed stream reads cleanly, so the rows above
// fail for their malformation alone.
func TestCommitDates_WellFormedFakeStream(t *testing.T) {
	body := "tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904\ncommitter C <c@x> 1704067200 +0000\n\nmsg\n"
	fakeBatchGit(t, fmt.Sprintf("%s commit %d\n%s\nmain^{commit} missing\n", strings.Repeat("a", 40), len(body), body))
	got, err := gitx.CommitDates(context.Background(), t.TempDir(), []string{"HEAD", "main"})
	if err != nil {
		t.Fatalf("CommitDates over a well-formed stream: %v", err)
	}
	if len(got) != 1 || got["HEAD"] != "2024-01-01T00:00:00+00:00" {
		t.Fatalf("CommitDates = %v, want only HEAD dated 2024-01-01T00:00:00+00:00", got)
	}
}
