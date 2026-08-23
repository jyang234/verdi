// Package experimentevaluator runs the one strict CSE evaluator command
// protocol through an already-authorized execution-workspace profile.
package experimentevaluator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
)

const transportLimit = 1 << 20

// Launch is the immutable command identity and candidate-local working
// directory used for one describe or run operation.
type Launch struct {
	Directory string
	Argv      []string
	Digest    string
}

// DiscoverInput binds one describe operation to its registered capability
// document digest.
type DiscoverInput struct {
	Launch             Launch
	CapabilitiesDigest string
}

// Discovery is a strict capabilities document and the exact canonical bytes
// registration persists.
type Discovery struct {
	Capabilities experiment.Capabilities
	Bytes        []byte
}

// ObserveInput binds one evaluator run request to its authorized launch.
type ObserveInput struct {
	Launch  Launch
	Request experiment.EvaluatorRequest
}

// Attempt is the validated result of one zero-exit evaluator invocation.
// Observation is present only for measured cycles. ProcessMeasurements and
// ProcessDisclosures remain transient for warmups.
type Attempt struct {
	Outcome             experiment.CandidateOutcome
	Observation         *experiment.Observation
	ProcessMeasurements []experiment.Measurement
	ProcessDisclosures  []string
}

type commandBuilder interface {
	Command(context.Context, string, ...string) (*exec.Cmd, context.Context, context.CancelFunc, error)
}

type processRunner interface {
	Run(*exec.Cmd) (processState, error)
}

type osProcessRunner struct{}

func (osProcessRunner) Run(cmd *exec.Cmd) (processState, error) {
	err := cmd.Run()
	if cmd.ProcessState == nil {
		return nil, err
	}
	return cmd.ProcessState, err
}

type adapter struct {
	commands  commandBuilder
	processes processRunner
	now       func() time.Time
}

// Discover runs only the derived describe operation through Profile.Command.
func Discover(ctx context.Context, profile execworkspace.Profile, input DiscoverInput) (Discovery, error) {
	a := adapter{commands: profile, processes: osProcessRunner{}, now: time.Now}
	return a.discover(ctx, input)
}

// Observe runs one strict evaluator request through Profile.Command and, for
// measured cycles, constructs the harness-owned V2 observation envelope.
func Observe(ctx context.Context, profile execworkspace.Profile, input ObserveInput) (Attempt, error) {
	a := adapter{commands: profile, processes: osProcessRunner{}, now: time.Now}
	return a.observe(ctx, input)
}

func (a adapter) discover(ctx context.Context, input DiscoverInput) (Discovery, error) {
	if err := validateLaunch(input.Launch); err != nil {
		return Discovery{}, operational("describe input", ErrProtocol, nil, err)
	}
	if err := experiment.ValidateDigest(input.CapabilitiesDigest); err != nil {
		return Discovery{}, operational("describe input", ErrCapabilitiesDigest, nil, err)
	}
	argv := append([]string(nil), input.Launch.Argv...)
	argv[len(argv)-1] = "describe"
	stdout, _, _, err := a.execute(ctx, input.Launch, argv, nil, false)
	if err != nil {
		return Discovery{}, err
	}
	capabilities, err := experiment.DecodeCapabilities(stdout)
	if err != nil {
		return Discovery{}, operational("describe response", ErrProtocol, nil, err)
	}
	if capabilities.Schema != experiment.CapabilitiesSchemaV2 {
		return Discovery{}, operational("describe response", ErrProtocol, nil, fmt.Errorf("schema %q is not %q", capabilities.Schema, experiment.CapabilitiesSchemaV2))
	}
	if got := rawDigest(stdout); got != input.CapabilitiesDigest {
		return Discovery{}, operational("describe response", ErrCapabilitiesDigest, nil, fmt.Errorf("got %q, want %q", got, input.CapabilitiesDigest))
	}
	return Discovery{Capabilities: capabilities, Bytes: append([]byte(nil), stdout...)}, nil
}

