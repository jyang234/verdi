package ritualwitness

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Binary is the built-binary Driver (spec/ritual-effect-witness dc-1):
// it runs a built verdi binary as a subprocess, in the fixture directory,
// with Args, and reports the process's exit code as the verb's exit
// classification.
//
// The binary roots its own context (parent dc-10), so no gitx.Observer a
// test attaches reaches it, and until spec/gitx-recorder-seam's
// VERDI_GITLOG hook lands it records no command log: Run reports the log
// unavailable (CommandLog.OK false), never an empty log inferred from
// silence, and Evaluate reads every effect only the log could attribute as
// unattributable.
type Binary struct {
	// Path is the built binary.
	Path string
	// Args are the verb and its arguments, after the binary's name.
	Args []string
	// Env is extra environment, appended to the test process's own, which
	// carries the fixture's git configuration isolation, and to
	// GOTRACEBACK=single, which Env may override. It may not set a
	// variable the CI field pins (ciEnvKeys).
	Env []string
	// CI is the continuous-integration environment the binary runs under.
	// Every run sets CI, GITHUB_ACTIONS, and GITHUB_BASE_REF to exactly
	// these values, empty ones included, and never inherits them from the
	// test process, so a verb that reads them (close's publish guard, for
	// one) behaves the same locally and in CI. The zero value is a run
	// outside CI.
	CI CIEnv
	// Stdin is the binary's standard input; nil is the null device.
	Stdin io.Reader
	// ExtraFiles are open files the binary inherits as file descriptors 3
	// and up, in order (os/exec's Cmd.ExtraFiles), for a verb that reads
	// one, such as context execution's controller socket. The caller owns
	// and closes them.
	ExtraFiles []*os.File
}

// CIEnv is the CI environment a Binary run sees, one field per variable.
type CIEnv struct {
	// CI is the generic "running in CI" marker.
	CI string
	// GitHubActions is GITHUB_ACTIONS, GitHub Actions' own marker.
	GitHubActions string
	// GitHubBaseRef is GITHUB_BASE_REF, a pull request's target branch.
	GitHubBaseRef string
}

// ciEnvKeys are the variables a Binary run takes from its CI field.
var ciEnvKeys = []string{"CI", "GITHUB_ACTIONS", "GITHUB_BASE_REF"}

// pairs returns c as KEY=value pairs, in ciEnvKeys' order.
func (c CIEnv) pairs() []string {
	return []string{"CI=" + c.CI, "GITHUB_ACTIONS=" + c.GitHubActions, "GITHUB_BASE_REF=" + c.GitHubBaseRef}
}

// ciEnvKey returns the CI variable kv (a KEY=value pair) sets, or "".
func ciEnvKey(kv string) string {
	key, _, _ := strings.Cut(kv, "=")
	for _, k := range ciEnvKeys {
		if key == k {
			return k
		}
	}
	return ""
}

// environ is the binary's environment: the test process's own without the
// CI variables, GOTRACEBACK=single, Env, and the CI field's values.
func (d Binary) environ() ([]string, error) {
	for _, kv := range d.Env {
		if k := ciEnvKey(kv); k != "" {
			return nil, fmt.Errorf("ritualwitness: Binary: Env sets %s; set it through the CI field, which every run pins", k)
		}
	}
	var env []string
	for _, kv := range os.Environ() {
		if ciEnvKey(kv) == "" {
			env = append(env, kv)
		}
	}
	env = append(env, "GOTRACEBACK=single")
	env = append(env, d.Env...)
	return append(env, d.CI.pairs()...), nil
}

// Run implements Driver. A non-zero exit returns its code with an error
// naming the binary's stdout and stderr. A binary that cannot be started,
// that ends without an exit code (a signal), or whose exit 2 is a Go
// runtime crash (goCrash: a panic trace or a fatal error) returns -1,
// which is no verb's exit class, so RunOn refuses the run instead of
// judging it a refusal (ledger SI-334 (4)). The child runs with
// GOTRACEBACK=single, so an ambient setting cannot hide or reshape the
// trace the detection reads; the fixture's own Env still wins. An Env
// that sets a variable the CI field pins runs nothing and returns -1.
func (d Binary) Run(ctx context.Context, dir string) (int, CommandLog, error) {
	if d.Path == "" {
		return -1, CommandLog{}, errors.New("ritualwitness: Binary: no binary path")
	}
	env, err := d.environ()
	if err != nil {
		return -1, CommandLog{}, err
	}
	cmd := exec.CommandContext(ctx, d.Path, d.Args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = d.Stdin
	cmd.ExtraFiles = d.ExtraFiles
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err == nil {
		return 0, CommandLog{}, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 && goCrash(stderr.String()) {
		return -1, CommandLog{}, fmt.Errorf("ritualwitness: %s %s crashed (panicked, or a runtime fatal error), which is no verb's exit\nstderr: %s",
			filepath.Base(d.Path), strings.Join(d.Args, " "), strings.TrimSpace(stderr.String()))
	}
	if errors.As(err, &exitErr) && exitErr.ExitCode() >= 0 {
		return exitErr.ExitCode(), CommandLog{}, fmt.Errorf("ritualwitness: %s %s exited %d\nstdout: %s\nstderr: %s",
			filepath.Base(d.Path), strings.Join(d.Args, " "), exitErr.ExitCode(),
			strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()))
	}
	return -1, CommandLog{}, fmt.Errorf("ritualwitness: running %s: %w", d.Path, err)
}

// panicTrace matches the Go runtime's report of an unrecovered panic: a
// line beginning "panic: ", then, on a later line, the first goroutine's
// stack header ("goroutine 1 [running]:").
var panicTrace = regexp.MustCompile(`(?ms)^panic: .*^goroutine \d+ \[[^\]\n]*\]:$`)

// goPanicTrace reports whether stderr carries a Go panic trace. The runtime
// exits 2 after one, the same code a verb's operational refusal uses, so
// the trace, not the code, tells the two apart.
func goPanicTrace(stderr string) bool {
	return panicTrace.MatchString(stderr)
}

// fatalError matches the Go runtime's report of a fatal error (a deadlock,
// concurrent map writes): a line beginning "fatal error: ".
var fatalError = regexp.MustCompile(`(?m)^fatal error: `)

// goCrash reports whether stderr carries a Go runtime crash: a panic trace
// or a fatal error, after either of which the runtime exits 2 (re-review
// RR-B1).
func goCrash(stderr string) bool {
	return goPanicTrace(stderr) || fatalError.MatchString(stderr)
}
