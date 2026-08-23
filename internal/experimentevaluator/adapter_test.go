package experimentevaluator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/experiment"
)

const canonicalCapabilities = "{\"evaluator_version\":\"fixture-evaluator/1\",\"metrics\":[],\"protocol_versions\":[\"verdi.experiment-evaluator/v1\",\"verdi.experiment-observation/v2\"],\"requires_elevated\":false,\"requires_network\":false,\"schema\":\"verdi.experiment-evaluator-capabilities/v2\"}\n"

const canonicalCompletedResponse = "{\"disclosures\":[],\"guards\":[],\"measurements\":[],\"outcome\":{\"kind\":\"completed\"},\"schema\":\"verdi.experiment-evaluator/v1\"}\n"

type fakeProcessState struct {
	success bool
	usage   any
}

func (s fakeProcessState) Success() bool { return s.success }
func (s fakeProcessState) SysUsage() any { return s.usage }

type fakeCommandBuilder struct {
	command       *exec.Cmd
	err           error
	runCtx        context.Context
	cancelCalls   int
	gotArgv0      string
	gotArgs       []string
	commandEnv    []string
	commandSys    *syscall.SysProcAttr
	commandExtras []*os.File
}

func (f *fakeCommandBuilder) Command(ctx context.Context, argv0 string, args ...string) (*exec.Cmd, context.Context, context.CancelFunc, error) {
	f.gotArgv0 = argv0
	f.gotArgs = append([]string(nil), args...)
	if f.err != nil {
		return nil, nil, nil, f.err
	}
	runCtx := f.runCtx
	if runCtx == nil {
		runCtx = ctx
	}
	f.command = &exec.Cmd{
		Path:        argv0,
		Args:        append([]string{argv0}, args...),
		Env:         append([]string(nil), f.commandEnv...),
		SysProcAttr: f.commandSys,
		ExtraFiles:  append([]*os.File(nil), f.commandExtras...),
	}
	return f.command, runCtx, func() { f.cancelCalls++ }, nil
}

type fakeProcessRunner struct {
	stdout []byte
	stderr []byte
	state  processState
	err    error
	runs   int
	stdin  []byte
	check  func(*exec.Cmd) error
}

func (f *fakeProcessRunner) Run(cmd *exec.Cmd) (processState, error) {
	f.runs++
	if f.check != nil {
		if err := f.check(cmd); err != nil {
			return f.state, err
		}
	}
	if cmd.Stdin != nil {
		data, err := io.ReadAll(cmd.Stdin)
		if err != nil {
			return f.state, err
		}
		f.stdin = data
	}
	if len(f.stdout) > 0 {
		if _, err := cmd.Stdout.Write(f.stdout); err != nil {
			return f.state, err
		}
	}
	if len(f.stderr) > 0 {
		if _, err := cmd.Stderr.Write(f.stderr); err != nil {
			return f.state, err
		}
	}
	return f.state, f.err
}

type sequenceClock struct {
	times []time.Time
	next  int
}

func (c *sequenceClock) Now() time.Time {
	t := c.times[c.next]
	c.next++
	return t
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func executableFixture(t *testing.T) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture-evaluator")
	data := []byte("fixture executable bytes\n")
	if err := os.WriteFile(path, data, 0o700); err != nil {
		t.Fatalf("write executable fixture: %v", err)
	}
	return path, digestBytes(data)
}

func validProtocolRequest(kind experiment.CycleKind) experiment.EvaluatorRequest {
	return experiment.EvaluatorRequest{
		Schema:           experiment.EvaluatorProtocolSchema,
		ExperimentDigest: "sha256:" + strings.Repeat("a", 64),
		Run:              "run-1",
		Candidate:        "candidate-a",
		Cycle:            experiment.EvaluatorCycle{Kind: kind, Number: 1},
		Workload:         experiment.ResolvedArtifact{ID: "workload-a", Path: "workloads/a.json", Digest: "sha256:" + strings.Repeat("b", 64)},
		Fixtures:         []experiment.ResolvedArtifact{{ID: "fixture-a", Path: "fixtures/a.json", Digest: "sha256:" + strings.Repeat("d", 64)}},
		Contract:         experiment.ResolvedArtifact{ID: "contract-a", Path: "contracts/a.json", Digest: "sha256:" + strings.Repeat("c", 64)},
	}
}

