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
	"github.com/jyang234/verdi/internal/mcpserve"
	"github.com/jyang234/verdi/internal/sealedexec"
	"github.com/jyang234/verdi/internal/sealedexec/claude"
	"github.com/jyang234/verdi/internal/sealedexec/codex"
	"github.com/jyang234/verdi/internal/store"
)

const contextExecutionUsage = "usage: verdi context execution --request <path|-> [--out <path>]"

// sealedClaudeToolPath is the fixed deterministic tool path activated for the
// Claude arm (Amendment 002 §3). It never inherits the caller's PATH.
const sealedClaudeToolPath = "/usr/bin:/bin"

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
	// Provider reap: close the adapter lifecycle (HTTP MCP server close + config
	// removal for Claude; no-op for Codex). Resource order: provider reap first,
	// then adapter close, then receipt/completion. Errors are operational-only.
	if runtime.closer != nil {
		if closeErr := runtime.closer(ctx); closeErr != nil {
			printSealedContextDiagnostic(stderr, "execution", request, closeErr, runtime.root)
		}
	}
	// A typed scoped-MCP terminal ends the run: exit 1 is a verdict, exit 2 is
	// operational, and neither reaches completion or a receipt.
	err = sealedTerminalOutcome(err, runtime.terminals.observed())
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
	// closer is called after provider reap to release the adapter lifecycle
	// (for Claude: HTTP MCP server close + config removal). nil for Codex.
	closer func(context.Context) error
	// terminals records the first typed scoped-MCP terminal of the run. nil for
	// Codex, whose scoped surface is a separate process with its own exit code.
	terminals *sealedMCPTerminalObserver
}

// sealedMCPTerminalObserver consumes the first typed terminal the parent-hosted
// scoped MCP surface raises while Execute is still live. The terminal is
// delivered only after its JSON-RPC frame was written, so a failed response
// write never signals one. Observing it ends the run through the same
// interruption seam SIGTERM uses, and its exit code classifies the run.
type sealedMCPTerminalObserver struct {
	mu        sync.Mutex
	terminal  *mcpserve.HandlerTerminal
	interrupt func() error
}

// bind installs the live interruption seam. It is called once, after the
// execution service exists and before Execute starts.
func (o *sealedMCPTerminalObserver) bind(interrupt func() error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.interrupt = interrupt
}

// observe records the first terminal and asks the live run to stop. Later
// terminals are ignored: the run ends on the first one.
func (o *sealedMCPTerminalObserver) observe(terminal *mcpserve.HandlerTerminal) {
	if terminal == nil {
		return
	}
	o.mu.Lock()
	if o.terminal != nil {
		o.mu.Unlock()
		return
	}
	first := *terminal
	o.terminal = &first
	interrupt := o.interrupt
	o.mu.Unlock()
	if interrupt != nil {
		// The interruption verdict is carried by Execute's own error; a stop
		// that finds no active run is not an additional failure here.
		_ = interrupt()
	}
}

func (o *sealedMCPTerminalObserver) observed() *mcpserve.HandlerTerminal {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.terminal == nil {
		return nil
	}
	first := *o.terminal
	return &first
}

