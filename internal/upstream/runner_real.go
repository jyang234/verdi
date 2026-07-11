package upstream

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

// RealRunner execs the pinned toolchain via `go run` (I-4: "exec via
// go run <module>/cmd/<bin>@<pseudo-version>"). Spike S1 found that
// `go run <module>@<full-sha>` resolves publicly via proxy.golang.org — the
// module proxy itself turns a full commit SHA into the matching
// pseudo-version, so RealRunner passes the pinned commit straight through
// rather than constructing a pseudo-version string by hand.
//
// RealRunner is never exercised by this module's tests (CLAUDE.md: "No
// network in any test" — `go run …@pin` needs a reachable proxy even with a
// warm cache, per PLAN.md I-4's CI note). It is covered by spike S1 and the
// opt-in, non-hermetic `make fixture-regen` target.
type RealRunner struct {
	// Module is the pinned toolchain module, e.g.
	// "github.com/jyang234/golang-code-graph" (verdi.yaml toolchain.module).
	Module string
	// Commit is the pinned commit SHA (verdi.yaml toolchain.commit).
	Commit string
	// Dir is the working directory the CLI runs in (typically a service
	// root).
	Dir string
	// Env, if non-nil, overrides the child process's environment
	// (e.g. to set GROUNDWORK_REQUIRE_STAMP=1 in CI per I-4). A nil Env
	// inherits the current process's environment via os.Environ().
	Env []string
}

// Run implements Runner.
func (r RealRunner) Run(ctx context.Context, req Request) (Result, error) {
	if r.Module == "" || r.Commit == "" {
		return Result{}, fmt.Errorf("upstream: RealRunner: module and commit must both be set (verdi.yaml toolchain: block, I-4)")
	}
	target := fmt.Sprintf("%s/cmd/%s@%s", r.Module, req.Bin, r.Commit)
	argv := append([]string{"run", target}, req.buildArgv()...)

	cmd := exec.CommandContext(ctx, "go", argv...)
	cmd.Dir = r.Dir
	if r.Env != nil {
		cmd.Env = r.Env
	} else {
		cmd.Env = os.Environ()
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	res := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}

	if runErr == nil {
		res.ExitCode = 0
		return res, nil
	}
	if exitErr, ok := runErr.(*exec.ExitError); ok {
		res.ExitCode = exitErr.ExitCode()
		return res, nil
	}
	// Not a clean exit-code failure (binary missing, context cancelled,
	// etc.) — an exec-level error, not an upstream verdict.
	return res, fmt.Errorf("upstream: RealRunner: go run %s: %w: %s", target, runErr, stderr.String())
}