func testAdapter(t *testing.T, stdout []byte) (adapter, *fakeCommandBuilder, *fakeProcessRunner, Launch) {
	t.Helper()
	executable, digest := executableFixture(t)
	sys := &syscall.SysProcAttr{}
	commands := &fakeCommandBuilder{
		commandEnv:    []string{"HOME=/sealed/home", "PATH=/sealed/bin"},
		commandSys:    sys,
		commandExtras: []*os.File{nil},
	}
	processes := &fakeProcessRunner{stdout: stdout, state: fakeProcessState{success: true}}
	start := time.Unix(100, 0)
	clock := &sequenceClock{times: []time.Time{start, start.Add(2500 * time.Microsecond)}}
	return adapter{commands: commands, processes: processes, now: clock.Now}, commands, processes, Launch{
		Directory: t.TempDir(),
		Argv:      []string{executable, "--config", "two words", "run"},
		Digest:    digest,
	}
}

func TestDiscoverReplacesOnlyOperationAndPreservesProfileCommand(t *testing.T) {
	a, commands, processes, launch := testAdapter(t, []byte(canonicalCapabilities))
	originalArgv := append([]string(nil), launch.Argv...)
	wantEnv := append([]string(nil), commands.commandEnv...)
	wantSys := commands.commandSys
	wantExtras := append([]*os.File(nil), commands.commandExtras...)
	processes.check = func(cmd *exec.Cmd) error {
		if cmd.Path != launch.Argv[0] || !reflect.DeepEqual(cmd.Args, []string{launch.Argv[0], "--config", "two words", "describe"}) {
			return errors.New("adapter changed Profile.Command path or args")
		}
		if !reflect.DeepEqual(cmd.Env, wantEnv) || cmd.SysProcAttr != wantSys || !reflect.DeepEqual(cmd.ExtraFiles, wantExtras) {
			return errors.New("adapter changed Profile.Command environment, SysProcAttr, or ExtraFiles")
		}
		if cmd.Dir != launch.Directory || cmd.Stdin == nil || cmd.Stdout == nil || cmd.Stderr == nil {
			return errors.New("adapter did not set exactly the consumer-owned command fields")
		}
		return nil
	}

	discovery, err := a.discover(context.Background(), DiscoverInput{
		Launch:             launch,
		CapabilitiesDigest: digestBytes([]byte(canonicalCapabilities)),
	})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if commands.gotArgv0 != launch.Argv[0] || !reflect.DeepEqual(commands.gotArgs, []string{"--config", "two words", "describe"}) {
		t.Fatalf("Command argv = %q %#v, want only final token replaced", commands.gotArgv0, commands.gotArgs)
	}
	if !reflect.DeepEqual(launch.Argv, originalArgv) {
		t.Fatalf("input argv mutated: got %#v, want %#v", launch.Argv, originalArgv)
	}
	if len(processes.stdin) != 0 {
		t.Fatalf("describe stdin = %q, want exact empty input", processes.stdin)
	}
	if string(discovery.Bytes) != canonicalCapabilities || discovery.Capabilities.Schema != experiment.CapabilitiesSchemaV2 {
		t.Fatalf("discovery = %+v bytes %q", discovery.Capabilities, discovery.Bytes)
	}
	if commands.cancelCalls != 1 {
		t.Fatalf("derived cancel calls = %d, want 1", commands.cancelCalls)
	}
}

func TestDiscoverRejectsDigestAndCanonicalityFailures(t *testing.T) {
	tests := []struct {
		name       string
		stdout     []byte
		capsDigest string
		want       error
	}{
		{name: "capabilities digest mismatch", stdout: []byte(canonicalCapabilities), capsDigest: "sha256:" + strings.Repeat("0", 64), want: ErrCapabilitiesDigest},
		{name: "noncanonical capabilities", stdout: []byte(strings.Replace(canonicalCapabilities, "{", "{ ", 1)), want: ErrProtocol},
		{name: "missing response", stdout: nil, want: ErrProtocol},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, _, _, launch := testAdapter(t, tt.stdout)
			digest := tt.capsDigest
			if digest == "" {
				digest = digestBytes(tt.stdout)
			}
			_, err := a.discover(context.Background(), DiscoverInput{Launch: launch, CapabilitiesDigest: digest})
			if !errors.Is(err, tt.want) {
				t.Fatalf("discover error = %v, want errors.Is(_, %v)", err, tt.want)
			}
		})
	}
}