// sealedTerminalOutcome maps the first scoped-MCP terminal onto the lifecycle's
// exit classes: exit 1 is a verdict, and any other terminal is operational. An
// operational terminal never downgrades to a verdict, and an observed terminal
// always yields a non-nil error so no completion or receipt can follow.
func sealedTerminalOutcome(cause error, terminal *mcpserve.HandlerTerminal) error {
	if terminal == nil {
		return cause
	}
	if terminal.ExitCode == 1 {
		if cause != nil {
			return cause
		}
		return fmt.Errorf("%w: scoped MCP ended the run with a verdict terminal", sealedexec.ErrVerdict)
	}
	if cause == nil {
		return fmt.Errorf("%w: scoped MCP ended the run with an operational terminal (exit %d)", sealedexec.ErrOperational, terminal.ExitCode)
	}
	return fmt.Errorf("%w: scoped MCP ended the run with an operational terminal (exit %d): %v", sealedexec.ErrOperational, terminal.ExitCode, cause)
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
	processor, err := sealedexec.NewDetailProcessor(controller)
	if err != nil {
		return sealedRuntime{}, err
	}
	var adapter sealedexec.ExecutionAdapter
	var adapterCloser func(context.Context) error
	var terminals *sealedMCPTerminalObserver
	switch request.Adapter {
	case contextevent.AdapterCodex:
		adapter, err = codex.New(commandCodexProcess{}, processor)
		if err != nil {
			return sealedRuntime{}, err
		}
	case contextevent.AdapterClaude:
		terminals = &sealedMCPTerminalObserver{}
		ca := &commandClaudeAdapter{
			process:    commandClaudeProcess{},
			processor:  processor,
			controller: controller,
			request:    request,
			observer:   terminals,
		}
		adapter = ca
		adapterCloser = ca.Close
	default:
		return sealedRuntime{}, fmt.Errorf("%w: unsupported adapter %q", sealedexec.ErrVerdict, request.Adapter)
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
		Segments: controller,
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
	if terminals != nil {
		terminals.bind(func() error {
			_, err := execution.InterruptRegistered(context.Background(), request)
			return err
		})
	}
	return sealedRuntime{root: root, data: data, execution: execution, completion: completion, handback: handback, closer: adapterCloser, terminals: terminals}, nil
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
	return resolvedProfileFromMaterial(material, ref, workspacePath, grants)
}

// resolvedProfileFromMaterial activates exactly one provider arm of the
// credential-free controller material. Amendment 002 §3 requires the Claude
// arm to carry a full model identifier and an absolute configuration
// directory beneath the resolved environment root, and requires the activated
// profile to bind that directory plus the amendment's three fixed Claude
// controls; the Codex arm is unchanged and admits neither Claude field. The
// locally activated ANTHROPIC_API_KEY and the classified secret set are
// credential material and are never fabricated here.
func resolvedProfileFromMaterial(material sealedexec.ProfileMaterial, ref sealedexec.LogicalRef, workspacePath string, grants execworkspace.GrantSet) (sealedexec.ResolvedProfile, error) {
	if material.Ref != ref || !filepath.IsAbs(material.AbsoluteExecutable) || !filepath.IsAbs(material.AbsoluteEnvRoot) {
		return sealedexec.ResolvedProfile{}, fmt.Errorf("%w: resolved profile material contradicts request or contains a relative path", sealedexec.ErrVerdict)
	}
	claudeArm := material.Model != "" || material.ClaudeConfigDir != ""
	declaredEnv := map[string]string{}
	var policySecretValues [][]byte
	switch {
	case claudeArm && material.AbsoluteCodexHome != "":
		return sealedexec.ResolvedProfile{}, fmt.Errorf("%w: resolved profile material selects both the Codex and Claude arms", sealedexec.ErrVerdict)
	case claudeArm:
		if material.Model == "" || !cleanPathBelow(material.AbsoluteEnvRoot, material.ClaudeConfigDir) {
			return sealedexec.ResolvedProfile{}, fmt.Errorf("%w: resolved Claude profile material lacks an exact model or a configuration directory beneath the environment root", sealedexec.ErrVerdict)
		}
		// Locally classify the ANTHROPIC_API_KEY: this value never enters the
		// controller wire or digest plaintext — it is read from the binary's own
		// process environment and must be at least 8 UTF-8 bytes (Amendment 002 §3).
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if len(apiKey) < 8 {
			return sealedexec.ResolvedProfile{}, fmt.Errorf("%w: ANTHROPIC_API_KEY must be present and at least 8 bytes for Claude activation", sealedexec.ErrVerdict)
		}
		declaredEnv["ANTHROPIC_API_KEY"] = apiKey
		declaredEnv["CLAUDE_CONFIG_DIR"] = material.ClaudeConfigDir
		// Amendment 002 §3 requires an exact profile-selected deterministic tool
		// path. The controller material carries no PATH field, so the binary
		// selects the fixed system tool path: it is absolute, clean, ambient-free,
		// and identical on every run.
		declaredEnv["PATH"] = sealedClaudeToolPath
		declaredEnv["DISABLE_AUTOUPDATER"] = "1"
		declaredEnv["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"] = "1"
		declaredEnv["CLAUDE_CODE_AUTO_CONNECT_IDE"] = "false"
		policySecretValues = [][]byte{[]byte(apiKey)}
	default:
		if !filepath.IsAbs(material.AbsoluteCodexHome) {
			return sealedexec.ResolvedProfile{}, fmt.Errorf("%w: resolved profile material contradicts request or contains a relative path", sealedexec.ErrVerdict)
		}
		declaredEnv["CODEX_HOME"] = material.AbsoluteCodexHome
		// Classify the Codex home path as the policy secret: the exact
		// execution-unit path is redacted from any provider-observable channel.
		policySecretValues = [][]byte{[]byte(material.AbsoluteCodexHome)}
	}
	profile, enforcement, err := execworkspace.BuildProfile(workspacePath, material.AbsoluteEnvRoot, grants, declaredEnv)
	if err != nil {
		message := redactSealedPathMessage(err.Error(), workspacePath, material.AbsoluteExecutable, material.AbsoluteEnvRoot, filepath.Dir(material.AbsoluteEnvRoot), material.AbsoluteCodexHome, material.ClaudeConfigDir)
		return sealedexec.ResolvedProfile{}, errors.New(message)
	}
	return sealedexec.ResolvedProfile{
		Verification: sealedexec.Verification{State: contextcompile.ResolutionProven, Witnesses: []string{}},
		Ref:          ref, Digest: ref.Digest, Name: material.Name, Executable: material.AbsoluteExecutable,
		CodexHome: material.AbsoluteCodexHome, Model: material.Model, ClaudeConfigDir: material.ClaudeConfigDir,
		AdapterVersion: material.AdapterVersion,
		DecoderProfile: material.DecoderProfile, WorkspacePath: workspacePath,
		Profile: profile, Grants: grants, Enforcement: *enforcement,
		PolicySecretValues: policySecretValues, ClassificationComplete: true,
	}, nil
}

// cleanPathBelow reports whether child is a clean absolute path strictly
// beneath parent.
func cleanPathBelow(parent, child string) bool {
	if !filepath.IsAbs(child) || filepath.Clean(child) != child {
		return false
	}
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != "." && relative != ".." &&
		!filepath.IsAbs(relative) && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
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

// ---------------------------------------------------------------------------
// Claude adapter: lazy process and HTTP MCP lifecycle
// ---------------------------------------------------------------------------

// commandClaudeAdapter is the binary-side Claude adapter that defers the HTTP
// MCP server and real Adapter construction to VerifyAdapter time. This allows
// the profile's env root (required for the scoped config path) to be resolved
// by the service before the MCP lifecycle is committed.
type commandClaudeAdapter struct {
	process    claude.Process
	processor  *sealedexec.DetailProcessor
	controller *sealedexec.ControllerClient
	request    sealedexec.ExecutionRequest

	observer *sealedMCPTerminalObserver

	mu        sync.Mutex
	inner     *claude.Adapter
	closeMCP  func(context.Context) error
	terminals <-chan *mcpserve.HandlerTerminal
	watching  chan struct{}
}

// init starts the scoped HTTP MCP server and constructs the real claude.Adapter.
// It is called once at VerifyAdapter time when the profile and workspace are
// resolved. Subsequent calls are no-ops (the inner adapter is already set).
func (a *commandClaudeAdapter) init(ctx context.Context, check sealedexec.AdapterCheck) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.inner != nil {
		return nil
	}

	// Derive the env root from HOME in the profile env: profile sets
	// HOME = filepath.Join(envRoot, ".home"), so parent is envRoot.
	home := sealedEnvValue(check.Profile.Profile.Env(), "HOME")
	if home == "" {
		return errors.New("commandClaudeAdapter: resolved profile env is missing HOME")
	}
	envRoot := filepath.Dir(home)
	if !filepath.IsAbs(envRoot) {
		return errors.New("commandClaudeAdapter: derived env root is not absolute")
	}

	// Encode the canonical request bytes for the MCP server digest.
	requestBytes, err := sealedexec.EncodeExecutionRequest(check.Request)
	if err != nil {
		return fmt.Errorf("commandClaudeAdapter: encode request: %w", err)
	}

	// I-86/I-110: the parent-hosted scoped surface never invents its own append
	// position. It reconstructs the exact durable state from the authoritative
	// controller checkpoint and expansion ledger through the same
	// buildMCPFlightSnapshot gate the out-of-process `verdi context mcp` server
	// uses, so a pristine, active-tail, or invalidated flight keeps its ratified
	// meaning instead of being presented as sequence one.
	key := sealedexec.ExecutionKey{Flight: check.Request.Flight, Lane: check.Request.Lane, Epoch: check.Request.Epoch}
	recorder := controllerRecorder{client: a.controller}
	checkpoint, err := recorder.Checkpoint(ctx, key)
	if err != nil {
		return fmt.Errorf("commandClaudeAdapter: recorder checkpoint: %w", err)
	}
	expansion, err := a.controller.VerifyExpansion(ctx, key)
	if err != nil {
		return fmt.Errorf("commandClaudeAdapter: verify expansion ledger: %w", err)
	}
	snapshot, err := buildMCPFlightSnapshot(check.Request, check.Workspace, checkpoint, expansion)
	if err != nil {
		return fmt.Errorf("commandClaudeAdapter: reconstruct scoped MCP flight state: %w", err)
	}

	// Create the loopback listener for the scoped HTTP MCP server.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("commandClaudeAdapter: create MCP listener: %w", err)
	}

	// Build the ScopedMCP server: same ports as context_mcp.go.
	server, err := sealedexec.NewScopedMCP(sealedexec.ScopedMCPPorts{
		Resolver: mcpControllerResolver{client: a.controller, key: key},
		Compiler: sealedexec.NewCanonicalChildCompiler(),
		Verifier: a.controller,
		Recorder: recorder,
		Store:    a.controller,
		Stamps:   a.controller,
	}, sealedexec.NewFlightState(snapshot))
	if err != nil {
		_ = listener.Close()
		return fmt.Errorf("commandClaudeAdapter: create scoped MCP server: %w", err)
	}

	// Start the HTTP MCP server. The returned closeMCP is called after provider
	// reap (in commandClaudeAdapter.Close) to shut down the server and remove
	// the config file.
	mcpConfig, terminals, closeMCP, err := claude.StartScopedMCP(
		ctx, listener, envRoot, requestBytes,
		check.Profile.Digest, check.Workspace.WorkspaceID,
		scopedMCPHandler{server: server},
	)
	if err != nil {
		_ = listener.Close()
		return fmt.Errorf("commandClaudeAdapter: start scoped MCP: %w", err)
	}

	// Construct the real per-command Claude adapter.
	inner, err := claude.New(a.process, a.processor, mcpConfig)
	if err != nil {
		_ = closeMCP(ctx)
		return fmt.Errorf("commandClaudeAdapter: construct Claude adapter: %w", err)
	}

	a.inner = inner
	a.closeMCP = closeMCP
	a.terminals = terminals
	// Consume the first typed terminal promptly, while Execute is still live,
	// so epoch invalidation or a malformed scoped frame ends the run before any
	// completion or receipt is attempted.
	watching := make(chan struct{})
	a.watching = watching
	go func() {
		select {
		case terminal := <-terminals:
			a.observer.observe(terminal)
		case <-watching:
		}
	}()
	return nil
}

