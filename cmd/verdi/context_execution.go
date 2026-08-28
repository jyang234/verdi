package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"

	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/sealedexec"
	"github.com/jyang234/verdi/internal/sealedexec/codex"
	"github.com/jyang234/verdi/internal/store"
)

const contextExecutionUsage = "usage: verdi context execution --request <path|-> [--out <path>]"

func cmdContextExecution(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	requestPath, outPath, hasOut, ok := parseContextExecutionArgs(args)
	if !ok {
		fmt.Fprintln(stderr, contextExecutionUsage)
		return 2
	}
	request, err := readContextExecutionRequest(requestPath, stdin)
	if err != nil {
		fmt.Fprintln(stderr, "context execution:", err)
		return 2
	}
	controller, controllerConn, err := openSealedController()
	if err != nil {
		fmt.Fprintln(stderr, "context execution:", err)
		return 2
	}
	defer controllerConn.Close()
	ctx := context.Background()
	authority, err := controller.VerifyAuthority(ctx, request)
	if err != nil {
		printSealedContextDiagnostic(stderr, "execution", request, err)
		return contextExecutionExitCode(err)
	}
	if err := validateSealedAuthority(request, authority); err != nil {
		printSealedContextDiagnostic(stderr, "execution", request, err)
		return contextExecutionExitCode(err)
	}
	runtime, err := assembleSealedRuntime(ctx, request, controller, authority)
	if err != nil {
		printSealedContextDiagnostic(stderr, "execution", request, err)
		return contextExecutionExitCode(err)
	}
	signalWatcher := startContextExecutionSignalWatcher(osContextExecutionSignalNotifier{}, func() error {
		_, err := runtime.execution.InterruptRegistered(context.Background(), request)
		return err
	})
	defer signalWatcher.Stop()
	run, err := runtime.execution.Execute(ctx, request, runtime.data)
	if err != nil {
		if quarantineErr := runtime.quarantineIncomplete(ctx, request, run, err); quarantineErr != nil {
			printSealedContextDiagnostic(stderr, "execution", request, quarantineErr, runtime.root, run.Workspace.Path, run.Profile.CodexHome, run.Profile.Executable)
			return 2
		}
		printSealedContextDiagnostic(stderr, "execution", request, err, runtime.root, run.Workspace.Path, run.Profile.CodexHome, run.Profile.Executable)
		return contextExecutionExitCode(err)
	}
	completion, err := runtime.completion.Complete(ctx, sealedexec.CompletionRequest{Request: request, Run: run})
	if err != nil {
		if quarantineErr := runtime.quarantineTerminal(ctx, request, run); quarantineErr != nil {
			printSealedContextDiagnostic(stderr, "execution", request, quarantineErr, runtime.root, run.Workspace.Path, run.Profile.CodexHome, run.Profile.Executable)
			return 2
		}
		printSealedContextDiagnostic(stderr, "execution", request, err, runtime.root, run.Workspace.Path, run.Profile.CodexHome, run.Profile.Executable)
		return 2
	}
	preserved, err := finalizedExecution(completion.ResultBytes)
	if err != nil {
		printSealedContextDiagnostic(stderr, "execution", request, err, runtime.root, run.Workspace.Path, run.Profile.CodexHome, run.Profile.Executable)
		return 2
	}
	if err := writeSealedExecutionResult(stdout, outPath, hasOut, completion.ResultBytes); err != nil {
		outcome, quarantineErr := runtime.handback.Apply(ctx, sealedexec.HandbackRequest{
			Phase: sealedexec.HandbackPhaseOutputWriteFailed, Request: request, Run: run,
			Completion: &completion, Preserved: preserved,
		})
		if !sealedQuarantineDurable(outcome) {
			if quarantineErr == nil {
				quarantineErr = fmt.Errorf("%w: output-failure quarantine lacks its durable controller acknowledgment", sealedexec.ErrOperational)
			}
			printSealedContextDiagnostic(stderr, "execution", request, quarantineErr, runtime.root, run.Workspace.Path, run.Profile.CodexHome, run.Profile.Executable)
			return 2
		}
		printSealedContextDiagnostic(stderr, "execution", request, fmt.Errorf("writing public result: %w", err), runtime.root, run.Workspace.Path, run.Profile.CodexHome, run.Profile.Executable)
		return 2
	}
	outcome, err := runtime.handback.Apply(ctx, sealedexec.HandbackRequest{
		Phase: sealedexec.HandbackPhaseCompleted, Request: request, Run: run,
		Completion: &completion, Preserved: preserved,
	})
	if err != nil {
		printSealedContextDiagnostic(stderr, "execution", request, err, runtime.root, run.Workspace.Path, run.Profile.CodexHome, run.Profile.Executable)
		return outcome.ExitCode
	}
	return outcome.ExitCode
}

