package ritualwitness

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/specstate"
)

// TestBinary_StdinAndExtraFiles: Stdin is the binary's standard input
// (the null device when nil; an *os.File as is; any other reader copied in
// through a pipe the driver owns), and ExtraFiles reach it as file
// descriptors 3 and up, as a verb reading a controller socket needs;
// without them, fd 3 is not open in the child.
func TestBinary_StdinAndExtraFiles(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	exe := selfBinary(t)
	for _, tt := range []struct {
		name     string
		spec     string
		stdin    func(t *testing.T) io.Reader
		wantExit int
		wantErr  string
	}{
		{"stdin reaches the binary", "stdin", func(*testing.T) io.Reader { return strings.NewReader("a request on stdin\n") }, 1, `stdin="a request on stdin\n"`},
		{"no stdin reads as empty", "stdin", func(*testing.T) io.Reader { return nil }, 1, `stdin=""`},
		{"an *os.File is the binary's standard input", "stdin", func(t *testing.T) io.Reader {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = r.Close() })
			if _, err := w.WriteString("a request in a file\n"); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			return r
		}, 1, `stdin="a request in a file\n"`},
		// The binary reads none of an input larger than a pipe's buffer and
		// exits: the copy ends at the closed pipe, which is the binary's
		// choice, not a fault, so the run is clean.
		{"input the binary leaves unread is a clean run", "0", func(*testing.T) io.Reader { return strings.NewReader(strings.Repeat("x", 1<<20)) }, 0, ""},
		{"a reader that fails is no verb's exit", "0", func(*testing.T) io.Reader {
			r, w := io.Pipe()
			_ = w.CloseWithError(errors.New("the request source failed"))
			return r
		}, -1, "feeding its standard input: the request source failed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			exit, log, err := Binary{Path: exe, Env: []string{helperEnv + "=" + tt.spec}, Stdin: tt.stdin(t)}.Run(boundedContext(t, ctx), fx.Dir)
			if exit != tt.wantExit || (err == nil) != (tt.wantErr == "") || (err != nil && !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("Run = (%d, %v), want exit %d naming %q", exit, err, tt.wantExit, tt.wantErr)
			}
			if log.OK || log.Calls != nil {
				t.Fatalf("log = %+v, want unavailable", log)
			}
		})
	}

	t.Run("extra files are file descriptors 3 and up", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = r.Close() }()
		exit, _, runErr := Binary{Path: exe, Env: []string{helperEnv + "=fd3"}, ExtraFiles: []*os.File{w}}.Run(boundedContext(t, ctx), fx.Dir)
		if cerr := w.Close(); cerr != nil {
			t.Fatal(cerr)
		}
		if exit != 0 || runErr != nil {
			t.Fatalf("Run = (%d, %v), want a clean run writing fd 3", exit, runErr)
		}
		got, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "written to fd 3\n" {
			t.Fatalf("fd 3 carried %q, want the binary's line", got)
		}
	})

	t.Run("without extra files fd 3 is not open", func(t *testing.T) {
		skipOnInheritedFD3(t)
		exit, _, err := Binary{Path: exe, Env: []string{helperEnv + "=fd3"}}.Run(boundedContext(t, ctx), fx.Dir)
		if exit != 3 || err == nil || !strings.Contains(err.Error(), "writing fd 3") {
			t.Fatalf("Run = (%d, %v), want the helper's exit 3 failing to write fd 3", exit, err)
		}
	})
}

// inheritedFD reports whether this process's file descriptor fd is open
// without close-on-exec, so a process it starts inherits it. Go opens every
// descriptor of its own close-on-exec, so such a descriptor came from the
// process that started this one (or was handed on deliberately).
func inheritedFD(fd uintptr) (bool, error) {
	flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_GETFD, 0)
	switch {
	case errno == syscall.EBADF:
		return false, nil
	case errno != 0:
		return false, fmt.Errorf("reading file descriptor %d's flags: %w", fd, errno)
	}
	return flags&syscall.FD_CLOEXEC == 0, nil
}

