package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/mcpserve"
	"github.com/jyang234/verdi/internal/sealedexec"
	"github.com/jyang234/verdi/internal/store"
)

const contextMCPUsage = "usage: verdi context mcp --request <path|->"

func cmdContextMCP(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	requestPath, ok := parseContextMCPArgs(args)
	if !ok {
		fmt.Fprintln(stderr, contextMCPUsage)
		return 2
	}
	request, mcpInput, err := readContextMCPRequest(requestPath, stdin)
	if err != nil {
		fmt.Fprintln(stderr, "context mcp:", err)
		return 2
	}
	controller, controllerConn, err := openSealedController()
	if err != nil {
		fmt.Fprintln(stderr, "context mcp:", err)
		return 2
	}
	defer controllerConn.Close()
	ctx := context.Background()
	authority, err := controller.VerifyAuthority(ctx, request)
	if err != nil {
		printSealedContextDiagnostic(stderr, "mcp", request, err)
		return contextExecutionExitCode(err)
	}
	if err := validateSealedAuthority(request, authority); err != nil {
		printSealedContextDiagnostic(stderr, "mcp", request, err)
		return contextExecutionExitCode(err)
	}
	root, err := store.FindRoot(request.ATCRunway)
	if err != nil {
		printSealedContextDiagnostic(stderr, "mcp", request, fmt.Errorf("derive store root from request runway: %w", err))
		return 2
	}
	runway, err := (localRunwayVerifier{}).VerifyRunway(ctx, request.ATCRunway)
	if err != nil {
		printSealedContextDiagnostic(stderr, "mcp", request, err, root)
		return 2
	}
	if err := validateMCPRunway(request, runway); err != nil {
		printSealedContextDiagnostic(stderr, "mcp", request, err, root)
		return contextExecutionExitCode(err)
	}
	workspaceID, err := request.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		printSealedContextDiagnostic(stderr, "mcp", request, err, root)
		return 2
	}
	workspacePath := execworkspace.UnitPath(root, workspaceID)
	workspace, err := (localWorkspaceVerifier{root: root}).VerifyWorkspace(ctx, workspacePath, request.ExecutionWorkspaceRequest)
	if err != nil {
		printSealedContextDiagnostic(stderr, "mcp", request, err, root, workspacePath)
		return 2
	}
	recorderFacts, err := controller.ResolveRecorder(ctx, request.RecorderEndpoint)
	if err != nil {
		printSealedContextDiagnostic(stderr, "mcp", request, err, root, workspacePath)
		return contextExecutionExitCode(err)
	}
	if err := validateMCPRecorder(request, recorderFacts); err != nil {
		printSealedContextDiagnostic(stderr, "mcp", request, err, root, workspacePath)
		return contextExecutionExitCode(err)
	}
	key := sealedexec.ExecutionKey{Flight: request.Flight, Lane: request.Lane, Epoch: request.Epoch}
	recorder := controllerRecorder{client: controller}
	checkpoint, err := recorder.Checkpoint(ctx, key)
	if err != nil {
		printSealedContextDiagnostic(stderr, "mcp", request, err, root, workspacePath)
		return contextExecutionExitCode(err)
	}
	expansion, err := controller.VerifyExpansion(ctx, key)
	if err != nil {
		printSealedContextDiagnostic(stderr, "mcp", request, err, root, workspacePath)
		return contextExecutionExitCode(err)
	}
	snapshot, err := buildMCPFlightSnapshot(request, workspace, checkpoint, expansion)
	if err != nil {
		printSealedContextDiagnostic(stderr, "mcp", request, err, root, workspacePath)
		return contextExecutionExitCode(err)
	}
	server, err := sealedexec.NewScopedMCP(sealedexec.ScopedMCPPorts{
		Resolver: mcpControllerResolver{client: controller, key: key},
		Compiler: sealedexec.NewCanonicalChildCompiler(), Verifier: controller,
		Recorder: recorder, Store: controller, Stamps: controller,
	}, sealedexec.NewFlightState(snapshot))
	if err != nil {
		printSealedContextDiagnostic(stderr, "mcp", request, err, root, workspacePath)
		return 2
	}
	terminal, err := mcpserve.ServeHandler(ctx, mcpInput, stdout, scopedMCPHandler{server: server})
	if err != nil {
		printSealedContextDiagnostic(stderr, "mcp", request, err, root, workspacePath)
		return 2
	}
	if terminal != nil {
		return terminal.ExitCode
	}
	return 0
}