type contextExecutionSignalNotifier interface {
	Notify(chan<- os.Signal, ...os.Signal)
	Stop(chan<- os.Signal)
}

type osContextExecutionSignalNotifier struct{}

func (osContextExecutionSignalNotifier) Notify(channel chan<- os.Signal, signals ...os.Signal) {
	signal.Notify(channel, signals...)
}

func (osContextExecutionSignalNotifier) Stop(channel chan<- os.Signal) {
	signal.Stop(channel)
}

type contextExecutionSignalWatcher struct {
	notifier contextExecutionSignalNotifier
	signals  chan os.Signal
	stop     chan struct{}
	done     chan struct{}
	once     sync.Once
}

func startContextExecutionSignalWatcher(notifier contextExecutionSignalNotifier, interrupt func() error) *contextExecutionSignalWatcher {
	watcher := &contextExecutionSignalWatcher{
		notifier: notifier, signals: make(chan os.Signal, 1),
		stop: make(chan struct{}), done: make(chan struct{}),
	}
	notifier.Notify(watcher.signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer close(watcher.done)
		for {
			select {
			case <-watcher.signals:
				if err := interrupt(); !errors.Is(err, sealedexec.ErrInterruptNotActive) {
					return
				}
			case <-watcher.stop:
				return
			}
		}
	}()
	return watcher
}

func (w *contextExecutionSignalWatcher) Stop() {
	w.once.Do(func() {
		w.notifier.Stop(w.signals)
		close(w.stop)
	})
	<-w.done
}

func parseContextExecutionArgs(args []string) (requestPath, outPath string, hasOut, ok bool) {
	hasRequest := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--request":
			if hasRequest || i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "--") {
				return "", "", false, false
			}
			hasRequest = true
			requestPath = args[i+1]
			i++
		case "--out":
			if hasOut || i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "--") {
				return "", "", false, false
			}
			hasOut = true
			outPath = args[i+1]
			i++
		default:
			return "", "", false, false
		}
	}
	if !hasRequest || (hasOut && requestPath != "-" && sameFileArg(requestPath, outPath)) {
		return "", "", false, false
	}
	return requestPath, outPath, hasOut, true
}

func readContextExecutionRequest(requestPath string, stdin io.Reader) (sealedexec.ExecutionRequest, error) {
	var data []byte
	var err error
	if requestPath == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(requestPath)
	}
	if err != nil {
		return sealedexec.ExecutionRequest{}, fmt.Errorf("reading request: %w", err)
	}
	request, err := sealedexec.DecodeExecutionRequest(bytes.NewReader(data))
	if err != nil {
		return sealedexec.ExecutionRequest{}, fmt.Errorf("decoding request: %w", err)
	}
	return request, nil
}

func contextExecutionExitCode(err error) int {
	if errors.Is(err, sealedexec.ErrVerdict) {
		return 1
	}
	return 2
}

