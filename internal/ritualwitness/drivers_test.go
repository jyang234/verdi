package ritualwitness

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	ws "github.com/jyang234/verdi/internal/writescope"
)

// branchDecl declares a ritual that may create any branch.
func branchDecl() ws.Declaration {
	return ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, RefsCreate: []ws.RefPattern{"refs/heads/*"}, IndexCarry: ws.CarryNoCommit}
}

// selfBinary is this test binary, which TestMain turns into helperVerb.
func selfBinary(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exe
}

// TestBinary_Run drives a real subprocess: the exit code is the
// process's, the directory is the fixture, Env reaches it, and the log is
// always unavailable (dc-1; spec/gitx-recorder-seam lands the binary's
// log).
func TestBinary_Run(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	exe := selfBinary(t)
	tests := []struct {
		name       string
		d          Binary
		wantExit   int
		wantErr    []string
		wantBranch string
	}{
		{"a clean run in the fixture", Binary{Path: exe, Env: []string{helperEnv + "=0:made-clean"}}, 0, nil, "made-clean"},
		{"a verdict keeps its exit and names the output", Binary{Path: exe, Args: []string{"verb", "arg"}, Env: []string{helperEnv + "=1"}}, 1,
			[]string{"exited 1", "verb arg", "helper stdout", "helper stderr"}, ""},
		{"a refusal keeps its exit", Binary{Path: exe, Env: []string{helperEnv + "=2"}}, 2, []string{"exited 2"}, ""},
		{"a binary that cannot start is no verb's exit", Binary{Path: filepath.Join(t.TempDir(), "absent")}, -1, []string{"running"}, ""},
		{"no path is no verb's exit", Binary{}, -1, []string{"no binary"}, ""},
		{"a Go panic's exit 2 is no verb's exit", Binary{Path: exe, Env: []string{helperEnv + "=panic"}}, -1,
			[]string{"panicked", "helper verb panicked", "goroutine"}, ""},
		{"a refusal whose words begin panic: but carry no trace keeps its exit", Binary{Path: exe, Env: []string{helperEnv + "=panic-words"}}, 2,
			[]string{"exited 2", "the refusal's own words"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exit, log, err := tt.d.Run(ctx, fx.Dir)
			if exit != tt.wantExit {
				t.Fatalf("exit = %d, want %d (err %v)", exit, tt.wantExit, err)
			}
			if log.OK || log.Calls != nil {
				t.Fatalf("log = %+v, want unavailable: a built binary supplies no command log", log)
			}
			if (err != nil) != (tt.wantErr != nil) {
				t.Fatalf("err = %v, want an error naming %q", err, tt.wantErr)
			}
			for _, w := range tt.wantErr {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("err = %q, want it to name %q", err, w)
				}
			}
			if tt.wantBranch != "" {
				runGitFixture(t, ctx, fx.Dir, "rev-parse", "--verify", "refs/heads/"+tt.wantBranch)
			}
		})
	}
}

// TestGoPanicTrace: a panic header followed by a goroutine stack is a
// trace; either alone, or the stack before the header, is not.
func TestGoPanicTrace(t *testing.T) {
	trace := "panic: boom\n\ngoroutine 1 [running]:\nmain.main()\n\t/x/main.go:3 +0x1d\nexit status 2\n"
	tests := []struct {
		name   string
		stderr string
		want   bool
	}{
		{"a panic and its stack", trace, true},
		{"after other output", "verdi: starting\n" + trace, true},
		{"a recovered-and-repanicked trace", "panic: boom [recovered]\n\tpanic: again\n\ngoroutine 7 [running]:\n", true},
		{"a refusal's words alone", "panic: the refusal's own words, not a trace\n", false},
		{"a stack alone", "goroutine 1 [running]:\nmain.main()\n", false},
		{"the stack before the header", "goroutine 1 [running]:\npanic: boom\n", false},
		{"the header mid-line", "build start: panic: boom\ngoroutine 1 [running]:\n", false},
		{"nothing", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := goPanicTrace(tt.stderr); got != tt.want {
				t.Fatalf("goPanicTrace(%q) = %v, want %v", tt.stderr, got, tt.want)
			}
		})
	}
}

