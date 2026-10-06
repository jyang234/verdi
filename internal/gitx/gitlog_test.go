package gitx

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// gitLogRepo is a one-commit repository for the VERDI_GITLOG tests, built
// before the variable is set: fixturegit runs plain git, never gitx, but
// building first keeps the log to the calls under test either way.
func gitLogRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "a\n"}, Message: "a"}})
}

// unsetGitLog removes VERDI_GITLOG from the environment for the rest of t,
// restoring any ambient value afterwards.
func unsetGitLog(t *testing.T) {
	t.Helper()
	t.Setenv(GitLogEnv, "")
	if err := os.Unsetenv(GitLogEnv); err != nil {
		t.Fatal(err)
	}
}

// readGitLogLines returns the log's lines, each with its newline, failing t
// on a file that does not end in one.
func readGitLogLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the git log: %v", err)
	}
	if len(data) == 0 {
		return nil
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		t.Fatalf("the git log does not end in a newline: %q", data)
	}
	lines := strings.SplitAfter(string(data), "\n")
	return lines[:len(lines)-1]
}

// decodeGitLogLine decodes one log line strictly: unknown fields and
// trailing data are refused.
func decodeGitLogLine(line string) (GitLogRecord, error) {
	dec := json.NewDecoder(strings.NewReader(line))
	dec.DisallowUnknownFields()
	var rec GitLogRecord
	if err := dec.Decode(&rec); err != nil {
		return GitLogRecord{}, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return GitLogRecord{}, fmt.Errorf("trailing data after the record: %q", line)
	}
	return rec, nil
}

// readGitLog decodes every line of the log at path.
func readGitLog(t *testing.T, path string) []GitLogRecord {
	t.Helper()
	var out []GitLogRecord
	for _, line := range readGitLogLines(t, path) {
		rec, err := decodeGitLogLine(line)
		if err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		out = append(out, rec)
	}
	return out
}

// execSite is one of gitx's three exec sites, driven through an exported
// function that reaches it, with the argv it issues.
type execSite struct {
	name string
	call func(ctx context.Context, dir string) error
	argv []string
}

func gitLogExecSites() []execSite {
	return []execSite{
		{"run (RevParse)", func(ctx context.Context, dir string) error {
			_, err := RevParse(ctx, dir, "HEAD")
			return err
		}, []string{"rev-parse", "--verify", "HEAD"}},
		{"ConfigValue", func(ctx context.Context, dir string) error {
			_, err := ConfigValue(ctx, dir, "core.bare")
			return err
		}, []string{"config", "--local", "--get-all", "core.bare"}},
		{"runStdin (WriteBlob)", func(ctx context.Context, dir string) error {
			_, err := WriteBlob(ctx, dir, []byte("logged\n"))
			return err
		}, []string{"hash-object", "-w", "--stdin"}},
	}
}

// TestGitLog_UnsetRecordsNothing: with VERDI_GITLOG unset, or set to the
// empty string, every exec site runs git as before, recording nothing,
// and the context's observer still sees each call.
func TestGitLog_UnsetRecordsNothing(t *testing.T) {
	repo := gitLogRepo(t)
	for _, tt := range []struct {
		name  string
		setup func(t *testing.T)
	}{
		{"unset", unsetGitLog},
		{"set to the empty string", func(t *testing.T) { t.Setenv(GitLogEnv, "") }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			cwd := t.TempDir()
			t.Chdir(cwd)
			obs := &recordingObserver{}
			ctx := WithObserver(context.Background(), obs)
			var want [][]string
			for _, s := range gitLogExecSites() {
				if err := s.call(ctx, repo.Dir); err != nil {
					t.Fatalf("%s: %v", s.name, err)
				}
				want = append(want, append([]string{repo.Dir}, s.argv...))
			}
			if !reflect.DeepEqual(obs.calls, want) {
				t.Fatalf("observed %v, want %v", obs.calls, want)
			}
			if entries, err := os.ReadDir(cwd); err != nil || len(entries) != 0 {
				t.Fatalf("the working directory holds %v (err %v), want nothing written", entries, err)
			}
		})
	}
}