// skipOnInheritedFD3 skips t, with the reason, when this test process
// itself holds an inherited file descriptor 3: every binary it starts
// inherits that descriptor too, so fd 3's absence in the child cannot be
// shown. Go's own os/exec TestExtraFiles skips the same way when the test
// runs with unexpected descriptors open (R3ab review R3-B7).
func skipOnInheritedFD3(t *testing.T) {
	t.Helper()
	inherited, err := inheritedFD(3)
	if err != nil {
		t.Fatal(err)
	}
	if inherited {
		t.Skip("this test process inherited an open file descriptor 3 (not close-on-exec), which every binary it starts inherits too, so fd 3's absence cannot be shown here")
	}
}

// TestInheritedFD: a descriptor Go opened (close-on-exec) is not
// inherited, the same descriptor with close-on-exec cleared is, and a
// descriptor that is not open is not, so skipOnInheritedFD3 skips exactly
// when a started binary would inherit fd 3.
func TestInheritedFD(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(); _ = w.Close() }()
	setCloseOnExec := func(fd uintptr, on bool) {
		t.Helper()
		var flags uintptr
		if on {
			flags = syscall.FD_CLOEXEC
		}
		if _, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_SETFD, flags); errno != 0 {
			t.Fatalf("setting fd %d's close-on-exec to %v: %v", fd, on, errno)
		}
	}
	for _, tt := range []struct {
		name  string
		fd    func() uintptr
		clear bool
		want  bool
	}{
		{"a descriptor Go opened is close-on-exec", w.Fd, false, false},
		{"a descriptor without close-on-exec is inherited", w.Fd, true, true},
		{"a descriptor that is not open is not inherited", func() uintptr { return 1 << 20 }, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fd := tt.fd()
			if tt.clear {
				setCloseOnExec(fd, false)
				defer setCloseOnExec(fd, true)
			}
			got, err := inheritedFD(fd)
			if err != nil || got != tt.want {
				t.Fatalf("inheritedFD(%d) = (%v, %v), want %v", fd, got, err, tt.want)
			}
		})
	}
}

// TestBinary_StdinThatNeverEndsIsBounded: a Stdin reader that never reaches
// EOF (here an io.Pipe nobody closes) cannot hold Run past its bounds
// (R3ab review R3-B1). A binary that exits at once returns -1, naming the
// standard input that never ended, when its context ends (the reviewer's
// P1) or binaryWaitDelay after its exit, whichever comes first; one that
// outlives its context is killed and returns -1 naming the context (P3).
// Each case releases the reader only after Run returns, or after the bound
// has passed, so a hang fails the test instead of the package.
func TestBinary_StdinThatNeverEndsIsBounded(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	exe := selfBinary(t)
	const slack = 10 * time.Second
	for _, tt := range []struct {
		name, spec   string
		runFor       time.Duration
		bound        time.Duration
		wantDeadline bool
		wantErr      []string
	}{
		{"a binary that exits at once, before its context ends", "0", 3 * time.Second, 3*time.Second + binaryWaitDelay + slack, true,
			[]string{"exited 0", "standard input never reached EOF before the context ended"}},
		{"a binary that exits at once, well within its context", "0", helperBound, binaryWaitDelay + slack, false,
			[]string{"exited 0", "standard input never reached EOF within " + binaryWaitDelay.String()}},
		{"a binary that outlives its context", "linger", 2 * time.Second, 2*time.Second + binaryWaitDelay + slack, true,
			[]string{"did not exit before its context ended", "killed"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r, w := io.Pipe()
			defer func() { _ = w.Close() }()
			runCtx, cancel := context.WithTimeout(ctx, tt.runFor)
			defer cancel()
			type result struct {
				exit int
				err  error
			}
			done := make(chan result, 1)
			start := time.Now()
			go func() {
				exit, _, err := Binary{Path: exe, Env: []string{helperEnv + "=" + tt.spec}, Stdin: r}.Run(runCtx, fx.Dir)
				done <- result{exit, err}
			}()
			var got result
			select {
			case got = <-done:
			case <-time.After(tt.bound):
				_ = w.Close()
				select {
				case got = <-done:
				case <-time.After(helperBound):
				}
				t.Fatalf("Run was still blocked %s after it began, past its bound; released, it returned (%d, %v)", tt.bound, got.exit, got.err)
			}
			if elapsed := time.Since(start); elapsed > tt.bound {
				t.Fatalf("Run returned after %s, past its bound %s", elapsed, tt.bound)
			}
			if got.exit != -1 || got.err == nil {
				t.Fatalf("Run = (%d, %v), want -1, no verb's exit", got.exit, got.err)
			}
			if errors.Is(got.err, context.DeadlineExceeded) != tt.wantDeadline {
				t.Errorf("err = %q wraps the context's deadline: %v, want %v", got.err, !tt.wantDeadline, tt.wantDeadline)
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(got.err.Error(), want) {
					t.Errorf("err = %q, want it to name %q", got.err, want)
				}
			}
			if strings.Contains(got.err.Error(), "left its output open") {
				t.Errorf("err = %q names output left open, which is not the cause", got.err)
			}
		})
	}
}