func openSealedController() (*sealedexec.ControllerClient, net.Conn, error) {
	var stat syscall.Stat_t
	if err := syscall.Fstat(3, &stat); err != nil {
		return nil, nil, fmt.Errorf("controller FD 3 is unavailable: %w", err)
	}
	if stat.Mode&syscall.S_IFMT != syscall.S_IFSOCK {
		return nil, nil, errors.New("controller FD 3 must be a connected bidirectional socket")
	}
	file := os.NewFile(uintptr(3), "verdi-controller")
	if file == nil {
		return nil, nil, errors.New("controller FD 3 is unavailable")
	}
	conn, err := net.FileConn(file)
	_ = file.Close()
	if err != nil {
		return nil, nil, fmt.Errorf("controller FD 3 must be a connected bidirectional socket: %w", err)
	}
	if conn.LocalAddr() == nil || conn.RemoteAddr() == nil {
		_ = conn.Close()
		return nil, nil, errors.New("controller FD 3 must be a connected bidirectional socket")
	}
	controller, err := sealedexec.NewControllerClient(conn)
	if err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("controller FD 3: %w", err)
	}
	return controller, conn, nil
}

func printSealedContextDiagnostic(stderr io.Writer, command string, request sealedexec.ExecutionRequest, err error, paths ...string) {
	message := redactSealedPathMessage(err.Error(), append([]string{request.ATCRunway}, paths...)...)
	fmt.Fprintf(stderr, "context %s: %s\n", command, message)
}

func redactSealedPathMessage(message string, paths ...string) string {
	for _, path := range paths {
		if path != "" && filepath.IsAbs(path) {
			message = strings.ReplaceAll(message, path, "<checkout>")
		}
	}
	return message
}

type sealedRuntime struct {
	root       string
	data       []contextcompile.DataItem
	execution  *sealedexec.Service
	completion *sealedexec.CompletionService
	handback   *sealedexec.HandbackService
}

func assembleSealedRuntime(ctx context.Context, request sealedexec.ExecutionRequest, controller *sealedexec.ControllerClient, authority sealedexec.AuthorityFacts) (sealedRuntime, error) {
	root, err := store.FindRoot(request.ATCRunway)
	if err != nil {
		return sealedRuntime{}, fmt.Errorf("derive store root from request runway: %w", err)
	}
	data, err := compileSealedContext(ctx, root, request)
	if err != nil {
		return sealedRuntime{}, err
	}
	materializer, err := execworkspace.NewMaterializer(root, root, execworkspace.NewGitReconciler(root))
	if err != nil {
		return sealedRuntime{}, fmt.Errorf("construct execution materializer: %w", err)
	}
	adapter, err := codex.New(commandCodexProcess{})
	if err != nil {
		return sealedRuntime{}, err
	}
	recorder := controllerRecorder{client: controller}
	workspace := localWorkspaceVerifier{root: root}
	execution, err := sealedexec.NewService(sealedexec.ServicePorts{
		Authority:    cachedAuthorityVerifier{request: request, facts: authority},
		Runway:       localRunwayVerifier{},
		Materializer: materializer,
		Workspace:    workspace,
		Profiles:     controllerProfileResolver{client: controller},
		Conflicts:    controller,
		Recorders:    controllerRecorderResolver{client: controller, recorder: recorder},
		Opaque:       controller,
		Adapter:      adapter,
		Sessions:     controller,
		Expansions:   controller,
		SessionStore: controller,
		Stamps:       controller,
	})
	if err != nil {
		return sealedRuntime{}, err
	}
	completion, err := sealedexec.NewCompletionService(sealedexec.CompletionPorts{
		Workspace: workspace, Recorder: recorder,
		Inputs: controller, Receipts: controller, Stamps: controller,
	})
	if err != nil {
		return sealedRuntime{}, err
	}
	handback, err := sealedexec.NewHandbackService(sealedexec.HandbackPorts{
		Repository: localHandbackRepository{}, Control: controller,
		Releaser: contextReleaser{releaser: execworkspace.NewReleaser(root)},
	})
	if err != nil {
		return sealedRuntime{}, err
	}
	return sealedRuntime{root: root, data: data, execution: execution, completion: completion, handback: handback}, nil
}

