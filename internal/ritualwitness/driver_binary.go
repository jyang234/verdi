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
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/gitx"
)

// Binary is the built-binary Driver (spec/ritual-effect-witness dc-1):
// it runs a built verdi binary as a subprocess, in the fixture directory,
// with Args, and reports the process's exit code as the verb's exit
// classification.
//
// The binary roots its own context (parent dc-10), so no gitx.Observer a
// test attaches reaches it. Its command log comes instead from the file
// VERDI_GITLOG names (gitx.GitLogEnv; spec/gitx-recorder-seam ac-2, dc-2;
// ledger SI-359 (1), (3)): Run gives each run its own empty file there,
// stripping any value the test process carries, and reads it back once
// the binary has exited. Every line must be one whole record that decodes
// strictly, with an argv, an absolute directory, and a pid; Run keeps the
// records of the process it started, in order, and reports the log with
// CommandLog.OK true.
//
// A log Run cannot read whole fails closed: when the file is gone or any
// line in it is not such a record, Run reports the log unavailable
// (CommandLog.OK false), never fewer calls, so Evaluate reads every effect
// only the log could attribute as unattributable. A non-zero exit's error
// also names why; a clean exit carries no error, so its run shows the
// failure only as that unattributable command log. A run that ends in no
// verb's exit (-1) carries no log.
//
// Disclosed, not proven: the pid filter drops the records of any process
// the binary starts that inherits VERDI_GITLOG, a verdi re-exec among them.
// No verb starts one today; if one ever does, its git calls vanish from
// this log, from attribution, and from the forbidden-token witness (SI-359
// (15)). Git that a child program runs itself is outside the log too
// (SI-359 (4b)).
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
	// Stdin is the binary's standard input; nil is the null device. An
	// *os.File is handed to the binary as is. Any other reader is copied
	// into the binary through a pipe the driver owns, and the copy ends at
	// the reader's EOF or when the binary stops reading. When the binary
	// exits 0 before the copy ends, Run waits for it until the context ends
	// or binaryWaitDelay passes, then abandons it and returns -1, naming
	// the standard input that never reached EOF. On any other end of the
	// run, Run abandons the copy at once. An abandoned copy stays blocked
	// in the reader's Read until that returns, so a caller ends a reader it
	// owns (closes an io.Pipe's writer, say) once Run returns.
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
// CI-context variable no field pins in a string literal; its doc lists the
// reads the scan cannot see (ledger SI-344 (2)).
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
// started can hold stdout and stderr open after the binary is gone. It
// also bounds, after a clean exit, the wait for the copy of a Stdin reader
// to end (stdinFeed).
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

// PinCIEnv sets, for the rest of t, every variable CIEnv names in the test
// process's own environment to ci's value for it, empty ones included, as
// Binary sets them for the binary. It uses t.Setenv, so neither t nor an
// ancestor may run in parallel. Binary's CI field reaches only the binary
// it starts, while an in-process driver (MCP, Workbench, InProcess) runs
// code that reads the test process's environment, the MCP server's
// default-branch read of CI_DEFAULT_BRANCH among it: a case driving one
// calls PinCIEnv so that code sees the same CI context locally and in CI
// (ledger SI-344 (2); R3ab review R3-B4).
func PinCIEnv(t testing.TB, ci CIEnv) {
	t.Helper()
	for _, kv := range ci.pairs() {
		key, value, _ := strings.Cut(kv, "=")
		t.Setenv(key, value)
	}
}

// environ is the binary's environment: the test process's own without the
// pinned CI variables or VERDI_GITLOG, GOTRACEBACK=single, Env, VERDI_GITLOG
// naming gitLog, and the CI field's values. Env may set neither a pinned
// CI variable nor VERDI_GITLOG.
func (d Binary) environ(gitLog string) ([]string, error) {
	pinned := ciEnvKeys()
	key := func(kv string) string {
		k, _, _ := strings.Cut(kv, "=")
		return k
	}
	for _, kv := range d.Env {
		if k := key(kv); slices.Contains(pinned, k) {
			return nil, fmt.Errorf("ritualwitness: Binary: Env sets %s; set it through the CI field, which every run pins", k)
		}
		if k := key(kv); k == gitx.GitLogEnv {
			return nil, fmt.Errorf("ritualwitness: Binary: Env sets %s; every run sets it to the run's own command log", k)
		}
	}
	var env []string
	for _, kv := range os.Environ() {
		if k := key(kv); !slices.Contains(pinned, k) && k != gitx.GitLogEnv {
			env = append(env, kv)
		}
	}
	env = append(env, "GOTRACEBACK=single")
	env = append(env, d.Env...)
	env = append(env, gitx.GitLogEnv+"="+gitLog)
	return append(env, d.CI.pairs()...), nil
}