// TestBinary_CrashIsNoVerbsExit (re-review RR-B1): the child runs with
// GOTRACEBACK=single, overriding an ambient setting that would hide or
// reshape the panic trace, while the fixture's own Env still wins; a
// runtime fatal error is a crash as a panic is; and a refusal that only
// mentions a fatal error mid-line keeps its exit 2.
func TestBinary_CrashIsNoVerbsExit(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	exe := selfBinary(t)
	tests := []struct {
		name     string
		ambient  string // GOTRACEBACK in the test process's own environment
		env      []string
		wantExit int
		wantErr  []string
	}{
		{"a panic under an ambient GOTRACEBACK=system", "system", []string{helperEnv + "=panic"}, -1, []string{"panicked", "goroutine 1 [running]:"}},
		{"a panic under an ambient GOTRACEBACK=none", "none", []string{helperEnv + "=panic"}, -1, []string{"panicked", "goroutine 1 [running]:"}},
		{"a runtime fatal error (deadlock)", "", []string{helperEnv + "=deadlock"}, -1, []string{"crashed", "fatal error: all goroutines are asleep"}},
		{"a refusal that mentions a fatal error mid-line", "", []string{helperEnv + "=fatal-words"}, 2, []string{"exited 2", "holds a fatal error"}},
		{"the child's GOTRACEBACK is single over an ambient one", "system", []string{helperEnv + "=gotraceback"}, 1, []string{"GOTRACEBACK=single"}},
		{"the fixture's own Env still wins", "system", []string{helperEnv + "=gotraceback", "GOTRACEBACK=crash"}, 1, []string{"GOTRACEBACK=crash"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.ambient != "" {
				t.Setenv("GOTRACEBACK", tt.ambient)
			}
			exit, log, err := Binary{Path: exe, Env: tt.env}.Run(ctx, fx.Dir)
			if exit != tt.wantExit {
				t.Fatalf("exit = %d, want %d (err %v)", exit, tt.wantExit, err)
			}
			if log.OK || log.Calls != nil {
				t.Fatalf("log = %+v, want unavailable", log)
			}
			for _, w := range tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), w) {
					t.Errorf("err = %v, want it to name %q", err, w)
				}
			}
		})
	}
}

// TestGoCrash: a Go panic trace or a line beginning "fatal error: " is a
// crash; "fatal error" anywhere else in a line is not.
func TestGoCrash(t *testing.T) {
	for _, tt := range []struct {
		name   string
		stderr string
		want   bool
	}{
		{"a panic trace", "panic: boom\n\ngoroutine 1 [running]:\nmain.main()\n", true},
		{"a runtime fatal error", "fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 [chan receive]:\n", true},
		{"a fatal error after other output", "verdi: starting\nfatal error: concurrent map writes\n", true},
		{"a fatal error mid-line", "close: refused: the index holds a fatal error: x\n", false},
		{"a refusal's panic words alone", "panic: the refusal's own words, not a trace\n", false},
		{"nothing", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := goCrash(tt.stderr); got != tt.want {
				t.Fatalf("goCrash(%q) = %v, want %v", tt.stderr, got, tt.want)
			}
		})
	}
}

// TestBinary_RunOnReportsTheLogUnavailable: through RunOn, an effect only
// the log could attribute stays unattributable, so the run is unproven,
// never a pass inferred from an absent log.
func TestBinary_RunOnReportsTheLogUnavailable(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	res := RunOn(t, ctx, fx, Binary{Path: selfBinary(t), Env: []string{helperEnv + "=0:made-by-binary"}}, branchDecl())
	want := []Verdict{
		v("command_log", Unattributable, "the driver supplied no git command log"),
		v("refs_create", Unattributable, "refs/heads/made-by-binary created"),
		v("index_carry", Within, "declares no_commit; observed no_commit"),
	}
	if diff := verdictDiff(res.Verdicts, want); diff != "" {
		t.Fatal(diff)
	}
	if got := Outcome(res.Verdicts); got != Unproven {
		t.Fatalf("Outcome = %s, want unproven", got)
	}
}

