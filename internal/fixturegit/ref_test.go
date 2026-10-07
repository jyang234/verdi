package fixturegit

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// fatalRecorder is a testing.TB whose Fatal and Fatalf record the failure
// and stop the calling goroutine, so a test can prove a helper fails its
// caller without failing itself.
type fatalRecorder struct {
	testing.TB
	failed bool
	msg    string
}

func (r *fatalRecorder) Helper() {}

func (r *fatalRecorder) Fatal(args ...any) {
	r.failed, r.msg = true, fmt.Sprint(args...)
	runtime.Goexit()
}

func (r *fatalRecorder) Fatalf(format string, args ...any) {
	r.failed, r.msg = true, fmt.Sprintf(format, args...)
	runtime.Goexit()
}

// runRecorded runs fn with a fatalRecorder on its own goroutine and
// returns the recorder once fn returns or fails.
func runRecorded(t *testing.T, fn func(tb testing.TB)) *fatalRecorder {
	t.Helper()
	rec := &fatalRecorder{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn(rec)
	}()
	<-done
	return rec
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return strings.TrimSpace(runGitOutput(t, dir, nil, args...))
}

func twoLayers(t *testing.T) *Repo {
	t.Helper()
	return Build(t, []Layer{
		{Files: map[string]string{"a.txt": "a\n"}, Message: "one"},
		{Files: map[string]string{"b.txt": "b\n"}, Message: "two"},
	})
}

// TestCreateRef_CreatesEachKindOfRef proves CreateRef points a new ref of
// any namespace at the commit, and leaves HEAD, the checked-out branch and
// the working tree where they were.
func TestCreateRef_CreatesEachKindOfRef(t *testing.T) {
	refs := []string{"refs/remotes/origin/main", "refs/remotes/origin/design/foo", "refs/heads/design/bar", "refs/tags/v1"}
	for _, ref := range refs {
		t.Run(ref, func(t *testing.T) {
			repo := twoLayers(t)
			CreateRef(t, repo.Dir, ref, repo.Heads[0])
			if got := gitOut(t, repo.Dir, "rev-parse", "--verify", ref); got != repo.Heads[0] {
				t.Fatalf("%s = %s, want %s", ref, got, repo.Heads[0])
			}
			if got := gitOut(t, repo.Dir, "rev-parse", "HEAD"); got != repo.Head {
				t.Fatalf("HEAD = %s after CreateRef, want unchanged %s", got, repo.Head)
			}
			if got := gitOut(t, repo.Dir, "symbolic-ref", "HEAD"); got != "refs/heads/main" {
				t.Fatalf("HEAD names %s after CreateRef, want refs/heads/main", got)
			}
			if got := gitOut(t, repo.Dir, "status", "--porcelain"); got != "" {
				t.Fatalf("status after CreateRef = %q, want clean", got)
			}
		})
	}
}

// TestCreateRef_Negative is CreateRef's failures: it is create-only, so an
// existing ref fails the test and keeps its value, and an unresolvable
// commit fails the test and creates nothing.
func TestCreateRef_Negative(t *testing.T) {
	tests := []struct {
		name   string
		ref    string
		commit func(r *Repo) string
		pre    bool
	}{
		{"an existing ref", "refs/remotes/origin/main", func(r *Repo) string { return r.Heads[0] }, true},
		{"a commit that does not resolve", "refs/remotes/origin/main", func(*Repo) string { return strings.Repeat("1", 40) }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := twoLayers(t)
			if tt.pre {
				CreateRef(t, repo.Dir, tt.ref, repo.Head)
			}
			before := gitOut(t, repo.Dir, "for-each-ref")
			rec := runRecorded(t, func(tb testing.TB) { CreateRef(tb, repo.Dir, tt.ref, tt.commit(repo)) })
			if !rec.failed {
				t.Fatal("CreateRef succeeded, want it to fail its caller")
			}
			if !strings.Contains(rec.msg, "update-ref") {
				t.Errorf("failure %q does not name the git command", rec.msg)
			}
			if after := gitOut(t, repo.Dir, "for-each-ref"); after != before {
				t.Fatalf("refs changed by a failed CreateRef:\nbefore %s\nafter  %s", before, after)
			}
		})
	}
}