// allCIEnv returns a CIEnv whose every field is value(key), key being the
// variable the field pins.
func allCIEnv(value func(key string) string) CIEnv {
	var c CIEnv
	v := reflect.ValueOf(&c).Elem()
	for i, key := range ciEnvKeys() {
		v.Field(i).SetString(value(key))
	}
	return c
}

// helperCIEnvLine is one line helper spec "cienv" prints.
var helperCIEnvLine = regexp.MustCompile(`^([A-Z0-9_]+)=(".*") set=(true|false)$`)

// helperCIEnv reads, from the error a Binary run of helper spec "cienv"
// returns, every variable the helper saw set, with its value.
func helperCIEnv(t *testing.T, runErr error) map[string]string {
	t.Helper()
	_, stderr, ok := strings.Cut(runErr.Error(), "\nstderr: ")
	if !ok {
		t.Fatalf("the helper's error carries no stderr: %v", runErr)
	}
	seen := map[string]string{}
	for _, line := range strings.Split(stderr, "\n") {
		m := helperCIEnvLine.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("helper line %q is not KEY=\"value\" set=bool", line)
		}
		if m[3] == "true" {
			v, err := strconv.Unquote(m[2])
			if err != nil {
				t.Fatal(err)
			}
			seen[m[1]] = v
		}
	}
	return seen
}

// TestBinary_PinsTheCIEnvironment: every CI-context variable is set on
// every run from the driver's CI field, empty values included, and never
// inherited, so a verb that reads one (close's publish guard, base
// resolution, the forge and CI-ref facts) behaves the same locally and in
// CI; Env may not set one.
func TestBinary_PinsTheCIEnvironment(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	exe := selfBinary(t)
	empty := func(string) string { return "" }
	tests := []struct {
		name    string
		ambient string // the value of every pinned variable in the test process; "" unsets it
		ci      CIEnv
		want    func(key string) string
	}{
		{"an ambient CI environment does not leak in", "ambient", CIEnv{}, empty},
		{"no ambient CI environment is set empty", "", CIEnv{}, empty},
		{"the CI field is what the binary sees", "ambient", allCIEnv(func(k string) string { return "field-" + k }), func(k string) string { return "field-" + k }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range ciEnvKeys() {
				t.Setenv(k, tt.ambient)
				if tt.ambient == "" {
					if err := os.Unsetenv(k); err != nil {
						t.Fatal(err)
					}
				}
			}
			exit, _, err := Binary{Path: exe, Env: []string{helperEnv + "=cienv"}, CI: tt.ci}.Run(boundedContext(t, ctx), fx.Dir)
			if exit != 1 || err == nil {
				t.Fatalf("Run = (%d, %v), want the helper's exit 1 printing its environment", exit, err)
			}
			seen := helperCIEnv(t, err)
			for _, k := range ciEnvKeys() {
				if got, ok := seen[k]; !ok || got != tt.want(k) {
					t.Errorf("the binary saw %s = %q (set %v), want %q set", k, got, ok, tt.want(k))
				}
			}
		})
	}

	t.Run("Env may not set a pinned variable", func(t *testing.T) {
		for _, key := range ciEnvKeys() {
			exit, _, err := Binary{Path: exe, Env: []string{helperEnv + "=0:never-made", key + "=true"}}.Run(boundedContext(t, ctx), fx.Dir)
			if exit != -1 || err == nil || !strings.Contains(err.Error(), "sets "+key+";") || !strings.Contains(err.Error(), "the CI field") {
				t.Errorf("Env %s=true: Run = (%d, %v), want -1 naming %s and the CI field", key, exit, err, key)
			}
		}
		if out, gerr := plainGit(ctx, fx.Dir, "branch", "--list", "never-made"); gerr != nil || out != "" {
			t.Fatalf("a refused Env still ran the binary (branch listing %q, %v)", out, gerr)
		}
	})
}