func compileSealedContext(ctx context.Context, root string, request sealedexec.ExecutionRequest) ([]contextcompile.DataItem, error) {
	compileRequest := contextcompile.Request{
		Schema: contextcompile.RequestSchema, Adapter: request.Manifest.Adapter,
		Grants: request.Grants, Phase: request.Manifest.Phase, Scope: request.Manifest.Scope,
		Spec: request.Manifest.AcceptedSpec.Ref,
	}
	compiled, err := contextcompile.NewCompiler().Compile(ctx, root, compileRequest)
	if err != nil {
		return nil, fmt.Errorf("compile sealed context: %w", err)
	}
	if !reflect.DeepEqual(compiled.Manifest, request.Manifest) || compiled.Manifest.Digest != request.ManifestDigest {
		return nil, fmt.Errorf("%w: compiled context manifest does not match the public request", sealedexec.ErrVerdict)
	}
	projection := sealedexec.InstructionProjection{Schema: sealedexec.InstructionProjectionSchemaID, Files: make([]sealedexec.InstructionFile, len(compiled.ProjectionFiles))}
	for i, file := range compiled.ProjectionFiles {
		projection.Files[i] = sealedexec.InstructionFile{Path: file.Path, ContentDigest: file.Digest, Content: string(file.Content)}
	}
	projectionBytes, err := sealedexec.EncodeInstructionProjection(projection)
	if err != nil {
		return nil, fmt.Errorf("encode compiled instruction projection: %w", err)
	}
	canonicalProjection, err := sealedexec.DecodeInstructionProjection(bytes.NewReader(projectionBytes))
	if err != nil {
		return nil, fmt.Errorf("round-trip compiled instruction projection: %w", err)
	}
	if !reflect.DeepEqual(canonicalProjection, request.InstructionProjection) || canonicalProjection.Digest != request.ProjectionDigest {
		return nil, fmt.Errorf("%w: compiled instruction projection does not match the public request", sealedexec.ErrVerdict)
	}
	return append([]contextcompile.DataItem(nil), compiled.DataItems...), nil
}

func writeSealedExecutionResult(stdout io.Writer, outPath string, hasOut bool, data []byte) error {
	if hasOut {
		return atomicfile.Write(outPath, data, 0o644)
	}
	_, err := stdout.Write(data)
	return err
}

func finalizedExecution(data []byte) (sealedexec.PreservedExecution, error) {
	preserved, err := sealedexec.PreservedExecutionForBytes(sealedexec.PreservedFinalized, data)
	if err != nil {
		return sealedexec.PreservedExecution{}, fmt.Errorf("%w: identify finalized execution: %v", sealedexec.ErrOperational, err)
	}
	return preserved, nil
}

func (runtime sealedRuntime) quarantineIncomplete(ctx context.Context, request sealedexec.ExecutionRequest, run sealedexec.ExecutionRun, cause error) error {
	if run.Workspace.WorkspaceID == "" {
		return nil
	}
	phase := sealedexec.HandbackPhaseExecutionIncompleteOperational
	if errors.Is(cause, sealedexec.ErrVerdict) {
		phase = sealedexec.HandbackPhaseExecutionIncompleteVerdict
	}
	partial, err := sealedexec.EncodeExecutionPartial(request, run)
	if err != nil {
		return fmt.Errorf("%w: encode incomplete execution preservation: %v", sealedexec.ErrOperational, err)
	}
	preserved, err := sealedexec.PreservedExecutionForBytes(sealedexec.PreservedPartial, partial)
	if err != nil {
		return fmt.Errorf("%w: identify incomplete execution preservation: %v", sealedexec.ErrOperational, err)
	}
	outcome, err := runtime.handback.Apply(ctx, sealedexec.HandbackRequest{
		Phase: phase, Request: request, Run: run,
		PartialBytes: partial, Preserved: preserved,
	})
	if sealedQuarantineDurable(outcome) {
		return nil
	}
	if err == nil {
		return fmt.Errorf("%w: incomplete-execution quarantine lacks its durable controller acknowledgment", sealedexec.ErrOperational)
	}
	return err
}