func TestLaunchGrammarIsRejectedBeforeCommandConstruction(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Launch)
	}{
		{name: "empty directory", mutate: func(launch *Launch) { launch.Directory = "" }},
		{name: "missing operation", mutate: func(launch *Launch) { launch.Argv = launch.Argv[:1] }},
		{name: "operation is not final run", mutate: func(launch *Launch) { launch.Argv[len(launch.Argv)-1] = "describe" }},
		{name: "malformed executable digest", mutate: func(launch *Launch) { launch.Digest = "sha256:nope" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, commands, processes, launch := testAdapter(t, []byte(canonicalCapabilities))
			tt.mutate(&launch)
			_, err := a.discover(context.Background(), DiscoverInput{
				Launch:             launch,
				CapabilitiesDigest: digestBytes([]byte(canonicalCapabilities)),
			})
			if !errors.Is(err, ErrProtocol) {
				t.Fatalf("discover error = %v, want ErrProtocol", err)
			}
			if processes.runs != 0 || commands.command != nil {
				t.Fatalf("invalid launch constructed or ran a command: command=%v runs=%d", commands.command, processes.runs)
			}
		})
	}
}

func TestProfileCommandRefusalIsOperational(t *testing.T) {
	a, commands, processes, launch := testAdapter(t, []byte(canonicalCompletedResponse))
	commands.err = errors.New("required isolation control unavailable")
	_, err := a.observe(context.Background(), ObserveInput{Launch: launch, Request: validProtocolRequest(experiment.CycleMeasured), ResponseLimit: HardResponseBytes})
	if !errors.Is(err, ErrLaunch) || !IsOperational(err) {
		t.Fatalf("observe error = %v, want operational ErrLaunch", err)
	}
	if processes.runs != 0 || commands.cancelCalls != 0 {
		t.Fatalf("refused command ran or canceled an absent derived context: runs=%d cancels=%d", processes.runs, commands.cancelCalls)
	}
}

func TestObserveSendsExactCanonicalRequestAndBuildsMeasuredObservation(t *testing.T) {
	a, commands, processes, launch := testAdapter(t, []byte(canonicalCompletedResponse))
	request := validProtocolRequest(experiment.CycleMeasured)
	wantStdin := "{\"candidate\":\"candidate-a\",\"contract\":{\"digest\":\"sha256:" + strings.Repeat("c", 64) + "\",\"id\":\"contract-a\",\"path\":\"contracts/a.json\"},\"cycle\":{\"kind\":\"measured\",\"number\":1},\"experiment_digest\":\"sha256:" + strings.Repeat("a", 64) + "\",\"fixtures\":[{\"digest\":\"sha256:" + strings.Repeat("d", 64) + "\",\"id\":\"fixture-a\",\"path\":\"fixtures/a.json\"}],\"run\":\"run-1\",\"schema\":\"verdi.experiment-evaluator/v1\",\"workload\":{\"digest\":\"sha256:" + strings.Repeat("b", 64) + "\",\"id\":\"workload-a\",\"path\":\"workloads/a.json\"}}\n"

	attempt, err := a.observe(context.Background(), ObserveInput{Launch: launch, Request: request, ResponseLimit: HardResponseBytes})
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if string(processes.stdin) != wantStdin {
		t.Fatalf("run stdin:\n got %q\nwant %q", processes.stdin, wantStdin)
	}
	if commands.gotArgv0 != launch.Argv[0] || !reflect.DeepEqual(commands.gotArgs, []string{"--config", "two words", "run"}) {
		t.Fatalf("run Command argv = %q %#v, want registered argv unchanged", commands.gotArgv0, commands.gotArgs)
	}
	if attempt.Observation == nil {
		t.Fatal("measured attempt has nil observation")
	}
	observation := attempt.Observation
	if observation.Schema != experiment.ObservationSchemaV2 || observation.ExperimentDigest != request.ExperimentDigest || observation.Run != request.Run || observation.Candidate != request.Candidate || observation.Round != request.Cycle.Number {
		t.Fatalf("observation identity = %+v, want request-owned identity", observation)
	}
	if len(attempt.ProcessMeasurements) == 0 || attempt.ProcessMeasurements[0].ID != experiment.EvaluatorWallDurationMetricID || attempt.ProcessMeasurements[0].Value.String() != "2500000" {
		t.Fatalf("process measurements = %+v, want 2500000ns duration first", attempt.ProcessMeasurements)
	}
	if err := observation.Validate(); err != nil {
		t.Fatalf("observation Validate: %v", err)
	}
	if commands.cancelCalls != 1 {
		t.Fatalf("derived cancel calls = %d, want 1", commands.cancelCalls)
	}
}

