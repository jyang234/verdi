package ritualwitness

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitforbid"
	"github.com/jyang234/verdi/internal/gitx"
)

// helperGitxVerb is the helper verb's gitx-driven specs, reporting false
// for any other spec. Spec "gitx:<exit>:<branch>" creates <branch> in its
// working directory through gitx (RevParse, then UpdateRef), so the
// binary's VERDI_GITLOG records both calls, and exits <exit>. Spec
// "gitx-child:<branch>" does as "gitx:0:<branch>", then runs this binary
// as spec "gitx:0:<branch>-child", which inherits VERDI_GITLOG and records
// under its own pid, and exits 0. Spec "gitlog:<mode>:<exit>" creates
// branch "logged-<mode>" as "gitx" does, then breaks the VERDI_GITLOG file
// as <mode> names (brokenGitLogs), and exits <exit>.
func helperGitxVerb(spec string) (int, bool) {
	kind, rest, _ := strings.Cut(spec, ":")
	switch kind {
	case "gitx":
		code, branch, _ := strings.Cut(rest, ":")
		exit, err := strconv.Atoi(code)
		if err != nil {
			fmt.Fprintln(os.Stderr, "helper: bad exit:", err)
			return 3, true
		}
		if err := gitxBranch(branch); err != nil {
			fmt.Fprintln(os.Stderr, "helper:", err)
			return 3, true
		}
		fmt.Println("helper stdout")
		fmt.Fprintln(os.Stderr, "helper stderr")
		return exit, true
	case "gitx-child":
		if err := gitxBranch(rest); err != nil {
			fmt.Fprintln(os.Stderr, "helper:", err)
			return 3, true
		}
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintln(os.Stderr, "helper: locating itself:", err)
			return 3, true
		}
		cmd := exec.CommandContext(context.Background(), exe)
		cmd.Env = append(os.Environ(), helperEnv+"=gitx:0:"+rest+"-child")
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "helper: the child verb: %v\n%s", err, out)
			return 3, true
		}
		return 0, true
	case "gitlog":
		mode, code, _ := strings.Cut(rest, ":")
		exit, err := strconv.Atoi(code)
		if err != nil {
			fmt.Fprintln(os.Stderr, "helper: bad exit:", err)
			return 3, true
		}
		if err := gitxBranch("logged-" + mode); err != nil {
			fmt.Fprintln(os.Stderr, "helper:", err)
			return 3, true
		}
		if err := breakGitLog(mode); err != nil {
			fmt.Fprintln(os.Stderr, "helper: breaking the command log:", err)
			return 3, true
		}
		fmt.Fprintln(os.Stderr, "helper stderr")
		return exit, true
	}
	return 0, false
}

// gitxBranch creates branch at HEAD in the working directory through gitx.
func gitxBranch(branch string) error {
	ctx := context.Background()
	head, err := gitx.RevParse(ctx, ".", "HEAD")
	if err != nil {
		return err
	}
	return gitx.UpdateRef(ctx, ".", "refs/heads/"+branch, head)
}

// brokenGitLogs is every way spec "gitlog" breaks the command log, each
// with the text it appends to it ("remove" deletes it instead). Every one
// must leave the run's log unavailable, never read as fewer calls.
func brokenGitLogs(pid int) map[string]string {
	p := strconv.Itoa(pid)
	return map[string]string{
		"garbage":       "not a record\n",
		"unknown":       `{"args":["status"],"dir":"/","extra":1,"pid":` + p + "}\n",
		"trailing":      `{"args":["status"],"dir":"/","pid":` + p + "} {}\n",
		"partial":       `{"args":["status"],"dir":"/","pid":` + p + "}",
		"blankline":     "\n",
		"emptyobject":   "{}\n",
		"nopid":         `{"args":["status"],"dir":"/"}` + "\n",
		"noargs":        `{"args":[],"dir":"/","pid":` + p + "}\n",
		"nullargs":      `{"args":null,"dir":"/","pid":` + p + "}\n",
		"relativedir":   `{"args":["status"],"dir":"repo","pid":` + p + "}\n",
		"stringpid":     `{"args":["status"],"dir":"/","pid":"` + p + `"}` + "\n",
		"otherpidbroke": `{"args":["status"],"dir":"/","pid":1,"x":0}` + "\n",
		"remove":        "",
	}
}

