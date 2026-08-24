package experimentapp

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"

	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentpolicy"
	"github.com/jyang234/verdi/internal/experimentrun"
)

// ExecutionInput identifies the caller-owned durable run identity.
type ExecutionInput struct {
	Run string
}

// ExecutionResult is the application projection of the existing runner's
// complete receipt, observations, and result.
type ExecutionResult struct {
	Outcome        Outcome
	AcceptedHead   string
	ExperimentPath string
	Run            string
	Execution      experimentrun.StartResult
}

// ExecutionRunner receives the exact already-projected authorization. Its
// production implementation below creates and delegates to experimentrun.
type ExecutionRunner interface {
	Start(context.Context, experimentrun.StartRequest, experimentrun.ExecutionAuthorization) (experimentrun.StartResult, error)
	Resume(context.Context, experimentrun.ResumeRequest, experimentrun.ExecutionAuthorization) (experimentrun.StartResult, error)
}

// RunDependencies are the existing experimentrun dependencies other than
// authorization, which is operation-scoped and injected by this package.
type RunDependencies struct {
	Inputs       experimentrun.InputResolver
	Materializer experimentrun.WorkspaceMaterializer
	Evaluator    experimentrun.AttemptEvaluator
	Versions     experiment.ReceiptVersions
}

// RunDelegate delegates every execution mechanic to experimentrun.Service.
type RunDelegate struct {
	dependencies RunDependencies
}

// NewRunDelegate validates the non-authority runner composition ports.
func NewRunDelegate(dependencies RunDependencies) (*RunDelegate, error) {
	if dependencies.Inputs == nil || dependencies.Materializer == nil {
		return nil, fmt.Errorf("experimentapp: run input resolver and materializer are required")
	}
	if err := dependencies.Versions.Validate(); err != nil {
		return nil, fmt.Errorf("experimentapp: run versions: %w", err)
	}
	return &RunDelegate{dependencies: dependencies}, nil
}

func (r *RunDelegate) service(authorization experimentrun.ExecutionAuthorization) (*experimentrun.Service, error) {
	if r == nil {
		return nil, fmt.Errorf("experimentapp: run delegate is nil")
	}
	return experimentrun.NewService(experimentrun.ServiceDependencies{
		Authorization: exactAuthorization{authorization: cloneExecutionAuthorization(authorization)},
		Inputs:        r.dependencies.Inputs, Materializer: r.dependencies.Materializer,
		Evaluator: r.dependencies.Evaluator, Versions: r.dependencies.Versions,
	})
}

// Start injects the exact application authorization and delegates unchanged.
func (r *RunDelegate) Start(ctx context.Context, request experimentrun.StartRequest, authorization experimentrun.ExecutionAuthorization) (experimentrun.StartResult, error) {
	runner, err := r.service(authorization)
	if err != nil {
		return experimentrun.StartResult{}, err
	}
	return runner.Start(ctx, request)
}

// Resume injects the exact application authorization and delegates unchanged.
func (r *RunDelegate) Resume(ctx context.Context, request experimentrun.ResumeRequest, authorization experimentrun.ExecutionAuthorization) (experimentrun.StartResult, error) {
	runner, err := r.service(authorization)
	if err != nil {
		return experimentrun.StartResult{}, err
	}
	return runner.Resume(ctx, request)
}

type exactAuthorization struct {
	authorization experimentrun.ExecutionAuthorization
}

func (a exactAuthorization) ResolveExecutionAuthorization(context.Context, experiment.Definition, experiment.Capabilities) (experimentrun.ExecutionAuthorization, error) {
	return cloneExecutionAuthorization(a.authorization), nil
}

func cloneExecutionAuthorization(in experimentrun.ExecutionAuthorization) experimentrun.ExecutionAuthorization {
	out := in
	out.GrantBytes = append([]byte(nil), in.GrantBytes...)
	out.DeclaredEnv = make(map[string]string, len(in.DeclaredEnv))
	for key, value := range in.DeclaredEnv {
		out.DeclaredEnv[key] = value
	}
	return out
}

// Start runs one accepted locked experiment under its exact accepted policy.
func (s *Service) Start(ctx context.Context, identity Identity, input ExecutionInput) ExecutionResult {
	return s.execute(ctx, identity, input, false)
}

// Resume resumes only the missing observations for the same accepted inputs.
func (s *Service) Resume(ctx context.Context, identity Identity, input ExecutionInput) ExecutionResult {
	return s.execute(ctx, identity, input, true)
}

