package ritualwitness

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