// branchingHandler serves one route, POST /act with body "go" and header
// X-Test "yes": it creates refs/heads/<branch> in root with plain git and
// answers status; any other request is 400.
func branchingHandler(branch string, status int) func(string) http.Handler {
	return func(root string) http.Handler {
		mux := http.NewServeMux()
		mux.HandleFunc("/act", func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			if r.Method != http.MethodPost || string(body) != "go" || r.Header.Get("X-Test") != "yes" {
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}
			if _, err := plainGit(r.Context(), root, "branch", branch); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(status)
			_, _ = io.WriteString(w, "answer body")
		})
		return mux
	}
}

// TestWorkbench_Run drives a handler over a loopback server: a 2xx answer
// is exit 0, any other is 2 with the status and body in the error, and the
// log is always unavailable (the workbench's actions root their own
// contexts).
func TestWorkbench_Run(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	hdr := http.Header{"X-Test": []string{"yes"}}
	tests := []struct {
		name       string
		d          Workbench
		wantExit   int
		wantErr    []string
		wantBranch string
	}{
		{"a 2xx answer is a clean run", Workbench{Serve: branchingHandler("made-ok", http.StatusOK), Method: http.MethodPost, Path: "/act", Body: []byte("go"), Header: hdr}, 0, nil, "made-ok"},
		{"a 201 is a clean run too", Workbench{Serve: branchingHandler("made-created", http.StatusCreated), Method: http.MethodPost, Path: "/act", Body: []byte("go"), Header: hdr}, 0, nil, "made-created"},
		{"a refusal is exit 2 naming the status and body", Workbench{Serve: branchingHandler("made-refused", http.StatusConflict), Method: http.MethodPost, Path: "/act", Body: []byte("go"), Header: hdr}, 2,
			[]string{"409", "answer body", "POST /act"}, "made-refused"},
		{"the method, body, and header reach the handler", Workbench{Serve: branchingHandler("made-never", http.StatusOK), Method: http.MethodGet, Path: "/act", Header: hdr}, 2, []string{"400", "unexpected request"}, ""},
		{"no handler is no verb's exit", Workbench{Method: http.MethodPost, Path: "/act"}, -1, []string{"no handler"}, ""},
		{"a malformed request is no verb's exit", Workbench{Serve: branchingHandler("x", http.StatusOK), Method: "BAD METHOD", Path: "/act"}, -1, []string{"request"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exit, log, err := tt.d.Run(ctx, fx.Dir)
			if exit != tt.wantExit {
				t.Fatalf("exit = %d, want %d (err %v)", exit, tt.wantExit, err)
			}
			if log.OK || log.Calls != nil {
				t.Fatalf("log = %+v, want unavailable", log)
			}
			if (err != nil) != (tt.wantErr != nil) {
				t.Fatalf("err = %v, want an error naming %q", err, tt.wantErr)
			}
			for _, w := range tt.wantErr {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("err = %q, want it to name %q", err, w)
				}
			}
			if tt.wantBranch != "" {
				runGitFixture(t, ctx, fx.Dir, "rev-parse", "--verify", "refs/heads/"+tt.wantBranch)
			}
		})
	}
}

// TestWorkbench_RedirectIsNoCleanRun (ledger SI-334 (3)): the driver
// follows no redirect, so a 303 is exit 2 naming its status, judged by
// itself and never by its target, which is never requested.
func TestWorkbench_RedirectIsNoCleanRun(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	serve := func(root string) http.Handler {
		mux := http.NewServeMux()
		mux.HandleFunc("/act", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/page?error=refused", http.StatusSeeOther)
		})
		mux.HandleFunc("/page", func(w http.ResponseWriter, r *http.Request) {
			if _, err := plainGit(r.Context(), root, "branch", "made-by-the-target"); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		})
		return mux
	}
	exit, log, err := Workbench{Serve: serve, Method: http.MethodPost, Path: "/act"}.Run(ctx, fx.Dir)
	if exit != 2 || err == nil || !strings.Contains(err.Error(), "303") {
		t.Fatalf("Run = exit %d, %v; want exit 2 naming the 303", exit, err)
	}
	if log.OK || log.Calls != nil {
		t.Fatalf("log = %+v, want unavailable", log)
	}
	if out, gerr := plainGit(ctx, fx.Dir, "branch", "--list", "made-by-the-target"); gerr != nil || out != "" {
		t.Fatalf("the redirect's target was requested (branch listing %q, %v)", out, gerr)
	}
}