func (s *Service) execute(ctx context.Context, identity Identity, input ExecutionInput, resume bool) ExecutionResult {
	if ctx == nil {
		return ExecutionResult{Outcome: operationalOutcome("invalid-request", fmt.Errorf("experimentapp: execution context is nil"))}
	}
	if err := identity.validate(); err != nil {
		return ExecutionResult{Outcome: operationalOutcome("invalid-request", err)}
	}
	if err := experiment.ValidateID(input.Run); err != nil {
		return ExecutionResult{Outcome: operationalOutcome("invalid-request", fmt.Errorf("experimentapp: run id: %w", err))}
	}
	if s == nil {
		return ExecutionResult{Outcome: operationalOutcome("runner-unavailable", fmt.Errorf("experimentapp: execution service is unavailable"))}
	}

	registration := s.AcceptedRegistration(ctx, identity)
	if registration.Outcome.Classification != ClassificationClean {
		return ExecutionResult{Outcome: registration.Outcome}
	}
	if s.runner == nil {
		return ExecutionResult{Outcome: operationalOutcome("runner-unavailable", fmt.Errorf("experimentapp: execution runner is unavailable"))}
	}
	snapshot, err := resolveAccepted(ctx, s.git, identity)
	if err != nil {
		var stale *staleAcceptedHeadError
		if errors.As(err, &stale) {
			return ExecutionResult{Outcome: verdictOutcome("accepted-head-stale", stale.Error())}
		}
		return ExecutionResult{Outcome: operationalOutcome("accepted-tree-invalid", err)}
	}
	if snapshot.revision.Head != registration.AcceptedHead || snapshot.experimentPath != registration.ExperimentPath {
		return ExecutionResult{Outcome: operationalOutcome("accepted-tree-invalid", fmt.Errorf("experimentapp: accepted registration and execution snapshot differ"))}
	}
	definition, capabilities, candidatePaths, err := acceptedExecutionInputs(snapshot)
	if err != nil {
		return ExecutionResult{Outcome: operationalOutcome("accepted-execution-invalid", err)}
	}
	decision, err := s.policy.ResolvePolicy(ctx, clonePolicyRequest(PolicyRequest{
		CheckoutRoot: identity.CheckoutRoot, ExperimentPath: snapshot.experimentPath,
		Spike: identity.Spike, AcceptedCommit: snapshot.revision.Head,
		Definition: definition, Capabilities: capabilities, CandidatePaths: candidatePaths,
	}))
	if err != nil {
		return ExecutionResult{Outcome: policyResolutionErrorOutcome(err)}
	}
	if decision == nil {
		return ExecutionResult{Outcome: operationalOutcome("policy-resolution-invalid", fmt.Errorf("experimentapp: policy resolver returned nil decision"))}
	}
	authorization, err := experimentpolicy.Authorize(decision, experimentpolicy.AuthorizationInput{
		Definition: definition, Capabilities: capabilities,
		ExperimentPath: snapshot.experimentPath, CandidatePaths: candidatePaths,
	})
	if err != nil {
		return ExecutionResult{Outcome: verdictOutcome("policy-refused", err.Error())}
	}
	worktreeFiles, err := readProposedArtifactFiles(identity.CheckoutRoot, snapshot.experimentPath)
	if err != nil {
		return ExecutionResult{Outcome: operationalOutcome("worktree-input-invalid", err)}
	}
	worktreeDigest, err := artifactSetDigest(worktreeFiles, snapshot.experimentPath)
	if err != nil {
		return ExecutionResult{Outcome: operationalOutcome("worktree-input-invalid", err)}
	}
	if worktreeDigest != registration.ArtifactDigest {
		return ExecutionResult{Outcome: operationalOutcome("locked-input-mismatch", fmt.Errorf("experimentapp: worktree mutation-artifact digest %s does not match accepted registration %s", worktreeDigest, registration.ArtifactDigest))}
	}

	request := experimentrun.StartRequest{
		Root: identity.CheckoutRoot, ExperimentDir: snapshot.experimentPath,
		Run: input.Run, Definition: cloneDefinition(definition),
	}
	var execution experimentrun.StartResult
	if resume {
		execution, err = s.runner.Resume(ctx, experimentrun.ResumeRequest(request), cloneExecutionAuthorization(authorization))
	} else {
		execution, err = s.runner.Start(ctx, request, cloneExecutionAuthorization(authorization))
	}
	if err != nil {
		return ExecutionResult{Outcome: operationalOutcome("runner-failed", err)}
	}
	outcome, err := classifyExecutionResult(input.Run, registration.DefinitionDigest, execution)
	if err != nil {
		return ExecutionResult{Outcome: operationalOutcome("runner-result-invalid", err)}
	}
	return ExecutionResult{
		Outcome: outcome, AcceptedHead: registration.AcceptedHead,
		ExperimentPath: registration.ExperimentPath, Run: input.Run, Execution: execution,
	}
}