func (runtime sealedRuntime) quarantineTerminal(ctx context.Context, request sealedexec.ExecutionRequest, run sealedexec.ExecutionRun) error {
	partial, err := sealedexec.EncodeExecutionPartial(request, run)
	if err != nil {
		return fmt.Errorf("%w: encode terminal execution preservation: %v", sealedexec.ErrOperational, err)
	}
	preserved, err := sealedexec.PreservedExecutionForBytes(sealedexec.PreservedPartial, partial)
	if err != nil {
		return fmt.Errorf("%w: identify terminal execution preservation: %v", sealedexec.ErrOperational, err)
	}
	outcome, err := runtime.handback.Apply(ctx, sealedexec.HandbackRequest{
		Phase: sealedexec.HandbackPhaseTerminalDurabilityFailed, Request: request, Run: run,
		PartialBytes: partial, Preserved: preserved,
	})
	if sealedQuarantineDurable(outcome) {
		return nil
	}
	if err == nil {
		return fmt.Errorf("%w: terminal-durability quarantine lacks its durable controller acknowledgment", sealedexec.ErrOperational)
	}
	return err
}

func sealedQuarantineDurable(outcome sealedexec.HandbackOutcome) bool {
	return outcome.Quarantine != nil && outcome.ControlAck.Digest != ""
}

type cachedAuthorityVerifier struct {
	request sealedexec.ExecutionRequest
	facts   sealedexec.AuthorityFacts
}

func (v cachedAuthorityVerifier) VerifyAuthority(_ context.Context, request sealedexec.ExecutionRequest) (sealedexec.AuthorityFacts, error) {
	if !reflect.DeepEqual(request, v.request) {
		return sealedexec.AuthorityFacts{}, fmt.Errorf("%w: authority request changed after FD-3 verification", sealedexec.ErrVerdict)
	}
	return v.facts, nil
}

type localRunwayVerifier struct{}

func (localRunwayVerifier) VerifyRunway(ctx context.Context, path string) (sealedexec.RunwayFacts, error) {
	commit, err := gitx.RevParse(ctx, path, "HEAD")
	if err != nil {
		return sealedexec.RunwayFacts{}, err
	}
	tree, err := gitx.RevParse(ctx, path, "HEAD^{tree}")
	if err != nil {
		return sealedexec.RunwayFacts{}, err
	}
	dirty, err := gitx.StatusDirty(ctx, path)
	if err != nil {
		return sealedexec.RunwayFacts{}, err
	}
	return sealedexec.RunwayFacts{
		Verification: sealedexec.Verification{State: contextcompile.ResolutionProven, Witnesses: []string{}},
		Path:         path, Commit: commit, Tree: tree, Clean: !dirty,
	}, nil
}

type localWorkspaceVerifier struct{ root string }

func (v localWorkspaceVerifier) VerifyWorkspace(ctx context.Context, path string, identity execworkspace.Identity) (sealedexec.WorkspaceFacts, error) {
	workspaceID, err := identity.WorkspaceID()
	if err != nil {
		return sealedexec.WorkspaceFacts{}, err
	}
	sidecarBytes, err := os.ReadFile(execworkspace.RequestPath(v.root, workspaceID))
	if err != nil {
		return sealedexec.WorkspaceFacts{}, err
	}
	sidecar, err := execworkspace.DecodeSidecar(sidecarBytes)
	if err != nil {
		return sealedexec.WorkspaceFacts{}, err
	}
	if !sidecar.Equal(identity) {
		return sealedexec.WorkspaceFacts{}, fmt.Errorf("workspace sidecar identity mismatch")
	}
	digest, err := sealedexec.ExecutionWorkspaceRequestDigest(identity)
	if err != nil {
		return sealedexec.WorkspaceFacts{}, err
	}
	commit, err := gitx.RevParse(ctx, path, "HEAD")
	if err != nil {
		return sealedexec.WorkspaceFacts{}, err
	}
	tree, err := gitx.RevParse(ctx, path, "HEAD^{tree}")
	if err != nil {
		return sealedexec.WorkspaceFacts{}, err
	}
	dirty, err := gitx.StatusDirty(ctx, path)
	if err != nil {
		return sealedexec.WorkspaceFacts{}, err
	}
	return sealedexec.WorkspaceFacts{
		Verification: sealedexec.Verification{State: contextcompile.ResolutionProven, Witnesses: []string{}},
		WorkspaceID:  workspaceID, Path: path, Request: sidecar, RequestDigest: digest,
		CurrentCommit: commit, CurrentTree: tree, Clean: !dirty,
	}, nil
}