func (a *commandClaudeAdapter) VerifyAdapter(ctx context.Context, check sealedexec.AdapterCheck) (sealedexec.AdapterFacts, error) {
	if err := a.init(ctx, check); err != nil {
		return sealedexec.AdapterFacts{}, err
	}
	return a.inner.VerifyAdapter(ctx, check)
}

func (a *commandClaudeAdapter) Start(ctx context.Context, launch sealedexec.AdapterLaunch) (sealedexec.ActiveAdapterRun, error) {
	a.mu.Lock()
	inner := a.inner
	a.mu.Unlock()
	if inner == nil {
		return nil, errors.New("commandClaudeAdapter: Start called before initialization")
	}
	return inner.Start(ctx, launch)
}

func (a *commandClaudeAdapter) Resume(ctx context.Context, launch sealedexec.AdapterLaunch, sessionRef string) (sealedexec.ActiveAdapterRun, error) {
	a.mu.Lock()
	inner := a.inner
	a.mu.Unlock()
	if inner == nil {
		return nil, errors.New("commandClaudeAdapter: Resume called before initialization")
	}
	return inner.Resume(ctx, launch, sessionRef)
}

// Close is called after provider reap. It retires the terminal watcher, drains
// any terminal raised in the reap window, and then shuts the HTTP MCP server
// down and removes only the scoped configuration, in that order.
func (a *commandClaudeAdapter) Close(ctx context.Context) error {
	a.mu.Lock()
	closeFn := a.closeMCP
	terminals := a.terminals
	watching := a.watching
	a.watching = nil
	a.mu.Unlock()

	if watching != nil {
		close(watching)
	}
	// The watcher may have retired between the provider's last framed response
	// and this reap; the buffered single delivery is drained here so the run is
	// still classified by that terminal.
	if terminals != nil {
		select {
		case terminal := <-terminals:
			a.observer.observe(terminal)
		default:
		}
	}

	if closeFn != nil {
		return closeFn(ctx)
	}
	return nil
}