func acceptedExecutionInputs(snapshot acceptedSnapshot) (experiment.Definition, experiment.Capabilities, []string, error) {
	definitionBytes, err := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, "experiment.yaml"))
	if err != nil {
		return experiment.Definition{}, experiment.Capabilities{}, nil, err
	}
	definition, err := experiment.DecodeDefinition(definitionBytes)
	if err != nil {
		return experiment.Definition{}, experiment.Capabilities{}, nil, err
	}
	capabilitiesBytes, err := fs.ReadFile(snapshot.source, path.Join(snapshot.experimentPath, "evaluator-capabilities.json"))
	if err != nil {
		return experiment.Definition{}, experiment.Capabilities{}, nil, err
	}
	capabilities, err := experiment.DecodeCapabilities(capabilitiesBytes)
	if err != nil {
		return experiment.Definition{}, experiment.Capabilities{}, nil, err
	}
	if rawDigest(capabilitiesBytes) != definition.Evaluator.CapabilitiesDigest {
		return experiment.Definition{}, experiment.Capabilities{}, nil, fmt.Errorf("accepted capabilities do not match the locked definition")
	}
	candidatePaths, err := experiment.ValidateCandidatePatchesFromSource(snapshot.source, snapshot.experimentPath, definition)
	if err != nil {
		return experiment.Definition{}, experiment.Capabilities{}, nil, err
	}
	return definition, capabilities, candidatePaths, nil
}

func classifyExecutionResult(run, definitionDigest string, execution experimentrun.StartResult) (Outcome, error) {
	if execution.Receipt.Run != run {
		return Outcome{}, fmt.Errorf("experimentapp: runner receipt run %q does not match caller run %q", execution.Receipt.Run, run)
	}
	if err := execution.Result.Validate(); err != nil {
		return Outcome{}, fmt.Errorf("experimentapp: runner result: %w", err)
	}
	decision, err := resultDecision(execution.Result)
	if err != nil {
		return Outcome{}, err
	}
	if decision.Run != run || decision.DefinitionDigest != definitionDigest {
		return Outcome{}, fmt.Errorf("experimentapp: runner result identity does not match accepted definition/run")
	}
	digest, err := experiment.ResultDigest(execution.Result)
	if err != nil {
		return Outcome{}, err
	}
	if execution.ResultDigest != digest {
		return Outcome{}, fmt.Errorf("experimentapp: runner result digest %q does not match exact result %q", execution.ResultDigest, digest)
	}
	return outcomeForVerdict(decision.Verdict), nil
}

func resultDecision(result experiment.Result) (experiment.ResultDecision, error) {
	switch result.Schema {
	case experiment.ResultSchemaV2:
		if result.Decision == nil {
			return experiment.ResultDecision{}, fmt.Errorf("experimentapp: result v2 has no decision")
		}
		return *result.Decision, nil
	case experiment.ResultSchema:
		candidates := make([]experiment.DecisionCandidate, len(result.Candidates))
		for index, candidate := range result.Candidates {
			candidates[index] = experiment.DecisionCandidate{
				ID: candidate.ID, Baseline: candidate.Baseline, Eligible: candidate.Eligible,
				Violations: candidate.Violations, Primary: candidate.Primary, Bounds: candidate.Bounds,
				ExecutionFailures: []experiment.CandidateExecutionFailure{},
			}
		}
		return experiment.ResultDecision{
			Experiment: result.Experiment, DefinitionDigest: result.DefinitionDigest, Run: result.Run,
			Algorithm: result.Algorithm, Verdict: result.Verdict, Winner: result.Winner,
			Reasons: result.Reasons, Candidates: candidates, ObservationsDigest: result.ObservationsDigest,
		}, nil
	default:
		return experiment.ResultDecision{}, fmt.Errorf("experimentapp: unknown result schema %q", result.Schema)
	}
}

func outcomeForVerdict(verdict experiment.Verdict) Outcome {
	switch verdict {
	case experiment.VerdictProvenWinner:
		return cleanOutcome()
	case experiment.VerdictViolatedWithWitness:
		return verdictOutcome("comparison-violated", "completed comparison is violated with witness")
	case experiment.VerdictDisclosedUnproven:
		return verdictOutcome("comparison-inconclusive", "completed comparison is disclosed as unproven")
	default:
		return operationalOutcome("runner-result-invalid", fmt.Errorf("experimentapp: unknown result verdict %q", verdict))
	}
}