// breakGitLog breaks this process's VERDI_GITLOG file as mode names.
func breakGitLog(mode string) error {
	path := os.Getenv(gitx.GitLogEnv)
	if path == "" {
		return errors.New("VERDI_GITLOG is not set")
	}
	if mode == "remove" {
		return os.Remove(path)
	}
	text, ok := brokenGitLogs(os.Getpid())[mode]
	if !ok {
		return fmt.Errorf("no mode %q", mode)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(text); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// checkBinaryLog checks the log a Binary run returned: a run that ends in
// an exit code carries the binary's log, read from its VERDI_GITLOG, with
// calls calls; one that ends in no verb's exit (-1) carries none.
func checkBinaryLog(t *testing.T, exit int, log CommandLog, calls int) {
	t.Helper()
	if exit < 0 {
		if log.OK || log.Calls != nil {
			t.Fatalf("log = %+v, want none for a run with no verb's exit", log)
		}
		return
	}
	if !log.OK || len(log.Calls) != calls {
		t.Fatalf("log = %+v, want the binary's log with %d call(s)", log, calls)
	}
}

// branchCalls is the log of gitxBranch creating branch at head in dir.
func branchCalls(dir, branch, head string) []Call {
	return []Call{
		{Dir: dir, Args: []string{"rev-parse", "--verify", "HEAD"}},
		{Dir: dir, Args: []string{"branch", branch, head}},
	}
}

// canonicalCalls is calls with each directory canonical, for comparing a
// binary's absolute directories with the fixture's.
func canonicalCalls(calls []Call) []Call {
	out := make([]Call, len(calls))
	for i, c := range calls {
		out[i] = Call{Dir: canonicalPath("", c.Dir), Args: c.Args}
	}
	return out
}

// TestBinary_LogsTheBinarysGitCalls (spec/gitx-recorder-seam ac-2): the
// driver gives each run its own VERDI_GITLOG and returns the binary's git
// calls from it, in order, on a clean exit and on a refusal alike.
func TestBinary_LogsTheBinarysGitCalls(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	exe := selfBinary(t)
	for _, tt := range []struct {
		name     string
		branch   string
		exit     int
		wantErr  string
		wantExit int
	}{
		{"a clean run", "logged-clean", 0, "", 0},
		{"a refusal keeps its log", "logged-refused", 2, "exited 2", 2},
		{"a verdict keeps its log", "logged-verdict", 1, "exited 1", 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			spec := fmt.Sprintf("gitx:%d:%s", tt.exit, tt.branch)
			exit, log, err := Binary{Path: exe, Env: []string{helperEnv + "=" + spec}}.Run(boundedContext(t, ctx), fx.Dir)
			if exit != tt.wantExit || (err == nil) != (tt.wantErr == "") || (err != nil && !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("Run = (%d, %v), want exit %d naming %q", exit, err, tt.wantExit, tt.wantErr)
			}
			if !log.OK {
				t.Fatalf("log = %+v, want the binary's log", log)
			}
			want := branchCalls(canonicalPath("", fx.Dir), tt.branch, fx.BaseCommit)
			if got := canonicalCalls(log.Calls); !reflect.DeepEqual(got, want) {
				t.Fatalf("logged %+v, want %+v", got, want)
			}
			for _, c := range log.Calls {
				if !filepath.IsAbs(c.Dir) {
					t.Fatalf("logged directory %q is not absolute", c.Dir)
				}
			}
		})
	}
}

// TestBinary_GitLogIsTheDriversOwn: an inherited VERDI_GITLOG never reaches
// the binary, which logs to the run's own file, and Env may not set it.
func TestBinary_GitLogIsTheDriversOwn(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	exe := selfBinary(t)

	t.Run("an inherited VERDI_GITLOG is stripped", func(t *testing.T) {
		ambient := filepath.Join(t.TempDir(), "ambient.jsonl")
		t.Setenv(gitx.GitLogEnv, ambient)
		exit, log, err := Binary{Path: exe, Env: []string{helperEnv + "=gitx:0:logged-not-ambient"}}.Run(boundedContext(t, ctx), fx.Dir)
		if exit != 0 || err != nil {
			t.Fatalf("Run = (%d, %v), want a clean run", exit, err)
		}
		checkBinaryLog(t, exit, log, 2)
		if _, err := os.Stat(ambient); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("the inherited VERDI_GITLOG %s was written (stat err %v)", ambient, err)
		}
	})

	t.Run("Env may not set VERDI_GITLOG", func(t *testing.T) {
		mine := filepath.Join(t.TempDir(), "mine.jsonl")
		exit, log, err := Binary{Path: exe, Env: []string{helperEnv + "=gitx:0:logged-never", gitx.GitLogEnv + "=" + mine}}.Run(boundedContext(t, ctx), fx.Dir)
		if exit != -1 || err == nil || !strings.Contains(err.Error(), "Env sets "+gitx.GitLogEnv) {
			t.Fatalf("Run = (%d, %v), want -1 refusing Env's %s", exit, err, gitx.GitLogEnv)
		}
		checkBinaryLog(t, exit, log, 0)
		if out, gerr := plainGit(ctx, fx.Dir, "branch", "--list", "logged-never"); gerr != nil || out != "" {
			t.Fatalf("the binary ran (branch listing %q, %v)", out, gerr)
		}
	})
}

