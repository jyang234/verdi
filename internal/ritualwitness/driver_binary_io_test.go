package ritualwitness

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
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

// TestBinary_PinsTheCIEnvironment: CI, GITHUB_ACTIONS, and GITHUB_BASE_REF
// are set on every run from the driver's CI field, never inherited, so a
// verb that reads them (close's publish guard, for one) behaves the same
// locally and in CI; Env may not set them.
func TestBinary_PinsTheCIEnvironment(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	exe := selfBinary(t)
	tests := []struct {
		name    string
		ambient string // the value of all three in the test process
		ci      CIEnv
		want    []string
	}{
		{"an ambient CI environment does not leak in", "true", CIEnv{},
			[]string{`CI="" set=true`, `GITHUB_ACTIONS="" set=true`, `GITHUB_BASE_REF="" set=true`}},
		{"no ambient CI environment is set empty", "", CIEnv{},
			[]string{`CI="" set=true`, `GITHUB_ACTIONS="" set=true`, `GITHUB_BASE_REF="" set=true`}},
		{"the CI field is what the binary sees", "ambient", CIEnv{CI: "true", GitHubActions: "true", GitHubBaseRef: "main"},
			[]string{`CI="true" set=true`, `GITHUB_ACTIONS="true" set=true`, `GITHUB_BASE_REF="main" set=true`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range []string{"CI", "GITHUB_ACTIONS", "GITHUB_BASE_REF"} {
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
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("the binary's environment %q, want %q", err, w)
				}
			}
		})
	}

	for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITHUB_BASE_REF"} {
		t.Run("Env may not set "+key, func(t *testing.T) {
			exit, _, err := Binary{Path: exe, Env: []string{helperEnv + "=0:never-made", key + "=true"}}.Run(boundedContext(t, ctx), fx.Dir)
			if exit != -1 || err == nil || !strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), "the CI field") {
				t.Fatalf("Run = (%d, %v), want -1 naming %s and the CI field", exit, err, key)
			}
			if out, gerr := plainGit(ctx, fx.Dir, "branch", "--list", "never-made"); gerr != nil || out != "" {
				t.Fatalf("a refused Env still ran the binary (branch listing %q, %v)", out, gerr)
			}
		})
	}
}