func parseContextMCPArgs(args []string) (string, bool) {
	if len(args) != 2 || args[0] != "--request" || args[1] == "" || strings.HasPrefix(args[1], "--") {
		return "", false
	}
	return args[1], true
}

func readContextMCPRequest(requestPath string, stdin io.Reader) (sealedexec.ExecutionRequest, io.Reader, error) {
	if requestPath != "-" {
		data, err := os.ReadFile(requestPath)
		if err != nil {
			return sealedexec.ExecutionRequest{}, nil, fmt.Errorf("reading request: %w", err)
		}
		request, err := sealedexec.DecodeExecutionRequest(bytes.NewReader(data))
		if err != nil {
			return sealedexec.ExecutionRequest{}, nil, fmt.Errorf("decoding request: %w", err)
		}
		return request, stdin, nil
	}

	buffered := bufio.NewReader(stdin)
	frame, err := buffered.ReadBytes('\n')
	if err != nil {
		return sealedexec.ExecutionRequest{}, nil, fmt.Errorf("reading request frame: %w", err)
	}
	request, err := sealedexec.DecodeExecutionRequest(bytes.NewReader(frame))
	if err != nil {
		return sealedexec.ExecutionRequest{}, nil, fmt.Errorf("decoding request: %w", err)
	}
	return request, buffered, nil
}

func validateSealedAuthority(request sealedexec.ExecutionRequest, facts sealedexec.AuthorityFacts) error {
	if err := requireMCPProven("authority", facts.Verification); err != nil {
		return err
	}
	if facts.ManifestRevision != request.ManifestRevision || facts.ManifestDigest != request.ManifestDigest ||
		facts.ProjectionDigest != request.ProjectionDigest || facts.AuthorityDigest != request.AuthorityVerdict.Digest ||
		facts.AcceptedSpecCommit != request.Manifest.AcceptedSpec.Commit {
		return fmt.Errorf("%w: controller authority facts contradict the public request", sealedexec.ErrVerdict)
	}
	return nil
}

func validateMCPRunway(request sealedexec.ExecutionRequest, facts sealedexec.RunwayFacts) error {
	if err := requireMCPProven("runway", facts.Verification); err != nil {
		return err
	}
	if facts.Path != request.ATCRunway || facts.Commit != request.InputCommit || facts.Tree != request.InputTree || !facts.Clean {
		return fmt.Errorf("%w: runway facts contradict the public request", sealedexec.ErrVerdict)
	}
	return nil
}

func validateMCPRecorder(request sealedexec.ExecutionRequest, facts sealedexec.RecorderFacts) error {
	if err := requireMCPProven("recorder", facts.Verification); err != nil {
		return err
	}
	if facts.Ref != request.RecorderEndpoint {
		return fmt.Errorf("%w: recorder facts contradict the public request", sealedexec.ErrVerdict)
	}
	return nil
}

func requireMCPProven(name string, verification sealedexec.Verification) error {
	if verification.State != contextcompile.ResolutionProven || verification.Failure != sealedexec.FailureNone || len(verification.Witnesses) != 0 {
		return fmt.Errorf("%w: %s is not proven", sealedexec.ErrVerdict, name)
	}
	return nil
}

