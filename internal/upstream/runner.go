package upstream

import "context"

// Request is one upstream CLI invocation, structured so flags-before-
// positional ordering (PLAN.md §3: spike S1's gotcha) cannot be gotten
// wrong by a caller: buildArgv always emits Subcommand, then every flag,
// then every positional argument, in that fixed order.
type Request struct {
	// Bin is the upstream binary: "flowmap" or "groundwork".
	Bin string
	// Subcommand is the leading token ("graph", "boundary", "review",
	// "diff", "version", ...).
	Subcommand string
	// Flags are already-formed "-name" or "-name=value" tokens, in the
	// order they should appear. Go's flag package accepts a single leading
	// dash, so this package uses that form throughout, matching upstream's
	// own --help text.
	Flags []string
	// Positional are the trailing positional arguments, in order.
	Positional []string
}

// buildArgv renders req into the exact argv upstream requires: subcommand,
// then flags, then positional arguments.
func (req Request) buildArgv() []string {
	argv := make([]string, 0, 1+len(req.Flags)+len(req.Positional))
	if req.Subcommand != "" {
		argv = append(argv, req.Subcommand)
	}
	argv = append(argv, req.Flags...)
	argv = append(argv, req.Positional...)
	return argv
}

// Result is one invocation's raw outcome.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// Runner execs one upstream CLI invocation. The real runner
// (RealRunner) execs `go run <module>/cmd/<bin>@<commit> <argv...>`
// (I-4); tests use FakeRunner to serve canned Results with no network and
// no exec at all (CLAUDE.md: "No network in any test").
//
// Run returns a non-nil error only for an exec-level failure (binary not
// found, context cancelled, I/O error) distinct from upstream's own exit
// code, which callers read from Result.ExitCode and interpret per the
// 0/1/2 contract (CLAUDE.md: verdict failure vs. operational error) — that
// interpretation is subcommand-specific and lives in exec.go, not here.
type Runner interface {
	Run(ctx context.Context, req Request) (Result, error)
}