// TestGitLog_EveryExecSiteRecords: each of gitx's three exec sites appends
// one record, in execution order, naming the argv after "git", the
// absolute directory, and this process's pid; and the observer still sees
// the same calls.
func TestGitLog_EveryExecSiteRecords(t *testing.T) {
	repo := gitLogRepo(t)
	logPath := filepath.Join(t.TempDir(), "git.jsonl")
	t.Setenv(GitLogEnv, logPath)
	obs := &recordingObserver{}
	ctx := WithObserver(context.Background(), obs)

	var want []GitLogRecord
	var wantObserved [][]string
	for _, s := range gitLogExecSites() {
		if err := s.call(ctx, repo.Dir); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		want = append(want, GitLogRecord{Args: s.argv, Dir: repo.Dir, PID: os.Getpid()})
		wantObserved = append(wantObserved, append([]string{repo.Dir}, s.argv...))
	}
	if got := readGitLog(t, logPath); !reflect.DeepEqual(got, want) {
		t.Fatalf("logged %+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(obs.calls, wantObserved) {
		t.Fatalf("observed %v, want %v", obs.calls, wantObserved)
	}
}

// TestGitLog_CanonicalBytes: a record is one canonical JSON line (keys
// sorted, no HTML escaping, a newline inside an argument escaped), and a
// call git then refuses is still recorded, since the record precedes git.
func TestGitLog_CanonicalBytes(t *testing.T) {
	repo := gitLogRepo(t)
	logPath := filepath.Join(t.TempDir(), "git.jsonl")
	t.Setenv(GitLogEnv, logPath)
	const rev = "a<b>&c\nd"
	if _, err := RevParse(context.Background(), repo.Dir, rev); err == nil {
		t.Fatalf("RevParse(%q) succeeded, want git's refusal", rev)
	}
	dir, err := json.Marshal(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"args":["rev-parse","--verify","a<b>&c\nd"],"dir":` + string(dir) + `,"pid":` + strconv.Itoa(os.Getpid()) + "}\n"
	if got := readGitLogLines(t, logPath); len(got) != 1 || got[0] != want {
		t.Fatalf("log lines = %q, want exactly %q", got, want)
	}
}

// TestGitLog_DirIsAbsoluteAndClean: a relative or unclean directory is
// recorded as the absolute, cleaned directory git ran in, and "" as the
// process's working directory.
func TestGitLog_DirIsAbsoluteAndClean(t *testing.T) {
	repo := gitLogRepo(t)
	if err := os.MkdirAll(filepath.Join(repo.Dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo.Dir)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, dir, want string
	}{
		{"the working directory", ".", wd},
		{"the empty directory", "", wd},
		{"a relative subdirectory", "sub", filepath.Join(wd, "sub")},
		{"an unclean relative path", "./sub/../sub/", filepath.Join(wd, "sub")},
		{"an unclean absolute path", repo.Dir + "/sub/./../sub/", filepath.Join(repo.Dir, "sub")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "git.jsonl")
			t.Setenv(GitLogEnv, logPath)
			if _, err := RevParse(context.Background(), tt.dir, "HEAD"); err != nil {
				t.Fatal(err)
			}
			got := readGitLog(t, logPath)
			if len(got) != 1 || got[0].Dir != tt.want || !filepath.IsAbs(got[0].Dir) {
				t.Fatalf("logged %+v, want one record in %q", got, tt.want)
			}
		})
	}
}

// TestGitLog_SequentialOrder: sequential calls are logged in the order they
// ran, every one under this process's pid.
func TestGitLog_SequentialOrder(t *testing.T) {
	repo := gitLogRepo(t)
	logPath := filepath.Join(t.TempDir(), "git.jsonl")
	t.Setenv(GitLogEnv, logPath)
	ctx := context.Background()
	var want []GitLogRecord
	for i := range 6 {
		key := fmt.Sprintf("verdi.k%d", i)
		if _, err := ConfigValue(ctx, repo.Dir, key); !errors.Is(err, ErrConfigUnset) {
			t.Fatalf("ConfigValue(%s) = %v, want ErrConfigUnset", key, err)
		}
		want = append(want, GitLogRecord{Args: []string{"config", "--local", "--get-all", key}, Dir: repo.Dir, PID: os.Getpid()})
	}
	if got := readGitLog(t, logPath); !reflect.DeepEqual(got, want) {
		t.Fatalf("logged %+v, want %+v", got, want)
	}
}

// TestGitLog_ConcurrentCallsNeverTear: concurrent execs in one process
// each append one whole line, never interleaved with another's, at volume
// through appendGitLog itself and through every exec site; each
// goroutine's own records keep its order.
func TestGitLog_ConcurrentCallsNeverTear(t *testing.T) {
	repo := gitLogRepo(t)
	for _, tt := range []struct {
		name               string
		goroutines, rounds int
		call               func(ctx context.Context, g, i int) error
	}{
		{"large records through the sink", 32, 40, func(_ context.Context, g, i int) error {
			return appendGitLog(repo.Dir, []string{"marker", strconv.Itoa(g), strconv.Itoa(i), strings.Repeat("x", 4096+g*97)})
		}},
		{"every exec site", 8, 3, func(ctx context.Context, g, i int) error {
			s := gitLogExecSites()[(g+i)%3]
			if err := s.call(ctx, repo.Dir); err != nil {
				return err
			}
			return appendGitLog(repo.Dir, []string{"marker", strconv.Itoa(g), strconv.Itoa(i)})
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "git.jsonl")
			t.Setenv(GitLogEnv, logPath)
			ctx := context.Background()
			var wg sync.WaitGroup
			errs := make(chan error, tt.goroutines*tt.rounds)
			for g := range tt.goroutines {
				wg.Go(func() {
					for i := range tt.rounds {
						if err := tt.call(ctx, g, i); err != nil {
							errs <- err
						}
					}
				})
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				t.Fatal(err)
			}
			next := make([]int, tt.goroutines)
			markers := 0
			for _, rec := range readGitLog(t, logPath) {
				if rec.PID != os.Getpid() || rec.Dir != repo.Dir {
					t.Fatalf("record %+v: want pid %d in %s", rec, os.Getpid(), repo.Dir)
				}
				if rec.Args[0] != "marker" {
					continue
				}
				g, _ := strconv.Atoi(rec.Args[1])
				i, _ := strconv.Atoi(rec.Args[2])
				if i != next[g] {
					t.Fatalf("goroutine %d's record %d came after %d of its records", g, i, next[g])
				}
				next[g]++
				markers++
			}
			if markers != tt.goroutines*tt.rounds {
				t.Fatalf("decoded %d whole marker records, want %d", markers, tt.goroutines*tt.rounds)
			}
		})
	}
}

// fakeGitOnPath puts a git on PATH that only appends a line to the file it
// returns and exits 1, so a test can count the git processes started.
func fakeGitOnPath(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	started := filepath.Join(t.TempDir(), "git-started")
	script := "#!/bin/sh\necho started >> " + strconv.Quote(started) + "\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	return started
}

// gitStarts counts the fake git's runs.
func gitStarts(t *testing.T, started string) int {
	t.Helper()
	f, err := os.Open(started)
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	n := 0
	for sc := bufio.NewScanner(f); sc.Scan(); {
		n++
	}
	return n
}

// TestGitLog_FailureRefusesBeforeGitRuns: when the record cannot be opened
// or appended, every exec site returns an operational error naming
// VERDI_GITLOG, starts no git process, and notifies no observer; the
// control case shows the same fake git does run once the record succeeds.
func TestGitLog_FailureRefusesBeforeGitRuns(t *testing.T) {
	repo := gitLogRepo(t)
	for _, tt := range []struct {
		name     string
		path     func(t *testing.T) string
		wantFail bool
	}{
		{"a writable file (control)", func(t *testing.T) string { return filepath.Join(t.TempDir(), "git.jsonl") }, false},
		{"a path whose directory does not exist", func(t *testing.T) string {
			return filepath.Join(t.TempDir(), "absent", "git.jsonl")
		}, true},
		{"a directory", func(t *testing.T) string { return t.TempDir() }, true},
		{"a read-only file", func(t *testing.T) string {
			if os.Geteuid() == 0 {
				t.Skip("root writes a read-only file, so the refusal cannot be shown")
			}
			p := filepath.Join(t.TempDir(), "git.jsonl")
			if err := os.WriteFile(p, nil, 0o400); err != nil {
				t.Fatal(err)
			}
			return p
		}, true},
	} {
		for _, s := range gitLogExecSites() {
			t.Run(tt.name+"/"+s.name, func(t *testing.T) {
				t.Setenv(GitLogEnv, tt.path(t))
				started := fakeGitOnPath(t)
				obs := &recordingObserver{}
				err := s.call(WithObserver(context.Background(), obs), repo.Dir)
				if err == nil {
					t.Fatalf("%s succeeded against a fake git that always fails", s.name)
				}
				if !tt.wantFail {
					if n := gitStarts(t, started); n != 1 || len(obs.calls) != 1 {
						t.Fatalf("control: git started %d time(s), observer saw %d call(s), want 1 and 1", n, len(obs.calls))
					}
					return
				}
				if !strings.Contains(err.Error(), GitLogEnv) {
					t.Errorf("err = %v, want it to name %s", err, GitLogEnv)
				}
				if n := gitStarts(t, started); n != 0 {
					t.Errorf("git started %d time(s), want none before a failed record", n)
				}
				if len(obs.calls) != 0 {
					t.Errorf("the observer saw %v, want no call that never ran", obs.calls)
				}
			})
		}
	}
}