func TestObservationLimitRejectsRawResponseAboveEffectiveCeiling(t *testing.T) {
	response := []byte(canonicalCompletedResponse)
	tests := []struct {
		name    string
		limit   int64
		wantErr bool
	}{
		{name: "below limit", limit: int64(len(response) + 1)},
		{name: "at limit", limit: int64(len(response))},
		{name: "above limit", limit: int64(len(response) - 1), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, _, _, launch := testAdapter(t, response)
			_, err := a.observe(context.Background(), ObserveInput{
				Launch: launch, Request: validProtocolRequest(experiment.CycleMeasured), ResponseLimit: tt.limit,
			})
			if tt.wantErr {
				if !errors.Is(err, ErrStdoutLimit) {
					t.Fatalf("observe() error = %v, want ErrStdoutLimit", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("observe() error = %v", err)
			}
		})
	}

	t.Run("nonpositive limit refuses before launch", func(t *testing.T) {
		a, _, processes, launch := testAdapter(t, response)
		_, err := a.observe(context.Background(), ObserveInput{
			Launch: launch, Request: validProtocolRequest(experiment.CycleMeasured), ResponseLimit: 0,
		})
		if err == nil || !errors.Is(err, ErrProtocol) {
			t.Fatalf("observe() error = %v, want protocol refusal", err)
		}
		if processes.runs != 0 {
			t.Fatalf("invalid limit launched evaluator %d times", processes.runs)
		}
	})

	t.Run("larger policy limit cannot weaken hard ceiling", func(t *testing.T) {
		oversized := bytes.Repeat([]byte{'x'}, int(HardResponseBytes)+1)
		a, _, _, launch := testAdapter(t, oversized)
		_, err := a.observe(context.Background(), ObserveInput{
			Launch: launch, Request: validProtocolRequest(experiment.CycleMeasured), ResponseLimit: HardResponseBytes * 2,
		})
		if !errors.Is(err, ErrStdoutLimit) {
			t.Fatalf("observe() error = %v, want hard-ceiling ErrStdoutLimit", err)
		}
	})
}

func TestObservePreservesClosedCandidateOutcomesAndHarnessFacts(t *testing.T) {
	witness := "candidate process exited unexpectedly"
	responses := []struct {
		name string
		kind experiment.OutcomeKind
	}{
		{name: "candidate crash", kind: experiment.OutcomeCandidateCrash},
		{name: "candidate timeout", kind: experiment.OutcomeCandidateTimeout},
	}
	for _, tt := range responses {
		t.Run(tt.name, func(t *testing.T) {
			response := "{\"disclosures\":[\"evaluator saw candidate exit\"],\"guards\":[],\"measurements\":[],\"outcome\":{\"kind\":\"" + string(tt.kind) + "\",\"witness\":\"" + witness + "\"},\"schema\":\"verdi.experiment-evaluator/v1\"}\n"
			a, _, _, launch := testAdapter(t, []byte(response))
			attempt, err := a.observe(context.Background(), ObserveInput{Launch: launch, Request: validProtocolRequest(experiment.CycleMeasured), ResponseLimit: HardResponseBytes})
			if err != nil {
				t.Fatalf("observe: %v", err)
			}
			if attempt.Outcome.Kind != tt.kind || attempt.Outcome.Witness == nil || *attempt.Outcome.Witness != witness {
				t.Fatalf("outcome = %+v", attempt.Outcome)
			}
			if attempt.Observation == nil || len(attempt.Observation.Guards) != 0 {
				t.Fatalf("failure observation = %+v, want present with no guards", attempt.Observation)
			}
			if len(attempt.Observation.Measurements) != len(attempt.ProcessMeasurements) || len(attempt.ProcessMeasurements) == 0 {
				t.Fatalf("failure measurements = %+v, process = %+v", attempt.Observation.Measurements, attempt.ProcessMeasurements)
			}
			if !reflect.DeepEqual(attempt.Observation.Disclosures, attempt.ProcessDisclosures) {
				t.Fatalf("failure disclosures = %+v, want only fixed process disclosures %+v", attempt.Observation.Disclosures, attempt.ProcessDisclosures)
			}
			if err := attempt.Observation.Validate(); err != nil {
				t.Fatalf("failure observation Validate: %v", err)
			}
		})
	}
}

func TestObserveWarmupFailureKeepsOnlyTransientProcessFacts(t *testing.T) {
	response := []byte("{\"disclosures\":[],\"guards\":[],\"measurements\":[],\"outcome\":{\"kind\":\"candidate-timeout\",\"witness\":\"workload timed out\"},\"schema\":\"verdi.experiment-evaluator/v1\"}\n")
	a, _, _, launch := testAdapter(t, response)
	attempt, err := a.observe(context.Background(), ObserveInput{Launch: launch, Request: validProtocolRequest(experiment.CycleWarmup), ResponseLimit: HardResponseBytes})
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if attempt.Observation != nil {
		t.Fatalf("warmup observation = %+v, want nil", attempt.Observation)
	}
	if len(attempt.ProcessMeasurements) == 0 || attempt.Outcome.Kind != experiment.OutcomeCandidateTimeout {
		t.Fatalf("warmup attempt = %+v, want transient process facts and timeout outcome", attempt)
	}
}

func TestObserveRejectsEvaluatorTrustBoundaryClaims(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "harness source",
			raw:  "{\"disclosures\":[],\"guards\":[],\"measurements\":[{\"id\":\"latency\",\"source\":\"harness-measured\",\"unit\":\"ns\",\"value\":1}],\"outcome\":{\"kind\":\"completed\"},\"schema\":\"verdi.experiment-evaluator/v1\"}\n",
		},
		{
			name: "reserved duration id",
			raw:  "{\"disclosures\":[],\"guards\":[],\"measurements\":[{\"id\":\"verdi-evaluator-wall-duration\",\"source\":\"evaluator-measured\",\"unit\":\"ns\",\"value\":1}],\"outcome\":{\"kind\":\"completed\"},\"schema\":\"verdi.experiment-evaluator/v1\"}\n",
		},
		{
			name: "reserved rss disclosure",
			raw:  "{\"disclosures\":[\"peak-rss-unavailable\"],\"guards\":[],\"measurements\":[],\"outcome\":{\"kind\":\"completed\"},\"schema\":\"verdi.experiment-evaluator/v1\"}\n",
		},
		{
			name: "candidate failure with evaluator measurement",
			raw:  "{\"disclosures\":[],\"guards\":[],\"measurements\":[{\"id\":\"latency\",\"source\":\"evaluator-measured\",\"unit\":\"ns\",\"value\":1}],\"outcome\":{\"kind\":\"candidate-crash\",\"witness\":\"crashed\"},\"schema\":\"verdi.experiment-evaluator/v1\"}\n",
		},
		{
			name: "noncanonical response",
			raw:  "{ \"disclosures\":[],\"guards\":[],\"measurements\":[],\"outcome\":{\"kind\":\"completed\"},\"schema\":\"verdi.experiment-evaluator/v1\"}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, _, _, launch := testAdapter(t, []byte(tt.raw))
			_, err := a.observe(context.Background(), ObserveInput{Launch: launch, Request: validProtocolRequest(experiment.CycleMeasured), ResponseLimit: HardResponseBytes})
			if !errors.Is(err, ErrProtocol) {
				t.Fatalf("observe error = %v, want ErrProtocol", err)
			}
		})
	}
}