type controllerProfileResolver struct{ client *sealedexec.ControllerClient }

func (r controllerProfileResolver) ResolveProfile(ctx context.Context, ref sealedexec.LogicalRef, workspacePath string, grants execworkspace.GrantSet) (sealedexec.ResolvedProfile, error) {
	material, err := r.client.ResolveProfile(ctx, sealedexec.ProfileQuery{Ref: ref, WorkspacePath: workspacePath, Grants: grants})
	if err != nil {
		return sealedexec.ResolvedProfile{}, err
	}
	if material.Ref != ref || !filepath.IsAbs(material.AbsoluteExecutable) || !filepath.IsAbs(material.AbsoluteEnvRoot) || !filepath.IsAbs(material.AbsoluteCodexHome) {
		return sealedexec.ResolvedProfile{}, fmt.Errorf("%w: resolved profile material contradicts request or contains a relative path", sealedexec.ErrVerdict)
	}
	profile, enforcement, err := execworkspace.BuildProfile(workspacePath, material.AbsoluteEnvRoot, grants, map[string]string{"CODEX_HOME": material.AbsoluteCodexHome})
	if err != nil {
		message := redactSealedPathMessage(err.Error(), workspacePath, material.AbsoluteExecutable, material.AbsoluteEnvRoot, filepath.Dir(material.AbsoluteEnvRoot), material.AbsoluteCodexHome)
		return sealedexec.ResolvedProfile{}, errors.New(message)
	}
	return sealedexec.ResolvedProfile{
		Verification: sealedexec.Verification{State: contextcompile.ResolutionProven, Witnesses: []string{}},
		Ref:          ref, Digest: ref.Digest, Name: material.Name, Executable: material.AbsoluteExecutable,
		CodexHome: material.AbsoluteCodexHome, AdapterVersion: material.AdapterVersion,
		DecoderProfile: material.DecoderProfile, WorkspacePath: workspacePath,
		Profile: profile, Grants: grants, Enforcement: *enforcement,
	}, nil
}

type controllerRecorder struct{ client *sealedexec.ControllerClient }

func (r controllerRecorder) Append(ctx context.Context, event contextevent.Event) (contextevent.EventAck, error) {
	return r.client.RecorderAppend(ctx, event)
}

func (r controllerRecorder) Checkpoint(ctx context.Context, key sealedexec.ExecutionKey) (sealedexec.RecorderCheckpoint, error) {
	return r.client.RecorderCheckpoint(ctx, key)
}

type controllerRecorderResolver struct {
	client   *sealedexec.ControllerClient
	recorder controllerRecorder
}

func (r controllerRecorderResolver) ResolveRecorder(ctx context.Context, ref sealedexec.LogicalRef) (sealedexec.RecorderFacts, sealedexec.Recorder, error) {
	facts, err := r.client.ResolveRecorder(ctx, ref)
	if err != nil {
		return sealedexec.RecorderFacts{}, nil, err
	}
	return facts, r.recorder, nil
}

type localHandbackRepository struct{}