func (a adapter) observe(ctx context.Context, input ObserveInput) (Attempt, error) {
	if err := validateLaunch(input.Launch); err != nil {
		return Attempt{}, operational("run input", ErrProtocol, nil, err)
	}
	requestBytes, err := experiment.EncodeEvaluatorRequest(input.Request)
	if err != nil {
		return Attempt{}, operational("run input", ErrProtocol, nil, err)
	}
	stdout, state, duration, err := a.execute(ctx, input.Launch, input.Launch.Argv, requestBytes, true)
	if err != nil {
		return Attempt{}, err
	}
	response, err := experiment.DecodeEvaluatorResponse(stdout)
	if err != nil {
		return Attempt{}, operational("run response", ErrProtocol, nil, err)
	}
	measurements, disclosures, err := processObservations(duration, state)
	if err != nil {
		return Attempt{}, operational("run observer", ErrObserver, nil, err)
	}
	attempt := Attempt{
		Outcome:             response.Outcome,
		ProcessMeasurements: append([]experiment.Measurement(nil), measurements...),
		ProcessDisclosures:  append([]string(nil), disclosures...),
	}
	if input.Request.Cycle.Kind == experiment.CycleWarmup {
		if err := ValidateAttempt(input, attempt); err != nil {
			return Attempt{}, operational("run attempt", ErrProtocol, nil, err)
		}
		return attempt, nil
	}
	guards := []experiment.GuardResult{}
	observationMeasurements := append([]experiment.Measurement{}, measurements...)
	observationDisclosures := append([]string{}, disclosures...)
	if response.Outcome.Kind == experiment.OutcomeCompleted {
		guards = append(guards, response.Guards...)
		observationMeasurements = append(append([]experiment.Measurement{}, response.Measurements...), measurements...)
		observationDisclosures = append(append([]string{}, response.Disclosures...), disclosures...)
	}
	observation := experiment.Observation{
		Schema:           experiment.ObservationSchemaV2,
		ExperimentDigest: input.Request.ExperimentDigest,
		Run:              input.Request.Run,
		Candidate:        input.Request.Candidate,
		Round:            input.Request.Cycle.Number,
		Outcome:          &response.Outcome,
		Guards:           guards,
		Measurements:     observationMeasurements,
		Disclosures:      observationDisclosures,
	}
	attempt.Observation = &observation
	if err := ValidateAttempt(input, attempt); err != nil {
		return Attempt{}, operational("run attempt", ErrProtocol, nil, err)
	}
	return attempt, nil
}

func validateLaunch(launch Launch) error {
	if launch.Directory == "" {
		return fmt.Errorf("working directory is empty")
	}
	if len(launch.Argv) < 2 || launch.Argv[0] == "" || launch.Argv[len(launch.Argv)-1] != "run" {
		return fmt.Errorf("evaluator argv must contain an executable and final literal run operation")
	}
	if err := experiment.ValidateDigest(launch.Digest); err != nil {
		return fmt.Errorf("evaluator digest: %w", err)
	}
	return nil
}

func (a adapter) execute(ctx context.Context, launch Launch, argv []string, stdin []byte, timed bool) ([]byte, processState, time.Duration, error) {
	cmd, runCtx, cancel, err := a.commands.Command(ctx, argv[0], argv[1:]...)
	if err != nil {
		return nil, nil, 0, operational("construct command", ErrLaunch, nil, err)
	}
	defer cancel()

	stdout := &boundedBuffer{limit: transportLimit}
	stderr := &boundedBuffer{limit: transportLimit}
	cmd.Dir = launch.Directory
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := verifyExecutable(cmd, launch.Digest); err != nil {
		return nil, nil, 0, operational("verify executable", ErrEvaluatorDigest, stderr.Bytes(), err)
	}

	start := time.Time{}
	if timed {
		start = a.now()
	}
	state, runErr := a.processes.Run(cmd)
	duration := time.Duration(0)
	if timed {
		duration = a.now().Sub(start)
	}
	if err := classifyProcess(ctx, runCtx, state, runErr, stdout, stderr); err != nil {
		return nil, state, 0, err
	}
	return stdout.Bytes(), state, duration, nil
}

func classifyProcess(ctx, runCtx context.Context, state processState, runErr error, stdout, stderr *boundedBuffer) error {
	if err := ctx.Err(); err != nil {
		return operational("run", ErrContextCancellation, stderr.Bytes(), err)
	}
	if err := runCtx.Err(); errors.Is(err, context.DeadlineExceeded) {
		return operational("run", ErrHarnessDeadline, stderr.Bytes(), err)
	} else if err != nil {
		return operational("run", ErrContextCancellation, stderr.Bytes(), err)
	}
	if stdout.Exceeded() {
		return operational("run", ErrStdoutLimit, stderr.Bytes(), nil)
	}
	if stderr.Exceeded() {
		return operational("run", ErrStderrLimit, stderr.Bytes(), nil)
	}
	if state != nil && !state.Success() {
		return operational("run", ErrEvaluatorExit, stderr.Bytes(), runErr)
	}
	if runErr != nil || state == nil {
		return operational("run", ErrLaunch, stderr.Bytes(), runErr)
	}
	return nil
}

func verifyExecutable(cmd *exec.Cmd, expectedDigest string) error {
	path := cmd.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(cmd.Dir, path)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("lstat %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("%q is not a non-symlink regular executable", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %q: %w", path, err)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("hash %q: %w", path, errors.Join(err, file.Close()))
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("finalize %q: %w", path, err)
	}
	got := "sha256:" + hex.EncodeToString(hash.Sum(nil))
	if got != expectedDigest {
		return fmt.Errorf("digest %q does not match registered %q", got, expectedDigest)
	}
	return nil
}

func rawDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

type boundedBuffer struct {
	buffer bytes.Buffer
	limit  int
	total  int64
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.total += int64(len(p))
	remaining := b.limit - b.buffer.Len()
	if remaining > len(p) {
		remaining = len(p)
	}
	if remaining > 0 {
		written, err := b.buffer.Write(p[:remaining])
		if err != nil {
			return 0, err
		}
		if written != remaining {
			return 0, io.ErrShortWrite
		}
	}
	return len(p), nil
}

func (b *boundedBuffer) Bytes() []byte {
	return append([]byte(nil), b.buffer.Bytes()...)
}

func (b *boundedBuffer) Exceeded() bool {
	return b.total > int64(b.limit)
}