func TestTransportBoundsAreExactAndOperational(t *testing.T) {
	const limit = 1 << 20
	tests := []struct {
		name   string
		stdout []byte
		stderr []byte
		want   error
	}{
		{name: "stderr exactly one MiB is admitted", stdout: []byte(canonicalCompletedResponse), stderr: bytes.Repeat([]byte("e"), limit)},
		{name: "stdout over one MiB", stdout: bytes.Repeat([]byte("o"), limit+1), want: ErrStdoutLimit},
		{name: "stderr over one MiB", stdout: []byte(canonicalCompletedResponse), stderr: bytes.Repeat([]byte("e"), limit+1), want: ErrStderrLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, _, processes, launch := testAdapter(t, tt.stdout)
			processes.stderr = tt.stderr
			_, err := a.observe(context.Background(), ObserveInput{Launch: launch, Request: validProtocolRequest(experiment.CycleMeasured), ResponseLimit: HardResponseBytes})
			if tt.want == nil {
				if err != nil {
					t.Fatalf("observe: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("observe error = %v, want errors.Is(_, %v)", err, tt.want)
			}
		})
	}
}

func TestProcessFailuresRemainOperational(t *testing.T) {
	tests := []struct {
		name      string
		parentCtx func() context.Context
		runCtx    func() context.Context
		state     processState
		runErr    error
		stdout    []byte
		want      error
		wantCause error
	}{
		{
			name:      "nonzero evaluator exit",
			parentCtx: context.Background,
			state:     fakeProcessState{success: false},
			runErr:    errors.New("exit status 7"),
			stdout:    []byte("{\"disclosures\":[],\"guards\":[],\"measurements\":[],\"outcome\":{\"kind\":\"candidate-crash\",\"witness\":\"child reported crash\"},\"schema\":\"verdi.experiment-evaluator/v1\"}\n"),
			want:      ErrEvaluatorExit,
		},
		{
			name:      "harness deadline",
			parentCtx: context.Background,
			runCtx: func() context.Context {
				ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
				defer cancel()
				return ctx
			},
			state:     fakeProcessState{success: false},
			runErr:    context.DeadlineExceeded,
			want:      ErrHarnessDeadline,
			wantCause: context.DeadlineExceeded,
		},
		{
			name: "caller cancellation",
			parentCtx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			state:     fakeProcessState{success: false},
			runErr:    context.Canceled,
			want:      ErrContextCancellation,
			wantCause: context.Canceled,
		},
		{
			name:      "process start failure",
			parentCtx: context.Background,
			runErr:    errors.New("start failed"),
			want:      ErrLaunch,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := tt.stdout
			if stdout == nil {
				stdout = []byte(canonicalCompletedResponse)
			}
			a, commands, processes, launch := testAdapter(t, stdout)
			if tt.runCtx != nil {
				commands.runCtx = tt.runCtx()
			}
			processes.state = tt.state
			processes.err = tt.runErr
			_, err := a.observe(tt.parentCtx(), ObserveInput{Launch: launch, Request: validProtocolRequest(experiment.CycleMeasured), ResponseLimit: HardResponseBytes})
			if !errors.Is(err, tt.want) {
				t.Fatalf("observe error = %v, want errors.Is(_, %v)", err, tt.want)
			}
			if tt.wantCause != nil && !errors.Is(err, tt.wantCause) {
				t.Fatalf("observe error = %v, want underlying %v", err, tt.wantCause)
			}
			if commands.cancelCalls != 1 {
				t.Fatalf("derived cancel calls = %d, want 1", commands.cancelCalls)
			}
		})
	}
}