// TestBinary_KeepsOnlyItsOwnRecords (SI-359 (3), (15)): a process the
// binary starts inherits VERDI_GITLOG and records under its own pid; the
// driver keeps only the binary's records, so the child's effect is left
// unattributable, the disclosed cost of the pid filter.
func TestBinary_KeepsOnlyItsOwnRecords(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	d := Binary{Path: selfBinary(t), Env: []string{helperEnv + "=gitx-child:logged-parent"}}
	res := RunOn(t, boundedContext(t, ctx), fx, d, branchDecl())
	want := branchCalls(canonicalPath("", fx.Dir), "logged-parent", fx.BaseCommit)
	if got := canonicalCalls(res.Log.Calls); !res.Log.OK || !reflect.DeepEqual(got, want) {
		t.Fatalf("log = %+v, want only the binary's own calls %+v", res.Log, want)
	}
	wantVerdicts := []Verdict{
		v("refs_create", Within, "refs/heads/logged-parent created"),
		v("refs_create", Unattributable, "refs/heads/logged-parent-child created"),
		v("index_carry", Within, "declares no_commit; observed no_commit"),
	}
	if diff := verdictDiff(res.Verdicts, wantVerdicts); diff != "" {
		t.Fatal(diff)
	}
}

// TestBinary_AnUnreadableLogFailsClosed: a command log that is gone, or
// holds any line that is not one whole, strictly decoded record, is
// reported unavailable, never read as fewer calls; a refusal's error also
// names why.
func TestBinary_AnUnreadableLogFailsClosed(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	exe := selfBinary(t)
	for mode := range brokenGitLogs(0) {
		t.Run(mode, func(t *testing.T) {
			exit, log, err := Binary{Path: exe, Env: []string{helperEnv + "=gitlog:" + mode + ":0"}}.Run(boundedContext(t, ctx), fx.Dir)
			if exit != 0 || err != nil {
				t.Fatalf("Run = (%d, %v), want a clean run", exit, err)
			}
			if log.OK || log.Calls != nil {
				t.Fatalf("log = %+v, want unavailable for a broken command log", log)
			}
			// A clean exit carries no error, so the log itself keeps why
			// it is unavailable (R5c1 review R5C1R-5).
			if !strings.Contains(log.Reason, "command log") {
				t.Fatalf("log.Reason = %q, want it to name why the command log is unavailable", log.Reason)
			}
		})
	}

	t.Run("a refusal names the broken log", func(t *testing.T) {
		fx := Build(t, ctx, SeedClean)
		exit, log, err := Binary{Path: exe, Env: []string{helperEnv + "=gitlog:garbage:2"}}.Run(boundedContext(t, ctx), fx.Dir)
		if exit != 2 || err == nil || !strings.Contains(err.Error(), "exited 2") || !strings.Contains(err.Error(), "command log") {
			t.Fatalf("Run = (%d, %v), want exit 2 naming the verb's exit and the command log", exit, err)
		}
		if log.OK || log.Calls != nil || !strings.Contains(log.Reason, "command log") {
			t.Fatalf("log = %+v, want unavailable, naming why", log)
		}
	})

	t.Run("through RunOn the logged effect is unattributable", func(t *testing.T) {
		fx := Build(t, ctx, SeedClean)
		res := RunOn(t, boundedContext(t, ctx), fx, Binary{Path: exe, Env: []string{helperEnv + "=gitlog:remove:0"}}, branchDecl())
		if res.Log.Reason == "" {
			t.Fatalf("log = %+v, want the reason it is unavailable", res.Log)
		}
		want := []Verdict{
			v("command_log", Unattributable, "the driver supplied no git command log: "+res.Log.Reason),
			v("refs_create", Unattributable, "refs/heads/logged-remove created"),
			v("index_carry", Within, "declares no_commit; observed no_commit"),
		}
		if diff := verdictDiff(res.Verdicts, want); diff != "" {
			t.Fatal(diff)
		}
		if got := Outcome(res.Verdicts); got != Unproven {
			t.Fatalf("Outcome = %s, want unproven", got)
		}
	})
}