// sealedEnvValue looks up name in a sorted KEY=VALUE env slice and returns
// the value, or "" when absent. It is the binary-local counterpart to the
// unexported envValueFromProfile in internal/sealedexec/service.go.
func sealedEnvValue(env []string, name string) string {
	prefix := name + "="
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return strings.TrimPrefix(kv, prefix)
		}
	}
	return ""
}

// commandClaudeProcess implements the claude.Process interface for the real
// binary environment. Probe runs the version probe synchronously; Start
// launches the provider and returns a pumping active run.
type commandClaudeProcess struct{}

func (commandClaudeProcess) Probe(_ context.Context, command *exec.Cmd) (stdout, stderr []byte, exitCode int, err error) {
	if command == nil {
		return nil, nil, 0, errors.New("sealed claude process: nil probe command")
	}
	var outBuf, errBuf bytes.Buffer
	command.Stdout = &outBuf
	command.Stderr = &errBuf
	runErr := command.Run()
	var exitErr *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exitErr) {
		return nil, nil, 0, runErr
	}
	code := 0
	if exitErr != nil {
		code = exitErr.ExitCode()
	}
	return outBuf.Bytes(), errBuf.Bytes(), code, nil
}

func (commandClaudeProcess) Start(_ context.Context, command *exec.Cmd, stdin []byte) (claude.ActiveProcess, error) {
	if command == nil {
		return nil, errors.New("sealed claude process: nil start command")
	}
	if len(command.ExtraFiles) != 0 {
		return nil, errors.New("sealed claude process: provider child must not inherit extra files")
	}
	stdoutPipe, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("sealed claude process: stdout pipe: %w", err)
	}
	stderrPipe, err := command.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("sealed claude process: stderr pipe: %w", err)
	}
	command.Stdin = bytes.NewReader(stdin)
	if err := command.Start(); err != nil {
		message := strings.ReplaceAll(err.Error(), command.Path, "<profile-executable>")
		return nil, errors.New(message)
	}
	active := &commandClaudeRun{
		command: command,
		frames:  make(chan commandClaudeFrame, 16),
	}
	go active.pump(stdoutPipe, stderrPipe)
	return active, nil
}