func TestEvaluatorExecutableTrustBoundary(t *testing.T) {
	t.Run("digest mismatch", func(t *testing.T) {
		a, commands, processes, launch := testAdapter(t, []byte(canonicalCompletedResponse))
		launch.Digest = "sha256:" + strings.Repeat("0", 64)
		_, err := a.observe(context.Background(), ObserveInput{Launch: launch, Request: validProtocolRequest(experiment.CycleMeasured), ResponseLimit: HardResponseBytes})
		if !errors.Is(err, ErrEvaluatorDigest) {
			t.Fatalf("observe error = %v, want ErrEvaluatorDigest", err)
		}
		if processes.runs != 0 {
			t.Fatalf("process runs = %d, want 0 before integrity proof", processes.runs)
		}
		if commands.cancelCalls != 1 {
			t.Fatalf("derived cancel calls = %d, want 1", commands.cancelCalls)
		}
	})

	t.Run("symlink executable", func(t *testing.T) {
		target, digest := executableFixture(t)
		link := filepath.Join(t.TempDir(), "evaluator-link")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("symlink fixture: %v", err)
		}
		commands := &fakeCommandBuilder{}
		processes := &fakeProcessRunner{stdout: []byte(canonicalCompletedResponse), state: fakeProcessState{success: true}}
		start := time.Unix(100, 0)
		clock := &sequenceClock{times: []time.Time{start, start.Add(time.Millisecond)}}
		a := adapter{commands: commands, processes: processes, now: clock.Now}
		launch := Launch{Directory: t.TempDir(), Argv: []string{link, "run"}, Digest: digest}
		_, err := a.observe(context.Background(), ObserveInput{Launch: launch, Request: validProtocolRequest(experiment.CycleMeasured), ResponseLimit: HardResponseBytes})
		if !errors.Is(err, ErrEvaluatorDigest) {
			t.Fatalf("observe error = %v, want ErrEvaluatorDigest", err)
		}
		if processes.runs != 0 {
			t.Fatalf("process runs = %d, want 0 for symlink", processes.runs)
		}
	})
}