// TestWorkbench_RunOnReportsTheLogUnavailable mirrors the Binary case.
func TestWorkbench_RunOnReportsTheLogUnavailable(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	d := Workbench{Serve: branchingHandler("made-by-handler", http.StatusOK), Method: http.MethodPost, Path: "/act", Body: []byte("go"),
		Header: http.Header{"X-Test": []string{"yes"}}}
	res := RunOn(t, ctx, fx, d, branchDecl())
	want := []Verdict{
		v("command_log", Unattributable, "the driver supplied no git command log"),
		v("refs_create", Unattributable, "refs/heads/made-by-handler created"),
		v("index_carry", Within, "declares no_commit; observed no_commit"),
	}
	if diff := verdictDiff(res.Verdicts, want); diff != "" {
		t.Fatal(diff)
	}
	if got := Outcome(res.Verdicts); got != Unproven {
		t.Fatalf("Outcome = %s, want unproven", got)
	}
}

// fatalRecorder is a testing.TB whose Fatalf records its message and ends
// the calling goroutine, as a real test's Fatalf does, so a test can watch
// RunOn refuse a run without failing itself.
type fatalRecorder struct {
	testing.TB
	mu    sync.Mutex
	fatal string
}

func (r *fatalRecorder) Helper() {}

func (r *fatalRecorder) Fatalf(format string, args ...any) {
	r.mu.Lock()
	r.fatal = fmt.Sprintf(format, args...)
	r.mu.Unlock()
	runtime.Goexit()
}

func (r *fatalRecorder) message() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fatal
}

// TestRunOn_RefusesAnExitNoVerbHas: a driver exit outside 0..2 (one that
// never got an exit, or one no verb uses) fails the run in RunOn itself,
// before any judgment; a verb's exit is judged.
func TestRunOn_RefusesAnExitNoVerbHas(t *testing.T) {
	ctx := context.Background()
	// Literal expectations, never verbExit itself: the oracle is not the
	// function under test (re-review RR-B3).
	for _, tt := range []struct {
		exit   int
		judged bool
	}{{-1, false}, {3, false}, {0, true}, {1, true}, {2, true}} {
		exit := tt.exit
		t.Run(strconv.Itoa(exit), func(t *testing.T) {
			fx := Build(t, ctx, SeedClean)
			d := InProcess{Fn: func(context.Context, string) (int, error) {
				if exit == 0 {
					return 0, nil
				}
				return exit, errors.New("the driver's own error")
			}}
			rec := &fatalRecorder{TB: t}
			var res Result
			done := make(chan struct{})
			go func() {
				defer close(done)
				res = RunOn(rec, ctx, fx, d, branchDecl())
			}()
			<-done
			got := rec.message()
			if tt.judged {
				if got != "" || res.Exit != exit {
					t.Fatalf("RunOn on exit %d = %+v, fatal %q; want it judged", exit, res, got)
				}
				return
			}
			if !strings.Contains(got, fmt.Sprintf("the driver reported exit %d, which is no verb's exit class", exit)) {
				t.Fatalf("RunOn on exit %d failed with %q, want it refused as no verb's exit", exit, got)
			}
		})
	}
}

// TestVerbExit: RunOn reads only 0, 1, and 2 as a verb's exit; anything
// else (a driver that never got an exit) is refused, never judged as a
// clean run or a refusal.
func TestVerbExit(t *testing.T) {
	for exit, want := range map[int]bool{-1: false, 0: true, 1: true, 2: true, 3: false, 128: false} {
		if got := verbExit(exit); got != want {
			t.Errorf("verbExit(%d) = %v, want %v", exit, got, want)
		}
	}
}
