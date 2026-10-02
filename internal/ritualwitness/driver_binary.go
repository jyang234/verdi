package ritualwitness

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
	// carries the fixture's git configuration isolation.
	Env []string
}

// Run implements Driver. A non-zero exit returns its code with an error
// naming the binary's stdout and stderr. A binary that cannot be started,
// that ends without an exit code (a signal), or whose exit 2 is a Go
// panic's (goPanicTrace) returns -1, which is no verb's exit class, so
// RunOn refuses the run instead of judging it a refusal (ledger SI-334
// (4)).
func (d Binary) Run(ctx context.Context, dir string) (int, CommandLog, error) {
	if d.Path == "" {
		return -1, CommandLog{}, errors.New("ritualwitness: Binary: no binary path")
	}
	cmd := exec.CommandContext(ctx, d.Path, d.Args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), d.Env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return 0, CommandLog{}, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 && goPanicTrace(stderr.String()) {
		return -1, CommandLog{}, fmt.Errorf("ritualwitness: %s %s panicked, which is no verb's exit\nstderr: %s",
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