// newGitLog creates a run's own empty command log file and returns its
// path. Created before the binary starts, it exists whether or not the
// binary runs git, so its absence afterwards is a failure, never a run
// without git.
func newGitLog() (string, error) {
	f, err := os.CreateTemp("", "verdi-gitlog-*.jsonl")
	if err != nil {
		return "", fmt.Errorf("ritualwitness: Binary: creating the command log: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("ritualwitness: Binary: creating the command log: %w", err)
	}
	return f.Name(), nil
}

// readGitLog reads the command log at path and returns the calls of the
// process pid, in order. It refuses the whole log when the file cannot be
// read, when it does not end in a newline (a record cut short), or when
// any line, whichever pid it names, is not one strictly decoded record
// with an argv, an absolute directory, and a positive pid.
func readGitLog(path string, pid int) ([]Call, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ritualwitness: Binary: reading the command log: %w", err)
	}
	if len(data) == 0 {
		return []Call{}, nil
	}
	if data[len(data)-1] != '\n' {
		return nil, errors.New("ritualwitness: Binary: the command log's last record is cut short (no newline)")
	}
	calls := []Call{}
	for i, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		var rec gitx.GitLogRecord
		if err := decodeStrict([]byte(line), &rec); err != nil {
			return nil, fmt.Errorf("ritualwitness: Binary: command log line %d %q: %w", i+1, line, err)
		}
		if len(rec.Args) == 0 || !filepath.IsAbs(rec.Dir) || rec.PID <= 0 {
			return nil, fmt.Errorf("ritualwitness: Binary: command log line %d %q: want an argv, an absolute directory, and a pid", i+1, line)
		}
		if rec.PID == pid {
			calls = append(calls, Call{Dir: rec.Dir, Args: rec.Args})
		}
	}
	return calls, nil
}

// commandLog is the log a run that exited with a code reports: the calls
// of process pid in the command log at path, or, when that log cannot be
// read whole, an unavailable log and the reason.
func commandLog(path string, pid int) (CommandLog, error) {
	calls, err := readGitLog(path, pid)
	if err != nil {
		return CommandLog{}, err
	}
	return CommandLog{Calls: calls, OK: true}, nil
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
// output returns -1 once binaryWaitDelay passes; a binary that exits 0
// while its Stdin has not reached EOF returns -1 once the context ends or
// binaryWaitDelay passes (Stdin); and a binary that outlives its context
// is killed, its output closed within that delay, and returns -1 naming
// the context. None of them hangs the run. An Env that sets VERDI_GITLOG
// runs nothing and returns -1. Every run that ends in an exit code returns
// the binary's command log (Binary), or the log unavailable when it cannot
// be read whole.
func (d Binary) Run(ctx context.Context, dir string) (int, CommandLog, error) {
	if d.Path == "" {
		return -1, CommandLog{}, errors.New("ritualwitness: Binary: no binary path")
	}
	logPath, err := newGitLog()
	if err != nil {
		return -1, CommandLog{}, err
	}
	defer func() { _ = os.Remove(logPath) }()
	env, err := d.environ(logPath)
	if err != nil {
		return -1, CommandLog{}, err
	}
	stdin, feed, err := stdinFor(d.Stdin)
	if err != nil {
		return -1, CommandLog{}, err
	}
	cmd := exec.CommandContext(ctx, d.Path, d.Args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = stdin
	cmd.ExtraFiles = d.ExtraFiles
	cmd.WaitDelay = binaryWaitDelay
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err = cmd.Start(); err != nil {
		feed.abandon()
		return -1, CommandLog{}, fmt.Errorf("ritualwitness: running %s: %w", d.Path, err)
	}
	feed.start(d.Stdin)
	pid := cmd.Process.Pid
	err = cmd.Wait()
	if err != nil {
		feed.abandon()
	} else if stdinErr := feed.wait(ctx); stdinErr != nil {
		return -1, CommandLog{}, fmt.Errorf("ritualwitness: %s %s exited 0, but %w, which is no verb's exit",
			filepath.Base(d.Path), strings.Join(d.Args, " "), stdinErr)
	}
	if err == nil {
		log, _ := commandLog(logPath, pid)
		return 0, log, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 && goCrash(stderr.String()) {
		return -1, CommandLog{}, fmt.Errorf("ritualwitness: %s %s crashed (panicked, or a runtime fatal error), which is no verb's exit\nstderr: %s",
			filepath.Base(d.Path), strings.Join(d.Args, " "), strings.TrimSpace(stderr.String()))
	}
	if errors.As(err, &exitErr) && exitErr.ExitCode() >= 0 {
		verbErr := fmt.Errorf("ritualwitness: %s %s exited %d\nstdout: %s\nstderr: %s",
			filepath.Base(d.Path), strings.Join(d.Args, " "), exitErr.ExitCode(),
			strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()))
		log, logErr := commandLog(logPath, pid)
		return exitErr.ExitCode(), log, errors.Join(verbErr, logErr)
	}
	if errors.Is(err, exec.ErrWaitDelay) {
		return -1, CommandLog{}, fmt.Errorf("ritualwitness: %s %s exited but left its output open past %s (a process it started still holds it), which is no verb's exit: %w",
			filepath.Base(d.Path), strings.Join(d.Args, " "), binaryWaitDelay, err)
	}
	if ctx.Err() != nil {
		return -1, CommandLog{}, fmt.Errorf("ritualwitness: %s %s did not exit before its context ended (%w), so the driver killed it, which is no verb's exit: %w",
			filepath.Base(d.Path), strings.Join(d.Args, " "), ctx.Err(), err)
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