func buildMCPFlightSnapshot(request sealedexec.ExecutionRequest, workspace sealedexec.WorkspaceFacts, checkpoint sealedexec.RecorderCheckpoint, expansion sealedexec.ExpansionFacts) (sealedexec.FlightStateSnapshot, error) {
	if err := requireMCPProven("workspace", workspace.Verification); err != nil {
		return sealedexec.FlightStateSnapshot{}, err
	}
	workspaceID, err := request.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		return sealedexec.FlightStateSnapshot{}, err
	}
	workspaceDigest, err := sealedexec.ExecutionWorkspaceRequestDigest(request.ExecutionWorkspaceRequest)
	if err != nil {
		return sealedexec.FlightStateSnapshot{}, err
	}
	if workspace.WorkspaceID != workspaceID || !workspace.Request.Equal(request.ExecutionWorkspaceRequest) || workspace.RequestDigest != workspaceDigest ||
		workspace.CurrentCommit == "" || workspace.CurrentTree == "" || !workspace.Clean {
		return sealedexec.FlightStateSnapshot{}, fmt.Errorf("%w: workspace facts contradict the public request", sealedexec.ErrVerdict)
	}
	if err := requireMCPProven("recorder checkpoint", checkpoint.Verification); err != nil {
		return sealedexec.FlightStateSnapshot{}, err
	}
	if err := requireMCPProven("expansion ledger", expansion.Verification); err != nil {
		return sealedexec.FlightStateSnapshot{}, err
	}
	if len(checkpoint.Revisions) == 0 {
		if checkpoint.EventChainRoot != "" || checkpoint.TerminalSourceSequence != 0 || checkpoint.TerminalGlobalSequence != 0 {
			return sealedexec.FlightStateSnapshot{}, errors.New("scoped MCP pristine checkpoint carries completed-revision facts")
		}
	} else {
		root, err := contextevent.EventChainRoot(checkpoint.Revisions)
		if err != nil {
			return sealedexec.FlightStateSnapshot{}, fmt.Errorf("validate scoped MCP recorder checkpoint: %w", err)
		}
		terminal := checkpoint.Revisions[len(checkpoint.Revisions)-1]
		if checkpoint.EventChainRoot != root || checkpoint.TerminalSourceSequence != terminal.TerminalSourceSequence ||
			checkpoint.TerminalGlobalSequence != terminal.TerminalGlobalSequence {
			return sealedexec.FlightStateSnapshot{}, errors.New("scoped MCP recorder checkpoint terminal identity mismatch")
		}
	}
	active := checkpoint.ActiveRevision
	if active == nil {
		if len(checkpoint.Revisions) != 0 {
			return sealedexec.FlightStateSnapshot{}, fmt.Errorf("%w: completed scoped MCP checkpoint has no active revision", sealedexec.ErrVerdict)
		}
		if expansion.Root != "" {
			return sealedexec.FlightStateSnapshot{}, fmt.Errorf("%w: pristine scoped MCP checkpoint contradicts the no-ledger state", sealedexec.ErrVerdict)
		}
		return sealedexec.FlightStateSnapshot{
			Request: request, Key: sealedexec.ExecutionKey{Flight: request.Flight, Lane: request.Lane, Epoch: request.Epoch},
			WorkspaceID: workspace.WorkspaceID, CandidateCommit: workspace.CurrentCommit, CandidateTree: workspace.CurrentTree,
			Revision: request.ManifestRevision, ManifestDigest: request.ManifestDigest,
			ProjectionDigest: request.ProjectionDigest, ExpansionRoot: "", NextSourceSequence: 1,
		}, nil
	}
	if active.Revision < request.ManifestRevision {
		return sealedexec.FlightStateSnapshot{}, fmt.Errorf("%w: scoped MCP revision is below the public request", sealedexec.ErrVerdict)
	}
	if active.Revision == request.ManifestRevision {
		if active.ManifestDigest != request.ManifestDigest || expansion.Root != "" {
			return sealedexec.FlightStateSnapshot{}, fmt.Errorf("%w: initial scoped MCP state contradicts the public request or no-ledger fact", sealedexec.ErrVerdict)
		}
	} else if expansion.Root == "" {
		return sealedexec.FlightStateSnapshot{}, fmt.Errorf("%w: later scoped MCP state lacks its installed expansion root", sealedexec.ErrVerdict)
	}
	if active.NextSourceSequence == 1 {
		if active.PriorEventDigest != "" {
			return sealedexec.FlightStateSnapshot{}, fmt.Errorf("%w: sequence-one scoped MCP state carries a prior event digest", sealedexec.ErrVerdict)
		}
		if active.Revision == request.ManifestRevision {
			if active.PriorRevision != nil {
				return sealedexec.FlightStateSnapshot{}, fmt.Errorf("%w: initial scoped MCP state carries a predecessor bridge", sealedexec.ErrVerdict)
			}
		} else {
			bridge := active.PriorRevision
			if bridge == nil || active.Revision == 0 || bridge.ManifestRevision != active.Revision-1 || active.LastGlobalSequence != bridge.TerminalGlobalSequence {
				return sealedexec.FlightStateSnapshot{}, fmt.Errorf("%w: later scoped MCP state lacks its exact acknowledged predecessor bridge", sealedexec.ErrVerdict)
			}
			if bridge.ManifestRevision == request.ManifestRevision && bridge.ManifestDigest != request.ManifestDigest {
				return sealedexec.FlightStateSnapshot{}, fmt.Errorf("%w: scoped MCP predecessor bridge contradicts the public request", sealedexec.ErrVerdict)
			}
		}
	} else if active.PriorRevision != nil {
		return sealedexec.FlightStateSnapshot{}, fmt.Errorf("%w: later scoped MCP source sequence retains a predecessor bridge", sealedexec.ErrVerdict)
	}
	var priorRevision *contextevent.PriorRevision
	if active.PriorRevision != nil {
		copy := *active.PriorRevision
		priorRevision = &copy
	}
	return sealedexec.FlightStateSnapshot{
		Request: request, Key: sealedexec.ExecutionKey{Flight: request.Flight, Lane: request.Lane, Epoch: request.Epoch},
		WorkspaceID: workspace.WorkspaceID, CandidateCommit: workspace.CurrentCommit, CandidateTree: workspace.CurrentTree,
		Revision: active.Revision, ManifestDigest: active.ManifestDigest,
		ProjectionDigest: request.ProjectionDigest, ExpansionRoot: expansion.Root,
		NextSourceSequence: active.NextSourceSequence, PriorEventDigest: active.PriorEventDigest, PriorRevision: priorRevision,
		LastGlobalSequence: active.LastGlobalSequence, Invalidated: active.Invalidated,
	}, nil
}