type commandClaudeFrame struct {
	obs claude.ProcessObservation
	err error
}

type commandClaudeRun struct {
	command  *exec.Cmd
	frames   chan commandClaudeFrame
	stopOnce sync.Once
}

func (run *commandClaudeRun) pump(stdout io.Reader, stderrReader io.Reader) {
	defer close(run.frames)

	// Accumulate stderr in a background goroutine so it does not block stdout.
	var stderrBuf bytes.Buffer
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		_, _ = io.Copy(&stderrBuf, stderrReader)
	}()

	// Emit JSONL frames from stdout.
	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) != 0 {
			complete := err == nil
			if complete {
				line = bytes.TrimSuffix(line, []byte{'\n'})
			}
			run.frames <- commandClaudeFrame{obs: claude.ProcessObservation{ForeignJSON: line, Complete: complete}}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				run.frames <- commandClaudeFrame{err: err}
			}
			break
		}
	}

	// Wait for stderr reader to finish before Wait().
	<-stderrDone

	// Reap the process and emit the terminal observation.
	waitErr := run.command.Wait()
	exitCode := 0
	if waitErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(waitErr, &exitErr) {
			run.frames <- commandClaudeFrame{err: waitErr}
			return
		}
		exitCode = exitErr.ExitCode()
	}
	run.frames <- commandClaudeFrame{obs: claude.ProcessObservation{
		Terminal: &claude.ProcessResult{ExitCode: exitCode, Stderr: stderrBuf.Bytes()},
	}}
}

func (run *commandClaudeRun) Next(ctx context.Context) (claude.ProcessObservation, error) {
	select {
	case <-ctx.Done():
		return claude.ProcessObservation{}, ctx.Err()
	case frame, ok := <-run.frames:
		if !ok {
			return claude.ProcessObservation{}, io.EOF
		}
		return frame.obs, frame.err
	}
}

func (run *commandClaudeRun) Stop(context.Context) (claude.ProcessStopResult, error) {
	var stopErr error
	run.stopOnce.Do(func() {
		stopErr = run.command.Process.Kill()
	})
	if stopErr != nil && !errors.Is(stopErr, os.ErrProcessDone) {
		return claude.ProcessStopResult{}, stopErr
	}
	return claude.ProcessStopResult{ExitCode: 130, ReasonCode: "interrupt-requested"}, nil
}