// TestBinary_GitLogPathIsAbsolute (R5c1 review R5C1R-5): the run's command
// log is named by an absolute path even when TMPDIR is relative, so the
// binary, which runs in the fixture, never resolves it there: the log
// holds the binary's calls, and the fixture shows no stray file.
func TestBinary_GitLogPathIsAbsolute(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	exe := selfBinary(t)
	t.Chdir(t.TempDir())
	if err := os.Mkdir("rel", 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", "rel")

	path, err := newGitLog()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	if !filepath.IsAbs(path) {
		t.Fatalf("newGitLog() = %q under TMPDIR=rel, want an absolute path", path)
	}

	before := plainGitStatus(t, ctx, fx.Dir)
	exit, log, err := Binary{Path: exe, Env: []string{helperEnv + "=gitx:0:logged-relative-tmpdir"}}.Run(boundedContext(t, ctx), fx.Dir)
	if exit != 0 || err != nil {
		t.Fatalf("Run = (%d, %v), want a clean run", exit, err)
	}
	checkBinaryLog(t, exit, log, 2)
	if after := plainGitStatus(t, ctx, fx.Dir); after != before {
		t.Fatalf("the fixture's status changed from %q to %q: the command log landed in it", before, after)
	}
}

// plainGitStatus is dir's porcelain status with untracked files listed.
func plainGitStatus(t *testing.T, ctx context.Context, dir string) string {
	t.Helper()
	out, err := plainGit(ctx, dir, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestGitLog_InvalidUTF8FailsClosed pins the disclosure R5c1 review
// R5C1R-2 asked for: a VERDI_GITLOG record spells each invalid UTF-8 byte
// as U+FFFD, so a pathspec naming such a path matches nothing (its effect
// is unattributable, never within), while a forbidden token's whole
// element, or a "--" token's prefix, still matches.
func TestGitLog_InvalidUTF8FailsClosed(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	path := filepath.Join(t.TempDir(), "git.jsonl")
	t.Setenv(gitx.GitLogEnv, path)
	const bad = "bad\xff.txt"
	// git refuses both pathspecs (no such file); gitx logs each call first.
	_ = gitx.AddPaths(ctx, fx.Dir, bad)
	_ = gitx.AddPaths(ctx, fx.Dir, "--force\xff")
	calls, err := readGitLog(path, os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("logged %+v, want the two add calls", calls)
	}
	if got, want := calls[0].Args, []string{"add", "--", "bad\uFFFD.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logged argv %q, want %q", got, want)
	}
	lc, ok := mutatingCall(calls[0], mutatingPrimitives())
	if !ok {
		t.Fatalf("the logged add %+v is no mutating primitive", calls[0])
	}
	specs, _ := lc.pathspecs()
	root := canonicalPath("", fx.Dir)
	if pathspecMatches(root, lc, specs, bad) {
		t.Fatalf("the U+FFFD spelling %q matched the real path %q; attribution must fail closed", specs, bad)
	}
	if !gitforbid.Forbids(calls[1].Args) {
		t.Fatalf("logged argv %q: the --force prefix no longer reads forbidden", calls[1].Args)
	}
}