type mcpControllerResolver struct {
	client *sealedexec.ControllerClient
	key    sealedexec.ExecutionKey
}

func (r mcpControllerResolver) ResolveContext(ctx context.Context, ref string) (sealedexec.ContextResolution, error) {
	return r.client.ResolveContext(ctx, sealedexec.ContextQuery{Key: r.key, Ref: ref})
}

type scopedMCPHandler struct{ server *sealedexec.ScopedMCP }

func (h scopedMCPHandler) Tools() []mcpserve.HandlerTool {
	tools := h.server.Tools()
	result := make([]mcpserve.HandlerTool, len(tools))
	for i, tool := range tools {
		result[i] = mcpserve.HandlerTool{Name: tool.Name, Description: tool.Description, InputSchema: append([]byte(nil), tool.InputSchema...)}
	}
	return result
}

func (h scopedMCPHandler) Call(ctx context.Context, name string, arguments json.RawMessage) (mcpserve.HandlerCallResult, error) {
	if name != sealedexec.ToolGetFlightPlan && name != sealedexec.ToolRequestContext {
		return mcpserve.HandlerCallResult{}, fmt.Errorf("unknown scoped tool %q", name)
	}
	result, err := h.server.Call(ctx, name, arguments)
	if err != nil {
		return mcpserve.HandlerCallResult{}, err
	}
	encoded, err := sealedexec.EncodeInspectionResult(result)
	if err != nil {
		return mcpserve.HandlerCallResult{}, err
	}
	call := mcpserve.HandlerCallResult{Text: string(encoded)}
	if result.Kind == sealedexec.InspectionEpochInvalidated {
		call.Terminal = &mcpserve.HandlerTerminal{ExitCode: 1}
	}
	return call, nil
}