// TestPinCIEnv_KeepsAmbientCIFromInProcessReads: under PinCIEnv every
// variable CIEnv names holds its field's value, empty ones set empty, in
// the test process itself, so an ambient CI_DEFAULT_BRANCH no longer
// reaches specstate's default-branch read, which code an in-process driver
// runs (the MCP server's tools) makes (R3ab review R3-B4).
func TestPinCIEnv_KeepsAmbientCIFromInProcessReads(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	for _, branch := range []string{"ambient-default", "field-default"} {
		if _, err := plainGit(ctx, fx.Dir, "branch", branch); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CI_DEFAULT_BRANCH", "ambient-default")
	// The control: unpinned, the ambient value is what the read sees.
	if b, ok := specstate.ResolveDefaultBranch(ctx, fx.Dir); !ok || b.Name != "ambient-default" {
		t.Fatalf("unpinned, the default branch reads %+v (resolved %v), want the ambient ambient-default", b, ok)
	}
	for _, tt := range []struct {
		name string
		ci   CIEnv
		want specstate.Branch
	}{
		{"the zero CIEnv hides the ambient value", CIEnv{}, specstate.Branch{Name: "main", Ref: "origin/main"}},
		{"the CI field is what the read sees", CIEnv{CIDefaultBranch: "field-default"}, specstate.Branch{Name: "field-default", Ref: "field-default"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			PinCIEnv(t, tt.ci)
			want := map[string]string{}
			for _, kv := range tt.ci.pairs() {
				k, v, _ := strings.Cut(kv, "=")
				want[k] = v
			}
			for _, k := range ciEnvKeys() {
				if got, ok := os.LookupEnv(k); !ok || got != want[k] {
					t.Errorf("under PinCIEnv %s = %q (set %v), want %q set", k, got, ok, want[k])
				}
			}
			if b, ok := specstate.ResolveDefaultBranch(ctx, fx.Dir); !ok || b != tt.want {
				t.Fatalf("under PinCIEnv the default branch reads %+v (resolved %v), want %+v", b, ok, tt.want)
			}
		})
	}
}

// ciVariableName matches a string literal naming a CI-context variable:
// CI itself, or a CI_, GITHUB_, or GITLAB_ name.
var ciVariableName = regexp.MustCompile(`^(CI|(CI|GITHUB|GITLAB)_[A-Z0-9_]+)$`)

// TestCIEnv_PinsEveryCIVariableTheBinaryReads: the driver pins exactly
// pinnedCIVariables, and every CI-context variable the built binary's code
// names literally is among them, so a new literal CI-context read the
// driver does not pin fails here instead of leaking CI's environment into
// a local run's meaning (ledger SI-344 (2), as narrowed after R3ab review
// R3-B3).
//
// What the scan proves: every Go string literal, in a non-test Go file
// under cmd/verdi or internal, whose whole value is CI or a CI_, GITHUB_,
// or GITLAB_ name (an os.Getenv or os.LookupEnv argument, a constant a
// getenv seam reads) names a pinned variable; and the scan finds the
// reads it exists for, so a scan that sees nothing fails.
//
// What it does not prove: a name computed at run time (GITHUB_ joined to
// a suffix); a variable outside those prefixes (RUNNER_*, ACTIONS_*); a
// read whose meaning hangs on os.LookupEnv's set-versus-unset, which a
// pinned empty value answers "set"; and a read made by a subprocess the
// binary starts. None of these exists at this revision; each is disclosed,
// not witnessed.
func TestCIEnv_PinsEveryCIVariableTheBinaryReads(t *testing.T) {
	// Every CI-context variable the Binary driver pins, listed here so a
	// change to the set is a reviewed change to this test.
	pinnedCIVariables := []string{
		"CI", "CI_API_V4_URL", "CI_COMMIT_BRANCH", "CI_COMMIT_REF_NAME", "CI_COMMIT_TAG",
		"CI_DEFAULT_BRANCH", "CI_JOB_ID", "CI_JOB_NAME", "CI_JOB_TOKEN", "CI_MERGE_REQUEST_IID",
		"CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "CI_PIPELINE_ID", "CI_PIPELINE_URL", "CI_PROJECT_ID",
		"CI_PROJECT_URL", "GITHUB_ACTIONS", "GITHUB_API_URL", "GITHUB_BASE_REF", "GITHUB_EVENT_NAME",
		"GITHUB_HEAD_REF", "GITHUB_JOB", "GITHUB_REF_NAME", "GITHUB_REF_TYPE", "GITHUB_REPOSITORY",
		"GITHUB_REPOSITORY_OWNER", "GITHUB_RUN_ATTEMPT", "GITHUB_RUN_ID", "GITHUB_SERVER_URL",
		"GITHUB_SHA", "GITHUB_TOKEN", "GITHUB_WORKFLOW_REF", "GITLAB_CI",
	}
	pinned := ciEnvKeys()
	got := slices.Sorted(slices.Values(pinned))
	if want := slices.Sorted(slices.Values(pinnedCIVariables)); !slices.Equal(got, want) {
		t.Errorf("the Binary driver pins %v, want exactly %v", got, want)
	}
	if len(slices.Compact(slices.Clone(got))) != len(got) {
		t.Fatalf("the Binary driver pins a variable twice: %v", pinned)
	}

	read := map[string][]string{}
	fset := token.NewFileSet()
	for _, dir := range []string{filepath.Join("..", "..", "cmd", "verdi"), filepath.Join("..", "..", "internal")} {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			ast.Inspect(f, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				if name, err := strconv.Unquote(lit.Value); err == nil && ciVariableName.MatchString(name) {
					read[name] = append(read[name], fset.Position(lit.Pos()).String())
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("scanning %s: %v", dir, err)
		}
	}
	// The scan must see the reads it exists for: silence is no pass.
	for _, known := range []string{"CI", "GITHUB_ACTIONS", "GITHUB_REF_NAME", "CI_DEFAULT_BRANCH"} {
		if len(read[known]) == 0 {
			t.Fatalf("the scan found no read of %s, which internal/lint and internal/repositoryfacts make: the scan is broken", known)
		}
	}
	for name, at := range read {
		if !slices.Contains(pinned, name) {
			t.Errorf("%s names CI-context variable %s, which the Binary driver does not pin (add a CIEnv field for it)", strings.Join(at, ", "), name)
		}
	}
}

// TestBinary_WaitDelayBoundsAnOrphanedPipe: a binary that exits while a
// process it started still holds its stdout and stderr ends the run
// within the driver's WaitDelay, as no verb's exit, instead of hanging
// until that process exits.
func TestBinary_WaitDelayBoundsAnOrphanedPipe(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	t.Cleanup(func() {
		data, err := os.ReadFile(pidFile)
		if err != nil {
			return
		}
		if pid, err := strconv.Atoi(string(data)); err == nil {
			if p, err := os.FindProcess(pid); err == nil {
				_ = p.Kill()
				_, _ = p.Wait()
			}
		}
	})
	start := time.Now()
	exit, _, err := Binary{Path: selfBinary(t), Env: []string{helperEnv + "=orphan", helperPIDFileEnv + "=" + pidFile}}.Run(boundedContext(t, ctx), fx.Dir)
	elapsed := time.Since(start)
	if bound := binaryWaitDelay + 10*time.Second; elapsed > bound {
		t.Fatalf("Run returned after %s, past %s: the orphaned pipe held Wait", elapsed, bound)
	}
	if exit != -1 || err == nil || !strings.Contains(err.Error(), "left its output open") {
		t.Fatalf("Run = (%d, %v), want -1 naming the output left open", exit, err)
	}
	if _, err := os.Stat(pidFile); err != nil {
		t.Fatalf("the helper recorded no grandchild, so the case proves nothing: %v", err)
	}
}
