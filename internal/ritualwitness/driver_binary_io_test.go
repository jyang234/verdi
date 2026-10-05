package ritualwitness

import (
	"context"
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
	"testing"
	"time"
)

// TestBinary_StdinAndExtraFiles: Stdin is the binary's standard input
// (the null device when nil), and ExtraFiles reach it as file descriptors
// 3 and up, as a verb reading a controller socket needs; without them, fd
// 3 is not open in the child.
func TestBinary_StdinAndExtraFiles(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	exe := selfBinary(t)
	for _, tt := range []struct {
		name     string
		stdin    io.Reader
		wantExit int
		wantErr  string
	}{
		{"stdin reaches the binary", strings.NewReader("a request on stdin\n"), 1, `stdin="a request on stdin\n"`},
		{"no stdin reads as empty", nil, 1, `stdin=""`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			exit, log, err := Binary{Path: exe, Env: []string{helperEnv + "=stdin"}, Stdin: tt.stdin}.Run(boundedContext(t, ctx), fx.Dir)
			if exit != tt.wantExit || err == nil || !strings.Contains(err.Error(), tt.wantErr) {
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
		exit, _, err := Binary{Path: exe, Env: []string{helperEnv + "=fd3"}}.Run(boundedContext(t, ctx), fx.Dir)
		if exit != 3 || err == nil || !strings.Contains(err.Error(), "writing fd 3") {
			t.Fatalf("Run = (%d, %v), want the helper's exit 3 failing to write fd 3", exit, err)
		}
	})
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

// ciVariableName matches a string literal naming a CI-context variable:
// CI itself, or a CI_, GITHUB_, or GITLAB_ name.
var ciVariableName = regexp.MustCompile(`^(CI|(CI|GITHUB|GITLAB)_[A-Z0-9_]+)$`)

// TestCIEnv_PinsEveryCIVariableTheBinaryReads: the driver pins exactly
// pinnedCIVariables, and every CI-context variable the built binary's code
// names (any string literal naming one, in a non-test Go file under
// cmd/verdi or internal: an os.Getenv argument, a constant a getenv seam
// reads) is among them, so a new CI-context read the driver does not pin
// fails here instead of leaking CI's environment into a local run's
// meaning.
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
