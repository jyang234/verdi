// serve_test.go carries `verdi serve`'s in-process startup-seam unit
// tests (co-1: cmdServeWithDeps with a fake builder and a fake runner —
// no listener, no lock, no judge, no network). The real two-process
// integration tests live in serve_integration_test.go.
package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/jyang234/verdi/internal/readinessload"
)

// startupRequestLine is the EXACT byte sequence R-RR1-16 requires on
// stdout when `verdi serve --context-request` warmed up successfully. The
// ruling disclosed the startup request's target ONCE, here, on stdout —
// never inside any derived document's bytes, because a request-specific
// witness in the document would re-create the ac-4 parity divergence
// R-RR1-15 closed. The line is pinned literally (not composed from
// serve.go's own format string) so a reworded sentence reds this test.
const startupRequestLine = "readiness: the startup context request targets spec/startup-target; every other spec derives without a request\n"

// TestServeStartupRequestLine is R-RR1-16: exactly one stdout line naming
// the startup context request's own target spec, printed after the
// warm-up succeeds and before the server run is entered, and nothing at
// all when no --context-request was supplied.
func TestServeStartupRequestLine(t *testing.T) {
	newDeps := func(t *testing.T, targetSpec string, run serveRunner) serveCommandDeps {
		t.Helper()
		return serveCommandDeps{
			findRoot: func(string) (string, error) { return "/store", nil },
			readiness: readinessSnapshotBuilderFunc(func(context.Context, string, string) (string, *readinessload.PredecodedRequest, error) {
				return targetSpec, &readinessload.PredecodedRequest{}, nil
			}),
			run: run,
		}
	}

	t.Run("a supplied request names its target exactly once, before the run", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		atRun := ""
		deps := newDeps(t, "spec/startup-target", func(_, _ string, _ readinessload.Loader, _ string, _, _ io.Writer) int {
			atRun = stdout.String()
			return 0
		})
		if code := cmdServeWithDeps([]string{"--context-request", "request.json"}, &stdout, &stderr, deps); code != 0 {
			t.Fatalf("exit = %d, stderr=%q", code, stderr.String())
		}
		if atRun != startupRequestLine {
			t.Fatalf("stdout as the server run was entered = %q, want exactly %q", atRun, startupRequestLine)
		}
		if stdout.String() != startupRequestLine {
			t.Fatalf("stdout = %q, want exactly one copy of %q", stdout.String(), startupRequestLine)
		}
		if stderr.Len() != 0 {
			t.Fatalf("stderr = %q, want nothing (the line is stdout's)", stderr.String())
		}
	})

	t.Run("a ref that already carries the prefix is never doubled", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		deps := newDeps(t, "spec/startup-target", func(string, string, readinessload.Loader, string, io.Writer, io.Writer) int { return 0 })
		if code := cmdServeWithDeps([]string{"--context-request", "request.json"}, &stdout, &stderr, deps); code != 0 {
			t.Fatalf("exit = %d, stderr=%q", code, stderr.String())
		}
		if bytes.Contains(stdout.Bytes(), []byte("spec/spec/")) {
			t.Fatalf("stdout = %q, want no doubled ref prefix", stdout.String())
		}
	})

	t.Run("no request supplied prints nothing", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		deps := serveCommandDeps{
			findRoot: func(string) (string, error) { return "/store", nil },
			readiness: readinessSnapshotBuilderFunc(func(context.Context, string, string) (string, *readinessload.PredecodedRequest, error) {
				t.Fatal("readiness builder called without --context-request")
				return "", nil, nil
			}),
			run: func(string, string, readinessload.Loader, string, io.Writer, io.Writer) int { return 0 },
		}
		if code := cmdServeWithDeps(nil, &stdout, &stderr, deps); code != 0 {
			t.Fatalf("exit = %d, stderr=%q", code, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want nothing without --context-request", stdout.String())
		}
	})

	t.Run("a failed warm-up prints no line", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		deps := serveCommandDeps{
			findRoot: func(string) (string, error) { return "/store", nil },
			readiness: readinessSnapshotBuilderFunc(func(context.Context, string, string) (string, *readinessload.PredecodedRequest, error) {
				return "", nil, errors.New("warm-up refused")
			}),
			run: func(string, string, readinessload.Loader, string, io.Writer, io.Writer) int {
				t.Fatal("run entered after a failed warm-up")
				return 0
			},
		}
		if code := cmdServeWithDeps([]string{"--context-request", "request.json"}, &stdout, &stderr, deps); code != 2 {
			t.Fatalf("exit = %d, want 2", code)
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want nothing after a failed warm-up", stdout.String())
		}
	})
}