func (localHandbackRepository) Observe(ctx context.Context, path string) (sealedexec.RepositoryState, error) {
	commit, err := gitx.RevParse(ctx, path, "HEAD")
	if err != nil {
		return sealedexec.RepositoryState{}, err
	}
	tree, err := gitx.RevParse(ctx, path, "HEAD^{tree}")
	if err != nil {
		return sealedexec.RepositoryState{}, err
	}
	dirty, err := gitx.StatusDirty(ctx, path)
	if err != nil {
		return sealedexec.RepositoryState{}, err
	}
	return sealedexec.RepositoryState{Path: path, Commit: commit, Tree: tree, Clean: !dirty}, nil
}

func (localHandbackRepository) IsAncestor(ctx context.Context, dir, ancestor, commit string) (bool, error) {
	return gitx.IsAncestor(ctx, dir, ancestor, commit)
}

func (localHandbackRepository) Diff(ctx context.Context, dir, base, head string) ([]gitx.DiffEntry, error) {
	return gitx.DiffNameStatusCopies(ctx, dir, base, head)
}

func (localHandbackRepository) FastForwardOnly(ctx context.Context, runway, outputCommit string) (gitx.FastForwardResult, error) {
	return gitx.FastForwardOnly(ctx, runway, outputCommit)
}

type contextReleaser struct{ releaser *execworkspace.Releaser }

func (r contextReleaser) Release(_ context.Context, workspaceID string) error {
	return r.releaser.Release(workspaceID)
}

type commandCodexProcess struct{}

func (commandCodexProcess) Start(_ context.Context, command *exec.Cmd, stdin []byte) (codex.ActiveProcess, error) {
	if command == nil {
		return nil, errors.New("sealed codex process: nil command")
	}
	if len(command.ExtraFiles) != 0 {
		return nil, errors.New("sealed codex process: provider child must not inherit extra files")
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	command.Stdin = bytes.NewReader(stdin)
	command.Stderr = io.Discard
	if err := command.Start(); err != nil {
		message := strings.ReplaceAll(err.Error(), command.Path, "<profile-executable>")
		return nil, errors.New(message)
	}
	active := &commandCodexRun{command: command, frames: make(chan commandCodexFrame)}
	go active.pump(stdout)
	return active, nil
}

type commandCodexFrame struct {
	observation codex.ProcessObservation
	err         error
}

type commandCodexRun struct {
	command  *exec.Cmd
	frames   chan commandCodexFrame
	stopOnce sync.Once
}

func (run *commandCodexRun) pump(stdout io.Reader) {
	defer close(run.frames)
	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) != 0 {
			complete := err == nil
			if complete {
				line = bytes.TrimSuffix(line, []byte{'\n'})
			}
			run.frames <- commandCodexFrame{observation: codex.ProcessObservation{ForeignJSON: line, Complete: complete}}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				run.frames <- commandCodexFrame{err: err}
			}
			break
		}
	}
	waitErr := run.command.Wait()
	exitCode := 0
	if waitErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(waitErr, &exitErr) {
			run.frames <- commandCodexFrame{err: waitErr}
			return
		}
		exitCode = exitErr.ExitCode()
	}
	run.frames <- commandCodexFrame{observation: codex.ProcessObservation{Terminal: &codex.ProcessResult{ExitCode: exitCode}}}
}

func (run *commandCodexRun) Next(ctx context.Context) (codex.ProcessObservation, error) {
	select {
	case <-ctx.Done():
		return codex.ProcessObservation{}, ctx.Err()
	case frame, ok := <-run.frames:
		if !ok {
			return codex.ProcessObservation{}, io.EOF
		}
		return frame.observation, frame.err
	}
}

func (run *commandCodexRun) Stop(context.Context) (codex.ProcessStopResult, error) {
	var stopErr error
	run.stopOnce.Do(func() {
		stopErr = run.command.Process.Kill()
	})
	if stopErr != nil && !errors.Is(stopErr, os.ErrProcessDone) {
		return codex.ProcessStopResult{}, stopErr
	}
	return codex.ProcessStopResult{ExitCode: 130, ReasonCode: "interrupt-requested"}, nil
}
