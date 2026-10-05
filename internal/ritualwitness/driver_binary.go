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
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"
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
	// variable the CI field pins (CIEnv).
	Env []string
	// CI is the continuous-integration environment the binary runs under.
	// Every run sets each variable CIEnv names to exactly its field's
	// value, empty ones included, and never inherits one from the test
	// process, so a verb that reads one (close's publish guard, base
	// resolution, the forge and CI-ref facts) behaves the same locally and
	// in CI. The zero value is a run outside CI.
	CI CIEnv
	// Stdin is the binary's standard input; nil is the null device.
	Stdin io.Reader
	// ExtraFiles are open files the binary inherits as file descriptors 3
	// and up, in order (os/exec's Cmd.ExtraFiles), for a verb that reads
	// one, such as context execution's controller socket. The caller owns
	// and closes them.
	ExtraFiles []*os.File
}

// CIEnv is the CI environment a Binary run sees: one field per CI-context
// variable the verdi binary's code reads, its env tag naming the variable.
// TestCIEnv_PinsEveryCIVariableTheBinaryReads fails when that code names a
// CI-context variable no field pins.
type CIEnv struct {
	// The generic markers and refs: internal/lint's ReadCIEnv,
	// internal/specstate's default branch.
	CI                             string `env:"CI"`
	CIDefaultBranch                string `env:"CI_DEFAULT_BRANCH"`
	CIMergeRequestTargetBranchName string `env:"CI_MERGE_REQUEST_TARGET_BRANCH_NAME"`

	// GitHub Actions: its marker, the run's refs and identity, and the
	// forge's repository, endpoint, and token.
	GitHubActions         string `env:"GITHUB_ACTIONS"`
	GitHubBaseRef         string `env:"GITHUB_BASE_REF"`
	GitHubHeadRef         string `env:"GITHUB_HEAD_REF"`
	GitHubRefName         string `env:"GITHUB_REF_NAME"`
	GitHubRefType         string `env:"GITHUB_REF_TYPE"`
	GitHubEventName       string `env:"GITHUB_EVENT_NAME"`
	GitHubJob             string `env:"GITHUB_JOB"`
	GitHubRunID           string `env:"GITHUB_RUN_ID"`
	GitHubRunAttempt      string `env:"GITHUB_RUN_ATTEMPT"`
	GitHubSHA             string `env:"GITHUB_SHA"`
	GitHubWorkflowRef     string `env:"GITHUB_WORKFLOW_REF"`
	GitHubRepository      string `env:"GITHUB_REPOSITORY"`
	GitHubRepositoryOwner string `env:"GITHUB_REPOSITORY_OWNER"`
	GitHubServerURL       string `env:"GITHUB_SERVER_URL"`
	GitHubAPIURL          string `env:"GITHUB_API_URL"`
	GitHubToken           string `env:"GITHUB_TOKEN"`

	// GitLab CI: its marker, the pipeline's refs and identity, and the
	// forge's project, endpoint, and token.
	GitLabCI          string `env:"GITLAB_CI"`
	CICommitBranch    string `env:"CI_COMMIT_BRANCH"`
	CICommitRefName   string `env:"CI_COMMIT_REF_NAME"`
	CICommitTag       string `env:"CI_COMMIT_TAG"`
	CIMergeRequestIID string `env:"CI_MERGE_REQUEST_IID"`
	CIPipelineID      string `env:"CI_PIPELINE_ID"`
	CIPipelineURL     string `env:"CI_PIPELINE_URL"`
	CIJobID           string `env:"CI_JOB_ID"`
	CIJobName         string `env:"CI_JOB_NAME"`
	CIProjectID       string `env:"CI_PROJECT_ID"`
	CIProjectURL      string `env:"CI_PROJECT_URL"`
	CIAPIV4URL        string `env:"CI_API_V4_URL"`
	CIJobToken        string `env:"CI_JOB_TOKEN"`
}

// binaryWaitDelay bounds how long Run waits, after the binary exits or its
// context ends, for the binary's output to close: a process the binary
// started can hold stdout and stderr open after the binary is gone.
const binaryWaitDelay = 5 * time.Second

// ciEnvKeys returns every variable CIEnv pins, in field order.
func ciEnvKeys() []string {
	t := reflect.TypeFor[CIEnv]()
	keys := make([]string, t.NumField())
	for i := range keys {
		keys[i] = t.Field(i).Tag.Get("env")
	}
	return keys
}

// pairs returns c as KEY=value pairs, one per variable it pins.
func (c CIEnv) pairs() []string {
	v := reflect.ValueOf(c)
	keys := ciEnvKeys()
	pairs := make([]string, len(keys))
	for i, key := range keys {
		pairs[i] = key + "=" + v.Field(i).String()
	}
	return pairs
}

// environ is the binary's environment: the test process's own without the
// pinned CI variables, GOTRACEBACK=single, Env, and the CI field's values.
func (d Binary) environ() ([]string, error) {
	pinned := ciEnvKeys()
	key := func(kv string) string {
		k, _, _ := strings.Cut(kv, "=")
		return k
	}
	for _, kv := range d.Env {
		if k := key(kv); slices.Contains(pinned, k) {
			return nil, fmt.Errorf("ritualwitness: Binary: Env sets %s; set it through the CI field, which every run pins", k)
		}
	}
	var env []string
	for _, kv := range os.Environ() {
		if !slices.Contains(pinned, key(kv)) {
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
// that sets a variable the CI field pins runs nothing and returns -1. A
// binary that exits cleanly while a process it started still holds its
// output returns -1 once binaryWaitDelay passes, and a binary that
// outlives its context is killed and its output closed within that delay
// too, so neither hangs the run.
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
	cmd.WaitDelay = binaryWaitDelay
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
	if errors.Is(err, exec.ErrWaitDelay) {
		return -1, CommandLog{}, fmt.Errorf("ritualwitness: %s %s exited but left its output open past %s (a process it started still holds it), which is no verb's exit: %w",
			filepath.Base(d.Path), strings.Join(d.Args, " "), binaryWaitDelay, err)
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
